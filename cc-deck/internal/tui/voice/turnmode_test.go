package voice

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

func newTestModelWithTurnMode(mode voicepkg.TurnMode) Model {
	relay := testRelay()
	m := New(relay, "test-ws", "", mode)
	m.width = 80
	m.height = 24
	return m
}

// withTdrzAvailable sets the tdrzStatus to report everything available.
func withTdrzAvailable(m *Model) {
	m.tdrzStatus = func() (error, error) { return nil, nil }
}

// withTdrzToolMissing sets the tdrzStatus to report whisper-cli missing.
func withTdrzToolMissing(m *Model) {
	m.tdrzStatus = func() (error, error) {
		return fmt.Errorf("whisper-cli not found"), nil
	}
}

// withTdrzModelMissing sets the tdrzStatus to report model missing.
func withTdrzModelMissing(m *Model) {
	m.tdrzStatus = func() (error, error) {
		return nil, fmt.Errorf("model not found at /path/to/model")
	}
}

func TestTurnMode_GTogglesWhenIdle(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Fatalf("initial mode = %q, want basic", m.turnMode)
	}

	// Press g: should switch to tdrz.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeTdrz {
		t.Errorf("mode after g = %q, want tdrz", m.turnMode)
	}

	// Press g again: should switch back to basic.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode after second g = %q, want basic", m.turnMode)
	}
}

func TestTurnMode_GIgnoredWhileRecording(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	m.recState = recRecording
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic while recording, got %q", m.turnMode)
	}
}

func TestTurnMode_GIgnoredWhilePaused(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	m.recState = recPaused
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic while paused, got %q", m.turnMode)
	}
}

func TestTurnMode_GIgnoredWhileDownloading(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	m.dlRunning = true
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic while downloading, got %q", m.turnMode)
	}
}

func TestTurnMode_ToolMissingShowsError(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzToolMissing(&m)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic when tool missing, got %q", m.turnMode)
	}
	if m.err == nil {
		t.Fatal("expected error for missing tool")
	}
	if !strings.Contains(m.err.Error(), "whisper-cli") {
		t.Errorf("error = %q, should mention whisper-cli", m.err.Error())
	}
	// No download prompt.
	if m.dlPrompt {
		t.Error("should not show download prompt for missing tool")
	}
}

func TestTurnMode_ModelMissingShowsPrompt(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzModelMissing(&m)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic until download, got %q", m.turnMode)
	}
	if !m.dlPrompt {
		t.Error("expected download prompt")
	}
}

func TestTurnMode_NKeepsBasic(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzModelMissing(&m)

	// Show prompt.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if !m.dlPrompt {
		t.Fatal("expected download prompt")
	}

	// Press n.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	if m.dlPrompt {
		t.Error("prompt should be dismissed after n")
	}
	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic after n, got %q", m.turnMode)
	}
}

func TestTurnMode_YStartsDownload_Success(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzModelMissing(&m)

	// Stub download that succeeds immediately.
	m.download = func(_ context.Context, progress func(done, total int64)) error {
		if progress != nil {
			progress(100, 100)
		}
		return nil
	}

	// Show prompt.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	// Press y.
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(Model)

	if !m.dlRunning {
		t.Error("expected download to be running after y")
	}
	if m.dlPrompt {
		t.Error("prompt should be dismissed")
	}

	// Drain messages from the download.
	if cmd != nil {
		for {
			msg := cmd()
			if msg == nil {
				break
			}
			result, cmd = m.Update(msg)
			m = result.(Model)
			if cmd == nil {
				break
			}
		}
	}

	if m.dlRunning {
		t.Error("download should have completed")
	}
	if m.turnMode != voicepkg.TurnModeTdrz {
		t.Errorf("mode after successful download = %q, want tdrz", m.turnMode)
	}
}

func TestTurnMode_DownloadFailure_KeepsBasic(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzModelMissing(&m)

	// Stub download that fails.
	m.download = func(_ context.Context, _ func(done, total int64)) error {
		return fmt.Errorf("network error")
	}

	// Show prompt.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	// Press y.
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(Model)

	// Drain messages.
	if cmd != nil {
		for {
			msg := cmd()
			if msg == nil {
				break
			}
			result, cmd = m.Update(msg)
			m = result.(Model)
			if cmd == nil {
				break
			}
		}
	}

	if m.dlRunning {
		t.Error("download should have completed")
	}
	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode after failed download = %q, want basic", m.turnMode)
	}
	if m.err == nil {
		t.Error("expected error after download failure")
	}
}

func TestTurnMode_QDuringDownloadCancels(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzModelMissing(&m)

	cancelDone := make(chan struct{})
	m.download = func(ctx context.Context, _ func(done, total int64)) error {
		<-ctx.Done()
		close(cancelDone)
		return ctx.Err()
	}

	// Show prompt, press y.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(Model)

	if !m.dlRunning {
		t.Fatal("expected download running")
	}

	// Press q: should cancel and quit.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = result.(Model)

	if !m.quitting {
		t.Error("expected quitting after q during download")
	}

	// Wait for the goroutine to notice the cancellation.
	select {
	case <-cancelDone:
		// Context cancellation reached the download goroutine.
	case <-time.After(2 * time.Second):
		t.Error("download context should have been cancelled within 2s")
	}
}

