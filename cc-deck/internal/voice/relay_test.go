package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockAudioSource struct {
	mu      sync.Mutex
	frames  chan []int16
	level   float64
	started bool
}

func newMockAudioSource(frames ...[]int16) *mockAudioSource {
	ch := make(chan []int16, len(frames)+1)
	for _, f := range frames {
		ch <- f
	}
	close(ch)
	return &mockAudioSource{frames: ch}
}

func (m *mockAudioSource) Start(_ context.Context, _ int) (<-chan []int16, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return m.frames, nil
}

func (m *mockAudioSource) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = false
	return nil
}

func (m *mockAudioSource) Level() float64 { return m.level }

func (m *mockAudioSource) ListDevices() ([]DeviceInfo, error) { return nil, nil }

type mockTranscriber struct {
	mu      sync.Mutex
	results []string
	idx     int
	err     error
}

func (t *mockTranscriber) Transcribe(_ context.Context, _ []int16, _ int) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return "", t.err
	}
	if t.idx >= len(t.results) {
		return "", nil
	}
	text := t.results[t.idx]
	t.idx++
	return text, nil
}

func (t *mockTranscriber) Close() error { return nil }

type mockPipeSender struct {
	mu       sync.Mutex
	sent     []pipeSend
	sendErr  error
}

type pipeSend struct {
	name    string
	payload string
}

func (p *mockPipeSender) Send(_ context.Context, pipeName string, payload string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sendErr != nil {
		return p.sendErr
	}
	p.sent = append(p.sent, pipeSend{name: pipeName, payload: payload})
	return nil
}

func (p *mockPipeSender) getSent() []pipeSend {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]pipeSend, len(p.sent))
	copy(cp, p.sent)
	return cp
}

func collectEvents(ch <-chan RelayEvent, timeout time.Duration) []RelayEvent {
	var events []RelayEvent
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			return events
		}
	}
}

func TestVoiceRelay_TextFlowsToSender(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"add error handling"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events := collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := pipe.getSent()
	// Filter out protocol messages (voice:on, voice:off, voice:ping)
	var textSends []pipeSend
	for _, s := range sent {
		if s.name == "cc-deck:voice" && !isProtocolMessage(s.payload) {
			textSends = append(textSends, s)
		}
	}
	if len(textSends) == 0 {
		t.Fatal("expected at least one text send, got none")
	}
	if textSends[0].payload != "add error handling " {
		t.Errorf("payload = %q, want %q", textSends[0].payload, "add error handling ")
	}
	if textSends[0].name != "cc-deck:voice" {
		t.Errorf("pipe name = %q, want %q", textSends[0].name, "cc-deck:voice")
	}

	var hasTranscription, hasDelivery bool
	for _, ev := range events {
		if ev.Type == "transcription" {
			hasTranscription = true
		}
		if ev.Type == "delivery" {
			hasDelivery = true
		}
	}
	if !hasTranscription {
		t.Error("expected transcription event")
	}
	if !hasDelivery {
		t.Error("expected delivery event")
	}
}

func TestVoiceRelay_CommandWordSendsEnter(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"send it"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := pipe.getSent()
	var hasEnter bool
	for _, s := range sent {
		if s.payload == "[[enter]]" {
			hasEnter = true
		}
	}
	if !hasEnter {
		t.Error("expected [[enter]] in sends for command word")
	}
}

func TestVoiceRelay_NonCommandRelaysFullText(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"please send the email"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := pipe.getSent()
	var textSends []pipeSend
	for _, s := range sent {
		if s.name == "cc-deck:voice" && !isProtocolMessage(s.payload) {
			textSends = append(textSends, s)
		}
	}
	if len(textSends) == 0 {
		t.Fatal("expected at least one text send, got none")
	}
	if textSends[0].payload != "please send the email " {
		t.Errorf("payload = %q, want full text with trailing space", textSends[0].payload)
	}
}

func TestVoiceRelay_WhisperArtifactDiscarded(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"[background noise]"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends for whisper artifact, got %d", len(sent))
	}
}

