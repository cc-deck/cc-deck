package voice

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

func TestResolveTranscriptPath_Relative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	// Re-initialize the package-level var for this test.
	// Since xdg.DataHome is set at init time, we work around it
	// by testing the logic directly with a known absolute path.

	name := "my-notes.txt"
	path, err := resolveTranscriptPath(name)
	if err != nil {
		t.Fatalf("resolveTranscriptPath(%q) error: %v", name, err)
	}

	if filepath.IsAbs(name) {
		t.Fatalf("test input %q should be relative", name)
	}

	// The returned path should end with the filename.
	if !strings.HasSuffix(path, name) {
		t.Errorf("path %q should end with %q", path, name)
	}

	// The parent directory should exist.
	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent dir %q should exist: %v", parentDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("parent %q should be a directory", parentDir)
	}
}

func TestResolveTranscriptPath_Absolute(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "sub", "recording.txt")

	path, err := resolveTranscriptPath(absPath)
	if err != nil {
		t.Fatalf("resolveTranscriptPath(%q) error: %v", absPath, err)
	}

	if path != absPath {
		t.Errorf("path = %q, want %q", path, absPath)
	}

	// The parent directory should have been created.
	parentDir := filepath.Dir(absPath)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent dir %q should exist: %v", parentDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("parent %q should be a directory", parentDir)
	}
}

func TestDefaultTranscriptName(t *testing.T) {
	name := defaultTranscriptName()

	if !strings.HasPrefix(name, "transcript-") {
		t.Errorf("name %q should start with 'transcript-'", name)
	}
	if !strings.HasSuffix(name, ".txt") {
		t.Errorf("name %q should end with '.txt'", name)
	}
	// Format: transcript-YYYY-MM-DDTHH-MM-SS.txt
	// Total length: 11 (prefix) + 19 (timestamp) + 4 (suffix) = 34
	if len(name) != 34 {
		t.Errorf("name %q length = %d, want 34", name, len(name))
	}
}

// testRelay creates a minimal VoiceRelay suitable for TUI state machine tests.
// It uses stub implementations that satisfy the interfaces without doing real I/O.
func testRelay() *voicepkg.VoiceRelay {
	return voicepkg.NewVoiceRelay(
		voicepkg.DefaultRelayConfig(),
		&stubAudio{},
		&stubTranscriber{},
		&stubPipe{},
	)
}

type stubAudio struct{}

func (s *stubAudio) Start(_ context.Context, _ int) (<-chan []int16, error) {
	ch := make(chan []int16)
	close(ch)
	return ch, nil
}
func (s *stubAudio) Stop() error                                { return nil }
func (s *stubAudio) Level() float64                              { return 0 }
func (s *stubAudio) ListDevices() ([]voicepkg.DeviceInfo, error) { return nil, nil }

type stubTranscriber struct{}

func (s *stubTranscriber) Transcribe(_ context.Context, _ []int16, _ int) (string, error) {
	return "", nil
}
func (s *stubTranscriber) Close() error { return nil }

type stubPipe struct{}

func (s *stubPipe) Send(_ context.Context, _ string, _ string) error {
	return nil
}

func TestWriteTranscriptLine(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "transcript-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()

	if err := writeTranscriptLine(f, "hello world", false); err != nil {
		t.Fatalf("writeTranscriptLine: %v", err)
	}
	if err := writeTranscriptLine(f, "second line", false); err != nil {
		t.Fatalf("writeTranscriptLine: %v", err)
	}

	// Read back the file contents.
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	want := "hello world\nsecond line\n"
	if string(data) != want {
		t.Errorf("file content = %q, want %q", string(data), want)
	}
}

func TestRecordingStateMachine(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "")

	// Initial state is idle.
	if m.recState != recIdle {
		t.Fatalf("initial recState = %d, want recIdle", m.recState)
	}

	// Press 'r' to start prompting.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.recState != recPrompting {
		t.Fatalf("after 'r', recState = %d, want recPrompting", m.recState)
	}

	// Set a filename and press Enter to start recording.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-recording.txt")
	m.recInput.SetValue(tmpFile)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	if m.recState != recRecording {
		t.Fatalf("after Enter, recState = %d, want recRecording", m.recState)
	}
	if m.recFile == nil {
		t.Fatal("recFile should not be nil after starting recording")
	}
	if !relay.IsRecording() {
		t.Fatal("relay.IsRecording() should be true after starting recording")
	}

	// Press 'R' to stop recording.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = result.(Model)
	if m.recState != recIdle {
		t.Fatalf("after 'R', recState = %d, want recIdle", m.recState)
	}
	if m.recFile != nil {
		t.Fatal("recFile should be nil after stopping recording")
	}
	if relay.IsRecording() {
		t.Fatal("relay.IsRecording() should be false after stopping recording")
	}
}

func TestRecordingStateMachine_EscCancels(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "")

	// Press 'r' then Esc to cancel.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.recState != recPrompting {
		t.Fatalf("after 'r', recState = %d, want recPrompting", m.recState)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)
	if m.recState != recIdle {
		t.Fatalf("after Esc, recState = %d, want recIdle", m.recState)
	}
}

func TestTranscriptionCapturedDuringRecording(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "")

	// Start recording to a temp file.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "capture-test.txt")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	m.recInput.SetValue(tmpFile)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	// Send transcription events.
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "hello world"}))
	m = result.(Model)
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "second utterance"}))
	m = result.(Model)

	if m.recCount != 2 {
		t.Errorf("recCount = %d, want 2", m.recCount)
	}

	// Stop recording and read file.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = result.(Model)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "hello world\nsecond utterance\n"
	if string(data) != want {
		t.Errorf("file content = %q, want %q", string(data), want)
	}
}

func TestQuitClosesTranscript(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "")

	// Start recording.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "quit-test.txt")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	m.recInput.SetValue(tmpFile)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	// Write something.
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "before quit"}))
	m = result.(Model)

	// Quit.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = result.(Model)

	if m.recFile != nil {
		t.Fatal("recFile should be nil after quit")
	}
	if relay.IsRecording() {
		t.Fatal("relay.IsRecording() should be false after quit")
	}

	// File should contain what was written before quit.
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "before quit\n"
	if string(data) != want {
		t.Errorf("file content = %q, want %q", string(data), want)
	}
}

func TestTranscriptionSkippedWhilePaused(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "")

	// Start recording.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "pause-test.txt")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	m.recInput.SetValue(tmpFile)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	// Write one line while recording.
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "before pause"}))
	m = result.(Model)

	// Pause recording.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.recState != recPaused {
		t.Fatalf("after pause 'r', recState = %d, want recPaused", m.recState)
	}

	// Send transcription while paused - should appear in history but not in file.
	historyBefore := len(m.history)
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "during pause"}))
	m = result.(Model)
	if len(m.history) != historyBefore+1 {
		t.Errorf("history length = %d, want %d (transcription should appear in history while paused)", len(m.history), historyBefore+1)
	}

	// Resume recording.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.recState != recRecording {
		t.Fatalf("after resume 'r', recState = %d, want recRecording", m.recState)
	}

	// Write one more line after resume.
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{Type: "transcription", Text: "after resume"}))
	m = result.(Model)

	// Stop and verify file: should have "before pause" and "after resume" but NOT "during pause".
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = result.(Model)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "before pause\nafter resume\n"
	if string(data) != want {
		t.Errorf("file content = %q, want %q", string(data), want)
	}
}
