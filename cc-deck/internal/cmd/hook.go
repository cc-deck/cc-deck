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

Exits silently (code 0) if not inside Zellij (empty pane-id),
input is malformed, or any error occurs. Never disrupts the agent.`,
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
	cmd.Flags().StringVar(&agentName, "agent", "claude", "Agent name (claude, opencode)")
	cmd.Flags().BoolVar(&rawMode, "raw", false, "Accept pre-normalized JSON payload (skip TranslateEvent)")

	return cmd
}

// paneMapFile is the path for the session_id -> pane_id cache.
var hookStateDir = filepath.Join(xdg.StateHome, "cc-deck")

var paneMapFile = filepath.Join(hookStateDir, "pane-map.json")

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
	_ = os.MkdirAll(hookStateDir, 0700)
	data, _ := json.Marshal(m)
	_ = os.WriteFile(paneMapFile, data, 0600)
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

	var paneID uint32
	if paneIDStr != "" {
		paneID64, err := strconv.ParseUint(paneIDStr, 10, 32)
		if err != nil {
			return
		}
		paneID = uint32(paneID64)
		if normalized.SessionID != "" {
			m := loadPaneMap()
			m[normalized.SessionID] = paneID
			savePaneMap(m)
		}
	} else if normalized.SessionID != "" {
		m := loadPaneMap()
		cached, ok := m[normalized.SessionID]
		if !ok {
			return
		}
		paneID = cached
	} else {
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, zellijPath, "pipe",
		"--name", "cc-deck:hook",
		"--", string(payloadJSON))
	_ = cmd.Run()

	session.AutoSave()

	if normalized.HookEvent == "SessionEnd" && normalized.SessionID != "" {
		m := loadPaneMap()
		delete(m, normalized.SessionID)
		savePaneMap(m)
	}
}

