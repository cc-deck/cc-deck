package ws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateSyncPath checks that a path is clean and absolute (for remote) or valid (for local).
func validateSyncPath(p string) error {
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path %q contains directory traversal", p)
	}
	return nil
}

// k8sPush transfers local files into the K8s Pod via tar-over-exec.
func k8sPush(ctx context.Context, ns, podName string, kubeconfigArgs []string, opts SyncOpts) error {
	localPath := opts.LocalPath
	if localPath == "" {
		return fmt.Errorf("local path is required for push")
	}
	if err := validateSyncPath(localPath); err != nil {
		return err
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = k8sWorkspacePath
	}
	if err := validateSyncPath(remotePath); err != nil {
		return err
	}

	if opts.UseGit {
		ch := &k8sGitChannel{
			name:           podName,
			ns:             ns,
			podName:        podName,
			kubeconfigArgs: kubeconfigArgs,
			workspacePath:  k8sWorkspacePath,
		}
		return ch.Push(ctx)
	}

	// Tar local files and pipe via kubectl exec.
	tarCmd := exec.CommandContext(ctx, "tar", "cf", "-", "-C", localPath, ".")
	kubectlArgs := append(append([]string(nil), kubeconfigArgs...), "exec", "-i", "-n", ns, podName, "--",
		"tar", "xf", "-", "--no-absolute-names", "-C", remotePath)
	extractCmd := exec.CommandContext(ctx, "kubectl", kubectlArgs...)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	extractCmd.Stdin = pipe
	extractCmd.Stdout = os.Stdout
	extractCmd.Stderr = os.Stderr

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("starting tar: %w", err)
	}
	if err := extractCmd.Start(); err != nil {
		return fmt.Errorf("starting kubectl exec: %w", err)
	}

	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("tar command: %w", err)
	}
	if err := extractCmd.Wait(); err != nil {
		return fmt.Errorf("kubectl exec: %w", err)
	}

	return nil
}

// k8sPull transfers files from the K8s Pod to local storage via tar-over-exec.
func k8sPull(ctx context.Context, ns, podName string, kubeconfigArgs []string, opts SyncOpts) error {
	remotePath := opts.RemotePath
	if remotePath == "" {
		return fmt.Errorf("remote path is required for pull")
	}
	if err := validateSyncPath(remotePath); err != nil {
		return err
	}

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}
	if err := validateSyncPath(localPath); err != nil {
		return err
	}

	// Tar remote files via kubectl exec and extract locally.
	kubectlArgs := append(append([]string(nil), kubeconfigArgs...), "exec", "-i", "-n", ns, podName, "--",
		"tar", "cf", "-", "-C", remotePath, ".")
	tarCmd := exec.CommandContext(ctx, "kubectl", kubectlArgs...)
	extractCmd := exec.CommandContext(ctx, "tar", "xf", "-", "--no-absolute-names", "-C", localPath)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	extractCmd.Stdin = pipe
	extractCmd.Stderr = os.Stderr

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("starting kubectl exec: %w", err)
	}
	if err := extractCmd.Start(); err != nil {
		return fmt.Errorf("starting tar extract: %w", err)
	}

	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("kubectl exec: %w", err)
	}
	if err := extractCmd.Wait(); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	return nil
}

func gitExec(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
