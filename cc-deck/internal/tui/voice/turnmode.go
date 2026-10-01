package voice

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

// downloadMsg carries the result of a background model download.
type downloadMsg struct {
	err error
}

// downloadProgressMsg carries progress from a background download.
type downloadProgressMsg struct {
	done  int64
	total int64
}

// toggleTurnMode switches between basic and tdrz turn modes. When switching
// to tdrz it checks tool and model availability via tdrzStatus. If the tool
// is missing, an error is shown and the mode stays basic. If only the model
// is missing, the download prompt is shown. Going back to basic always works.
func (m *Model) toggleTurnMode() tea.Cmd {
	if m.turnMode == voicepkg.TurnModeTdrz {
		m.turnMode = voicepkg.TurnModeBasic
		return nil
	}

	// Check tdrz availability.
	toolErr, modelErr := m.tdrzStatus()

	if toolErr != nil {
		m.err = fmt.Errorf("turn mode tdrz needs whisper-cli (install whisper-cpp, e.g. brew install whisper-cpp)")
		return nil
	}

	if modelErr != nil {
		m.dlPrompt = true
		m.err = fmt.Errorf("tdrz model not installed (488 MB). Download now? [y/n]")
		return nil
	}

	// Both tool and model present.
	m.turnMode = voicepkg.TurnModeTdrz
	return nil
}

// startDownload begins a background download of the tdrz model. It spawns a
// goroutine that sends progress and completion messages through a channel
// consumed by a bubbletea command.
func (m *Model) startDownload() tea.Cmd {
	m.dlPrompt = false
	m.dlRunning = true
	m.dlDone = 0
	m.dlTotal = 0
	m.err = nil

	ctx, cancel := context.WithCancel(context.Background())
	m.dlCancel = cancel

	ch := make(chan tea.Msg, 16)
	m.dlCh = ch

	go func() {
		err := m.download(ctx, func(done, total int64) {
			select {
			case ch <- downloadProgressMsg{done: done, total: total}:
			default:
			}
		})
		ch <- downloadMsg{err: err}
		close(ch)
	}()

	return waitForDownload(ch)
}

// waitForDownload returns a tea.Cmd that reads the next message from the
// download channel.
func waitForDownload(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return downloadMsg{err: fmt.Errorf("download channel closed")}
		}
		return msg
	}
}

// handleDownloadProgress updates download progress state.
func (m *Model) handleDownloadProgress(msg downloadProgressMsg) tea.Cmd {
	m.dlDone = msg.done
	m.dlTotal = msg.total
	if m.dlCh != nil {
		return waitForDownload(m.dlCh)
	}
	return nil
}

// handleDownloadComplete handles download completion (success or failure).
func (m *Model) handleDownloadComplete(msg downloadMsg) {
	m.dlRunning = false
	m.dlCancel = nil
	m.dlCh = nil

	if msg.err != nil {
		m.err = fmt.Errorf("tdrz model download failed: %w", msg.err)
		// Mode stays basic on failure.
		return
	}

	m.turnMode = voicepkg.TurnModeTdrz
	m.err = nil
}

// cancelDownload cancels a running download.
func (m *Model) cancelDownload() {
	if m.dlCancel != nil {
		m.dlCancel()
	}
}