// The other relay tests disable MinTranscriptionLatency, because a mock that
// answers instantly would otherwise be filtered as a hallucination. This one
// keeps the guard on to prove it still discards a too-fast transcription.
func TestVoiceRelay_FastTranscriptionDiscarded(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"add error handling"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// No real transcriber can beat this, so every answer looks suspicious.
	config.MinTranscriptionLatency = time.Hour
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends for a suspiciously fast transcription, got %d", len(sent))
	}
}

func TestVoiceRelay_EmptyTranscriptionDiscarded(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"  "}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends for empty transcription, got %d", len(sent))
	}
}

func TestVoiceRelay_TranscriptionErrorProducesEvent(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{err: fmt.Errorf("model crashed")}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events := collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	var hasError bool
	for _, ev := range events {
		if ev.Type == "error" && ev.Err != nil {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error event for transcription failure")
	}

	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends on transcription error, got %d", len(sent))
	}
}

func TestVoiceRelay_DeliveryErrorProducesEvent(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"hello"}}
	pipe := &mockPipeSender{sendErr: fmt.Errorf("workspace disconnected")}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events := collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	var hasError bool
	for _, ev := range events {
		if ev.Type == "error" && ev.Err != nil {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error event with non-nil Err for delivery failure")
	}
}

func TestVoiceRelay_StopClosesEvents(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	relay.Stop()

	select {
	case _, ok := <-relay.Events():
		if ok {
			t.Error("expected events channel to be closed after Stop")
		}
	case <-time.After(time.Second):
		t.Error("events channel not closed within 1s after Stop")
	}
}

func TestVoiceRelay_DoubleStartReturnsError(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer relay.Stop()

	err := relay.Start(context.Background())
	if err == nil {
		t.Error("expected error from double Start")
	}
}

func TestParseDumpStateResponse(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name       string
		input      string
		wantTarget string
		wantMute   *bool
	}{
		{
			name:       "valid JSON with attended session",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":42}`,
			wantTarget: "claude-1",
		},
		{
			name:       "attended pane not in sessions",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":99}`,
			wantTarget: "claude-1",
		},
		{
			name:       "no attended pane ID",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}}}`,
			wantTarget: "claude-1",
		},
		{
			name:       "null attended pane ID",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":null}`,
			wantTarget: "claude-1",
		},
		{
			name:       "empty sessions map",
			input:      `{"sessions":{},"attended_pane_id":42}`,
			wantTarget: "",
		},
		{
			name:       "null sessions",
			input:      `{"sessions":null}`,
			wantTarget: "",
		},
		{
			name:       "malformed JSON",
			input:      `not json at all`,
			wantTarget: "",
		},
		{
			name:       "empty string",
			input:      ``,
			wantTarget: "",
		},
		{
			name:       "session with empty display name",
			input:      `{"sessions":{"10":{"display_name":""}},"attended_pane_id":10}`,
			wantTarget: "",
		},
		{
			name:       "voice mute requested true",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":42,"voice_mute_requested":true}`,
			wantTarget: "claude-1",
			wantMute:   boolPtr(true),
		},
		{
			name:       "voice mute requested false",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":42,"voice_mute_requested":false}`,
			wantTarget: "claude-1",
			wantMute:   boolPtr(false),
		},
		{
			name:       "voice mute requested absent",
			input:      `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":42}`,
			wantTarget: "claude-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDumpStateResponse(tt.input)
			if result.targetName != tt.wantTarget {
				t.Errorf("targetName = %q, want %q", result.targetName, tt.wantTarget)
			}
			if tt.wantMute == nil {
				if result.voiceMuteRequested != nil {
					t.Errorf("voiceMuteRequested = %v, want nil", *result.voiceMuteRequested)
				}
			} else {
				if result.voiceMuteRequested == nil {
					t.Errorf("voiceMuteRequested = nil, want %v", *tt.wantMute)
				} else if *result.voiceMuteRequested != *tt.wantMute {
					t.Errorf("voiceMuteRequested = %v, want %v", *result.voiceMuteRequested, *tt.wantMute)
				}
			}
		})
	}
}

