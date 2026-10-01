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

// turnBlock is a contiguous block of text from one speaker turn in
// the reading view. Each block starts when a segment has TurnStart
// set to true (a new dash marker, a pause break, or the first passage).
type turnBlock struct {
	at      time.Time // wall-clock time of the first segment in the block
	parts   []string  // text fragments joined into the block
	speaker string    // reserved for future speaker labels
}

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

	recState      recStatus
	recFile       *os.File
	recPath       string
	recCount      int
	recInput      textinput.Model
	recTimestamps bool

	// Reading view state
	recBuffer []turnBlock     // turn blocks accumulated during recording
	reading   bool            // true when the reading view is open
	readView  viewport.Model  // viewport for the reading view
	readReady bool            // true after the reading viewport is initialized
	follow    bool            // auto-scroll to bottom on new content
	newBlocks int             // count of new blocks since the user scrolled up

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
