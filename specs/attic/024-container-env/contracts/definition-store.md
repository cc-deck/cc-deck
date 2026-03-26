# Contract: DefinitionStore

**Feature**: 024-container-env | **Date**: 2026-03-20

## Purpose

Manages human-editable environment definitions in `$XDG_CONFIG_HOME/cc-deck/environments.yaml`. Parallel to `FileStateStore` (which manages runtime state in `$XDG_STATE_HOME/cc-deck/state.yaml`).

## Go Interface

```go
package env

// DefinitionStore manages environment definitions persisted as YAML.
type DefinitionStore struct {
    path string
}

// DefinitionFile is the top-level structure of the definitions file.
type DefinitionFile struct {
    Version      int                     `yaml:"version"`
    Environments []EnvironmentDefinition `yaml:"environments"`
}

// EnvironmentDefinition is the declarative, user-editable description
// of an environment.
type EnvironmentDefinition struct {
    Name        string        `yaml:"name"`
    Type        EnvironmentType `yaml:"type"`
    Image       string        `yaml:"image,omitempty"`
    Storage     *StorageConfig `yaml:"storage,omitempty"`
    Ports       []string      `yaml:"ports,omitempty"`
    Credentials []string      `yaml:"credentials,omitempty"`
}

// NewDefinitionStore creates a new DefinitionStore. If path is empty,
// the default XDG config path is used.
func NewDefinitionStore(path string) *DefinitionStore

// DefaultDefinitionPath returns "$XDG_CONFIG_HOME/cc-deck/environments.yaml".
func DefaultDefinitionPath() string

// Load reads and parses the definitions file. Returns an empty
// DefinitionFile if the file does not exist.
func (s *DefinitionStore) Load() (*DefinitionFile, error)

// Save writes the definitions file atomically.
func (s *DefinitionStore) Save(defs *DefinitionFile) error

// FindByName returns the definition with the given name, or ErrNotFound.
func (s *DefinitionStore) FindByName(name string) (*EnvironmentDefinition, error)

// Add appends a new definition. Returns ErrNameConflict if name exists.
func (s *DefinitionStore) Add(def *EnvironmentDefinition) error

// Update replaces an existing definition by name.
func (s *DefinitionStore) Update(def *EnvironmentDefinition) error

// Remove deletes a definition by name.
func (s *DefinitionStore) Remove(name string) error

// List returns all definitions, optionally filtered by type.
func (s *DefinitionStore) List(filter *ListFilter) ([]*EnvironmentDefinition, error)
```

## File Format

```yaml
# $XDG_CONFIG_HOME/cc-deck/environments.yaml
version: 1
environments:
  - name: my-project
    type: container
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage:
      type: named-volume
    ports:
      - "8082:8082"
    credentials:
      - ANTHROPIC_API_KEY

  - name: local-dev
    type: local
```

## Environment Variable Override

`CC_DECK_DEFINITIONS_FILE` overrides the default path (used by tests), consistent with `CC_DECK_STATE_FILE` for the state store.

## Joining Definitions and State

The `cc-deck env list` command joins both stores:

```go
// Pseudocode for list display
defs := defStore.List(filter)
for _, def := range defs {
    instance, _ := stateStore.FindByName(def.Name)
    // Display: def.Name, def.Type, instance.State (or "not created"),
    //          def.Storage, instance.LastAttached, instance.CreatedAt
}
```

Definitions without a matching instance are shown with state "not created".
Instances without a matching definition are shown with a warning (orphaned state).
