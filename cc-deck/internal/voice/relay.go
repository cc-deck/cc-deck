package voice

import (
	"context"
	"fmt"
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

	vad := NewVAD(r.config.VADConfig, r.config.SampleRate)
	utterances := vad.Process(frames)

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
	r.wg.Wait()
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

	text, err := r.transcriber.Transcribe(ctx, u.Audio, u.SampleRate)
	if err != nil {
		r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("transcription: %w", err)})
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	latency := time.Since(start)
	result := ProcessStopwords(text)

	r.sendEvent(RelayEvent{
		Type:    "transcription",
		Text:    result.Text,
		Latency: latency,
	})

	var payload string
	if result.IsCommand {
		payload = "\n"
	} else {
		payload = result.Text
	}

	if err := r.pipe.Send(ctx, "cc-deck:voice", payload); err != nil {
		r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("delivery: %w", err)})
		return
	}

	r.sendEvent(RelayEvent{Type: "delivery", Text: payload})
}

func (r *VoiceRelay) sendEvent(ev RelayEvent) {
	if ev.Type == "level" {
		select {
		case r.events <- ev:
		default:
		}
		return
	}
	r.events <- ev
}
