package ws

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// deleteContainerZellijSession tolerates a session disappearing during deletion.
// Zellij can kill the server successfully, then fail to delete saved state that
// the server had not written yet. Only a confirmed absence makes that success.
func deleteContainerZellijSession(ctx context.Context, container, session string) error {
	out, err := exec.CommandContext(ctx, "podman", "exec", container,
		"zellij", "delete-session", "--force", session).CombinedOutput()
	if err == nil {
		return nil
	}

	listed, listErr := exec.CommandContext(ctx, "podman", "exec", container,
		"zellij", "list-sessions", "-n").CombinedOutput()
	if listErr == nil && !slices.Contains(parseZellijSessionNames(string(listed)), session) {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(listErr, &exitErr) && exitErr.ExitCode() == 1 &&
		strings.TrimSpace(string(listed)) == "No active zellij sessions found." {
		return nil
	}
	return fmt.Errorf("deleting session: %w (%s)", err, strings.TrimSpace(string(out)))
}
