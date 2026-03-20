package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/k8s"
	"github.com/cc-deck/cc-deck/internal/session"
)

// NewListCmd creates the list cobra command.
func NewListCmd(globalFlags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Claude Code sessions",
		Hidden:  true,
		Long: `DEPRECATED: Use 'cc-deck env list' instead. This command will be removed in a future release.

List all Claude Code sessions tracked in the local config.

Shows live status by checking Pod state on the cluster. Sessions whose
StatefulSet no longer exists are automatically marked as deleted.

Output formats:
  text    Table with NAME, NAMESPACE, STATUS, AGE, PROFILE, CONNECTION (default)
  json    JSON array
  yaml    YAML list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, globalFlags)
		},
	}

	return cmd
}

func runList(cmd *cobra.Command, gf *GlobalFlags) error {
	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  gf.Namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	return session.List(cmd.Context(), client.Clientset, os.Stdout, session.ListOptions{
		ConfigPath: gf.ConfigFile,
		Output:     gf.Output,
	})
}
