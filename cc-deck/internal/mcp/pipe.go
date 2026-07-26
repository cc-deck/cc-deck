package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PipeSender is a function type for sending pipe messages to the Zellij plugin.
// This abstraction allows for testability by substituting the real pipe sender
// with a mock in tests.
type PipeSender func(ctx context.Context, name string, payload string) (string, error)

// SendPipe sends a named pipe message to the cc-deck Zellij plugin and returns
// the response. The message is sent via `zellij pipe` with the given name and
// payload. Context is used for timeout control.
func SendPipe(ctx context.Context, name string, payload string) (string, error) {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return "", fmt.Errorf("cc-deck plugin not running")
	}

	args := []string{"pipe", "--name", name}
	if payload != "" {
		args = append(args, "--", payload)
	}

	cmd := exec.CommandContext(ctx, zellijPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("cc-deck plugin not running: %s", stderrStr)
		}
		return "", fmt.Errorf("cc-deck plugin not running")
	}

	return strings.TrimSpace(stdout.String()), nil
}
