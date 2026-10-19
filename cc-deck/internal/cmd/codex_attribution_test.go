package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/procinfo"
)

// fakeProcs is a scripted procinfo.Source.
type fakeProcs struct {
	self    int
	procs   map[int]procinfo.Process
	listErr error
	getErr  map[int]error
	// ancestorErr, when it holds pid, replaces getErr for Ancestor; a nil
	// value means Ancestor succeeds even though Get fails.
	ancestorErr map[int]error
	listCalls   int
	// listed provides a separate snapshot to simulate exit or pid reuse after listing.
	listed []procinfo.Process
}

func (f *fakeProcs) Self() int { return f.self }

func (f *fakeProcs) Get(pid int) (procinfo.Process, error) {
	if err := f.getErr[pid]; err != nil {
		return procinfo.Process{}, err
	}
	p, ok := f.procs[pid]
	if !ok {
		return procinfo.Process{}, fmt.Errorf("no process %d", pid)
	}
	return p, nil
}

// Ancestor scripts the argv-only lookup like Get; ancestorErr overrides it
// for processes whose environment is unreadable but whose arguments are not.
func (f *fakeProcs) Ancestor(pid int) (procinfo.Process, error) {
	if err, ok := f.ancestorErr[pid]; ok {
		if err != nil {
			return procinfo.Process{}, err
		}
		p := f.procs[pid]
		p.Env, p.Cwd = nil, ""
		return p, nil
	}
	return f.Get(pid)
}

func (f *fakeProcs) ListUser() ([]procinfo.Process, error) {
	return f.ListUserMatching(func(comm string) bool { return comm == procinfo.CodexComm })
}

func (f *fakeProcs) ListUserMatching(match func(comm string) bool) ([]procinfo.Process, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listed != nil {
		var out []procinfo.Process
		for _, p := range f.listed {
			comm := p.Comm
			if comm == "" {
				comm = p.Basename()
			}
			if match(comm) {
				out = append(out, p)
			}
		}
		return out, nil
	}
	pids := make([]int, 0, len(f.procs))
	for pid := range f.procs {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	var out []procinfo.Process
	for _, pid := range pids {
		p := f.procs[pid]
		comm := p.Comm
		if comm == "" {
			comm = p.Basename()
		}
		if match(comm) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProcs) add(p procinfo.Process) { f.procs[p.PID] = p }

const (
	hookPID   = 300
	daemonPID = 100
	// inheritedPane is the pane the daemon's environment names; a daemon-
	// hosted event must never be delivered there.
	inheritedPane    = "19"
	inheritedSession = "daemon-session"
)

// codexScenario is a daemon-hosted hook (cc-deck under sh under the
// app-server daemon) with a temp HOME, plus helpers to add clients.
type codexScenario struct {
	*hookTestEnv
	t    *testing.T
	src  *fakeProcs
	home string
	next int
}

func newCodexScenario(t *testing.T, daemon bool) *codexScenario {
	t.Helper()
	env := newHookTestEnv(t)
	home := t.TempDir()
	if r, err := filepath.EvalSymlinks(home); err == nil {
		home = r
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CC_DECK_PROFILE", "")
	t.Setenv("CC_DECK_PROFILE_COLOR", "")
	t.Setenv("CC_DECK_PROFILE_ICON", "")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	src := &fakeProcs{self: hookPID, procs: map[int]procinfo.Process{}, getErr: map[int]error{}}
	parent := 1
	if daemon {
		src.add(procinfo.Process{PID: daemonPID, PPID: 1, StartTime: 1,
			Exe: "/opt/codex/app-server-daemon/bin/codex", Args: []string{"app-server", "--listen", "unix://", "--managed-daemon"},
			Env: map[string]string{"PWD": home, "ZELLIJ_PANE_ID": inheritedPane, "ZELLIJ_SESSION_NAME": inheritedSession, "HOME": home}})
		src.add(procinfo.Process{PID: daemonPID + 1, PPID: 1, StartTime: 2,
			Exe: "/opt/codex/app-server-daemon/bin/codex", Args: []string{"app-server", "daemon", "pid-update-loop"},
			Env: map[string]string{"PWD": home, "ZELLIJ_PANE_ID": inheritedPane, "ZELLIJ_SESSION_NAME": inheritedSession, "HOME": home}})
		parent = daemonPID
	}
	src.add(procinfo.Process{PID: 200, PPID: parent, StartTime: 3, Exe: "/bin/sh", Args: []string{"-c", "cc-deck hook"}})
	src.add(procinfo.Process{PID: hookPID, PPID: 200, StartTime: 4, Exe: "/usr/local/bin/cc-deck", Args: []string{"hook", "--agent", "codex"}})

	orig := hookProcSource
	hookProcSource = src
	t.Cleanup(func() { hookProcSource = orig })

	return &codexScenario{hookTestEnv: env, t: t, src: src, home: home, next: 1000}
}

// dir returns a real, symlink-resolved directory under the scenario home.
func (s *codexScenario) dir(name string) string {
	s.t.Helper()
	d := filepath.Join(s.home, "work", name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		s.t.Fatal(err)
	}
	return d
}

type clientOpt func(*procinfo.Process)

func withArgs(args ...string) clientOpt { return func(p *procinfo.Process) { p.Args = args } }
func withEnv(k, v string) clientOpt {
	return func(p *procinfo.Process) {
		if v == "" {
			delete(p.Env, k)
		} else {
			p.Env[k] = v
		}
	}
}
func outsideZellijClient() clientOpt {
	return func(p *procinfo.Process) {
		delete(p.Env, "ZELLIJ")
		delete(p.Env, "ZELLIJ_PANE_ID")
		delete(p.Env, "ZELLIJ_SESSION_NAME")
	}
}

// client adds an interactive Codex TUI in dir, in the given pane and Zellij
// session, and returns its pid.
func (s *codexScenario) client(dir string, pane int, session string, opts ...clientOpt) int {
	s.t.Helper()
	s.next++
	p := procinfo.Process{
		PID: s.next, PPID: 50, StartTime: int64(5000 + s.next), Exe: "codex",
		Env: map[string]string{
			"PWD":                 dir,
			"HOME":                s.home,
			"ZELLIJ":              "0",
			"ZELLIJ_PANE_ID":      fmt.Sprint(pane),
			"ZELLIJ_SESSION_NAME": session,
		},
	}
	for _, o := range opts {
		o(&p)
	}
	s.src.add(p)
	return p.PID
}

// hook runs `cc-deck hook --agent codex --pane-id <inherited>` from inside
// the daemon's (inherited) Zellij environment.
func (s *codexScenario) hook(event, sessionID, cwd string) {
	s.t.Helper()
	s.inZellij(inheritedSession)
	runHook(strings.NewReader(codexEvent(event, sessionID, cwd)), inheritedPane, "codex")
}

func codexEvent(event, sessionID, cwd string) string {
	data, _ := json.Marshal(map[string]string{
		"hook_event_name": event,
		"session_id":      sessionID,
		"cwd":             cwd,
	})
	return string(data)
}

func (e *hookTestEnv) payloads() []agent.NormalizedPayload {
	e.t.Helper()
	var out []agent.NormalizedPayload
	for _, raw := range e.sent {
		var p agent.NormalizedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			e.t.Fatalf("unmarshal payload: %v", err)
		}
		out = append(out, p)
	}
	return out
}

// expectDelivered asserts exactly n payloads, the last one to pane.
func (s *codexScenario) expectDelivered(n int, pane uint32) agent.NormalizedPayload {
	s.t.Helper()
	got := s.payloads()
	if len(got) != n {
		s.t.Fatalf("payloads: got %d, want %d", len(got), n)
	}
	last := got[n-1]
	if last.PaneID != pane {
		s.t.Fatalf("pane: got %d, want %d", last.PaneID, pane)
	}
	return last
}

func (s *codexScenario) expectNothingSent() {
	s.t.Helper()
	if len(s.sent) != 0 {
		s.t.Fatalf("expected no payload, got %d: %s", len(s.sent), s.sent[0])
	}
}

// expectNoInheritedPane asserts the cache never records the daemon's pane
// (no scenario using it places a client there) nor an entry without a
// client identity.
func (s *codexScenario) expectNoInheritedPane() {
	s.t.Helper()
	for id, e := range loadPaneMap() {
		if fmt.Sprint(e.PaneID) == inheritedPane || e.ClientPID == 0 {
			s.t.Errorf("cache entry %s records pane %d without a client (%+v)", id, e.PaneID, e)
		}
	}
}

// Contract test 1: the inherited pane exists, but the one matching client
// sits elsewhere.
func TestCodexDaemon_DeliversToSingleClientNotInheritedPane(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)

	s.hook("UserPromptSubmit", "sess-1", a)

	s.expectDelivered(1, 25)
	e, ok := loadPaneMap()["sess-1"]
	if !ok || e.PaneID != 25 || e.ZellijSession != inheritedSession || e.ClientPID != pid || e.ClientStart != s.src.procs[pid].StartTime {
		t.Fatalf("mapping: (%+v, %v)", e, ok)
	}
	s.expectNoInheritedPane()
}

// After the first discovery, later events use the cache without listing.
func TestCodexDaemon_CacheHitSkipsDiscovery(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)

	s.hook("UserPromptSubmit", "sess-1", a)
	calls := s.src.listCalls
	s.hook("PostToolUse", "sess-1", a)

	s.expectDelivered(2, 25)
	if s.src.listCalls != calls {
		t.Errorf("cache hit listed processes (%d -> %d calls)", calls, s.src.listCalls)
	}
}

