package env

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
		return k8sGitPush(ctx, ns, podName, kubeconfigArgs)
	}

	// Tar local files and pipe via kubectl exec.
	tarCmd := exec.CommandContext(ctx, "tar", "cf", "-", "-C", localPath, ".")
	kubectlArgs := append(kubeconfigArgs, "exec", "-i", "-n", ns, podName, "--",
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
	kubectlArgs := append(kubeconfigArgs, "exec", "-i", "-n", ns, podName, "--",
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

// k8sHarvest extracts git commits from the K8s Pod via ext::kubectl exec.
func k8sHarvest(ctx context.Context, ns, podName string, kubeconfigArgs []string, opts HarvestOpts) error {
	if opts.Branch == "" {
		return fmt.Errorf("--branch is required for harvest")
	}

	// Build the ext:: remote URL for git.
	kubeconfigStr := ""
	contextStr := ""
	for i := 0; i < len(kubeconfigArgs)-1; i += 2 {
		switch kubeconfigArgs[i] {
		case "--kubeconfig":
			kubeconfigStr = kubeconfigArgs[i+1]
		case "--context":
			contextStr = kubeconfigArgs[i+1]
		}
	}

	remoteHelper := fmt.Sprintf("ext::kubectl")
	var extraArgs []string
	if kubeconfigStr != "" {
		extraArgs = append(extraArgs, "--kubeconfig", kubeconfigStr)
	}
	if contextStr != "" {
		extraArgs = append(extraArgs, "--context", contextStr)
	}
	extraArgs = append(extraArgs, "exec", "-i", "-n", ns, podName, "--", "%S", k8sWorkspacePath)

	remoteURL := remoteHelper
	for _, arg := range extraArgs {
		remoteURL += " " + arg
	}

	remoteName := "k8s-" + podName

	// Add remote (remove first if exists).
	_ = gitExec(ctx, "remote", "remove", remoteName)
	if err := gitExec(ctx, "remote", "add", remoteName, remoteURL); err != nil {
		return fmt.Errorf("adding git remote: %w", err)
	}
	defer func() { _ = gitExec(ctx, "remote", "remove", remoteName) }()

	// Fetch from remote.
	if err := gitExec(ctx, "fetch", remoteName); err != nil {
		return fmt.Errorf("fetching from remote: %w", err)
	}

	// Create local branch.
	if err := gitExec(ctx, "checkout", "-b", opts.Branch, remoteName+"/"+opts.Branch); err != nil {
		// Try without the remote prefix (if the branch name doesn't exist on remote).
		if err2 := gitExec(ctx, "checkout", "-b", opts.Branch, "FETCH_HEAD"); err2 != nil {
			return fmt.Errorf("creating local branch: %w", err)
		}
	}

	// Create PR if requested.
	if opts.CreatePR {
		if err := gitExec(ctx, "push", "-u", "origin", opts.Branch); err != nil {
			return fmt.Errorf("pushing branch: %w", err)
		}
		prCmd := exec.CommandContext(ctx, "gh", "pr", "create", "--fill")
		prCmd.Stdout = os.Stdout
		prCmd.Stderr = os.Stderr
		if err := prCmd.Run(); err != nil {
			return fmt.Errorf("creating PR: %w", err)
		}
	}

	return nil
}

// k8sGitPush pushes a local git repository into the K8s Pod via ext::kubectl exec.
func k8sGitPush(ctx context.Context, ns, podName string, kubeconfigArgs []string) error {
	kubeconfigStr := ""
	contextStr := ""
	for i := 0; i < len(kubeconfigArgs)-1; i += 2 {
		switch kubeconfigArgs[i] {
		case "--kubeconfig":
			kubeconfigStr = kubeconfigArgs[i+1]
		case "--context":
			contextStr = kubeconfigArgs[i+1]
		}
	}

	remoteHelper := "ext::kubectl"
	var extraArgs []string
	if kubeconfigStr != "" {
		extraArgs = append(extraArgs, "--kubeconfig", kubeconfigStr)
	}
	if contextStr != "" {
		extraArgs = append(extraArgs, "--context", contextStr)
	}
	extraArgs = append(extraArgs, "exec", "-i", "-n", ns, podName, "--", "%S", k8sWorkspacePath)

	remoteURL := remoteHelper
	for _, arg := range extraArgs {
		remoteURL += " " + arg
	}

	remoteName := "k8s-" + podName

	_ = gitExec(ctx, "remote", "remove", remoteName)
	if err := gitExec(ctx, "remote", "add", remoteName, remoteURL); err != nil {
		return fmt.Errorf("adding git remote: %w", err)
	}
	defer func() { _ = gitExec(ctx, "remote", "remove", remoteName) }()

	// Get current branch.
	branchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("detecting current branch: %w", err)
	}
	branch := string(branchOut)
	if len(branch) > 0 && branch[len(branch)-1] == '\n' {
		branch = branch[:len(branch)-1]
	}

	if err := gitExec(ctx, "push", remoteName, branch); err != nil {
		return fmt.Errorf("pushing to remote: %w", err)
	}

	return nil
}

func gitExec(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
