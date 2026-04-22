package ws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// gitFetch is the shared Fetch implementation for all GitChannel types.
func gitFetch(ctx context.Context, name, remoteURL string, opts HarvestOpts) error {
	remoteName := "cc-deck-" + name

	return withTemporaryRemote(ctx, remoteName, remoteURL, func() error {
		if err := gitExec(ctx, "fetch", remoteName); err != nil {
			return newChannelError("git", "fetch", name, "fetching from remote", err)
		}

		if opts.Branch != "" {
			if err := gitExec(ctx, "checkout", "-b", opts.Branch, remoteName+"/"+opts.Branch); err != nil {
				if err2 := gitExec(ctx, "checkout", "-b", opts.Branch, "FETCH_HEAD"); err2 != nil {
					return newChannelError("git", "fetch", name,
						fmt.Sprintf("creating local branch %q", opts.Branch), err2)
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
	return gitFetch(ctx, c.name, buildExtPodmanURL(c.containerName(), c.workspacePath), opts)
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
	return gitFetch(ctx, c.name, buildExtKubectlURL(c.ns, c.podName, c.workspacePath, c.kubeconfigArgs), opts)
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

func (c *sshGitChannel) remoteURL() string {
	return fmt.Sprintf("ssh://%s%s", c.host, c.workspace)
}

func (c *sshGitChannel) Fetch(ctx context.Context, opts HarvestOpts) error {
	return gitFetch(ctx, c.name, c.remoteURL(), opts)
}

func (c *sshGitChannel) Push(ctx context.Context) error {
	return gitPush(ctx, c.name, c.remoteURL())
}

func createPR(ctx context.Context, branch, remoteName string) error {
	if branch == "" {
		branch = remoteName + "/main"
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