// Contract test 5: the cached pid now belongs to a different process. Only
// the start time tells them apart: the reused process sits in the same pane
// and Zellij session, so the pid-reuse guard alone must reject it.
func TestCodexDaemon_ReusedPidIgnoresCache(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a)

	reused := s.src.procs[pid]
	reused.StartTime += 999
	reused.Env["PWD"] = s.dir("other")
	s.src.procs[pid] = reused

	// A later event of the session is dropped: its client is gone.
	s.hook("PostToolUse", "sess-1", a)
	s.expectDelivered(1, 25)

	// The session is resumed in another client; SessionStart rebinds it.
	s.client(a, 40, inheritedSession)
	calls := s.src.listCalls
	s.hook("SessionStart", "sess-1", a)

	s.expectDelivered(2, 40)
	if s.src.listCalls == calls {
		t.Error("expected discovery to run after the cache was invalidated")
	}
	if e := loadPaneMap()["sess-1"]; e.PaneID != 40 {
		t.Errorf("mapping not replaced: %+v", e)
	}
}

// A session whose client exited is never rebound by an ordinary event, even
// when exactly one other client now matches its directory: its trailing
// events would land in that client's row.
func TestCodexDaemon_StaleMappingNotRebound(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pidA := s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a)
	pidB := s.client(a, 26, inheritedSession)

	delete(s.src.procs, pidA)
	s.hook("PostToolUse", "sess-1", a)
	s.hook("Stop", "sess-1", a)

	s.expectDelivered(1, 25)
	if e := loadPaneMap()["sess-1"]; e.ClientPID == pidB || e.PaneID == 26 {
		t.Errorf("stale session rebound to the other client: %+v", e)
	}
}

// SessionStart (resume in another client) rebinds a live mapping when one
// other client matches uniquely; when the directory is shared, the live
// mapping stays (for example SessionStart after compaction, US2-2).
func TestCodexDaemon_SessionStartRebinds(t *testing.T) {
	t.Run("resumed in another client", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a, b := s.dir("a"), s.dir("b")
		s.client(a, 25, inheritedSession)
		pidB := s.client(b, 26, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)

		s.hook("SessionStart", "sess-1", b)

		s.expectDelivered(2, 26)
		if e := loadPaneMap()["sess-1"]; e.ClientPID != pidB {
			t.Errorf("mapping not moved to the resuming client: %+v", e)
		}
	})
	t.Run("moved to a shared directory forgets the mapping", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a, b := s.dir("a"), s.dir("b")
		s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		s.client(b, 26, inheritedSession)
		s.client(b, 27, inheritedSession)

		s.hook("SessionStart", "sess-1", b) // ambiguous in b
		s.hook("PostToolUse", "sess-1", b)

		s.expectDelivered(1, 25)
		if _, ok := loadPaneMap()["sess-1"]; ok {
			t.Error("mapping of a session that left its client was kept")
		}
	})
	t.Run("moved outside Zellij forgets the mapping", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a, b := s.dir("a"), s.dir("b")
		s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		s.client(b, 0, "", outsideZellijClient())

		s.hook("SessionStart", "sess-1", b)
		s.hook("PostToolUse", "sess-1", b)

		s.expectDelivered(1, 25)
		if _, ok := loadPaneMap()["sess-1"]; ok {
			t.Error("mapping of a session that left its client was kept")
		}
	})
	t.Run("stale mapping and ambiguous discovery drop", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		pidA := s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		delete(s.src.procs, pidA)
		s.client(a, 26, inheritedSession)
		s.client(a, 27, inheritedSession)

		s.hook("SessionStart", "sess-1", a)

		s.expectDelivered(1, 25)
	})
	t.Run("other Codex home in the same directory forgets the mapping", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		pidA := s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		// The client now reports another Codex home: it is not among the
		// matches of this daemon, although the directory is the same.
		p := s.src.procs[pidA]
		p.Env["CODEX_HOME"] = filepath.Join(s.home, "profile-home")
		s.src.procs[pidA] = p

		s.hook("SessionStart", "sess-1", a)

		s.expectDelivered(1, 25)
		if _, ok := loadPaneMap()["sess-1"]; ok {
			t.Error("mapping of a client of another Codex home was kept")
		}
	})
	t.Run("listing error keeps the live mapping", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		pidA := s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		s.src.listErr = errors.New("permission denied")

		s.hook("SessionStart", "sess-1", a)

		s.expectDelivered(2, 25)
		if e := loadPaneMap()["sess-1"]; e.ClientPID != pidA {
			t.Errorf("mapping changed: %+v", e)
		}
	})
	t.Run("shared directory keeps the live mapping", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		pidA := s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		s.client(a, 26, inheritedSession)

		s.hook("SessionStart", "sess-1", a)

		s.expectDelivered(2, 25)
		if e := loadPaneMap()["sess-1"]; e.ClientPID != pidA {
			t.Errorf("mapping changed: %+v", e)
		}
	})
}

