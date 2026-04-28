//go:build cgo

package voice

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"
)

type malgoSource struct {
	mu            sync.Mutex
	ctx           *malgo.AllocatedContext
	device        *malgo.Device
	level         atomic.Value // float64
	droppedFrames atomic.Int64
	closing       atomic.Bool
	stopped       chan struct{}
	out           chan []int16
}

func NewAudioSource() AudioSource {
	return &malgoSource{}
}

func (s *malgoSource) Start(ctx context.Context, sampleRate int) (<-chan []int16, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.device != nil {
		return nil, fmt.Errorf("audio source already started")
	}

	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing audio context: %w", err)
	}
	s.ctx = mctx
	s.level.Store(float64(0))

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(sampleRate)

	out := make(chan []int16, 64)
	s.stopped = make(chan struct{})

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, inputSamples []byte, frameCount uint32) {
			if len(inputSamples) < int(frameCount*2) {
				return
			}
			samples := make([]int16, frameCount)
			for i := uint32(0); i < frameCount; i++ {
				lo := inputSamples[i*2]
				hi := inputSamples[i*2+1]
				samples[i] = int16(uint16(lo) | uint16(hi)<<8)
			}

			if len(samples) == 0 {
				return
			}
			var sum float64
			for _, sample := range samples {
				v := float64(sample) / 32768.0
				sum += v * v
			}
			rms := math.Sqrt(sum / float64(len(samples)))
			s.level.Store(rms)

			if s.closing.Load() {
				return
			}
			select {
			case out <- samples:
			default:
				s.droppedFrames.Add(1)
			}
		},
	}

	device, err := malgo.InitDevice(mctx.Context, deviceConfig, callbacks)
	if err != nil {
		_ = mctx.Uninit()
		mctx.Free()
		return nil, fmt.Errorf("initializing capture device: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		_ = mctx.Uninit()
		mctx.Free()
		return nil, fmt.Errorf("starting capture: %w", err)
	}

	s.device = device
	s.out = out

	go func() {
		select {
		case <-ctx.Done():
			_ = s.Stop()
		case <-s.stopped:
		}
	}()

	return out, nil
}

func (s *malgoSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.device == nil {
		return nil
	}

	done := make(chan struct{})
	dev := s.device
	mctx := s.ctx
	go func() {
		dev.Uninit()
		_ = mctx.Uninit()
		mctx.Free()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	s.device = nil
	s.ctx = nil

	select {
	case <-s.stopped:
	default:
		close(s.stopped)
	}

	s.closing.Store(true)
	if s.out != nil {
		close(s.out)
		s.out = nil
	}
	s.closing.Store(false)

	return nil
}

func (s *malgoSource) Level() float64 {
	v, ok := s.level.Load().(float64)
	if !ok {
		return 0
	}
	return v
}

func (s *malgoSource) DroppedFrames() int64 {
	return s.droppedFrames.Load()
}

func (s *malgoSource) ListDevices() ([]DeviceInfo, error) {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing context for device list: %w", err)
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()

	devices, err := mctx.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("enumerating devices: %w", err)
	}

	result := make([]DeviceInfo, len(devices))
	for i, d := range devices {
		full, err := mctx.DeviceInfo(malgo.Capture, d.ID, malgo.Shared)
		if err != nil {
			result[i] = DeviceInfo{
				ID:   d.ID.String(),
				Name: d.Name(),
			}
			continue
		}
		result[i] = DeviceInfo{
			ID:        d.ID.String(),
			Name:      full.Name(),
			IsDefault: d.IsDefault != 0,
		}
	}

	return result, nil
}
