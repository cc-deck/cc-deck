package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PipeSender abstracts the PipeChannel.Send method for voice relay.
type PipeSender interface {
	Send(ctx context.Context, pipeName string, payload string) error
}

// PipeSendReceiver extends PipeSender with blocking request-response for dump-state polling.
type PipeSendReceiver interface {
	PipeSender
	SendReceive(ctx context.Context, pipeName string, payload string) (string, error)
}

// RelayConfig configures the voice relay pipeline.
type RelayConfig struct {
	SampleRate int
	VADConfig  VADConfig
	Verbose    bool
	Commands   map[string]string // word -> action lookup (built by BuildCommandMap)
}

// DefaultRelayConfig returns sensible defaults for the relay.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		SampleRate: 16000,
		VADConfig:  DefaultVADConfig(),
		Commands:   BuildCommandMap(DefaultCommands),
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
	muted       bool
	recording   bool
	parentCtx   context.Context
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

// IsMuted returns whether the relay is currently muted.
func (r *VoiceRelay) IsMuted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.muted
}

// IsRecording returns whether transcript recording is active.
func (r *VoiceRelay) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// SetRecording sets the transcript recording state.
func (r *VoiceRelay) SetRecording(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording = on
}

// SendMuteCommand sends a mute/unmute protocol message to the plugin
// and updates the local muted state immediately so handleUtterance
// stops processing without waiting for the dump-state poll round-trip.
func (r *VoiceRelay) SendMuteCommand(cmd string) error {
	r.mu.Lock()
	if cmd == "[[voice:mute]]" {
		r.muted = true
	} else if cmd == "[[voice:unmute]]" {
		r.muted = false
	}
	ctx := r.ctx
	r.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return r.pipe.Send(ctx, "cc-deck:voice", cmd)
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
	r.parentCtx = ctx
	relayCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.ctx = relayCtx
	r.wg = sync.WaitGroup{}
	r.mu.Unlock()

	if err := r.startVAD(relayCtx); err != nil {
		return err
	}

	// Send voice:on protocol message
	if err := r.pipe.Send(ctx, "cc-deck:voice", "[[voice:on]]"); err != nil {
		if r.config.Verbose {
			log.Printf("[voice] failed to send voice:on: %v", err)
		}
	}

	// No dedicated heartbeat goroutine needed: the dump-state poll (every 1s)
	// serves as the heartbeat. The plugin refreshes voice_last_ping_ms on each
	// dump-state request when voice is enabled.

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
	// Send voice:off with a fresh context (parentCtx may already be cancelled)
	offCtx, offCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer offCancel()
	_ = r.pipe.Send(offCtx, "cc-deck:voice", "[[voice:off]]")

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

func (r *VoiceRelay) statePoll(ctx context.Context, sr PipeSendReceiver) {
	defer r.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastTarget string
	voiceOnConfirmed := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !voiceOnConfirmed {
				_ = r.pipe.Send(ctx, "cc-deck:voice", "[[voice:on]]")
				voiceOnConfirmed = true
			}

			resp, err := sr.SendReceive(ctx, "cc-deck:dump-state", "")
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				voiceOnConfirmed = false
				continue
			}

			state := parseDumpStateResponse(resp)

			if state.targetName != lastTarget {
				lastTarget = state.targetName
				r.sendEvent(RelayEvent{Type: "target_changed", Text: state.targetName})
			}

			if state.voiceMuteRequested != nil {
				requested := *state.voiceMuteRequested

				r.mu.Lock()
				muted := r.muted
				if requested != muted {
					r.muted = requested
				}
				r.mu.Unlock()

				if requested != muted {
					if requested {
						_ = r.pipe.Send(ctx, "cc-deck:voice", "[[voice:mute]]")
						r.sendEvent(RelayEvent{Type: "muted"})
					} else {
						_ = r.pipe.Send(ctx, "cc-deck:voice", "[[voice:unmute]]")
						r.sendEvent(RelayEvent{Type: "unmuted"})
					}
				}
			}
		}
	}
}

type dumpStateResult struct {
	targetName        string
	voiceMuteRequested *bool
}

func parseDumpStateResponse(stateJSON string) dumpStateResult {
	var envelope struct {
		Sessions            map[string]json.RawMessage `json:"sessions"`
		AttendedPaneID      *int                       `json:"attended_pane_id"`
		VoiceMuteRequested  *bool                      `json:"voice_mute_requested"`
	}
	// Zellij broadcast pipes can produce concatenated JSON objects when
	// multiple plugin instances respond. Use Decoder to parse only the
	// first complete JSON value.
	dec := json.NewDecoder(strings.NewReader(stateJSON))
	if err := dec.Decode(&envelope); err != nil {
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
	result.voiceMuteRequested = envelope.VoiceMuteRequested

	if attendedKey != "" {
		if raw, ok := envelope.Sessions[attendedKey]; ok {
			var s struct {
				DisplayName string `json:"display_name"`
			}
			if json.Unmarshal(raw, &s) == nil {
				result.targetName = s.DisplayName
			}
		}
	}

	// Fall back to first session when no pane is attended (e.g., after
	// --reset). Mirrors the controller's voice injection fallback.
	if result.targetName == "" && len(envelope.Sessions) > 0 {
		for _, raw := range envelope.Sessions {
			var s struct {
				DisplayName string `json:"display_name"`
			}
			if json.Unmarshal(raw, &s) == nil && s.DisplayName != "" {
				result.targetName = s.DisplayName
				break
			}
		}
	}

	if result.targetName == "" {
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
	muted := r.IsMuted()
	recording := r.IsRecording()

	if muted && !recording {
		if r.config.Verbose {
			log.Printf("[voice] muted, discarding utterance")
		}
		return
	}

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
	text = sanitizeTerminalText(text)
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

	// When muted but recording, emit the transcription event for the TUI
	// and transcript file, but skip stopword processing and pipe delivery.
	if muted && recording {
		r.sendEvent(RelayEvent{
			Type:    "transcription",
			Text:    text,
			Latency: latency,
		})
		return
	}

	result := ProcessStopwords(text, r.config.Commands)

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
		switch result.CommandAction {
		case "submit":
			payload = "[[enter]]"
		case "attend":
			payload = "[[attend]]"
		default:
			payload = "[[enter]]"
		}
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

var termEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func sanitizeTerminalText(text string) string {
	text = termEscapeRe.ReplaceAllString(text, "")
	// Strip any remaining ESC bytes (covers OSC, DCS, and other non-CSI sequences)
	text = strings.ReplaceAll(text, "\x1b", "")
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r == '\t' || r == ' ' || (r >= 0x20 && r != 0x7f) {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	if ctx == nil {
		select {
		case r.events <- ev:
		case <-time.After(2 * time.Second):
		}
		return
	}
	select {
	case r.events <- ev:
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
	}
}