// A cached client that exited is not used, and an ordinary event of its
// session does not run discovery.
func TestCodexDaemon_ExitedCachedClientDropsWithoutDiscovery(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a)

	delete(s.src.procs, pid)
	calls := s.src.listCalls
	s.hook("PostToolUse", "sess-1", a)

	if len(s.sent) != 1 {
		t.Fatalf("expected the event after the exit to be dropped, sent %d", len(s.sent))
	}
	if s.src.listCalls != calls {
		t.Errorf("stale mapping ran discovery (%d -> %d calls)", calls, s.src.listCalls)
	}
}

// A cached entry whose Zellij session no longer matches the client's
// environment is not used (FR-009): ordinary events are dropped, and the
// next SessionStart rediscovers the client.
func TestCodexDaemon_CachedEntryWithOtherZellijSessionIgnored(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, "s2")
	s.writeMap(map[string]paneMapEntry{
		"sess-1": {PaneID: 25, ZellijSession: "s1", UpdatedAt: time.Now().Unix(), ClientPID: pid, ClientStart: s.src.procs[pid].StartTime},
	})

	s.hook("PostToolUse", "sess-1", a)
	s.expectNothingSent()

	calls := s.src.listCalls
	s.hook("SessionStart", "sess-1", a)
	if s.src.listCalls == calls {
		t.Error("expected discovery on SessionStart after a session mismatch")
	}
	s.expectDelivered(1, 25)
	if e := loadPaneMap()["sess-1"]; e.ZellijSession != "s2" {
		t.Errorf("mapping not corrected: %+v", e)
	}
}

// Legacy entries without a client identity are never used for daemon-
// hosted events (FR-002).
func TestCodexDaemon_LegacyEntryWithoutClientIgnored(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.writeMap(map[string]paneMapEntry{
		"sess-1": {PaneID: 19, ZellijSession: inheritedSession, UpdatedAt: time.Now().Unix()},
	})

	s.hook("PostToolUse", "sess-1", a) // no client in a: dropped

	s.expectNothingSent()
}

// Inspection errors drop the event; they never fall back to --pane-id.
func TestCodexDaemon_InspectionErrorsDrop(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 25, inheritedSession)
		s.src.listErr = errors.New("permission denied")
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
		s.expectNoInheritedPane()
	})
	t.Run("get cached client", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		pid := s.client(a, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", a)
		s.src.getErr[pid] = errors.New("permission denied")
		s.src.listErr = errors.New("permission denied")
		s.hook("PostToolUse", "sess-1", a)
		if len(s.sent) != 1 {
			t.Fatalf("expected the second event to be dropped, sent %d", len(s.sent))
		}
	})
}

// After daemon-hosted events of every outcome, the cache holds no entry for
// the inherited pane.
func TestCodexDaemon_NeverCachesInheritedPane(t *testing.T) {
	s := newCodexScenario(t, true)
	shared := s.dir("shared")
	s.client(shared, 21, inheritedSession)
	s.client(shared, 22, inheritedSession)
	outside := s.dir("outside")
	s.client(outside, 0, "", outsideZellijClient())
	single := s.dir("single")
	s.client(single, 23, inheritedSession)

	s.hook("UserPromptSubmit", "amb", shared)
	s.hook("UserPromptSubmit", "none", s.dir("empty"))
	s.hook("UserPromptSubmit", "out", outside)
	s.hook("UserPromptSubmit", "ok", single)
	s.hook("SessionEnd", "ok", single)

	if _, ok := loadPaneMap()["ok"]; ok {
		t.Error("SessionEnd did not remove the daemon-hosted mapping")
	}
	s.expectNoInheritedPane()
	for _, p := range s.payloads() {
		if fmt.Sprint(p.PaneID) == inheritedPane {
			t.Errorf("payload delivered to the inherited pane: %+v", p)
		}
	}
}

// The daemon was first started from a plain terminal: the hook has no
// Zellij environment at all, but the client does.
func TestCodexDaemon_HookWithoutZellijEnvStillDelivers(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, "cc-deck-local")
	s.outsideZellij()
	t.Setenv("ZELLIJ_PANE_ID", "")

	runHook(strings.NewReader(codexEvent("UserPromptSubmit", "sess-1", a)), "", "codex")

	s.expectDelivered(1, 25)
	s.expectTarget("cc-deck-local")
}

// Profile enrichment uses the client's profile, not the daemon's (FR-017).
func TestCodexDaemon_ProfileFromClient(t *testing.T) {
	s := newCodexScenario(t, true)
	withProfileConfig(t, hookProfileFixture)
	a := s.dir("a")
	s.client(a, 25, inheritedSession,
		withEnv("CC_DECK_PROFILE", "work"),
		withEnv("CC_DECK_PROFILE_COLOR", "#112233"),
		withEnv("CC_DECK_PROFILE_ICON", "K"))
	t.Setenv("CC_DECK_PROFILE", "daemon-profile")
	t.Setenv("CC_DECK_PROFILE_COLOR", "#AABBCC")
	t.Setenv("CC_DECK_PROFILE_ICON", "D")

	s.hook("UserPromptSubmit", "sess-1", a)
	got := s.expectDelivered(1, 25)
	if got.Profile != "work" || got.ProfileColor != "#112233" || got.ProfileIcon != "K" {
		t.Errorf("profile: got %q %q %q, want the client's work #112233 K", got.Profile, got.ProfileColor, got.ProfileIcon)
	}
	if e := loadPaneMap()["sess-1"]; e.Profile != "work" {
		t.Errorf("mapping profile: got %q", e.Profile)
	}

	// Cache hit: same profile.
	s.hook("PostToolUse", "sess-1", a)
	got = s.expectDelivered(2, 25)
	if got.Profile != "work" || got.ProfileColor != "#112233" {
		t.Errorf("cache-hit profile: got %q %q", got.Profile, got.ProfileColor)
	}
}

// A client without a profile gets none, even when the daemon has one.
func TestCodexDaemon_NoClientProfileIgnoresDaemonProfile(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	t.Setenv("CC_DECK_PROFILE", "daemon-profile")

	s.hook("UserPromptSubmit", "sess-1", a)
	if got := s.expectDelivered(1, 25); got.Profile != "" {
		t.Errorf("profile: got %q, want none", got.Profile)
	}
}

