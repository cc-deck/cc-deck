package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type cliTranscriber struct {
	modelPath string
}

// NewCLITranscriber creates a transcriber that invokes whisper-cli
// as a subprocess for each utterance.
func NewCLITranscriber(modelPath string) Transcriber {
	return &cliTranscriber{modelPath: modelPath}
}

func (t *cliTranscriber) Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error) {
	tmpDir, err := os.MkdirTemp("", "cc-deck-voice-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wavPath := filepath.Join(tmpDir, "utterance.wav")
	if err := writeWAVFile(wavPath, audio, sampleRate); err != nil {
		return "", fmt.Errorf("writing WAV file: %w", err)
	}

	cmd := exec.CommandContext(ctx, "whisper-cli",
		"-m", t.modelPath,
		"-f", wavPath,
		"--no-timestamps",
		"--output-txt",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running whisper-cli: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

func (t *cliTranscriber) Close() error {
	return nil
}

func writeWAVFile(path string, samples []int16, sampleRate int) error {
	data := pcmToWAV(samples, sampleRate)
	return os.WriteFile(path, data, 0o600)
}
