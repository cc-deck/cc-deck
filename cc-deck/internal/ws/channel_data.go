package ws

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cc-deck/cc-deck/internal/podman"
	"github.com/cc-deck/cc-deck/internal/ssh"
)

// localDataChannel copies files on the local filesystem.
type localDataChannel struct {
	name string
}

func (c *localDataChannel) Push(_ context.Context, opts SyncOpts) error {
	if opts.LocalPath == "" {
		return newChannelError("data", "push", c.name, "local path is required", nil)
	}
	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = "./" + baseNameFromPath(opts.LocalPath)
	}
	if err := copyPath(opts.LocalPath, remotePath); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("copy %q to %q", opts.LocalPath, remotePath), err)
	}
	return nil
}

func (c *localDataChannel) Pull(_ context.Context, opts SyncOpts) error {
	if opts.RemotePath == "" {
		return newChannelError("data", "pull", c.name, "remote path is required", nil)
	}
	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}
	if err := copyPath(opts.RemotePath, localPath); err != nil {
		return newChannelError("data", "pull", c.name,
			fmt.Sprintf("copy %q to %q", opts.RemotePath, localPath), err)
	}
	return nil
}

func (c *localDataChannel) PushBytes(_ context.Context, data []byte, remotePath string) error {
	if remotePath == "" {
		return newChannelError("data", "push", c.name, "remote path is required", nil)
	}
	dir := filepath.Dir(remotePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("creating directory %q", dir), err)
	}
	if err := os.WriteFile(remotePath, data, 0o644); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("writing %q", remotePath), err)
	}
	return nil
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstInfo, statErr := os.Stat(dst)
	if statErr == nil && dstInfo.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

// podmanDataChannel transfers files via podman cp.
type podmanDataChannel struct {
	name          string
	containerName func() string
}

func (c *podmanDataChannel) Push(ctx context.Context, opts SyncOpts) error {
	if opts.LocalPath == "" {
		return newChannelError("data", "push", c.name, "local path is required", nil)
	}
	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = "/workspace/" + baseNameFromPath(opts.LocalPath)
	}
	if err := podman.Cp(ctx, opts.LocalPath, c.containerName()+":"+remotePath); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("podman cp to %q", remotePath), err)
	}
	return nil
}

func (c *podmanDataChannel) Pull(ctx context.Context, opts SyncOpts) error {
	if opts.RemotePath == "" {
		return newChannelError("data", "pull", c.name, "remote path is required", nil)
	}
	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}
	if err := podman.Cp(ctx, c.containerName()+":"+opts.RemotePath, localPath); err != nil {
		return newChannelError("data", "pull", c.name,
			fmt.Sprintf("podman cp from %q", opts.RemotePath), err)
	}
	return nil
}

func (c *podmanDataChannel) PushBytes(ctx context.Context, data []byte, remotePath string) error {
	if remotePath == "" {
		return newChannelError("data", "push", c.name, "remote path is required", nil)
	}
	cmd := exec.CommandContext(ctx, "podman", "exec", "-i", c.containerName(), "sh", "-c",
		fmt.Sprintf("cat > %s", remotePath))
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("podman exec: %s", strings.TrimSpace(string(out))), err)
	}
	return nil
}

// k8sDataChannel transfers files via tar-over-kubectl-exec.
type k8sDataChannel struct {
	name           string
	ns             string
	podName        string
	kubeconfigArgs []string
}

func (c *k8sDataChannel) Push(ctx context.Context, opts SyncOpts) error {
	return k8sPush(ctx, c.ns, c.podName, c.kubeconfigArgs, opts)
}

func (c *k8sDataChannel) Pull(ctx context.Context, opts SyncOpts) error {
	return k8sPull(ctx, c.ns, c.podName, c.kubeconfigArgs, opts)
}

func (c *k8sDataChannel) PushBytes(ctx context.Context, data []byte, remotePath string) error {
	if remotePath == "" {
		return newChannelError("data", "push", c.name, "remote path is required", nil)
	}
	args := append(c.kubeconfigArgs, "exec", "-i", "-n", c.ns, c.podName, "--", "sh", "-c",
		fmt.Sprintf("cat > %s", remotePath))
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("kubectl exec: %s", strings.TrimSpace(string(out))), err)
	}
	return nil
}

// sshDataChannel transfers files via rsync/scp over SSH.
type sshDataChannel struct {
	name      string
	clientFn  func() *ssh.Client
	workspace func(context.Context) (string, error)
}

func (c *sshDataChannel) Push(ctx context.Context, opts SyncOpts) error {
	if opts.LocalPath == "" {
		return newChannelError("data", "push", c.name, "local path is required", nil)
	}
	client := c.clientFn()
	remotePath := opts.RemotePath
	if remotePath == "" {
		resolved, err := c.workspace(ctx)
		if err != nil {
			return newChannelError("data", "push", c.name, "resolving workspace path", err)
		}
		remotePath = resolved
	}
	if err := client.Rsync(ctx, opts.LocalPath, client.Host+":"+remotePath, opts.Excludes, true); err != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("rsync to %s:%s", client.Host, remotePath), err)
	}
	return nil
}

func (c *sshDataChannel) Pull(ctx context.Context, opts SyncOpts) error {
	if opts.RemotePath == "" {
		return newChannelError("data", "pull", c.name, "remote path is required", nil)
	}
	client := c.clientFn()
	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}
	if err := client.Rsync(ctx, client.Host+":"+opts.RemotePath, localPath, opts.Excludes, false); err != nil {
		return newChannelError("data", "pull", c.name,
			fmt.Sprintf("rsync from %s:%s", client.Host, opts.RemotePath), err)
	}
	return nil
}

func (c *sshDataChannel) PushBytes(ctx context.Context, data []byte, remotePath string) error {
	if remotePath == "" {
		return newChannelError("data", "push", c.name, "remote path is required", nil)
	}
	client := c.clientFn()
	remoteCmd := fmt.Sprintf("cat > %q", remotePath)
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return newChannelError("data", "push", c.name, "ssh binary not found", err)
	}
	args := sshArgs(client)
	args = append(args, client.Host, "--", remoteCmd)
	cmd := exec.CommandContext(ctx, sshBin, args...)
	cmd.Stdin = bytes.NewReader(data)
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		return newChannelError("data", "push", c.name,
			fmt.Sprintf("SSH push bytes to %q: %s", remotePath, strings.TrimSpace(string(out))), runErr)
	}
	return nil
}

func sshArgs(c *ssh.Client) []string {
	var args []string
	if c.SSHConfig != "" {
		args = append(args, "-F", c.SSHConfig)
	}
	if c.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.JumpHost != "" {
		args = append(args, "-J", c.JumpHost)
	}
	args = append(args, "-o", "StrictHostKeyChecking=accept-new", "-o", "BatchMode=yes")
	return args
}