func TestCodexDaemon_ClientDirectoryRules(t *testing.T) {
	type tc struct {
		name string
		opts func(s *codexScenario) []clientOpt
		// clientDir is the PWD of the client; eventDir the payload cwd.
		clientDir, eventDir string
		want                bool
	}
	cases := []tc{
		{name: "-C absolute", clientDir: "shell", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("-C", s.dir("proj"))} }, want: true},
		{name: "--cd relative to PWD", clientDir: "shell", eventDir: "shell/sub",
			opts: func(s *codexScenario) []clientOpt { s.dir("shell/sub"); return []clientOpt{withArgs("--cd", "sub")} }, want: true},
		{name: "--cd= form", clientDir: "shell", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("--cd=" + s.dir("proj"))} }, want: true},
		{name: "-C wins over PWD", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("-C", s.dir("elsewhere"))} }, want: false},
		{name: "resume is a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("resume", "--last")} }, want: true},
		{name: "fork is a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("-m", "o3", "fork")} }, want: true},
		{name: "initial prompt is a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("fix the failing tests")} }, want: true},
		{name: "exec is not a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("exec", "do it")} }, want: false},
		{name: "mcp-server is not a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("mcp-server")} }, want: false},
		{name: "app-server is not a client", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt { return []clientOpt{withArgs("app-server")} }, want: false},
		{name: "other CODEX_HOME excluded", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt {
				return []clientOpt{withEnv("CODEX_HOME", filepath.Join(s.home, "profile-home"))}
			}, want: false},
		{name: "explicit default CODEX_HOME matches", clientDir: "proj", eventDir: "proj",
			opts: func(s *codexScenario) []clientOpt {
				return []clientOpt{withEnv("CODEX_HOME", filepath.Join(s.home, ".codex"))}
			}, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newCodexScenario(t, true)
			s.client(s.dir(c.clientDir), 25, inheritedSession, c.opts(s)...)
			s.hook("UserPromptSubmit", "sess-1", s.dir(c.eventDir))
			if c.want {
				s.expectDelivered(1, 25)
			} else {
				s.expectNothingSent()
			}
		})
	}
}

// /tmp and /private/tmp on macOS, or any symlinked project directory.
func TestCodexDaemon_SymlinkedDirectoriesMatch(t *testing.T) {
	s := newCodexScenario(t, true)
	real := s.dir("real")
	link := filepath.Join(s.home, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	s.client(link, 25, inheritedSession)

	s.hook("UserPromptSubmit", "sess-1", real)

	s.expectDelivered(1, 25)
}

// The ambiguity count includes every Codex process (research R4).
func TestCodexDaemon_AmbiguityCountsEveryCodexProcess(t *testing.T) {
	t.Run("exec next to a TUI is ambiguous", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 25, inheritedSession)
		s.client(a, 26, inheritedSession, withArgs("exec", "run tests"))
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
	})
	t.Run("lone exec is no client", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 26, inheritedSession, withArgs("exec", "run tests"))
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
	})
	t.Run("daemon processes never count", func(t *testing.T) {
		s := newCodexScenario(t, true)
		// The daemon's PWD is the scenario home; put the client there.
		s.client(s.home, 25, inheritedSession)
		s.hook("UserPromptSubmit", "sess-1", s.home)
		s.expectDelivered(1, 25)
	})
	t.Run("plain-terminal client makes it ambiguous", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 25, inheritedSession)
		s.client(a, 0, "", outsideZellijClient())
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
	})
}

// Contract test 6: without an app-server ancestor, Codex keeps today's
// behavior for the existing hook table.
func TestCodexNotDaemonHosted_KeepsTodaysBehavior(t *testing.T) {
	t.Run("from arg and cached", func(t *testing.T) {
		s := newCodexScenario(t, false)
		s.inZellij("cc-deck-local")
		runHook(strings.NewReader(codexEvent("PreToolUse", "sess-a", "/tmp")), "7", "codex")
		s.expectDelivered(1, 7)
		e := loadPaneMap()["sess-a"]
		if e.PaneID != 7 || e.ZellijSession != "cc-deck-local" || e.ClientPID != 0 {
			t.Errorf("mapping: %+v", e)
		}
		runHook(strings.NewReader(codexEvent("PreToolUse", "sess-a", "/tmp")), "", "codex")
		s.expectDelivered(2, 7)
		if s.src.listCalls != 0 {
			t.Errorf("non-daemon hook listed processes")
		}
	})
	t.Run("outside zellij", func(t *testing.T) {
		s := newCodexScenario(t, false)
		s.writeMap(map[string]paneMapEntry{"sess-r": {PaneID: 3, ZellijSession: "cc-deck-local", UpdatedAt: time.Now().Unix()}})
		s.outsideZellij()
		runHook(strings.NewReader(codexEvent("PreToolUse", "sess-r", "/tmp")), "", "codex")
		s.expectNothingSent()
	})
	t.Run("cache from another zellij session", func(t *testing.T) {
		s := newCodexScenario(t, false)
		s.writeMap(map[string]paneMapEntry{"sess-b": {PaneID: 3, ZellijSession: "older", UpdatedAt: time.Now().Unix()}})
		s.inZellij("cc-deck-local")
		runHook(strings.NewReader(codexEvent("PreToolUse", "sess-b", "/tmp")), "", "codex")
		s.expectNothingSent()
	})
	t.Run("session end removes the entry", func(t *testing.T) {
		s := newCodexScenario(t, false)
		s.writeMap(map[string]paneMapEntry{"sess-d": {PaneID: 4, ZellijSession: "cc-deck-local", UpdatedAt: time.Now().Unix()}})
		s.inZellij("cc-deck-local")
		runHook(strings.NewReader(codexEvent("SessionEnd", "sess-d", "/tmp")), "4", "codex")
		if _, ok := loadPaneMap()["sess-d"]; ok {
			t.Error("SessionEnd should remove the cache entry")
		}
	})
	t.Run("hook profile used", func(t *testing.T) {
		s := newCodexScenario(t, false)
		withProfileConfig(t, hookProfileFixture)
		t.Setenv("CC_DECK_PROFILE", "work")
		s.inZellij("cc-deck-local")
		runHook(strings.NewReader(codexEvent("PreToolUse", "sess-p", "/tmp")), "7", "codex")
		if got := s.expectDelivered(1, 7); got.Profile != "work" {
			t.Errorf("profile: got %q", got.Profile)
		}
	})
}

