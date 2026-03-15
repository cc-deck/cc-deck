package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/k8s"
	filesync "github.com/cc-deck/cc-deck/internal/sync"
)

// SyncFlags holds the flags for the sync command.
type SyncFlags struct {
	Pull    bool
	Dir     string
	Exclude []string
}

// NewSyncCmd creates the sync cobra command.
func NewSyncCmd(globalFlags *GlobalFlags) *cobra.Command {
	flags := &SyncFlags{}

	cmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Sync files between local directory and a session Pod",
		Long: `Sync files between a local directory and a running Claude Code session.

By default, pushes files from the local directory to the Pod's /workspace.
Use --pull to sync files from the Pod to the local directory.

Default excluded patterns: .git, node_modules, target, __pycache__`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), args[0], flags, globalFlags)
		},
	}

	cmd.Flags().BoolVar(&flags.Pull, "pull", false, "Pull files from Pod to local (default: push local to Pod)")
	cmd.Flags().StringVar(&flags.Dir, "dir", ".", "Local directory to sync")
	cmd.Flags().StringSliceVar(&flags.Exclude, "exclude", nil, "Additional exclude patterns (added to defaults)")

	return cmd
}

func runSync(ctx context.Context, sessionName string, flags *SyncFlags, gf *GlobalFlags) error {
	// Resolve directory to absolute path
	dir, err := filepath.Abs(flags.Dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("accessing directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Create K8s client
	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  gf.Namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	podName := k8s.ResourcePrefix(sessionName) + "-0"

	syncOpts := filesync.SyncOptions{
		PodName:       podName,
		Namespace:     client.Namespace,
		ContainerName: "claude",
		LocalDir:      dir,
		RemoteDir:     "/workspace",
		Excludes:      flags.Exclude,
		Clientset:     client.Clientset,
		RestConfig:    client.RestConfig,
	}

	if flags.Pull {
		fmt.Fprintf(os.Stderr, "Pulling files from %s:/workspace to %s\n", podName, dir)
		if err := filesync.Pull(ctx, syncOpts); err != nil {
			return fmt.Errorf("pull sync failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Pull sync complete.")
	} else {
		fmt.Fprintf(os.Stderr, "Pushing files from %s to %s:/workspace\n", dir, podName)
		if err := filesync.Push(ctx, syncOpts); err != nil {
			return fmt.Errorf("push sync failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Push sync complete.")
	}

	return nil
}
