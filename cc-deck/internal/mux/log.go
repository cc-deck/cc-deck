package mux

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// Logger defines the interface for broker event logging.
type Logger interface {
	Incoming(msg Message)
	DedupHit(key string, msg Message)
	Flush(count int, duration time.Duration)
	FlushError(msg Message, err error)
	Drop(msg Message)
	Lifecycle(event string)
}

// NoopLogger discards all log events.
type NoopLogger struct{}

func (NoopLogger) Incoming(Message)         {}
func (NoopLogger) DedupHit(string, Message) {}
func (NoopLogger) Flush(int, time.Duration) {}
func (NoopLogger) FlushError(Message, error) {}
func (NoopLogger) Drop(Message)             {}
func (NoopLogger) Lifecycle(string)         {}

// FileLogger writes timestamped log entries to the mux log file.
type FileLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileLogger creates a FileLogger writing to ~/.local/state/cc-deck/mux.log.
// The file is truncated on open. Returns a NoopLogger if the file cannot be created.
func NewFileLogger() Logger {
	logDir := filepath.Join(xdg.StateHome, "cc-deck")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return NoopLogger{}
	}
	logPath := filepath.Join(logDir, "mux.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return NoopLogger{}
	}
	return &FileLogger{file: f}
}

func (l *FileLogger) write(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format("2006-01-02T15:04:05.000")
	fmt.Fprintf(l.file, ts+" "+format+"\n", args...)
}

func (l *FileLogger) Incoming(msg Message) {
	l.write("INCOMING session=%s pipe=%s args_len=%d", msg.SessionName, msg.PipeName, len(msg.Args))
}

func (l *FileLogger) DedupHit(key string, msg Message) {
	l.write("DEDUP key=%s session=%s", key, msg.SessionName)
}

func (l *FileLogger) Flush(count int, duration time.Duration) {
	l.write("FLUSH count=%d duration=%s", count, duration)
}

func (l *FileLogger) FlushError(msg Message, err error) {
	l.write("FLUSH_ERROR session=%s pipe=%s err=%v", msg.SessionName, msg.PipeName, err)
}

func (l *FileLogger) Drop(msg Message) {
	l.write("DROP session=%s pipe=%s", msg.SessionName, msg.PipeName)
}

func (l *FileLogger) Lifecycle(event string) {
	l.write("LIFECYCLE %s", event)
}
