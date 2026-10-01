package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// tdrzTranscriber implements TurnTranscriber by shelling out to whisper-cli
// with the -tdrz flag and parsing the JSON output.
type tdrzTranscriber struct {
	modelPath    string
	vadModelPath string // speech filter model; empty disables --vad

	mu     sync.Mutex
	prompt string

	// runCmd is the function that runs whisper-cli. It is a field so tests
	// can substitute a fake that writes canned JSON output.
	runCmd func(ctx context.Context, name string, args ...string) error
}

// NewTdrzTranscriber creates a TurnTranscriber that uses whisper-cli -tdrz.
// vadModelPath enables the speech filter (--vad); pass "" to disable it.
func NewTdrzTranscriber(modelPath, vadModelPath string) TurnTranscriber {
	return &tdrzTranscriber{
		modelPath:    modelPath,
		vadModelPath: vadModelPath,
		runCmd:       defaultRunCmd,
	}
}

func defaultRunCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// SetPrompt sets the glossary prompt passed as --prompt.
func (t *tdrzTranscriber) SetPrompt(prompt string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prompt = prompt
}

// Close is a no-op for the CLI transcriber.
func (t *tdrzTranscriber) Close() error {
	return nil
}

// TranscribeTurns writes audio to a temporary WAV file, runs whisper-cli
// -tdrz, parses the JSON output, and returns segments. The temporary
// directory is removed before returning.
func (t *tdrzTranscriber) TranscribeTurns(ctx context.Context, audio []int16, sampleRate int) ([]Segment, error) {
	tmpDir, err := os.MkdirTemp("", "cc-deck-tdrz-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wavPath := filepath.Join(tmpDir, "passage.wav")
	if err := writeWAVFile(wavPath, audio, sampleRate); err != nil {
		return nil, fmt.Errorf("writing WAV file: %w", err)
	}

	outPrefix := filepath.Join(tmpDir, "out")

	args := []string{
		"-m", t.modelPath,
		"-f", wavPath,
		"-tdrz",
		"-oj",
		"-of", outPrefix,
		"-np",
	}

	t.mu.Lock()
	prompt := t.prompt
	t.mu.Unlock()
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}
	if t.vadModelPath != "" {
		args = append(args, "--vad", "--vad-model", t.vadModelPath)
	}

	if err := t.runCmd(ctx, "whisper-cli", args...); err != nil {
		return nil, fmt.Errorf("whisper-cli failed: %w", err)
	}

	jsonPath := outPrefix + ".json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("reading tdrz output: %w", err)
	}

	segs, err := ParseTdrzJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parsing tdrz output: %w", err)
	}

	return segs, nil
}

// writeWAVFile is defined in transcriber_cli.go and reused here.
