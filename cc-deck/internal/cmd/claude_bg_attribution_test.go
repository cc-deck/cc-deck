package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/procinfo"
)

const bgTestStart int64 = 1792307557
const bgTestJob = "2402e991"

type claudeBGScenario struct {
	t *testing.T
	*hookTestEnv
	src   *fakeProcs
	dir   string
	pipes []string
}

func newClaudeBGScenario(t *testing.T) *claudeBGScenario {
	t.Helper()
	s := &claudeBGScenario{t: t, hookTestEnv: newHookTestEnv(t)}
	s.dir = resolvePath(t.TempDir(), "")
	t.Setenv("HOME", s.dir)
	t.Setenv("CLAUDE_CONFIG_DIR", s.dir)
	t.Setenv("CLAUDE_BG_BACKEND", "daemon")
	t.Setenv("CC_DECK_PROFILE", "inherited")
	t.Setenv("CC_DECK_PROFILE_COLOR", "#ff0000")
	t.Setenv("ZELLIJ", "0")
	t.Setenv("ZELLIJ_SESSION_NAME", "inherited-session")
	s.src = &fakeProcs{self: 300, procs: map[int]procinfo.Process{}, getErr: map[int]error{}, ancestorErr: map[int]error{}}
	for _, p := range []procinfo.Process{
		{PID: 300, PPID: 200, Exe: "cc-deck"},
		{PID: 200, PPID: 100, Exe: "sh"},
		{PID: 100, PPID: 90, Exe: "claude", Comm: "2.1.302", StartTime: 42, StartUnix: bgTestStart, Env: map[string]string{"CLAUDE_CONFIG_DIR": s.dir, "ZELLIJ_PANE_ID": "19", "ZELLIJ": "0"}},
		{PID: 90, PPID: 1, Exe: "claude", Args: []string{"bg-pty-host", "--bg-pty-host"}},
	} {
		s.src.add(p)
	}
	origSource, origFile := hookProcSource, bgJobPaneFile
	hookProcSource = s.src
	bgJobPaneFile = filepath.Join(hookStateDir, "bg-job-panes.json")
	t.Cleanup(func() { hookProcSource = origSource; bgJobPaneFile = origFile })
	s.record(claudeRecord{PID: 100, Kind: "bg", JobID: bgTestJob, SessionID: "job-session", ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)})
	sendHookPayload = func(_ context.Context, _ string, target, pipe string, payload []byte) error {
		s.sent = append(s.sent, append([]byte(nil), payload...))
		s.targets = append(s.targets, target)
		s.pipes = append(s.pipes, pipe)
		return nil
	}
	return s
}

func (s *claudeBGScenario) record(r claudeRecord) {
	s.t.Helper()
	dir := filepath.Join(s.dir, "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		s.t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		s.t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, fmt.Sprint(r.PID)+".json"), b, 0600); err != nil {
		s.t.Fatal(err)
	}
}
func (s *claudeBGScenario) viewer(pid int, via, pane, session string) {
	p := procinfo.Process{PID: pid, PPID: 1, Exe: "/Users/x/.local/bin/claude", Comm: "2.1.299", StartTime: int64(pid), StartUnix: bgTestStart, Env: map[string]string{"CLAUDE_CONFIG_DIR": s.dir, "ZELLIJ": "0", "ZELLIJ_PANE_ID": pane, "ZELLIJ_SESSION_NAME": session, "CC_DECK_PROFILE": "viewer", "CC_DECK_PROFILE_COLOR": "#123456", "CC_DECK_PROFILE_ICON": "V"}}
	if via == "attach" {
		p.Args = []string{"attach", bgTestJob}
	} else {
		s.record(claudeRecord{PID: pid, Kind: "interactive", ParkedJobID: bgTestJob, ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)})
	}
	s.src.add(p)
}
func (s *claudeBGScenario) hook(event string) {
	s.t.Helper()
	runHook(strings.NewReader(claudeEvent(event, "job-session")), "19", "claude")
}
func (s *claudeBGScenario) payload(i int) agent.NormalizedPayload {
	s.t.Helper()
	var p agent.NormalizedPayload
	if err := json.Unmarshal(s.sent[i], &p); err != nil {
		s.t.Fatal(err)
	}
	return p
}

func TestClaudeExternalRecordContract(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "parked", "7", "S")
	// Independent producer-schema fixtures: never marshal claudeRecord here.
	fixtures := map[int]string{
		100: `{"pid":100,"sessionId":"job-session","kind":"bg","jobId":"2402e991","procStart":"Sun Oct 18 07:12:37 2026"}`,
		400: `{"pid":400,"sessionId":"foreground-session","kind":"interactive","parkedJobId":"2402e991","procStart":"Sun Oct 18 07:12:37 2026"}`,
	}
	origLocal := time.Local
	time.Local = time.FixedZone("non-UTC producer host", 2*60*60)
	t.Cleanup(func() { time.Local = origLocal })
	for pid, literal := range fixtures {
		if err := os.WriteFile(filepath.Join(s.dir, "sessions", fmt.Sprint(pid)+".json"), []byte(literal), 0600); err != nil {
			t.Fatal(err)
		}
		r, err := readClaudeRecord(filepath.Join(s.dir, "sessions"), pid)
		if err != nil {
			t.Fatal(err)
		}
		if r.PID != pid || r.ProcStart != "Sun Oct 18 07:12:37 2026" || !verifyRecord(r, s.src.procs[pid]) {
			t.Fatalf("external record identity/start mismatch: %+v", r)
		}
		if pid == 100 && (r.Kind != "bg" || r.SessionID != "job-session" || r.JobID != bgTestJob || r.ParkedJobID != "") {
			t.Fatalf("worker schema mismatch: %+v", r)
		}
		if pid == 400 && (r.Kind != "interactive" || r.SessionID != "foreground-session" || r.ParkedJobID != bgTestJob || r.JobID != "") {
			t.Fatalf("viewer schema mismatch: %+v", r)
		}
	}
	s.hook("Notification")
	if len(s.sent) != 1 || s.payload(0).PaneID != 7 || !s.payload(0).Background || s.targets[0] != "S" {
		t.Fatalf("literal record attribution failed: %v", s.targets)
	}
}

