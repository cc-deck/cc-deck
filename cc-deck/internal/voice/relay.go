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
	SampleRate    int
	VADConfig     VADConfig
	Verbose       bool
	Commands      map[string]string // word -> action lookup (built by BuildCommandMap)
	PollInterval  time.Duration
}

// DefaultRelayConfig returns sensible defaults for the relay.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		SampleRate:   16000,
		VADConfig:    DefaultVADConfig(),
		Commands:     BuildCommandMap(DefaultCommands),
		PollInterval: 3 * time.Second,
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
	glossary    *Glossary

	mu              sync.Mutex
	running         bool
	muted           bool
	recording       bool
	savedThreshold  float64
	savedMuted      bool
	lastWorkingDir  string
	lastText        string
	repeatCount     int
	parentCtx   context.Context
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
	wg          sync.WaitGroup
}

// NewVoiceRelay creates a new relay connecting all pipeline stages.
// globalTerms is the list of glossary terms from the config file; pass nil
// when no glossary is configured.
func NewVoiceRelay(config RelayConfig, audio AudioSource, transcriber Transcriber, pipe PipeSender, globalTerms []string) *VoiceRelay {
	r := &VoiceRelay{
		config:      config,
		audio:       audio,
		transcriber: transcriber,
		pipe:        pipe,
		events:      make(chan RelayEvent, 32),
		glossary:    NewGlossary(globalTerms),
	}
	if len(globalTerms) > 0 {
		if ht, ok := transcriber.(*httpTranscriber); ok {
			ht.SetPrompt(r.glossary.ResolvePrompt(""))
		}
	}
	return r
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
// When recording starts, the VAD threshold is saved and set to 0
// so all audio is captured without filtering, and the relay is
// automatically muted so transcriptions are recorded but not
// delivered to the pipe. When recording stops, both threshold
// and mute state are restored.
func (r *VoiceRelay) SetRecording(on bool) {
	var muteChanged bool
	var nowMuted bool

	r.mu.Lock()
	if on && !r.recording {
		r.savedThreshold = r.config.VADConfig.Threshold
		r.config.VADConfig.Threshold = PercentToThreshold(0)
		r.savedMuted = r.muted
		if !r.muted {
			r.muted = true
			muteChanged = true
		}
	} else if !on && r.recording {
		r.config.VADConfig.Threshold = r.savedThreshold
		if r.muted != r.savedMuted {
			r.muted = r.savedMuted
			muteChanged = true
		}
	}
	nowMuted = r.muted
	r.recording = on
	r.mu.Unlock()

	if muteChanged {
		if nowMuted {
			r.sendEvent(RelayEvent{Type: "muted"})
		} else {
			r.sendEvent(RelayEvent{Type: "unmuted"})
		}
	}
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

	// Send voice:on with a short timeout so a hung pipe doesn't block the TUI.
	// The statePoll heartbeat will establish the connection regardless.
	onCtx, onCancel := context.WithTimeout(ctx, 3*time.Second)
	if err := r.pipe.Send(onCtx, "cc-deck:voice", "[[voice:on]]"); err != nil {
		if r.config.Verbose {
			log.Printf("[voice] failed to send voice:on: %v", err)
		}
	}
	onCancel()

	// No dedicated heartbeat goroutine needed: the dump-state poll (every 3s)
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
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	var lastTarget string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			muteState := "unmuted"
			if r.muted {
				muteState = "muted"
			}
			r.mu.Unlock()
			hbCtx, hbCancel := context.WithTimeout(ctx, 3*time.Second)
			_ = r.pipe.Send(hbCtx, "cc-deck:voice", fmt.Sprintf("[[voice:on:%s]]", muteState))
			hbCancel()

			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			resp, err := sr.SendReceive(pollCtx, "cc-deck:dump-state", "")
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			state := parseDumpStateResponse(resp)

			// Only update displayed target when we have a concrete
			// name. Empty targetName means no attended pane and
			// multiple sessions; keep showing the previous target.
			if state.targetName != "" && state.targetName != lastTarget {
				lastTarget = state.targetName
				r.sendEvent(RelayEvent{Type: "target_changed", Text: state.targetName})
			}

			// Update glossary prompt when the attended session's
			// working directory changes. Only act on non-empty
			// workingDir to avoid clearing the active prompt on
			// transient ambiguous poll responses (mirrors the
			// targetName guard above).
			r.mu.Lock()
			prevDir := r.lastWorkingDir
			r.mu.Unlock()
			if state.workingDir != "" && state.workingDir != prevDir {
				r.mu.Lock()
				r.lastWorkingDir = state.workingDir
				r.mu.Unlock()
				prompt := r.glossary.ResolvePrompt(state.workingDir)
				if ht, ok := r.transcriber.(*httpTranscriber); ok {
					ht.SetPrompt(prompt)
				}
				if r.config.Verbose {
					if prompt != "" {
						log.Printf("[voice] glossary prompt updated for %s (%d chars)", state.workingDir, len(prompt))
					} else {
						log.Printf("[voice] glossary prompt cleared (no terms for %s)", state.workingDir)
					}
				}
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
	targetName         string
	workingDir         string
	hasAttendedPane    bool
	hasFocusedPane     bool
	voiceMuteRequested *bool
}

func parseDumpStateResponse(stateJSON string) dumpStateResult {
	var envelope struct {
		Sessions            map[string]json.RawMessage `json:"sessions"`
		AttendedPaneID      *int                       `json:"attended_pane_id"`
		FocusedPaneID       *int                       `json:"focused_pane_id"`
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

	var result dumpStateResult
	result.voiceMuteRequested = envelope.VoiceMuteRequested

	type sessionFields struct {
		DisplayName string  `json:"display_name"`
		WorkingDir  *string `json:"working_dir"`
	}

	resolveSession := func(paneID int) sessionFields {
		key := fmt.Sprintf("%d", paneID)
		if raw, ok := envelope.Sessions[key]; ok {
			var s sessionFields
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
		}
		return sessionFields{}
	}

	derefDir := func(p *string) string {
		if p != nil {
			return *p
		}
		return ""
	}

	if envelope.FocusedPaneID != nil {
		result.hasFocusedPane = true
		fields := resolveSession(*envelope.FocusedPaneID)
		result.targetName = fields.DisplayName
		result.workingDir = derefDir(fields.WorkingDir)
	}

	if envelope.AttendedPaneID != nil {
		result.hasAttendedPane = true
		fields := resolveSession(*envelope.AttendedPaneID)
		if result.targetName == "" {
			result.targetName = fields.DisplayName
		}
		if result.workingDir == "" {
			result.workingDir = derefDir(fields.WorkingDir)
		}
	}

	if result.targetName == "" && len(envelope.Sessions) == 1 {
		for _, raw := range envelope.Sessions {
			var s sessionFields
			if json.Unmarshal(raw, &s) == nil && s.DisplayName != "" {
				result.targetName = s.DisplayName
				result.workingDir = derefDir(s.WorkingDir)
				break
			}
		}
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
	text = stripBracketedAnnotations(text)
	text = stripLeadingDash(text)
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

	r.mu.Lock()
	if text == r.lastText {
		r.repeatCount++
	} else {
		r.lastText = text
		r.repeatCount = 1
	}
	repeats := r.repeatCount
	r.mu.Unlock()
	if repeats >= 3 {
		if r.config.Verbose {
			log.Printf("[voice] suppressed cross-utterance repeat (%dx): %q", repeats, text)
		}
		return
	}

	latency := time.Since(start)

	if latency < 300*time.Millisecond {
		if r.config.Verbose {
			log.Printf("[voice] suspiciously fast transcription (%s), likely hallucination: %q", latency, text)
		}
		return
	}

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
		case "submit_attend":
			if err := r.pipe.Send(ctx, "cc-deck:voice", "[[enter]]"); err != nil {
				log.Printf("[voice] submit_attend: enter failed: %v", err)
			}
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
var bracketAnnotationRe = regexp.MustCompile(`\[[^\[\]]*\]`)

var speakerLabelRe = regexp.MustCompile(`^-\s?[A-Z][a-z]+[.,:]?\s*`)

func stripLeadingDash(text string) string {
	if m := speakerLabelRe.FindString(text); m != "" {
		rest := strings.TrimSpace(text[len(m):])
		if rest != "" {
			return rest
		}
	}
	if strings.HasPrefix(text, "- ") {
		return text[2:]
	}
	if strings.HasPrefix(text, "-") && len(text) > 1 && text[1] != '-' {
		return text[1:]
	}
	return text
}

func stripBracketedAnnotations(text string) string {
	result := bracketAnnotationRe.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(result), " ")
}

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
