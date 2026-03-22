package env

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const statusFileName = "status.yaml"

// ProjectStatusStore manages per-project status files at .cc-deck/status.yaml.
type ProjectStatusStore struct {
	projectRoot string
}

// NewProjectStatusStore creates a store for the given project root directory.
func NewProjectStatusStore(projectRoot string) *ProjectStatusStore {
	return &ProjectStatusStore{projectRoot: projectRoot}
}

// statusPath returns the full path to .cc-deck/status.yaml.
func (s *ProjectStatusStore) statusPath() string {
	return filepath.Join(s.projectRoot, ccDeckDir, statusFileName)
}

// Load reads the project status file. Returns an empty status if the file
// does not exist.
func (s *ProjectStatusStore) Load() (*ProjectStatusFile, error) {
	data, err := os.ReadFile(s.statusPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectStatusFile{}, nil
		}
		return nil, fmt.Errorf("reading project status file: %w", err)
	}

	var status ProjectStatusFile
	if err := yaml.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parsing project status file: %w", err)
	}

	return &status, nil
}

// Save writes the status file atomically (write-to-tmp + rename).
func (s *ProjectStatusStore) Save(status *ProjectStatusFile) error {
	dir := filepath.Join(s.projectRoot, ccDeckDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .cc-deck directory: %w", err)
	}

	data, err := yaml.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshaling project status: %w", err)
	}

	path := s.statusPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temporary status file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming status file: %w", err)
	}

	return nil
}

// Remove deletes the status file.
func (s *ProjectStatusStore) Remove() error {
	err := os.Remove(s.statusPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing project status file: %w", err)
	}
	return nil
}
