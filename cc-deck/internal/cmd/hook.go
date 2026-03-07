// T023: cc-deck hook subcommand - forward Claude Code hook events to Zellij plugin

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

// hookPayload represents the JSON structure from Claude Code hook events.
type hookPayload struct {
	SessionID string `json:"session_id,omitempty"`
	HookEvent string `json:"hook_event"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
}

// pipePayload is what we send to the Zellij plugin via pipe.
type pipePayload struct {
	SessionID string `json:"session_id,omitempty"`
	PaneID    uint32 `json:"pane_id"`
	HookEvent string `json:"hook_event"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
}

// NewHookCmd creates the hook cobra command.
func NewHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "Forward Claude Code hook events to the Zellij plugin",
		Long: `Reads Claude Code hook JSON from stdin and forwards it as a pipe message
to the cc-deck Zellij plugin. Designed to be registered in
~/.claude/settings.json as a hook command.

Exits silently (code 0) if Zellij is not running, input is malformed,
or any error occurs. Never disrupts Claude Code operation.`,
		Args:   cobra.NoArgs,
		Hidden: true, // Not intended for direct user invocation
		Run: func(cmd *cobra.Command, _ []string) {
			runHook(os.Stdin)
		},
	}
}

func runHook(stdin io.Reader) {
	// Check if we're inside Zellij
	paneIDStr := os.Getenv("ZELLIJ_PANE_ID")
	if paneIDStr == "" {
		return // Not in Zellij, exit silently
	}

	paneID, err := strconv.ParseUint(paneIDStr, 10, 32)
	if err != nil {
		return // Invalid pane ID, exit silently
	}

	// Check if zellij CLI is available
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return // zellij not on PATH, exit silently
	}

	// Read hook JSON from stdin
	var hook hookPayload
	decoder := json.NewDecoder(stdin)
	if err := decoder.Decode(&hook); err != nil {
		return // Malformed JSON, exit silently
	}

	if hook.HookEvent == "" {
		return // No event, exit silently
	}

	// Build pipe payload
	payload := pipePayload{
		SessionID: hook.SessionID,
		PaneID:    uint32(paneID),
		HookEvent: hook.HookEvent,
		ToolName:  hook.ToolName,
		CWD:       hook.CWD,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return // Encoding error, exit silently
	}

	// Send to Zellij plugin via pipe
	cmd := exec.Command(zellijPath, "pipe", "--name", "cc-deck:hook", "--", string(payloadJSON))
	// Ignore errors - never disrupt Claude Code
	_ = cmd.Run()
}

// FormatHookUsage returns help text showing how to register the hook.
func FormatHookUsage() string {
	return fmt.Sprintf(`To register hooks in ~/.claude/settings.json:

  cc-deck plugin install

Or manually add to settings.json:

  {
    "hooks": {
      "PreToolUse": ["cc-deck hook"],
      "PostToolUse": ["cc-deck hook"],
      "Notification": ["cc-deck hook"],
      "Stop": ["cc-deck hook"],
      "SubagentStop": ["cc-deck hook"]
    }
  }
`)
}
