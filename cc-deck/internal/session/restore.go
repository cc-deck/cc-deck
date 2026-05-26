package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
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

	// Send pending metadata overrides to the plugin before creating tabs.
	// Keyed by resolved working directory so the plugin can match when
	// sessions start and report their CWD via hook events.
	sendPendingOverrides(snap.Sessions)

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

		// Change to the original working directory where the session ran.
		if entry.WorkingDir != "" {
			writeChars(fmt.Sprintf("cd %q\n", entry.WorkingDir))
			time.Sleep(200 * time.Millisecond)
		}

		// Start Claude with --resume, fall back to fresh start
		if entry.SessionID != "" {
			writeChars(fmt.Sprintf("claude --resume %s\n", entry.SessionID))
		} else {
			writeChars("claude\n")
		}

		// Brief pause between tabs
		time.Sleep(200 * time.Millisecond)
	}

	// Switch to first restored tab (tab 2, since tab 1 is the original)
	zellijAction("go-to-tab", "2")

	fmt.Fprintf(w, "Restored %d session(s) from snapshot %q\n", total, snap.Name)
	return nil
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
	DisplayName string `json:"display_name"`
	Paused      bool   `json:"paused"`
}

// sendPendingOverrides pipes session metadata to the plugin so custom names
// and paused state survive restore. Keyed by resolved working directory,
// with a list per directory to support multiple sessions sharing the same dir.
func sendPendingOverrides(sessions []SessionEntry) {
	overrides := make(map[string][]pendingOverride)
	for _, entry := range sessions {
		if entry.WorkingDir == "" {
			continue
		}
		overrides[entry.WorkingDir] = append(overrides[entry.WorkingDir], pendingOverride{
			DisplayName: entry.DisplayName,
			Paused:      entry.Paused,
		})
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
