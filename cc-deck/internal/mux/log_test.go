package mux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLogger_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mux.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	require.NoError(t, err)

	logger := &FileLogger{file: f}
	msg := Message{SessionName: "test-sess", PipeName: "cc-deck:hook", Args: `{"x":1}`}
	logger.Incoming(msg)
	f.Close()

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "INCOMING")
	assert.Contains(t, content, "test-sess")
	assert.Contains(t, content, "cc-deck:hook")
}

func TestFileLogger_EntriesContainTimestamp(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mux.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	require.NoError(t, err)

	logger := &FileLogger{file: f}
	logger.Lifecycle("test-event")
	f.Close()

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	// Timestamp format: 2006-01-02T15:04:05.000
	year := time.Now().Format("2006")
	assert.True(t, strings.HasPrefix(content, year), "log entry should start with current year timestamp")
}

func TestFileLogger_AllMethods(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mux.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	require.NoError(t, err)

	logger := &FileLogger{file: f}
	msg := Message{SessionName: "s1", PipeName: "p1", Args: "a"}

	logger.Incoming(msg)
	logger.DedupHit("abc123", msg)
	logger.Flush(5, 10*time.Millisecond)
	logger.FlushError(msg, assert.AnError)
	logger.Drop(msg)
	logger.Lifecycle("started")
	f.Close()

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "INCOMING")
	assert.Contains(t, content, "DEDUP")
	assert.Contains(t, content, "FLUSH count=5")
	assert.Contains(t, content, "FLUSH_ERROR")
	assert.Contains(t, content, "DROP")
	assert.Contains(t, content, "LIFECYCLE started")
}

func TestNoopLogger_ProducesNoOutput(t *testing.T) {
	logger := NoopLogger{}
	msg := Message{SessionName: "s", PipeName: "p", Args: "a"}

	// These should not panic or produce any side effects
	logger.Incoming(msg)
	logger.DedupHit("key", msg)
	logger.Flush(1, time.Millisecond)
	logger.FlushError(msg, assert.AnError)
	logger.Drop(msg)
	logger.Lifecycle("event")
}
