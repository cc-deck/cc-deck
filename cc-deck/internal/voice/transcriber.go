package voice

import "context"

// Transcriber converts audio samples to text using a local speech
// recognition backend.
type Transcriber interface {
	// Transcribe converts PCM audio samples to text.
	// Audio must be mono, signed 16-bit, at the specified sample rate.
	// Returns empty string for silence/noise (not an error).
	Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error)

	// Close releases resources. For HTTP transcriber, stops the server
	// if it was auto-started. Safe to call multiple times.
	Close() error
}

// TranscriptionResult holds the output of a transcription plus
// command detection metadata.
type TranscriptionResult struct {
	Text          string
	IsCommand     bool
	CommandAction string // action name (e.g. "submit") when IsCommand is true
}
