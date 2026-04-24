//go:build cgo

package voice

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
)

type malgoSource struct {
	mu      sync.Mutex
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	level   atomic.Value // float64
	stopped chan struct{}
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

	out := make(chan []int16, 16)
	s.stopped = make(chan struct{})

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, inputSamples []byte, frameCount uint32) {
			samples := make([]int16, frameCount)
			for i := uint32(0); i < frameCount; i++ {
				lo := inputSamples[i*2]
				hi := inputSamples[i*2+1]
				samples[i] = int16(uint16(lo) | uint16(hi)<<8)
			}

			var sum float64
			for _, sample := range samples {
				v := float64(sample) / 32768.0
				sum += v * v
			}
			rms := math.Sqrt(sum / float64(len(samples)))
			s.level.Store(rms)

			select {
			case out <- samples:
			default:
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

	go func() {
		defer close(out)
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

	s.device.Uninit()
	_ = s.ctx.Uninit()
	s.ctx.Free()
	s.device = nil
	s.ctx = nil

	select {
	case <-s.stopped:
	default:
		close(s.stopped)
	}

	return nil
}

func (s *malgoSource) Level() float64 {
	v, ok := s.level.Load().(float64)
	if !ok {
		return 0
	}
	return v
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
