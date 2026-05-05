package ws

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

// openShellDataChannel transfers files via SSH tunnel into OpenShell sandboxes.
type openShellDataChannel struct {
	ws *OpenShellWorkspace
}

func (c *openShellDataChannel) Push(ctx context.Context, opts SyncOpts) error {
	if opts.LocalPath == "" {
		return newChannelError("data", "push", c.ws.name, "local path is required", nil)
	}
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("data", "push", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("data", "push", c.ws.name, "gateway connection failed", err)
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = "/sandbox"
	}

	tarArgs := buildTarArgs("cf", opts.LocalPath, opts.Excludes)
	tarCmd := exec.CommandContext(ctx, "tar", tarArgs...)
	tarOut, err := tarCmd.Output()
	if err != nil {
		return newChannelError("data", "push", c.ws.name, "creating tar archive", err)
	}

	encoded := base64.StdEncoding.EncodeToString(tarOut)
	shellCmd := fmt.Sprintf("echo %s | base64 -d | tar xf - -C %s",
		shellQuote(encoded), shellQuote(remotePath))
	_, err = c.ws.client.ExecSandbox(ctx, c.ws.sandboxID, []string{"sh", "-c", shellCmd})
	if err != nil {
		return newChannelError("data", "push", c.ws.name, "extracting tar in sandbox", err)
	}
	return nil
}

func (c *openShellDataChannel) Pull(ctx context.Context, opts SyncOpts) error {
	remotePath := opts.RemotePath
	if remotePath == "" {
		return newChannelError("data", "pull", c.ws.name, "remote path is required", nil)
	}
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("data", "pull", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("data", "pull", c.ws.name, "gateway connection failed", err)
	}

	tarCmd := buildRemoteTarArgs(remotePath, opts.Excludes)
	shellCmd := fmt.Sprintf("%s | base64", strings.Join(tarCmd, " "))
	result, err := c.ws.client.ExecSandbox(ctx, c.ws.sandboxID, []string{"sh", "-c", shellCmd})
	if err != nil {
		return newChannelError("data", "pull", c.ws.name, "creating tar in sandbox", err)
	}

	tarData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return newChannelError("data", "pull", c.ws.name, "decoding tar data", err)
	}

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	extractCmd := exec.CommandContext(ctx, "tar", "xf", "-", "-C", localPath)
	extractCmd.Stdin = strings.NewReader(string(tarData))
	if err := extractCmd.Run(); err != nil {
		return newChannelError("data", "pull", c.ws.name, "extracting tar locally", err)
	}
	return nil
}

func (c *openShellDataChannel) PushBytes(ctx context.Context, data []byte, remotePath string) error {
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("data", "push-bytes", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "gateway connection failed", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	shellCmd := fmt.Sprintf("echo %s | base64 -d > %s",
		shellQuote(encoded), shellQuote(remotePath))
	_, err := c.ws.client.ExecSandbox(ctx, c.ws.sandboxID, []string{"sh", "-c", shellCmd})
	if err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "writing bytes to sandbox", err)
	}
	return nil
}

// buildTarArgs constructs tar arguments with excludes placed before positional args.
func buildTarArgs(mode, dir string, excludes []string) []string {
	var args []string
	for _, exc := range excludes {
		args = append(args, "--exclude="+exc)
	}
	args = append(args, mode, "-", "-C", dir, ".")
	return args
}

// buildRemoteTarArgs constructs a remote tar command with excludes.
func buildRemoteTarArgs(remotePath string, excludes []string) []string {
	args := []string{"tar"}
	for _, exc := range excludes {
		args = append(args, "--exclude="+exc)
	}
	args = append(args, "cf", "-", "-C", remotePath, ".")
	return args
}

// openShellGitChannel synchronizes git commits via ext:: transport over SSH.
type openShellGitChannel struct {
	ws *OpenShellWorkspace
}

func (c *openShellGitChannel) Fetch(ctx context.Context, opts HarvestOpts) error {
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("git", "fetch", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("git", "fetch", c.ws.name, "gateway connection failed", err)
	}

	workspacePath := "/sandbox"
	if opts.Path != "" {
		workspacePath = opts.Path
	}

	remoteURL := buildExtOpenShellURL(c.ws.client.Address(), c.ws.sandboxID, workspacePath)
	return gitFetch(ctx, c.ws.name, remoteURL, opts)
}

func (c *openShellGitChannel) Push(ctx context.Context) error {
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("git", "push", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("git", "push", c.ws.name, "gateway connection failed", err)
	}

	workspacePath := "/sandbox"
	remoteURL := buildExtOpenShellURL(c.ws.client.Address(), c.ws.sandboxID, workspacePath)
	return gitPush(ctx, c.ws.name, remoteURL)
}

// buildExtOpenShellURL constructs an ext:: remote URL for git operations
// over the OpenShell CLI exec transport.
func buildExtOpenShellURL(gateway, sandboxID, workspacePath string) string {
	return fmt.Sprintf("ext::openshell sandbox exec --gateway %s %s -- %%S %s",
		gateway, sandboxID, workspacePath)
}
