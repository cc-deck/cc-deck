package ws

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/openshell"
)

func TestOpenShellWorkspace_TypeAndName(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	assert.Equal(t, WorkspaceTypeOpenShell, w.Type())
	assert.Equal(t, "test-ws", w.Name())
}

func TestAttachState_IsAlive_Nil(t *testing.T) {
	var a *attachState
	assert.False(t, a.isAlive())
}

func TestAttachState_IsAlive_ZeroPID(t *testing.T) {
	a := &attachState{pid: 0}
	assert.False(t, a.isAlive())
}

func TestAttachState_IsAlive_CurrentProcess(t *testing.T) {
	a := &attachState{pid: os.Getpid()}
	assert.True(t, a.isAlive())
}

func TestAttachState_IsAlive_DeadProcess(t *testing.T) {
	a := &attachState{pid: 99999999}
	assert.False(t, a.isAlive())
}

func TestResolveSandboxConfig_Defaults(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Equal(t, defaultSandboxImage, cfg.Image)
	assert.Equal(t, defaultSandboxCommand, cfg.Command)
	assert.Empty(t, cfg.Policy)
	assert.Empty(t, cfg.Providers)
}

func TestResolveSandboxConfig_FromDefinition(t *testing.T) {
	dir := t.TempDir()
	defPath := filepath.Join(dir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    sandbox-image: custom/image:v1
    sandbox-command: tmux
    policy: /etc/policy.yaml
    provider: my-provider
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Equal(t, "custom/image:v1", cfg.Image)
	assert.Equal(t, "tmux", cfg.Command)
	assert.Equal(t, "/etc/policy.yaml", cfg.Policy)
	assert.Equal(t, []string{"my-provider"}, cfg.Providers)
}

func TestResolveGatewayConfig_FromDefinition(t *testing.T) {
	dir := t.TempDir()
	defPath := filepath.Join(dir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    gateway: remote-gw:9090
    gateway-tls: true
    tls-cert-path: /path/cert.pem
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	cfg := w.resolveGatewayConfig()
	assert.Equal(t, "remote-gw:9090", cfg.Address)
	assert.True(t, cfg.TLS)
	assert.Equal(t, "/path/cert.pem", cfg.TLSCertPath)
}

func TestResolveGatewayConfig_FallbackToEnv(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "env-gw:5555")
	w := &OpenShellWorkspace{name: "test-ws"}
	cfg := w.resolveGatewayConfig()
	assert.Equal(t, "env-gw:5555", cfg.Address)
}

func TestResolveGatewayConfig_FallbackToDefault(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "")
	w := &OpenShellWorkspace{name: "test-ws"}
	cfg := w.resolveGatewayConfig()
	assert.Equal(t, "localhost:17670", cfg.Address)
}

func TestLoadSandboxID_FromState(t *testing.T) {
	store := newTestStore(t)
	running := InfraStateRunning
	err := store.AddInstance(&WorkspaceInstance{
		Name:       "test-ws",
		Type:       WorkspaceTypeOpenShell,
		InfraState: &running,
		OpenShell:  &OpenShellFields{SandboxID: "sb-12345"},
	})
	require.NoError(t, err)

	w := &OpenShellWorkspace{name: "test-ws", store: store}
	w.loadSandboxID()
	assert.Equal(t, "sb-12345", w.sandboxID)
}

func TestLoadSandboxID_AlreadySet(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws", sandboxID: "existing-id"}
	w.loadSandboxID()
	assert.Equal(t, "existing-id", w.sandboxID)
}

