package voice

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// PipeSender abstracts the PipeChannel.Send method for voice relay.
type PipeSender interface {
	Send(ctx context.Context, pipeName string, payload string) error
}

// RelayConfig configures the voice relay pipeline.
type RelayConfig struct {
	Mode       string // "vad" or "ptt"
	SampleRate int
	VADConfig  VADConfig
	Verbose    bool
}

// DefaultRelayConfig returns sensible defaults for the relay.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		Mode:       "vad",
		SampleRate: 16000,
		VADConfig:  DefaultVADConfig(),
	}
}

// RelayEvent represents an event from the relay pipeline to the TUI.
type RelayEvent struct {
	Type    string // "level", "transcription", "delivery", "error", "paused"
	Text    string
	Level   float64
	Latency time.Duration
	Err     error
}

// VoiceRelay orchestrates the audio -> VAD -> transcription -> pipe delivery pipeline.
type VoiceRelay struct {
	config      RelayConfig
	audio       AudioSource
	transcriber Transcriber
	pipe        PipeSender
	events      chan RelayEvent

	mu        sync.Mutex
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewVoiceRelay creates a new relay connecting all pipeline stages.
func NewVoiceRelay(config RelayConfig, audio AudioSource, transcriber Transcriber, pipe PipeSender) *VoiceRelay {
	return &VoiceRelay{
		config:      config,
		audio:       audio,
		transcriber: transcriber,
		pipe:        pipe,
		events:      make(chan RelayEvent, 32),
	}
}

// VADThreshold returns the current VAD threshold as a 0-100 percentage
// on a logarithmic scale.
func (r *VoiceRelay) VADThreshold() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ThresholdToPercent(r.config.VADConfig.Threshold)
}

// SetVADThreshold updates the VAD threshold from a 0-100 percentage
// on a logarithmic scale.
func (r *VoiceRelay) SetVADThreshold(pct int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.VADConfig.Threshold = PercentToThreshold(pct)
}

// ListDevices returns available audio input devices.
func (r *VoiceRelay) ListDevices() ([]DeviceInfo, error) {
	return r.audio.ListDevices()
}

// Events returns the channel of relay events for TUI consumption.
func (r *VoiceRelay) Events() <-chan RelayEvent {
	return r.events
}

// Start begins the voice relay pipeline.
func (r *VoiceRelay) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("voice relay already running")
	}
	r.running = true
	relayCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.mu.Unlock()

	frames, err := r.audio.Start(relayCtx, r.config.SampleRate)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		cancel()
		return fmt.Errorf("starting audio capture: %w", err)
	}

	vad := NewVAD(&r.config.VADConfig, r.config.SampleRate)
	utterances := vad.Process(frames)

	r.mu.Lock()
	r.ctx = relayCtx
	r.mu.Unlock()

	r.wg.Add(2)
	go r.levelPoll(relayCtx)
	go r.processUtterances(relayCtx, utterances)

	return nil
}

// Stop halts the voice relay pipeline and waits for goroutines to finish.
func (r *VoiceRelay) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()

	_ = r.audio.Stop()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}

	_ = r.transcriber.Close()
	r.closeOnce.Do(func() { close(r.events) })
}

func (r *VoiceRelay) levelPoll(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			level := r.audio.Level()
			select {
			case r.events <- RelayEvent{Type: "level", Level: level}:
			default:
			}
		}
	}
}

func (r *VoiceRelay) processUtterances(ctx context.Context, utterances <-chan Utterance) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case u, ok := <-utterances:
			if !ok {
				return
			}
			r.handleUtterance(ctx, u)
		}
	}
}

func (r *VoiceRelay) handleUtterance(ctx context.Context, u Utterance) {
	start := time.Now()

	if r.config.Verbose {
		log.Printf("[voice] utterance: %d samples, %d Hz, duration=%s",
			len(u.Audio), u.SampleRate, UtteranceDuration(u))
	}

	text, err := r.transcriber.Transcribe(ctx, u.Audio, u.SampleRate)
	if err != nil {
		r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("transcription: %w", err)})
		return
	}

	text = strings.TrimSpace(text)
	if r.config.Verbose {
		log.Printf("[voice] transcribed: %q", text)
	}
	if text == "" {
		if r.config.Verbose {
			log.Printf("[voice] empty transcription, skipping")
		}
		return
	}

	if IsWhisperArtifact(text) {
		if r.config.Verbose {
			log.Printf("[voice] filtered whisper artifact: %q", text)
		}
		return
	}

	latency := time.Since(start)
	result := ProcessStopwords(text)

	if r.config.Verbose {
		log.Printf("[voice] stopword: text=%q isCommand=%v action=%q latency=%s",
			result.Text, result.IsCommand, result.CommandAction, latency)
	}

	r.sendEvent(RelayEvent{
		Type:    "transcription",
		Text:    result.Text,
		Latency: latency,
	})

	var payload string
	if result.IsCommand {
		payload = "\r"
	} else {
		payload = result.Text
	}

	if r.config.Verbose {
		log.Printf("[voice] sending to pipe cc-deck:voice, payload=%q (%d bytes)",
			payload, len(payload))
	}

	if err := r.pipe.Send(ctx, "cc-deck:voice", payload); err != nil {
		if r.config.Verbose {
			log.Printf("[voice] pipe send error: %v", err)
		}
		r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("delivery: %w", err)})
		return
	}

	if r.config.Verbose {
		log.Printf("[voice] delivered successfully")
	}
	r.sendEvent(RelayEvent{Type: "delivery", Text: payload})
}

func (r *VoiceRelay) sendEvent(ev RelayEvent) {
	r.mu.Lock()
	ctx := r.ctx
	r.mu.Unlock()

	if ev.Type == "level" {
		select {
		case r.events <- ev:
		default:
		}
		return
	}
	select {
	case r.events <- ev:
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
	}
}
