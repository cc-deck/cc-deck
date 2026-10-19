// Claude Code background hook attribution.
//
// See specs/090-claude-bg-pane-attribution/contracts/claude-bg-attribution.md.
package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/procinfo"
)

const (
	reasonViewerEnv         = "viewer-env"
	reasonParkedViewer      = "parked-viewer"
	reasonAttachViewer      = "attach-viewer"
	reasonHeadless          = "headless"
	reasonSeveralViewers    = "several-viewers"
	reasonViewerNotInZellij = "viewer-not-in-zellij"
	reasonRecordMissing     = "record-missing"
)

var claudeJobID = regexp.MustCompile(`^[0-9a-f]{8}$`)
var claudeVersionComm = regexp.MustCompile(`^\d+\.\d+\.\d+([.-].*)?$`)

type claudeRecord struct {
	PID         int    `json:"pid"`
	SessionID   string `json:"sessionId"`
	Kind        string `json:"kind"`
	JobID       string `json:"jobId"`
	ParkedJobID string `json:"parkedJobId"`
	ProcStart   string `json:"procStart"`
}

func parseProcStart(s string) (int64, bool) {
	s = strings.Join(strings.Fields(s), " ")
	t, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", s, time.UTC)
	return t.Unix(), err == nil
}

func readClaudeRecord(dir string, pid int) (claudeRecord, error) {
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(pid)+".json"))
	if err != nil {
		return claudeRecord{}, err
	}
	var r claudeRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return claudeRecord{}, err
	}
	if r.PID != pid {
		return claudeRecord{}, fmt.Errorf("claude record pid %d does not match %d", r.PID, pid)
	}
	if r.Kind != "interactive" && r.Kind != "bg" {
		return claudeRecord{}, nil
	}
	if r.JobID != "" && !claudeJobID.MatchString(r.JobID) {
		return claudeRecord{}, fmt.Errorf("invalid job id")
	}
	if r.ParkedJobID != "" && !claudeJobID.MatchString(r.ParkedJobID) {
		return claudeRecord{}, fmt.Errorf("invalid parked job id")
	}
	if r.Kind == "interactive" {
		r.JobID = ""
	}
	return r, nil
}

func verifyRecord(r claudeRecord, p procinfo.Process) bool {
	start, ok := parseProcStart(r.ProcStart)
	if !ok || start == 0 || p.StartUnix == 0 {
		return false
	}
	d := start - p.StartUnix
	return d >= -1 && d <= 1
}

type claudeHost int

const (
	claudeForeground claudeHost = iota
	claudeBackground
	claudeHostUnknown
)

func claudeHostOf(src procinfo.Source, recordsDir string, bgHint bool) (claudeHost, claudeRecord, procinfo.Process, string) {
	p, err := src.Ancestor(src.Self())
	if err != nil {
		if bgHint {
			return claudeBackground, claudeRecord{}, procinfo.Process{}, "host-unknown"
		}
		return claudeForeground, claudeRecord{}, procinfo.Process{}, ""
	}
	for i := 0; i < daemonAncestorDepthLimit; i++ {
		if p.PPID <= 1 {
			break
		}
		p, err = src.Ancestor(p.PPID)
		if errors.Is(err, procinfo.ErrForeign) {
			break
		}
		if err != nil {
			if bgHint {
				return claudeBackground, claudeRecord{}, procinfo.Process{}, "host-unknown"
			}
			return claudeForeground, claudeRecord{}, procinfo.Process{}, ""
		}
		if r, err := readClaudeRecord(recordsDir, p.PID); err == nil && r.Kind != "" && verifyRecord(r, p) {
			if r.Kind == "interactive" {
				return claudeForeground, claudeRecord{}, procinfo.Process{}, ""
			}
			return claudeBackground, r, p, ""
		}
		for _, arg := range p.Args {
			if arg == "--bg-pty-host" {
				return claudeBackground, claudeRecord{}, procinfo.Process{}, ""
			}
		}
	}
	return claudeForeground, claudeRecord{}, procinfo.Process{}, ""
}

type bgWorker struct {
	pid                         int
	start                       int64
	jobID, sessionID, configDir string
}
type claudeViewer struct {
	pid           int
	start         int64
	startUnix     int64
	via           string
	configDir     string
	paneID        uint32
	zellijSession string
	profile       profileEnv
	inZellij      bool
}
type terminalKey struct {
	ZellijSession string
	PaneID        uint32
}
type bgAttribution struct {
	PaneID                uint32
	ZellijSession         string
	Profile               profileEnv
	Reason, Detail, JobID string
	Terminals             []terminalKey
}

