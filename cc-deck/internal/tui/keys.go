package tui

import "github.com/charmbracelet/bubbles/key"

// globalKeys are available in all views.
type globalKeys struct {
	Quit    key.Binding
	Help    key.Binding
	Escape  key.Binding
	Refresh key.Binding
}

// listKeys are specific to the environment list view.
type listKeys struct {
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Attach key.Binding
	New    key.Binding
	Start  key.Binding
	Stop   key.Binding
	Delete key.Binding
}

// createKeys are specific to the create wizard.
type createKeys struct {
	NextField key.Binding
	PrevField key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
}

var defaultGlobalKeys = globalKeys{
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:    key.NewBinding(key.WithKeys("?", "f1"), key.WithHelp("?", "help")),
	Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Refresh: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
}

var defaultListKeys = listKeys{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	Attach: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "attach")),
	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Start:  key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "start")),
	Stop:   key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "stop")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
}

var defaultCreateKeys = createKeys{
	NextField: key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab/↓", "next")),
	PrevField: key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab/↑", "prev")),
	Confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}