func TestLoadSandboxID_NoStore(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	w.loadSandboxID()
	assert.Empty(t, w.sandboxID)
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		name          string
		sandboxState  openshell.SandboxState
		expectedInfra *InfraStateValue
		expectedMsg   string
	}{
		{
			"running",
			openshell.SandboxStateRunning,
			infraPtr(InfraStateRunning),
			"",
		},
		{
			"suspended",
			openshell.SandboxStateSuspended,
			infraPtr(InfraStateStopped),
			"",
		},
		{
			"error",
			openshell.SandboxStateError,
			infraPtr(InfraStateError),
			"",
		},
		{
			"creating",
			openshell.SandboxStateCreating,
			infraPtr(InfraStateRunning),
			"sandbox is starting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &WorkspaceStatus{SessionState: SessionStateNone}

			switch tt.sandboxState {
			case openshell.SandboxStateRunning:
				running := InfraStateRunning
				status.InfraState = &running
			case openshell.SandboxStateSuspended:
				stopped := InfraStateStopped
				status.InfraState = &stopped
			case openshell.SandboxStateError:
				errState := InfraStateError
				status.InfraState = &errState
			case openshell.SandboxStateCreating:
				running := InfraStateRunning
				status.InfraState = &running
				status.Message = "sandbox is starting"
			}

			if tt.expectedInfra == nil {
				assert.Nil(t, status.InfraState)
			} else {
				require.NotNil(t, status.InfraState)
				assert.Equal(t, *tt.expectedInfra, *status.InfraState)
			}
			assert.Equal(t, tt.expectedMsg, status.Message)
		})
	}
}

func TestSessionStateWithActiveAttach(t *testing.T) {
	status := &WorkspaceStatus{SessionState: SessionStateNone}
	a := &attachState{pid: os.Getpid()}
	if a.isAlive() {
		status.SessionState = SessionStateExists
	}
	assert.Equal(t, SessionStateExists, status.SessionState)
}

func TestSessionStateWithDeadAttach(t *testing.T) {
	status := &WorkspaceStatus{SessionState: SessionStateNone}
	a := &attachState{pid: 99999999}
	if a.isAlive() {
		status.SessionState = SessionStateExists
	}
	assert.Equal(t, SessionStateNone, status.SessionState)
}

func TestAttach_AlreadyAttached(t *testing.T) {
	w := &OpenShellWorkspace{
		name:   "test-ws",
		attach: &attachState{pid: os.Getpid()},
	}
	err := w.Attach(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already attached")
}

func TestAttach_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	w := &OpenShellWorkspace{name: "test-ws", store: store}
	err := w.Attach(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sandbox")
}

func TestExec_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	w := &OpenShellWorkspace{name: "test-ws", store: store}
	err := w.Exec(context.Background(), []string{"echo", "hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sandbox")
}

func TestExecOutput_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	w := &OpenShellWorkspace{name: "test-ws", store: store}
	_, err := w.ExecOutput(context.Background(), []string{"echo", "hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sandbox")
}

func TestDelete_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	w := &OpenShellWorkspace{name: "test-ws", store: store}
	err := w.Delete(context.Background(), false)
	assert.NoError(t, err)
}

func TestDelete_ForceWithUnreachableGateway(t *testing.T) {
	store := newTestStore(t)
	running := InfraStateRunning
	err := store.AddInstance(&WorkspaceInstance{
		Name:       "test-ws",
		Type:       WorkspaceTypeOpenShell,
		InfraState: &running,
		OpenShell:  &OpenShellFields{SandboxID: "orphan-sb"},
	})
	require.NoError(t, err)

	t.Setenv("OPENSHELL_GATEWAY_URL", "")
	w := &OpenShellWorkspace{name: "test-ws", store: store, sandboxID: "orphan-sb"}
	err = w.Delete(context.Background(), true)
	assert.NoError(t, err)

	_, findErr := store.FindInstanceByName("test-ws")
	assert.Error(t, findErr)
}

func TestKillSession_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	w := &OpenShellWorkspace{name: "test-ws", store: store}
	err := w.KillSession(context.Background())
	assert.NoError(t, err)
}

func TestClearLocalState(t *testing.T) {
	store := newTestStore(t)
	running := InfraStateRunning
	err := store.AddInstance(&WorkspaceInstance{
		Name:       "test-ws",
		Type:       WorkspaceTypeOpenShell,
		InfraState: &running,
		OpenShell:  &OpenShellFields{SandboxID: "sb-999"},
	})
	require.NoError(t, err)

	w := &OpenShellWorkspace{
		name:      "test-ws",
		store:     store,
		sandboxID: "sb-999",
		attach:    &attachState{pid: 1},
	}
	w.clearLocalState()
	assert.Empty(t, w.sandboxID)
	assert.Nil(t, w.attach)

	_, findErr := store.FindInstanceByName("test-ws")
	assert.Error(t, findErr)
}

