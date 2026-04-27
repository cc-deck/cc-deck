package voice

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.devicePick {
		return m.updateDevicePicker(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.relay.Stop()
			return m, tea.Quit
		case "up", "+":
			t := m.relay.VADThreshold()
			m.relay.SetVADThreshold(t + 0.005)
			return m, nil
		case "down", "-":
			t := m.relay.VADThreshold()
			m.relay.SetVADThreshold(t - 0.005)
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
	case relayEventMsg:
		switch msg.Type {
		case "level":
			m.audioLevel = msg.Level
		}
		return m, waitForEvent(m.relay)
	}
	return m, nil
}
