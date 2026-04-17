package env

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8sDeployEnvironment_Type(t *testing.T) {
	e := &K8sDeployEnvironment{name: "test"}
	assert.Equal(t, EnvironmentTypeK8sDeploy, e.Type())
}

func TestK8sDeployEnvironment_Name(t *testing.T) {
	e := &K8sDeployEnvironment{name: "my-env"}
	assert.Equal(t, "my-env", e.Name())
}

func TestK8sDeployEnvironment_ResolveNamespace(t *testing.T) {
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
			e := &K8sDeployEnvironment{Namespace: tt.envNS}
			inst := &EnvironmentInstance{K8s: &K8sFields{Namespace: tt.instNS}}
			assert.Equal(t, tt.expected, e.resolveNamespace(inst))
		})
	}
}

func TestK8sDeployEnvironment_ResolveKubeconfig(t *testing.T) {
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
			e := &K8sDeployEnvironment{Kubeconfig: tt.envKC}
			inst := &EnvironmentInstance{K8s: &K8sFields{Kubeconfig: tt.instKC}}
			assert.Equal(t, tt.expected, e.resolveKubeconfig(inst))
		})
	}
}

func TestK8sDeployEnvironment_ResolveContext(t *testing.T) {
	e := &K8sDeployEnvironment{Context: "env-ctx"}
	inst := &EnvironmentInstance{K8s: &K8sFields{Profile: "inst-ctx"}}
	assert.Equal(t, "inst-ctx", e.resolveContext(inst))

	inst2 := &EnvironmentInstance{K8s: &K8sFields{}}
	assert.Equal(t, "env-ctx", e.resolveContext(inst2))
}

func TestK8sDeployEnvironment_KubeconfigArgs(t *testing.T) {
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
			e := &K8sDeployEnvironment{Kubeconfig: tt.kc, Context: tt.ctx}
			inst := &EnvironmentInstance{K8s: &K8sFields{}}
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

func TestK8sDeployEnvironment_DefaultTimeout(t *testing.T) {
	e := &K8sDeployEnvironment{}
	assert.Equal(t, time.Duration(0), e.Timeout)
	// The Create method should use defaultPodTimeout when Timeout is 0.
	assert.Equal(t, 5*time.Minute, defaultPodTimeout)
}

func TestK8sDeployEnvironment_DefaultStorageSize(t *testing.T) {
	assert.Equal(t, "10Gi", defaultStorageSize)
}

func TestK8sDeployEnvironment_Create_InvalidName(t *testing.T) {
	store := createTempStateStore(t)
	e := &K8sDeployEnvironment{name: "INVALID_NAME", store: store}
	err := e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
}

func TestK8sDeployEnvironment_Create_NameConflict(t *testing.T) {
	store := createTempStateStore(t)
	// Pre-populate with an instance.
	_ = store.AddInstance(&EnvironmentInstance{
		Name:      "existing",
		Type:      EnvironmentTypeK8sDeploy,
		State:     EnvironmentStateRunning,
		CreatedAt: time.Now(),
	})

	e := &K8sDeployEnvironment{name: "existing", store: store}
	err := e.Create(t.Context(), CreateOpts{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNameConflict)
}

func TestK8sDeployEnvironment_Delete_KeepVolumes(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "keep-vol",
		Type:  EnvironmentTypeK8sDeploy,
		State: EnvironmentStateStopped,
		K8s:   &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployEnvironment{name: "keep-vol", store: store, KeepVolumes: true}
	assert.True(t, e.KeepVolumes)

	e2 := &K8sDeployEnvironment{name: "keep-vol", store: store, KeepVolumes: false}
	assert.False(t, e2.KeepVolumes)
}

func TestK8sDeployEnvironment_Delete_RunningWithoutForce(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "running-env",
		Type:  EnvironmentTypeK8sDeploy,
		State: EnvironmentStateRunning,
		K8s:   &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployEnvironment{name: "running-env", store: store}
	err := e.Delete(t.Context(), false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunning)
}

func TestK8sDeployEnvironment_NotRunningErrorIsActionable(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "stopped-env",
		Type:  EnvironmentTypeK8sDeploy,
		State: EnvironmentStateStopped,
		K8s:   &K8sFields{Namespace: "default"},
	})

	e := &K8sDeployEnvironment{name: "stopped-env", store: store}
	err := e.Exec(t.Context(), []string{"ls"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cc-deck env start")
}

func TestK8sDeployEnvironment_ESOValidation(t *testing.T) {
	store := createTempStateStore(t)
	e := &K8sDeployEnvironment{
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

func TestReconcileK8sDeployEnvs_EmptyStore(t *testing.T) {
	store := createTempStateStore(t)
	err := ReconcileK8sDeployEnvs(store)
	require.NoError(t, err)
}

func TestReconcileK8sDeployEnvs_SkipsNonK8s(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "local-env",
		Type:  EnvironmentTypeLocal,
		State: EnvironmentStateRunning,
	})

	err := ReconcileK8sDeployEnvs(store)
	require.NoError(t, err)

	inst, _ := store.FindInstanceByName("local-env")
	assert.Equal(t, EnvironmentStateRunning, inst.State)
}

func TestReconcileK8sDeployEnvs_SkipsNilK8sFields(t *testing.T) {
	store := createTempStateStore(t)
	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "broken-k8s",
		Type:  EnvironmentTypeK8sDeploy,
		State: EnvironmentStateRunning,
		K8s:   nil,
	})

	err := ReconcileK8sDeployEnvs(store)
	require.NoError(t, err)

	inst, _ := store.FindInstanceByName("broken-k8s")
	assert.Equal(t, EnvironmentStateRunning, inst.State)
}

func createTempStateStore(t *testing.T) *FileStateStore {
	t.Helper()
	dir := t.TempDir()
	return NewStateStore(dir + "/state.yaml")
}
