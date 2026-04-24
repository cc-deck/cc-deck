package voice

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

const maxHistoryLen = 10

// Model is the Bubbletea model for the voice relay TUI.
type Model struct {
	relay       *voicepkg.VoiceRelay
	mode        string // "vad" or "ptt"
	audioLevel  float64
	history     []historyEntry
	target      string
	verbose     bool
	quitting    bool
	err         error
}

type historyEntry struct {
	text    string
	latency time.Duration
	status  string // "transcribed", "delivered", "error"
	at      time.Time
}

type relayEventMsg voicepkg.RelayEvent
type levelTickMsg struct{}

// New creates a new voice TUI model.
func New(relay *voicepkg.VoiceRelay, mode string, target string, verbose bool) Model {
	return Model{
		relay:   relay,
		mode:    mode,
		target:  target,
		verbose: verbose,
	}
}

// Init starts the relay and subscribes to events.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForEvent(m.relay),
	)
}

func waitForEvent(relay *voicepkg.VoiceRelay) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-relay.Events()
		if !ok {
			return tea.Quit()
		}
		return relayEventMsg(ev)
	}
}