func TestDaemonHosted(t *testing.T) {
	mk := func(chain ...procinfo.Process) *fakeProcs {
		f := &fakeProcs{procs: map[int]procinfo.Process{}, getErr: map[int]error{}}
		for _, p := range chain {
			f.add(p)
		}
		f.self = chain[0].PID
		return f
	}
	appServer := procinfo.Process{PID: 10, PPID: 1, Exe: "/x/codex", Args: []string{"app-server", "--listen", "unix://"}}

	// An IDE extension's own stdio app-server is not the shared daemon.
	if daemonHosted(mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "/x/codex", Args: []string{"app-server"}})) {
		t.Error("a stdio app-server is not the shared daemon")
	}
	for _, args := range [][]string{
		{"app-server", "--listen", "unix:///tmp/s", "--managed-daemon"},
		{"app-server", "--managed-daemon"},
		{"app-server", "--listen=unix://"},
		{"app-server", "daemon", "pid-update-loop"},
	} {
		if !daemonHosted(mk(procinfo.Process{PID: 30, PPID: 10}, procinfo.Process{PID: 10, PPID: 1, Exe: "codex", Args: args})) {
			t.Errorf("%q not recognized as the shared daemon", args)
		}
	}

	if !daemonHosted(mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"}, appServer)) {
		t.Error("grandparent app-server not detected")
	}
	if daemonHosted(mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "codex"})) {
		t.Error("a codex TUI ancestor (no-daemon) is not the daemon")
	}
	if daemonHosted(mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "/x/node", Args: []string{"app-server"}})) {
		t.Error("a non-codex executable with app-server args is not the daemon")
	}
	// The shared daemon as the n-th ancestor: found up to the eighth level,
	// not beyond.
	daemonAt := func(n int) *fakeProcs {
		chain := []procinfo.Process{{PID: 100, PPID: 99}}
		pid := 99
		for i := 1; i < n; i++ {
			chain = append(chain, procinfo.Process{PID: pid, PPID: pid - 1, Exe: "/bin/sh"})
			pid--
		}
		chain = append(chain, procinfo.Process{PID: pid, PPID: 1, Exe: "/x/codex",
			Args: []string{"app-server", "--listen", "unix://", "--managed-daemon"}})
		return mk(chain...)
	}
	if !daemonHosted(daemonAt(daemonAncestorDepthLimit)) {
		t.Error("the shared daemon at the depth limit must be found")
	}
	if daemonHosted(daemonAt(daemonAncestorDepthLimit + 1)) {
		t.Error("ancestor walk must stop after eight levels")
	}
	// A lookup failure ends the walk.
	f := mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"}, appServer)
	f.getErr[20] = errors.New("gone")
	if got, detail := codexHost(f); got != hostUnknown || !strings.Contains(detail, "gone") {
		t.Errorf("lookup failure: got host %v (%q), want hostUnknown with the error", got, detail)
	}
	// An ancestor whose environment is unreadable (non-dumpable) is still
	// classified from its arguments, both ways.
	f = mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "/x/codex", Args: []string{"app-server", "--managed-daemon"}})
	f.getErr[10] = errors.New("environ: permission denied")
	f.ancestorErr = map[int]error{10: nil}
	if got, detail := codexHost(f); got != hostDaemon {
		t.Errorf("non-dumpable daemon: got host %v (%q), want hostDaemon", got, detail)
	}
	f = mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "/x/codex"})
	f.getErr[10] = errors.New("environ: permission denied")
	f.ancestorErr = map[int]error{10: nil}
	if got, detail := codexHost(f); got != hostOwn {
		t.Errorf("non-dumpable TUI (no daemon): got host %v (%q), want hostOwn", got, detail)
	}
	// An ancestor that cannot be classified (status denied for a
	// non-dumpable process under hidepid) drops the event.
	f = mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"},
		procinfo.Process{PID: 10, PPID: 1, Exe: "/x/codex", Args: []string{"app-server", "--managed-daemon"}})
	f.ancestorErr = map[int]error{10: fmt.Errorf("procinfo: status 10: %w", os.ErrPermission)}
	if got, _ := codexHost(f); got != hostUnknown {
		t.Errorf("unreadable ancestor: got host %v, want hostUnknown", got)
	}
	// An ancestor of another user (sshd, sudo) ends the walk: the user's
	// daemon cannot sit above it, so the hook's own identity is trusted.
	f = mk(procinfo.Process{PID: 30, PPID: 20}, procinfo.Process{PID: 20, PPID: 10, Exe: "/bin/sh"}, appServer)
	f.getErr[20] = fmt.Errorf("procinfo: process 20: %w", procinfo.ErrForeign)
	if got, _ := codexHost(f); got != hostOwn {
		t.Errorf("foreign ancestor: got host %v, want hostOwn", got)
	}
}

func TestParseCodexArgs(t *testing.T) {
	cases := []struct {
		args          []string
		first, cd     string
		hasCd, prompt bool
	}{
		{nil, "", "", false, false},
		{[]string{"-C", "/p"}, "", "/p", true, false},
		{[]string{"--cd=/p", "resume"}, "resume", "/p", true, false},
		{[]string{"-C/p"}, "", "/p", true, false},
		{[]string{"-m", "o3", "-c", "k=v", "exec", "x"}, "exec", "", false, false},
		{[]string{"--full-auto", "fix it"}, "fix it", "", false, false},
		{[]string{"--", "-weird prompt"}, "-weird prompt", "", false, true},
		{[]string{"--", "exec"}, "exec", "", false, true},
	}
	for _, c := range cases {
		first, cd, hasCd, prompt := parseCodexArgs(c.args)
		if first != c.first || cd != c.cd || hasCd != c.hasCd || prompt != c.prompt {
			t.Errorf("parseCodexArgs(%q) = (%q, %q, %v, %v), want (%q, %q, %v, %v)", c.args, first, cd, hasCd, prompt, c.first, c.cd, c.hasCd, c.prompt)
		}
	}
}

// US2 acceptance 1: three clients in distinct directories share one daemon.
func TestCodexDaemon_ThreeClientsDistinctDirectories(t *testing.T) {
	s := newCodexScenario(t, true)
	dirs := []string{s.dir("a"), s.dir("b"), s.dir("c")}
	panes := []int{21, 22, 23}
	for i, d := range dirs {
		s.client(d, panes[i], inheritedSession)
	}

	// Interleaved events of the three sessions.
	for round := 0; round < 3; round++ {
		for i, d := range dirs {
			s.hook("PostToolUse", fmt.Sprintf("sess-%d", i), d)
		}
	}

	got := s.payloads()
	if len(got) != 9 {
		t.Fatalf("payloads: got %d, want 9", len(got))
	}
	for _, p := range got {
		var i int
		if _, err := fmt.Sscanf(p.SessionID, "sess-%d", &i); err != nil {
			t.Fatalf("session id %q", p.SessionID)
		}
		if p.PaneID != uint32(panes[i]) {
			t.Errorf("%s delivered to pane %d, want %d", p.SessionID, p.PaneID, panes[i])
		}
	}
}

// US1 acceptance 4: /new in a bound client; every session stays in its pane.
func TestCodexDaemon_OneClientSeveralSessions(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)

	s.hook("UserPromptSubmit", "first", a)
	s.hook("SessionStart", "second", a) // /new
	s.hook("PostToolUse", "first", a)
	s.hook("PostToolUse", "second", a)

	for _, p := range s.payloads() {
		if p.PaneID != 25 {
			t.Errorf("%s delivered to pane %d", p.SessionID, p.PaneID)
		}
	}
	if len(s.sent) != 4 {
		t.Fatalf("payloads: got %d, want 4", len(s.sent))
	}
	m := loadPaneMap()
	for _, id := range []string{"first", "second"} {
		if m[id].ClientPID != pid || m[id].PaneID != 25 {
			t.Errorf("mapping %s: %+v", id, m[id])
		}
	}
}

// Contract test 2 and US2 acceptance 2: two clients in one directory, one
// already bound.
func TestCodexDaemon_SharedDirectoryDropsNewSessions(t *testing.T) {
	s := newCodexScenario(t, true)
	d := s.dir("same")
	s.client(d, 21, inheritedSession)
	s.hook("UserPromptSubmit", "bound", d)
	s.expectDelivered(1, 21)

	second := s.client(d, 22, inheritedSession)
	s.hook("SessionStart", "new", d)
	if len(s.sent) != 1 {
		t.Fatalf("a new session in a shared directory must be dropped, sent %d", len(s.sent))
	}
	if _, ok := loadPaneMap()["new"]; ok {
		t.Error("an ambiguous session must not be cached")
	}

	// The bound session keeps resolving through its cached client.
	calls := s.src.listCalls
	s.hook("PostToolUse", "bound", d)
	s.expectDelivered(2, 21)
	if s.src.listCalls != calls {
		t.Error("the bound session must resolve from the cache, without listing")
	}

	// Attribution resumes once only one matching client remains.
	delete(s.src.procs, second)
	s.hook("PostToolUse", "new", d)
	s.expectDelivered(3, 21)
	if e := loadPaneMap()["new"]; e.PaneID != 21 {
		t.Errorf("mapping after ambiguity ended: %+v", e)
	}
}

