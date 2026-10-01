package voice

import (
	"context"
	"math"
	"time"
)

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
	Start      time.Duration // offset of first emitted sample (including pre-roll) from audio stream start
	End        time.Duration // Start plus emitted audio length (after hangover trim)
}

// RecordingConfig holds VAD parameters used while recording, separate from
// the dictation settings. These values keep silence detection active during
// recording so passages end after short pauses instead of running to the
// maximum chunk length.
type RecordingConfig struct {
	Threshold            float64       // RMS energy threshold (recording sensitivity)
	SilenceDuration      float64       // seconds of silence to end a recording passage
	MaxUtteranceDuration float64       // maximum seconds per recording passage
	PauseBreak           time.Duration // silence gap that starts a new turn
}

// DefaultRecordingConfig returns recording defaults: sensitivity 20% (on the
// 0-100 logarithmic scale), 1.0 second silence, 12 second maximum chunk, and
// a 3 second pause-break threshold.
func DefaultRecordingConfig() RecordingConfig {
	return RecordingConfig{
		Threshold:            PercentToThreshold(20),
		SilenceDuration:      1.0,
		MaxUtteranceDuration: 12,
		PauseBreak:           3 * time.Second,
	}
}

// VADConfig controls voice activity detection parameters.
type VADConfig struct {
	Threshold            float64 // RMS energy threshold for speech detection (default 0.015)
	PreRollDuration      float64 // Seconds of audio to keep before speech onset (default 0.3)
	SilenceDuration      float64 // Seconds of silence to end an utterance (default 2.5)
	HangoverDuration     float64 // Seconds of below-threshold audio to keep after last loud frame (default 0.3)
	MaxUtteranceDuration float64 // Maximum utterance length in seconds (default 30)
	MinSpeechDuration    float64 // Minimum above-threshold audio to emit an utterance (default 0.3)
}

// DefaultVADConfig returns the default VAD configuration.
func DefaultVADConfig() VADConfig {
	return VADConfig{
		Threshold:            0.015,
		PreRollDuration:      0.3,
		SilenceDuration:      2.5,
		HangoverDuration:     0.3,
		MaxUtteranceDuration: 30,
		MinSpeechDuration:    0.2,
	}
}

const (
	vadRMSMin = 0.001
	vadRMSMax = 0.5
)

// ThresholdToPercent converts an internal RMS threshold (0.001-0.5) to a
// 0-100 logarithmic scale. Low RMS values (where most tuning happens)
// spread across the lower half of the scale.
func ThresholdToPercent(rms float64) int {
	if rms <= vadRMSMin {
		return 0
	}
	if rms >= vadRMSMax {
		return 100
	}
	logMin := math.Log(vadRMSMin)
	logMax := math.Log(vadRMSMax)
	pct := (math.Log(rms) - logMin) / (logMax - logMin) * 100
	return int(math.Round(pct))
}

// RMSToLogScale converts an RMS level to a 0.0-1.0 logarithmic scale
// matching the same mapping as ThresholdToPercent.
func RMSToLogScale(rms float64) float64 {
	if rms <= vadRMSMin {
		return 0
	}
	if rms >= vadRMSMax {
		return 1
	}
	logMin := math.Log(vadRMSMin)
	logMax := math.Log(vadRMSMax)
	return (math.Log(rms) - logMin) / (logMax - logMin)
}

// PercentToThreshold converts a 0-100 logarithmic scale value back to
// an internal RMS threshold (0.001-0.5).
func PercentToThreshold(pct int) float64 {
	if pct <= 0 {
		return vadRMSMin
	}
	if pct >= 100 {
		return vadRMSMax
	}
	logMin := math.Log(vadRMSMin)
	logMax := math.Log(vadRMSMax)
	return math.Exp(logMin + float64(pct)/100*(logMax-logMin))
}
