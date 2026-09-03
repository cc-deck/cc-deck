package voice

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestNewWhisperServer(t *testing.T) {
	s := NewWhisperServer("/tmp/model.bin", 9999)
	if s == nil {
		t.Fatal("NewWhisperServer returned nil")
	}
	if s.Endpoint() != "http://127.0.0.1:9999" {
		t.Errorf("Endpoint() = %q, want %q", s.Endpoint(), "http://127.0.0.1:9999")
	}
}

func TestWhisperServer_Endpoint(t *testing.T) {
	s := NewWhisperServer("/tmp/model.bin", 8080)
	want := "http://127.0.0.1:8080"
	if got := s.Endpoint(); got != want {
		t.Errorf("Endpoint() = %q, want %q", got, want)
	}
}

// portFromURL extracts the numeric port from an httptest server URL so
// WhisperServer.Endpoint()+"/health" resolves to it.
func portFromURL(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing url %q: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing port from %q: %v", rawURL, err)
	}
	return port
}

func TestWhisperServer_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewWhisperServer("/tmp/model.bin", portFromURL(t, srv.URL))
	if !s.Healthy(context.Background()) {
		t.Error("expected Healthy() to return true for a 200 response")
	}
}

func TestWhisperServer_HealthyNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewWhisperServer("/tmp/model.bin", portFromURL(t, srv.URL))
	if s.Healthy(context.Background()) {
		t.Error("expected Healthy() to return false for a 503 response")
	}
}

func TestWhisperServer_HealthyUnreachable(t *testing.T) {
	// Port 1 is a privileged port that is very unlikely to have a
	// listener in a test sandbox, so the connection should fail fast.
	s := NewWhisperServer("/tmp/model.bin", 1)
	if s.Healthy(context.Background()) {
		t.Error("expected Healthy() to return false when nothing is listening")
	}
}

func TestWhisperServer_StopWithoutStart(t *testing.T) {
	s := NewWhisperServer("/tmp/model.bin", 9998)
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() on a never-started server returned error: %v", err)
	}
	// Calling Stop again must remain a safe no-op.
	if err := s.Stop(); err != nil {
		t.Errorf("second Stop() call returned error: %v", err)
	}
}

func TestWhisperServer_WaitReadyContextCanceled(t *testing.T) {
	s := NewWhisperServer("/tmp/model.bin", 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.waitReady(ctx)
	if err == nil {
		t.Fatal("expected error when context is already canceled")
	}
}

func TestWhisperServer_WaitReadySucceedsWhenHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewWhisperServer("/tmp/model.bin", portFromURL(t, srv.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.waitReady(ctx); err != nil {
		t.Fatalf("waitReady: %v", err)
	}
}

func TestWhisperServer_StartMissingBinary(t *testing.T) {
	// whisper-server is not expected to be installed in the test
	// environment; Start should fail cleanly rather than hang.
	s := NewWhisperServer("/tmp/model.bin", 9997)
	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when whisper-server binary is not in PATH")
	}
}

func TestWhisperServer_RestartExceedsMaxRetries(t *testing.T) {
	s := NewWhisperServer("/tmp/model.bin", 9996)
	s.maxRetries = 1
	s.retries = 1

	err := s.Restart(context.Background())
	if err == nil {
		t.Fatal("expected error once max retries exceeded")
	}
	want := fmt.Sprintf("whisper-server crashed %d times, giving up", s.retries)
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