type mockPipeSendReceiver struct {
	mockPipeSender
	mu           sync.Mutex
	recvResponse string
	recvErr      error
	recvCalled   chan struct{}
}

func (m *mockPipeSendReceiver) SendReceive(_ context.Context, pipeName string, payload string) (string, error) {
	m.mockPipeSender.mu.Lock()
	m.mockPipeSender.sent = append(m.mockPipeSender.sent, pipeSend{name: pipeName, payload: payload})
	m.mockPipeSender.mu.Unlock()
	m.mu.Lock()
	resp := m.recvResponse
	err := m.recvErr
	ch := m.recvCalled
	m.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	return resp, err
}

func TestVoiceRelay_ContextCancelGracefulShutdown(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSendReceiver{
		recvResponse: "",
		recvErr:      fmt.Errorf("context cancelled"),
	}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	relay.Stop()
}

func TestVoiceRelay_AttendCommandSendsAttend(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"go next"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := pipe.getSent()
	var hasAttend bool
	for _, s := range sent {
		if s.payload == "[[attend]]" {
			hasAttend = true
		}
	}
	if !hasAttend {
		t.Error("expected [[attend]] in sends for 'go next' command phrase")
	}
}

func TestParseDumpStateResponse_PrefersFocusedPaneID(t *testing.T) {
	input := `{"sessions":{"10":{"display_name":"backend"},"20":{"display_name":"frontend"}},"attended_pane_id":10,"focused_pane_id":20}`
	result := parseDumpStateResponse(input)
	if result.targetName != "frontend" {
		t.Errorf("targetName = %q, want %q (should prefer focused_pane_id)", result.targetName, "frontend")
	}
	if !result.hasFocusedPane {
		t.Error("hasFocusedPane should be true")
	}
}

