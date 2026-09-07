package ws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PipeChannel sends text payloads from the local machine to a named
// zellij pipe in the remote workspace.
type PipeChannel interface {
	Send(ctx context.Context, pipeName string, payload string) error
	SendReceive(ctx context.Context, pipeName string, payload string) (string, error)
}

// DataChannel transfers files and binary data between the local machine
// and a remote workspace.
type DataChannel interface {
	Push(ctx context.Context, opts SyncOpts) error
	Pull(ctx context.Context, opts SyncOpts) error
	PushBytes(ctx context.Context, data []byte, remotePath string) error
}

// GitChannel synchronizes git commits between local and remote
// repositories. Operations are NOT safe for concurrent use with the
// same workspace (git remote add/remove creates shared state).
type GitChannel interface {
	Fetch(ctx context.Context, opts HarvestOpts) error
	Push(ctx context.Context) error
}

// withTemporaryRemote adds a named git remote, executes fn, then
// removes the remote. Cleanup runs even if fn returns an error.
func withTemporaryRemote(ctx context.Context, remoteName, remoteURL string, fn func() error) error {
	_ = gitExecSilent(ctx, "remote", "remove", remoteName)
	if err := gitExec(ctx, "remote", "add", remoteName, remoteURL); err != nil {
		return fmt.Errorf("adding git remote %q: %w", remoteName, err)
	}
	defer func() { _ = gitExecSilent(ctx, "remote", "remove", remoteName) }()

	return fn()
}

// buildExtKubectlURL constructs an ext:: remote URL for git operations
// over kubectl exec.
func buildExtKubectlURL(ns, podName, workspacePath string, kubeconfigArgs []string) string {
	parts := []string{"ext::kubectl"}
	for i := 0; i < len(kubeconfigArgs)-1; i += 2 {
		parts = append(parts, kubeconfigArgs[i], kubeconfigArgs[i+1])
	}
	parts = append(parts, "exec", "-i", "-n", ns, podName, "--", "%S", workspacePath)
	return strings.Join(parts, " ")
}

// buildExtPodmanURL constructs an ext:: remote URL for git operations
// over podman exec.
func buildExtPodmanURL(containerName, workspacePath string) string {
	return fmt.Sprintf("ext::podman exec -i %s -- %%S %s", containerName, workspacePath)
}

func gitExec(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitExecSilent(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, "git", args...).Run()
}

// shellQuote wraps a string in single quotes with proper escaping for
// safe use in sh -c commands. Single quotes within the string are
// escaped as '\'' (end quote, escaped quote, start quote).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// currentBranch returns the name of the current git branch.
func currentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("detecting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
