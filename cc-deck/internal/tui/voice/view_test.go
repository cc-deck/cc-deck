package voice

import (
	"context"
	"strings"
	"testing"
	"time"

	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
	tea "github.com/charmbracelet/bubbletea"
)

// fakeAudioSource is a no-op AudioSource for constructing a VoiceRelay in tests.
type fakeAudioSource struct{}

func (fakeAudioSource) Start(ctx context.Context, sampleRate int) (<-chan []int16, error) {
	ch := make(chan []int16)
	close(ch)
	return ch, nil
}
func (fakeAudioSource) Stop() error                                 { return nil }
func (fakeAudioSource) Level() float64                              { return 0 }
func (fakeAudioSource) ListDevices() ([]voicepkg.DeviceInfo, error) { return nil, nil }

// fakeTranscriber is a no-op Transcriber for constructing a VoiceRelay in tests.
type fakeTranscriber struct{}

func (fakeTranscriber) Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error) {
	return "", nil
}
func (fakeTranscriber) Close() error { return nil }

// fakePipeSender is a no-op PipeSender for constructing a VoiceRelay in tests.
type fakePipeSender struct{}

func (fakePipeSender) Send(ctx context.Context, pipeName string, payload string) error {
	return nil
}

func newTestModel() Model {
	relay := voicepkg.NewVoiceRelay(voicepkg.DefaultRelayConfig(), fakeAudioSource{}, fakeTranscriber{}, fakePipeSender{}, nil)
	m := New(relay, "my-workspace", "")
	m.width = 60
	m.height = 20
	return m
}

func TestView_Quitting(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	got := m.View()
	if !strings.Contains(got, "Voice relay stopped.") {
		t.Errorf("View() = %q, want it to contain stopped message", got)
	}
}

func TestView_NotReady(t *testing.T) {
	m := newTestModel()
	got := m.View()
	if !strings.Contains(got, "Initializing...") {
		t.Errorf("View() = %q, want it to contain Initializing message", got)
	}
}

func TestView_DevicePicker(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	m.devices = []voicepkg.DeviceInfo{
		{Name: "Built-in Mic", IsDefault: true},
		{Name: "USB Mic", IsDefault: false},
	}
	m.deviceIdx = 1
	got := m.View()
	if !strings.Contains(got, "Audio Devices") {
		t.Errorf("View() = %q, want device picker title", got)
	}
	if !strings.Contains(got, "Built-in Mic") || !strings.Contains(got, "USB Mic") {
		t.Errorf("View() = %q, want both device names", got)
	}
}

func TestView_Full(t *testing.T) {
	m := newTestModel()
	m.viewport = newViewport(m.width-1, 10)
	m.viewportReady = true
	m.history = []historyEntry{
		{text: "hello world", latency: 20 * time.Millisecond, status: "delivered", at: time.Now()},
		{text: "pending item", latency: 5 * time.Millisecond, status: "pending", at: time.Now()},
		{text: "boom", latency: 1 * time.Millisecond, status: "error", at: time.Now()},
	}
	m.syncViewport()
	got := m.View()
	if !strings.Contains(got, "cc-deck Voice Relay") {
		t.Errorf("View() missing header: %q", got)
	}
	if !strings.Contains(got, "my-workspace") {
		t.Errorf("View() missing workspace target: %q", got)
	}
}

func TestRenderHeader_Muted(t *testing.T) {
	m := newTestModel()
	m.muted = true
	m.session = "sess-1"
	got := m.renderHeader()
	if !strings.Contains(got, "MUTED") {
		t.Errorf("renderHeader() = %q, want MUTED", got)
	}
	if !strings.Contains(got, "sess-1") {
		t.Errorf("renderHeader() = %q, want session name", got)
	}
}

