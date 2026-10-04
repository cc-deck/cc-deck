package voice

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

// makeSegments builds test segments with the given text and TurnStart flags.
func makeSegments(texts []string, turnStarts []bool) []voicepkg.Segment {
	segs := make([]voicepkg.Segment, len(texts))
	for i, text := range texts {
		segs[i] = voicepkg.Segment{
			Text:      text,
			TurnStart: turnStarts[i],
			At:        time.Date(2026, 10, 1, 14, 3, 12+i*10, 0, time.UTC),
			Start:     time.Duration(i*10) * time.Second,
			End:       time.Duration(i*10+5) * time.Second,
		}
	}
	return segs
}

func startRecordingModel(t *testing.T) Model {
	t.Helper()
	relay := testRelay()
	m := New(relay, "test-ws", "", voicepkg.TurnModeBasic)

	// Go through the recording setup flow.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "reading-test.txt")
	m.recInput.SetValue(tmpFile)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	if m.recState != recRecording {
		t.Fatalf("expected recRecording, got %d", m.recState)
	}
	return m
}

func TestAppendSegments_GroupByTurnStart(t *testing.T) {
	m := startRecordingModel(t)

	segs := makeSegments(
		[]string{"first turn", "still first", "second turn"},
		[]bool{true, false, true},
	)
	added := m.appendSegments(segs)

	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if len(m.recBuffer) != 2 {
		t.Fatalf("recBuffer length = %d, want 2", len(m.recBuffer))
	}
	if len(m.recBuffer[0].parts) != 2 {
		t.Errorf("block 0 parts = %d, want 2", len(m.recBuffer[0].parts))
	}
	if len(m.recBuffer[1].parts) != 1 {
		t.Errorf("block 1 parts = %d, want 1", len(m.recBuffer[1].parts))
	}
}

func TestAppendSegments_EmptyBufferCreatesTurnBlock(t *testing.T) {
	m := startRecordingModel(t)

	// A segment without TurnStart still creates a block when the buffer is empty.
	segs := makeSegments([]string{"hello"}, []bool{false})
	added := m.appendSegments(segs)

	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if len(m.recBuffer) != 1 {
		t.Fatalf("recBuffer length = %d, want 1", len(m.recBuffer))
	}
}

func TestRenderBlocks_WrappingRespectsWidth(t *testing.T) {
	m := startRecordingModel(t)

	longText := "This is a very long sentence that should be wrapped when the width is narrow enough to force it"
	segs := makeSegments([]string{longText}, []bool{true})
	m.appendSegments(segs)

	// Render at a narrow width (30 chars).
	output := m.renderBlocks(30)
	lines := strings.Split(output, "\n")

	// Should have more than 2 lines (timestamp + at least 2 wrapped lines).
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines at width 30, got %d", len(lines))
	}
}

func TestRenderBlocks_GutterAlternation(t *testing.T) {
	m := startRecordingModel(t)

	segs := makeSegments(
		[]string{"block zero", "block one", "block two"},
		[]bool{true, true, true},
	)
	m.appendSegments(segs)

	output := m.renderBlocks(80)

	// Check that even blocks use ▌ and odd blocks use ┃.
	if !strings.Contains(output, "▌") {
		t.Error("expected even block gutter ▌")
	}
	if !strings.Contains(output, "┃") {
		t.Error("expected odd block gutter ┃")
	}
}

func TestRenderBlocks_PlaceholderWhenEmpty(t *testing.T) {
	m := startRecordingModel(t)

	output := m.renderBlocks(80)
	if !strings.Contains(output, "Waiting for speech...") {
		t.Error("expected placeholder text when buffer is empty")
	}
}

func TestReadingView_VIgnoredWhenIdle(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "", voicepkg.TurnModeBasic)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if m.reading {
		t.Error("v should not open reading view when idle")
	}
}

func TestReadingView_VIgnoredDuringFilenamePrompt(t *testing.T) {
	relay := testRelay()
	m := New(relay, "test-ws", "", voicepkg.TurnModeBasic)

	// Enter filename prompt.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	if m.recState != recPrompting {
		t.Fatalf("expected recPrompting, got %d", m.recState)
	}

	// v during prompt should be typed as a character, not open reading view.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if m.reading {
		t.Error("v should not open reading view during filename prompt")
	}
}