func TestClaudeRecords(t *testing.T) {
	for _, tc := range []struct {
		name, kind, job, parked, start string
		pid                            int
		valid                          bool
		wantErr                        bool
	}{
		{"interactive", "interactive", "", "", "Sun Oct 18 07:12:37 2026", 42, true, false},
		{"worker", "bg", bgTestJob, "", "Sun Oct 18 07:12:37 2026", 42, true, false},
		{"parked", "interactive", "", bgTestJob, "Sun Oct 18 07:12:37 2026", 42, true, false},
		{"single-digit day", "interactive", "", "", "Sun Oct  4 07:12:37 2026", 42, true, false},
		{"missing start", "interactive", "", "", "", 42, false, false},
		{"mismatched pid", "interactive", "", "", "Sun Oct 18 07:12:37 2026", 99, false, true},
		{"bad job", "bg", "../oops", "", "Sun Oct 18 07:12:37 2026", 42, false, true},
		{"bad parked job", "interactive", "", "UPPERHEX", "Sun Oct 18 07:12:37 2026", 42, false, true},
		{"unknown kind", "future", "", "", "Sun Oct 18 07:12:37 2026", 42, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r := claudeRecord{PID: tc.pid, Kind: tc.kind, JobID: tc.job, ParkedJobID: tc.parked, ProcStart: tc.start}
			b, _ := json.Marshal(r)
			if err := os.WriteFile(filepath.Join(dir, "42.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := readClaudeRecord(dir, 42)
			if (err != nil) != tc.wantErr {
				t.Fatalf("read error = %v", err)
			}
			start, _ := parseProcStart(tc.start)
			if valid := verifyRecord(got, procinfo.Process{PID: 42, StartUnix: start}); valid != tc.valid {
				t.Fatalf("verified = %v, want %v", valid, tc.valid)
			}
		})
	}
	r := claudeRecord{ProcStart: "Sun Oct 18 07:12:37 2026"}
	start, _ := parseProcStart(r.ProcStart)
	for _, delta := range []int64{-5, -1, 0, 1, 5} {
		if got := verifyRecord(r, procinfo.Process{StartUnix: start + delta}); got != (delta >= -1 && delta <= 1) {
			t.Errorf("delta %d verified=%v", delta, got)
		}
	}
	if verifyRecord(r, procinfo.Process{}) {
		t.Fatal("unknown process start verified")
	}
}

func TestClaudeHostDetection(t *testing.T) {
	for _, name := range []string{"worker", "nested foreground", "stale foreground", "host unknown", "unhinted error", "foreign ancestor", "missing worker", "pty spelling", "deep helper", "depth eight"} {
		t.Run(name, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			want := claudeBackground
			switch name {
			case "nested foreground", "stale foreground":
				p := s.src.procs[200]
				p.StartUnix = bgTestStart
				p.StartTime = 11
				s.src.add(p)
				start := bgTestStart
				if name == "stale foreground" {
					start -= 5
				} else {
					want = claudeForeground
				}
				s.record(claudeRecord{PID: 200, Kind: "interactive", ProcStart: time.Unix(start, 0).UTC().Format(time.ANSIC)})
			case "host unknown", "unhinted error":
				s.src.ancestorErr[200] = errors.New("inspection denied")
				if name == "unhinted error" {
					t.Setenv("CLAUDE_BG_BACKEND", "")
					want = claudeForeground
				}
			case "foreign ancestor":
				s.src.ancestorErr[200] = procinfo.ErrForeign
				want = claudeForeground
			case "missing worker":
				if err := os.Remove(filepath.Join(s.dir, "sessions/100.json")); err != nil {
					t.Fatal(err)
				}
			case "pty spelling":
				p := s.src.procs[90]
				p.Exe = "/Applications/ClaudeCode.app/Contents/MacOS/claude"
				p.Args = []string{"--bg-pty-host", "socket"}
				s.src.add(p)
				if err := os.Remove(filepath.Join(s.dir, "sessions/100.json")); err != nil {
					t.Fatal(err)
				}
			case "deep helper", "depth eight":
				count := 3
				if name == "depth eight" {
					count = 7
				}
				parent := 100
				for i := 0; i < count; i++ {
					pid := 210 + i
					s.src.add(procinfo.Process{PID: pid, PPID: parent, Exe: "sh"})
					parent = pid
				}
				p := s.src.procs[300]
				p.PPID = parent
				s.src.add(p)
			}
			host, rec, _, detail := claudeHostOf(s.src, filepath.Join(s.dir, "sessions"), os.Getenv("CLAUDE_BG_BACKEND") != "")
			if host != want {
				t.Fatalf("host %v want %v", host, want)
			}
			if name == "host unknown" && detail != "host-unknown" {
				t.Errorf("detail=%q", detail)
			}
			if name == "worker" && rec.JobID != bgTestJob {
				t.Errorf("worker=%+v", rec)
			}
		})
	}
}

