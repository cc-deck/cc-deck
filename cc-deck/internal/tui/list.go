package tui

import (
	"fmt"
	"strings"
	"time"
)

// viewList renders the main environment list view.
func (m model) viewList() string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", min(m.width, 80))))
	b.WriteString("\n")

	if len(m.envs) == 0 {
		b.WriteString("\n")
		b.WriteString(emptyStateStyle.Render("  No environments found. Press 'n' to create one."))
		b.WriteString("\n")
	} else {
		// Table header
		b.WriteString(m.renderTableHeader())
		b.WriteString("\n")

		// Table rows
		availableHeight := m.height - 6 // header + separator + table header + footer + separator + padding
		rowCount := len(m.envs)
		if rowCount > availableHeight && availableHeight > 0 {
			rowCount = availableHeight
		}

		for i := 0; i < rowCount && i < len(m.envs); i++ {
			row := m.renderRow(i)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	// Confirmation dialog overlay
	if m.confirm != nil {
		return m.confirm.View(m.width, m.height)
	}

	// Error / status message
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  Error: " + m.err.Error()))
		b.WriteString("\n")
	} else if m.message != "" {
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("  " + m.message))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(separatorStyle.Render(strings.Repeat("─", min(m.width, 80))))
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderHeader renders the aggregate header with environment counts.
func (m model) renderHeader() string {
	running, stopped, creating, errCount := 0, 0, 0, 0
	for _, e := range m.envs {
		switch e.state {
		case "running":
			running++
		case "stopped":
			stopped++
		case "creating":
			creating++
		case "error":
			errCount++
		}
	}

	title := headerStyle.Render(" cc-deck")
	counts := ""
	if running > 0 {
		counts += "  " + headerCountRunning.Render(fmt.Sprintf("%d running", running))
	}
	if stopped > 0 {
		counts += "  " + headerCountStopped.Render(fmt.Sprintf("%d stopped", stopped))
	}
	if creating > 0 {
		counts += "  " + headerCountStopped.Render(fmt.Sprintf("%d creating", creating))
	}
	if errCount > 0 {
		counts += "  " + headerCountError.Render(fmt.Sprintf("%d error", errCount))
	}

	return title + counts
}

// renderTableHeader renders the column headers.
func (m model) renderTableHeader() string {
	cols := m.columnWidths()
	return tableHeaderStyle.Render(fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s",
		cols[0], "NAME",
		cols[1], "TYPE",
		cols[2], "STATUS",
		cols[3], "STORAGE",
		cols[4], "LAST ATTACHED",
	))
}

// renderRow renders a single environment row.
func (m model) renderRow(idx int) string {
	e := m.envs[idx]
	cols := m.columnWidths()

	indicator := statusIndicator(e.state)
	lastAttached := "never"
	if e.lastAttached != nil {
		lastAttached = formatRelative(*e.lastAttached)
	}

	cursor := "  "
	if idx == m.cursor {
		cursor = " ▸"
	}

	row := fmt.Sprintf("%s%-*s  %-*s  %s %-*s  %-*s  %-*s",
		cursor,
		cols[0], truncate(e.name, cols[0]),
		cols[1], e.envType,
		indicator,
		cols[2]-2, truncate(e.state, cols[2]-2),
		cols[3], e.storageName,
		cols[4], lastAttached,
	)

	if idx == m.cursor {
		return selectedRowStyle.Render(row)
	}
	return normalRowStyle.Render(row)
}

// renderFooter renders context-sensitive key hints.
func (m model) renderFooter() string {
	hints := " ↑↓/jk navigate  Enter attach  n new  S start  X stop  d delete  ? help  q quit"
	return footerStyle.Render(hints)
}

// columnWidths returns the column widths based on terminal width.
func (m model) columnWidths() [5]int {
	available := m.width - 10 // padding + cursor
	if available < 60 {
		available = 60
	}

	// Proportional allocation.
	nameW := available * 25 / 100
	typeW := available * 12 / 100
	statusW := available * 15 / 100
	storageW := available * 18 / 100
	attachedW := available * 18 / 100

	return [5]int{nameW, typeW, statusW, storageW, attachedW}
}

// formatRelative formats a time as a relative duration string.
func formatRelative(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd ago", days)
	}
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
