package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	headerCountRunning = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	headerCountStopped = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	headerCountError = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("245")).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240"))

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("15"))

	normalRowStyle = lipgloss.NewStyle()

	// Status indicators
	statusRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	statusStopped  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("○")
	statusCreating = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("◎")
	statusError    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✕")
	statusUnknown  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("◌")

	// Session health indicators
	sessionHealthy   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	sessionAttention = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚠")

	// Footer styles
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Separator
	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Dialog styles
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196"))

	// Help overlay styles
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))

	helpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252")).
				MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	// Error message style
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// Empty state style
	emptyStateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)

	// Create wizard styles
	wizardLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Width(12)

	wizardActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	wizardInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

// statusIndicator returns the styled indicator for an environment state.
func statusIndicator(state string) string {
	switch state {
	case "running":
		return statusRunning
	case "stopped":
		return statusStopped
	case "creating":
		return statusCreating
	case "error":
		return statusError
	default:
		return statusUnknown
	}
}
