//go:build !cgo

package voice

import (
	"runtime"
	"testing"
)

func TestFfmpegCaptureArgs_ContainsSampleRate(t *testing.T) {
	args := ffmpegCaptureArgs(16000)

	found := false
	for i, a := range args {
		if a == "-ar" && i+1 < len(args) && args[i+1] == "16000" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ffmpegCaptureArgs(16000) = %v, expected -ar 16000", args)
	}
}

func TestFfmpegCaptureArgs_PlatformInputDevice(t *testing.T) {
	args := ffmpegCaptureArgs(16000)

	wantInput := "default"
	if runtime.GOOS == "darwin" {
		wantInput = ":0"
	}

	found := false
	for i, a := range args {
		if a == "-i" && i+1 < len(args) && args[i+1] == wantInput {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ffmpegCaptureArgs() = %v, expected -i %s for GOOS=%s", args, wantInput, runtime.GOOS)
	}
}

func TestNewAudioSource(t *testing.T) {
	src := NewAudioSource()
	if src == nil {
		t.Fatal("NewAudioSource returned nil")
	}
}

func TestFfmpegSource_LevelDefaultsToZero(t *testing.T) {
	src := NewAudioSource()
	if got := src.Level(); got != 0 {
		t.Errorf("Level() before Start = %v, want 0", got)
	}
}

func TestFfmpegSource_ListDevicesReturnsNil(t *testing.T) {
	src := NewAudioSource()
	devices, err := src.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if devices != nil {
		t.Errorf("ListDevices() = %v, want nil", devices)
	}
}

func TestFfmpegSource_StopWithoutStartIsNoop(t *testing.T) {
	src := NewAudioSource()
	if err := src.Stop(); err != nil {
		t.Errorf("Stop() before Start returned error: %v", err)
	}
}