func (a bgAttribution) delivers() bool {
	return a.Reason == reasonViewerEnv || a.Reason == reasonParkedViewer || a.Reason == reasonAttachViewer
}
func (a bgAttribution) pane() string {
	if !a.delivers() {
		return ""
	}
	return strconv.FormatUint(uint64(a.PaneID), 10)
}

func claudeConfigDir(env map[string]string) string {
	if dir := env["CLAUDE_CONFIG_DIR"]; dir != "" {
		return resolvePath(dir, env["PWD"])
	}
	home := env["HOME"]
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return ""
	}
	return resolvePath(filepath.Join(home, ".claude"), "")
}

func workerOf(src procinfo.Source, rec claudeRecord, p procinfo.Process) (bgWorker, error) {
	if rec.Kind != "bg" || rec.JobID == "" || !verifyRecord(rec, p) {
		return bgWorker{}, errors.New("unverified worker")
	}
	live, err := src.Get(p.PID)
	if err != nil {
		return bgWorker{}, fmt.Errorf("worker: %w", err)
	}
	if live.StartTime != p.StartTime || !verifyRecord(rec, live) {
		return bgWorker{}, errors.New("worker pid reused")
	}
	dir := claudeConfigDir(live.Env)
	if dir == "" {
		return bgWorker{}, errors.New("worker has no config directory")
	}
	return bgWorker{pid: p.PID, start: p.StartTime, jobID: rec.JobID, sessionID: rec.SessionID, configDir: dir}, nil
}

func viewerFromProcess(p procinfo.Process, via, configDir string) (claudeViewer, bool) {
	dir := claudeConfigDir(p.Env)
	if dir == "" || dir != configDir {
		return claudeViewer{}, false
	}
	v := claudeViewer{pid: p.PID, start: p.StartTime, startUnix: p.StartUnix, via: via, configDir: dir, profile: profileEnv{Name: p.Env["CC_DECK_PROFILE"], Color: p.Env["CC_DECK_PROFILE_COLOR"], Icon: p.Env["CC_DECK_PROFILE_ICON"]}}
	if p.Env["ZELLIJ"] == "" || !validZellijSessionName(p.Env["ZELLIJ_SESSION_NAME"]) {
		return v, true
	}
	n, err := strconv.ParseUint(p.Env["ZELLIJ_PANE_ID"], 10, 32)
	if err != nil {
		return v, true
	}
	v.inZellij, v.paneID, v.zellijSession = true, uint32(n), p.Env["ZELLIJ_SESSION_NAME"]
	return v, true
}

func parkedViewers(src procinfo.Source, w bgWorker) ([]claudeViewer, error) {
	entries, err := os.ReadDir(filepath.Join(w.configDir, "sessions"))
	if err != nil {
		return nil, err
	}
	var out []claudeViewer
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		r, err := readClaudeRecord(filepath.Join(w.configDir, "sessions"), pid)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if r.Kind != "interactive" || r.ParkedJobID != w.jobID {
			continue
		}
		p, err := src.Get(pid)
		if err != nil {
			// An exited or stale record is harmless. A live matching process
			// whose environment is unreadable could be another terminal.
			ancestor, inspectErr := src.Ancestor(pid)
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, procinfo.ErrForeign) || errors.Is(inspectErr, os.ErrNotExist) || errors.Is(inspectErr, procinfo.ErrForeign) {
				continue
			}
			if inspectErr == nil && !verifyRecord(r, ancestor) {
				continue
			}
			return nil, err
		}
		if !verifyRecord(r, p) {
			continue
		}
		if v, ok := viewerFromProcess(p, "parked", w.configDir); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

