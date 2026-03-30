package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// createModel is the create wizard sub-model.
type createModel struct {
	fields    []textinput.Model
	typeIndex int // 0 = local, 1 = container
	focus     int // currently focused field index
	image     string
	err       string
}

const (
	fieldName    = 0
	fieldImage   = 1
	fieldStorage = 2
)

func newCreateModel() createModel {
	nameInput := textinput.New()
	nameInput.Placeholder = "my-project"
	nameInput.Focus()
	nameInput.CharLimit = 64
	nameInput.Width = 40

	imageInput := textinput.New()
	imageInput.Placeholder = "quay.io/cc-deck/cc-deck-demo:latest"
	imageInput.CharLimit = 256
	imageInput.Width = 40

	storageInput := textinput.New()
	storageInput.Placeholder = "named-volume"
	storageInput.CharLimit = 64
	storageInput.Width = 40

	return createModel{
		fields: []textinput.Model{nameInput, imageInput, storageInput},
	}
}

// values returns the name and type from the wizard.
func (c createModel) values() (string, string) {
	name := strings.TrimSpace(c.fields[fieldName].Value())
	envType := "local"
	if c.typeIndex == 1 {
		envType = "container"
		c.image = strings.TrimSpace(c.fields[fieldImage].Value())
	}
	return name, envType
}

// Update handles key input for the create wizard.
func (c createModel) Update(msg tea.Msg) (createModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			return c.nextField()
		case "shift+tab", "up":
			return c.prevField()
		case "left", "right":
			if c.focus == -1 { // type selector focused
				c.typeIndex = 1 - c.typeIndex
				return c, nil
			}
		}
	}

	// Update the focused text input.
	if c.focus >= 0 && c.focus < len(c.fields) {
		var cmd tea.Cmd
		c.fields[c.focus], cmd = c.fields[c.focus].Update(msg)
		return c, cmd
	}

	return c, nil
}

func (c createModel) nextField() (createModel, tea.Cmd) {
	maxField := fieldName // local only has name
	if c.typeIndex == 1 {
		maxField = fieldStorage
	}

	// Special: focus == -1 means type selector.
	if c.focus == -1 {
		c.focus = 0
		if c.typeIndex == 1 && c.focus == fieldName {
			// Skip to name which is first.
		}
	} else if c.focus < maxField {
		c.fields[c.focus].Blur()
		c.focus++
	} else {
		// Wrap to type selector.
		c.fields[c.focus].Blur()
		c.focus = -1
		return c, nil
	}

	if c.focus >= 0 && c.focus < len(c.fields) {
		c.fields[c.focus].Focus()
	}
	return c, nil
}

func (c createModel) prevField() (createModel, tea.Cmd) {
	if c.focus == -1 {
		maxField := fieldName
		if c.typeIndex == 1 {
			maxField = fieldStorage
		}
		c.focus = maxField
	} else if c.focus > 0 {
		c.fields[c.focus].Blur()
		c.focus--
	} else {
		c.fields[c.focus].Blur()
		c.focus = -1
		return c, nil
	}

	if c.focus >= 0 && c.focus < len(c.fields) {
		c.fields[c.focus].Focus()
	}
	return c, nil
}

// View renders the create wizard.
func (c createModel) View(width, height int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("cc-deck > New Environment"))
	b.WriteString("\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteString("\n\n")

	// Name field
	label := wizardLabelStyle.Render("Name")
	b.WriteString(fmt.Sprintf("  %s %s\n", label, c.fields[fieldName].View()))

	// Type selector
	label = wizardLabelStyle.Render("Type")
	localStyle := wizardInactiveStyle
	containerStyle := wizardInactiveStyle
	if c.typeIndex == 0 {
		localStyle = wizardActiveStyle
	} else {
		containerStyle = wizardActiveStyle
	}
	selector := fmt.Sprintf("%s %s  %s %s",
		radioButton(c.typeIndex == 0), localStyle.Render("local"),
		radioButton(c.typeIndex == 1), containerStyle.Render("container"))
	b.WriteString(fmt.Sprintf("  %s %s\n", label, selector))

	// Container-specific fields
	if c.typeIndex == 1 {
		b.WriteString("\n")
		b.WriteString(separatorStyle.Render("  ── Container Settings ──"))
		b.WriteString("\n")
		label = wizardLabelStyle.Render("Image")
		b.WriteString(fmt.Sprintf("  %s %s\n", label, c.fields[fieldImage].View()))
		label = wizardLabelStyle.Render("Storage")
		b.WriteString(fmt.Sprintf("  %s %s\n", label, c.fields[fieldStorage].View()))
	}

	if c.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  Error: " + c.err))
	}

	b.WriteString("\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("  Tab/↓ next  Shift+Tab/↑ prev  Enter create  Esc cancel"))

	return b.String()
}

func radioButton(selected bool) string {
	if selected {
		return wizardActiveStyle.Render("●")
	}
	return wizardInactiveStyle.Render("○")
}
