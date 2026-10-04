package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_CreatesFileWithContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	if err := AtomicWrite(target, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", data, "hello world")
	}
}

func TestAtomicWrite_SetsPermissions(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "perm.txt")

	if err := AtomicWrite(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %v, want %v", perm, os.FileMode(0o600))
	}
}

func TestAtomicWrite_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")

	if err := os.WriteFile(target, []byte("old content"), 0o644); err != nil {
		t.Fatalf("seeding existing file: %v", err)
	}

	if err := AtomicWrite(target, []byte("new content"), 0o644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("content = %q, want %q", data, "new content")
	}
}

func TestAtomicWrite_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "clean.txt")

	if err := AtomicWrite(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "clean.txt" {
		t.Errorf("dir entries = %v, want only clean.txt", entries)
	}
}

func TestAtomicWrite_ErrorsOnMissingDirectory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "does-not-exist", "out.txt")

	if err := AtomicWrite(target, []byte("data"), 0o644); err == nil {
		t.Fatal("expected error when target directory does not exist, got nil")
	}
}

func TestAtomicWrite_EmptyData(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.txt")

	if err := AtomicWrite(target, []byte{}, 0o644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("content length = %d, want 0", len(data))
	}
}
