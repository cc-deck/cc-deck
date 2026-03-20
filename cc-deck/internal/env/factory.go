package env

// TODO: Implement NewEnvironment factory function alongside LocalEnvironment
// in T009 (Wave 2). The factory will dispatch on EnvironmentType:
//
//   func NewEnvironment(envType EnvironmentType, name string, store *FileStateStore) (Environment, error)
//
// - EnvironmentTypeLocal  -> &LocalEnvironment{name: name, store: store}
// - All others            -> nil, fmt.Errorf("%s: %w", envType, ErrNotImplemented)
//
// LocalEnvironment will be defined in local.go as part of Wave 2.
