package session

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
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

	total := len(snap.Sessions)
	for i, entry := range snap.Sessions {
		fmt.Fprintf(w, "Creating tab %d/%d: %s...\n", i+1, total, entry.DisplayName)

		// Create a new tab (uses new_tab_template from layout)
		if err := zellijAction("new-tab"); err != nil {
			fmt.Fprintf(w, "  Warning: failed to create tab: %v\n", err)
			continue
		}

		// Brief pause for tab initialization
		time.Sleep(500 * time.Millisecond)

		// Change to the working directory, preferring git root so that
		// claude --resume finds the session in the correct project.
		if entry.WorkingDir != "" {
			dir := resolveProjectDir(entry.WorkingDir)
			writeChars(fmt.Sprintf("cd %q\n", dir))
			time.Sleep(200 * time.Millisecond)
		}

		// Start Claude with --resume, fall back to fresh start
		if entry.SessionID != "" {
			writeChars(fmt.Sprintf("claude --resume %s\n", entry.SessionID))
		} else {
			writeChars("claude\n")
		}

		// Brief pause between tabs
		time.Sleep(300 * time.Millisecond)
	}

	// Switch to first restored tab (tab 2, since tab 1 is the original)
	zellijAction("go-to-tab", "2")

	fmt.Fprintf(w, "Restored %d session(s) from snapshot %q\n", total, snap.Name)
	return nil
}

func zellijAction(args ...string) error {
	cmdArgs := append([]string{"action"}, args...)
	return exec.Command("zellij", cmdArgs...).Run()
}

func writeChars(text string) {
	exec.Command("zellij", "action", "write-chars", text).Run()
}

// resolveProjectDir returns the git root of dir if it's inside a git repo,
// otherwise returns dir unchanged. This ensures claude --resume finds
// sessions stored under the project root rather than a subdirectory.
func resolveProjectDir(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	if root := strings.TrimSpace(string(out)); root != "" {
		return root
	}
	return dir
}
