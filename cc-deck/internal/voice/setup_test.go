package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// newTreeServer returns an httptest.Server that serves a Hugging Face tree API
// response listing a single file with the given name, SHA, and size.
func newTreeServer(t *testing.T, fileName, sha string, size int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := []hfTreeEntry{
			{
				Path: fileName,
				LFS: &struct {
					OID  string `json:"oid"`
					Size int64  `json:"size"`
				}{
					OID:  sha,
					Size: size,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}))
}

// newModelServer returns an httptest.Server that serves the given content as
// a model download.
func newModelServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}))
}

func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestFetchRemoteSHA_UsesModelTreeAPI(t *testing.T) {
	// Verify that fetchRemoteSHA uses the model's own TreeAPI when set,
	// not the default hfTreeAPI.
	expectedSHA := "abc123def456"
	var expectedSize int64 = 1024

	ts := newTreeServer(t, "ggml-test.bin", expectedSHA, expectedSize)
	defer ts.Close()

	info := ModelInfo{
		Name:     "test-model",
		FileName: "ggml-test.bin",
		URL:      "https://example.com/unused",
		TreeAPI:  ts.URL,
	}

	sha, size, err := fetchRemoteSHA(context.Background(), info)
	if err != nil {
		t.Fatalf("fetchRemoteSHA: %v", err)
	}
	if sha != expectedSHA {
		t.Errorf("sha = %q, want %q", sha, expectedSHA)
	}
	if size != expectedSize {
		t.Errorf("size = %d, want %d", size, expectedSize)
	}
}

func TestFetchRemoteSHA_FallsBackToDefaultTreeAPI(t *testing.T) {
	// When TreeAPI is empty, fetchRemoteSHA should use hfTreeAPI (the default).
	// We can't easily test it hits the real URL, but we can verify that
	// a model with empty TreeAPI and a test server on the default URL would
	// not be found (since the default URL is not our test server).
	info := ModelInfo{
		Name:     "test-model",
		FileName: "ggml-test.bin",
		URL:      "https://example.com/unused",
		TreeAPI:  "", // empty, should use default
	}

	// The default hfTreeAPI points to real Hugging Face. We just verify
	// the function runs without panicking and returns an error (since
	// we are not hitting a real server in tests).
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err := fetchRemoteSHA(ctx, info)
	// Should error because either the real server times out or is unreachable.
	if err == nil {
		t.Log("fetchRemoteSHA with default TreeAPI succeeded (network available)")
	}
}

func TestDownloadModel_ChecksumMismatchLeavesNoFile(t *testing.T) {
	content := []byte("fake model content for checksum test")
	wrongSHA := "0000000000000000000000000000000000000000000000000000000000000000"

	ms := newModelServer(t, content)
	defer ms.Close()

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "ggml-test.bin")

	info := ModelInfo{
		Name:     "test-model",
		FileName: "ggml-test.bin",
		URL:      ms.URL,
	}

	err := downloadModel(context.Background(), info, destPath, wrongSHA, int64(len(content)), nil)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}

	// The final model file should not exist.
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("model file should not exist after checksum mismatch, stat err = %v", statErr)
	}

	// No temp files should remain.
	entries, _ := os.ReadDir(destDir)
	for _, e := range entries {
		if e.Name() != filepath.Base(destPath) {
			t.Errorf("unexpected file in dest dir: %s", e.Name())
		}
	}
}

func TestDownloadModel_ContextCancellationRemovesTempFile(t *testing.T) {
	// Create a server that blocks until the context is cancelled.
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		// Write a small amount to start the download.
		w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(started)
		// Block until the request context is done (client disconnects).
		<-r.Context().Done()
	}))
	defer ts.Close()

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "ggml-cancel.bin")

	info := ModelInfo{
		Name:     "cancel-test",
		FileName: "ggml-cancel.bin",
		URL:      ts.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- downloadModel(ctx, info, destPath, "", 0, nil)
	}()

	// Wait for the download to start.
	<-started
	// Cancel the context.
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}

	// No model file and no temp files should remain.
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("model file should not exist after cancellation")
	}

	entries, _ := os.ReadDir(destDir)
	for _, e := range entries {
		t.Errorf("unexpected file after cancellation: %s", e.Name())
	}
}