func TestClaudeBackgroundParkedDelivery(t *testing.T) {
	for _, inherited := range []bool{true, false} {
		t.Run(fmt.Sprint(inherited), func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "parked", "7", "viewer-session")
			if !inherited {
				s.outsideZellij()
				p := s.src.procs[100]
				p.Env = map[string]string{"CLAUDE_CONFIG_DIR": s.dir}
				s.src.add(p)
			}
			s.writeMap(map[string]paneMapEntry{"sentinel": {PaneID: 19, ZellijSession: "inherited-session", UpdatedAt: time.Now().Unix()}})
			before, _ := os.ReadFile(paneMapFile)
			s.hook("PostToolUse")
			after, _ := os.ReadFile(paneMapFile)
			if string(before) != string(after) {
				t.Fatal("background changed pane-map")
			}
			if len(s.sent) != 1 {
				t.Fatalf("sent %d", len(s.sent))
			}
			p := s.payload(0)
			if !p.Background || p.PaneID != 7 || s.targets[0] != "viewer-session" || p.Profile != "viewer" || p.ProfileColor != "#123456" || p.ProfileIcon != "V" {
				t.Fatalf("delivery=%+v target=%v", p, s.targets)
			}
		})
	}
}

func TestClaudeBackgroundResolutionSafety(t *testing.T) {
	for _, tc := range []struct{ name, reason string }{
		{"headless", reasonHeadless}, {"outside", reasonViewerNotInZellij}, {"several", reasonSeveralViewers}, {"two outside", reasonSeveralViewers}, {"same terminal", reasonParkedViewer}, {"invalid session", reasonViewerNotInZellij}, {"slash session", reasonViewerNotInZellij}, {"stale viewer", reasonHeadless}, {"missing job", reasonRecordMissing}, {"malformed worker", reasonRecordMissing}, {"unreadable worker", reasonRecordMissing}, {"unreadable viewer", reasonRecordMissing}, {"listing failure", reasonRecordMissing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			logs := enableHookDebug(t)
			switch tc.name {
			case "outside", "invalid session", "slash session", "stale viewer", "unreadable viewer":
				s.viewer(400, "parked", "7", "S2")
				p := s.src.procs[400]
				switch tc.name {
				case "outside":
					delete(p.Env, "ZELLIJ")
				case "invalid session":
					p.Env["ZELLIJ_SESSION_NAME"] = "-bad"
				case "slash session":
					p.Env["ZELLIJ_SESSION_NAME"] = "../bad"
				case "stale viewer":
					p.StartUnix += 5
					p.Exe = "unrelated"
				case "unreadable viewer":
					s.src.getErr[400] = errors.New("permission denied")
				}
				s.src.add(p)
			case "several", "same terminal", "two outside":
				s.viewer(400, "parked", "7", "S2")
				pane := "8"
				if tc.name == "same terminal" {
					pane = "7"
				}
				s.viewer(401, "attach", pane, "S2")
				if tc.name == "two outside" {
					for _, pid := range []int{400, 401} {
						p := s.src.procs[pid]
						delete(p.Env, "ZELLIJ")
						s.src.add(p)
					}
				}
			case "missing job":
				s.record(claudeRecord{PID: 100, Kind: "bg", ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)})
			case "malformed worker":
				if err := os.WriteFile(filepath.Join(s.dir, "sessions/100.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable worker":
				s.src.getErr[100] = errors.New("permission denied")
				s.src.ancestorErr[100] = nil
			case "listing failure":
				s.src.listErr = errors.New("listing denied")
			}
			s.hook("PostToolUse")
			lines := logs()
			if len(lines) != 1 || !strings.Contains(lines[0], "resolution="+tc.reason+" ") {
				t.Fatalf("logs=%v want %s", lines, tc.reason)
			}
			want := 0
			if tc.reason == reasonParkedViewer {
				want = 1
			}
			if len(s.sent) != want {
				t.Fatalf("sent=%d want %d", len(s.sent), want)
			}
		})
	}
}

func TestClaudeAttachForms(t *testing.T) {
	for _, form := range []string{"native", "full session", "npm", "other job", "unreadable", "exited", "reused"} {
		t.Run(form, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "attach", "9", "S2")
			p := s.src.procs[400]
			want := 1
			switch form {
			case "full session":
				p.Args = []string{"attach", "job-session"}
			case "npm":
				p.Exe = "node"
				p.Comm = "node"
				p.Args = []string{"/usr/lib/node_modules/@anthropic-ai/claude-code/cli.js", "attach", bgTestJob}
			case "other job":
				p.Args = []string{"attach", "deadbeef"}
				want = 0
			case "unreadable":
				s.src.getErr[400] = errors.New("denied")
				want = 0
			case "exited":
				s.src.getErr[400] = os.ErrNotExist
				want = 0
			case "reused":
				s.src.listed = []procinfo.Process{p}
				p.StartTime++
				want = 0
			}
			s.src.add(p)
			// Script the listing snapshot independently to reproduce exit / pid reuse.
			if form == "reused" {
				s.src.add(p)
				s.src.ancestorErr[400] = nil
			}
			s.hook("PostToolUse")
			if len(s.sent) != want {
				t.Fatalf("sent=%d want %d", len(s.sent), want)
			}
			if want == 1 {
				got := s.payload(0)
				if got.PaneID != 9 || s.targets[0] != "S2" {
					t.Fatalf("got=%+v targets=%v", got, s.targets)
				}
			}
		})
	}
}

