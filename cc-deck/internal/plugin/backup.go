// T018: Timestamped backup logic for settings.json

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// maxBackups is the maximum number of backup files to retain per source file.
const maxBackups = 3

// BackupFile creates a timestamped backup of the given file.
// Returns the backup path, or empty string if the file doesn't exist.
// If skipBackup is true, no backup is created.
// Old backups beyond maxBackups are pruned after the new backup is created.
func BackupFile(path string, skipBackup bool) (string, error) {
	if skipBackup {
		return "", nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}

	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	backupPath := fmt.Sprintf("%s%s.bak.%s", base, ext, timestamp)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s for backup: %w", path, err)
	}

	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing backup to %s: %w", backupPath, err)
	}

	pruneOldBackups(path)

	return backupPath, nil
}

// pruneOldBackups removes the oldest backups for the given file,
// keeping only the maxBackups most recent ones.
func pruneOldBackups(path string) {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	prefix := base + ext + ".bak."

	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if strings.HasPrefix(full, prefix) {
			backups = append(backups, full)
		}
	}

	if len(backups) <= maxBackups {
		return
	}

	// Sort lexicographically (timestamp format sorts chronologically)
	sort.Strings(backups)

	// Remove oldest, keep the last maxBackups entries
	for _, old := range backups[:len(backups)-maxBackups] {
		_ = os.Remove(old)
	}
}
