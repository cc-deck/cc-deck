package voice

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	targetStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	threshStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	tsStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	delivStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	pendStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	deviceStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	pickerTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	pickerActive = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	pickerNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	pickerDef    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

var brailleFill = []rune{'⠀', '⣀', '⣤', '⣶', '⣿'}

var levelColors = []lipgloss.Color{
	"22", "28", "34", "40", "46",
	"82", "118", "154", "190",
	"226", "220", "214", "208",
	"202", "196",
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Voice relay stopped.\n"
	}

	if m.devicePick {
		return m.viewDevicePicker()
	}

	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Voice Relay"))
	if m.target != "" {
		b.WriteString("  ")
		b.WriteString(targetStyle.Render(m.target))
	}
	b.WriteString("\n")

	// Device + Mode line
	b.WriteString(labelStyle.Render("Device: "))
	if m.deviceName != "" {
		b.WriteString(deviceStyle.Render(m.deviceName))
	} else {
		b.WriteString(deviceStyle.Render("(default)"))
	}
	b.WriteString("  ")
	modeLabel := "VAD (auto)"
	if m.mode == "ptt" {
		modeLabel = "PTT (F8)"
	}
	b.WriteString(labelStyle.Render("Mode: "))
	b.WriteString(modeLabel)
	b.WriteString("\n")

	// Audio level meter with braille bar and threshold
	threshold := m.relay.VADThreshold()
	b.WriteString(labelStyle.Render("Level "))
	b.WriteString(renderBrailleBar(m.audioLevel, threshold))
	b.WriteString("  ")
	b.WriteString(threshStyle.Render(fmt.Sprintf("T:%.3f", threshold)))
	b.WriteString("\n\n")

	// Error
	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Transcription history
	if len(m.history) == 0 {
		b.WriteString(hintStyle.Render("Listening..."))
		b.WriteString("\n")
	} else {
		for _, entry := range m.history {
			ts := tsStyle.Render(entry.at.Format("15:04:05"))
			var icon, text string
			switch entry.status {
			case "delivered":
				icon = delivStyle.Render("+")
				text = textStyle.Render(entry.text)
			case "error":
				icon = errStyle.Render("!")
				text = errStyle.Render(entry.text)
			default:
				icon = pendStyle.Render("~")
				text = pendStyle.Render(entry.text)
			}
			if m.verbose {
				lat := tsStyle.Render(fmt.Sprintf("(%s)", entry.latency.Round(time.Millisecond)))
				fmt.Fprintf(&b, " %s %s %s %s\n", icon, ts, lat, text)
			} else {
				fmt.Fprintf(&b, " %s %s %s\n", icon, ts, text)
			}
		}
	}

	// Log indicator
	if m.logPath != "" {
		b.WriteString(tsStyle.Render(fmt.Sprintf("  log: %s", m.logPath)))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  q: quit  +/-: threshold  d: device"))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewDevicePicker() string {
	var b strings.Builder

	b.WriteString(pickerTitle.Render("Select Audio Device"))
	b.WriteString("\n\n")

	for i, dev := range m.devices {
		cursor := "  "
		style := pickerNormal
		if i == m.deviceIdx {
			cursor = "> "
			style = pickerActive
		}

		name := dev.Name
		if dev.IsDefault {
			name += pickerDef.Render(" (default)")
		}

		b.WriteString(style.Render(cursor + name))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  enter: select  esc: cancel"))
	b.WriteString("\n")

	return b.String()
}

func renderBrailleBar(level, threshold float64) string {
	const barLen = 20

	filled := level * float64(barLen) * 10
	threshIdx := int(threshold * float64(barLen) * 10)
	if threshIdx < 0 {
		threshIdx = 0
	}
	if threshIdx > barLen {
		threshIdx = barLen
	}

	var sb strings.Builder
	for i := 0; i < barLen; i++ {
		pos := float64(i)
		var ch rune
		if pos+1 <= filled {
			ch = brailleFill[4]
		} else if pos < filled {
			frac := filled - pos
			idx := int(frac * float64(len(brailleFill)-1))
			if idx >= len(brailleFill) {
				idx = len(brailleFill) - 1
			}
			ch = brailleFill[idx]
		} else {
			ch = brailleFill[0]
		}

		colorIdx := i * (len(levelColors) - 1) / barLen
		if colorIdx >= len(levelColors) {
			colorIdx = len(levelColors) - 1
		}

		var style lipgloss.Style
		if i == threshIdx {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			if ch == brailleFill[0] {
				ch = '⠿'
			}
		} else if float64(i) >= filled {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
		} else {
			style = lipgloss.NewStyle().Foreground(levelColors[colorIdx])
		}

		sb.WriteString(style.Render(string(ch)))
	}

	return sb.String()
}
