package ws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfraManagerImplementation(t *testing.T) {
	store := newTestStore(t)

	tests := []struct {
		name         string
		workspace    Workspace
		implementsIM bool
	}{
		{"local", &LocalWorkspace{name: "t", store: store}, false},
		{"container", &ContainerWorkspace{name: "t", store: store}, true},
		{"compose", &ComposeWorkspace{name: "t", store: store}, true},
		{"ssh", &SSHWorkspace{name: "t", store: store}, false},
		{"k8s-deploy", &K8sDeployWorkspace{name: "t", store: store}, true},
		{"openshell", &OpenShellWorkspace{name: "t", store: store}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.workspace.(InfraManager)
			assert.Equal(t, tt.implementsIM, ok,
				"%s: InfraManager implementation mismatch", tt.name)
		})
	}
}

func TestNonLocalSessionManagersRejectWebSharing(t *testing.T) {
	managers := map[string]SessionManager{
		"container":  &ContainerWorkspace{},
		"compose":    &ComposeWorkspace{},
		"ssh":        &SSHWorkspace{},
		"k8s-deploy": &K8sDeployWorkspace{},
		"openshell":  &OpenShellWorkspace{},
	}
	for name, manager := range managers {
		t.Run(name, func(t *testing.T) {
			_, err := manager.EnsureSession(context.Background(), SessionStartOptions{WebSharing: true})
			require.ErrorContains(t, err, "sharing is currently supported for local workspaces only")
		})
	}
}

// Compile-time interface satisfaction checks.
var (
	_ Workspace      = (*LocalWorkspace)(nil)
	_ SessionManager = (*LocalWorkspace)(nil)
	_ Workspace      = (*ContainerWorkspace)(nil)
	_ SessionManager = (*ContainerWorkspace)(nil)
	_ Workspace      = (*ComposeWorkspace)(nil)
	_ SessionManager = (*ComposeWorkspace)(nil)
	_ Workspace      = (*SSHWorkspace)(nil)
	_ SessionManager = (*SSHWorkspace)(nil)
	_ Workspace      = (*K8sDeployWorkspace)(nil)
	_ SessionManager = (*K8sDeployWorkspace)(nil)
	_ InfraManager   = (*ContainerWorkspace)(nil)
	_ InfraManager   = (*ComposeWorkspace)(nil)
	_ InfraManager   = (*K8sDeployWorkspace)(nil)
	_ Workspace      = (*OpenShellWorkspace)(nil)
	_ SessionManager = (*OpenShellWorkspace)(nil)
	_ InfraManager   = (*OpenShellWorkspace)(nil)
)
