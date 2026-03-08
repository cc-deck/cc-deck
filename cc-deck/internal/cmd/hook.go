// T023: cc-deck hook subcommand - forward Claude Code hook events to Zellij plugin

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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

func runHook(stdin io.Reader, paneIDStr string) {
	// Lightweight trace log for debugging hook delivery
	if f, err := os.OpenFile("/tmp/cc-deck-hook.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(f, "pane=%q zellij_env=", paneIDStr)
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "ZELLIJ") {
				fmt.Fprintf(f, "%s ", e)
			}
		}
		fmt.Fprintln(f)
		f.Close()
	}

	// If pane_id is empty or not a number, we're not inside Zellij. Exit silently.
	if paneIDStr == "" {
		return
	}
	paneID64, err := strconv.ParseUint(paneIDStr, 10, 32)
	if err != nil {
		return
	}
	paneID := uint32(paneID64)

	// Check if zellij CLI is available
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cc-deck hook: zellij command not found on PATH, skipping")
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
