package env

import "fmt"

// NewEnvironment creates an Environment implementation for the given type.
// Currently only EnvironmentTypeLocal is implemented; all other types
// return ErrNotImplemented.
func NewEnvironment(envType EnvironmentType, name string, store *FileStateStore) (Environment, error) {
	switch envType {
	case EnvironmentTypeLocal:
		return &LocalEnvironment{name: name, store: store}, nil
	default:
		return nil, fmt.Errorf("%s: %w", envType, ErrNotImplemented)
	}
}
