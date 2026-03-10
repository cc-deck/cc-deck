package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

const sessionsDirName = "cc-deck/sessions"

// SessionEntry represents one Claude Code session within a snapshot.
type SessionEntry struct {
	TabName     string `json:"tab_name"`
	WorkingDir  string `json:"working_dir"`
	SessionID   string `json:"session_id"`
	DisplayName string `json:"display_name"`
	Paused      bool   `json:"paused"`
	GitBranch   string `json:"git_branch,omitempty"`
	tabIndex    int    // internal, for sorting during save
}

// Snapshot is a point-in-time capture of all tracked sessions.
type Snapshot struct {
	Version  int            `json:"version"`
	Name     string         `json:"name"`
	SavedAt  time.Time      `json:"saved_at"`
	AutoSave bool           `json:"auto_save"`
	Sessions []SessionEntry `json:"sessions"`
}

// SessionsDir returns the XDG-conformant sessions directory path.
func SessionsDir() string {
	return filepath.Join(xdg.ConfigHome, sessionsDirName)
}

// snapshotPath returns the file path for a named snapshot.
func snapshotPath(name string) string {
	return filepath.Join(SessionsDir(), name+".json")
}

// SaveSnapshot writes a snapshot to disk using atomic rename.
func SaveSnapshot(snap *Snapshot) error {
	dir := SessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating sessions directory: %w", err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling snapshot: %w", err)
	}

	target := snapshotPath(snap.Name)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// LoadSnapshot reads a snapshot from disk by name.
func LoadSnapshot(name string) (*Snapshot, error) {
	data, err := os.ReadFile(snapshotPath(name))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %q: %w", name, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing snapshot %q: %w", name, err)
	}
	return &snap, nil
}

// SnapshotInfo holds summary info for listing.
type SnapshotInfo struct {
	Name         string
	SavedAt      time.Time
	SessionCount int
	AutoSave     bool
}

// ListSnapshots returns all snapshots sorted by timestamp (newest first).
func ListSnapshots() ([]SnapshotInfo, error) {
	dir := SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var infos []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		snap, err := LoadSnapshot(name)
		if err != nil {
			continue
		}
		infos = append(infos, SnapshotInfo{
			Name:         snap.Name,
			SavedAt:      snap.SavedAt,
			SessionCount: len(snap.Sessions),
			AutoSave:     snap.AutoSave,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].SavedAt.After(infos[j].SavedAt)
	})
	return infos, nil
}

// LatestSnapshot returns the most recent snapshot (auto or named).
func LatestSnapshot() (*Snapshot, error) {
	infos, err := ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}
	return LoadSnapshot(infos[0].Name)
}

// RemoveSnapshot deletes a snapshot by name.
func RemoveSnapshot(name string) error {
	path := snapshotPath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("snapshot %q not found", name)
	}
	return os.Remove(path)
}

// RemoveAllSnapshots deletes all snapshots.
func RemoveAllSnapshots() (int, error) {
	dir := SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
				count++
			}
		}
	}
	return count, nil
}