func isClaudeComm(c string) bool {
	return c == "claude" || c == "node" || claudeVersionComm.MatchString(c)
}
func claudeCLIArgs(p procinfo.Process) ([]string, bool) {
	b := filepath.Base(p.Exe)
	if b == "claude" || claudeVersionComm.MatchString(b) {
		return p.Args, true
	}
	if b == "node" || p.Comm == "node" {
		if len(p.Args) > 0 && strings.HasSuffix(p.Args[0], "/@anthropic-ai/claude-code/cli.js") {
			return p.Args[1:], true
		}
	}
	return nil, false
}
func attachTarget(args []string) string {
	var values = map[string]bool{"--model": true, "-m": true, "--profile": true}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if values[a] && i+1 < len(args) {
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) >= 2 && positional[0] == "attach" {
		return positional[1]
	}
	return ""
}
func attachViewers(src procinfo.Source, w bgWorker) ([]claudeViewer, error) {
	procs, err := src.ListUserMatching(isClaudeComm)
	if err != nil {
		return nil, err
	}
	var out []claudeViewer
	for _, p := range procs {
		args, ok := claudeCLIArgs(p)
		if !ok {
			continue
		}
		target := attachTarget(args)
		if target == "" || (target != w.jobID && target != w.sessionID) {
			continue
		}
		live, err := src.Get(p.PID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, procinfo.ErrForeign) {
				continue
			}
			return nil, err
		}
		if live.StartTime != p.StartTime {
			continue
		}
		if v, ok := viewerFromProcess(live, "attach", w.configDir); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

func resolveClaudeBackground(w bgWorker, viewers func() ([]claudeViewer, error)) bgAttribution {
	att := bgAttribution{JobID: w.jobID}
	vs, err := viewers()
	if err != nil {
		att.Reason, att.Detail = reasonRecordMissing, err.Error()
		return att
	}
	types := map[terminalKey][]claudeViewer{}
	outside := 0
	for _, v := range vs {
		if !v.inZellij {
			outside++
			continue
		}
		k := terminalKey{v.zellijSession, v.paneID}
		if _, exists := types[k]; !exists {
			att.Terminals = append(att.Terminals, k)
		}
		types[k] = append(types[k], v)
	}
	if len(types)+outside > 1 {
		att.Reason = reasonSeveralViewers
		return att
	}
	if len(types) == 0 {
		if outside == 1 {
			att.Reason = reasonViewerNotInZellij
		} else {
			att.Reason = reasonHeadless
		}
		return att
	}
	for key, group := range types {
		att.PaneID, att.ZellijSession = key.PaneID, key.ZellijSession
		chosen := group[0]
		for _, v := range group {
			if v.via == "parked" {
				chosen = v
				break
			}
		}
		att.Profile = chosen.profile
		if chosen.via == "parked" {
			att.Reason = reasonParkedViewer
		} else {
			att.Reason = reasonAttachViewer
		}
		return att
	}
	return att
}

type bgJobPane struct {
	PaneID        uint32         `json:"pane_id"`
	ZellijSession string         `json:"zellij_session"`
	SessionID     string         `json:"session_id"`
	UpdatedAt     int64          `json:"updated_at"`
	Released      bool           `json:"released,omitempty"`
	Pending       []bgRowRelease `json:"pending,omitempty"`
}

type bgRowRelease struct {
	PaneID        uint32 `json:"pane_id"`
	ZellijSession string `json:"zellij_session"`
	SessionID     string `json:"session_id"`
	RetryAfter    int64  `json:"retry_after,omitempty"`
	Attempts      uint8  `json:"attempts,omitempty"`
}

var bgJobPaneFile = filepath.Join(hookStateDir, "bg-job-panes.json")

const bgJobLockTimeout = 100 * time.Millisecond
const bgLedgerLockTimeout = 20 * time.Millisecond

// Four concurrent real Zellij senders measured up to 76 ms on the review host.
// Leave transport headroom while keeping cleanup below the 150 ms budget.
const bgCleanupBudget = 100 * time.Millisecond

func bgJobKey(w bgWorker) string { return w.configDir + "|" + w.jobID }

func bgJobLockPath(w bgWorker) string {
	return filepath.Join(filepath.Dir(bgJobPaneFile), fmt.Sprintf("bg-job-%x.lock", sha256.Sum256([]byte(bgJobKey(w)))))
}

func withBGLedger(fn func()) bool {
	return withStateLockTimeout(filepath.Join(filepath.Dir(bgJobPaneFile), "bg-job-panes.lock"), bgLedgerLockTimeout, fn)
}

// Merge only this job under the short ledger lock. Never hold it during Zellij
// I/O: an unavailable session must not stall other jobs.
func storeBGJobPane(key string, next *bgJobPane) error {
	var err error
	if !withBGLedger(func() {
		m := loadBGJobPanes()
		if next == nil {
			delete(m, key)
		} else {
			m[key] = *next
		}
		err = saveStateFileAtomic(bgJobPaneFile, m)
	}) {
		return errors.New("background ledger lock unavailable")
	}
	return err
}

func loadBGJobPanes() map[string]bgJobPane {
	data, err := os.ReadFile(bgJobPaneFile)
	if err != nil {
		return map[string]bgJobPane{}
	}
	var m map[string]bgJobPane
	if json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]bgJobPane{}
	}
	// Rows can outlive an arbitrary idle interval. Age alone cannot prove that
	// the remembered row has disappeared, so retain outstanding ownership.
	return m
}
func jobPaneTransition(cur *bgJobPane, att bgAttribution, sessionID, event string, now time.Time) (release, next *bgJobPane, write bool) {
	if event == "SessionEnd" {
		if cur == nil {
			return nil, nil, false
		}
		if att.Reason == reasonRecordMissing {
			return nil, cur, false
		}
		if !att.delivers() || cur.PaneID != att.PaneID || cur.ZellijSession != att.ZellijSession {
			return cur, nil, true
		}
		return nil, nil, true
	}
	if cur == nil {
		if att.delivers() {
			n := &bgJobPane{PaneID: att.PaneID, ZellijSession: att.ZellijSession, SessionID: sessionID, UpdatedAt: now.Unix()}
			return nil, n, true
		}
		return
	}
	switch att.Reason {
	case reasonRecordMissing:
		return nil, cur, false
	case reasonSeveralViewers:
		for _, t := range att.Terminals {
			if t.PaneID == cur.PaneID && t.ZellijSession == cur.ZellijSession {
				return nil, cur, false
			}
		}
		return cur, nil, true
	case reasonHeadless, reasonViewerNotInZellij:
		return cur, nil, true
	}
	if !att.delivers() {
		return nil, cur, false
	}
	if cur.PaneID == att.PaneID && cur.ZellijSession == att.ZellijSession {
		if cur.SessionID != sessionID || now.Unix()-cur.UpdatedAt >= int64(time.Hour.Seconds()) {
			n := *cur
			n.SessionID = sessionID
			n.UpdatedAt = now.Unix()
			return nil, &n, true
		}
		return nil, cur, false
	}
	n := &bgJobPane{PaneID: att.PaneID, ZellijSession: att.ZellijSession, SessionID: sessionID, UpdatedAt: now.Unix()}
	return cur, n, true
}
func sendRowRelease(ctx context.Context, zellijPath string, r bgRowRelease) error {
	payload, _ := json.Marshal(struct {
		PaneID    uint32 `json:"pane_id"`
		SessionID string `json:"session_id"`
	}{r.PaneID, r.SessionID})
	return sendHookPayload(ctx, zellijPath, r.ZellijSession, "cc-deck:release-row", payload)
}