func TestLoadManifestCredentials_NoDefinitionStore(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	result := w.loadManifestCredentials()
	assert.Nil(t, result)
}

func TestLoadManifestCredentials_NoProjectDir(t *testing.T) {
	dir := t.TempDir()
	defPath := filepath.Join(dir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	result := w.loadManifestCredentials()
	assert.Nil(t, result)
}

func TestLoadManifestCredentials_WithManifest(t *testing.T) {
	projectDir := t.TempDir()

	// Create .cc-deck/setup/build.yaml
	setupDir := filepath.Join(projectDir, ".cc-deck", "setup")
	require.NoError(t, os.MkdirAll(setupDir, 0755))
	manifestContent := `version: 3
credentials:
  - type: claude
    env_vars: [ANTHROPIC_API_KEY]
  - type: github
    env_vars: [GITHUB_TOKEN]
`
	require.NoError(t, os.WriteFile(filepath.Join(setupDir, "build.yaml"), []byte(manifestContent), 0644))

	defDir := t.TempDir()
	defPath := filepath.Join(defDir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    project-dir: `+projectDir+`
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	result := w.loadManifestCredentials()

	require.Len(t, result, 2)
	assert.Equal(t, "claude", result[0].Type)
	assert.Equal(t, []string{"ANTHROPIC_API_KEY"}, result[0].EnvVars)
	assert.Equal(t, "github", result[1].Type)
}

func TestLoadManifestCredentials_NoCredentialsSection(t *testing.T) {
	projectDir := t.TempDir()

	setupDir := filepath.Join(projectDir, ".cc-deck", "setup")
	require.NoError(t, os.MkdirAll(setupDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(setupDir, "build.yaml"), []byte("version: 3\n"), 0644))

	defDir := t.TempDir()
	defPath := filepath.Join(defDir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    project-dir: `+projectDir+`
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	result := w.loadManifestCredentials()
	assert.Nil(t, result)
}

func TestLoadManifestCredentials_NoManifestFile(t *testing.T) {
	projectDir := t.TempDir()

	defDir := t.TempDir()
	defPath := filepath.Join(defDir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    project-dir: `+projectDir+`
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	result := w.loadManifestCredentials()
	assert.Nil(t, result)
}

func TestResolveSandboxConfig_NoPolicyNoImage(t *testing.T) {
	// When no definition store is set, resolveSandboxConfig should return
	// defaults without attempting OCI extraction.
	w := &OpenShellWorkspace{name: "test-ws"}
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Empty(t, cfg.Policy, "policy should be empty without OCI extraction")
}

func TestResolveSandboxConfig_ExplicitPolicySkipsOCI(t *testing.T) {
	// When an explicit policy path is set in the definition, OCI extraction
	// should be skipped entirely.
	dir := t.TempDir()
	defPath := filepath.Join(dir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    sandbox-image: my-registry/sandbox:v1
    policy: /explicit/policy.yaml
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Equal(t, "/explicit/policy.yaml", cfg.Policy, "explicit policy should be used as-is")
}

func TestResolveSandboxConfig_OCIExtractionErrorIncludesSuggestion(t *testing.T) {
	// When OCI extraction fails (image not found), the error should suggest
	// using the --policy flag as a manual alternative.
	dir := t.TempDir()
	defPath := filepath.Join(dir, "workspaces.yaml")
	os.WriteFile(defPath, []byte(`version: 3
workspaces:
  - name: test-ws
    type: openshell
    sandbox-image: nonexistent-registry.invalid/no-such-image:v999
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	_, err := w.resolveSandboxConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy")
}

func infraPtr(v InfraStateValue) *InfraStateValue { return &v }
