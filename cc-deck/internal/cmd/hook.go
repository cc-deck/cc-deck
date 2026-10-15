package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/badge"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/procinfo"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/session"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// NewHookCmd creates the hook cobra command.
func NewHookCmd() *cobra.Command {
	var paneIDStr string
	var agentName string
	var rawMode bool

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Forward AI agent hook events to the Zellij plugin",
		Long: `Reads hook event JSON from stdin and forwards it as a pipe message
to the cc-deck Zellij plugin. Designed to be registered as a hook command
in an AI agent's configuration.

The --agent flag identifies the calling agent (default: "claude").
The --raw flag accepts pre-normalized JSON payloads and forwards them
directly, skipping TranslateEvent().
The --pane-id flag should use shell expansion ($ZELLIJ_PANE_ID) so the
shell resolves it before the binary runs.

Exits silently (code 0) when the process is not running inside a Zellij
session, when the input is malformed, or on any other error, so that it
never disrupts the agent. Zellij membership is decided from the $ZELLIJ
environment variable rather than from an empty --pane-id: an agent outside
Zellij has no pane, and "zellij pipe" would otherwise deliver its events to
whichever session happens to be running.`,
		Args:   cobra.NoArgs,
		Hidden: true,
		Run: func(cmd *cobra.Command, _ []string) {
			if rawMode {
				runHookRaw(os.Stdin, os.Stderr)
			} else {
				runHook(os.Stdin, paneIDStr, agentName)
			}
		},
	}

	cmd.Flags().StringVar(&paneIDStr, "pane-id", "", "Zellij pane ID (use $ZELLIJ_PANE_ID for shell expansion)")
	cmd.Flags().StringVar(&agentName, "agent", "claude", "Agent name (claude, codex, opencode)")
	cmd.Flags().BoolVar(&rawMode, "raw", false, "Accept pre-normalized JSON payload (skip TranslateEvent)")

	return cmd
}

// paneMapFile is the path for the session_id -> pane cache.
var hookStateDir = filepath.Join(xdg.StateHome, "cc-deck")

var paneMapFile = filepath.Join(hookStateDir, "pane-map.json")

// paneMapTTL bounds how long a cached pane id stays usable.
//
// Entries are removed on SessionEnd, but an agent that is killed or that
// crashes never sends one, so without an expiry the cache grows a permanent
// tail of mappings to panes that stopped existing long ago.
const paneMapTTL = 12 * time.Hour

// paneMapEntry records the pane an agent session was last seen in, scoped to
// the Zellij session that observed it.
//
// The scope matters because Zellij numbers panes from zero in every run: a
// pane id borrowed from an earlier run does not merely miss, it addresses a
// different, live pane in the current one.
//
// ClientPID, ClientStart and Profile are set only by daemon-hosted Codex
// attribution: they identify the Codex terminal client that owns the session
// (process id plus start time, which guards against pid reuse) and its
// cc-deck profile. Entries without ClientPID are never used for daemon-hosted
// Codex events, because their pane may be the one the daemon inherited.
type paneMapEntry struct {
	PaneID        uint32 `json:"pane_id"`
	ZellijSession string `json:"zellij_session"`
	UpdatedAt     int64  `json:"updated_at"`
	ClientPID     int    `json:"client_pid,omitempty"`
	ClientStart   int64  `json:"client_start,omitempty"`
	Profile       string `json:"profile,omitempty"`
}

// loadPaneMap reads the cache, dropping entries that have expired.
//
// A file written in the older flat `session_id -> pane_id` shape fails to
// decode and yields an empty map, which is the intended migration: those
// entries carry no Zellij session and so could never be trusted anyway.
func loadPaneMap() map[string]paneMapEntry {
	data, err := os.ReadFile(paneMapFile)
	if err != nil {
		return make(map[string]paneMapEntry)
	}
	var m map[string]paneMapEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]paneMapEntry)
	}
	cutoff := time.Now().Add(-paneMapTTL).Unix()
	for sessionID, entry := range m {
		if entry.UpdatedAt < cutoff {
			delete(m, sessionID)
		}
	}
	return m
}

