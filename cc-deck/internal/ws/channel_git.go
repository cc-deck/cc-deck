package ws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
)

// gitFetch is the shared Fetch implementation for all GitChannel types.
func gitFetch(ctx context.Context, name, remoteURL string, opts HarvestOpts) error {
	remoteName := "cc-deck-" + name

	return withTemporaryRemote(ctx, remoteName, remoteURL, func() error {
		if err := gitExec(ctx, "fetch", remoteName); err != nil {
			return newChannelError("git", "fetch", name, "fetching from remote", err)
		}

		if opts.Branch != "" {
			ref := remoteName + "/" + opts.Branch
			if err := gitExec(ctx, "checkout", "-b", opts.Branch, ref); err != nil {
				if err2 := gitExec(ctx, "checkout", "-b", opts.Branch, "FETCH_HEAD"); err2 != nil {
					// Branch already exists: switch to it and update
					if err3 := gitExec(ctx, "checkout", opts.Branch); err3 != nil {
						return newChannelError("git", "fetch", name,
							fmt.Sprintf("switching to branch %q", opts.Branch), err3)
					}
					_ = gitExec(ctx, "merge", "--ff-only", "FETCH_HEAD")
				}
			}
		}

		fmt.Fprintf(os.Stderr, "Harvested commits from %s\n", name)

		if opts.CreatePR {
			return createPR(ctx, opts.Branch, remoteName)
		}
		return nil
	})
}

// gitPush is the shared Push implementation for all GitChannel types.
func gitPush(ctx context.Context, name, remoteURL string) error {
	remoteName := "cc-deck-" + name

	return withTemporaryRemote(ctx, remoteName, remoteURL, func() error {
		branch, err := currentBranch(ctx)
		if err != nil {
			return newChannelError("git", "push", name, "detecting branch", err)
		}
		if err := gitExec(ctx, "push", remoteName, branch); err != nil {
			return newChannelError("git", "push", name, "pushing to remote", err)
		}
		return nil
	})
}

// podmanGitChannel synchronizes git commits via ext::podman exec.
type podmanGitChannel struct {
	name          string
	containerName func() string
	workspacePath string
}

func (c *podmanGitChannel) Fetch(ctx context.Context, opts HarvestOpts) error {
	wsPath := c.workspacePath
	if opts.Path != "" {
		wsPath = path.Join(wsPath, opts.Path)
	}
	return gitFetch(ctx, c.name, buildExtPodmanURL(c.containerName(), wsPath), opts)
}

func (c *podmanGitChannel) Push(ctx context.Context) error {
	return gitPush(ctx, c.name, buildExtPodmanURL(c.containerName(), c.workspacePath))
}

// k8sGitChannel synchronizes git commits via ext::kubectl exec.
type k8sGitChannel struct {
	name           string
	ns             string
	podName        string
	kubeconfigArgs []string
	workspacePath  string
}

func (c *k8sGitChannel) Fetch(ctx context.Context, opts HarvestOpts) error {
	wsPath := c.workspacePath
	if opts.Path != "" {
		wsPath = path.Join(wsPath, opts.Path)
	}
	return gitFetch(ctx, c.name, buildExtKubectlURL(c.ns, c.podName, wsPath, c.kubeconfigArgs), opts)
}

func (c *k8sGitChannel) Push(ctx context.Context) error {
	return gitPush(ctx, c.name, buildExtKubectlURL(c.ns, c.podName, c.workspacePath, c.kubeconfigArgs))
}

// sshGitChannel synchronizes git commits via ssh:// URL.
type sshGitChannel struct {
	name      string
	host      string
	workspace string
}

func (c *sshGitChannel) remoteURL(subpath string) string {
	wsPath := c.workspace
	if subpath != "" {
		wsPath = path.Join(wsPath, subpath)
	}
	return fmt.Sprintf("ssh://%s%s", c.host, wsPath)
}

func (c *sshGitChannel) Fetch(ctx context.Context, opts HarvestOpts) error {
	return gitFetch(ctx, c.name, c.remoteURL(opts.Path), opts)
}

func (c *sshGitChannel) Push(ctx context.Context) error {
	return gitPush(ctx, c.name, c.remoteURL(""))
}

func createPR(ctx context.Context, branch, _ string) error {
	if branch == "" {
		branch = "main"
	}
	if err := gitExec(ctx, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("pushing branch: %w", err)
	}
	prCmd := exec.CommandContext(ctx, "gh", "pr", "create", "--fill")
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	if err := prCmd.Run(); err != nil {
		return fmt.Errorf("creating PR: %w", err)
	}
	return nil
}
