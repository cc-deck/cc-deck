package voice

import (
	"math"
	"time"
)

// VAD segments continuous audio into discrete utterances using
// energy-based speech detection.
type VAD struct {
	configFunc func() VADConfig
	sampleRate int
}

// NewVAD creates a voice activity detector with the given config.
// The config pointer is retained so threshold changes take effect immediately.
func NewVAD(config *VADConfig, sampleRate int) *VAD {
	return NewVADFunc(func() VADConfig { return *config }, sampleRate)
}

// NewVADFunc creates a VAD that takes a fresh config snapshot on every
// frame. This lets the caller change threshold, silence duration, and
// maximum utterance length while the stream is running (for example,
// when switching from dictation to recording settings). Pre-roll is
// captured once at creation time because the ring buffer size must
// not change mid-stream.
func NewVADFunc(params func() VADConfig, sampleRate int) *VAD {
	return &VAD{configFunc: params, sampleRate: sampleRate}
}

// Process reads PCM frames from the input channel and produces
// Utterances on the returned channel. The output channel is closed
// when the input channel is closed.
func (v *VAD) Process(frames <-chan []int16) <-chan Utterance {
	out := make(chan Utterance, 4)

	// Pre-roll is captured once: the ring buffer size must not change.
	initCfg := v.configFunc()
	preRollSamples := int(initCfg.PreRollDuration * float64(v.sampleRate))

	go func() {
		defer close(out)

		var (
			ringBuf        = make([]int16, 0, preRollSamples)
			utterance      []int16
			speaking       bool
			silenceSmpCnt  int
			speechSmpCnt   int
			totalSamples   int // total samples processed since stream start
			uttStartSample int // sample index where the utterance starts (onset - pre-roll)
		)

		for frame := range frames {
			// Take a fresh config snapshot each frame so threshold,
			// silence duration, and maximum can change at runtime.
			cfg := v.configFunc()
			silenceSamples := int(cfg.SilenceDuration * float64(v.sampleRate))
			hangoverSamples := int(cfg.HangoverDuration * float64(v.sampleRate))
			maxSamples := int(cfg.MaxUtteranceDuration * float64(v.sampleRate))
			minSpeechSamples := int(cfg.MinSpeechDuration * float64(v.sampleRate))

			frameRMS := rmsLevel(frame)
			frameSilent := frameRMS < cfg.Threshold

			if !speaking {
				ringBuf = append(ringBuf, frame...)
				if len(ringBuf) > preRollSamples {
					ringBuf = ringBuf[len(ringBuf)-preRollSamples:]
				}

				if !frameSilent {
					speaking = true
					silenceSmpCnt = 0
					speechSmpCnt = len(frame)
					// The utterance starts at (onset - pre-roll). The onset
					// is at totalSamples (current position before adding this
					// frame). Pre-roll is len(ringBuf) samples before that.
					uttStartSample = totalSamples - len(ringBuf)
					if uttStartSample < 0 {
						uttStartSample = 0
					}
					utterance = make([]int16, 0, v.sampleRate*2)
					utterance = append(utterance, ringBuf...)
					utterance = append(utterance, frame...)
					ringBuf = ringBuf[:0]
				}
			} else {
				utterance = append(utterance, frame...)

				if frameSilent {
					silenceSmpCnt += len(frame)
				} else {
					silenceSmpCnt = 0
					speechSmpCnt += len(frame)
				}

				if silenceSmpCnt >= silenceSamples || len(utterance) >= maxSamples {
					trimSamples := silenceSmpCnt - hangoverSamples
					if trimSamples > 0 && trimSamples < len(utterance) {
						trimmed := utterance[:len(utterance)-trimSamples]
						if len(trimmed) > 0 {
							utterance = trimmed
						}
					}

					if speechSmpCnt >= minSpeechSamples {
						uStart := samplesToDuration(uttStartSample, v.sampleRate)
						uEnd := uStart + samplesToDuration(len(utterance), v.sampleRate)
						out <- Utterance{
							Audio:      utterance,
							SampleRate: v.sampleRate,
							Start:      uStart,
							End:        uEnd,
						}
					}

					utterance = nil
					speaking = false
					silenceSmpCnt = 0
					speechSmpCnt = 0
				}
			}

			totalSamples += len(frame)
		}

		if speaking && len(utterance) > 0 {
			cfg := v.configFunc()
			minSpeechSamples := int(cfg.MinSpeechDuration * float64(v.sampleRate))
			if speechSmpCnt >= minSpeechSamples {
				uStart := samplesToDuration(uttStartSample, v.sampleRate)
				uEnd := uStart + samplesToDuration(len(utterance), v.sampleRate)
				out <- Utterance{
					Audio:      utterance,
					SampleRate: v.sampleRate,
					Start:      uStart,
					End:        uEnd,
				}
			}
		}
	}()

	return out
}

func rmsLevel(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s) / 32768.0
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// UtteranceDuration returns the duration of an utterance.
func UtteranceDuration(u Utterance) time.Duration {
	if u.SampleRate == 0 {
		return 0
	}
	return time.Duration(float64(len(u.Audio)) / float64(u.SampleRate) * float64(time.Second))
}

// samplesToDuration converts a sample count to a time.Duration.
func samplesToDuration(samples, sampleRate int) time.Duration {
	if sampleRate == 0 {
		return 0
	}
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}
