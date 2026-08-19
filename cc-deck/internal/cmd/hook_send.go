package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/mux"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// sendPipeMessage delivers payloadJSON to the Zellij plugin, preferring the mux
// broker when enabled. Falls back to a direct zellij pipe invocation.
// Returns nil on success, an error only when both paths fail.
func sendPipeMessage(zellijPath string, cfg *config.Config, payloadJSON []byte) error {
	if cfg != nil && cfg.Mux.Enabled {
		if sessionName := os.Getenv("ZELLIJ_SESSION_NAME"); sessionName != "" {
			socketPath := filepath.Join(xdg.RuntimeDir(), "cc-deck", "mux.sock")
			msg := mux.Message{SessionName: sessionName, PipeName: "cc-deck:hook", Args: string(payloadJSON)}
			if err := mux.SendOrStart(socketPath, msg); err == nil {
				return nil
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, zellijPath, "pipe",
		"--name", "cc-deck:hook",
		"--", string(payloadJSON))
	return cmd.Run()
}
