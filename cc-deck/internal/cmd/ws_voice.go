package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cc-deck/cc-deck/internal/voice"
	voicetui "github.com/cc-deck/cc-deck/internal/tui/voice"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/cc-deck/cc-deck/internal/xdg"
	"github.com/spf13/cobra"
)

func newWsVoiceCmd(_ *GlobalFlags) *cobra.Command {
	var (
		mode        string
		model       string
		device      string
		verbose     bool
		setup       bool
		listDevices bool
		serverPort  int
	)

	cmd := &cobra.Command{
		Use:   "voice <workspace>",
		Short: "Start voice relay to dictate into a workspace",
		Long: `Capture audio from the local microphone, transcribe via a local
Whisper model, and relay text into the attended agent pane. Supports
VAD (voice activity detection) and PTT (push-to-talk via F8) modes.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if setup {
				return runVoiceSetup(model)
			}
			if listDevices {
				return runListDevices()
			}
			if len(args) == 0 {
				return fmt.Errorf("workspace name required (or use --setup / --list-devices)")
			}
			return runVoiceRelay(args[0], mode, model, device, verbose, serverPort)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "vad", "capture mode: vad or ptt")
	cmd.Flags().StringVar(&model, "model", "base.en", "whisper model name")
	cmd.Flags().StringVar(&device, "device", "", "audio input device ID")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "show diagnostic details")
	cmd.Flags().BoolVar(&setup, "setup", false, "check dependencies and download model")
	cmd.Flags().BoolVar(&listDevices, "list-devices", false, "list audio input devices")
	cmd.Flags().IntVar(&serverPort, "port", 8234, "whisper-server port")

	return cmd
}

func runVoiceRelay(wsName, mode, modelName, deviceID string, verbose bool, port int) error {
	modelPath := filepath.Join(xdg.CacheHome, "cc-deck", "models", modelFileName(modelName))
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model %q not found at %s; run: cc-deck ws voice --setup", modelName, modelPath)
	}

	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")
	e, err := resolveWorkspace(wsName, store, defs)
	if err != nil {
		return err
	}

	ctx := cmd_context()

	ch, err := e.PipeChannel(ctx)
	if err != nil {
		return fmt.Errorf("getting pipe channel: %w", err)
	}

	server := voice.NewWhisperServer(modelPath, port)
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("starting whisper-server: %w", err)
	}
	defer server.Stop()

	transcriber := voice.NewHTTPTranscriber(server.Endpoint(), server)
	audio := voice.NewAudioSource()

	config := voice.DefaultRelayConfig()
	config.Mode = mode
	config.Verbose = verbose

	relay := voice.NewVoiceRelay(config, audio, transcriber, pipeAdapter{ch: ch})

	if err := relay.Start(ctx); err != nil {
		return fmt.Errorf("starting voice relay: %w", err)
	}
	defer relay.Stop()

	model := voicetui.New(relay, mode, wsName, verbose)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func runListDevices() error {
	src := voice.NewAudioSource()
	devices, err := src.ListDevices()
	if err != nil {
		return fmt.Errorf("listing devices: %w", err)
	}
	if len(devices) == 0 {
		fmt.Println("No audio input devices found.")
		return nil
	}
	for _, d := range devices {
		marker := " "
		if d.IsDefault {
			marker = "*"
		}
		fmt.Printf(" %s %s (%s)\n", marker, d.Name, d.ID)
	}
	return nil
}

func modelFileName(name string) string {
	return fmt.Sprintf("ggml-%s.bin", name)
}

func runVoiceSetup(modelName string) error {
	return voice.RunSetup(modelName)
}

// pipeAdapter wraps ws.PipeChannel to satisfy voice.PipeSender.
type pipeAdapter struct {
	ch ws.PipeChannel
}

func (a pipeAdapter) Send(ctx context.Context, pipeName string, payload string) error {
	return a.ch.Send(ctx, pipeName, payload)
}
