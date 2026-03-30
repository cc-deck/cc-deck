package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpModel renders the help overlay.
type helpModel struct{}

func newHelpModel() helpModel {
	return helpModel{}
}

// View renders the help overlay content.
func (h helpModel) View(width, height int) string {
	var b strings.Builder

	title := helpTitleStyle.Render("cc-deck Help")
	b.WriteString(title)
	b.WriteString("\n\n")

	categories := []struct {
		name  string
		binds []struct{ key, desc string }
	}{
		{
			name: "NAVIGATION",
			binds: []struct{ key, desc string }{
				{"↑↓ / j k", "Move"},
				{"Enter", "Attach"},
				{"g / G", "Top / Bottom"},
				{"Esc", "Back"},
			},
		},
		{
			name: "LIFECYCLE",
			binds: []struct{ key, desc string }{
				{"n", "New environment"},
				{"S", "Start"},
				{"X", "Stop"},
				{"d", "Delete"},
			},
		},
		{
			name: "DISPLAY",
			binds: []struct{ key, desc string }{
				{"R", "Refresh"},
				{"?", "This help"},
				{"q", "Quit"},
			},
		},
	}

	for _, cat := range categories {
		b.WriteString(helpCategoryStyle.Render(cat.name))
		b.WriteString("\n")
		for _, bind := range cat.binds {
			k := helpKeyStyle.Render(fmt.Sprintf("  %-14s", bind.key))
			d := helpDescStyle.Render(bind.desc)
			b.WriteString(k + d + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("Press ? or Esc to close"))

	content := b.String()
	box := dialogBoxStyle.Render(content)

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		box)
}
