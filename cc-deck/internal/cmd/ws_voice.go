package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cc-deck/cc-deck/internal/voice"
	voicetui "github.com/cc-deck/cc-deck/internal/tui/voice"
	"github.com/cc-deck/cc-deck/internal/ws"
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
	if verbose {
		logFile, err := os.OpenFile("/tmp/cc-deck-voice.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err == nil {
			log.SetOutput(logFile)
			log.SetFlags(log.Ltime | log.Lmicroseconds)
			defer logFile.Close()
		}
		log.Printf("[voice] starting: workspace=%s mode=%s model=%s port=%d", wsName, mode, modelName, port)
	}

	modelPath := voice.ModelPath(modelName)
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
	if verbose {
		log.Printf("[voice] pipe channel type: %T", ch)
	}

	server := voice.NewWhisperServer(modelPath, port)
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("starting whisper-server: %w", err)
	}
	defer server.Stop()
	if verbose {
		log.Printf("[voice] whisper-server ready at %s", server.Endpoint())
	}

	transcriber := voice.NewHTTPTranscriber(server.Endpoint(), server)
	audio := voice.NewAudioSource()

	config := voice.DefaultRelayConfig()
	config.Mode = mode
	config.Verbose = verbose

	var sessionName string
	if e.Type() == ws.WorkspaceTypeLocal {
		sessionName = "cc-deck-" + wsName
	}

	relay := voice.NewVoiceRelay(config, audio, transcriber, pipeAdapter{
		ch: ch, sessionName: sessionName, verbose: verbose,
	})

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

func runVoiceSetup(modelName string) error {
	return voice.RunSetup(modelName)
}

// pipeAdapter injects text into the focused pane of a local workspace
// via `zellij action write-chars`. For remote workspaces, it falls back
// to PipeChannel.Send (plugin-side write_chars_to_pane_id).
type pipeAdapter struct {
	ch          ws.PipeChannel
	sessionName string
	verbose     bool
}

func (a pipeAdapter) Send(ctx context.Context, pipeName string, payload string) error {
	if a.verbose {
		log.Printf("[voice] pipeAdapter.Send: name=%q payload=%q (%d bytes) session=%q",
			pipeName, payload, len(payload), a.sessionName)
	}

	var err error
	if a.sessionName != "" {
		cmd := exec.CommandContext(ctx, "zellij", "action", "write-chars", payload)
		cmd.Env = append(os.Environ(), "ZELLIJ_SESSION_NAME="+a.sessionName)
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			err = fmt.Errorf("write-chars: %w: %s", cmdErr, string(out))
		}
	} else {
		err = a.ch.Send(ctx, pipeName, payload)
	}

	if a.verbose {
		if err != nil {
			log.Printf("[voice] pipeAdapter.Send: ERROR %v", err)
		} else {
			log.Printf("[voice] pipeAdapter.Send: OK")
		}
	}
	return err
}
