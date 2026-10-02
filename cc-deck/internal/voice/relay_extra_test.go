package voice

import "testing"

func TestVoiceRelay_ListDevices(t *testing.T) {
	want := []DeviceInfo{{ID: "dev-1", Name: "Built-in Mic"}}
	audio := &mockAudioSourceWithDevices{devices: want}
	pipe := &mockPipeSender{}

	relay := NewVoiceRelay(DefaultRelayConfig(), audio, &mockTranscriber{}, pipe, nil)

	got, err := relay.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(got) != 1 || got[0].ID != "dev-1" {
		t.Errorf("ListDevices() = %+v, want %+v", got, want)
	}
}

func TestVoiceRelay_VADThresholdRoundTrip(t *testing.T) {
	audio := newMockAudioSource()
	pipe := &mockPipeSender{}
	relay := NewVoiceRelay(DefaultRelayConfig(), audio, &mockTranscriber{}, pipe, nil)

	relay.SetVADThreshold(42)
	if got := relay.VADThreshold(); got != 42 {
		t.Errorf("VADThreshold() = %d, want 42", got)
	}
}

func TestVoiceRelay_SendMuteCommand(t *testing.T) {
	audio := newMockAudioSource()
	pipe := &mockPipeSender{}
	relay := NewVoiceRelay(DefaultRelayConfig(), audio, &mockTranscriber{}, pipe, nil)

	if err := relay.SendMuteCommand("[[voice:mute]]"); err != nil {
		t.Fatalf("SendMuteCommand(mute): %v", err)
	}
	if !relay.IsMuted() {
		t.Error("expected relay to be muted after [[voice:mute]]")
	}

	if err := relay.SendMuteCommand("[[voice:unmute]]"); err != nil {
		t.Fatalf("SendMuteCommand(unmute): %v", err)
	}
	if relay.IsMuted() {
		t.Error("expected relay to be unmuted after [[voice:unmute]]")
	}

	sent := pipe.getSent()
	if len(sent) != 2 {
		t.Fatalf("expected 2 pipe sends, got %d", len(sent))
	}
	if sent[0].name != "cc-deck:voice" || sent[0].payload != "[[voice:mute]]" {
		t.Errorf("sent[0] = %+v, want name=cc-deck:voice payload=[[voice:mute]]", sent[0])
	}
	if sent[1].payload != "[[voice:unmute]]" {
		t.Errorf("sent[1].payload = %q, want %q", sent[1].payload, "[[voice:unmute]]")
	}
}

func TestVoiceRelay_SendMuteCommand_UnrecognizedLeavesStateUnchanged(t *testing.T) {
	audio := newMockAudioSource()
	pipe := &mockPipeSender{}
	relay := NewVoiceRelay(DefaultRelayConfig(), audio, &mockTranscriber{}, pipe, nil)

	if err := relay.SendMuteCommand("[[voice:ping]]"); err != nil {
		t.Fatalf("SendMuteCommand(ping): %v", err)
	}
	if relay.IsMuted() {
		t.Error("expected an unrecognized command not to change muted state")
	}
}

// mockAudioSourceWithDevices extends the base mock with a configurable
// ListDevices result, for testing VoiceRelay.ListDevices delegation.
type mockAudioSourceWithDevices struct {
	mockAudioSource
	devices []DeviceInfo
}

func (m *mockAudioSourceWithDevices) ListDevices() ([]DeviceInfo, error) {
	return m.devices, nil
}
