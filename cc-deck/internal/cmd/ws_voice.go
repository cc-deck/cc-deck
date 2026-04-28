package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	ccconfig "github.com/cc-deck/cc-deck/internal/config"
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
		verbose     bool
		setup       bool
		listDevices bool
		serverPort  int
		threshold   int
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
			return runVoiceRelay(args[0], mode, model, verbose, serverPort, threshold)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "vad", "capture mode: vad or ptt")
	cmd.Flags().StringVar(&model, "model", "base.en", "whisper model name")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "show diagnostic details")
	cmd.Flags().BoolVar(&setup, "setup", false, "check dependencies and download model")
	cmd.Flags().BoolVar(&listDevices, "list-devices", false, "list audio input devices")
	cmd.Flags().IntVar(&serverPort, "port", 8234, "whisper-server port")
	cmd.Flags().IntVar(&threshold, "threshold", -1, "VAD sensitivity (0-100, logarithmic); overrides config file")

	return cmd
}

func voiceLogPath() string {
	return filepath.Join(xdg.StateHome, "cc-deck", "voice.log")
}

func runVoiceRelay(wsName, mode, modelName string, verbose bool, port int, thresholdFlag int) error {
	var logPath string
	if verbose {
		logPath = voiceLogPath()
		if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err == nil {
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err == nil {
				log.SetOutput(logFile)
				log.SetFlags(log.Ltime | log.Lmicroseconds)
				defer logFile.Close()
			}
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
	if verbose {
		log.Printf("[voice] whisper-server ready at %s", server.Endpoint())
	}

	transcriber := voice.NewHTTPTranscriber(server.Endpoint(), server)
	audio := voice.NewAudioSource()

	config := voice.DefaultRelayConfig()
	config.Mode = mode
	config.Verbose = verbose

	thresholdPct := voice.ThresholdToPercent(config.VADConfig.Threshold)
	if cfg, err := ccconfig.Load(""); err == nil && cfg.Defaults.Voice.Threshold != nil {
		thresholdPct = *cfg.Defaults.Voice.Threshold
	}
	if thresholdFlag >= 0 {
		thresholdPct = thresholdFlag
	}
	config.VADConfig.Threshold = voice.PercentToThreshold(thresholdPct)

	relay := voice.NewVoiceRelay(config, audio, transcriber, &pipeAdapter{
		ch: ch, verbose: verbose,
	})

	if err := relay.Start(ctx); err != nil {
		return fmt.Errorf("starting voice relay: %w", err)
	}
	defer func() {
		done := make(chan struct{})
		go func() {
			relay.Stop()
			server.Stop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if verbose {
				log.Printf("[voice] cleanup timed out, forcing exit")
			}
		}
	}()

	model := voicetui.New(relay, mode, wsName, verbose, logPath)
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

// pipeAdapter sends voice text through PipeChannel to the cc-deck plugin,
// which injects it into the attended pane via write_chars_to_pane_id.
type pipeAdapter struct {
	ch      ws.PipeChannel
	verbose bool
}

func (a *pipeAdapter) Send(ctx context.Context, pipeName string, payload string) error {
	if a.verbose {
		log.Printf("[voice] pipeAdapter.Send: payload=%q (%d bytes)", payload, len(payload))
	}

	err := a.ch.Send(ctx, pipeName, payload)

	if a.verbose {
		if err != nil {
			log.Printf("[voice] pipeAdapter.Send: ERROR %v", err)
		} else {
			log.Printf("[voice] pipeAdapter.Send: OK")
		}
	}
	return err
}

func (a *pipeAdapter) SendReceive(ctx context.Context, pipeName string, payload string) (string, error) {
	if a.verbose {
		log.Printf("[voice] pipeAdapter.SendReceive: pipeName=%q payload=%q", pipeName, payload)
	}

	resp, err := a.ch.SendReceive(ctx, pipeName, payload)

	if a.verbose {
		if err != nil {
			log.Printf("[voice] pipeAdapter.SendReceive: ERROR %v", err)
		} else {
			log.Printf("[voice] pipeAdapter.SendReceive: response=%q", resp)
		}
	}
	return resp, err
}