func TestParseDumpStateResponse_FallsBackToAttendedPaneID(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"focused_pane_id absent",
			`{"sessions":{"10":{"display_name":"backend"},"20":{"display_name":"frontend"}},"attended_pane_id":10}`,
		},
		{
			"focused_pane_id not in sessions",
			`{"sessions":{"10":{"display_name":"backend"}},"attended_pane_id":10,"focused_pane_id":99}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDumpStateResponse(tt.input)
			if result.targetName != "backend" {
				t.Errorf("targetName = %q, want %q (should fall back to attended_pane_id)", result.targetName, "backend")
			}
		})
	}
}

func TestParseDumpStateResponse_WorkingDir(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantDir    string
	}{
		{
			name:    "working_dir from attended session",
			input:   `{"sessions":{"42":{"display_name":"claude-1","working_dir":"/home/user/project"}},"attended_pane_id":42}`,
			wantDir: "/home/user/project",
		},
		{
			name:    "working_dir from focused session",
			input:   `{"sessions":{"10":{"display_name":"backend","working_dir":"/home/backend"},"20":{"display_name":"frontend","working_dir":"/home/frontend"}},"attended_pane_id":10,"focused_pane_id":20}`,
			wantDir: "/home/frontend",
		},
		{
			name:    "working_dir absent",
			input:   `{"sessions":{"42":{"display_name":"claude-1"}},"attended_pane_id":42}`,
			wantDir: "",
		},
		{
			name:    "working_dir from single session fallback",
			input:   `{"sessions":{"42":{"display_name":"claude-1","working_dir":"/home/user/solo"}}}`,
			wantDir: "/home/user/solo",
		},
		{
			name:    "working_dir fallback to attended when focused has none",
			input:   `{"sessions":{"10":{"display_name":"backend","working_dir":"/home/backend"},"20":{"display_name":"frontend"}},"attended_pane_id":10,"focused_pane_id":20}`,
			wantDir: "/home/backend",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDumpStateResponse(tt.input)
			if result.workingDir != tt.wantDir {
				t.Errorf("workingDir = %q, want %q", result.workingDir, tt.wantDir)
			}
		})
	}
}

func TestVoiceRelay_HeartbeatSendsMuteState(t *testing.T) {
	// The state poll is the heartbeat: its request body carries the mute
	// state, so no separate [[voice:on:*]] message is sent per tick.
	tests := []struct {
		name    string
		muted   bool
		wantMsg string
	}{
		{"unmuted polls with muted=false", false, `{"voice":{"on":true,"muted":false},"scope":"voice"}`},
		{"muted polls with muted=true", true, `{"voice":{"on":true,"muted":true},"scope":"voice"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio := newMockAudioSource()
			transcriber := &mockTranscriber{}
			recvCalled := make(chan struct{}, 8)
			pipe := &mockPipeSendReceiver{
				recvResponse: `{"sessions":{}}`,
				recvCalled:   recvCalled,
			}

			config := DefaultRelayConfig()
			// The mock transcriber answers instantly, so its speed says nothing
			// about whether the text is a hallucination.
			config.MinTranscriptionLatency = 0
			// Poll fast: the assertion below needs only that one tick has
			// happened, and at the production interval the first tick and this
			// test's own deadline are a dead heat that the ticker loses.
			config.StatePollInterval = 20 * time.Millisecond
			relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
			relay.mu.Lock()
			relay.muted = tt.muted
			relay.mu.Unlock()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := relay.Start(ctx); err != nil {
				t.Fatalf("Start failed: %v", err)
			}

			// Wait for at least one statePoll tick
			select {
			case <-recvCalled:
			case <-time.After(3 * time.Second):
				t.Fatal("statePoll did not fire within 3s")
			}

			cancel()
			relay.Stop()

			sent := pipe.getSent()
			var found bool
			for _, s := range sent {
				if s.name == "cc-deck:dump-state" && s.payload == tt.wantMsg {
					found = true
				}
				if s.name == "cc-deck:voice" && strings.HasPrefix(s.payload, "[[voice:on:") {
					t.Errorf("per-tick heartbeat %q must no longer be sent", s.payload)
				}
			}
			if !found {
				var payloads []string
				for _, s := range sent {
					payloads = append(payloads, s.payload)
				}
				t.Errorf("expected %q in sends, got: %v", tt.wantMsg, payloads)
			}
		})
	}
}

func isProtocolMessage(payload string) bool {
	return strings.HasPrefix(payload, "[[voice:") || payload == "[[enter]]" || payload == "[[attend]]"
}

func filterNonProtocol(sends []pipeSend) []pipeSend {
	var result []pipeSend
	for _, s := range sends {
		if s.name == "cc-deck:voice" && !isProtocolMessage(s.payload) {
			result = append(result, s)
		}
	}
	return result
}

func TestVoiceRelay_SendsVoiceOnAtStart(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	relay.Stop()

	sent := pipe.getSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one send")
	}
	if sent[0].payload != "[[voice:on]]" {
		t.Errorf("first payload = %q, want [[voice:on]]", sent[0].payload)
	}

	// Check voice:off is sent
	lastVoice := sent[len(sent)-1]
	if lastVoice.payload != "[[voice:off]]" {
		t.Errorf("last payload = %q, want [[voice:off]]", lastVoice.payload)
	}
}

func TestVoiceRelay_StartRecordingTdrzFallsBackWithoutTranscriber(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0
	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	// No turn transcriber set, so tdrz should fall back to basic
	mode, err := relay.StartRecording(TurnModeTdrz)
	if mode != TurnModeBasic {
		t.Errorf("mode = %q, want %q", mode, TurnModeBasic)
	}
	if err == nil {
		t.Error("expected fallback error, got nil")
	}
	if !relay.IsRecording() {
		t.Error("recording should still start even on fallback")
	}
	relay.StopRecording()
}

