package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	autoSavePrefix   = "auto-"
	maxAutoSaves     = 5
	autoSaveCooldown = 5 * time.Minute
)

// AutoSave performs a rolling auto-save if the cooldown has elapsed.
// Returns true if a save was performed, false if skipped.
func AutoSave() (bool, error) {
	if !cooldownElapsed() {
		return false, nil
	}

	snap, err := QueryPluginState("", true)
	if err != nil {
		return false, err
	}

	// Generate auto-save name
	snap.Name = autoSavePrefix + "1"
	snap.AutoSave = true

	// Rotate existing auto-saves: auto-1 -> auto-2, etc.
	if err := rotateAutoSaves(); err != nil {
		return false, fmt.Errorf("rotating auto-saves: %w", err)
	}

	if err := SaveSnapshot(snap); err != nil {
		return false, err
	}

	return true, nil
}

// cooldownElapsed checks if enough time has passed since the last auto-save.
func cooldownElapsed() bool {
	dir := SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true // no dir = no previous saves = proceed
	}

	var latest time.Time
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), autoSavePrefix) || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	if latest.IsZero() {
		return true
	}
	return time.Since(latest) >= autoSaveCooldown
}

// rotateAutoSaves shifts auto-N.json to auto-(N+1).json, deleting the oldest
// if it exceeds maxAutoSaves.
func rotateAutoSaves() error {
	dir := SessionsDir()

	// Collect existing auto-save numbers
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type autoFile struct {
		num  int
		name string
	}
	var files []autoFile
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), autoSavePrefix) || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		numStr := strings.TrimPrefix(e.Name(), autoSavePrefix)
		numStr = strings.TrimSuffix(numStr, ".json")
		var num int
		if _, err := fmt.Sscanf(numStr, "%d", &num); err == nil {
			files = append(files, autoFile{num: num, name: e.Name()})
		}
	}

	// Sort descending so we rename from highest to lowest
	sort.Slice(files, func(i, j int) bool {
		return files[i].num > files[j].num
	})

	for _, f := range files {
		newNum := f.num + 1
		if newNum > maxAutoSaves {
			// Delete the oldest
			os.Remove(filepath.Join(dir, f.name))
		} else {
			newName := fmt.Sprintf("%s%d.json", autoSavePrefix, newNum)
			os.Rename(
				filepath.Join(dir, f.name),
				filepath.Join(dir, newName),
			)
		}
	}

	return nil
}