// US2 acceptance 3: two unbound clients in one directory.
func TestCodexDaemon_SharedDirectoryNeitherBound(t *testing.T) {
	s := newCodexScenario(t, true)
	d := s.dir("same")
	s.client(d, 21, inheritedSession)
	s.client(d, 22, inheritedSession)
	s.hook("UserPromptSubmit", "x", d)
	s.hook("PostToolUse", "x", d)
	s.expectNothingSent()
}

// Edge case: a client exits and a new one starts in the same pane; the old
// session's mapping is not reused, the new session is attributed.
func TestCodexDaemon_NewClientInSamePane(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	old := s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "old-sess", a)
	delete(s.src.procs, old)

	s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "new-sess", a)
	s.expectDelivered(2, 25)
	if e := loadPaneMap()["new-sess"]; e.ClientPID == old {
		t.Errorf("new session bound to the exited client: %+v", e)
	}
}

// expectTarget asserts the --session argument of the last send.
func (s *codexScenario) expectTarget(want string) {
	s.t.Helper()
	if len(s.targets) == 0 {
		s.t.Fatal("nothing sent")
	}
	if got := s.targets[len(s.targets)-1]; got != want {
		s.t.Fatalf("target session: got %q, want %q", got, want)
	}
}

// Contract test 3 and US3 acceptance 2: the client runs in another Zellij
// session than the one the daemon's identity names.
func TestCodexDaemon_DeliversToClientsZellijSession(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 4, "s2")

	s.hook("UserPromptSubmit", "sess-1", a)
	s.expectDelivered(1, 4)
	s.expectTarget("s2")

	// Cache hits keep the client's session.
	s.hook("PostToolUse", "sess-1", a)
	s.expectDelivered(2, 4)
	s.expectTarget("s2")
}

// Delivery names the session even when it equals the daemon's: the hook's
// ambient session must never be what decides.
func TestCodexDaemon_TargetsSessionExplicitly(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a)
	s.expectTarget(inheritedSession)
}

// Contract test 4 and US3 acceptance 1: the single matching client runs
// outside Zellij.
func TestCodexDaemon_ClientOutsideZellijDropped(t *testing.T) {
	t.Run("daemon inside zellij", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 0, "", outsideZellijClient())
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
	})
	t.Run("daemon outside zellij", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 0, "", outsideZellijClient())
		s.outsideZellij()
		runHook(strings.NewReader(codexEvent("UserPromptSubmit", "sess-1", a)), "", "codex")
		s.expectNothingSent()
	})
	t.Run("pane without session name", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 7, "", withEnv("ZELLIJ_SESSION_NAME", ""))
		s.hook("UserPromptSubmit", "sess-1", a)
		s.expectNothingSent()
	})
}

// Non-daemon events keep today's zellij command: no --session.
func TestRunHook_NonDaemonSendsWithoutSession(t *testing.T) {
	for _, agentName := range []string{"claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			s := newCodexScenario(t, false)
			s.inZellij("cc-deck-local")
			runHook(strings.NewReader(codexEvent("PreToolUse", "sess-a", "/tmp")), "7", agentName)
			s.expectDelivered(1, 7)
			s.expectTarget("")
		})
	}
}

// An ancestor that cannot be inspected leaves the hook unable to tell its own
// pane from an inherited one: the event is dropped, never sent to --pane-id.
func TestCodex_UnknownHostDropsEvent(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	s.src.getErr[200] = errors.New("permission denied")

	s.hook("UserPromptSubmit", "sess-1", a)

	s.expectNothingSent()
	if len(loadPaneMap()) != 0 {
		t.Error("nothing may be cached for an unknown host")
	}
}

// An IDE's stdio app-server in the same directory is a matching, non-
// bindable Codex process: the TUI's new sessions become ambiguous.
func TestCodexDaemon_IDEAppServerCountsForAmbiguity(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	s.client(a, 0, "", outsideZellijClient(), withArgs("app-server"))

	s.hook("UserPromptSubmit", "sess-1", a)

	s.expectNothingSent()
}

// A hook hosted by an IDE's own app-server is not daemon-hosted: today's
// attribution applies (here: outside Zellij, dropped).
func TestCodex_IDEAppServerHookKeepsTodaysPath(t *testing.T) {
	s := newCodexScenario(t, false)
	s.src.add(procinfo.Process{PID: 150, PPID: 1, StartTime: 9, Exe: "/ide/codex", Args: []string{"app-server"},
		Env: map[string]string{"PWD": s.home}})
	sh := s.src.procs[200]
	sh.PPID = 150
	s.src.procs[200] = sh
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	s.outsideZellij()

	runHook(strings.NewReader(codexEvent("UserPromptSubmit", "sess-1", a)), "", "codex")

	s.expectNothingSent()
	if s.src.listCalls != 0 {
		t.Error("an IDE-hosted hook must not run discovery")
	}
}

func codexEventWithClientEnv(event, sessionID, cwd string, clientEnv map[string]string) string {
	data, _ := json.Marshal(map[string]any{
		"hook_event_name": event,
		"session_id":      sessionID,
		"cwd":             cwd,
		"client_env":      clientEnv,
	})
	return string(data)
}

// Contract test 7: client_env wins over the cache and discovery.
func TestCodexDaemon_ClientEnvWins(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a) // discovered and cached: pane 25
	calls := s.src.listCalls
	s.inZellij(inheritedSession)

	runHook(strings.NewReader(codexEventWithClientEnv("PostToolUse", "sess-1", a, map[string]string{
		"ZELLIJ_PANE_ID": "33", "ZELLIJ_SESSION_NAME": "s3",
		"CC_DECK_PROFILE": "work", "CC_DECK_PROFILE_COLOR": "#123456",
	})), inheritedPane, "codex")

	got := s.expectDelivered(2, 33)
	s.expectTarget("s3")
	if got.Profile != "work" || got.ProfileColor != "#123456" {
		t.Errorf("profile: %q %q", got.Profile, got.ProfileColor)
	}
	if s.src.listCalls != calls {
		t.Error("client_env must not run discovery")
	}
	if e := loadPaneMap()["sess-1"]; e.PaneID != 25 {
		t.Errorf("client_env must not rewrite the client mapping: %+v", e)
	}
}

// A daemon-hosted hook may not borrow its own (inherited) session for a
// client_env pane: without ZELLIJ_SESSION_NAME the cache and discovery run.
func TestCodexDaemon_ClientEnvWithoutSessionFallsThrough(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	s.client(a, 25, "client-session")
	s.inZellij(inheritedSession)

	runHook(strings.NewReader(codexEventWithClientEnv("PostToolUse", "sess-1", a, map[string]string{
		"ZELLIJ_PANE_ID": "33",
	})), inheritedPane, "codex")

	s.expectDelivered(1, 25)
	s.expectTarget("client-session")
}

// Without the daemon, a client_env pane may use the hook's own session.
func TestCodexNotDaemonHosted_ClientEnvUsesOwnSession(t *testing.T) {
	s := newCodexScenario(t, false)
	s.inZellij("cc-deck-local")
	t.Setenv("CC_DECK_PROFILE", "")

	runHook(strings.NewReader(codexEventWithClientEnv("PostToolUse", "sess-1", "/tmp", map[string]string{
		"ZELLIJ_PANE_ID": "33",
	})), "7", "codex")

	s.expectDelivered(1, 33)
	s.expectTarget("cc-deck-local")
}

