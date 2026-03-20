package env

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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

// FindByName loads the state and returns the record with the given name,
// or ErrNotFound if no such record exists.
func (s *FileStateStore) FindByName(name string) (*EnvironmentRecord, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}

	for i := range state.Environments {
		if state.Environments[i].Name == name {
			return &state.Environments[i], nil
		}
	}

	return nil, fmt.Errorf("environment %q: %w", name, ErrNotFound)
}

// Add appends a new environment record to the state file. Returns
// ErrNameConflict if an environment with the same name already exists.
func (s *FileStateStore) Add(record *EnvironmentRecord) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for _, env := range state.Environments {
		if env.Name == record.Name {
			return fmt.Errorf("environment %q: %w", record.Name, ErrNameConflict)
		}
	}

	state.Environments = append(state.Environments, *record)
	return s.Save(state)
}

// Update replaces an existing environment record matched by name.
// Returns ErrNotFound if no record with the given name exists.
func (s *FileStateStore) Update(record *EnvironmentRecord) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for i := range state.Environments {
		if state.Environments[i].Name == record.Name {
			state.Environments[i] = *record
			return s.Save(state)
		}
	}

	return fmt.Errorf("environment %q: %w", record.Name, ErrNotFound)
}

// Remove deletes an environment record by name. Returns ErrNotFound if
// no record with the given name exists.
func (s *FileStateStore) Remove(name string) error {
	state, err := s.Load()
	if err != nil {
		return err
	}

	for i := range state.Environments {
		if state.Environments[i].Name == name {
			state.Environments = append(state.Environments[:i], state.Environments[i+1:]...)
			return s.Save(state)
		}
	}

	return fmt.Errorf("environment %q: %w", name, ErrNotFound)
}

// List returns all environment records, optionally filtered by type.
func (s *FileStateStore) List(filter *ListFilter) ([]*EnvironmentRecord, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}

	var result []*EnvironmentRecord
	for i := range state.Environments {
		if filter != nil && filter.Type != nil && state.Environments[i].Type != *filter.Type {
			continue
		}
		result = append(result, &state.Environments[i])
	}

	return result, nil
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

	return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
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
			return fmt.Errorf("instance %q: %w", inst.Name, ErrNameConflict)
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

	return fmt.Errorf("instance %q: %w", inst.Name, ErrNotFound)
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

	return fmt.Errorf("instance %q: %w", name, ErrNotFound)
}

// ListInstances returns all environment instances from the state file.
func (s *FileStateStore) ListInstances() ([]*EnvironmentInstance, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}

	var result []*EnvironmentInstance
	for i := range state.Instances {
		result = append(result, &state.Instances[i])
	}

	return result, nil
}
