package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
)

// Restore recreates tabs and starts Claude sessions from a saved snapshot.
// If name is empty, the most recent snapshot is used.
func Restore(name string, w io.Writer) error {
	var snap *Snapshot
	var err error

	if name == "" {
		snap, err = LatestSnapshot()
	} else {
		snap, err = LoadSnapshot(name)
	}
	if err != nil {
		return err
	}

	if len(snap.Sessions) == 0 {
		fmt.Fprintln(w, "Snapshot has no sessions to restore.")
		return nil
	}

	// Load config for profile-aware launch commands.
	cfg, _ := config.Load("")

	// Send pending metadata overrides to the plugin before creating tabs.
	// Keyed by resolved working directory so the plugin can match when
	// sessions start and report their CWD via hook events.
	sendPendingOverrides(snap.Sessions, cfg)

	// Track which directories have already had a session started so we
	// can add extra delay between sessions sharing the same directory.
	// Claude Code needs time to fully initialize before a second instance
	// can start in the same project directory.
	startedDirs := make(map[string]bool)

	total := len(snap.Sessions)
	for i, entry := range snap.Sessions {
		fmt.Fprintf(w, "Creating tab %d/%d: %s...\n", i+1, total, entry.DisplayName)

		// Create a new tab (uses new_tab_template from layout)
		if err := zellijAction("new-tab"); err != nil {
			fmt.Fprintf(w, "  Warning: failed to create tab: %v\n", err)
			continue
		}

		// Wait for the plugin on the new tab to be fully initialized
		// (WASM loaded, permissions granted, pipe handler active).
		// Uses the dump-state pipe as a readiness probe: if the plugin
		// responds, it is ready to handle events.
		waitForPluginReady(3 * time.Second)

		// If another session was already started in this directory,
		// wait for the previous Claude instance to finish initializing.
		// Without this delay, the second instance may fail to start
		// due to project-level initialization conflicts.
		if entry.WorkingDir != "" && startedDirs[entry.WorkingDir] {
			fmt.Fprintf(w, "  Waiting for previous session in %s to initialize...\n",
				entry.WorkingDir)
			time.Sleep(5 * time.Second)
		}

		// Change to the original working directory where the session ran.
		if entry.WorkingDir != "" {
			if _, err := os.Stat(entry.WorkingDir); err == nil {
				writeChars(fmt.Sprintf("cd %q\n", entry.WorkingDir))
				time.Sleep(200 * time.Millisecond)
			} else {
				fmt.Fprintf(w, "  Warning: directory %s no longer exists, starting fresh session\n", entry.WorkingDir)
				entry.SessionID = ""
			}
		}

		// Build the launch command for this entry, respecting agent and
		// profile fields when present.
		cmd, warning := launchCommand(entry, cfg)
		if warning != "" {
			fmt.Fprintf(w, "  Warning: %s\n", warning)
		}
		writeChars(cmd + "\n")

		if entry.WorkingDir != "" {
			startedDirs[entry.WorkingDir] = true
		}

		// Brief pause between tabs
		time.Sleep(200 * time.Millisecond)
	}

	// Switch to first restored tab (tab 2, since tab 1 is the original)
	zellijAction("go-to-tab", "2")

	fmt.Fprintf(w, "Restored %d session(s) from snapshot %q\n", total, snap.Name)
	return nil
}

// launchCommand builds the shell command string for restoring a session entry.
// It resolves the correct binary or wrapper based on the entry's Agent and
// Profile fields, appends ResumeArgs when a session ID is available, and
// returns a warning when a profile referenced in the snapshot no longer exists.
func launchCommand(entry SessionEntry, cfg *config.Config) (cmd string, warning string) {
	agentName := entry.Agent
	if agentName == "" {
		agentName = "claude"
	}
	a := agent.Get(agentName)
	if a == nil {
		// Unknown agent, fall back to claude.
		a = agent.Get("claude")
		if a == nil {
			// Absolute fallback: bare command.
			if entry.SessionID != "" {
				return "claude --resume " + entry.SessionID, fmt.Sprintf("unknown agent %q, falling back to claude", agentName)
			}
			return "claude", fmt.Sprintf("unknown agent %q, falling back to claude", agentName)
		}
		warning = fmt.Sprintf("unknown agent %q, falling back to %s", agentName, a.Binary())
	}

	binary := a.Binary()

	// When a profile is set, try to use the wrapper command.
	if entry.Profile != "" && cfg != nil {
		if _, err := cfg.GetProfile(entry.Profile); err == nil {
			// Profile exists: use the wrapper name (binary-profilename).
			binary = a.Binary() + "-" + entry.Profile
		} else {
			// Profile referenced in snapshot no longer exists in config.
			warning = fmt.Sprintf("profile %q not found in config, using plain %s", entry.Profile, a.Binary())
		}
	}

	// Build the full command with resume args.
	args := a.ResumeArgs(entry.SessionID)
	if len(args) > 0 {
		return binary + " " + strings.Join(args, " "), warning
	}
	return binary, warning
}

// waitForPluginReady polls the plugin using dump-state until it responds,
// proving the WASM is loaded, permissions are granted, and the pipe handler
// is active. Falls back to a fixed delay after timeout.
func waitForPluginReady(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		_, err := queryPluginCtx(ctx)
		if err == nil {
			return
		}

		select {
		case <-ctx.Done():
			// Timed out waiting for plugin; use a fallback delay
			time.Sleep(500 * time.Millisecond)
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func zellijAction(args ...string) error {
	cmdArgs := append([]string{"action"}, args...)
	return exec.Command("zellij", cmdArgs...).Run()
}

func writeChars(text string) {
	exec.Command("zellij", "action", "write-chars", text).Run()
}

// pendingOverride is the JSON structure sent to the plugin for restore metadata.
type pendingOverride struct {
	DisplayName  string `json:"display_name"`
	Paused       bool   `json:"paused"`
	Profile      string `json:"profile,omitempty"`
	ProfileColor string `json:"profile_color,omitempty"`
}

// sendPendingOverrides pipes session metadata to the plugin so custom names,
// paused state, and profile information survive restore. Keyed by resolved
// working directory, with a list per directory to support multiple sessions
// sharing the same dir.
func sendPendingOverrides(sessions []SessionEntry, cfg *config.Config) {
	overrides := make(map[string][]pendingOverride)
	for _, entry := range sessions {
		if entry.WorkingDir == "" {
			continue
		}
		po := pendingOverride{
			DisplayName: entry.DisplayName,
			Paused:      entry.Paused,
			Profile:     entry.Profile,
		}
		// Resolve the profile color from config if the profile still exists.
		if entry.Profile != "" && cfg != nil {
			if p, err := cfg.GetProfile(entry.Profile); err == nil {
				color := p.Color
				if color == "" {
					color = profile.Derive(entry.Profile)
				}
				po.ProfileColor = color
			}
		}
		overrides[entry.WorkingDir] = append(overrides[entry.WorkingDir], po)
	}
	if len(overrides) == 0 {
		return
	}
	data, err := json.Marshal(overrides)
	if err != nil {
		return
	}
	exec.Command("zellij", "pipe", "--name", "cc-deck:restore-meta", "--", string(data)).Run()
}
