package voice

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
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
	pickerTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	pickerActive = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	pickerNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	pickerDef    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	scrollThumb    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	scrollTrack    = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
)

var brailleFill = []rune{'⠀', '⣀', '⣤', '⣶', '⣿'}

var levelColors = []lipgloss.Color{
	"22", "28", "34", "40", "46",
	"82", "118", "154", "190",
	"226", "220", "214", "208",
	"202", "196",
}

func newViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Voice relay stopped.\n"
	}

	if m.devicePick {
		return m.viewDevicePicker()
	}

	if !m.viewportReady {
		return "Initializing...\n"
	}

	var b strings.Builder

	// Header (fixed top)
	b.WriteString(m.renderHeader())
	b.WriteString(m.renderSeparator())

	// Scrollable history with scrollbar
	b.WriteString(m.renderViewportWithScrollbar())

	// Footer (fixed bottom)
	b.WriteString(m.renderSeparator())
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m Model) renderHeader() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Voice Relay"))
	if m.target != "" {
		b.WriteString("  ")
		b.WriteString(targetStyle.Render(m.target))
	}
	b.WriteString("\n")

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

	threshPct := m.relay.VADThreshold()
	levelLog := voicepkg.RMSToLogScale(m.audioLevel)
	b.WriteString(labelStyle.Render("Level "))
	b.WriteString(renderBrailleBar(levelLog, float64(threshPct)/100.0))
	b.WriteString("  ")
	b.WriteString(threshStyle.Render(fmt.Sprintf("T:%d%%", threshPct)))
	b.WriteString("\n")

	b.WriteString("\n")

	return b.String()
}

func (m Model) renderFooter() string {
	var b strings.Builder

	if m.err != nil {
		errText := fmt.Sprintf("Error: %v", m.err)
		w := m.width - 4
		if w < 20 {
			w = 20
		}
		wrapped := errStyle.Width(w).Render(errText)
		b.WriteString("  ")
		b.WriteString(wrapped)
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  q: quit  +/-: threshold  d: device  pgup/pgdn: scroll"))

	return b.String()
}

func (m Model) footerHeight() int {
	lines := 2 // hints line + separator
	if m.err != nil {
		errText := fmt.Sprintf("Error: %v", m.err)
		w := m.width - 4
		if w < 20 {
			w = 20
		}
		lines += strings.Count(errStyle.Width(w).Render(errText), "\n") + 1
	} else {
		lines++ // blank status line
	}
	return lines
}

func (m Model) renderSeparator() string {
	w := m.width
	if w <= 0 {
		w = 40
	}
	return separatorStyle.Render(strings.Repeat("─", w)) + "\n"
}

func (m Model) renderViewportWithScrollbar() string {
	vpContent := m.viewport.View()
	lines := strings.Split(vpContent, "\n")
	totalContent := strings.Count(m.viewport.View(), "\n") + 1
	vpHeight := m.viewport.Height

	if totalContent <= vpHeight || vpHeight <= 0 {
		return vpContent + "\n"
	}

	thumbSize := vpHeight * vpHeight / totalContent
	if thumbSize < 1 {
		thumbSize = 1
	}
	scrollRange := totalContent - vpHeight
	thumbPos := 0
	if scrollRange > 0 {
		thumbPos = m.viewport.YOffset * (vpHeight - thumbSize) / scrollRange
	}
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > vpHeight {
		thumbPos = vpHeight - thumbSize
	}

	var sb strings.Builder
	for i := 0; i < vpHeight && i < len(lines); i++ {
		sb.WriteString(lines[i])
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(scrollThumb.Render("┃"))
		} else {
			sb.WriteString(scrollTrack.Render("│"))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m Model) renderHistory() string {
	if len(m.history) == 0 {
		return hintStyle.Render("Listening...")
	}

	var b strings.Builder
	for i, entry := range m.history {
		if i > 0 {
			b.WriteString("\n")
		}
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
			fmt.Fprintf(&b, " %s %s %s %s", icon, ts, lat, text)
		} else {
			fmt.Fprintf(&b, " %s %s %s", icon, ts, text)
		}
	}

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

// renderBrailleBar renders a level meter where both level and threshold
// are on the same 0.0-1.0 logarithmic scale.
func renderBrailleBar(level, threshold float64) string {
	const barLen = 20

	filled := level * float64(barLen)
	threshIdx := int(threshold * float64(barLen))
	if threshIdx < 0 {
		threshIdx = 0
	}
	if threshIdx >= barLen {
		threshIdx = barLen - 1
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
		} else if pos >= filled {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
		} else {
			style = lipgloss.NewStyle().Foreground(levelColors[colorIdx])
		}

		sb.WriteString(style.Render(string(ch)))
	}

	return sb.String()
}