func TestReadingView_VOpensWhileRecording(t *testing.T) {
	m := startRecordingModel(t)

	// Set a window size so the viewport can initialize.
	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if !m.reading {
		t.Error("v should open reading view while recording")
	}
	if !m.follow {
		t.Error("reading view should start in follow mode")
	}
}

func TestReadingView_EscReturnsToNormalView(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if !m.reading {
		t.Fatal("expected reading view open")
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.reading {
		t.Error("esc should close reading view")
	}
}

func TestReadingView_RStopClosesView(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if !m.reading {
		t.Fatal("expected reading view open")
	}

	// R should stop the recording and close the reading view.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = result.(Model)

	if m.reading {
		t.Error("R should close reading view")
	}
	if m.recState != recIdle {
		t.Errorf("recState = %d, want recIdle", m.recState)
	}
}

func TestReadingView_PausedSegmentsNotBuffered(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	// Send a transcription while recording.
	segs := makeSegments([]string{"while recording"}, []bool{true})
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
		Type:     "transcription",
		Text:     "while recording",
		Segments: segs,
	}))
	m = result.(Model)

	if len(m.recBuffer) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.recBuffer))
	}

	// Pause recording.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	if m.recState != recPaused {
		t.Fatalf("expected recPaused, got %d", m.recState)
	}

	// Send a transcription while paused (should not be buffered).
	pausedSegs := makeSegments([]string{"while paused"}, []bool{true})
	result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
		Type:     "transcription",
		Text:     "while paused",
		Segments: pausedSegs,
	}))
	m = result.(Model)

	if len(m.recBuffer) != 1 {
		t.Errorf("recBuffer should still have 1 block, got %d", len(m.recBuffer))
	}
}

func TestReadingView_FollowKeepsBottom(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = result.(Model)

	// Open reading view.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	// Send multiple segments to fill the viewport.
	for i := 0; i < 20; i++ {
		segs := makeSegments([]string{"line content"}, []bool{true})
		result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
			Type:     "transcription",
			Text:     "line content",
			Segments: segs,
		}))
		m = result.(Model)
	}

	if !m.follow {
		t.Error("should still be following after new content")
	}
}

func TestReadingView_ScrollUpFreezesAndCountsBlocks(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = result.(Model)

	// Add content and open reading view.
	for i := 0; i < 10; i++ {
		segs := makeSegments([]string{"content"}, []bool{true})
		result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
			Type:     "transcription",
			Text:     "content",
			Segments: segs,
		}))
		m = result.(Model)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	// Scroll up.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(Model)

	if m.follow {
		t.Error("scroll up should stop following")
	}

	// Add more content while not following.
	for i := 0; i < 3; i++ {
		segs := makeSegments([]string{"new block"}, []bool{true})
		result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
			Type:     "transcription",
			Text:     "new block",
			Segments: segs,
		}))
		m = result.(Model)
	}

	if m.newBlocks != 3 {
		t.Errorf("newBlocks = %d, want 3", m.newBlocks)
	}
}

func TestReadingView_GResumesAndResetsCounter(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = result.(Model)

	// Add content and open reading view.
	for i := 0; i < 10; i++ {
		segs := makeSegments([]string{"content"}, []bool{true})
		result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
			Type:     "transcription",
			Text:     "content",
			Segments: segs,
		}))
		m = result.(Model)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	// Scroll up and add new content.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(Model)
	m.newBlocks = 5

	// Press G to jump to bottom.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = result.(Model)

	if !m.follow {
		t.Error("G should resume following")
	}
	if m.newBlocks != 0 {
		t.Errorf("G should reset newBlocks, got %d", m.newBlocks)
	}
}

