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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
	transcriber := &mockTranscriber{results: []string{"send"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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

func TestVoiceRelay_EmptyTranscriptionDiscarded(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"  "}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe)
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

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe)
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
			wantTarget: "(no session attended)",
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
			wantTarget: "(no session attended)",
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

func (m *mockPipeSendReceiver) SendReceive(_ context.Context, _ string, _ string) (string, error) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
	if err := relay.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	relay.Stop()
}

func TestVoiceRelay_AttendCommandSendsAttend(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"next"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)
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
		t.Error("expected [[attend]] in sends for 'next' command word")
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

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, transcriber, pipe)
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

func TestVoiceRelay_TranscribesWhileMutedAndRecording(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"notes to self"}}
	pipe := &mockPipeSender{}

	config := DefaultRelayConfig()
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)

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
	config.VADConfig.Threshold = 0.01
	config.VADConfig.SilenceDuration = 0.1
	config.VADConfig.PreRollDuration = 0

	relay := NewVoiceRelay(config, audio, transcriber, pipe)

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
