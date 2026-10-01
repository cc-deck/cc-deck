package voice

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

const (
	headerLines = 6 // title, title-separator, workspace, device+mode, level bar, separator
)

// handleRelayEvent processes a single relay event, updating model state.
// It is the single handler used by Update, updateFilenamePrompt, and
// updateDevicePicker so relay event processing is not duplicated.
func (m *Model) handleRelayEvent(msg relayEventMsg) {
	switch msg.Type {
	case "level":
		m.audioLevel = msg.Level
	case "transcription":
		m.err = nil
		m.history = append(m.history, historyEntry{
			text:    msg.Text,
			latency: msg.Latency,
			status:  "transcribed",
			at:      time.Now(),
		})
		if len(m.history) > maxHistoryLen {
			m.history = m.history[len(m.history)-maxHistoryLen:]
		}
		if m.recState == recRecording && m.recFile != nil {
			if err := writeTranscriptLine(m.recFile, msg.Text, m.recTimestamps); err != nil {
				m.err = err
				m.closeTranscript()
			} else {
				m.recCount++
			}
		}
		// Append segments to the reading buffer while recording.
		if m.recState == recRecording && msg.Segments != nil {
			added := m.appendSegments(msg.Segments)
			if m.reading {
				m.syncReading(added)
			}
		}
		m.resizeViewport()
		m.syncViewport()
	case "delivery":
		m.err = nil
		if len(m.history) > 0 {
			m.history[len(m.history)-1].status = "delivered"
		}
		m.resizeViewport()
		m.syncViewport()
	case "error":
		m.err = msg.Err
		m.resizeViewport()
	case "muted":
		m.muted = true
	case "unmuted":
		m.muted = false
	case "target_changed":
		m.session = msg.Text
	}
}

// Update handles incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.reading {
		return m.updateReading(msg)
	}
	if m.devicePick {
		return m.updateDevicePicker(msg)
	}
	if m.recState == recPrompting {
		return m.updateFilenamePrompt(msg)
	}

	// Download prompt: only y/n/esc are handled.
	if m.dlPrompt {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "y":
				cmd := m.startDownload()
				return m, cmd
			case "n", "esc":
				m.dlPrompt = false
				m.err = nil
				return m, nil
			}
		}
		if rm, ok := msg.(relayEventMsg); ok {
			m.handleRelayEvent(rm)
			return m, waitForEvent(m.relay)
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - headerLines - m.footerHeight()
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

	case downloadProgressMsg:
		cmd := m.handleDownloadProgress(msg)
		return m, cmd

	case downloadMsg:
		m.handleDownloadComplete(msg)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.dlRunning {
				m.cancelDownload()
			}
			m.closeTranscript()
			m.quitting = true
			return m, tea.Quit
		case "up", "+":
			m.relay.SetVADThreshold(m.relay.VADThreshold() + 2)
			return m, nil
		case "down", "-":
			m.relay.SetVADThreshold(m.relay.VADThreshold() - 2)
			return m, nil
		case "m":
			wantMuted := !m.muted
			return m, func() tea.Msg {
				var cmd string
				if wantMuted {
					cmd = "[[voice:mute]]"
				} else {
					cmd = "[[voice:unmute]]"
				}
				if err := m.relay.SendMuteCommand(cmd); err != nil {
					return relayEventMsg(voicepkg.RelayEvent{Type: "error", Err: err})
				}
				if wantMuted {
					return relayEventMsg(voicepkg.RelayEvent{Type: "muted"})
				}
				return relayEventMsg(voicepkg.RelayEvent{Type: "unmuted"})
			}
		case "d":
			devices, err := m.relay.ListDevices()
			if err != nil || len(devices) == 0 {
				return m, nil
			}
			m.devices = devices
			m.devicePick = true
			m.deviceIdx = 0
			return m, nil
		case "g":
			if m.recState == recIdle && !m.dlRunning {
				cmd := m.toggleTurnMode()
				return m, cmd
			}
			return m, nil
		case "r":
			switch m.recState {
			case recIdle:
				name := defaultTranscriptName()
				m.recInput.SetValue(name)
				m.recInput.SetCursor(len(name) - len(".txt"))
				m.recInput.Focus()
				m.recState = recPrompting
				m.resizeViewport()
			case recRecording:
				m.recState = recPaused
			case recPaused:
				m.recState = recRecording
			}
			return m, nil
		case "R":
			if m.recState == recRecording || m.recState == recPaused {
				m.closeTranscript()
				m.resizeViewport()
			}
			return m, nil
		case "v":
			if m.recState == recRecording || m.recState == recPaused {
				m.reading = true
				m.follow = true
				m.newBlocks = 0
				m.resizeReadingViewport()
				m.readView.GotoBottom()
			}
			return m, nil
		case "pgup", "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case relayEventMsg:
		m.handleRelayEvent(msg)
		return m, waitForEvent(m.relay)
	}

	return m, nil
}

