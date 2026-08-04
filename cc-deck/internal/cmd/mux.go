package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/mux"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// NewMuxCmd creates the mux broker cobra command.
func NewMuxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "mux",
		Short:  "Run the pipe mux broker daemon",
		Long:   `Starts a background daemon that receives pipe messages on a Unix domain socket, deduplicates them, and flushes to zellij pipe at regular intervals. Normally started automatically by hook commands.`,
		Hidden: true,
		RunE:   runMux,
	}

	return cmd
}

func runMux(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load("")
	if err != nil {
		cfg = &config.Config{}
	}

	mc := cfg.Mux.WithDefaults()

	socketDir := filepath.Join(xdg.RuntimeDir(), "cc-deck")
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return err
	}
	socketPath := filepath.Join(socketDir, "mux.sock")

	dedup := true
	if mc.Dedup != nil {
		dedup = *mc.Dedup
	}

	var logger mux.Logger
	if mc.Log {
		logger = mux.NewFileLogger()
	}

	broker := mux.NewBroker(socketPath, mc.FlushInterval, mc.IdleTimeout, mc.QueueSize, dedup, logger)
	return broker.Run()
}
