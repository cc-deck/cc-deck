package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type httpTranscriber struct {
	endpoint string
	client   *http.Client
	server   *WhisperServer
}

// NewHTTPTranscriber creates a transcriber that posts audio to a
// whisper-server HTTP endpoint.
func NewHTTPTranscriber(endpoint string, server *WhisperServer) Transcriber {
	return &httpTranscriber{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 30 * time.Second},
		server:   server,
	}
}

func (t *httpTranscriber) Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error) {
	wavData := pcmToWAV(audio, sampleRate)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("creating multipart form: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return "", fmt.Errorf("writing audio data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint+"/inference", &body)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("posting to whisper-server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper-server returned status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	return strings.TrimSpace(string(respBody)), nil
}

func (t *httpTranscriber) Close() error {
	if t.server != nil {
		return t.server.Stop()
	}
	return nil
}

// pcmToWAV wraps raw PCM samples in a WAV header (16-bit mono).
func pcmToWAV(samples []int16, sampleRate int) []byte {
	dataSize := len(samples) * 2
	fileSize := 44 + dataSize

	buf := &bytes.Buffer{}
	buf.Grow(fileSize)

	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(fileSize-8))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))     // chunk size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))      // PCM format
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))      // mono
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))      // block align
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))     // bits per sample

	// data chunk
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	_ = binary.Write(buf, binary.LittleEndian, samples)

	return buf.Bytes()
}
