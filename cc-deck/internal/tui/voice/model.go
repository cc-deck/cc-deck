package voice

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

const maxHistoryLen = 200

// Model is the Bubbletea model for the voice relay TUI.
type Model struct {
	relay       *voicepkg.VoiceRelay
	muted       bool
	audioLevel  float64
	history     []historyEntry
	target      string
	session     string
	logPath     string
	devices     []voicepkg.DeviceInfo
	devicePick  bool
	deviceIdx   int
	quitting    bool
	err         error

	recState  recStatus
	recFile   *os.File
	recPath   string
	recCount  int
	recInput  textinput.Model

	width         int
	height        int
	viewport      viewport.Model
	viewportReady bool
}

type historyEntry struct {
	text    string
	latency time.Duration
	status  string // "transcribed", "delivered", "error"
	at      time.Time
}

type relayEventMsg voicepkg.RelayEvent

// New creates a new voice TUI model.
func New(relay *voicepkg.VoiceRelay, target string, logPath string) Model {
	ti := textinput.New()
	ti.Placeholder = "transcript.txt"
	ti.CharLimit = 256
	return Model{
		relay:    relay,
		target:   target,
		logPath:  logPath,
		recInput: ti,
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
			return tea.QuitMsg{}
		}
		return relayEventMsg(ev)
	}
}