func (m *Model) resizeViewport() {
	if !m.viewportReady || m.width == 0 || m.height == 0 {
		return
	}
	vpHeight := m.height - headerLines - m.footerHeight()
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Height = vpHeight
}

func (m *Model) syncViewport() {
	if !m.viewportReady {
		return
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

func (m Model) updateFilenamePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			name := m.recInput.Value()
			if name == "" {
				name = defaultTranscriptName()
			}
			path, err := resolveTranscriptPath(name)
			if err != nil {
				m.err = err
				m.recState = recIdle
				m.recInput.Blur()
				m.resizeViewport()
				return m, nil
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				m.err = err
				m.recState = recIdle
				m.recInput.Blur()
				m.resizeViewport()
				return m, nil
			}
			m.recFile = f
			m.recPath = path
			m.recCount = 0
			m.recBuffer = nil
			m.newBlocks = 0
			m.follow = true
			m.recState = recRecording
			effective, fallbackErr := m.relay.StartRecording(m.turnMode)
			if fallbackErr != nil {
				m.turnMode = effective
				m.err = fallbackErr
			}
			m.recInput.Blur()
			m.resizeViewport()
			return m, nil
		case "tab":
			m.recTimestamps = !m.recTimestamps
			return m, nil
		case "esc":
			m.recState = recIdle
			m.recInput.Blur()
			m.resizeViewport()
			return m, nil
		default:
			var cmd tea.Cmd
			m.recInput, cmd = m.recInput.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		return m, nil
	case relayEventMsg:
		m.handleRelayEvent(msg)
		return m, waitForEvent(m.relay)
	}
	return m, nil
}

func (m Model) updateReading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeReadingViewport()
		m.resizeViewport()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.reading = false
			return m, nil
		case "q", "ctrl+c":
			m.closeTranscript()
			m.quitting = true
			return m, tea.Quit
		case "r":
			switch m.recState {
			case recRecording:
				m.recState = recPaused
			case recPaused:
				m.recState = recRecording
			}
			return m, nil
		case "R":
			if m.recState == recRecording || m.recState == recPaused {
				m.closeTranscript()
				m.resizeViewport()
			}
			return m, nil
		case "up", "k":
			m.readView.LineUp(1)
			if !m.readView.AtBottom() {
				m.follow = false
			}
			return m, nil
		case "down", "j":
			m.readView.LineDown(1)
			if m.readView.AtBottom() {
				m.follow = true
				m.newBlocks = 0
			}
			return m, nil
		case "pgup":
			m.readView.HalfViewUp()
			if !m.readView.AtBottom() {
				m.follow = false
			}
			return m, nil
		case "pgdown":
			m.readView.HalfViewDown()
			if m.readView.AtBottom() {
				m.follow = true
				m.newBlocks = 0
			}
			return m, nil
		case "G", "end":
			m.readView.GotoBottom()
			m.follow = true
			m.newBlocks = 0
			return m, nil
		default:
			// +, -, d, m, g, v and all other keys are ignored
			return m, nil
		}
	case relayEventMsg:
		m.handleRelayEvent(msg)
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
		m.handleRelayEvent(msg)
		return m, waitForEvent(m.relay)
	}
	return m, nil
}
