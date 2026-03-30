package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/tui"
)

// NewTuiCmd creates the tui subcommand.
func NewTuiCmd(_ *GlobalFlags) *cobra.Command {
	var pollLocal time.Duration
	var pollContainer time.Duration
	var noColor bool

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive environment dashboard",
		Long: `Launch a full-screen terminal UI that provides a live overview of all
cc-deck environments. View status, attach to environments, create new
environments, and manage lifecycle operations from a single interface.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(tui.Options{
				PollLocal:     pollLocal,
				PollContainer: pollContainer,
				NoColor:       noColor,
			})
		},
	}

	cmd.Flags().DurationVar(&pollLocal, "poll-local", 2*time.Second, "Polling interval for local environments")
	cmd.Flags().DurationVar(&pollContainer, "poll-container", 5*time.Second, "Polling interval for container environments")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable color output")

	return cmd
}
