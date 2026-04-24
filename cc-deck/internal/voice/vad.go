package voice

import (
	"math"
	"time"
)

// VAD segments continuous audio into discrete utterances using
// energy-based speech detection.
type VAD struct {
	config     VADConfig
	sampleRate int
}

// NewVAD creates a voice activity detector with the given config.
func NewVAD(config VADConfig, sampleRate int) *VAD {
	return &VAD{config: config, sampleRate: sampleRate}
}

// Process reads PCM frames from the input channel and produces
// Utterances on the returned channel. The output channel is closed
// when the input channel is closed.
func (v *VAD) Process(frames <-chan []int16) <-chan Utterance {
	out := make(chan Utterance, 4)

	preRollSamples := int(v.config.PreRollDuration * float64(v.sampleRate))
	silenceSamples := int(v.config.SilenceDuration * float64(v.sampleRate))
	maxSamples := int(v.config.MaxUtteranceDuration * float64(v.sampleRate))

	go func() {
		defer close(out)

		var (
			ringBuf       = make([]int16, 0, preRollSamples)
			utterance     []int16
			speaking      bool
			silenceSmpCnt int
		)

		for frame := range frames {
			frameRMS := rmsLevel(frame)
			frameSilent := frameRMS < v.config.Threshold

			if !speaking {
				ringBuf = append(ringBuf, frame...)
				if len(ringBuf) > preRollSamples {
					ringBuf = ringBuf[len(ringBuf)-preRollSamples:]
				}

				if !frameSilent {
					speaking = true
					silenceSmpCnt = 0
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
				}

				if silenceSmpCnt >= silenceSamples || len(utterance) >= maxSamples {
					if silenceSmpCnt < len(utterance) {
						trimmed := utterance[:len(utterance)-silenceSmpCnt]
						if len(trimmed) > 0 {
							utterance = trimmed
						}
					}

					out <- Utterance{
						Audio:      utterance,
						SampleRate: v.sampleRate,
					}

					utterance = nil
					speaking = false
					silenceSmpCnt = 0
				}
			}
		}

		if speaking && len(utterance) > 0 {
			out <- Utterance{
				Audio:      utterance,
				SampleRate: v.sampleRate,
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