// savePaneMap replaces the cache atomically: the map is written to a temp
// file in the same directory, synced, and renamed over pane-map.json, so a
// concurrent reader sees either the old or the new file, never a partial
// one. Callers that read, modify and write the map must hold the pane-map
// lock (withPaneMapLock) so that concurrent hooks do not lose each other's
// entries.
func savePaneMap(m map[string]paneMapEntry) {
	dir := filepath.Dir(paneMapFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "pane-map-*.json")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, paneMapFile); err != nil {
		_ = os.Remove(tmpName)
	}
}

// paneMapLockTimeout bounds how long a hook waits for the pane-map lock.
// The hook must never block its agent for long, so a lock that stays taken
// skips the cache update; the event itself is still delivered.
var paneMapLockTimeout = 2 * time.Second

// paneMapLockPoll is the retry interval while the lock is taken.
const paneMapLockPoll = 10 * time.Millisecond

// withPaneMapLock runs fn while holding an exclusive flock on pane-map.lock
// next to pane-map.json. It reports false, without running fn, when the
// lock cannot be acquired within paneMapLockTimeout. The lock is released
// when fn returns, and by the kernel when the process dies.
func withPaneMapLock(fn func()) bool {
	dir := filepath.Dir(paneMapFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	f, err := os.OpenFile(filepath.Join(dir, "pane-map.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	deadline := time.Now().Add(paneMapLockTimeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN && err != syscall.EINTR {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(paneMapLockPoll)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	fn()
	return true
}

// paneMapRefresh is how old a still-correct cache entry may get before its
// timestamp is renewed. Between refreshes an unchanged mapping costs no
// write, which matters because this runs on every hook event.
const paneMapRefresh = time.Hour

// recordPaneMapping stores the session's pane, writing the cache only when
// the mapping is new, changed, or due for a timestamp refresh.
func recordPaneMapping(sessionID string, paneID uint32, zellijSession string) {
	recordClientMapping(sessionID, paneMapEntry{PaneID: paneID, ZellijSession: zellijSession})
}

// paneMappingFresh reports whether cur already records want (pane, Zellij
// session, client identity and profile) and is younger than paneMapRefresh.
func paneMappingFresh(cur, want paneMapEntry, now time.Time) bool {
	if cur.UpdatedAt == 0 {
		return false
	}
	return cur.PaneID == want.PaneID &&
		cur.ZellijSession == want.ZellijSession &&
		cur.ClientPID == want.ClientPID &&
		cur.ClientStart == want.ClientStart &&
		cur.Profile == want.Profile &&
		now.Unix()-cur.UpdatedAt < int64(paneMapRefresh.Seconds())
}

// recordClientMapping stores a session's mapping (for daemon-hosted Codex
// sessions including the identity of the client): an unchanged mapping is
// rewritten only when its timestamp is older than paneMapRefresh.
// UpdatedAt in e is ignored.
func recordClientMapping(sessionID string, e paneMapEntry) {
	// Cheap unlocked check first: an unchanged, fresh mapping needs neither
	// the lock nor a write, and this runs on every hook event.
	now := time.Now()
	if paneMappingFresh(loadPaneMap()[sessionID], e, now) {
		return
	}
	withPaneMapLock(func() {
		m := loadPaneMap()
		if paneMappingFresh(m[sessionID], e, now) {
			return
		}
		e.UpdatedAt = now.Unix()
		m[sessionID] = e
		savePaneMap(m)
	})
}

// removePaneMapping deletes the session's entry under the pane-map lock.
func removePaneMapping(sessionID string) {
	if _, ok := loadPaneMap()[sessionID]; !ok {
		return
	}
	withPaneMapLock(func() {
		m := loadPaneMap()
		if _, ok := m[sessionID]; !ok {
			return
		}
		delete(m, sessionID)
		savePaneMap(m)
	})
}

// currentZellijSession reports the Zellij session this process belongs to.
//
// Zellij exports ZELLIJ and ZELLIJ_SESSION_NAME into every pane, and an agent
// started in a pane inherits them, as does the shell that runs its hooks. A
// hook process without them is not running in any pane of any session.
func currentZellijSession() (string, bool) {
	if os.Getenv("ZELLIJ") == "" {
		return "", false
	}
	return os.Getenv("ZELLIJ_SESSION_NAME"), true
}

// sendHookPayload forwards a normalized payload to the plugin.
//
// An empty targetSession runs `zellij pipe`, which reaches the caller's own
// session (today's command for every non-daemon event). A non-empty one runs
// `zellij --session <name> pipe`: a daemon-hosted Codex hook carries the
// daemon's inherited session, so its events must name the client's session
// explicitly.
//
// A package-level variable so tests can observe what would have been sent
// without shelling out to zellij.
var sendHookPayload = func(zellijPath, targetSession string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, zellijPath, hookPipeArgs(targetSession, payload)...)
	_ = cmd.Run()
}

// hookPipeArgs builds the zellij arguments that deliver a hook payload. An
// empty target session keeps the ambient session, exactly as before
// daemon-hosted Codex attribution existed.
func hookPipeArgs(targetSession string, payload []byte) []string {
	var args []string
	if targetSession != "" {
		args = append(args, "--session", targetSession)
	}
	return append(args, "pipe", "--name", "cc-deck:hook", "--", string(payload))
}

func runHook(stdin io.Reader, paneIDStr string, agentName string) {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return
	}

	input, err := io.ReadAll(stdin)
	if err != nil || len(input) == 0 {
		return
	}

	a := agent.Get(agentName)
	if a == nil {
		return
	}

	normalized, err := a.TranslateEvent(input)
	if err != nil || normalized == nil {
		return
	}

	if normalized.HookEvent == "" {
		return
	}

	normalized.AgentIndicator = a.Indicator()

	// Codex hooks may run inside the shared app-server daemon, whose
	// environment (and so --pane-id and ZELLIJ_SESSION_NAME) belongs to
	// whichever client started it. Such events are attributed to their
	// originating client instead, and the daemon's own Zellij membership is
	// irrelevant, so this runs before the not-in-zellij exit.
	if agentName == "codex" && runCodexHook(zellijPath, input, normalized, paneIDStr) {
		return
	}

	// Profile enrichment: when CC_DECK_PROFILE is set, attach profile name
	// and color to the payload so the sidebar can distinguish sessions.
	if profileName := os.Getenv("CC_DECK_PROFILE"); profileName != "" {
		enrichProfile(normalized, profileName)
	}

	// logRes records the resolution: Codex events get the attribution line
	// with session, pane and target (FR-014), other agents the original one.
	logRes := func(reason, pane, target string) {
		if agentName == "codex" {
			logCodexAttribution(normalized.HookEvent, normalized.SessionID, reason, "", pane, target, paneIDStr)
			return
		}
		logHookEnv(normalized.HookEvent, paneIDStr, reason)
	}

	// An agent running outside Zellij has no pane and so no sidebar row to
	// own. Stopping here is what keeps it out: `zellij pipe` succeeds from
	// outside a session and delivers to whichever session is running, so
	// without this check an agent in a plain terminal reaches the sidebar of
	// an unrelated Zellij session and stays there.
	zellijSession, inZellij := currentZellijSession()
	if !inZellij {
		logRes("not-in-zellij", "", "")
		return
	}

	var paneID uint32
	if paneIDStr != "" {
		paneID64, err := strconv.ParseUint(paneIDStr, 10, 32)
		if err != nil {
			logRes("bad-pane-id-arg", "", "")
			return
		}
		paneID = uint32(paneID64)
		logRes(reasonFromArg, strconv.FormatUint(uint64(paneID), 10), zellijSession)
		if normalized.SessionID != "" {
			recordPaneMapping(normalized.SessionID, paneID, zellijSession)
		}
	} else if normalized.SessionID != "" {
		// The pane id reaches this command by shell expansion of
		// $ZELLIJ_PANE_ID. Claude Code was once observed dropping the Zellij
		// environment from hook subprocesses, which is why this cache exists;
		// it is consulted only when the pane id really is missing.
		m := loadPaneMap()
		cached, ok := m[normalized.SessionID]
		if !ok {
			logRes("no-cache-entry", "", "")
			return
		}
		if cached.ZellijSession != zellijSession {
			// The mapping was recorded by a different Zellij session, where
			// this pane id meant something else entirely.
			logRes("cache-session-mismatch", "", "")
			return
		}
		paneID = cached.PaneID
		logRes("from-cache", strconv.FormatUint(uint64(cached.PaneID), 10), zellijSession)
	} else {
		logRes("no-pane-id-no-session-id", "", "")
		return
	}

	normalized.PaneID = paneID
	deliverHookPayload(zellijPath, "", normalized, true)
}

// deliverHookPayload evaluates badges, sends the payload to the plugin (in
// targetSession, or the caller's own session when empty) and handles the
// post-delivery bookkeeping (auto-save, SessionEnd cache removal). autoSave
// is false when the snapshot would be taken in a Zellij session other than
// the one the event belongs to.
func deliverHookPayload(zellijPath, targetSession string, normalized *agent.NormalizedPayload, autoSave bool) {
	if normalized.Cwd != "" {
		cfg, _ := config.Load("")
		if cfg != nil && len(cfg.Badges) > 0 {
			normalized.Badges = badge.Evaluate(cfg.Badges, normalized.Cwd)
		}
	}

	payloadJSON, err := json.Marshal(normalized)
	if err != nil {
		return
	}

	sendHookPayload(zellijPath, targetSession, payloadJSON)

	if autoSave {
		session.AutoSave()
	}

	if normalized.HookEvent == "SessionEnd" && normalized.SessionID != "" {
		removePaneMapping(normalized.SessionID)
	}
}

// runCodexHook handles the Codex-specific resolution steps (contract
// section 2): an explicit client identity in the payload, then, for hooks
// inside the shared daemon, cache and discovery. It reports true when it
// handled the event (delivered or dropped) and false when the event takes
// today's path from the hook's own pane identity.
func runCodexHook(zellijPath string, input []byte, normalized *agent.NormalizedPayload, paneIDStr string) bool {
	host, hostDetail := codexHost(hookProcSource)
	ambient, inZellij := currentZellijSession()

	// Step 1: an explicit client identity wins (FR-011). A hook whose host is
	// unknown is treated like a daemon-hosted one: its own session may be
	// inherited.
	if att, ok := clientEnvTarget(agent.CodexClientEnv(input), host != hostOwn, ambient); ok {
		logCodexAttribution(normalized.HookEvent, normalized.SessionID, att.Reason, att.Detail, att.pane(), att.ZellijSession, paneIDStr)
		deliverCodexAttribution(zellijPath, normalized, att, inZellij && ambient == att.ZellijSession)
		return true
	}

	switch host {
	case hostDaemon:
		runDaemonHostedCodex(zellijPath, normalized, paneIDStr, ambient, inZellij)
		return true
	case hostUnknown:
		// Without the process ancestry the hook cannot tell its own pane
		// from one inherited through the daemon; never risk a wrong row.
		logCodexAttribution(normalized.HookEvent, normalized.SessionID, reasonNoClient, "host-unknown: "+hostDetail, "", "", paneIDStr)
		return true
	}
	return false
}

// deliverCodexAttribution sends an attributed Codex event to the client's
// pane and Zellij session. The profile is the client's, never the
// daemon's (FR-017); a name that is not a valid profile name is dropped,
// because it comes from another process or the payload and reaches the
// sidebar unescaped.
func deliverCodexAttribution(zellijPath string, normalized *agent.NormalizedPayload, att attribution, autoSave bool) {
	normalized.PaneID = att.PaneID
	if att.Profile.Name != "" && config.ValidProfileName(att.Profile.Name) {
		enrichProfileFrom(normalized, att.Profile)
	}
	deliverHookPayload(zellijPath, att.ZellijSession, normalized, autoSave)
}

// hookProcSource inspects processes for Codex daemon attribution. Tests
// replace it with a scripted source.
var hookProcSource procinfo.Source = procinfo.Default()

// runDaemonHostedCodex attributes and delivers a Codex event whose hook runs
// inside the shared daemon. The inherited --pane-id and the hook's own
// ZELLIJ_SESSION_NAME are never used to choose the target, and the inherited
// pane is never written to the cache (FR-002).
func runDaemonHostedCodex(zellijPath string, normalized *agent.NormalizedPayload, paneIDStr, ambient string, inZellij bool) {
	in := hookCodexInput(normalized.HookEvent, normalized.SessionID, normalized.Cwd)
	att := resolveCodex(in, hookProcSource, loadPaneMap())
	logCodexAttribution(normalized.HookEvent, normalized.SessionID, att.Reason, att.Detail, att.pane(), att.ZellijSession, paneIDStr)
	if att.Forget && normalized.SessionID != "" {
		removePaneMapping(normalized.SessionID)
	}
	if !att.delivers() {
		return
	}

	if normalized.SessionID != "" && att.ClientPID > 0 && normalized.HookEvent != "SessionEnd" {
		profileName := att.Profile.Name
		if !config.ValidProfileName(profileName) {
			profileName = ""
		}
		recordClientMapping(normalized.SessionID, paneMapEntry{
			PaneID:        att.PaneID,
			ZellijSession: att.ZellijSession,
			ClientPID:     att.ClientPID,
			ClientStart:   att.ClientStart,
			Profile:       profileName,
		})
	}

	// The snapshot runs in the hook's ambient Zellij session; take it only
	// when that is the session the event belongs to.
	deliverCodexAttribution(zellijPath, normalized, att, inZellij && ambient == att.ZellijSession)
}

// loadProfileConfig loads config.yaml for the profile fallback in
// enrichProfile. Tests replace it to count reads (SC-010).
var loadProfileConfig = config.Load

// enrichProfile sets the profile name, color and optional icon on a hook
// payload. Wrappers export the effective color and icon, so sessions they
// start need no config.yaml read for profile data (research R17). Sessions
// from wrappers generated before that export existed, or with an unparsable
// color, fall back to the config lookup; an unknown profile, or one with an
// invalid declared color, gets the color derived from its name. A declared
// icon travels both as the agent indicator (it replaces the harness glyph,
// FR-018) and as profile_icon (the legend, FR-019).
func enrichProfile(normalized *agent.NormalizedPayload, profileName string) {
	enrichProfileFrom(normalized, profileEnv{
		Name:  profileName,
		Color: os.Getenv("CC_DECK_PROFILE_COLOR"),
		Icon:  os.Getenv("CC_DECK_PROFILE_ICON"),
	})
}

// enrichProfileFrom is enrichProfile for an explicit profile identity: the
// hook's own environment, or for daemon-hosted Codex events the originating
// client's (FR-017).
func enrichProfileFrom(normalized *agent.NormalizedPayload, pe profileEnv) {
	profileName := pe.Name
	normalized.Profile = profileName

	if envColor := pe.Color; envColor != "" {
		if _, _, _, err := profile.ParseHex(envColor); err == nil {
			normalized.ProfileColor = envColor
			// The icon reaches the sidebar unescaped, so an environment value
			// that config validation never saw must pass the same check.
			if icon := pe.Icon; config.ValidIcon(icon) {
				setProfileIcon(normalized, icon)
			}
			return
		}
	}

	normalized.ProfileColor = profile.Derive(profileName)
	cfg, _ := loadProfileConfig("")
	if cfg == nil {
		return
	}
	p, err := cfg.GetProfile(profileName)
	if err != nil {
		return
	}
	normalized.ProfileColor = profile.EffectiveColor(profileName, p)
	if config.ValidIcon(p.Icon) {
		setProfileIcon(normalized, p.Icon)
	}
}

// setProfileIcon records a declared profile icon on the payload.
func setProfileIcon(normalized *agent.NormalizedPayload, icon string) {
	normalized.AgentIndicator = icon
	normalized.ProfileIcon = icon
}
