package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// recStatus represents the current recording state.
type recStatus int

const (
	recIdle      recStatus = iota // no recording active
	recPrompting                  // filename prompt visible
	recRecording                  // actively writing to file
	recPaused                     // file open but not writing
)

// defaultTranscriptDir returns the standard directory for transcript files.
func defaultTranscriptDir() string {
	return filepath.Join(xdg.DataHome, "cc-deck", "transcripts")
}

// resolveTranscriptPath resolves a filename to an absolute path.
// Absolute paths are used as-is. Relative names are placed in the
// default transcript directory, which is created if it does not exist.
func resolveTranscriptPath(name string) (string, error) {
	if filepath.IsAbs(name) {
		dir := filepath.Dir(name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("creating directory %s: %w", dir, err)
		}
		return name, nil
	}
	dir := defaultTranscriptDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating transcript directory: %w", err)
	}
	return filepath.Join(dir, name), nil
}

// defaultTranscriptName returns a timestamped filename for a new transcript.
func defaultTranscriptName() string {
	return "transcript-" + time.Now().Format("2006-01-02T15-04-05") + ".txt"
}

// writeTranscriptLine appends a single line of text to the transcript file.
func writeTranscriptLine(f *os.File, text string) error {
	_, err := fmt.Fprintln(f, text)
	return err
}

// closeTranscript closes the transcript file and resets recording state.
func (m *Model) closeTranscript() {
	if m.recFile != nil {
		_ = m.recFile.Close()
		m.recFile = nil
	}
	m.recState = recIdle
	m.recPath = ""
	m.recCount = 0
	if m.relay != nil {
		m.relay.SetRecording(false)
	}
}