func TestRenderHeader_Recording(t *testing.T) {
	for _, tc := range []struct {
		state recStatus
		want  string
	}{
		{recRecording, "REC"},
		{recPaused, "REC"},
	} {
		m := newTestModel()
		m.recState = tc.state
		got := m.renderHeader()
		if !strings.Contains(got, tc.want) {
			t.Errorf("renderHeader() with state %v = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestRenderFooter_Prompting(t *testing.T) {
	m := newTestModel()
	m.recState = recPrompting
	m.recInput.SetValue("notes.txt")
	m.recTimestamps = true
	got := m.renderFooter()
	if !strings.Contains(got, "Transcript:") {
		t.Errorf("renderFooter() = %q, want Transcript label", got)
	}
	if !strings.Contains(got, "timestamps [on]") {
		t.Errorf("renderFooter() = %q, want timestamps on", got)
	}
}

func TestRenderFooter_ErrorAndStates(t *testing.T) {
	for _, tc := range []struct {
		state recStatus
		want  string
	}{
		{recIdle, "r: record"},
		{recRecording, "r: pause"},
		{recPaused, "r: resume"},
	} {
		m := newTestModel()
		m.recState = tc.state
		m.err = context.DeadlineExceeded
		got := m.renderFooter()
		if !strings.Contains(got, "Error:") {
			t.Errorf("renderFooter() = %q, want Error text", got)
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("renderFooter() with state %v = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestRenderFooter_Muted(t *testing.T) {
	m := newTestModel()
	m.muted = true
	got := m.renderFooter()
	if !strings.Contains(got, "m: unmute") {
		t.Errorf("renderFooter() = %q, want unmute hint", got)
	}
}

func TestFooterHeight(t *testing.T) {
	m := newTestModel()
	m.recState = recPrompting
	if got := m.footerHeight(); got != 4 {
		t.Errorf("footerHeight() (prompting) = %d, want 4", got)
	}

	m2 := newTestModel()
	if got := m2.footerHeight(); got != 3 {
		t.Errorf("footerHeight() (no error) = %d, want 3", got)
	}

	m3 := newTestModel()
	m3.err = context.DeadlineExceeded
	m3.width = 60
	if got := m3.footerHeight(); got < 2 {
		t.Errorf("footerHeight() (error) = %d, want >= 2", got)
	}
}

func TestRenderSeparator(t *testing.T) {
	m := newTestModel()
	m.width = 10
	got := m.renderSeparator()
	if !strings.Contains(got, strings.Repeat("─", 10)) {
		t.Errorf("renderSeparator() = %q, want 10 dashes", got)
	}

	m.width = 0
	got = m.renderSeparator()
	if !strings.Contains(got, strings.Repeat("─", 40)) {
		t.Errorf("renderSeparator() with zero width = %q, want default 40 dashes", got)
	}
}

func TestRenderViewportWithScrollbar_NoOverflow(t *testing.T) {
	m := newTestModel()
	m.viewport = newViewport(20, 10)
	m.viewport.SetContent("line1\nline2")
	got := m.renderViewportWithScrollbar()
	if !strings.Contains(got, "line1") {
		t.Errorf("renderViewportWithScrollbar() = %q, want content", got)
	}
}

func TestRenderViewportWithScrollbar_Overflow(t *testing.T) {
	m := newTestModel()
	m.viewport = newViewport(20, 2)
	content := strings.Repeat("line\n", 20)
	m.viewport.SetContent(content)
	got := m.renderViewportWithScrollbar()
	if got == "" {
		t.Errorf("renderViewportWithScrollbar() should not be empty when content overflows")
	}
}

func TestRenderHistory_Empty(t *testing.T) {
	m := newTestModel()
	got := m.renderHistory()
	if !strings.Contains(got, "Listening...") {
		t.Errorf("renderHistory() = %q, want Listening placeholder", got)
	}
}

func TestRenderHistory_Entries(t *testing.T) {
	m := newTestModel()
	m.history = []historyEntry{
		{text: "one", status: "delivered", at: time.Now()},
		{text: "two", status: "error", at: time.Now()},
		{text: "three", status: "pending", at: time.Now()},
	}
	got := m.renderHistory()
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderHistory() = %q, want to contain %q", got, want)
		}
	}
}

func TestViewDevicePicker(t *testing.T) {
	m := newTestModel()
	m.devices = []voicepkg.DeviceInfo{
		{Name: "Mic A", IsDefault: false},
		{Name: "Mic B", IsDefault: true},
	}
	m.deviceIdx = 0
	got := m.viewDevicePicker()
	if !strings.Contains(got, "Mic A") || !strings.Contains(got, "Mic B (default)") {
		t.Errorf("viewDevicePicker() = %q, want both mic names with default marker", got)
	}
	if !strings.Contains(got, "enter: select") {
		t.Errorf("viewDevicePicker() = %q, want hint text", got)
	}
}

func TestRenderBrailleBar(t *testing.T) {
	for _, tc := range []struct {
		level, threshold float64
	}{
		{0.0, 0.5},
		{0.5, 0.5},
		{1.0, 1.0},
		{0.5, -1.0},
		{0.5, 2.0},
	} {
		got := renderBrailleBar(tc.level, tc.threshold)
		if got == "" {
			t.Errorf("renderBrailleBar(%v, %v) returned empty string", tc.level, tc.threshold)
		}
	}
}

func TestNewViewport(t *testing.T) {
	vp := newViewport(10, 5)
	if vp.Width != 10 || vp.Height != 5 {
		t.Errorf("newViewport() width/height = %d/%d, want 10/5", vp.Width, vp.Height)
	}
	if vp.View() == "" && vp.Height > 0 {
		// content set to empty string, view should still be valid (no panic).
	}
}

func TestInit(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil Cmd")
	}
	// Executing the batched command should not panic; the relay's event
	// channel is unbuffered/empty so waitForEvent will block until closed.
	relay := m.relay
	relay.Stop()
	msg := cmd()
	// tea.Batch with a single sub-command returns it unwrapped, and
	// waitForEvent yields tea.QuitMsg once the relay's event channel closes.
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Init() cmd produced unexpected message type %T, want tea.QuitMsg", msg)
	}
}

func TestUpdateDevicePicker_Navigation(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	m.devices = []voicepkg.DeviceInfo{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	m.deviceIdx = 0

	next, _ := m.updateDevicePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm := next.(Model)
	if nm.deviceIdx != 1 {
		t.Errorf("after down, deviceIdx = %d, want 1", nm.deviceIdx)
	}

	next, _ = nm.updateDevicePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	nm = next.(Model)
	if nm.deviceIdx != 0 {
		t.Errorf("after up, deviceIdx = %d, want 0", nm.deviceIdx)
	}

	// Boundary: up at index 0 stays at 0.
	next, _ = nm.updateDevicePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	nm = next.(Model)
	if nm.deviceIdx != 0 {
		t.Errorf("up at boundary, deviceIdx = %d, want 0", nm.deviceIdx)
	}
}

func TestUpdateDevicePicker_EnterClosesPicker(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	m.devices = []voicepkg.DeviceInfo{{Name: "A"}}

	next, _ := m.updateDevicePicker(tea.KeyMsg{Type: tea.KeyEnter})
	nm := next.(Model)
	if nm.devicePick {
		t.Error("updateDevicePicker(enter) should close the picker")
	}
	if nm.devices != nil {
		t.Error("updateDevicePicker(enter) should clear devices")
	}
}

func TestUpdateDevicePicker_EscClosesPicker(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	m.devices = []voicepkg.DeviceInfo{{Name: "A"}}

	next, _ := m.updateDevicePicker(tea.KeyMsg{Type: tea.KeyEscape})
	nm := next.(Model)
	if nm.devicePick {
		t.Error("updateDevicePicker(esc) should close the picker")
	}
}

func TestUpdateDevicePicker_WindowSize(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	next, _ := m.updateDevicePicker(tea.WindowSizeMsg{Width: 100, Height: 50})
	nm := next.(Model)
	if nm.width != 100 || nm.height != 50 {
		t.Errorf("updateDevicePicker(WindowSizeMsg) width/height = %d/%d, want 100/50", nm.width, nm.height)
	}
}

func TestUpdateDevicePicker_RelayEventLevel(t *testing.T) {
	m := newTestModel()
	m.devicePick = true
	next, cmd := m.updateDevicePicker(relayEventMsg(voicepkg.RelayEvent{Type: "level", Level: 0.75}))
	nm := next.(Model)
	if nm.audioLevel != 0.75 {
		t.Errorf("audioLevel = %v, want 0.75", nm.audioLevel)
	}
	if cmd == nil {
		t.Error("updateDevicePicker should return a waitForEvent command after a relay event")
	}
}
