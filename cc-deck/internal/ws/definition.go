package ws

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cc-deck/cc-deck/internal/xdg"
	"gopkg.in/yaml.v3"
)

const (
	definitionFileName = "workspaces.yaml"
)

// WorkspaceSpec contains the shared configuration fields for workspace definitions
// and templates. Both WorkspaceDefinition (via embedding) and WorkspaceTemplate
// variants use this type, ensuring fields stay in sync.
type WorkspaceSpec struct {
	Image          string            `yaml:"image,omitempty"`
	Auth           string            `yaml:"auth,omitempty"`
	Storage        *StorageConfig    `yaml:"storage,omitempty"`
	Ports          []string          `yaml:"ports,omitempty"`
	Credentials    []string          `yaml:"credentials,omitempty"`
	Mounts         []string          `yaml:"mounts,omitempty"`
	AllowedDomains []string          `yaml:"allowed-domains,omitempty"`
	ProjectDir     string            `yaml:"project-dir,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`
	Host           string            `yaml:"host,omitempty"`
	Port           int               `yaml:"port,omitempty"`
	IdentityFile   string            `yaml:"identity-file,omitempty"`
	JumpHost       string            `yaml:"jump-host,omitempty"`
	SSHConfig      string            `yaml:"ssh-config,omitempty"`
	Workspace      string            `yaml:"workspace,omitempty"`
	Repos          []RepoEntry       `yaml:"repos,omitempty"`
	RemoteBG       string            `yaml:"remote-bg,omitempty"`
	Namespace      string            `yaml:"namespace,omitempty"`
	Kubeconfig     string            `yaml:"kubeconfig,omitempty"`
	K8sContext     string            `yaml:"context,omitempty"`
	StorageSize    string            `yaml:"storage-size,omitempty"`
	StorageClass   string            `yaml:"storage-class,omitempty"`
}

// WorkspaceDefinition is the declarative, user-editable description of a workspace.
type WorkspaceDefinition struct {
	Name            string            `yaml:"name"`
	Type            WorkspaceType     `yaml:"type"`
	WorkspaceSpec   `yaml:",inline"`
	ExtraRemotes    map[string]string `yaml:"-"`
	AutoDetectedURL string            `yaml:"-"`
}

// DefinitionFile is the top-level structure of the workspace definitions file.
type DefinitionFile struct {
	Version      int                     `yaml:"version"`
	Workspaces []WorkspaceDefinition `yaml:"workspaces"`
}

// DefinitionStore manages workspace definitions persisted as YAML on disk.
type DefinitionStore struct {
	path string
}

// DefaultDefinitionPath returns the XDG-compliant definition file path.
// If CC_DECK_WORKSPACES_FILE is set, it takes precedence (used by tests).
func DefaultDefinitionPath() string {
	if p := os.Getenv("CC_DECK_WORKSPACES_FILE"); p != "" {
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
// needed with mode 0o700. Files are written with mode 0o600.
func (s *DefinitionStore) Save(defs *DefinitionFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating definitions directory: %w", err)
	}

	data, err := yaml.Marshal(defs)
	if err != nil {
		return fmt.Errorf("marshaling definitions: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary definitions file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, writeErr := tmpFile.Write(data); writeErr != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing temporary definitions file: %w", writeErr)
	}
	if err := tmpFile.Chmod(0o600); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting file permissions: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming definitions file: %w", err)
	}

	return nil
}

// FindByName loads the definitions and returns the definition with the given
// name, or ErrNotFound if no such definition exists.
func (s *DefinitionStore) FindByName(name string) (*WorkspaceDefinition, error) {
	defs, err := s.Load()
	if err != nil {
		return nil, err
	}

	for i := range defs.Workspaces {
		if defs.Workspaces[i].Name == name {
			return &defs.Workspaces[i], nil
		}
	}

	return nil, fmt.Errorf("workspace definition %q: %w", name, ErrNotFound)
}

