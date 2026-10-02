package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCleanupPluginSymlinks_RemovesMatchingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "cc_deck.wasm")
	if err := os.WriteFile(target, []byte("wasm"), 0644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}
	link := filepath.Join(dir, "cc_deck_controller.wasm")
	if err := os.Symlink("cc_deck.wasm", link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	var buf bytes.Buffer
	cleanupPluginSymlinks(dir, &buf)

	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("expected symlink to be removed, lstat err = %v", err)
	}
	if !strings.Contains(buf.String(), "symlink") {
		t.Errorf("expected output to mention symlink removal, got %q", buf.String())
	}
}

func TestCleanupPluginSymlinks_IgnoresUnrelatedSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on windows")
	}
	dir := t.TempDir()
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}
	link := filepath.Join(dir, "other_link")
	if err := os.Symlink("other.txt", link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	var buf bytes.Buffer
	cleanupPluginSymlinks(dir, &buf)

	if _, err := os.Lstat(link); err != nil {
		t.Errorf("expected unrelated symlink to remain, lstat err = %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected no output for unrelated symlink, got %q", buf.String())
	}
}

func TestCleanupPluginSymlinks_NonexistentDir(t *testing.T) {
	var buf bytes.Buffer
	// Should return without error or panic when the directory doesn't exist.
	cleanupPluginSymlinks("/nonexistent/plugins/dir", &buf)
	if buf.String() != "" {
		t.Errorf("expected no output for nonexistent dir, got %q", buf.String())
	}
}

func TestIsZellijRunning_DoesNotPanic(t *testing.T) {
	// We can't control whether zellij is actually running in the test
	// environment, so just verify the function executes without panicking.
	_ = isZellijRunning()
}

func TestRemove_NothingInstalled(t *testing.T) {
	// In the test environment zellij is not installed, so DetectZellij
	// returns a not-installed state and Remove should report that there is
	// nothing to remove without touching the filesystem.
	var stdout, stderr bytes.Buffer
	err := Remove(RemoveOptions{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Nothing to remove") {
		t.Errorf("expected 'Nothing to remove' message, got %q", stdout.String())
	}
}