func TestVoiceRelay_TranscribesWhileMutedAndRecording(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"notes to self"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	// Set muted AND recording before starting so the utterance is processed
	// in the muted+recording path.
	relay.mu.Lock()
	relay.muted = true
	relay.recording = true
	relay.mu.Unlock()

	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events := collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	// Should have a transcription event.
	var hasTranscription bool
	for _, ev := range events {
		if ev.Type == "transcription" && ev.Text == "notes to self" {
			hasTranscription = true
		}
	}
	if !hasTranscription {
		t.Error("expected transcription event for muted+recording utterance")
	}

	// Should NOT have any text pipe sends (no delivery).
	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends while muted+recording, got %d: %v", len(sent), sent)
	}

	// Should NOT have a delivery event.
	for _, ev := range events {
		if ev.Type == "delivery" {
			t.Error("expected no delivery event while muted+recording")
		}
	}
}

func TestVoiceRelay_DiscardsWhileMutedNotRecording(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"should be discarded"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	// Set muted but NOT recording.
	relay.mu.Lock()
	relay.muted = true
	relay.recording = false
	relay.mu.Unlock()

	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events := collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	// Should NOT have any transcription events.
	for _, ev := range events {
		if ev.Type == "transcription" {
			t.Error("expected no transcription event while muted without recording")
		}
	}

	// Should NOT have any text pipe sends.
	sent := filterNonProtocol(pipe.getSent())
	if len(sent) != 0 {
		t.Errorf("expected no text sends while muted, got %d", len(sent))
	}
}

func TestVoiceRelay_StartRecordingAutoMutes(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	// Start unmuted
	if relay.IsMuted() {
		t.Fatal("expected unmuted initially")
	}

	// Recording should auto-mute
	mode, err := relay.StartRecording(TurnModeBasic)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mode != TurnModeBasic {
		t.Errorf("mode = %q, want %q", mode, TurnModeBasic)
	}
	if !relay.IsMuted() {
		t.Error("expected muted after StartRecording")
	}
	if !relay.IsRecording() {
		t.Error("expected recording after StartRecording")
	}

	// Stop recording should restore unmuted
	relay.StopRecording()
	if relay.IsMuted() {
		t.Error("expected unmuted after StopRecording")
	}
	if relay.IsRecording() {
		t.Error("expected not recording after StopRecording")
	}
}

func TestVoiceRelay_StartRecordingPreservesMuted(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	// Start already muted
	relay.mu.Lock()
	relay.muted = true
	relay.mu.Unlock()

	// Recording should keep muted
	relay.StartRecording(TurnModeBasic)
	if !relay.IsMuted() {
		t.Error("expected muted after StartRecording when already muted")
	}

	// Stop recording should stay muted (was muted before)
	relay.StopRecording()
	if !relay.IsMuted() {
		t.Error("expected still muted after StopRecording when was muted before")
	}
}

func TestVoiceRelay_StartRecordingEmitsMuteEvent(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)

	relay.StartRecording(TurnModeBasic)

	// Drain the muted event
	var gotMuted bool
	timeout := time.After(500 * time.Millisecond)
	for !gotMuted {
		select {
		case ev := <-relay.Events():
			if ev.Type == "muted" {
				gotMuted = true
			}
		case <-timeout:
			t.Fatal("expected muted event after StartRecording")
		}
	}

	relay.StopRecording()

	// Drain the unmuted event
	var gotUnmuted bool
	timeout = time.After(500 * time.Millisecond)
	for !gotUnmuted {
		select {
		case ev := <-relay.Events():
			if ev.Type == "unmuted" {
				gotUnmuted = true
			}
		case <-timeout:
			t.Fatal("expected unmuted event after StopRecording")
		}
	}
}

