package voice

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	headerLines = 5 // title, device+mode, level bar, blank line, separator
	footerLines = 5 // separator, log path, blank, keybindings, trailing newline
)

// Update handles incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.devicePick {
		return m.updateDevicePicker(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - headerLines - footerLines
		if vpHeight < 1 {
			vpHeight = 1
		}
		vpWidth := m.width - 1 // reserve 1 column for scrollbar
		if vpWidth < 1 {
			vpWidth = 1
		}
		if !m.viewportReady {
			m.viewport = newViewport(vpWidth, vpHeight)
			m.viewportReady = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.relay.Stop()
			return m, tea.Quit
		case "up", "+":
			m.relay.SetVADThreshold(m.relay.VADThreshold() + 2)
			return m, nil
		case "down", "-":
			m.relay.SetVADThreshold(m.relay.VADThreshold() - 2)
			return m, nil
		case "d":
			devices, err := m.relay.ListDevices()
			if err != nil || len(devices) == 0 {
				return m, nil
			}
			m.devices = devices
			m.devicePick = true
			m.deviceIdx = 0
			return m, nil
		case "pgup", "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
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
			m.syncViewport()
		case "delivery":
			if len(m.history) > 0 {
				m.history[len(m.history)-1].status = "delivered"
			}
			m.syncViewport()
		case "error":
			m.err = msg.Err
		case "paused":
			// TUI shows paused state via view
		}
		return m, waitForEvent(m.relay)
	}

	return m, nil
}

func (m *Model) syncViewport() {
	if !m.viewportReady {
		return
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

func (m Model) updateDevicePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.deviceIdx > 0 {
				m.deviceIdx--
			}
		case "down", "j":
			if m.deviceIdx < len(m.devices)-1 {
				m.deviceIdx++
			}
		case "enter":
			if m.deviceIdx < len(m.devices) {
				m.deviceName = m.devices[m.deviceIdx].Name
			}
			m.devicePick = false
			m.devices = nil
		case "esc", "q", "d":
			m.devicePick = false
			m.devices = nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case relayEventMsg:
		switch msg.Type {
		case "level":
			m.audioLevel = msg.Level
		}
		return m, waitForEvent(m.relay)
	}
	return m, nil
}
