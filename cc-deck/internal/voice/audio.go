package voice

import "context"

// AudioSource captures PCM audio from a local input device.
type AudioSource interface {
	// Start begins audio capture at the given sample rate (Hz).
	// Returns a channel of signed 16-bit mono PCM frames.
	// The channel is closed when Stop is called or an error occurs.
	// Callers MUST call Stop to release the device.
	Start(ctx context.Context, sampleRate int) (<-chan []int16, error)

	// Stop halts audio capture and releases the device.
	// Safe to call multiple times.
	Stop() error

	// Level returns the current RMS audio level (0.0 to 1.0)
	// for TUI visualization. Returns 0.0 if not capturing.
	Level() float64

	// ListDevices enumerates available audio input devices.
	ListDevices() ([]DeviceInfo, error)
}

// DeviceInfo describes an available audio input device.
type DeviceInfo struct {
	ID        string
	Name      string
	IsDefault bool
}

// Utterance represents a segmented audio chunk detected by VAD.
type Utterance struct {
	Audio      []int16
	SampleRate int
}

// VADConfig controls voice activity detection parameters.
type VADConfig struct {
	Threshold            float64 // RMS energy threshold for speech detection (default 0.015)
	PreRollDuration      float64 // Seconds of audio to keep before speech onset (default 0.3)
	SilenceDuration      float64 // Seconds of silence to end an utterance (default 1.5)
	MaxUtteranceDuration float64 // Maximum utterance length in seconds (default 30)
}

// DefaultVADConfig returns the default VAD configuration.
func DefaultVADConfig() VADConfig {
	return VADConfig{
		Threshold:            0.015,
		PreRollDuration:      0.3,
		SilenceDuration:      1.5,
		MaxUtteranceDuration: 30,
	}
}