// An unparsable client_env pane is ignored.
func TestCodex_ClientEnvBadPaneIgnored(t *testing.T) {
	s := newCodexScenario(t, false)
	s.inZellij("cc-deck-local")
	runHook(strings.NewReader(codexEventWithClientEnv("PostToolUse", "sess-1", "/tmp", map[string]string{
		"ZELLIJ_PANE_ID": "abc", "ZELLIJ_SESSION_NAME": "x",
	})), "7", "codex")
	s.expectDelivered(1, 7)
	s.expectTarget("")
}

func TestClientEnvTarget(t *testing.T) {
	t.Setenv("CC_DECK_PROFILE", "hook-profile")
	if _, ok := clientEnvTarget(nil, false, "s"); ok {
		t.Error("no client_env must not resolve")
	}
	att, ok := clientEnvTarget(map[string]string{"ZELLIJ_PANE_ID": "4"}, false, "own")
	if !ok || att.PaneID != 4 || att.ZellijSession != "own" || att.Reason != reasonClientEnv || att.Profile.Name != "hook-profile" {
		t.Errorf("non-daemon: (%+v, %v)", att, ok)
	}
	if _, ok := clientEnvTarget(map[string]string{"ZELLIJ_PANE_ID": "4"}, true, "inherited"); ok {
		t.Error("daemon-hosted without a client session must fall through")
	}
	att, ok = clientEnvTarget(map[string]string{"ZELLIJ_PANE_ID": "4", "ZELLIJ_SESSION_NAME": "c"}, true, "inherited")
	if !ok || att.ZellijSession != "c" || att.Profile.Name != "" {
		t.Errorf("daemon-hosted: (%+v, %v); the daemon's profile must not be used", att, ok)
	}
	if _, ok := clientEnvTarget(map[string]string{"ZELLIJ_PANE_ID": "4"}, false, ""); ok {
		t.Error("no session at all must not resolve")
	}
}

// enableHookDebug turns the hook debug log on for this test, in the temp
// state dir, and returns a function reading the log lines.
func enableHookDebug(t *testing.T) func() []string {
	t.Helper()
	dir := t.TempDir()
	origFlag, origLog := hookDebugFlagFile, hookDebugLogFile
	hookDebugFlagFile = filepath.Join(dir, "hook-debug")
	hookDebugLogFile = filepath.Join(dir, "hook.log")
	if err := os.WriteFile(hookDebugFlagFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	hookDebugOnce = sync.Once{}
	t.Cleanup(func() {
		hookDebugFlagFile, hookDebugLogFile = origFlag, origLog
		hookDebugOnce = sync.Once{}
		hookDebugOn = false
	})
	return func() []string {
		data, err := os.ReadFile(hookDebugLogFile)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

// US4: one log line per Codex event, naming the resolution path.
func TestCodexDebugLog_OneLinePerResolution(t *testing.T) {
	type step struct {
		resolution, session, pane, target string
		run                               func(s *codexScenario)
	}
	steps := []step{
		{"discovered", "s-disc", "25", inheritedSession, func(s *codexScenario) {
			s.hook("UserPromptSubmit", "s-disc", s.dir("a"))
		}},
		{"from-client-cache", "s-disc", "25", inheritedSession, func(s *codexScenario) {
			s.hook("PostToolUse", "s-disc", s.dir("a"))
		}},
		{"ambiguous", "s-amb", "", "", func(s *codexScenario) {
			s.hook("UserPromptSubmit", "s-amb", s.dir("shared"))
		}},
		{"no-client", "s-none", "", "", func(s *codexScenario) {
			s.hook("UserPromptSubmit", "s-none", s.dir("empty"))
		}},
		{"client-not-in-zellij", "s-out", "", "", func(s *codexScenario) {
			s.hook("UserPromptSubmit", "s-out", s.dir("outside"))
		}},
		{"client-env", "s-env", "33", "s3", func(s *codexScenario) {
			s.inZellij(inheritedSession)
			runHook(strings.NewReader(codexEventWithClientEnv("PostToolUse", "s-env", s.dir("a"),
				map[string]string{"ZELLIJ_PANE_ID": "33", "ZELLIJ_SESSION_NAME": "s3"})), inheritedPane, "codex")
		}},
	}

	s := newCodexScenario(t, true)
	read := enableHookDebug(t)
	s.client(s.dir("a"), 25, inheritedSession)
	s.client(s.dir("shared"), 21, inheritedSession)
	s.client(s.dir("shared"), 22, inheritedSession)
	s.client(s.dir("outside"), 0, "", outsideZellijClient())

	for i, st := range steps {
		st.run(s)
		lines := read()
		if len(lines) != i+1 {
			t.Fatalf("%s: got %d log lines, want %d:\n%s", st.resolution, len(lines), i+1, strings.Join(lines, "\n"))
		}
		line := lines[i]
		for _, want := range []string{
			"resolution=" + st.resolution + " ",
			"session=" + st.session + " ",
			"pane=" + st.pane + " ",
			fmt.Sprintf("target_session=%q", st.target),
			fmt.Sprintf("ZELLIJ_SESSION_NAME=%q", inheritedSession),
		} {
			if !strings.Contains(line, want) {
				t.Errorf("%s: line lacks %q:\n%s", st.resolution, want, line)
			}
		}
		if !regexp.MustCompile(`event=(UserPromptSubmit|PostToolUse) `).MatchString(line) {
			t.Errorf("%s: line lacks the event name: %s", st.resolution, line)
		}
	}

	// The non-daemon path: from-arg.
	t.Run("from-arg", func(t *testing.T) {
		s := newCodexScenario(t, false)
		read := enableHookDebug(t)
		s.inZellij("cc-deck-local")
		runHook(strings.NewReader(codexEvent("PreToolUse", "s-arg", "/tmp")), "7", "codex")
		lines := read()
		if len(lines) != 1 || !strings.Contains(lines[0], "resolution=from-arg ") ||
			!strings.Contains(lines[0], "pane=7 ") || !strings.Contains(lines[0], `target_session="cc-deck-local"`) {
			t.Fatalf("from-arg line: %q", lines)
		}
	})
}

// BenchmarkCodexAttribution measures the attribution costs behind SC-004
// (cache hit adds at most 20 ms) and SC-005 (first event within 500 ms with
// up to 10 Codex clients). Results are recorded in
// specs/089-codex-daemon-pane-attribution/VERIFY.md.
//
//	go test ./internal/cmd/ -run '^$' -bench CodexAttribution
func BenchmarkCodexAttribution(b *testing.B) {
	dir := b.TempDir()
	origStateDir, origMapFile := hookStateDir, paneMapFile
	hookStateDir, paneMapFile = dir, filepath.Join(dir, "pane-map.json")
	b.Cleanup(func() { hookStateDir, paneMapFile = origStateDir, origMapFile })

	// A realistic cache: 30 sessions, one of them the benchmarked one.
	m := map[string]paneMapEntry{}
	for i := 0; i < 30; i++ {
		m[fmt.Sprintf("sess-%02d", i)] = paneMapEntry{PaneID: uint32(i), ZellijSession: "s", UpdatedAt: time.Now().Unix(),
			ClientPID: 1000 + i, ClientStart: int64(i)}
	}
	savePaneMap(m)

	// Today's non-daemon cache lookup, the baseline for the SC-004 delta.
	b.Run("baseline-pane-map-lookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, ok := loadPaneMap()["sess-07"]; !ok {
				b.Fatal("missing entry")
			}
		}
	})

	// Cache-hit path against the real process table: the ancestor walk plus
	// one liveness check of a client (this process stands in for it).
	real := procinfo.Default()
	b.Run("cache-hit-real-procinfo", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = daemonHosted(real)
			if _, err := real.Get(real.Self()); err != nil {
				b.Fatal(err)
			}
			_ = loadPaneMap()["sess-07"]
		}
	})

	// Discovery: the real listing of this host's Codex processes...
	b.Run("list-user-real-procinfo", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := real.ListUser(); err != nil {
				b.Fatal(err)
			}
		}
	})

	// ...plus resolution over a synthetic table of 10 clients.
	home := b.TempDir()
	src := &fakeProcs{self: 1, procs: map[int]procinfo.Process{}, getErr: map[int]error{}}
	var target string
	for i := 0; i < 10; i++ {
		d := filepath.Join(home, fmt.Sprintf("w%d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			b.Fatal(err)
		}
		if i == 9 {
			target = d
		}
		src.add(procinfo.Process{PID: 2000 + i, PPID: 1, StartTime: int64(i), Exe: "codex",
			Env: map[string]string{"PWD": d, "HOME": home, "ZELLIJ_PANE_ID": fmt.Sprint(i), "ZELLIJ_SESSION_NAME": "s"}})
	}
	in := codexInput{SessionID: "new", Cwd: target, CodexHome: resolvePath(filepath.Join(home, ".codex"), "")}
	b.Run("discovery-fake-10-clients", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if att := resolveCodex(in, src, loadPaneMap()); att.Reason != reasonDiscovered {
				b.Fatalf("reason %s", att.Reason)
			}
		}
	})
}