func TestStripBracketedAnnotations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no brackets", "hello world", "hello world"},
		{"typing annotation", "hello [typing] world", "hello world"},
		{"multiple annotations", "one [typing] two [silence] three", "one two three"},
		{"annotation at start", "[typing] hello", "hello"},
		{"annotation at end", "hello [typing]", "hello"},
		{"only annotation", "[typing]", ""},
		{"empty brackets", "hello [] world", "hello world"},
		{"nested text preserved", "use array[0] here", "use array here"},
		{"no extra whitespace", "a  [x]  b", "a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBracketedAnnotations(tt.input)
			if got != tt.want {
				t.Errorf("stripBracketedAnnotations(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVoiceRelay_BracketAnnotationStripped(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"hello [typing] world"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	// The mock transcriber answers instantly, so its speed says nothing
	// about whether the text is a hallucination.
	config.MinTranscriptionLatency = 0
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0
	config.VADConfig.MinSpeechDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	collectEvents(relay.Events(), 2*time.Second)
	relay.Stop()

	sent := filterNonProtocol(pipe.getSent())
	if len(sent) == 0 {
		t.Fatal("expected at least one text send")
	}
	if sent[0].payload != "hello world " {
		t.Errorf("payload = %q, want %q", sent[0].payload, "hello world ")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// drainBufferedEvents reads all events currently buffered in the channel
// without blocking. Used for tests that call handleUtterance directly.
func drainBufferedEvents(ch chan RelayEvent) []RelayEvent {
	var events []RelayEvent
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
		default:
			return events
		}
	}
}

func findTranscriptionEvents(events []RelayEvent) []RelayEvent {
	var result []RelayEvent
	for _, ev := range events {
		if ev.Type == "transcription" {
			result = append(result, ev)
		}
	}
	return result
}

func TestVoiceRelay_RecordingFirstPassageHasTurnStart(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"Hello there"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	relay.StartRecording(TurnModeBasic)

	u := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      0,
		End:        time.Second,
	}
	relay.handleUtterance(context.Background(), u)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) == 0 {
		t.Fatal("expected transcription event")
	}

	ev := transcriptions[0]
	if ev.Segments == nil {
		t.Fatal("expected non-nil Segments for recording event")
	}
	if len(ev.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(ev.Segments))
	}
	if !ev.Segments[0].TurnStart {
		t.Error("first passage segment should have TurnStart = true")
	}
	if ev.Segments[0].Text != "Hello there" {
		t.Errorf("segment text = %q, want %q", ev.Segments[0].Text, "Hello there")
	}
}

func TestVoiceRelay_RecordingDashMarkerSplitsSegments(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"Sure. - What about Friday?"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	relay.StartRecording(TurnModeBasic)

	u := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      0,
		End:        time.Second,
	}
	relay.handleUtterance(context.Background(), u)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) == 0 {
		t.Fatal("expected transcription event")
	}

	ev := transcriptions[0]
	if len(ev.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(ev.Segments))
	}
	// First piece: "Sure." with TurnStart (first passage)
	if ev.Segments[0].Text != "Sure." {
		t.Errorf("seg[0].Text = %q, want %q", ev.Segments[0].Text, "Sure.")
	}
	if !ev.Segments[0].TurnStart {
		t.Error("seg[0].TurnStart should be true (first passage)")
	}
	// Second piece: "What about Friday?" with TurnStart (dash marker)
	if ev.Segments[1].Text != "What about Friday?" {
		t.Errorf("seg[1].Text = %q, want %q", ev.Segments[1].Text, "What about Friday?")
	}
	if !ev.Segments[1].TurnStart {
		t.Error("seg[1].TurnStart should be true (dash marker)")
	}
}

func TestVoiceRelay_RecordingPauseBreakForcesTurnStart(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"first passage", "second passage"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0
	config.Recording.PauseBreak = 3 * time.Second

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	relay.StartRecording(TurnModeBasic)

	ctx := context.Background()

	// First utterance: Start=0, End=1s
	u1 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      0,
		End:        time.Second,
	}
	relay.handleUtterance(ctx, u1)

	// Second utterance: Start=5s, End=6s -> gap = 5s - 1s = 4s > 3s PauseBreak
	u2 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      5 * time.Second,
		End:        6 * time.Second,
	}
	relay.handleUtterance(ctx, u2)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) != 2 {
		t.Fatalf("expected 2 transcription events, got %d", len(transcriptions))
	}

	// First passage: TurnStart forced because it is the first passage
	if !transcriptions[0].Segments[0].TurnStart {
		t.Error("first passage should have TurnStart (first passage rule)")
	}

	// Second passage: TurnStart forced because gap > PauseBreak
	if !transcriptions[1].Segments[0].TurnStart {
		t.Error("second passage should have TurnStart (gap > PauseBreak)")
	}
}

