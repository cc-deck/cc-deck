package voice

import (
	"context"
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

	// Turn mode state
	turnMode   voicepkg.TurnMode                               // current turn detection mode
	tdrzStatus func() (toolErr, modelErr error)                // checks tdrz readiness
	download   func(ctx context.Context, progress func(done, total int64)) error // downloads tdrz model

	// Download state
	dlPrompt  bool             // true when prompting for download confirmation
	dlRunning bool             // true while a download is in progress
	dlDone    int64            // bytes downloaded so far
	dlTotal   int64            // total bytes expected
	dlCancel  context.CancelFunc // cancels the running download
	dlCh      <-chan tea.Msg   // channel for download progress/completion

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

// New creates a new voice TUI model. The turnMode sets the initial turn
// detection mode (basic or tdrz). Pass voicepkg.TurnModeBasic when no
// preference is configured.
func New(relay *voicepkg.VoiceRelay, target string, logPath string, turnMode voicepkg.TurnMode) Model {
	ti := textinput.New()
	ti.Placeholder = "transcript.txt"
	ti.CharLimit = 256
	return Model{
		relay:    relay,
		target:   target,
		logPath:  logPath,
		recInput: ti,
		turnMode: turnMode,
		tdrzStatus: func() (error, error) {
			return voicepkg.TdrzStatus()
		},
		download: func(ctx context.Context, progress func(done, total int64)) error {
			return voicepkg.DownloadModel(ctx, voicepkg.TdrzModelName, progress)
		},
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