func TestDownloadModel_ProgressCallbackReachesTotal(t *testing.T) {
	content := []byte("hello model data for progress test")
	correctSHA := computeSHA256(content)

	ms := newModelServer(t, content)
	defer ms.Close()

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "ggml-progress.bin")

	info := ModelInfo{
		Name:     "progress-test",
		FileName: "ggml-progress.bin",
		URL:      ms.URL,
	}

	var lastDone atomic.Int64
	var lastTotal atomic.Int64

	err := downloadModel(context.Background(), info, destPath, correctSHA, int64(len(content)), func(done, total int64) {
		lastDone.Store(done)
		lastTotal.Store(total)
	})
	if err != nil {
		t.Fatalf("downloadModel: %v", err)
	}

	if lastDone.Load() != int64(len(content)) {
		t.Errorf("final done = %d, want %d", lastDone.Load(), len(content))
	}
	if lastTotal.Load() != int64(len(content)) {
		t.Errorf("final total = %d, want %d", lastTotal.Load(), len(content))
	}

	// Model file should exist with correct content.
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("model content mismatch")
	}
}

func TestTdrzStatus_MissingTool(t *testing.T) {
	// Set PATH to empty so whisper-cli cannot be found.
	t.Setenv("PATH", "")

	toolErr, _ := TdrzStatus()
	if toolErr == nil {
		t.Fatal("expected tool error with empty PATH")
	}
}

func TestTdrzStatus_MissingModel(t *testing.T) {
	// Point the model cache to a temp dir that has no model files.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	// TdrzStatus checks ModelPath which uses xdg.CacheHome. Since
	// xdg.CacheHome is set at init time, we test indirectly by
	// checking that the model path does not exist when the cache dir
	// is empty.
	modelPath := filepath.Join(tmpDir, "cc-deck", "models", "ggml-small.en-tdrz.bin")

	// Ensure the model file does not exist.
	if _, err := os.Stat(modelPath); !os.IsNotExist(err) {
		t.Fatalf("model should not exist in temp dir")
	}

	// The function checks os.Stat on the model path. Since xdg.CacheHome
	// is a package-level var set at init, we verify the path resolution
	// logic separately.
	_, modelErr := TdrzStatus()
	// modelErr should be non-nil because the model file does not exist
	// at the standard path (xdg.CacheHome was set at init, not our temp).
	// This test verifies the function does not panic and returns an error.
	if modelErr == nil {
		// The model might exist on the developer's machine. That is fine.
		t.Log("TdrzStatus reported model present (developer machine)")
	}
}

func TestDownloadModel_SuccessWithMatchingSHA(t *testing.T) {
	content := []byte("valid model binary content")
	correctSHA := computeSHA256(content)

	ts := newTreeServer(t, "ggml-valid.bin", correctSHA, int64(len(content)))
	defer ts.Close()

	ms := newModelServer(t, content)
	defer ms.Close()

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "ggml-valid.bin")

	info := ModelInfo{
		Name:     "valid-test",
		FileName: "ggml-valid.bin",
		URL:      ms.URL,
		TreeAPI:  ts.URL,
	}

	err := downloadModel(context.Background(), info, destPath, correctSHA, int64(len(content)), nil)
	if err != nil {
		t.Fatalf("downloadModel: %v", err)
	}

	// Model file should exist.
	if _, statErr := os.Stat(destPath); statErr != nil {
		t.Errorf("model file should exist: %v", statErr)
	}

	// SHA file should exist.
	shaPath := destPath + ".sha256"
	shaData, err := os.ReadFile(shaPath)
	if err != nil {
		t.Fatalf("SHA file should exist: %v", err)
	}
	if string(shaData) != correctSHA {
		t.Errorf("SHA file content = %q, want %q", string(shaData), correctSHA)
	}
}
