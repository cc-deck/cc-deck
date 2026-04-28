//go:build !cgo

package voice

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
)

type ffmpegSource struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	level     atomic.Value // float64
	stopped   chan struct{}
	closeOnce sync.Once
}

func NewAudioSource() AudioSource {
	return &ffmpegSource{}
}

func (s *ffmpegSource) Start(ctx context.Context, sampleRate int) (<-chan []int16, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil {
		return nil, fmt.Errorf("audio source already started")
	}

	args := ffmpegCaptureArgs(sampleRate)
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, "ffmpeg", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	s.cmd = cmd
	s.cancel = cancel
	s.level.Store(float64(0))
	stopped := make(chan struct{})
	s.stopped = stopped
	s.closeOnce = sync.Once{}

	out := make(chan []int16, 16)
	frameSize := sampleRate / 50 // 20ms frames
	closeOnce := &s.closeOnce

	go func() {
		defer close(out)
		defer closeOnce.Do(func() { close(stopped) })

		buf := make([]int16, frameSize)
		for {
			if err := binary.Read(stdout, binary.LittleEndian, buf); err != nil {
				return
			}

			samples := make([]int16, len(buf))
			copy(samples, buf)

			var sum float64
			for _, sample := range samples {
				v := float64(sample) / 32768.0
				sum += v * v
			}
			rms := math.Sqrt(sum / float64(len(samples)))
			s.level.Store(rms)

			select {
			case out <- samples:
			case <-cmdCtx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (s *ffmpegSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil {
		return nil
	}

	s.cancel()
	_ = s.cmd.Wait()
	s.cmd = nil
	s.closeOnce.Do(func() { close(s.stopped) })

	return nil
}

func (s *ffmpegSource) Level() float64 {
	v, ok := s.level.Load().(float64)
	if !ok {
		return 0
	}
	return v
}

func (s *ffmpegSource) ListDevices() ([]DeviceInfo, error) {
	return nil, nil
}

func ffmpegCaptureArgs(sampleRate int) []string {
	rate := fmt.Sprintf("%d", sampleRate)
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"-f", "avfoundation", "-i", ":0",
			"-f", "s16le", "-ac", "1", "-ar", rate,
			"-loglevel", "error", "-",
		}
	default:
		return []string{
			"-f", "pulse", "-i", "default",
			"-f", "s16le", "-ac", "1", "-ar", rate,
			"-loglevel", "error", "-",
		}
	}
}
