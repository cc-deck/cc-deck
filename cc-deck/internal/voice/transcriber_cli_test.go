package voice

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCLITranscriber(t *testing.T) {
	tr := NewCLITranscriber("/tmp/model.bin")
	if tr == nil {
		t.Fatal("NewCLITranscriber returned nil")
	}
	if err := tr.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestWriteWAVFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "utterance.wav")
	samples := []int16{0, 100, -100, 32767, -32768}

	if err := writeWAVFile(path, samples, 16000); err != nil {
		t.Fatalf("writeWAVFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written WAV file: %v", err)
	}

	wantSize := 44 + len(samples)*2
	if len(data) != wantSize {
		t.Fatalf("wrote %d bytes, want %d", len(data), wantSize)
	}
	if string(data[0:4]) != "RIFF" {
		t.Errorf("missing RIFF header, got %q", data[0:4])
	}
	if string(data[8:12]) != "WAVE" {
		t.Errorf("missing WAVE marker, got %q", data[8:12])
	}
	if string(data[12:16]) != "fmt " {
		t.Errorf("missing fmt chunk, got %q", data[12:16])
	}
	if string(data[36:40]) != "data" {
		t.Errorf("missing data chunk, got %q", data[36:40])
	}

	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != 16000 {
		t.Errorf("sample rate = %d, want 16000", sampleRate)
	}

	// Verify the first PCM sample round-trips correctly.
	firstSample := int16(binary.LittleEndian.Uint16(data[44:46]))
	if firstSample != samples[0] {
		t.Errorf("first PCM sample = %d, want %d", firstSample, samples[0])
	}
}

func TestWriteWAVFile_EmptySamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.wav")

	if err := writeWAVFile(path, nil, 16000); err != nil {
		t.Fatalf("writeWAVFile with no samples: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written WAV file: %v", err)
	}
	if len(data) != 44 {
		t.Errorf("wrote %d bytes for empty samples, want 44 (header only)", len(data))
	}
}

func TestWriteWAVFile_InvalidPath(t *testing.T) {
	err := writeWAVFile(filepath.Join(t.TempDir(), "missing-dir", "file.wav"), []int16{1, 2, 3}, 16000)
	if err == nil {
		t.Fatal("expected error writing to a nonexistent directory")
	}
}
