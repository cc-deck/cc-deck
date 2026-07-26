package mcp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// generateUUID creates a UUID v4 string using crypto/rand.
func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating UUID: %w", err)
	}
	// Set version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// AskSession injects a question into a target session and polls for a
// file-based response. It creates a temporary directory for the response
// file, sends the question as an injection prompt via the pipe, and waits
// for the target agent to write its answer.
func AskSession(
	ctx context.Context,
	pipeFunc PipeSender,
	paneID uint32,
	sessionName string,
	question string,
	timeoutSecs int,
) (string, error) {
	uuid, err := generateUUID()
	if err != nil {
		return "", fmt.Errorf("cannot create temporary files: %v", err)
	}

	tempDir := filepath.Join(os.TempDir(), "cc-deck-ask")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create temporary files: %v", err)
	}

	responsePath := filepath.Join(tempDir, uuid+".response")

	// Clean up on return regardless of outcome.
	defer func() {
		os.Remove(responsePath)
		// Try to remove the temp dir (will only succeed if empty).
		os.Remove(tempDir)
	}()

	// Build the injection prompt that tells the target agent to answer.
	injectionPrompt := fmt.Sprintf(
		`A colleague agent is asking you a question. Please read it carefully, think about it from your project context, and write your answer to the file path below.

Question: %s

Write your complete answer to this file: %s

After writing the file, you can continue with your other work.
`, question, responsePath)

	// Send injection to the plugin.
	injectPayload, _ := json.Marshal(map[string]any{
		"pane_id": paneID,
		"text":    injectionPrompt + "\n",
	})

	pipeCtx, cancel := context.WithTimeout(ctx, pipeTimeout)
	defer cancel()

	resp, err := pipeFunc(pipeCtx, "cc-deck:mcp-inject", string(injectPayload))
	if err != nil {
		return "", fmt.Errorf("cc-deck plugin not running")
	}

	// Check if the injection was rejected (session not idle).
	var injectResult map[string]any
	if err := json.Unmarshal([]byte(resp), &injectResult); err == nil {
		if errMsg, ok := injectResult["error"].(string); ok {
			if errMsg != "" {
				return "", fmt.Errorf("session '%s' is not idle (current state: working)", sessionName)
			}
		}
	}

	// Poll for the response file.
	deadline := time.After(time.Duration(timeoutSecs) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return "", fmt.Errorf("ask timed out after %ds waiting for response from '%s'",
				timeoutSecs, sessionName)
		case <-ctx.Done():
			return "", fmt.Errorf("ask timed out after %ds waiting for response from '%s'",
				timeoutSecs, sessionName)
		case <-ticker.C:
			data, err := os.ReadFile(responsePath)
			if err != nil {
				continue // File not yet written.
			}
			if len(data) == 0 {
				continue // File exists but empty (still being written).
			}
			return string(data), nil
		}
	}
}
