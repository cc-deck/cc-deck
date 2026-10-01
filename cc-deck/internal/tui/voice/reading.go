package voice

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

var (
	gutterEven = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	gutterOdd  = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	blockTime  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// appendSegments adds segments to the reading buffer. A new turn block
// is created when a segment has TurnStart set or when the buffer is
// empty. Otherwise the text is appended to the last block.
// Returns the number of new blocks added.
func (m *Model) appendSegments(segs []voicepkg.Segment) int {
	added := 0
	for _, seg := range segs {
		if seg.TurnStart || len(m.recBuffer) == 0 {
			m.recBuffer = append(m.recBuffer, turnBlock{
				at:    seg.At,
				parts: []string{seg.Text},
			})
			added++
		} else {
			last := &m.recBuffer[len(m.recBuffer)-1]
			last.parts = append(last.parts, seg.Text)
		}
	}
	return added
}

// renderBlocks produces the reading view content: a timestamp line
// followed by wrapped text with alternating gutter bars for each block.
func (m *Model) renderBlocks(width int) string {
	if len(m.recBuffer) == 0 {
		return hintStyle.Render("Waiting for speech...")
	}

	// Text width reserves 3 characters for the gutter prefix (" X ").
	textWidth := width - 3
	if textWidth < 10 {
		textWidth = 10
	}
	wrapStyle := lipgloss.NewStyle().Width(textWidth)

	var b strings.Builder
	for i, block := range m.recBuffer {
		if i > 0 {
			b.WriteString("\n")
		}

		// Timestamp line
		ts := block.at.Format("15:04:05")
		b.WriteString(" ")
		b.WriteString(blockTime.Render(ts))
		b.WriteString("\n")

		// Join parts and wrap
		joined := strings.Join(block.parts, " ")
		wrapped := wrapStyle.Render(joined)
		lines := strings.Split(wrapped, "\n")

		// Gutter bar: even blocks get ▌ (color 39), odd blocks get ┃ (color 105)
		var gutter string
		if i%2 == 0 {
			gutter = gutterEven.Render(" ▌")
		} else {
			gutter = gutterOdd.Render(" ┃")
		}

		for _, line := range lines {
			b.WriteString(gutter)
			b.WriteString(" ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// syncReading updates the reading viewport content and manages the
// follow/scroll state. It checks whether the viewport was at the
// bottom before updating content, then either follows (scrolls to
// bottom) or freezes the offset and counts new blocks.
func (m *Model) syncReading(added int) {
	if !m.readReady {
		return
	}

	wasAtBottom := m.readView.AtBottom()

	content := m.renderBlocks(m.readView.Width)
	m.readView.SetContent(content)

	if m.follow {
		m.readView.GotoBottom()
	} else {
		// When not following, count the new blocks for the footer indicator.
		m.newBlocks += added
		// If the user scrolled back to the bottom, resume following.
		if wasAtBottom {
			m.follow = true
			m.newBlocks = 0
			m.readView.GotoBottom()
		}
	}
}

// resizeReadingViewport adjusts the reading viewport dimensions on
// a window size change. The reading view uses a simpler layout:
// 1 header line + 1 separator + viewport + 1 separator + 1 footer = 4 fixed lines.
func (m *Model) resizeReadingViewport() {
	vpHeight := m.height - 4
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := m.width - 1 // 1 column for scrollbar
	if vpWidth < 1 {
		vpWidth = 1
	}

	if !m.readReady {
		m.readView = newViewport(vpWidth, vpHeight)
		m.readReady = true
	} else {
		m.readView.Width = vpWidth
		m.readView.Height = vpHeight
	}

	// Re-render with the new width.
	content := m.renderBlocks(vpWidth)
	m.readView.SetContent(content)
	if m.follow {
		m.readView.GotoBottom()
	}
}

// readingViewportWithScrollbar renders the reading viewport with a
// scrollbar, reusing the same scrollbar style as the normal view.
func (m Model) readingViewportWithScrollbar() string {
	vpContent := m.readView.View()
	lines := strings.Split(vpContent, "\n")
	totalContent := strings.Count(m.readView.View(), "\n") + 1
	vpHeight := m.readView.Height

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
		thumbPos = m.readView.YOffset * (vpHeight - thumbSize) / scrollRange
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

// renderReadingHeader produces the single header line for the reading view.
func (m Model) renderReadingHeader() string {
	var b strings.Builder
	b.WriteString(" ")
	b.WriteString(headerStyle.Render("Reading"))
	b.WriteString("  ")
	switch m.recState {
	case recRecording:
		b.WriteString(recStyle.Render("● REC"))
	case recPaused:
		b.WriteString(pauseStyle.Render("⏸ PAUSED"))
	}
	if m.recPath != "" {
		b.WriteString("  ")
		base := m.recPath
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		b.WriteString(labelStyle.Render(base))
	}
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Turns: basic"))
	b.WriteString("\n")
	return b.String()
}

// renderReadingFooter produces the footer line for the reading view.
func (m Model) renderReadingFooter() string {
	var b strings.Builder
	b.WriteString(hintStyle.Render(" ↑↓/jk scroll  PgUp/PgDn page  G end  esc back"))
	b.WriteString("    ")
	if m.follow {
		b.WriteString(recStyle.Render("● following"))
	} else if m.newBlocks > 0 {
		b.WriteString(hintStyle.Render(fmt.Sprintf("%d new ↓", m.newBlocks)))
	}
	return b.String()
}
