package voice

import (
	"context"
	"encoding/json"
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

// PipeSendReceiver extends PipeSender with blocking request-response for PTT long-poll.
type PipeSendReceiver interface {
	PipeSender
	SendReceive(ctx context.Context, pipeName string, payload string) (string, error)
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

	mu          sync.Mutex
	running     bool
	pttActive   bool
	paused      bool
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
	wg          sync.WaitGroup
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

// IsPTTActive returns whether PTT recording is currently active.
func (r *VoiceRelay) IsPTTActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pttActive
}

// Mode returns the current relay mode ("vad" or "ptt").
func (r *VoiceRelay) Mode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.config.Mode
}

// SetMode changes the relay mode. Takes effect on next Start or restart.
func (r *VoiceRelay) SetMode(mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.Mode = mode
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
	r.pttActive = false
	relayCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.ctx = relayCtx
	r.mu.Unlock()

	var err error
	if r.config.Mode == "ptt" {
		err = r.startPTT(relayCtx)
	} else {
		err = r.startVAD(relayCtx)
	}
	if err != nil {
		return err
	}

	if sr, ok := r.pipe.(PipeSendReceiver); ok {
		r.wg.Add(1)
		go r.statePoll(relayCtx, sr)
	}

	return nil
}

func (r *VoiceRelay) startVAD(ctx context.Context) error {
	frames, err := r.audio.Start(ctx, r.config.SampleRate)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		r.cancel()
		return fmt.Errorf("starting audio capture: %w", err)
	}

	vad := NewVAD(&r.config.VADConfig, r.config.SampleRate)
	utterances := vad.Process(frames)

	r.wg.Add(2)
	go r.levelPoll(ctx)
	go r.processUtterances(ctx, utterances)

	return nil
}

func (r *VoiceRelay) startPTT(ctx context.Context) error {
	sr, ok := r.pipe.(PipeSendReceiver)
	if !ok {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		r.cancel()
		return fmt.Errorf("PTT mode requires PipeSendReceiver (pipe does not support SendReceive)")
	}

	r.wg.Add(1)
	go r.pttLoop(ctx, sr)

	return nil
}

func (r *VoiceRelay) pttLoop(ctx context.Context, sr PipeSendReceiver) {
	defer r.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}

		r.sendEvent(RelayEvent{Type: "ptt_waiting"})

		if r.config.Verbose {
			log.Printf("[voice] PTT: sending long-poll to cc-deck:voice-control")
		}

		resp, err := sr.SendReceive(ctx, "cc-deck:voice-control", "listen")
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("PTT long-poll: %w", err)})
			time.Sleep(time.Second)
			continue
		}

		resp = strings.TrimSpace(resp)
		if r.config.Verbose {
			log.Printf("[voice] PTT: received response %q", resp)
		}

		if resp != "toggle" {
			continue
		}

		r.mu.Lock()
		r.pttActive = true
		r.mu.Unlock()
		r.sendEvent(RelayEvent{Type: "ptt_recording"})

		if r.config.Verbose {
			log.Printf("[voice] PTT: starting audio capture")
		}

		frames, err := r.audio.Start(ctx, r.config.SampleRate)
		if err != nil {
			r.mu.Lock()
			r.pttActive = false
			r.mu.Unlock()
			r.sendEvent(RelayEvent{Type: "error", Err: fmt.Errorf("PTT audio start: %w", err)})
			continue
		}

		r.collectPTTUtterance(ctx, frames, sr)
	}
}

