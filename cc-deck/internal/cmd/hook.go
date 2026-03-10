// T023: cc-deck hook subcommand - forward Claude Code hook events to Zellij plugin

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/rhuss/cc-mux/cc-deck/internal/session"
)

// hookPayload represents the JSON structure from Claude Code hook events.
type hookPayload struct {
	SessionID string `json:"session_id,omitempty"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
}

// pipePayload is what we send to the Zellij plugin via pipe.
type pipePayload struct {
	SessionID string `json:"session_id,omitempty"`
	PaneID    uint32 `json:"pane_id"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
}

// NewHookCmd creates the hook cobra command.
func NewHookCmd() *cobra.Command {
	var paneIDStr string

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Forward Claude Code hook events to the Zellij plugin",
		Long: `Reads Claude Code hook JSON from stdin and forwards it as a pipe message
to the cc-deck Zellij plugin. Designed to be registered in
~/.claude/settings.json as a hook command.

The --pane-id flag should use shell expansion ($ZELLIJ_PANE_ID) so the
shell resolves it before the binary runs, since Claude Code strips
Zellij environment variables from hook subprocesses.

Exits silently (code 0) if not inside Zellij (empty pane-id),
input is malformed, or any error occurs. Never disrupts Claude Code.`,
		Args:   cobra.NoArgs,
		Hidden: true, // Not intended for direct user invocation
		Run: func(cmd *cobra.Command, _ []string) {
			runHook(os.Stdin, paneIDStr)
		},
	}

	cmd.Flags().StringVar(&paneIDStr, "pane-id", "", "Zellij pane ID (use $ZELLIJ_PANE_ID for shell expansion)")

	return cmd
}

// paneMapFile is the path for the session_id -> pane_id cache.
// Claude Code strips ZELLIJ env vars from hook subprocesses, so $ZELLIJ_PANE_ID
// is often empty. When we DO get a pane_id, we cache it keyed by session_id so
// subsequent events for the same session can recover the pane_id.
var paneMapFile = filepath.Join(os.TempDir(), "cc-deck-pane-map.json")

func loadPaneMap() map[string]uint32 {
	data, err := os.ReadFile(paneMapFile)
	if err != nil {
		return make(map[string]uint32)
	}
	var m map[string]uint32
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]uint32)
	}
	return m
}

func savePaneMap(m map[string]uint32) {
	data, _ := json.Marshal(m)
	_ = os.WriteFile(paneMapFile, data, 0644)
}

func runHook(stdin io.Reader, paneIDStr string) {
	// Check if zellij CLI is available first (fast exit if not in Zellij)
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return
	}

	// Read hook JSON from stdin
	var hook hookPayload
	decoder := json.NewDecoder(stdin)
	if err := decoder.Decode(&hook); err != nil {
		return
	}

	if hook.HookEvent == "" {
		return
	}

	// Resolve pane_id: prefer flag, fallback to session_id cache
	var paneID uint32
	if paneIDStr != "" {
		paneID64, err := strconv.ParseUint(paneIDStr, 10, 32)
		if err != nil {
			return
		}
		paneID = uint32(paneID64)
		// Cache successful mapping for future calls
		if hook.SessionID != "" {
			m := loadPaneMap()
			m[hook.SessionID] = paneID
			savePaneMap(m)
		}
	} else if hook.SessionID != "" {
		// Pane ID missing (CC stripped env vars). Try cache lookup.
		m := loadPaneMap()
		cached, ok := m[hook.SessionID]
		if !ok {
			return
		}
		paneID = cached
	} else {
		// No pane_id and no session_id: nothing we can do
		return
	}

	// Build pipe payload
	payload := pipePayload{
		SessionID: hook.SessionID,
		PaneID:    paneID,
		HookEvent: hook.HookEvent,
		ToolName:  hook.ToolName,
		CWD:       hook.CWD,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Send via broadcast pipe (like zellij-attention does).
	// zellij pipe --name sends to all listening plugins in the current session.
	cmd := exec.Command(zellijPath, "pipe",
		"--name", "cc-deck:hook",
		"--", string(payloadJSON))
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cc-deck hook: zellij pipe failed: %v\n", err)
	}

	// Auto-save session state (5-minute cooldown, rolling retention)
	go func() {
		session.AutoSave()
	}()

	// Clean up cache on session end
	if (hook.HookEvent == "Stop" || hook.HookEvent == "SessionEnd") && hook.SessionID != "" {
		m := loadPaneMap()
		delete(m, hook.SessionID)
		savePaneMap(m)
	}
}

// FormatHookUsage returns help text showing how to register the hook.
func FormatHookUsage() string {
	return fmt.Sprintf(`To register hooks in ~/.claude/settings.json:

  cc-deck plugin install

Or manually add to settings.json:

  {
    "hooks": {
      "PreToolUse": [{"matcher": "", "hooks": [{"type": "command", "command": "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""}]}],
      "PostToolUse": [{"matcher": "", "hooks": [{"type": "command", "command": "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""}]}],
      "Notification": [{"matcher": "", "hooks": [{"type": "command", "command": "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""}]}],
      "Stop": [{"hooks": [{"type": "command", "command": "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""}]}],
      "SubagentStop": [{"hooks": [{"type": "command", "command": "cc-deck hook --pane-id \"$ZELLIJ_PANE_ID\""}]}]
    }
  }
`)
}
