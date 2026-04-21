package ws

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cc-deck/cc-deck/internal/xdg"
	"gopkg.in/yaml.v3"
)

const (
	definitionFileName = "workspaces.yaml"
)

// WorkspaceDefinition is the declarative, user-editable description of a workspace.
type WorkspaceDefinition struct {
	Name        string          `yaml:"name"`
	Type        WorkspaceType `yaml:"type"`
	Image       string          `yaml:"image,omitempty"`
	Auth        string          `yaml:"auth,omitempty"` // Auth mode: auto (default), none, api, vertex, bedrock
	Storage     *StorageConfig  `yaml:"storage,omitempty"`
	Ports       []string        `yaml:"ports,omitempty"`
	Credentials []string        `yaml:"credentials,omitempty"`
	Mounts         []string          `yaml:"mounts,omitempty"`          // Bind mounts as "src:dst[:ro]" (container/compose only)
	AllowedDomains []string          `yaml:"allowed-domains,omitempty"` // Domain groups for network filtering
	ProjectDir     string            `yaml:"project-dir,omitempty"`     // Project directory (compose only)
	Env            map[string]string `yaml:"env,omitempty"`             // Arbitrary environment variables
	Host           string            `yaml:"host,omitempty"`            // SSH target (user@host)
	Port           int               `yaml:"port,omitempty"`            // SSH port
	IdentityFile   string            `yaml:"identity-file,omitempty"`   // Path to SSH private key
	JumpHost       string            `yaml:"jump-host,omitempty"`       // Bastion/jump host
	SSHConfig      string            `yaml:"ssh-config,omitempty"`      // Custom SSH config file
	Workspace      string            `yaml:"workspace,omitempty"`       // Remote workspace directory
	Repos          []RepoEntry       `yaml:"repos,omitempty"`           // Git repos to clone into workspace
	ExtraRemotes   map[string]string `yaml:"-"`                         // Transient: additional remotes for auto-detected repo
	AutoDetectedURL string           `yaml:"-"`                         // Transient: normalized URL of auto-detected repo

	// k8s-deploy fields
	Namespace    string `yaml:"namespace,omitempty"`     // K8s namespace
	Kubeconfig   string `yaml:"kubeconfig,omitempty"`    // Path to kubeconfig
	K8sContext   string `yaml:"context,omitempty"`       // Kubeconfig context name
	StorageSize  string `yaml:"storage-size,omitempty"`  // PVC size (default: 10Gi)
	StorageClass string `yaml:"storage-class,omitempty"` // K8s StorageClass name
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

const projectDefinitionFile = ".cc-deck/workspace.yaml"

// LoadProjectDefinition reads the workspace definition from
// .cc-deck/workspace.yaml in the given project root.
// Returns a bare WorkspaceDefinition (not wrapped in DefinitionFile).
func LoadProjectDefinition(projectRoot string) (*WorkspaceDefinition, error) {
	path := filepath.Join(projectRoot, projectDefinitionFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project definition %q: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("reading project definition: %w", err)
	}

	var def WorkspaceDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing project definition: %w", err)
	}

	return &def, nil
}

// SaveProjectDefinition writes a workspace definition to
// .cc-deck/workspace.yaml, creating the directory and .gitignore if needed.
func SaveProjectDefinition(projectRoot string, def *WorkspaceDefinition) error {
	dir := filepath.Join(projectRoot, ".cc-deck")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .cc-deck directory: %w", err)
	}

	if err := EnsureCCDeckGitignore(projectRoot); err != nil {
		return fmt.Errorf("ensuring .gitignore: %w", err)
	}

	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshaling project definition: %w", err)
	}

	path := filepath.Join(projectRoot, projectDefinitionFile)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing temporary project definition: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming project definition: %w", err)
	}

	return nil
}