func runClaudeBackground(zellijPath string, input []byte, normalized *agent.NormalizedPayload, paneIDArg string, workerRec claudeRecord, workerProc procinfo.Process, detail string) {
	var worker bgWorker
	var workerErr error
	if workerRec.Kind == "bg" {
		worker, workerErr = workerOf(hookProcSource, workerRec, workerProc)
	}
	run := func() {
		runClaudeBackgroundLocked(zellijPath, input, normalized, paneIDArg, workerRec, worker, workerErr, detail)
	}
	if worker.jobID != "" {
		// Resolve only after serialization: an older hook must not apply its
		// pre-lock viewer snapshot after a newer hook has already moved the row.
		if !withStateLockTimeout(bgJobLockPath(worker), bgJobLockTimeout, run) {
			logClaudeBackground(normalized.HookEvent, normalized.SessionID, worker.jobID, reasonRecordMissing, "background job lock unavailable", "", "", paneIDArg)
			hookDebugf("event=%s job=%s state_error=%q", stripControl(normalized.HookEvent), stripControl(worker.jobID), "background job lock unavailable")
		}
		return
	}
	run()
}

func runClaudeBackgroundLocked(zellijPath string, input []byte, normalized *agent.NormalizedPayload, paneIDArg string, workerRec claudeRecord, worker bgWorker, workerErr error, detail string) {
	att := bgAttribution{Reason: reasonRecordMissing, Detail: detail}
	if target, ok := clientEnvTarget(agent.ClaudeViewerEnv(input), true, ""); ok {
		att = bgAttribution{PaneID: target.PaneID, ZellijSession: target.ZellijSession, Profile: target.Profile, Reason: reasonViewerEnv}
		if workerRec.JobID != "" {
			att.JobID = workerRec.JobID
		}
	} else if workerRec.Kind == "bg" {
		if workerErr != nil {
			att.Detail = workerErr.Error()
		} else {
			att = resolveClaudeBackground(worker, func() ([]claudeViewer, error) {
				parked, e := parkedViewers(hookProcSource, worker)
				if e != nil {
					return nil, e
				}
				attached, e := attachViewers(hookProcSource, worker)
				return append(parked, attached...), e
			})
		}
	}
	if att.JobID == "" {
		att.JobID = workerRec.JobID
	}
	if worker.jobID != "" {
		att.JobID = worker.jobID
	}
	logClaudeBackground(normalized.HookEvent, normalized.SessionID, att.JobID, att.Reason, att.Detail, att.pane(), att.ZellijSession, paneIDArg)
	deliver := func() {
		if !att.delivers() {
			if normalized.HookEvent == "SessionEnd" && normalized.SessionID != "" {
				removePaneMapping(normalized.SessionID)
			}
			return
		}
		normalized.Background = true
		normalized.PaneID = att.PaneID
		if att.Profile.Name != "" && config.ValidProfileName(att.Profile.Name) {
			enrichProfileFrom(normalized, att.Profile)
		}
		if err := deliverHookPayload(zellijPath, att.ZellijSession, normalized, false); err != nil {
			hookDebugf("event=%s job=%s delivery_error=%q", stripControl(normalized.HookEvent), stripControl(att.JobID), err)
		}
	}
	if worker.jobID != "" {
		func() {
			key := bgJobKey(worker)
			// Atomic replacement guarantees a complete snapshot; the per-job
			// lock excludes competing updates to this entry. Only merges write
			// under the global ledger lock.
			cur, ok := loadBGJobPanes()[key]
			var ptr *bgJobPane
			if ok && !cur.Released {
				ptr = &cur
			}
			release, next, write := jobPaneTransition(ptr, att, normalized.SessionID, normalized.HookEvent, time.Now())
			pending := append([]bgRowRelease(nil), cur.Pending...)
			if release != nil {
				pending = append(pending, bgRowRelease{PaneID: release.PaneID, ZellijSession: release.ZellijSession, SessionID: release.SessionID})
			}
			if len(pending) > 0 {
				if next == nil {
					next = &bgJobPane{Released: true}
				}
				// Do not release a row the unique viewer has returned to. It is
				// active ownership again, not stale cleanup; retain manual names.
				next.Pending = nil
				for _, r := range pending {
					if !next.Released && r.PaneID == next.PaneID && r.ZellijSession == next.ZellijSession && r.SessionID == next.SessionID {
						continue
					}
					if !containsBGRelease(next.Pending, r) {
						next.Pending = append(next.Pending, r)
					}
				}
				write = true
			}
			// Persist intent before any pipe send. A failed send (or process
			// death) must not lose either the old cleanup or the new owner.
			if write {
				if err := storeBGJobPane(key, next); err != nil {
					hookDebugf("event=%s job=%s state_error=%q", stripControl(normalized.HookEvent), stripControl(att.JobID), err)
					return
				}
			}
			if next != nil && len(next.Pending) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), bgCleanupBudget)
				defer cancel()
				remaining := []bgRowRelease{}
				for _, r := range next.Pending {
					if !canReleaseBGRow(att, r) || r.RetryAfter > time.Now().UnixMilli() || ctx.Err() != nil {
						remaining = append(remaining, r)
						continue
					}
					if err := sendRowRelease(ctx, zellijPath, r); err != nil {
						hookDebugf("event=%s job=%s release_pane=%d release_error=%q", stripControl(normalized.HookEvent), stripControl(att.JobID), r.PaneID, err)
						if r.Attempts < 6 {
							r.Attempts++
						}
						r.RetryAfter = time.Now().Add(time.Second << (r.Attempts - 1)).UnixMilli()
						remaining = append(remaining, r)
					} else {
						logRowRelease(normalized.HookEvent, normalized.SessionID, att.JobID, r)
					}
				}
				next.Pending = remaining
				if next.Released && len(remaining) == 0 {
					next = nil
				}
				if err := storeBGJobPane(key, next); err != nil {
					hookDebugf("event=%s job=%s state_error=%q", stripControl(normalized.HookEvent), stripControl(att.JobID), err)
				}
			}
			deliver()
		}()
		// Never create a row without its removal ledger, even on contention.
		return
	}
	deliver()
}

func containsBGRelease(releases []bgRowRelease, r bgRowRelease) bool {
	for _, existing := range releases {
		if existing.PaneID == r.PaneID && existing.ZellijSession == r.ZellijSession && existing.SessionID == r.SessionID {
			return true
		}
	}
	return false
}

func canReleaseBGRow(att bgAttribution, r bgRowRelease) bool {
	if att.Reason == reasonRecordMissing {
		return false
	}
	for _, t := range att.Terminals {
		if t.PaneID == r.PaneID && t.ZellijSession == r.ZellijSession {
			return false
		}
	}
	return !att.delivers() || att.PaneID != r.PaneID || att.ZellijSession != r.ZellijSession
}