func TestVoiceRelay_RecordingGapBelowPauseBreakNoTurnStart(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"first passage", "second passage"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0
	config.Recording.PauseBreak = 3 * time.Second

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	relay.StartRecording(TurnModeBasic)

	ctx := context.Background()

	// First utterance: Start=0, End=1s
	u1 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      0,
		End:        time.Second,
	}
	relay.handleUtterance(ctx, u1)

	// Second utterance: Start=2s, End=3s -> gap = 2s - 1s = 1s < 3s PauseBreak
	u2 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      2 * time.Second,
		End:        3 * time.Second,
	}
	relay.handleUtterance(ctx, u2)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) != 2 {
		t.Fatalf("expected 2 transcription events, got %d", len(transcriptions))
	}

	// Second passage: no TurnStart because gap < PauseBreak and no dash marker
	if transcriptions[1].Segments[0].TurnStart {
		t.Error("second passage should NOT have TurnStart (gap < PauseBreak)")
	}
}

func TestVoiceRelay_DictationEventHasNilSegments(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"hello world"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	// Do NOT call StartRecording: this is a dictation utterance.

	u := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      0,
		End:        time.Second,
	}
	relay.handleUtterance(context.Background(), u)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) == 0 {
		t.Fatal("expected transcription event")
	}

	if transcriptions[0].Segments != nil {
		t.Errorf("dictation event should have nil Segments, got %d segments", len(transcriptions[0].Segments))
	}
}

func TestVoiceRelay_RecordingSegmentTimingRelativeToStart(t *testing.T) {
	audio := newMockAudioSource()
	transcriber := &mockTranscriber{results: []string{"hello", "world"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.MinTranscriptionLatency = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe, nil)
	relay.StartRecording(TurnModeBasic)

	ctx := context.Background()

	// First utterance starts at 2s (simulating audio captured before recording)
	u1 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      2 * time.Second,
		End:        3 * time.Second,
	}
	relay.handleUtterance(ctx, u1)

	// Second utterance at 8s -> relative Start should be 6s (8s - 2s offset)
	u2 := Utterance{
		Audio:      make([]int16, 16000),
		SampleRate: 16000,
		Start:      8 * time.Second,
		End:        9 * time.Second,
	}
	relay.handleUtterance(ctx, u2)

	events := drainBufferedEvents(relay.events)
	transcriptions := findTranscriptionEvents(events)
	if len(transcriptions) != 2 {
		t.Fatalf("expected 2 transcription events, got %d", len(transcriptions))
	}

	// First passage: Start should be 0 (relative to recStartOffset = 2s)
	if transcriptions[0].Segments[0].Start != 0 {
		t.Errorf("first segment Start = %v, want 0", transcriptions[0].Segments[0].Start)
	}
	if transcriptions[0].Segments[0].End != time.Second {
		t.Errorf("first segment End = %v, want 1s", transcriptions[0].Segments[0].End)
	}

	// Second passage: Start should be 6s (8s - 2s offset)
	if transcriptions[1].Segments[0].Start != 6*time.Second {
		t.Errorf("second segment Start = %v, want 6s", transcriptions[1].Segments[0].Start)
	}
	if transcriptions[1].Segments[0].End != 7*time.Second {
		t.Errorf("second segment End = %v, want 7s", transcriptions[1].Segments[0].End)
	}

	// At should be set (non-zero)
	if transcriptions[0].Segments[0].At.IsZero() {
		t.Error("first segment At should be non-zero")
	}
}