func TestReadingView_500PassagesRetained(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	// Add 500 segments.
	for i := 0; i < 500; i++ {
		segs := makeSegments([]string{"passage"}, []bool{true})
		result, _ = m.Update(relayEventMsg(voicepkg.RelayEvent{
			Type:     "transcription",
			Text:     "passage",
			Segments: segs,
		}))
		m = result.(Model)
	}

	if len(m.recBuffer) != 500 {
		t.Errorf("recBuffer should have 500 blocks, got %d (no 200 cap)", len(m.recBuffer))
	}
}

func TestReadingView_IgnoredKeysInReadingView(t *testing.T) {
	m := startRecordingModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	if !m.reading {
		t.Fatal("expected reading view open")
	}

	// These keys should be ignored in the reading view.
	for _, key := range []rune{'d', 'm', 's'} {
		result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = result.(Model)

		if !m.reading {
			t.Errorf("key %q should not close reading view", string(key))
		}
	}
}

func TestReadingView_VTogglesViewClosed(t *testing.T) {
	m := startRecordingModel(t)
	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)
	if !m.reading {
		t.Fatal("first v should open the reading view")
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)
	if m.reading {
		t.Error("second v should close the reading view, like esc")
	}
	if m.recState != recRecording {
		t.Errorf("closing the view must not affect the recording, recState = %v", m.recState)
	}
}

func TestReadingView_PlusMinusAdjustThreshold(t *testing.T) {
	m := startRecordingModel(t)
	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = result.(Model)

	before := m.relay.VADThreshold()
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = result.(Model)
	if got := m.relay.VADThreshold(); got <= before {
		t.Errorf("+ should raise the threshold: before %d, after %d", before, got)
	}

	raised := m.relay.VADThreshold()
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	m = result.(Model)
	if got := m.relay.VADThreshold(); got >= raised {
		t.Errorf("- should lower the threshold: before %d, after %d", raised, got)
	}
	if !m.reading {
		t.Error("+/- must not close the reading view")
	}
}

func TestReadingView_HeaderShowsLevelAndThreshold(t *testing.T) {
	m := startRecordingModel(t)
	m.width = 200
	m.recPath = "/tmp/2026-10-01-standup.txt"

	header := m.renderReadingHeader()
	want := fmt.Sprintf("T:%d%%", m.relay.VADThreshold())
	if !strings.Contains(header, want) {
		t.Errorf("header should show the threshold %q, got: %s", want, header)
	}
	if !strings.Contains(header, "2026-10-01-standup.txt") {
		t.Error("wide header should include the transcript file name")
	}
}

func TestReadingView_HeaderStaysOneLineWhenNarrow(t *testing.T) {
	m := startRecordingModel(t)
	m.width = 70
	m.recPath = "/tmp/2026-10-01-a-rather-long-transcript-name.txt"

	header := m.renderReadingHeader()
	if strings.Count(header, "\n") != 1 {
		t.Errorf("header must be exactly one line, got %q", header)
	}
	if strings.Contains(header, "a-rather-long-transcript-name") {
		t.Error("narrow header should drop the file name")
	}
	if !strings.Contains(header, "T:") {
		t.Error("narrow header should still show the threshold")
	}
}

func TestReadingView_RenderReadingHeader(t *testing.T) {
	m := startRecordingModel(t)
	m.recPath = "/tmp/2026-10-01-standup.txt"

	header := m.renderReadingHeader()

	if !strings.Contains(header, "Reading") {
		t.Error("header should contain 'Reading'")
	}
	if !strings.Contains(header, "REC") {
		t.Error("header should contain 'REC' while recording")
	}
	if !strings.Contains(header, "2026-10-01-standup.txt") {
		t.Error("header should contain the transcript filename")
	}
	if !strings.Contains(header, "Speakers: by pause") {
		t.Error("header should contain 'Speakers: by pause'")
	}
}

func TestReadingView_RenderReadingFooter(t *testing.T) {
	m := startRecordingModel(t)
	m.follow = true

	footer := m.renderReadingFooter()
	if !strings.Contains(footer, "following") {
		t.Error("footer should show 'following' when in follow mode")
	}

	m.follow = false
	m.newBlocks = 3
	footer = m.renderReadingFooter()
	if !strings.Contains(footer, "3 new") {
		t.Error("footer should show '3 new' when not following")
	}
}
