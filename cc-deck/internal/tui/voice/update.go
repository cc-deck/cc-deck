package voice

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.relay.Stop()
			return m, tea.Quit
		case "m":
			if m.mode == "vad" {
				m.mode = "ptt"
			} else {
				m.mode = "vad"
			}
			return m, nil
		}

	case relayEventMsg:
		switch msg.Type {
		case "level":
			m.audioLevel = msg.Level
		case "transcription":
			m.history = append(m.history, historyEntry{
				text:    msg.Text,
				latency: msg.Latency,
				status:  "transcribed",
				at:      time.Now(),
			})
			if len(m.history) > maxHistoryLen {
				m.history = m.history[len(m.history)-maxHistoryLen:]
			}
		case "delivery":
			if len(m.history) > 0 {
				m.history[len(m.history)-1].status = "delivered"
			}
		case "error":
			m.err = msg.Err
		case "paused":
			// TUI shows paused state via view
		}
		return m, waitForEvent(m.relay)
	}

	return m, nil
}
