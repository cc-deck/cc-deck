package podman

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrPodmanNotFound = errors.New("podman binary not found in PATH")

// Available returns true if podman is installed and in PATH.
func Available() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// IsRootless returns true if podman is running in rootless mode.
func IsRootless(ctx context.Context) (bool, error) {
	out, err := run(ctx, "info", "--format", "{{.Host.Security.Rootless}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// run executes a podman command and returns trimmed stdout.
func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman %s: %w (%s)", args[0], err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
