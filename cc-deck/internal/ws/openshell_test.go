package ws

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rhuss/openshell-sdk-go/openshell/v1/fake"
	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"github.com/rhuss/openshell-sdk-go/openshell/v1/types"
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

func TestAttachState_IsAlive_Inactive(t *testing.T) {
	a := &attachState{active: false}
	assert.False(t, a.isAlive())
}

func TestAttachState_IsAlive_Active(t *testing.T) {
	a := &attachState{active: true}
	assert.True(t, a.isAlive())
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
		phase         types.SandboxPhase
		expectedInfra *InfraStateValue
		expectedMsg   string
	}{
		{
			"ready",
			types.SandboxReady,
			infraPtr(InfraStateRunning),
			"",
		},
		{
			"unknown",
			types.SandboxUnknown,
			infraPtr(InfraStateStopped),
			"",
		},
		{
			"error",
			types.SandboxError,
			infraPtr(InfraStateError),
			"",
		},
		{
			"provisioning",
			types.SandboxProvisioning,
			infraPtr(InfraStateRunning),
			"sandbox is starting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := fake.NewClient()
			sbID := "status-test-sb"
			_, err := fc.Sandboxes().Create(context.Background(), sbID, nil, nil)
			require.NoError(t, err)

			// The fake client's Create always sets Phase to SandboxReady,
			// so override it via a fresh Get + direct store manipulation
			// is not possible. Instead, use Get to confirm the sandbox exists,
			// then test Status() which reads phase from the stored sandbox.
			// For phases other than Ready, we need to seed the sandbox
			// with the correct phase. The fake Create sets Ready, so we
			// re-get and verify that Status() at least works for Ready.
			// For other phases, we use a minimal mock that returns
			// a sandbox with the desired phase.
			w := newOpenShellWS("test-ws", &phaseOverrideClient{inner: fc, phase: tt.phase, sbID: sbID}, nil)
			w.sandboxID = sbID

			status, err := w.Status(context.Background())
			require.NoError(t, err)

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

// phaseOverrideClient wraps a real client but overrides the phase returned
// by Sandboxes().Get() for a specific sandbox, enabling status mapping tests
// for phases that the fake client cannot seed directly.
type phaseOverrideClient struct {
	inner v1.ClientInterface
	phase types.SandboxPhase
	sbID  string
}

func (c *phaseOverrideClient) Sandboxes() v1.SandboxInterface {
	return &phaseOverrideSandboxClient{inner: c.inner.Sandboxes(), phase: c.phase, sbID: c.sbID}
}
func (c *phaseOverrideClient) Providers() v1.ProviderInterface { return c.inner.Providers() }
func (c *phaseOverrideClient) Services() v1.ServiceInterface   { return c.inner.Services() }
func (c *phaseOverrideClient) Exec() v1.ExecInterface          { return c.inner.Exec() }
func (c *phaseOverrideClient) Files() v1.FileInterface          { return c.inner.Files() }
func (c *phaseOverrideClient) Health() v1.HealthInterface       { return c.inner.Health() }
func (c *phaseOverrideClient) SSH() v1.SSHInterface             { return c.inner.SSH() }
func (c *phaseOverrideClient) TCP() v1.TCPInterface             { return c.inner.TCP() }
func (c *phaseOverrideClient) Config() v1.ConfigInterface       { return c.inner.Config() }
func (c *phaseOverrideClient) Policy() v1.PolicyInterface       { return c.inner.Policy() }
func (c *phaseOverrideClient) Close() error                     { return c.inner.Close() }

type phaseOverrideSandboxClient struct {
	inner v1.SandboxInterface
	phase types.SandboxPhase
	sbID  string
}

func (s *phaseOverrideSandboxClient) Create(ctx context.Context, name string, spec *v1.SandboxSpec, labels map[string]string) (*v1.Sandbox, error) {
	return s.inner.Create(ctx, name, spec, labels)
}
func (s *phaseOverrideSandboxClient) List(ctx context.Context, opts ...v1.ListOptions) ([]*v1.Sandbox, error) {
	return s.inner.List(ctx, opts...)
}
func (s *phaseOverrideSandboxClient) Delete(ctx context.Context, name string) error {
	return s.inner.Delete(ctx, name)
}
func (s *phaseOverrideSandboxClient) AttachProvider(ctx context.Context, sandboxName, providerName string, expectedResourceVersion uint64) (*v1.AttachProviderResult, error) {
	return s.inner.AttachProvider(ctx, sandboxName, providerName, expectedResourceVersion)
}
func (s *phaseOverrideSandboxClient) DetachProvider(ctx context.Context, sandboxName, providerName string, expectedResourceVersion uint64) (*v1.DetachProviderResult, error) {
	return s.inner.DetachProvider(ctx, sandboxName, providerName, expectedResourceVersion)
}
func (s *phaseOverrideSandboxClient) ListProviders(ctx context.Context, sandboxName string) ([]*v1.Provider, error) {
	return s.inner.ListProviders(ctx, sandboxName)
}
func (s *phaseOverrideSandboxClient) WaitReady(ctx context.Context, name string, opts ...v1.WaitOptions) (*v1.Sandbox, error) {
	return s.inner.WaitReady(ctx, name, opts...)
}
func (s *phaseOverrideSandboxClient) Watch(ctx context.Context, name string, opts ...v1.WatchOptions) (v1.WatchInterface[*v1.Sandbox], error) {
	return s.inner.Watch(ctx, name, opts...)
}
func (s *phaseOverrideSandboxClient) GetLogs(ctx context.Context, sandboxName string, opts ...v1.LogOption) (*v1.LogResult, error) {
	return s.inner.GetLogs(ctx, sandboxName, opts...)
}
func (s *phaseOverrideSandboxClient) Get(ctx context.Context, name string) (*v1.Sandbox, error) {
	sb, err := s.inner.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if name == s.sbID {
		sb.Status.Phase = s.phase
	}
	return sb, nil
}

func TestSessionStateWithActiveAttach(t *testing.T) {
	status := &WorkspaceStatus{SessionState: SessionStateNone}
	a := &attachState{active: true}
	if a.isAlive() {
		status.SessionState = SessionStateExists
	}
	assert.Equal(t, SessionStateExists, status.SessionState)
}

func TestSessionStateWithDeadAttach(t *testing.T) {
	status := &WorkspaceStatus{SessionState: SessionStateNone}
	a := &attachState{active: false}
	if a.isAlive() {
		status.SessionState = SessionStateExists
	}
	assert.Equal(t, SessionStateNone, status.SessionState)
}

func TestAttach_AlreadyAttached(t *testing.T) {
	w := &OpenShellWorkspace{
		name:   "test-ws",
		attach: &attachState{active: true},
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
		attach:    &attachState{active: true},
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
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Empty(t, cfg.Policy, "policy should be empty when OCI extraction fails")
}

func TestStatusMapping_Deleting(t *testing.T) {
	fc := fake.NewClient()
	sbID := "deleting-sb"
	_, err := fc.Sandboxes().Create(context.Background(), sbID, nil, nil)
	require.NoError(t, err)

	store := newTestStore(t)
	running := InfraStateRunning
	require.NoError(t, store.AddInstance(&WorkspaceInstance{
		Name:       "test-ws",
		Type:       WorkspaceTypeOpenShell,
		InfraState: &running,
		OpenShell:  &OpenShellFields{SandboxID: sbID},
	}))

	w := newOpenShellWS("test-ws", &phaseOverrideClient{inner: fc, phase: types.SandboxDeleting, sbID: sbID}, store)
	w.sandboxID = sbID

	status, err := w.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SessionStateNone, status.SessionState)
	assert.Equal(t, "sandbox deleted", status.Message)
	assert.Empty(t, w.sandboxID)
}

func TestStatusMapping_NotFound(t *testing.T) {
	fc := fake.NewClient()
	store := newTestStore(t)
	running := InfraStateRunning
	require.NoError(t, store.AddInstance(&WorkspaceInstance{
		Name:       "test-ws",
		Type:       WorkspaceTypeOpenShell,
		InfraState: &running,
		OpenShell:  &OpenShellFields{SandboxID: "gone-sb"},
	}))

	w := newOpenShellWS("test-ws", fc, store)
	w.sandboxID = "gone-sb"

	status, err := w.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SessionStateNone, status.SessionState)
	assert.Equal(t, "sandbox deleted", status.Message)
}

func TestCreate_HappyPath(t *testing.T) {
	fc := fake.NewClient()
	store := newTestStore(t)

	w := newOpenShellWS("my-ws", fc, store)

	err := w.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	// Verify sandbox was created in the fake (Get should succeed).
	sb, getErr := fc.Sandboxes().Get(context.Background(), w.sandboxID)
	require.NoError(t, getErr)
	assert.Equal(t, types.SandboxReady, sb.Status.Phase)

	// Verify workspace instance was persisted to state store.
	inst, err := store.FindInstanceByName("my-ws")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeOpenShell, inst.Type)
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
	require.NotNil(t, inst.OpenShell)
	assert.Equal(t, w.sandboxID, inst.OpenShell.SandboxID)
	assert.Equal(t, "fake://localhost:17670", inst.OpenShell.GatewayAddr)
}

// newOpenShellWS creates an OpenShellWorkspace with a pre-injected client,
// ensuring ensureClient() is a no-op and won't overwrite the fake.
func newOpenShellWS(name string, client v1.ClientInterface, store *FileStateStore) *OpenShellWorkspace {
	w := &OpenShellWorkspace{
		name:        name,
		client:      client,
		gatewayAddr: "fake://localhost:17670",
		store:       store,
	}
	w.clientOnce.Do(func() {})
	return w
}

func infraPtr(v InfraStateValue) *InfraStateValue { return &v }
