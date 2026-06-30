package ws

import (
	"context"
	"fmt"
	"os"
)

// openShellDataChannel transfers files using the SDK's file client.
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

	if err := c.ws.client.Files().Upload(ctx, c.ws.sandboxID, opts.LocalPath, remotePath); err != nil {
		return newChannelError("data", "push", c.ws.name, "uploading files to sandbox", err)
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

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	if err := c.ws.client.Files().Download(ctx, c.ws.sandboxID, remotePath, localPath); err != nil {
		return newChannelError("data", "pull", c.ws.name, "downloading files from sandbox", err)
	}
	return nil
}

func (c *openShellDataChannel) PushBytes(ctx context.Context, data []byte, remotePath string) error {
	if remotePath == "" {
		return newChannelError("data", "push-bytes", c.ws.name, "remote path is required", nil)
	}
	c.ws.loadSandboxID()
	if c.ws.sandboxID == "" {
		return newChannelError("data", "push-bytes", c.ws.name, "no sandbox available", nil)
	}
	if err := c.ws.ensureClient(); err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "gateway connection failed", err)
	}

	tmpFile, err := os.CreateTemp("", "cc-deck-push-bytes-*")
	if err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "creating temp file", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return newChannelError("data", "push-bytes", c.ws.name, "writing temp file", err)
	}
	if err := tmpFile.Close(); err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "closing temp file", err)
	}

	if err := c.ws.client.Files().Upload(ctx, c.ws.sandboxID, tmpFile.Name(), remotePath); err != nil {
		return newChannelError("data", "push-bytes", c.ws.name, "writing bytes to sandbox", err)
	}
	return nil
}

// openShellGitChannel synchronizes git commits via ext:: transport
// over the openshell CLI.
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

	remoteURL := buildExtOpenShellURL(c.ws.sandboxID, workspacePath)
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
	remoteURL := buildExtOpenShellURL(c.ws.sandboxID, workspacePath)
	return gitPush(ctx, c.ws.name, remoteURL)
}

// buildExtOpenShellURL constructs an ext:: remote URL for git operations.
// Git ext:: protocol requires a command-line tool for stdin/stdout piping.
func buildExtOpenShellURL(sandboxName, workspacePath string) string {
	return fmt.Sprintf("ext::openshell sandbox exec -n %s -- %%S %s", sandboxName, workspacePath)
}
