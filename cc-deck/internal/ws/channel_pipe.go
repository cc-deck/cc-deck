package ws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// localPipeChannel sends text to a local zellij pipe via subprocess.
type localPipeChannel struct {
	name string
}

func (c *localPipeChannel) zellijSessionName() string {
	return zellijSessionPrefix + c.name
}

func (c *localPipeChannel) pipeCmd(ctx context.Context, pipeName string, payload string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "zellij", "pipe", "--name", pipeName, "--", payload)
	cmd.Env = append(os.Environ(), "ZELLIJ_SESSION_NAME="+c.zellijSessionName())
	return cmd
}

func (c *localPipeChannel) Send(ctx context.Context, pipeName string, payload string) error {
	if pipeName == "" {
		return newChannelError("pipe", "send", c.name, "pipe name is required", nil)
	}
	cmd := c.pipeCmd(ctx, pipeName, payload)
	if out, err := cmd.CombinedOutput(); err != nil {
		return newChannelError("pipe", "send", c.name,
			fmt.Sprintf("zellij pipe to %q: %s", pipeName, string(out)), err)
	}
	return nil
}

func (c *localPipeChannel) SendReceive(ctx context.Context, pipeName string, payload string) (string, error) {
	if pipeName == "" {
		return "", newChannelError("pipe", "sendReceive", c.name, "pipe name is required", nil)
	}
	cmd := c.pipeCmd(ctx, pipeName, payload)
	out, err := cmd.Output()
	if err != nil {
		return "", newChannelError("pipe", "sendReceive", c.name,
			fmt.Sprintf("zellij pipe to %q", pipeName), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// execPipeChannel sends text to a remote zellij pipe via workspace Exec.
type execPipeChannel struct {
	name          string
	execFn        func(ctx context.Context, cmd []string) error
	execOutputFn  func(ctx context.Context, cmd []string) (string, error)
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

func (c *execPipeChannel) SendReceive(ctx context.Context, pipeName string, payload string) (string, error) {
	if pipeName == "" {
		return "", newChannelError("pipe", "sendReceive", c.name, "pipe name is required", nil)
	}
	if c.execOutputFn == nil {
		return "", fmt.Errorf("pipe SendReceive: %w", ErrNotSupported)
	}
	cmd := []string{"zellij", "pipe", "--name", pipeName, "--", payload}
	out, err := c.execOutputFn(ctx, cmd)
	if err != nil {
		return "", newChannelError("pipe", "sendReceive", c.name,
			fmt.Sprintf("workspace %q: pipe to %q", c.name, pipeName), err)
	}
	return strings.TrimSpace(out), nil
}
