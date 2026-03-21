package env

import "fmt"

// NewEnvironment creates an Environment implementation for the given type.
// The defs parameter is optional (may be nil) and is used by container
// environments to read/write environment definitions.
func NewEnvironment(envType EnvironmentType, name string, store *FileStateStore, defs *DefinitionStore) (Environment, error) {
	switch envType {
	case EnvironmentTypeLocal:
		return &LocalEnvironment{name: name, store: store}, nil
	case EnvironmentTypeContainer:
		return &ContainerEnvironment{name: name, store: store, defs: defs}, nil
	default:
		return nil, fmt.Errorf("%s: %w", envType, ErrNotImplemented)
	}
}
