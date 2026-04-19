package env

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
	"gopkg.in/yaml.v3"
)

const (
	stateDirName  = "cc-deck"
	stateFileName = "state.yaml"
)

// DefaultStatePath returns the XDG-compliant state file path.
// If CC_DECK_STATE_FILE is set, it takes precedence (used by tests).
func DefaultStatePath() string {
	if p := os.Getenv("CC_DECK_STATE_FILE"); p != "" {
		return p
	}
	return filepath.Join(xdg.StateHome, stateDirName, stateFileName)
}

// FileStateStore manages environment records persisted as YAML on disk.
type FileStateStore struct {
	path string
}

// NewStateStore creates a new FileStateStore. If path is empty, the
// default XDG state path is used.
func NewStateStore(path string) *FileStateStore {
	if path == "" {
		path = DefaultStatePath()
	}
	return &FileStateStore{path: path}
}

// Path returns the file path used by this store.
func (s *FileStateStore) Path() string {
	return s.path
}

// Load reads and parses the state file. If the file does not exist, an
// empty StateFile with Version=2 is returned. If the file is corrupted,
// a warning is logged and an empty state is returned.
func (s *FileStateStore) Load() (*StateFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StateFile{Version: 2}, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state StateFile
	if err := yaml.Unmarshal(data, &state); err != nil {
		log.Printf("WARNING: corrupted state file %s: %v; returning empty state", s.path, err)
		return &StateFile{Version: 2}, nil
	}

	if state.Version == 0 {
		state.Version = 2
	}

	return &state, nil
}

// Save writes the state file atomically by writing to a temporary file
// first, then renaming it into place. Parent directories are created as
// needed with mode 0o755.
func (s *FileStateStore) Save(state *StateFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temporary state file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		// Clean up the temporary file on rename failure.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// FindInstanceByName loads the state and returns the instance with the given
// name, or ErrNotFound if no such instance exists.
func (s *FileStateStore) FindInstanceByName(name string) (*EnvironmentInstance, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}

	for i := range state.Instances {
		if state.Instances[i].Name == name {
			return &state.Instances[i], nil
		}
	}

	return nil, fmt.Errorf("environment %q: %w", name, ErrNotFound)
}

// AddInstance appends a new environment instance to the state file. Returns
// ErrNameConflict if an instance with the same name already exists.
func (s *FileStateStore) AddInstance(inst *EnvironmentInstance) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for _, existing := range state.Instances {
		if existing.Name == inst.Name {
			return fmt.Errorf("environment %q already exists (type: %s): %w", inst.Name, existing.Type, ErrNameConflict)
		}
	}

	state.Instances = append(state.Instances, *inst)
	return s.Save(state)
}

// UpdateInstance replaces an existing environment instance matched by name.
// Returns ErrNotFound if no instance with the given name exists.
func (s *FileStateStore) UpdateInstance(inst *EnvironmentInstance) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for i := range state.Instances {
		if state.Instances[i].Name == inst.Name {
			state.Instances[i] = *inst
			return s.Save(state)
		}
	}

	return fmt.Errorf("environment %q: %w", inst.Name, ErrNotFound)
}

// RemoveInstance deletes an environment instance by name. Returns ErrNotFound
// if no instance with the given name exists.
func (s *FileStateStore) RemoveInstance(name string) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for i := range state.Instances {
		if state.Instances[i].Name == name {
			state.Instances = append(state.Instances[:i], state.Instances[i+1:]...)
			return s.Save(state)
		}
	}

	return fmt.Errorf("environment %q: %w", name, ErrNotFound)
}

// ListInstances returns all environment instances, optionally filtered by type.
func (s *FileStateStore) ListInstances(filter *ListFilter) ([]*EnvironmentInstance, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}

	var result []*EnvironmentInstance
	for i := range state.Instances {
		if filter != nil && filter.Type != nil && state.Instances[i].Type != *filter.Type {
			continue
		}
		result = append(result, &state.Instances[i])
	}

	return result, nil
}

// RegisterProject adds or updates a project entry in the global registry.
// Uses canonical (symlink-resolved) paths. Updates last_seen if already registered.
func (s *FileStateStore) RegisterProject(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}

	if strings.Contains(resolved, "/.cc-deck/") {
		return fmt.Errorf("refusing to register project inside a .cc-deck/ directory: %s", resolved)
	}

	state, err := s.Load()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Truncate(time.Second)
	for i := range state.Projects {
		if state.Projects[i].Path == resolved {
			state.Projects[i].LastSeen = now
			return s.Save(state)
		}
	}

	state.Projects = append(state.Projects, ProjectEntry{
		Path:     resolved,
		LastSeen: now,
	})
	return s.Save(state)
}

// UnregisterProject removes a project entry from the global registry.
func (s *FileStateStore) UnregisterProject(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}

	state, err := s.Load()
	if err != nil {
		return err
	}

	for i := range state.Projects {
		if state.Projects[i].Path == resolved {
			state.Projects = append(state.Projects[:i], state.Projects[i+1:]...)
			return s.Save(state)
		}
	}

	return nil
}

// ListProjects returns all project entries from the global registry.
func (s *FileStateStore) ListProjects() ([]ProjectEntry, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}
	return state.Projects, nil
}

// AllProjectEnvironmentNames returns a map of environment name to project path
// for all registered projects that have a .cc-deck/workspace.yaml definition.
// The excludePath argument (if non-empty) is skipped, allowing callers to
// exclude the current project when checking for cross-project collisions.
func (s *FileStateStore) AllProjectEnvironmentNames(excludePath string) (map[string]string, error) {
	projects, err := s.ListProjects()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, p := range projects {
		if excludePath != "" && p.Path == excludePath {
			continue
		}
		if _, statErr := os.Stat(p.Path); statErr != nil {
			continue
		}
		def, loadErr := LoadProjectDefinition(p.Path)
		if loadErr != nil {
			continue
		}
		result[def.Name] = p.Path
	}
	return result, nil
}

// PruneStaleProjects removes entries whose paths no longer exist.
// Returns the count of removed entries.
func (s *FileStateStore) PruneStaleProjects() (int, error) {
	_, count, err := s.PruneStaleProjectsVerbose()
	return count, err
}

// PruneStaleProjectsVerbose removes entries whose paths no longer exist.
// Returns the removed paths and count.
func (s *FileStateStore) PruneStaleProjectsVerbose() ([]string, int, error) {
	state, err := s.Load()
	if err != nil {
		return nil, 0, err
	}

	var kept []ProjectEntry
	var removedPaths []string
	for _, p := range state.Projects {
		if _, err := os.Stat(p.Path); err != nil {
			removedPaths = append(removedPaths, p.Path)
			continue
		}
		kept = append(kept, p)
	}

	if len(removedPaths) == 0 {
		return nil, 0, nil
	}

	state.Projects = kept
	if err := s.Save(state); err != nil {
		return nil, 0, err
	}
	return removedPaths, len(removedPaths), nil
}
