package ws

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.workspace.(InfraManager)
			assert.Equal(t, tt.implementsIM, ok,
				"%s: InfraManager implementation mismatch", tt.name)
		})
	}
}

// Compile-time interface satisfaction checks.
var (
	_ Workspace    = (*LocalWorkspace)(nil)
	_ Workspace    = (*ContainerWorkspace)(nil)
	_ Workspace    = (*ComposeWorkspace)(nil)
	_ Workspace    = (*SSHWorkspace)(nil)
	_ Workspace    = (*K8sDeployWorkspace)(nil)
	_ InfraManager = (*ContainerWorkspace)(nil)
	_ InfraManager = (*ComposeWorkspace)(nil)
	_ InfraManager = (*K8sDeployWorkspace)(nil)
)
