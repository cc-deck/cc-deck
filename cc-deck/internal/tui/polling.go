package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cc-deck/cc-deck/internal/env"
)

// tickMsg is sent by the polling timer.
type tickMsg time.Time

// tickPoll returns a tea.Cmd that sends a tickMsg after the given interval.
func tickPoll(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// pollEnvs fetches the current environment list in a goroutine.
func pollEnvs(store *env.FileStateStore, defs *env.DefinitionStore) tea.Cmd {
	return func() tea.Msg {
		rows := buildEnvRows(store, defs)
		return envListMsg{envs: rows}
	}
}
