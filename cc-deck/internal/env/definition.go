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
	definitionFileName = "environments.yaml"
)

// EnvironmentDefinition is the declarative, user-editable description of an environment.
type EnvironmentDefinition struct {
	Name        string          `yaml:"name"`
	Type        EnvironmentType `yaml:"type"`
	Image       string          `yaml:"image,omitempty"`
	Auth        string          `yaml:"auth,omitempty"` // Auth mode: auto (default), none, api, vertex, bedrock
	Storage     *StorageConfig  `yaml:"storage,omitempty"`
	Ports       []string        `yaml:"ports,omitempty"`
	Credentials []string        `yaml:"credentials,omitempty"`
	Mounts      []string        `yaml:"mounts,omitempty"` // Bind mounts as "src:dst[:ro]" (container/compose only)
}

// DefinitionFile is the top-level structure of the environment definitions file.
type DefinitionFile struct {
	Version      int                     `yaml:"version"`
	Environments []EnvironmentDefinition `yaml:"environments"`
}

// DefinitionStore manages environment definitions persisted as YAML on disk.
type DefinitionStore struct {
	path string
}

// DefaultDefinitionPath returns the XDG-compliant definition file path.
// If CC_DECK_DEFINITIONS_FILE is set, it takes precedence (used by tests).
func DefaultDefinitionPath() string {
	if p := os.Getenv("CC_DECK_DEFINITIONS_FILE"); p != "" {
		return p
	}
	return filepath.Join(xdg.ConfigHome, stateDirName, definitionFileName)
}

// NewDefinitionStore creates a new DefinitionStore. If path is empty, the
// default XDG config path is used.
func NewDefinitionStore(path string) *DefinitionStore {
	if path == "" {
		path = DefaultDefinitionPath()
	}
	return &DefinitionStore{path: path}
}

// Path returns the file path used by this store.
func (s *DefinitionStore) Path() string {
	return s.path
}

// Load reads and parses the definitions file. If the file does not exist, an
// empty DefinitionFile with Version=1 is returned. If the file is corrupted,
// a warning is logged and an empty definition file is returned.
func (s *DefinitionStore) Load() (*DefinitionFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DefinitionFile{Version: 1}, nil
		}
		return nil, fmt.Errorf("reading definitions file: %w", err)
	}

	var defs DefinitionFile
	if err := yaml.Unmarshal(data, &defs); err != nil {
		log.Printf("WARNING: corrupted definitions file %s: %v; returning empty definitions", s.path, err)
		return &DefinitionFile{Version: 1}, nil
	}

	if defs.Version == 0 {
		defs.Version = 1
	}

	return &defs, nil
}

// Save writes the definitions file atomically by writing to a temporary file
// first, then renaming it into place. Parent directories are created as
// needed with mode 0o755.
func (s *DefinitionStore) Save(defs *DefinitionFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating definitions directory: %w", err)
	}

	data, err := yaml.Marshal(defs)
	if err != nil {
		return fmt.Errorf("marshaling definitions: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temporary definitions file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming definitions file: %w", err)
	}

	return nil
}

// FindByName loads the definitions and returns the definition with the given
// name, or ErrNotFound if no such definition exists.
func (s *DefinitionStore) FindByName(name string) (*EnvironmentDefinition, error) {
	defs, err := s.Load()
	if err != nil {
		return nil, err
	}

	for i := range defs.Environments {
		if defs.Environments[i].Name == name {
			return &defs.Environments[i], nil
		}
	}

	return nil, fmt.Errorf("environment definition %q: %w", name, ErrNotFound)
}

// Add appends a new environment definition to the definitions file. Returns
// ErrNameConflict if a definition with the same name already exists.
func (s *DefinitionStore) Add(def *EnvironmentDefinition) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for _, d := range defs.Environments {
		if d.Name == def.Name {
			return fmt.Errorf("environment definition %q: %w", def.Name, ErrNameConflict)
		}
	}

	defs.Environments = append(defs.Environments, *def)
	return s.Save(defs)
}

// Update replaces an existing environment definition matched by name.
// Returns ErrNotFound if no definition with the given name exists.
func (s *DefinitionStore) Update(def *EnvironmentDefinition) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for i := range defs.Environments {
		if defs.Environments[i].Name == def.Name {
			defs.Environments[i] = *def
			return s.Save(defs)
		}
	}

	return fmt.Errorf("environment definition %q: %w", def.Name, ErrNotFound)
}

// Remove deletes an environment definition by name. Returns ErrNotFound if
// no definition with the given name exists.
func (s *DefinitionStore) Remove(name string) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for i := range defs.Environments {
		if defs.Environments[i].Name == name {
			defs.Environments = append(defs.Environments[:i], defs.Environments[i+1:]...)
			return s.Save(defs)
		}
	}

	return fmt.Errorf("environment definition %q: %w", name, ErrNotFound)
}

// List returns all environment definitions, optionally filtered by type.
func (s *DefinitionStore) List(filter *ListFilter) ([]*EnvironmentDefinition, error) {
	defs, err := s.Load()
	if err != nil {
		return nil, err
	}

	var result []*EnvironmentDefinition
	for i := range defs.Environments {
		if filter != nil && filter.Type != nil && defs.Environments[i].Type != *filter.Type {
			continue
		}
		result = append(result, &defs.Environments[i])
	}

	return result, nil
}
