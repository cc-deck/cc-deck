package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8sDeployWorkspace_Type(t *testing.T) {
	e := &K8sDeployWorkspace{name: "test"}
	assert.Equal(t, WorkspaceTypeK8sDeploy, e.Type())
}

func TestK8sDeployWorkspace_Name(t *testing.T) {
	e := &K8sDeployWorkspace{name: "my-env"}
	assert.Equal(t, "my-env", e.Name())
}

func TestK8sDeployWorkspace_ResolveNamespace(t *testing.T) {
	tests := []struct {
		name      string
		envNS     string
		instNS    string
		expected  string
	}{
		{"from instance", "", "inst-ns", "inst-ns"},
		{"from env config", "env-ns", "", "env-ns"},
		{"instance takes precedence", "env-ns", "inst-ns", "inst-ns"},
		{"default", "", "", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sDeployWorkspace{Namespace: tt.envNS}
			inst := &WorkspaceInstance{K8s: &K8sFields{Namespace: tt.instNS}}
			assert.Equal(t, tt.expected, e.resolveNamespace(inst))
		})
	}
}

func TestK8sDeployWorkspace_ResolveKubeconfig(t *testing.T) {
	tests := []struct {
		name     string
		envKC    string
		instKC   string
		expected string
	}{
		{"from instance", "", "/inst/kc", "/inst/kc"},
		{"from env config", "/env/kc", "", "/env/kc"},
		{"instance takes precedence", "/env/kc", "/inst/kc", "/inst/kc"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sDeployWorkspace{Kubeconfig: tt.envKC}
			inst := &WorkspaceInstance{K8s: &K8sFields{Kubeconfig: tt.instKC}}
			assert.Equal(t, tt.expected, e.resolveKubeconfig(inst))
		})
	}
}

func TestK8sDeployWorkspace_ResolveContext(t *testing.T) {
	e := &K8sDeployWorkspace{Context: "env-ctx"}
	inst := &WorkspaceInstance{K8s: &K8sFields{Profile: "inst-ctx"}}
	assert.Equal(t, "inst-ctx", e.resolveContext(inst))

	inst2 := &WorkspaceInstance{K8s: &K8sFields{}}
	assert.Equal(t, "env-ctx", e.resolveContext(inst2))
}