func TestClaudeViewerEnvAndProfile(t *testing.T) {
	for _, name := range []string{"valid", "missing record", "invalid pane", "invalid session", "invalid profile"} {
		t.Run(name, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			logs := enableHookDebug(t)
			s.viewer(400, "parked", "7", "S2")
			env := map[string]string{"ZELLIJ_PANE_ID": "11", "ZELLIJ_SESSION_NAME": "S3", "CC_DECK_PROFILE": "explicit", "CC_DECK_PROFILE_COLOR": "#aabbcc"}
			wantPane := uint32(11)
			switch name {
			case "missing record":
				if err := os.Remove(filepath.Join(s.dir, "sessions/100.json")); err != nil {
					t.Fatal(err)
				}
			case "invalid pane":
				env["ZELLIJ_PANE_ID"] = "-1"
				wantPane = 7
			case "invalid session":
				env["ZELLIJ_SESSION_NAME"] = "/bad"
				wantPane = 7
			case "invalid profile":
				env["CC_DECK_PROFILE"] = "bad\nname"
			}
			b, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": "job-session", "viewer_env": env})
			runHook(strings.NewReader(string(b)), "19", "claude")
			if len(s.sent) != 1 {
				t.Fatalf("sent=%d", len(s.sent))
			}
			p := s.payload(0)
			if p.PaneID != wantPane {
				t.Fatalf("pane=%d", p.PaneID)
			}
			if name == "invalid profile" && p.Profile != "" {
				t.Fatalf("unsafe profile=%q", p.Profile)
			}
			wantReason := reasonViewerEnv
			if wantPane == 7 {
				wantReason = reasonParkedViewer
			}
			if len(logs()) != 1 || !strings.Contains(logs()[0], "resolution="+wantReason+" ") {
				t.Fatalf("logs=%v", logs())
			}
		})
	}
}

