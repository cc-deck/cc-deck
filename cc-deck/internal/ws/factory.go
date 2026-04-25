package ws

import "fmt"

// NewWorkspace creates a Workspace implementation for the given type.
// The defs parameter is optional (may be nil) and is used by container
// workspaces to read/write workspace definitions.
func NewWorkspace(wsType WorkspaceType, name string, store *FileStateStore, defs *DefinitionStore) (Workspace, error) {
	switch wsType {
	case WorkspaceTypeLocal:
		return &LocalWorkspace{name: name, store: store, defs: defs}, nil
	case WorkspaceTypeContainer:
		return &ContainerWorkspace{name: name, store: store, defs: defs}, nil
	case WorkspaceTypeCompose:
		return &ComposeWorkspace{name: name, store: store, defs: defs}, nil
	case WorkspaceTypeSSH:
		return &SSHWorkspace{name: name, store: store, defs: defs}, nil
	case WorkspaceTypeK8sDeploy:
		return &K8sDeployWorkspace{name: name, store: store, defs: defs}, nil
	default:
		return nil, fmt.Errorf("%s: %w", wsType, ErrNotImplemented)
	}
}
