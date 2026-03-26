package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupFile_SkipBackup(t *testing.T) {
	path, err := BackupFile("/nonexistent", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
}

func TestBackupFile_NonexistentFile(t *testing.T) {
	path, err := BackupFile("/nonexistent/file.json", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
}

func TestBackupFile_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(src, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	backupPath, err := BackupFile(src, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected non-empty backup path")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Fatalf("backup content mismatch: %q", data)
	}
}

func TestPruneOldBackups_KeepsMaxThree(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(src, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create 5 fake backups with known timestamps
	timestamps := []string{
		"20260101-100000",
		"20260102-100000",
		"20260103-100000",
		"20260104-100000",
		"20260105-100000",
	}
	for _, ts := range timestamps {
		bp := filepath.Join(dir, "settings.json.bak."+ts)
		if err := os.WriteFile(bp, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pruneOldBackups(src)

	// Should keep only the 3 newest
	entries, _ := os.ReadDir(dir)
	var backups []string
	for _, e := range entries {
		if e.Name() != "settings.json" {
			backups = append(backups, e.Name())
		}
	}

	if len(backups) != maxBackups {
		t.Fatalf("expected %d backups, got %d: %v", maxBackups, len(backups), backups)
	}

	// Verify the oldest two were removed
	for _, ts := range timestamps[:2] {
		removed := filepath.Join(dir, "settings.json.bak."+ts)
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", removed)
		}
	}

	// Verify the newest three remain
	for _, ts := range timestamps[2:] {
		kept := filepath.Join(dir, "settings.json.bak."+ts)
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected %s to exist: %v", kept, err)
		}
	}
}

func TestPruneOldBackups_NoPruneWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(src, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create exactly 2 backups (under limit)
	for _, ts := range []string{"20260101-100000", "20260102-100000"} {
		bp := filepath.Join(dir, "settings.json.bak."+ts)
		if err := os.WriteFile(bp, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pruneOldBackups(src)

	entries, _ := os.ReadDir(dir)
	var backups []string
	for _, e := range entries {
		if e.Name() != "settings.json" {
			backups = append(backups, e.Name())
		}
	}

	if len(backups) != 2 {
		t.Fatalf("expected 2 backups (no pruning), got %d", len(backups))
	}
}