func TestClaudeBackgroundReleaseTransitions(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "7", "S2")
	s.hook("PostToolUse")
	p := s.src.procs[400]
	p.Env["ZELLIJ_PANE_ID"] = "8"
	p.Env["ZELLIJ_SESSION_NAME"] = "S3"
	s.src.add(p)
	s.hook("PostToolUse")
	if !reflect.DeepEqual(s.pipes, []string{"cc-deck:hook", "cc-deck:release-row", "cc-deck:hook"}) || !reflect.DeepEqual(s.targets, []string{"S2", "S2", "S3"}) {
		t.Fatalf("pipes=%v targets=%v", s.pipes, s.targets)
	}
	var release struct {
		PaneID    uint32 `json:"pane_id"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(s.sent[1], &release); err != nil {
		t.Fatal(err)
	}
	if release.PaneID != 7 || release.SessionID != "job-session" {
		t.Fatalf("release=%+v", release)
	}
	s.hook("PostToolUse")
	if len(s.sent) != 4 {
		t.Fatal("repeated release")
	}
	delete(s.src.procs, 400)
	s.hook("PostToolUse")
	if len(s.sent) != 5 || s.pipes[4] != "cc-deck:release-row" {
		t.Fatalf("headless pipes=%v", s.pipes)
	}
	s.hook("PostToolUse")
	if len(s.sent) != 5 || len(loadBGJobPanes()) != 0 {
		t.Fatal("headless release repeated or entry retained")
	}
}

func TestClaudeBackgroundSessionEnd(t *testing.T) {
	for _, delivered := range []bool{true, false} {
		t.Run(fmt.Sprint(delivered), func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "attach", "7", "S2")
			s.hook("PostToolUse")
			s.writeMap(map[string]paneMapEntry{"job-session": {PaneID: 19, ZellijSession: "old", UpdatedAt: time.Now().Unix()}})
			if !delivered {
				delete(s.src.procs, 400)
			}
			s.hook("SessionEnd")
			if len(s.readMap()) != 0 || len(loadBGJobPanes()) != 0 {
				t.Fatalf("SessionEnd retained mapping")
			}
		})
	}
}

func TestClaudeJobPaneTransition(t *testing.T) {
	now := time.Unix(bgTestStart, 0)
	cur := &bgJobPane{PaneID: 7, ZellijSession: "S", SessionID: "sid", UpdatedAt: now.Unix()}
	for _, tc := range []struct {
		name                    string
		cur                     *bgJobPane
		att                     bgAttribution
		event                   string
		release, write, hasNext bool
	}{
		{"new", nil, bgAttribution{Reason: reasonAttachViewer, PaneID: 7, ZellijSession: "S"}, "Stop", false, true, true},
		{"fresh", cur, bgAttribution{Reason: reasonParkedViewer, PaneID: 7, ZellijSession: "S"}, "Stop", false, false, true},
		{"moved", cur, bgAttribution{Reason: reasonAttachViewer, PaneID: 8, ZellijSession: "S"}, "Stop", true, true, true},
		{"session moved", cur, bgAttribution{Reason: reasonAttachViewer, PaneID: 7, ZellijSession: "T"}, "Stop", true, true, true},
		{"headless", cur, bgAttribution{Reason: reasonHeadless}, "Stop", true, true, false},
		{"outside", cur, bgAttribution{Reason: reasonViewerNotInZellij}, "Stop", true, true, false},
		{"ambiguous present", cur, bgAttribution{Reason: reasonSeveralViewers, Terminals: []terminalKey{{"S", 7}, {"S", 8}}}, "Stop", false, false, true},
		{"ambiguous absent", cur, bgAttribution{Reason: reasonSeveralViewers, Terminals: []terminalKey{{"S", 8}, {"S", 9}}}, "Stop", true, true, false},
		{"unknown", cur, bgAttribution{Reason: reasonRecordMissing}, "Stop", false, false, true},
		{"end", cur, bgAttribution{Reason: reasonAttachViewer, PaneID: 7, ZellijSession: "S"}, "SessionEnd", false, true, false},
		{"end unrecorded", nil, bgAttribution{Reason: reasonAttachViewer, PaneID: 7, ZellijSession: "S"}, "SessionEnd", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, n, w := jobPaneTransition(tc.cur, tc.att, "sid", tc.event, now)
			if (r != nil) != tc.release || w != tc.write || (n != nil) != tc.hasNext {
				t.Fatalf("release=%+v next=%+v write=%v", r, n, w)
			}
		})
	}
	old := *cur
	old.UpdatedAt = now.Add(-2 * time.Hour).Unix()
	_, next, write := jobPaneTransition(&old, bgAttribution{Reason: reasonAttachViewer, PaneID: 7, ZellijSession: "S"}, "sid", "Stop", now)
	if !write || next.UpdatedAt != now.Unix() {
		t.Fatal("did not refresh")
	}
}

func TestClaudeBackgroundOvernightRelease(t *testing.T) {
	for _, moved := range []bool{false, true} {
		t.Run(fmt.Sprint(moved), func(t *testing.T) {
			s := newClaudeBGScenario(t)
			key := s.dir + "|" + bgTestJob
			saveStateFileAtomic(bgJobPaneFile, map[string]bgJobPane{key: {PaneID: 7, ZellijSession: "old", SessionID: "sid", UpdatedAt: time.Now().Add(-13 * time.Hour).Unix()}})
			if moved {
				s.viewer(400, "attach", "8", "new")
			}
			s.hook("PostToolUse")
			if len(s.sent) == 0 || s.pipes[0] != "cc-deck:release-row" || s.targets[0] != "old" || s.payload(0).PaneID != 7 || s.payload(0).SessionID != "sid" {
				t.Fatalf("overnight release missing: pipes=%v targets=%v", s.pipes, s.targets)
			}
		})
	}
}

func TestClaudeBackgroundLockFailureNeverCreatesUntrackedRow(t *testing.T) {
	for _, jobLock := range []bool{false, true} {
		t.Run(fmt.Sprint(jobLock), func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "attach", "7", "S")
			logs := enableHookDebug(t)
			path := filepath.Join(hookStateDir, "bg-job-panes.lock")
			if jobLock {
				path = bgJobLockPath(bgWorker{configDir: s.dir, jobID: bgTestJob})
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
				t.Fatal(err)
			}
			s.hook("PostToolUse")
			if len(s.sent) != 0 || len(loadBGJobPanes()) != 0 || !strings.Contains(strings.Join(logs(), "\n"), "state_error=") {
				t.Fatalf("lock timeout created row or missed diagnostic: %v %v", s.pipes, logs())
			}
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
				t.Fatal(err)
			}
			s.hook("PostToolUse")
			if len(s.sent) != 1 || len(loadBGJobPanes()) != 1 {
				t.Fatal("next event did not recover")
			}
		})
	}
}

func TestClaudeBackgroundReleaseFailureRetainsBothOwners(t *testing.T) {
	s := newClaudeBGScenario(t)
	logs := enableHookDebug(t)
	s.viewer(400, "attach", "7", "old")
	s.hook("PostToolUse")
	baseSend := sendHookPayload
	failed := true
	sendHookPayload = func(ctx context.Context, path, target, pipe string, payload []byte) error {
		if pipe == "cc-deck:release-row" && target == "old" && failed {
			return errors.New("simulated timeout")
		}
		return baseSend(ctx, path, target, pipe, payload)
	}
	p := s.src.procs[400]
	p.Env["ZELLIJ_PANE_ID"], p.Env["ZELLIJ_SESSION_NAME"] = "8", "new"
	s.src.add(p)
	s.hook("PostToolUse")
	entry := loadBGJobPanes()[s.dir+"|"+bgTestJob]
	if entry.PaneID != 8 || len(entry.Pending) != 1 || entry.Pending[0].PaneID != 7 || len(s.sent) != 2 || s.targets[1] != "new" {
		t.Fatalf("lost owner or failed to deliver new row: %+v %v", entry, s.targets)
	}
	if !strings.Contains(strings.Join(logs(), "\n"), "release_error=") || strings.Contains(strings.Join(logs(), "\n"), "release_sid=") {
		t.Fatalf("failed release logged as successful: %v", logs())
	}
	// Detachment must retry old cleanup AND release the new row, even after
	// another hook process has reloaded the persisted ledger.
	failed = false
	entry.Pending[0].RetryAfter = 0 // Advance the persisted retry deadline without sleeping.
	saveStateFileAtomic(bgJobPaneFile, map[string]bgJobPane{s.dir + "|" + bgTestJob: entry})
	delete(s.src.procs, 400)
	s.hook("PostToolUse")
	if !reflect.DeepEqual(s.targets, []string{"old", "new", "old", "new"}) || len(loadBGJobPanes()) != 0 {
		t.Fatalf("cleanup lost: targets=%v ledger=%v", s.targets, loadBGJobPanes())
	}
	s.hook("PostToolUse")
	if len(s.sent) != 4 {
		t.Fatal("successful release repeated")
	}
}

func TestClaudeBackgroundPipeDoesNotHoldGlobalLedgerLock(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "7", "S")
	baseSend := sendHookPayload
	sendHookPayload = func(ctx context.Context, path, target, pipe string, payload []byte) error {
		// A second job can commit while this one's pipe command is running.
		other := &bgJobPane{PaneID: 9, ZellijSession: "other", SessionID: "other-job"}
		if err := storeBGJobPane("other-config|12345678", other); err != nil {
			t.Fatalf("pipe I/O held global lock: %v", err)
		}
		return baseSend(ctx, path, target, pipe, payload)
	}
	s.hook("PostToolUse")
	if len(loadBGJobPanes()) != 2 {
		t.Fatal("independent job ledger update lost")
	}
}

type lockedClaudeViewerSource struct {
	procinfo.Source
	t    *testing.T
	path string
}

func (s lockedClaudeViewerSource) ListUserMatching(match func(string) bool) ([]procinfo.Process, error) {
	f, err := os.OpenFile(s.path, os.O_RDWR, 0600)
	if err != nil {
		s.t.Fatalf("job lock must exist before viewer lookup: %v", err)
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		s.t.Fatal("viewer snapshot was captured outside job serialization")
	}
	if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
		s.t.Fatal(err)
	}
	return s.Source.ListUserMatching(match)
}

func TestClaudeBackgroundResolvesInsideJobLock(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "7", "S")
	hookProcSource = lockedClaudeViewerSource{Source: s.src, t: t, path: bgJobLockPath(bgWorker{configDir: s.dir, jobID: bgTestJob})}
	s.hook("PostToolUse")
	if len(s.sent) != 1 {
		t.Fatal("viewer lookup failed")
	}
}

func TestClaudeBackgroundPendingCleanupRevalidatesViewers(t *testing.T) {
	for _, reason := range []string{reasonSeveralViewers, reasonRecordMissing, reasonAttachViewer} {
		t.Run(reason, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "attach", "8", "S")
			key := s.dir + "|" + bgTestJob
			entry := bgJobPane{PaneID: 8, ZellijSession: "S", SessionID: "job-session", UpdatedAt: time.Now().Unix(), Pending: []bgRowRelease{{PaneID: 7, ZellijSession: "S", SessionID: "job-session"}}}
			saveStateFileAtomic(bgJobPaneFile, map[string]bgJobPane{key: entry})
			switch reason {
			case reasonSeveralViewers:
				s.viewer(401, "attach", "7", "S")
			case reasonRecordMissing:
				s.src.listErr = errors.New("inspection denied")
			case reasonAttachViewer:
				p := s.src.procs[400]
				p.Env["ZELLIJ_PANE_ID"] = "7"
				s.src.add(p)
			}
			s.hook("Notification")
			if reason == reasonAttachViewer {
				if !reflect.DeepEqual(s.pipes, []string{"cc-deck:release-row", "cc-deck:hook"}) || s.payload(0).PaneID != 8 || s.payload(1).PaneID != 7 || len(loadBGJobPanes()[key].Pending) != 0 {
					t.Fatalf("returned owner was released: %v", s.pipes)
				}
				return
			}
			if len(s.sent) != 0 || len(loadBGJobPanes()[key].Pending) != 1 {
				t.Fatalf("uncertain/present pending viewer released: %v", s.pipes)
			}
			delete(s.src.procs, 401)
			s.src.listErr = nil
			s.hook("Notification")
			if !reflect.DeepEqual(s.pipes, []string{"cc-deck:release-row", "cc-deck:hook"}) || s.payload(0).PaneID != 7 || len(loadBGJobPanes()[key].Pending) != 0 {
				t.Fatalf("verified absent pending viewer not released: %v", s.pipes)
			}
		})
	}
}

func TestClaudeBackgroundCleanupBudgetAndBackoff(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "8", "healthy")
	key := s.dir + "|" + bgTestJob
	entry := bgJobPane{PaneID: 8, ZellijSession: "healthy", SessionID: "job-session", UpdatedAt: time.Now().Unix()}
	for _, pane := range []uint32{1, 2, 3} {
		entry.Pending = append(entry.Pending, bgRowRelease{PaneID: pane, ZellijSession: "unavailable", SessionID: "job-session"})
	}
	saveStateFileAtomic(bgJobPaneFile, map[string]bgJobPane{key: entry})
	baseSend := sendHookPayload
	attempts := 0
	sendHookPayload = func(ctx context.Context, path, target, pipe string, payload []byte) error {
		if pipe == "cc-deck:release-row" {
			attempts++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > bgCleanupBudget {
				t.Fatal("cleanup does not have a short aggregate deadline")
			}
			<-ctx.Done()
			return ctx.Err()
		}
		return baseSend(ctx, path, target, pipe, payload)
	}
	start := time.Now()
	s.hook("PostToolUse")
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond || attempts != 1 || len(s.sent) != 1 || s.targets[0] != "healthy" {
		t.Fatalf("cleanup blocked healthy delivery: elapsed=%v attempts=%d sent=%v", elapsed, attempts, s.targets)
	}
	entry = loadBGJobPanes()[key]
	if len(entry.Pending) != 3 || entry.Pending[0].Attempts != 1 || entry.Pending[0].RetryAfter <= time.Now().UnixMilli() {
		t.Fatalf("missing durable retry/backoff: %+v", entry.Pending)
	}
	// With only the failed destination left, an immediate hook skips its retry.
	entry.Pending = entry.Pending[:1]
	saveStateFileAtomic(bgJobPaneFile, map[string]bgJobPane{key: entry})
	s.hook("PostToolUse")
	if attempts != 1 || len(s.sent) != 2 {
		t.Fatalf("backoff not honored: attempts=%d sent=%v", attempts, s.targets)
	}
}

func TestClaudeBackgroundStateWriteFailureDropsDelivery(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "7", "S")
	// A directory at the JSON path makes the atomic rename fail even as root.
	if err := os.Mkdir(bgJobPaneFile, 0700); err != nil {
		t.Fatal(err)
	}
	s.hook("PostToolUse")
	if len(s.sent) != 0 {
		t.Fatal("created a row without a durable cleanup record")
	}
}

func TestClaudeConfigIsolation(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "parked", "7", "S2")
	s.viewer(401, "attach", "8", "S2")
	other := resolvePath(t.TempDir(), "")
	if err := os.Symlink(filepath.Join(s.dir, "sessions"), filepath.Join(other, "sessions")); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{400, 401} {
		p := s.src.procs[pid]
		p.Env["CLAUDE_CONFIG_DIR"] = other
		s.src.add(p)
	}
	s.hook("PostToolUse")
	if len(s.sent) != 0 {
		t.Fatal("foreign profile viewer received event")
	}
}

func TestClaudeForegroundCompatibility(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.record(claudeRecord{PID: 100, Kind: "interactive", ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)})
	logs := enableHookDebug(t)
	s.hook("PostToolUse")
	if len(s.sent) != 1 || s.targets[0] != "" || s.payload(0).Background || s.payload(0).PaneID != 19 {
		t.Fatalf("foreground changed: %v", s.sent)
	}
	if strings.Contains(logs()[0], "background=") {
		t.Fatalf("foreground log changed: %v", logs())
	}
}

func TestClaudeForegroundExistingHookCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"outside", TestRunHook_OutsideZellijSendsNothingDespiteCachedPane},
		{"pane argument", TestRunHook_InsideZellijStampsSessionAndPane},
		{"foreign cache", TestRunHook_CacheFromDifferentZellijSessionIsNotReused},
		{"same cache", TestRunHook_CacheFromSameZellijSessionIsReused},
		{"session end", TestRunHook_SessionEndRemovesCacheEntry},
		{"unchanged cache", TestRunHook_UnchangedPaneMappingDoesNotRewriteCache},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.record(claudeRecord{PID: 100, Kind: "interactive", ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)})
			tc.run(t)
		})
	}
}

func TestClaudeMalformedAndStaleRecordsDrop(t *testing.T) {
	for _, name := range []string{"stale worker", "missing procStart", "malformed viewer", "unreadable directory"} {
		t.Run(name, func(t *testing.T) {
			s := newClaudeBGScenario(t)
			s.viewer(400, "parked", "7", "S")
			logs := enableHookDebug(t)
			switch name {
			case "stale worker":
				p := s.src.procs[100]
				p.StartUnix += 5
				s.src.add(p)
			case "missing procStart":
				s.record(claudeRecord{PID: 100, Kind: "bg", JobID: bgTestJob})
			case "malformed viewer":
				if err := os.WriteFile(filepath.Join(s.dir, "sessions/400.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable directory":
				p := s.src.procs[100]
				p.Env["CLAUDE_CONFIG_DIR"] = filepath.Join(s.dir, "absent")
				s.src.add(p)
			}
			s.hook("PostToolUse")
			if len(s.sent) != 0 || len(logs()) != 1 || !strings.Contains(logs()[0], "resolution=record-missing") {
				t.Fatalf("sent=%v logs=%v", s.sent, logs())
			}
		})
	}
}

func TestClaudeAmbiguityRecoversAndReleasesOnce(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "attach", "7", "S")
	s.hook("PostToolUse")
	s.viewer(401, "attach", "8", "S")
	s.hook("PostToolUse")
	if len(s.sent) != 1 || len(loadBGJobPanes()) != 1 {
		t.Fatal("remembered viewer among ambiguous terminals must stay")
	}
	p := s.src.procs[400]
	p.Env["ZELLIJ_PANE_ID"] = "9"
	s.src.add(p)
	s.hook("PostToolUse")
	if len(s.sent) != 2 || s.pipes[1] != "cc-deck:release-row" || len(loadBGJobPanes()) != 0 {
		t.Fatal("absent remembered viewer must release")
	}
	s.hook("PostToolUse")
	if len(s.sent) != 2 {
		t.Fatal("ambiguous release repeated")
	}
	delete(s.src.procs, 401)
	s.hook("PostToolUse")
	if len(s.sent) != 3 || s.payload(2).PaneID != 9 {
		t.Fatal("unique viewer did not recover")
	}
}

func TestClaudeBackgroundDebugLog(t *testing.T) {
	s := newClaudeBGScenario(t)
	logs := enableHookDebug(t)
	s.viewer(400, "attach", "7", "S2")
	s.hook("PostToolUse")
	delete(s.src.procs, 400)
	s.hook("PostToolUse")
	lines := logs()
	if len(lines) != 3 || !strings.Contains(lines[2], "release_pane=7") || !strings.Contains(lines[1], "resolution=headless") {
		t.Fatalf("logs=%v", lines)
	}
	logClaudeBackground("Stop", "sid\nforged", bgTestJob, reasonRecordMissing, "inspection denied", "", "", "19")
	lines = logs()
	line := lines[len(lines)-1]
	for _, part := range []string{"event=Stop", "session=sid?forged", "background=true", "job=" + bgTestJob, "resolution=record-missing", `detail="inspection denied"`, `pane= target_session=""`, `CLAUDE_BG_BACKEND="daemon"`} {
		if !strings.Contains(line, part) {
			t.Errorf("missing %q in %s", part, line)
		}
	}
}

func TestClaudeExplicitViewerTracksMoves(t *testing.T) {
	s := newClaudeBGScenario(t)
	for _, pane := range []string{"7", "8"} {
		b, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "session_id": "job-session", "viewer_env": map[string]string{"ZELLIJ_PANE_ID": pane, "ZELLIJ_SESSION_NAME": "S"}})
		runHook(strings.NewReader(string(b)), "19", "claude")
	}
	if !reflect.DeepEqual(s.pipes, []string{"cc-deck:hook", "cc-deck:release-row", "cc-deck:hook"}) {
		t.Fatalf("explicit viewer transitions = %v", s.pipes)
	}
}

func TestClaudeParkedProcessExited(t *testing.T) {
	s := newClaudeBGScenario(t)
	s.viewer(400, "parked", "7", "S")
	s.src.getErr[400] = os.ErrNotExist
	s.hook("PostToolUse")
	if len(s.sent) != 0 {
		t.Fatal("exited parked process received event")
	}
}

func BenchmarkClaudeBackgroundAttribution(b *testing.B) {
	b.Run("ForegroundAncestors", func(b *testing.B) {
		src := procinfo.Default()
		dir := b.TempDir()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			claudeHostOf(src, dir, false)
		}
	})
	b.Run("HostList", func(b *testing.B) {
		src := procinfo.Default()
		if _, err := src.ListUserMatching(isClaudeComm); err != nil {
			b.Skipf("host inspection unavailable: %v", err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := src.ListUserMatching(isClaudeComm); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ThirtyProcessesAndRecords", func(b *testing.B) {
		dir := resolvePath(b.TempDir(), "")
		records := filepath.Join(dir, "sessions")
		if err := os.MkdirAll(records, 0700); err != nil {
			b.Fatal(err)
		}
		src := &fakeProcs{procs: map[int]procinfo.Process{}}
		for i := 0; i < 30; i++ {
			pid := 100 + i
			p := procinfo.Process{PID: pid, PPID: 1, StartTime: int64(pid), StartUnix: bgTestStart, Comm: "claude", Exe: "claude", Env: map[string]string{"CLAUDE_CONFIG_DIR": dir, "ZELLIJ": "0", "ZELLIJ_PANE_ID": "7", "ZELLIJ_SESSION_NAME": "S"}}
			src.add(p)
			r := claudeRecord{PID: pid, Kind: "interactive", ProcStart: time.Unix(bgTestStart, 0).UTC().Format(time.ANSIC)}
			if i == 0 {
				r.Kind = "bg"
				r.JobID = bgTestJob
				r.SessionID = "job-session"
			}
			if i == 1 {
				r.ParkedJobID = bgTestJob
			}
			data, _ := json.Marshal(r)
			if err := os.WriteFile(filepath.Join(records, fmt.Sprint(pid)+".json"), data, 0600); err != nil {
				b.Fatal(err)
			}
		}
		origSource, origFile, origSend := hookProcSource, bgJobPaneFile, sendHookPayload
		hookProcSource = src
		bgJobPaneFile = filepath.Join(dir, "bg-job-panes.json")
		sendHookPayload = func(context.Context, string, string, string, []byte) error { return nil }
		b.Cleanup(func() { hookProcSource = origSource; bgJobPaneFile = origFile; sendHookPayload = origSend })
		r, err := readClaudeRecord(records, 100)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := &agent.NormalizedPayload{Agent: "claude", SessionID: "job-session", HookEvent: "PostToolUse"}
			runClaudeBackground("zellij", nil, p, "19", r, src.procs[100], "")
			if !p.Background || p.PaneID != 7 {
				b.Fatal("benchmark failed to attribute")
			}
		}
	})
}
