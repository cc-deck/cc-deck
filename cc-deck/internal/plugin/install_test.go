package plugin

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")

	if err := atomicWrite(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("atomicWrite() unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("atomicWrite() wrote %q, want %q", got, "hello")
	}

	// Overwriting an existing file should succeed and replace the content.
	if err := atomicWrite(path, []byte("world"), 0o644); err != nil {
		t.Fatalf("atomicWrite() overwrite unexpected error: %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading overwritten file: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("atomicWrite() overwrite wrote %q, want %q", got, "world")
	}
}

func TestWrapPermissionError(t *testing.T) {
	permErr := &os.PathError{Op: "open", Path: "/root/secret", Err: syscall.EACCES}
	if !os.IsPermission(permErr) {
		t.Fatalf("test fixture is not a permission error")
	}

	err := wrapPermissionError(permErr, "/root/secret", "write")
	if err == nil {
		t.Fatal("wrapPermissionError() returned nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "/root/secret") || !strings.Contains(msg, "write") {
		t.Errorf("wrapPermissionError() = %q, want it to mention path and access", msg)
	}
	if !strings.Contains(msg, "permission denied") {
		t.Errorf("wrapPermissionError() = %q, want it to mention permission denied", msg)
	}

	other := errors.New("boom")
	err = wrapPermissionError(other, "/tmp/file", "read")
	if err == nil {
		t.Fatal("wrapPermissionError() returned nil for non-permission error")
	}
	msg = err.Error()
	if strings.Contains(msg, "permission denied") {
		t.Errorf("wrapPermissionError() = %q, should not claim permission denied for generic errors", msg)
	}
	if !strings.Contains(msg, "/tmp/file") {
		t.Errorf("wrapPermissionError() = %q, want it to mention the path", msg)
	}
}

func TestShowColorDiff(t *testing.T) {
	var buf bytes.Buffer
	showColorDiff("line1\nline2\nline3", "line1\nCHANGED\nline3\nline4", &buf)

	out := buf.String()
	if strings.Contains(out, "line1") {
		t.Errorf("showColorDiff() should skip identical lines, got: %q", out)
	}
	if !strings.Contains(out, "- line2") {
		t.Errorf("showColorDiff() missing removed line, got: %q", out)
	}
	if !strings.Contains(out, "+ CHANGED") {
		t.Errorf("showColorDiff() missing added line, got: %q", out)
	}
	if !strings.Contains(out, "+ line4") {
		t.Errorf("showColorDiff() missing trailing added line, got: %q", out)
	}
}

func TestShowColorDiff_Identical(t *testing.T) {
	var buf bytes.Buffer
	showColorDiff("same\ncontent", "same\ncontent", &buf)
	if buf.Len() != 0 {
		t.Errorf("showColorDiff() for identical content should produce no output, got: %q", buf.String())
	}
}

func TestWriteLayoutWithDiff_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	opts := InstallOptions{Stdout: &bytes.Buffer{}}

	if err := writeLayoutWithDiff(path, []byte("content-v1"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written layout: %v", err)
	}
	if string(got) != "content-v1" {
		t.Errorf("writeLayoutWithDiff() wrote %q, want %q", got, "content-v1")
	}
}

func TestWriteLayoutWithDiff_UnchangedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	opts := InstallOptions{Stdout: &bytes.Buffer{}}

	if err := writeLayoutWithDiff(path, []byte("same"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading layout: %v", err)
	}
	if string(got) != "same" {
		t.Errorf("writeLayoutWithDiff() should leave unchanged content as-is, got %q", got)
	}
}

func TestWriteLayoutWithDiff_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	opts := InstallOptions{Stdout: &bytes.Buffer{}, Force: true}

	if err := writeLayoutWithDiff(path, []byte("new"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading layout: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("writeLayoutWithDiff() with Force should overwrite, got %q", got)
	}
}

func TestWriteLayoutWithDiff_PromptOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	opts := InstallOptions{
		Stdout: &bytes.Buffer{},
		Stdin:  strings.NewReader("o\n"),
	}

	if err := writeLayoutWithDiff(path, []byte("new"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading layout: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("writeLayoutWithDiff() answering 'o' should overwrite, got %q", got)
	}
}

func TestWriteLayoutWithDiff_PromptSkip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	opts := InstallOptions{
		Stdout: &bytes.Buffer{},
		Stdin:  strings.NewReader("s\n"),
	}

	if err := writeLayoutWithDiff(path, []byte("new"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading layout: %v", err)
	}
	if string(got) != "old" {
		t.Errorf("writeLayoutWithDiff() answering 's' should skip, got %q", got)
	}
}

func TestWriteLayoutWithDiff_PromptBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.kdl")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	opts := InstallOptions{
		Stdout: &bytes.Buffer{},
		Stdin:  strings.NewReader("b\n"),
	}

	if err := writeLayoutWithDiff(path, []byte("new"), opts); err != nil {
		t.Fatalf("writeLayoutWithDiff() unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading layout: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("writeLayoutWithDiff() answering 'b' should overwrite, got %q", got)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading backup file: %v", err)
	}
	if string(backup) != "old" {
		t.Errorf("writeLayoutWithDiff() backup should contain old content, got %q", backup)
	}
}
