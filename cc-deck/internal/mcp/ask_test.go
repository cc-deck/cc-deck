package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateUUID(t *testing.T) {
	t.Run("generates valid UUID format", func(t *testing.T) {
		uuid, err := generateUUID()
		require.NoError(t, err)
		// UUID format: 8-4-4-4-12 hex chars
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, uuid)
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		uuid1, err := generateUUID()
		require.NoError(t, err)
		uuid2, err := generateUUID()
		require.NoError(t, err)
		assert.NotEqual(t, uuid1, uuid2)
	})
}

// extractResponsePath parses the inject payload to find the response file path.
// The payload is JSON with a "text" field containing the injection prompt,
// which includes a line like "Write your complete answer to this file: /path/to/file"
func extractResponsePath(payload string) string {
	var p map[string]any
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return ""
	}
	text, _ := p["text"].(string)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Write your complete answer to this file: ") {
			return strings.TrimPrefix(line, "Write your complete answer to this file: ")
		}
	}
	return ""
}

func TestAskSession(t *testing.T) {
	t.Run("returns response when file is written", func(t *testing.T) {
		// Use a channel to signal when the inject payload is captured.
		payloadCh := make(chan string, 1)
		mockPipe := func(ctx context.Context, name string, payload string) (string, error) {
			if name == "cc-deck:mcp-inject" {
				payloadCh <- payload
				return `{"ok": true}`, nil
			}
			return "", nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		doneCh := make(chan struct{})
		var result string
		var askErr error

		go func() {
			result, askErr = AskSession(ctx, mockPipe, 42, "test-session", "What is the status?", 10)
			close(doneCh)
		}()

		// Wait for the inject payload to be sent.
		select {
		case payload := <-payloadCh:
			// Extract the response file path from the inject text.
			responsePath := extractResponsePath(payload)
			require.NotEmpty(t, responsePath, "could not find response path in inject payload")

			// Wait a moment for the poller to start, then write the response.
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, os.WriteFile(responsePath, []byte("Everything looks good!"), 0644))

		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for inject payload")
		}

		// Wait for AskSession to complete.
		select {
		case <-doneCh:
			require.NoError(t, askErr)
			assert.Equal(t, "Everything looks good!", result)
		case <-time.After(5 * time.Second):
			t.Fatal("AskSession did not complete in time")
		}
	})

	t.Run("returns error when session is not idle", func(t *testing.T) {
		mockPipe := func(ctx context.Context, name string, payload string) (string, error) {
			if name == "cc-deck:mcp-inject" {
				return `{"error": "session not idle"}`, nil
			}
			return "", nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := AskSession(ctx, mockPipe, 42, "test-session", "question", 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not idle")
	})

	t.Run("times out when no response is written", func(t *testing.T) {
		mockPipe := func(ctx context.Context, name string, payload string) (string, error) {
			if name == "cc-deck:mcp-inject" {
				return `{"ok": true}`, nil
			}
			return "", nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		start := time.Now()
		_, err := AskSession(ctx, mockPipe, 42, "test-session", "question", 2)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
		assert.Contains(t, err.Error(), "test-session")
		// Should timeout after approximately 2 seconds.
		assert.True(t, elapsed >= 2*time.Second, "expected at least 2s, got %v", elapsed)
	})
}
