package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/k8s"
	"github.com/cc-deck/cc-deck/internal/session"
)

// LogsFlags holds the flags for the logs command.
type LogsFlags struct {
	Follow     bool
	TailLines  int64
	Timestamps bool
}

// NewLogsCmd creates the logs cobra command.
func NewLogsCmd(globalFlags *GlobalFlags) *cobra.Command {
	flags := &LogsFlags{}

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Stream logs from a Claude Code session",
		Long: `Stream Pod logs from a running Claude Code session.

By default shows all existing logs. Use --follow (-f) to stream
new log output in real-time.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, args[0], flags, globalFlags)
		},
	}

	cmd.Flags().BoolVarP(&flags.Follow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int64Var(&flags.TailLines, "tail", 0, "Number of lines from the end to show (0 = all)")
	cmd.Flags().BoolVar(&flags.Timestamps, "timestamps", false, "Show timestamps in log output")

	return cmd
}

func runLogs(cmd *cobra.Command, sessionName string, flags *LogsFlags, gf *GlobalFlags) error {
	// Load config to find the session's namespace
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	sess := cfg.FindSession(sessionName)
	if sess == nil {
		return fmt.Errorf("session %q not found in config", sessionName)
	}

	// Use the session's namespace, but allow override from flag
	namespace := sess.Namespace
	if gf.Namespace != "" {
		namespace = gf.Namespace
	}

	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	return session.Logs(cmd.Context(), client.Clientset, os.Stdout, sessionName, client.Namespace, session.LogsOptions{
		Follow:     flags.Follow,
		TailLines:  flags.TailLines,
		Timestamps: flags.Timestamps,
	})
}
