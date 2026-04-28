package voice

import (
	"context"
	"fmt"
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
	if len(sent) == 0 {
		t.Fatal("expected at least one send, got none")
	}
	if sent[0].payload != "add error handling " {
		t.Errorf("payload = %q, want %q", sent[0].payload, "add error handling ")
	}
	if sent[0].name != "cc-deck:voice" {
		t.Errorf("pipe name = %q, want %q", sent[0].name, "cc-deck:voice")
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

func TestVoiceRelay_CommandWordSendsNewline(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"submit"}}
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
	if len(sent) == 0 {
		t.Fatal("expected at least one send, got none")
	}
	if sent[0].payload != "\r" {
		t.Errorf("payload = %q, want carriage return for terminal Enter", sent[0].payload)
	}
}

func TestVoiceRelay_NonCommandRelaysFullText(t *testing.T) {
	audio := newMockAudioSource(makeSpeech(500, 5000), makeSilence(500))
	transcriber := &mockTranscriber{results: []string{"please submit the form"}}
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
	if len(sent) == 0 {
		t.Fatal("expected at least one send, got none")
	}
	if sent[0].payload != "please submit the form " {
		t.Errorf("payload = %q, want full text with trailing space", sent[0].payload)
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

	sent := pipe.getSent()
	if len(sent) != 0 {
		t.Errorf("expected no sends for whisper artifact, got %d", len(sent))
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

	sent := pipe.getSent()
	if len(sent) != 0 {
		t.Errorf("expected no sends for empty transcription, got %d", len(sent))
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

	sent := pipe.getSent()
	if len(sent) != 0 {
		t.Errorf("expected no sends on transcription error, got %d", len(sent))
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
		if ev.Type == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error event for delivery failure")
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
