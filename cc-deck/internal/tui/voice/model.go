package voice

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	voicepkg "github.com/cc-deck/cc-deck/internal/voice"
)

const maxHistoryLen = 200

// Model is the Bubbletea model for the voice relay TUI.
type Model struct {
	relay       *voicepkg.VoiceRelay
	mode        string // "vad" or "ptt"
	audioLevel  float64
	history     []historyEntry
	target      string
	session     string
	verbose     bool
	logPath     string
	deviceName  string
	devices     []voicepkg.DeviceInfo
	devicePick  bool
	deviceIdx   int
	quitting    bool
	paused      bool
	err         error

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
type levelTickMsg struct{}
type pttStateMsg string // "recording", "waiting"

// New creates a new voice TUI model.
func New(relay *voicepkg.VoiceRelay, mode string, target string, verbose bool, logPath string, deviceName string) Model {
	return Model{
		relay:      relay,
		mode:       mode,
		target:     target,
		verbose:    verbose,
		logPath:    logPath,
		deviceName: deviceName,
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
