package voice

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// StartupTimeout bounds how long Start waits for whisper-server to answer
// its health check. A warm start takes about a second, but the first start
// after a whisper-cpp upgrade compiles Metal kernels and reads the model
// from a cold disk cache, which has been observed to exceed 15 seconds
// with the medium model.
const StartupTimeout = 60 * time.Second

// WhisperServer manages a local whisper-server process.
type WhisperServer struct {
	modelPath  string
	port       int
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	mu         sync.Mutex
	maxRetries int
	retries    int
	// logWriter receives whisper-server's own stderr when set, so a start
	// that fails leaves a reason behind instead of a bare timeout.
	logWriter io.Writer
}

// SetLogWriter forwards whisper-server's stderr to w.
func (s *WhisperServer) SetLogWriter(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logWriter = w
}

// NewWhisperServer creates a lifecycle manager for whisper-server.
func NewWhisperServer(modelPath string, port int) *WhisperServer {
	return &WhisperServer{
		modelPath:  modelPath,
		port:       port,
		maxRetries: 3,
	}
}

// Start launches whisper-server and waits for it to be ready.
func (s *WhisperServer) Start(ctx context.Context) error {
	s.mu.Lock()

	if s.cmd != nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if s.Healthy(ctx) {
		return nil
	}

	s.mu.Lock()
	if s.cmd != nil {
		s.mu.Unlock()
		return nil
	}
	if _, err := exec.LookPath("whisper-server"); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("whisper-server not found in PATH; install: brew install whisper-cpp")
	}

	cmdCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(cmdCtx, "whisper-server",
		"-m", s.modelPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", s.port),
	)
	if s.logWriter != nil {
		cmd.Stderr = s.logWriter
	}

	if err := cmd.Start(); err != nil {
		cancel()
		s.mu.Unlock()
		return fmt.Errorf("starting whisper-server: %w", err)
	}

	s.cmd = cmd
	s.cancel = cancel
	s.retries = 0
	s.mu.Unlock()

	go func() {
		_ = cmd.Wait()
	}()

	if err := s.waitReady(ctx); err != nil {
		s.mu.Lock()
		s.stopLocked()
		s.mu.Unlock()
		return fmt.Errorf("whisper-server failed to start: %w", err)
	}

	return nil
}

// Restart attempts to restart the server after a crash.
func (s *WhisperServer) Restart(ctx context.Context) error {
	s.mu.Lock()
	s.retries++
	if s.retries > s.maxRetries {
		s.mu.Unlock()
		return fmt.Errorf("whisper-server crashed %d times, giving up", s.retries)
	}
	s.stopLocked()
	s.mu.Unlock()

	return s.Start(ctx)
}

// Stop shuts down whisper-server.
func (s *WhisperServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	return nil
}

func (s *WhisperServer) stopLocked() {
	if s.cmd == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	done := make(chan struct{})
	go func() {
		_ = s.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}
	s.cmd = nil
	s.cancel = nil
}

// Endpoint returns the HTTP base URL for the server.
func (s *WhisperServer) Endpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.port)
}

// Healthy returns true if the server responds to health checks.
func (s *WhisperServer) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.Endpoint()+"/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (s *WhisperServer) waitReady(ctx context.Context) error {
	deadline := time.After(StartupTimeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out after %s waiting for whisper-server on port %d (run with --verbose to see the server's output)", StartupTimeout, s.port)
		case <-ticker.C:
			if s.Healthy(ctx) {
				return nil
			}
		}
	}
}
