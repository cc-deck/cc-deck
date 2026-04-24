package voice

import (
	"fmt"
	"strings"
	"time"
)

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Voice relay stopped.\n"
	}

	var b strings.Builder

	// Header
	b.WriteString("Voice Relay")
	if m.target != "" {
		fmt.Fprintf(&b, "  >  %s", m.target)
	}
	b.WriteString("\n")

	// Mode indicator
	modeLabel := "VAD (auto)"
	if m.mode == "ptt" {
		modeLabel = "PTT (F8)"
	}
	fmt.Fprintf(&b, "Mode: %s\n", modeLabel)

	// Audio level meter
	b.WriteString("Level: ")
	b.WriteString(levelBar(m.audioLevel))
	b.WriteString("\n\n")

	// Error
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.err)
	}

	// Transcription history
	if len(m.history) == 0 {
		b.WriteString("Listening...\n")
	} else {
		for _, entry := range m.history {
			icon := " "
			switch entry.status {
			case "delivered":
				icon = "+"
			case "error":
				icon = "!"
			}
			ts := entry.at.Format("15:04:05")
			if m.verbose {
				fmt.Fprintf(&b, " %s [%s] (%s) %s\n", icon, ts, entry.latency.Round(time.Millisecond), entry.text)
			} else {
				fmt.Fprintf(&b, " %s [%s] %s\n", icon, ts, entry.text)
			}
		}
	}

	// Footer
	b.WriteString("\n  q: quit  m: toggle mode\n")

	return b.String()
}

func levelBar(level float64) string {
	const barLen = 30
	filled := int(level * float64(barLen) * 10)
	if filled < 0 {
		filled = 0
	}
	if filled > barLen {
		filled = barLen
	}
	return "[" + strings.Repeat("|", filled) + strings.Repeat(" ", barLen-filled) + "]"
}
