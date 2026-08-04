package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/mux"
	"github.com/cc-deck/cc-deck/internal/session"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// runHookRaw reads a pre-normalized JSON payload from stdin and forwards it
// directly to the Zellij plugin. Unlike runHook, it skips TranslateEvent()
// and expects the payload to already be in NormalizedPayload format.
func runHookRaw(stdin io.Reader, stderr io.Writer) {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		fmt.Fprintln(stderr, "error: zellij not found in PATH")
		os.Exit(1)
	}

	input, err := io.ReadAll(stdin)
	if err != nil || len(input) == 0 {
		fmt.Fprintln(stderr, "error: failed to read payload from stdin")
		os.Exit(1)
	}

	var payload agent.NormalizedPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		fmt.Fprintf(stderr, "error: malformed JSON payload: %v\n", err)
		os.Exit(1)
	}

	if payload.HookEvent == "" {
		fmt.Fprintln(stderr, "error: missing required field 'hook_event_name'")
		os.Exit(1)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(stderr, "error: encoding payload: %v\n", err)
		os.Exit(1)
	}

	muxSent := false
	if cfg, _ := config.Load(""); cfg != nil && cfg.Mux.Enabled {
		if sessionName := os.Getenv("ZELLIJ_SESSION_NAME"); sessionName != "" {
			socketPath := filepath.Join(xdg.RuntimeDir(), "cc-deck", "mux.sock")
			msg := mux.Message{SessionName: sessionName, PipeName: "cc-deck:hook", Args: string(payloadJSON)}
			if err := mux.SendOrStart(socketPath, msg); err == nil {
				muxSent = true
			}
		}
	}

	if !muxSent {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pipeCmd := exec.CommandContext(ctx, zellijPath, "pipe",
			"--name", "cc-deck:hook",
			"--", string(payloadJSON))
		if err := pipeCmd.Run(); err != nil {
			fmt.Fprintf(stderr, "error: failed to send pipe message: %v\n", err)
			os.Exit(1)
		}
	}

	session.AutoSave()
}
