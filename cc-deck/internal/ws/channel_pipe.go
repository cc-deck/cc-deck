package ws

import (
	"context"
	"fmt"
	"os/exec"
)

// localPipeChannel sends text to a local zellij pipe via subprocess.
type localPipeChannel struct {
	name string
}

func (c *localPipeChannel) Send(ctx context.Context, pipeName string, payload string) error {
	if pipeName == "" {
		return newChannelError("pipe", "send", c.name, "pipe name is required", nil)
	}
	cmd := exec.CommandContext(ctx, "zellij", "pipe", "--name", pipeName, "--", payload)
	if out, err := cmd.CombinedOutput(); err != nil {
		return newChannelError("pipe", "send", c.name,
			fmt.Sprintf("zellij pipe to %q: %s", pipeName, string(out)), err)
	}
	return nil
}

func (c *localPipeChannel) SendReceive(_ context.Context, _ string, _ string) (string, error) {
	return "", fmt.Errorf("pipe SendReceive: %w", ErrNotSupported)
}

// execPipeChannel sends text to a remote zellij pipe via workspace Exec.
type execPipeChannel struct {
	name    string
	execFn  func(ctx context.Context, cmd []string) error
}

func (c *execPipeChannel) Send(ctx context.Context, pipeName string, payload string) error {
	if pipeName == "" {
		return newChannelError("pipe", "send", c.name, "pipe name is required", nil)
	}
	cmd := []string{"zellij", "pipe", "--name", pipeName, "--", payload}
	if err := c.execFn(ctx, cmd); err != nil {
		return newChannelError("pipe", "send", c.name,
			fmt.Sprintf("workspace %q: pipe to %q", c.name, pipeName), err)
	}
	return nil
}

func (c *execPipeChannel) SendReceive(_ context.Context, _ string, _ string) (string, error) {
	return "", fmt.Errorf("pipe SendReceive: %w", ErrNotSupported)
}
