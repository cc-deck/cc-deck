package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmModel presents a confirmation dialog requiring the user to type
// the environment name to confirm a destructive operation.
type confirmModel struct {
	targetName string
	input      textinput.Model
	confirmed  bool
}

func newConfirmModel(name string) confirmModel {
	ti := textinput.New()
	ti.Placeholder = name
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 40

	return confirmModel{
		targetName: name,
		input:      ti,
	}
}

// Update handles key input for the confirmation dialog.
func (c confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if strings.TrimSpace(c.input.Value()) == c.targetName {
				c.confirmed = true
			}
			return c, nil
		}
	}

	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

// View renders the confirmation dialog.
func (c confirmModel) View(width, height int) string {
	var b strings.Builder

	title := dialogTitleStyle.Render(fmt.Sprintf("Delete environment %q?", c.targetName))
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("This will remove the environment and its resources.\n")
	b.WriteString("Data cannot be recovered.\n\n")
	b.WriteString("Type the environment name to confirm:\n")
	b.WriteString(c.input.View())
	b.WriteString("\n\n")
	b.WriteString(footerStyle.Render("Enter confirm  Esc cancel"))

	content := b.String()
	box := dialogBoxStyle.Render(content)

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		box)
}
