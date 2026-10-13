package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/badge"
	"github.com/cc-deck/cc-deck/internal/config"
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
type paneMapEntry struct {
	PaneID        uint32 `json:"pane_id"`
	ZellijSession string `json:"zellij_session"`
	UpdatedAt     int64  `json:"updated_at"`
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

func savePaneMap(m map[string]paneMapEntry) {
	_ = os.MkdirAll(hookStateDir, 0700)
	data, _ := json.Marshal(m)
	_ = os.WriteFile(paneMapFile, data, 0600)
}

// paneMapRefresh is how old a still-correct cache entry may get before its
// timestamp is renewed. Between refreshes an unchanged mapping costs no
// write, which matters because this runs on every hook event.
const paneMapRefresh = time.Hour

// recordPaneMapping stores the session's pane, writing the cache only when
// the mapping is new, changed, or due for a timestamp refresh.
func recordPaneMapping(sessionID string, paneID uint32, zellijSession string) {
	m := loadPaneMap()
	now := time.Now()
	if cur, ok := m[sessionID]; ok &&
		cur.PaneID == paneID &&
		cur.ZellijSession == zellijSession &&
		now.Unix()-cur.UpdatedAt < int64(paneMapRefresh.Seconds()) {
		return
	}
	m[sessionID] = paneMapEntry{
		PaneID:        paneID,
		ZellijSession: zellijSession,
		UpdatedAt:     now.Unix(),
	}
	savePaneMap(m)
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
// A package-level variable so tests can observe what would have been sent
// without shelling out to zellij.
var sendHookPayload = func(zellijPath string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, zellijPath, "pipe",
		"--name", "cc-deck:hook",
		"--", string(payload))
	_ = cmd.Run()
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

	// Profile enrichment: when CC_DECK_PROFILE is set, attach profile name
	// and color to the payload so the sidebar can distinguish sessions.
	if profileName := os.Getenv("CC_DECK_PROFILE"); profileName != "" {
		enrichProfile(normalized, profileName)
	}

	// An agent running outside Zellij has no pane and so no sidebar row to
	// own. Stopping here is what keeps it out: `zellij pipe` succeeds from
	// outside a session and delivers to whichever session is running, so
	// without this check an agent in a plain terminal reaches the sidebar of
	// an unrelated Zellij session and stays there.
	zellijSession, inZellij := currentZellijSession()
	if !inZellij {
		logHookEnv(normalized.HookEvent, paneIDStr, "not-in-zellij")
		return
	}

	var paneID uint32
	if paneIDStr != "" {
		paneID64, err := strconv.ParseUint(paneIDStr, 10, 32)
		if err != nil {
			logHookEnv(normalized.HookEvent, paneIDStr, "bad-pane-id-arg")
			return
		}
		paneID = uint32(paneID64)
		logHookEnv(normalized.HookEvent, paneIDStr, "from-arg")
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
			logHookEnv(normalized.HookEvent, paneIDStr, "no-cache-entry")
			return
		}
		if cached.ZellijSession != zellijSession {
			// The mapping was recorded by a different Zellij session, where
			// this pane id meant something else entirely.
			logHookEnv(normalized.HookEvent, paneIDStr, "cache-session-mismatch")
			return
		}
		paneID = cached.PaneID
		logHookEnv(normalized.HookEvent, paneIDStr, "from-cache")
	} else {
		logHookEnv(normalized.HookEvent, paneIDStr, "no-pane-id-no-session-id")
		return
	}

	normalized.PaneID = paneID

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

	sendHookPayload(zellijPath, payloadJSON)

	session.AutoSave()

	if normalized.HookEvent == "SessionEnd" && normalized.SessionID != "" {
		m := loadPaneMap()
		delete(m, normalized.SessionID)
		savePaneMap(m)
	}
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
	normalized.Profile = profileName

	if envColor := os.Getenv("CC_DECK_PROFILE_COLOR"); envColor != "" {
		if _, _, _, err := profile.ParseHex(envColor); err == nil {
			normalized.ProfileColor = envColor
			// The icon reaches the sidebar unescaped, so an environment value
			// that config validation never saw must pass the same check.
			if icon := os.Getenv("CC_DECK_PROFILE_ICON"); config.ValidIcon(icon) {
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