// Add appends a new workspace definition to the definitions file. Returns
// ErrNameConflict if a definition with the same name already exists.
func (s *DefinitionStore) Add(def *WorkspaceDefinition) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for _, d := range defs.Workspaces {
		if d.Name == def.Name {
			return fmt.Errorf("workspace definition %q: %w", def.Name, ErrNameConflict)
		}
	}

	defs.Workspaces = append(defs.Workspaces, *def)
	return s.Save(defs)
}

// Update replaces an existing workspace definition matched by name.
// Returns ErrNotFound if no definition with the given name exists.
func (s *DefinitionStore) Update(def *WorkspaceDefinition) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for i := range defs.Workspaces {
		if defs.Workspaces[i].Name == def.Name {
			defs.Workspaces[i] = *def
			return s.Save(defs)
		}
	}

	return fmt.Errorf("workspace definition %q: %w", def.Name, ErrNotFound)
}

// Remove deletes a workspace definition by name. Returns ErrNotFound if
// no definition with the given name exists.
func (s *DefinitionStore) Remove(name string) error {
	defs, err := s.Load()
	if err != nil {
		return err
	}

	for i := range defs.Workspaces {
		if defs.Workspaces[i].Name == name {
			defs.Workspaces = append(defs.Workspaces[:i], defs.Workspaces[i+1:]...)
			return s.Save(defs)
		}
	}

	return fmt.Errorf("workspace definition %q: %w", name, ErrNotFound)
}

// FindByProjectDir returns all definitions whose ProjectDir is an ancestor of
// (or equal to) the given path. Uses canonical paths for comparison.
func (s *DefinitionStore) FindByProjectDir(path string) ([]*WorkspaceDefinition, error) {
	defs, err := s.Load()
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolved = absPath
	}

	var result []*WorkspaceDefinition
	for i := range defs.Workspaces {
		projDir := defs.Workspaces[i].ProjectDir
		if projDir == "" {
			continue
		}
		absProjDir, absErr := filepath.Abs(projDir)
		if absErr != nil {
			continue
		}
		resolvedProj, evalErr := filepath.EvalSymlinks(absProjDir)
		if evalErr != nil {
			resolvedProj = absProjDir
		}

		if resolved == resolvedProj || strings.HasPrefix(resolved+"/", resolvedProj+"/") {
			result = append(result, &defs.Workspaces[i])
		}
	}

	return result, nil
}

// AddWithCollisionHandling adds a workspace definition with collision handling.
// If a definition with the same name and same type exists, returns an error.
// If a definition with the same name but different type exists, auto-suffixes
// the name with the type (e.g., "foo" -> "foo-ssh"). Returns the final name used.
func (s *DefinitionStore) AddWithCollisionHandling(def *WorkspaceDefinition) (string, error) {
	defs, err := s.Load()
	if err != nil {
		return "", err
	}

	finalName := def.Name
	for _, d := range defs.Workspaces {
		if d.Name == finalName {
			if d.Type == def.Type {
				return "", fmt.Errorf("workspace %q already exists (type: %s); delete it first", finalName, d.Type)
			}
			finalName = finalName + "-" + string(def.Type)
			break
		}
	}

	if finalName != def.Name {
		if err := ValidateWsName(finalName); err != nil {
			return "", fmt.Errorf("auto-suffixed name %q is invalid: %w", finalName, err)
		}
	}

	for _, d := range defs.Workspaces {
		if d.Name == finalName {
			return "", fmt.Errorf("workspace %q: %w", finalName, ErrNameConflict)
		}
	}

	saved := *def
	saved.Name = finalName
	defs.Workspaces = append(defs.Workspaces, saved)
	if err := s.Save(defs); err != nil {
		return "", err
	}
	def.Name = finalName
	return finalName, nil
}

// List returns all workspace definitions, optionally filtered by type.
func (s *DefinitionStore) List(filter *ListFilter) ([]*WorkspaceDefinition, error) {
	defs, err := s.Load()
	if err != nil {
		return nil, err
	}

	var result []*WorkspaceDefinition
	for i := range defs.Workspaces {
		if filter != nil && filter.Type != nil && defs.Workspaces[i].Type != *filter.Type {
			continue
		}
		result = append(result, &defs.Workspaces[i])
	}

	return result, nil
}