func (r *VoiceRelay) collectPTTUtterance(ctx context.Context, frames <-chan []int16, sr PipeSendReceiver) {
	var allSamples []int16
	levelTicker := time.NewTicker(50 * time.Millisecond)
	defer levelTicker.Stop()

	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		_, _ = sr.SendReceive(ctx, "cc-deck:voice-control", "listen")
	}()

	collecting := true
	for collecting {
		select {
		case <-ctx.Done():
			_ = r.audio.Stop()
			r.mu.Lock()
			r.pttActive = false
			r.mu.Unlock()
			return
		case <-stopCh:
			collecting = false
		case f, ok := <-frames:
			if !ok {
				collecting = false
			} else {
				allSamples = append(allSamples, f...)
			}
		case <-levelTicker.C:
			level := r.audio.Level()
			select {
			case r.events <- RelayEvent{Type: "level", Level: level}:
			default:
			}
		}
	}

	_ = r.audio.Stop()
	r.mu.Lock()
	r.pttActive = false
	r.mu.Unlock()
	r.sendEvent(RelayEvent{Type: "ptt_waiting"})

	if len(allSamples) == 0 {
		return
	}

	if r.config.Verbose {
		log.Printf("[voice] PTT: collected %d samples, transcribing", len(allSamples))
	}

	u := Utterance{
		Audio:      allSamples,
		SampleRate: r.config.SampleRate,
	}
	r.handleUtterance(ctx, u)
}

// SwitchMode stops the current pipeline, changes mode, and restarts.
// The events channel is preserved so callers keep receiving events.
func (r *VoiceRelay) SwitchMode(mode string) error {
	r.stopInternal()

	r.mu.Lock()
	r.config.Mode = mode
	r.running = false
	r.pttActive = false
	r.mu.Unlock()

	ctx := context.Background()
	return r.Start(ctx)
}

// stopInternal halts goroutines and audio without closing the events channel.
func (r *VoiceRelay) stopInternal() {
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
}

// Stop halts the voice relay pipeline and waits for goroutines to finish.
func (r *VoiceRelay) Stop() {
	r.stopInternal()
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

// IsPaused returns whether the attended session is in a permission prompt state.
func (r *VoiceRelay) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

func (r *VoiceRelay) statePoll(ctx context.Context, sr PipeSendReceiver) {
	defer r.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastTarget string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := sr.SendReceive(ctx, "cc-deck:dump-state", "")
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			state := parseDumpStateResponse(resp)

			r.mu.Lock()
			wasPaused := r.paused
			r.paused = state.paused
			r.mu.Unlock()

			if state.paused && !wasPaused {
				r.sendEvent(RelayEvent{Type: "paused", Text: "permission prompt active"})
			} else if !state.paused && wasPaused {
				r.sendEvent(RelayEvent{Type: "resumed"})
			}

			if state.targetName != lastTarget {
				lastTarget = state.targetName
				r.sendEvent(RelayEvent{Type: "target_changed", Text: state.targetName})
			}
		}
	}
}

type dumpStateResult struct {
	paused     bool
	targetName string
}

func parseDumpStateResponse(stateJSON string) dumpStateResult {
	var envelope struct {
		Sessions       map[string]json.RawMessage `json:"sessions"`
		AttendedPaneID *int                       `json:"attended_pane_id"`
	}
	if err := json.Unmarshal([]byte(stateJSON), &envelope); err != nil {
		return dumpStateResult{}
	}

	if envelope.Sessions == nil {
		return dumpStateResult{}
	}

	attendedKey := ""
	if envelope.AttendedPaneID != nil {
		attendedKey = fmt.Sprintf("%d", *envelope.AttendedPaneID)
	}

	var result dumpStateResult

	if attendedKey != "" {
		if raw, ok := envelope.Sessions[attendedKey]; ok {
			var s struct {
				DisplayName string          `json:"display_name"`
				Activity    json.RawMessage `json:"activity"`
			}
			if json.Unmarshal(raw, &s) == nil {
				result.targetName = s.DisplayName
				var waiting map[string]string
				if json.Unmarshal(s.Activity, &waiting) == nil {
					result.paused = waiting["Waiting"] == "Permission"
				}
			}
		}
	}

	if result.targetName == "" && attendedKey == "" {
		result.targetName = "(no session attended)"
	}

	return result
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

	text = strings.Join(strings.Fields(text), " ")
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
		payload = result.Text + " "
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