func TestK8sDeployWorkspace_KubeconfigArgs(t *testing.T) {
	tests := []struct {
		name     string
		kc       string
		ctx      string
		expected []string
	}{
		{"empty", "", "", nil},
		{"kubeconfig only", "/path/kc", "", []string{"--kubeconfig", "/path/kc"}},
		{"context only", "", "my-ctx", []string{"--context", "my-ctx"}},
		{"both", "/path/kc", "my-ctx", []string{"--kubeconfig", "/path/kc", "--context", "my-ctx"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sDeployWorkspace{Kubeconfig: tt.kc, Context: tt.ctx}
			inst := &WorkspaceInstance{K8s: &K8sFields{}}
			args := e.kubeconfigArgs(inst)
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestK8sStandardLabels(t *testing.T) {
	labels := k8sStandardLabels("test-env")
	assert.Equal(t, "cc-deck", labels["app.kubernetes.io/name"])
	assert.Equal(t, "test-env", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "cc-deck", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "workspace", labels["app.kubernetes.io/component"])
}

func TestK8sDeployWorkspace_DefaultTimeout(t *testing.T) {
	e := &K8sDeployWorkspace{}
	assert.Equal(t, time.Duration(0), e.Timeout)
	// The Create method should use defaultPodTimeout when Timeout is 0.
	assert.Equal(t, 5*time.Minute, defaultPodTimeout)
}

func TestK8sDeployWorkspace_DefaultStorageSize(t *testing.T) {
	assert.Equal(t, "10Gi", defaultStorageSize)
}

func TestK8sDeployWorkspace_Create_InvalidName(t *testing.T) {
	store := createTempStateStore(t)
	e := &K8sDeployWorkspace{name: "INVALID_NAME", store: store}
	err := e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
}

func TestK8sDeployWorkspace_Create_NameConflict(t *testing.T) {
	store := createTempStateStore(t)
	// Pre-populate with an instance.
	_ = store.AddInstance(&WorkspaceInstance{
		Name:      "existing",
		Type:      WorkspaceTypeK8sDeploy,
		State:     WorkspaceStateRunning,
		CreatedAt: time.Now(),
	})

	e := &K8sDeployWorkspace{name: "existing", store: store}
	err := e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNameConflict)
}

func TestK8sDeployWorkspace_Delete_KeepVolumes(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&WorkspaceInstance{
		Name:  "keep-vol",
		Type:  WorkspaceTypeK8sDeploy,
		InfraState: infraStatePtr(InfraStateStopped), SessionState: SessionStateNone,
		K8s:   &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployWorkspace{name: "keep-vol", store: store, KeepVolumes: true}
	assert.True(t, e.KeepVolumes)

	e2 := &K8sDeployWorkspace{name: "keep-vol", store: store, KeepVolumes: false}
	assert.False(t, e2.KeepVolumes)
}

func TestK8sDeployWorkspace_Delete_RunningWithoutForce(t *testing.T) {
	store := createTempStateStore(t)
	running := InfraStateRunning
	_ = store.AddInstance(&WorkspaceInstance{
		Name:         "running-env",
		Type:         WorkspaceTypeK8sDeploy,
		InfraState:   &running,
		SessionState: SessionStateNone,
		K8s:          &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployWorkspace{name: "running-env", store: store}
	err := e.Delete(t.Context(), false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunning)
}

func TestK8sDeployWorkspace_NotRunningErrorIsActionable(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&WorkspaceInstance{
		Name:  "stopped-env",
		Type:  WorkspaceTypeK8sDeploy,
		InfraState: infraStatePtr(InfraStateStopped), SessionState: SessionStateNone,
		K8s:   &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployWorkspace{name: "stopped-env", store: store}
	err := e.Exec(t.Context(), []string{"ls"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cc-deck ws start")
}

func TestK8sDeployWorkspace_ESOValidation(t *testing.T) {
	store := createTempStateStore(t)
	e := &K8sDeployWorkspace{
		name:        "eso-test",
		store:       store,
		SecretStore: "vault",
	}

	err := e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--secret-store-ref is required")

	e.SecretStoreRef = "my-vault"
	err = e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--secret-path is required")
}

func TestReconcileK8sDeployWorkspaces_EmptyStore(t *testing.T) {
	store := createTempStateStore(t)
	err := ReconcileK8sDeployWorkspaces(store)
	require.NoError(t, err)
}

func TestReconcileK8sDeployWorkspaces_SkipsNonK8s(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&WorkspaceInstance{
		Name:  "local-env",
		Type:  WorkspaceTypeLocal,
		InfraState: infraStatePtr(InfraStateRunning), SessionState: SessionStateNone,
	})

	err := ReconcileK8sDeployWorkspaces(store)
	require.NoError(t, err)

	inst, _ := store.FindInstanceByName("local-env")
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
}

func TestReconcileK8sDeployWorkspaces_SkipsNilK8sFields(t *testing.T) {
	store := createTempStateStore(t)
	running := InfraStateRunning
	_ = store.AddInstance(&WorkspaceInstance{
		Name:         "broken-k8s",
		Type:         WorkspaceTypeK8sDeploy,
		InfraState:   &running,
		SessionState: SessionStateNone,
		K8s:          nil,
	})

	err := ReconcileK8sDeployWorkspaces(store)
	require.NoError(t, err)

	inst, _ := store.FindInstanceByName("broken-k8s")
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
}

func infraStatePtr(s InfraStateValue) *InfraStateValue { return &s }

func createTempStateStore(t *testing.T) *FileStateStore {
	t.Helper()
	dir := t.TempDir()
	return NewStateStore(dir + "/state.yaml")
}