// daemonHosted reports whether the scripted hook process runs inside the
// Codex shared daemon (see codexHost).
func daemonHosted(src procinfo.Source) bool {
	h, _ := codexHost(src)
	return h == hostDaemon
}

// US1 acceptance 5 and the edge case "the pane that started the daemon
// still runs its own Codex client": that client's events go to its pane
// (which is also the inherited one), other clients' events do not.
func TestCodexDaemon_ClientInInheritedPane(t *testing.T) {
	s := newCodexScenario(t, true)
	a, b := s.dir("a"), s.dir("b")
	pidA := s.client(a, 19, inheritedSession)
	s.client(b, 25, inheritedSession)

	s.hook("UserPromptSubmit", "sess-a", a)
	s.expectDelivered(1, 19)
	s.hook("UserPromptSubmit", "sess-b", b)
	s.expectDelivered(2, 25)

	if e := loadPaneMap()["sess-a"]; e.PaneID != 19 || e.ClientPID != pidA {
		t.Errorf("mapping for the daemon's own pane: %+v", e)
	}
	for id, e := range loadPaneMap() {
		if e.ClientPID == 0 {
			t.Errorf("cache entry %s without a client: %+v", id, e)
		}
	}
}

// A daemon of a profile's Codex home only considers clients of that home:
// a default-home client in the same directory neither matches nor makes
// the match ambiguous.
func TestCodexDaemon_ProfileCodexHome(t *testing.T) {
	s := newCodexScenario(t, true)
	profileHome := filepath.Join(s.home, "profile-home")
	if err := os.MkdirAll(profileHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", profileHome)
	a := s.dir("a")
	s.client(a, 25, inheritedSession) // default home
	s.client(a, 26, inheritedSession, withEnv("CODEX_HOME", profileHome))

	s.hook("UserPromptSubmit", "sess-1", a)

	s.expectDelivered(1, 26)
}

// The real working directory wins over a stale PWD (a Zellij command pane
// or another non-shell launcher changed the directory).
func TestCodexDaemon_RealCwdWinsOverStalePWD(t *testing.T) {
	s := newCodexScenario(t, true)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)
	p := s.src.procs[pid]
	p.Env["PWD"] = s.home // stale: the launcher's directory
	p.Cwd = a
	s.src.procs[pid] = p

	s.hook("UserPromptSubmit", "sess-1", a)

	s.expectDelivered(1, 25)
}

// Values from another process's environment that would be unsafe as a
// zellij argument or in the sidebar are rejected.
func TestCodexDaemon_RejectsUnsafeClientIdentity(t *testing.T) {
	t.Run("session name", func(t *testing.T) {
		for _, name := range []string{"-x", "a/b", "bad\x1bname"} {
			s := newCodexScenario(t, true)
			a := s.dir("a")
			s.client(a, 25, name)
			s.hook("UserPromptSubmit", "sess-1", a)
			s.expectNothingSent()
		}
	})
	t.Run("profile name", func(t *testing.T) {
		s := newCodexScenario(t, true)
		a := s.dir("a")
		s.client(a, 25, inheritedSession, withEnv("CC_DECK_PROFILE", "bad\x1b[2Jname"))
		s.hook("UserPromptSubmit", "sess-1", a)
		got := s.expectDelivered(1, 25)
		if got.Profile != "" {
			t.Errorf("unsafe profile name delivered: %q", got.Profile)
		}
		if e := loadPaneMap()["sess-1"]; e.Profile != "" {
			t.Errorf("unsafe profile name cached: %q", e.Profile)
		}
	})
}

// Dropped events say why in the debug log (US4): inspection failures and
// stale mappings are distinguishable from "no codex in this directory".
func TestCodexDaemon_DebugLogDetail(t *testing.T) {
	s := newCodexScenario(t, true)
	read := enableHookDebug(t)
	a := s.dir("a")
	pid := s.client(a, 25, inheritedSession)
	s.hook("UserPromptSubmit", "sess-1", a)
	delete(s.src.procs, pid)
	s.hook("PostToolUse", "sess-1", a)
	s.src.listErr = errors.New("permission denied")
	s.hook("UserPromptSubmit", "sess-2", a)
	s.src.getErr[hookPID] = errors.New("self gone")
	s.hook("UserPromptSubmit", "sess-3", a)

	lines := read()
	if len(lines) != 4 {
		t.Fatalf("got %d log lines:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []string{
		"",
		`resolution=no-client detail="stale-mapping"`,
		`resolution=no-client detail="list: permission denied"`,
		`resolution=no-client detail="host-unknown: self: self gone"`,
	} {
		if want != "" && !strings.Contains(lines[i], want) {
			t.Errorf("line %d lacks %q:\n%s", i, want, lines[i])
		}
	}
	if strings.Contains(lines[0], "detail=") {
		t.Errorf("a delivered event has no detail: %s", lines[0])
	}
}

// Payload values cannot forge debug log lines.
func TestCodexDebugLogStripsControlCharacters(t *testing.T) {
	s := newCodexScenario(t, true)
	read := enableHookDebug(t)
	s.hook("UserPromptSubmit", "sess\nforged line", s.dir("empty"))
	lines := read()
	if len(lines) != 1 || !strings.Contains(lines[0], "session=sess?forged line ") {
		t.Fatalf("log lines: %q", lines)
	}
}