func TestTurnMode_HeaderShowsSpeakerSplit(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeTdrz)
	view := m.renderHeader()
	if !strings.Contains(view, "Speakers:") || !strings.Contains(view, "by voice") {
		t.Errorf("header should show 'Speakers: by voice', got: %s", view)
	}

	m = newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	view = m.renderHeader()
	if !strings.Contains(view, "by pause") {
		t.Errorf("header should show 'by pause', got: %s", view)
	}
	if strings.Contains(view, "tdrz") || strings.Contains(view, "Turns:") {
		t.Errorf("header should not expose internal mode names, got: %s", view)
	}
}

func TestTurnMode_StartRecordingFallbackResetsMode(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeTdrz)
	// The relay has no turn transcriber, so StartRecording(tdrz) will fall back.

	m.turnMode = voicepkg.TurnModeTdrz
	m.recState = recRecording

	// Simulate what happens in updateFilenamePrompt after opening a file.
	effective, fallbackErr := m.relay.StartRecording(m.turnMode)
	if fallbackErr != nil {
		m.turnMode = effective
		m.err = fallbackErr
	}

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("turnMode after fallback = %q, want basic", m.turnMode)
	}
	if m.err == nil {
		t.Error("expected fallback error")
	}
}

func TestTurnMode_SIgnoredWhilePrompting(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	m.recState = recPrompting
	// In the prompting state, keys go to the text input, so s should be a typed character.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.turnMode != voicepkg.TurnModeBasic {
		t.Errorf("mode should stay basic while prompting, got %q", m.turnMode)
	}
}

func TestTurnMode_FooterShowsSpeakerSplitHintWhenIdle(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	m.recState = recIdle
	footer := m.renderFooter()

	if !strings.Contains(footer, "s: speaker split") {
		t.Errorf("idle footer should show s: speaker split, got: %s", footer)
	}
}

func TestTurnMode_FooterHidesSpeakerSplitHintWhenRecording(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	m.recState = recRecording
	footer := m.renderFooter()

	if strings.Contains(footer, "s: speaker split") {
		t.Error("recording footer should not show s: speaker split")
	}
}

func TestTurnMode_ToggleShowsNoticeAndSchedulesClear(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	withTdrzAvailable(&m)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)
	if m.notice != "Next recording separates speakers by voice (English only)" {
		t.Errorf("notice after switching to voice = %q", m.notice)
	}
	if cmd == nil {
		t.Fatal("toggle should return a command that clears the notice")
	}
	if !strings.Contains(m.renderFooter(), "separates speakers by voice") {
		t.Error("footer should show the notice")
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)
	if m.notice != "Next recording separates speakers by pauses" {
		t.Errorf("notice after switching back = %q", m.notice)
	}
}

func TestTurnMode_ClearNoticeIgnoresStaleTimer(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	_ = m.setNotice("first")
	staleSeq := m.noticeSeq
	_ = m.setNotice("second")

	result, _ := m.Update(clearNoticeMsg{seq: staleSeq})
	m = result.(Model)
	if m.notice != "second" {
		t.Errorf("stale timer cleared the newer notice, notice = %q", m.notice)
	}

	result, _ = m.Update(clearNoticeMsg{seq: m.noticeSeq})
	m = result.(Model)
	if m.notice != "" {
		t.Errorf("current timer should clear the notice, notice = %q", m.notice)
	}
}

func TestTurnMode_ClearNoticeWorksInReadingView(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	_ = m.setNotice("hello")
	m.reading = true

	result, _ := m.Update(clearNoticeMsg{seq: m.noticeSeq})
	m = result.(Model)
	if m.notice != "" {
		t.Errorf("notice should clear even while the reading view is open, notice = %q", m.notice)
	}
}

func TestTurnMode_ErrorTakesPrecedenceOverNotice(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	_ = m.setNotice("a notice")
	m.err = fmt.Errorf("boom")

	footer := m.renderFooter()
	if strings.Contains(footer, "a notice") {
		t.Error("an error should hide the notice")
	}
	if !strings.Contains(footer, "boom") {
		t.Error("footer should show the error")
	}
}

func TestTurnMode_DownloadSuccessShowsNotice(t *testing.T) {
	m := newTestModelWithTurnMode(voicepkg.TurnModeBasic)
	m.dlRunning = true

	cmd := m.handleDownloadComplete(downloadMsg{})
	if cmd == nil {
		t.Fatal("download success should schedule the notice clear")
	}
	if m.turnMode != voicepkg.TurnModeTdrz {
		t.Errorf("mode after successful download = %q, want tdrz", m.turnMode)
	}
	if !strings.HasPrefix(m.notice, "Voice model installed.") {
		t.Errorf("notice after download = %q", m.notice)
	}
}

func TestTruncateToWidth(t *testing.T) {
	if got := truncateToWidth("short", 10); got != "short" {
		t.Errorf("truncateToWidth(short, 10) = %q", got)
	}
	if got := truncateToWidth("a long notice text", 10); got != "a long ..." {
		t.Errorf("truncateToWidth = %q, want %q", got, "a long ...")
	}
	if got := truncateToWidth("abc", 0); got != "abc" {
		t.Errorf("width 0 should not truncate, got %q", got)
	}
}
