// T018: Timestamped backup logic for settings.json

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BackupFile creates a timestamped backup of the given file.
// Returns the backup path, or empty string if the file doesn't exist.
// If skipBackup is true, no backup is created.
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

	return backupPath, nil
}
