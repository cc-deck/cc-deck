package ws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/credential"
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
    provider: my-provider
`), 0644)

	defs := NewDefinitionStore(defPath)
	w := &OpenShellWorkspace{name: "test-ws", defs: defs}
	cfg, err := w.resolveSandboxConfig()
	require.NoError(t, err)
	assert.Equal(t, "custom/image:v1", cfg.Image)
	assert.Equal(t, "tmux", cfg.Command)
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

func TestSelectCredentialMode_EmptyAvailable(t *testing.T) {
	_, found, err := selectCredentialMode(nil, "")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSelectCredentialMode_AutoSelect(t *testing.T) {
	available := []credential.AvailableMode{
		{Spec: agent.CredentialSpec{Name: "api"}},
		{Spec: agent.CredentialSpec{Name: "vertex"}},
	}
	spec, found, err := selectCredentialMode(available, "")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "api", spec.Name)
}

func TestSelectCredentialMode_AutoExplicit(t *testing.T) {
	available := []credential.AvailableMode{
		{Spec: agent.CredentialSpec{Name: "api"}},
		{Spec: agent.CredentialSpec{Name: "vertex"}},
	}
	spec, found, err := selectCredentialMode(available, "auto")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "api", spec.Name)
}

func TestSelectCredentialMode_ExplicitMatch(t *testing.T) {
	available := []credential.AvailableMode{
		{Spec: agent.CredentialSpec{Name: "api"}},
		{Spec: agent.CredentialSpec{Name: "vertex"}},
	}
	spec, found, err := selectCredentialMode(available, "vertex")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "vertex", spec.Name)
}

func TestSelectCredentialMode_ExplicitNoMatch(t *testing.T) {
	available := []credential.AvailableMode{
		{Spec: agent.CredentialSpec{Name: "api"}},
	}
	_, _, err := selectCredentialMode(available, "vertex")
	assert.Error(t, err)
}

func TestSelectCredentialMode_NoneAuth(t *testing.T) {
	available := []credential.AvailableMode{
		{Spec: agent.CredentialSpec{Name: "api"}},
	}
	_, found, err := selectCredentialMode(available, "none")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMapToOpenShellProvider_API(t *testing.T) {
	spec := agent.CredentialSpec{Name: "api"}
	resolved := credential.ResolvedCredentials{
		EnvVars: map[string]string{"ANTHROPIC_API_KEY": "sk-test"},
	}
	name, pType, creds := mapToOpenShellProvider("ws1", "claude", spec, resolved)
	assert.Equal(t, "cc-deck-ws1-api", name)
	assert.Equal(t, "anthropic", pType)
	assert.Equal(t, "sk-test", creds["ANTHROPIC_API_KEY"])
}

func TestMapToOpenShellProvider_APIOpenCode(t *testing.T) {
	spec := agent.CredentialSpec{Name: "api"}
	resolved := credential.ResolvedCredentials{
		EnvVars: map[string]string{"OPENAI_API_KEY": "sk-test"},
	}
	_, pType, _ := mapToOpenShellProvider("ws1", "opencode", spec, resolved)
	assert.Equal(t, "openai", pType)
}

func TestMapToOpenShellProvider_Vertex(t *testing.T) {
	spec := agent.CredentialSpec{Name: "vertex"}
	resolved := credential.ResolvedCredentials{
		EnvVars: map[string]string{
			"ANTHROPIC_VERTEX_PROJECT_ID": "my-project",
			"CLOUD_ML_REGION":            "us-east5",
		},
	}
	name, pType, creds := mapToOpenShellProvider("ws1", "claude", spec, resolved)
	assert.Equal(t, "cc-deck-ws1-vertex", name)
	assert.Equal(t, "vertexai", pType)
	assert.Equal(t, "my-project", creds["project_id"])
	assert.Equal(t, "us-east5", creds["region"])
}

func TestMapToOpenShellProvider_VertexDefaultRegion(t *testing.T) {
	spec := agent.CredentialSpec{Name: "vertex"}
	resolved := credential.ResolvedCredentials{
		EnvVars: map[string]string{
			"ANTHROPIC_VERTEX_PROJECT_ID": "my-project",
		},
	}
	_, _, creds := mapToOpenShellProvider("ws1", "claude", spec, resolved)
	assert.Equal(t, "global", creds["region"])
}

func TestMapToOpenShellProvider_Bedrock(t *testing.T) {
	spec := agent.CredentialSpec{Name: "bedrock"}
	resolved := credential.ResolvedCredentials{
		EnvVars: map[string]string{"AWS_REGION": "us-east-1"},
	}
	_, pType, _ := mapToOpenShellProvider("ws1", "claude", spec, resolved)
	assert.Empty(t, pType, "bedrock has no OpenShell provider")
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

func TestCreate_ProfileBased_NoPolicy(t *testing.T) {
	fc := fake.NewClient()
	store := newTestStore(t)

	w := newOpenShellWS("profile-ws", fc, store)
	err := w.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	sb, getErr := fc.Sandboxes().Get(context.Background(), w.sandboxID)
	require.NoError(t, getErr)
	assert.Equal(t, types.SandboxReady, sb.Status.Phase)
	assert.Nil(t, sb.Spec.Policy, "SandboxSpec.Policy must be nil for profile-based path (FR-001)")
}

func TestCreate_BackwardCompat_EmptyAgents(t *testing.T) {
	fc := fake.NewClient()
	store := newTestStore(t)

	w := newOpenShellWS("compat-ws", fc, store)
	err := w.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	inst, findErr := store.FindInstanceByName("compat-ws")
	require.NoError(t, findErr)
	assert.Equal(t, WorkspaceTypeOpenShell, inst.Type)
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
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

func TestExtractProfileManifest_NoImage(t *testing.T) {
	pm, err := extractProfileManifest("nonexistent-registry.invalid/no-image:v999")
	require.NoError(t, err)
	assert.Nil(t, pm, "should return nil for missing image")
}

func TestExtractProfileManifest_EmptyImage(t *testing.T) {
	pm, err := extractProfileManifest("")
	require.NoError(t, err)
	assert.Nil(t, pm)
}

func TestCreateProfileProviders_Empty(t *testing.T) {
	fc := fake.NewClient()
	providers, err := createProfileProviders(context.Background(), fc, "test-ws", nil)
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestCreateProfileProviders_WithProfiles(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("anthropic", "python", "github")
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	providers, err := createProfileProviders(context.Background(), client, "test-ws", []string{"anthropic", "python", "github"})
	require.NoError(t, err)
	assert.Len(t, providers, 3)
	assert.Contains(t, providers, "cc-deck-test-ws-anthropic")
	assert.Contains(t, providers, "cc-deck-test-ws-python")
	assert.Contains(t, providers, "cc-deck-test-ws-github")

	for _, name := range providers {
		p, getErr := client.Providers().Get(context.Background(), name)
		require.NoError(t, getErr, "provider %s should exist in fake after Ensure()", name)
		assert.NotEmpty(t, p.Type, "provider %s should have a Type referencing a profile ID", name)
	}
}

func TestCreateProfileProviders_MissingProfilesSkipped(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("anthropic")
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	providers, err := createProfileProviders(context.Background(), client, "ws1", []string{"anthropic", "missing-profile"})
	require.NoError(t, err)
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, "cc-deck-ws1-anthropic")
}

func TestCreateProfileProviders_NameSanitization(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("python")
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	providers, err := createProfileProviders(context.Background(), client, "My_Workspace!!", []string{"python"})
	require.NoError(t, err)
	assert.Len(t, providers, 1)
	assert.Equal(t, "cc-deck-my-workspace-python", providers[0])
}

// profileOverrideClientWS wraps a fake client but overrides the Providers()
// method to use a custom profile client. This is the ws package equivalent of
// the openshell package's profileOverrideClient.
type profileOverrideClientWS struct {
	inner    v1.ClientInterface
	profiles v1.ProfileInterface
}

func (c *profileOverrideClientWS) Sandboxes() v1.SandboxInterface  { return c.inner.Sandboxes() }
func (c *profileOverrideClientWS) Providers() v1.ProviderInterface {
	return &profileOverrideProviderWS{inner: c.inner.Providers(), profiles: c.profiles}
}
func (c *profileOverrideClientWS) Services() v1.ServiceInterface { return c.inner.Services() }
func (c *profileOverrideClientWS) Exec() v1.ExecInterface        { return c.inner.Exec() }
func (c *profileOverrideClientWS) Files() v1.FileInterface       { return c.inner.Files() }
func (c *profileOverrideClientWS) Health() v1.HealthInterface    { return c.inner.Health() }
func (c *profileOverrideClientWS) SSH() v1.SSHInterface          { return c.inner.SSH() }
func (c *profileOverrideClientWS) TCP() v1.TCPInterface          { return c.inner.TCP() }
func (c *profileOverrideClientWS) Config() v1.ConfigInterface    { return c.inner.Config() }
func (c *profileOverrideClientWS) Policy() v1.PolicyInterface    { return c.inner.Policy() }
func (c *profileOverrideClientWS) Close() error                  { return c.inner.Close() }

type profileOverrideProviderWS struct {
	inner    v1.ProviderInterface
	profiles v1.ProfileInterface
}

func (c *profileOverrideProviderWS) Create(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Create(ctx, p)
}
func (c *profileOverrideProviderWS) Get(ctx context.Context, name string) (*types.Provider, error) {
	return c.inner.Get(ctx, name)
}
func (c *profileOverrideProviderWS) List(ctx context.Context, opts ...v1.ListOptions) ([]*types.Provider, error) {
	return c.inner.List(ctx, opts...)
}
func (c *profileOverrideProviderWS) Update(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Update(ctx, p)
}
func (c *profileOverrideProviderWS) Delete(ctx context.Context, name string) error {
	return c.inner.Delete(ctx, name)
}
func (c *profileOverrideProviderWS) Ensure(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Ensure(ctx, p)
}
func (c *profileOverrideProviderWS) Profiles() v1.ProfileInterface { return c.profiles }
func (c *profileOverrideProviderWS) Refresh() v1.RefreshInterface  { return c.inner.Refresh() }

// testProfileClientWS implements ProfileInterface with an in-memory store.
type testProfileClientWS struct {
	profiles map[string]*types.ProviderProfile
}

func newTestProfileClient(existing ...string) *testProfileClientWS {
	c := &testProfileClientWS{profiles: make(map[string]*types.ProviderProfile)}
	for _, id := range existing {
		c.profiles[id] = &types.ProviderProfile{ID: id, DisplayName: id}
	}
	return c
}

func (c *testProfileClientWS) List(_ context.Context, _ ...v1.ListOptions) ([]*types.ProviderProfile, error) {
	var result []*types.ProviderProfile
	for _, p := range c.profiles {
		result = append(result, p)
	}
	return result, nil
}

func (c *testProfileClientWS) Get(_ context.Context, id string) (*types.ProviderProfile, error) {
	p, ok := c.profiles[id]
	if !ok {
		return nil, &types.StatusError{Code: types.ErrorNotFound, Message: "profile not found: " + id}
	}
	return p, nil
}

func (c *testProfileClientWS) Import(_ context.Context, items []types.ProfileImportItem) (*types.ImportResult, error) {
	result := &types.ImportResult{Imported: true}
	for _, item := range items {
		p := item.Profile
		c.profiles[p.ID] = &p
		result.Profiles = append(result.Profiles, p)
	}
	return result, nil
}

func (c *testProfileClientWS) Update(_ context.Context, _ string, _ uint64, _ types.ProfileImportItem) (*types.UpdateResult, error) {
	return &types.UpdateResult{Updated: true}, nil
}

func (c *testProfileClientWS) Lint(_ context.Context, _ []types.ProfileImportItem) (*types.LintResult, error) {
	return &types.LintResult{Valid: true}, nil
}

func (c *testProfileClientWS) Delete(_ context.Context, id string) (bool, error) {
	_, ok := c.profiles[id]
	if ok {
		delete(c.profiles, id)
	}
	return ok, nil
}

func TestImportMCPProfile_SingleEndpoint(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	entries := []build.MCPManifestEntry{
		{Name: "my-mcp", Endpoint: "localhost:8080"},
	}
	providerName, err := importMCPProfile(context.Background(), client, "test-ws", entries, nil)
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-test-ws-mcp", providerName)

	imported, ok := pc.profiles["cc-deck-test-ws-mcp"]
	require.True(t, ok, "profile should be imported into fake")
	assert.Len(t, imported.Endpoints, 1)
	assert.Equal(t, "localhost", imported.Endpoints[0].Host)
	assert.Equal(t, uint32(8080), imported.Endpoints[0].Port)

	p, getErr := client.Providers().Get(context.Background(), providerName)
	require.NoError(t, getErr, "provider should be created via Ensure()")
	assert.Equal(t, "cc-deck-test-ws-mcp", p.Type)
}

func TestImportMCPProfile_MultipleEndpoints(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	entries := []build.MCPManifestEntry{
		{Name: "mcp-a", Endpoint: "host-a:9090"},
		{Name: "mcp-b", Endpoint: "host-b:9091"},
	}
	providerName, err := importMCPProfile(context.Background(), client, "ws1", entries, []string{"/usr/bin/claude"})
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-ws1-mcp", providerName)

	imported, ok := pc.profiles["cc-deck-ws1-mcp"]
	require.True(t, ok)
	assert.Len(t, imported.Endpoints, 2)
	assert.Equal(t, "host-a", imported.Endpoints[0].Host)
	assert.Equal(t, uint32(9090), imported.Endpoints[0].Port)
	assert.Equal(t, "host-b", imported.Endpoints[1].Host)
	assert.Equal(t, uint32(9091), imported.Endpoints[1].Port)
	assert.Len(t, imported.Binaries, 1)
	assert.Equal(t, "/usr/bin/claude", imported.Binaries[0].Path)
}

func TestImportMCPProfile_NoEndpoints(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	entries := []build.MCPManifestEntry{
		{Name: "mcp-no-endpoint"},
	}
	providerName, err := importMCPProfile(context.Background(), client, "ws1", entries, nil)
	require.NoError(t, err)
	assert.Empty(t, providerName, "no endpoints means no profile import")
}

func TestImportMCPProfile_InvalidEndpointSkipped(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	entries := []build.MCPManifestEntry{
		{Name: "bad-mcp", Endpoint: "no-port"},
		{Name: "good-mcp", Endpoint: "host:8080"},
	}
	providerName, err := importMCPProfile(context.Background(), client, "ws1", entries, nil)
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-ws1-mcp", providerName)

	imported := pc.profiles["cc-deck-ws1-mcp"]
	assert.Len(t, imported.Endpoints, 1, "invalid endpoint should be skipped")
	assert.Equal(t, "host", imported.Endpoints[0].Host)
}

func TestImportMCPProfile_ImportFailure(t *testing.T) {
	fc := fake.NewClient()
	failPC := &failingProfileClientWS{}
	client := &profileOverrideClientWS{inner: fc, profiles: failPC}

	entries := []build.MCPManifestEntry{
		{Name: "mcp", Endpoint: "host:8080"},
	}
	providerName, err := importMCPProfile(context.Background(), client, "ws1", entries, nil)
	require.NoError(t, err, "import failure should warn, not error")
	assert.Empty(t, providerName, "failed import returns empty provider name")
}

func TestImportCustomDomainsProfile_BasicDomains(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClientWS{inner: fc, profiles: pc}

	domains := []string{"custom.example.com", "api.internal.io"}
	providerName, err := importCustomDomainsProfile(context.Background(), client, "ws1", domains)
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-ws1-custom", providerName)

	imported, ok := pc.profiles["cc-deck-ws1-custom"]
	require.True(t, ok)
	assert.Len(t, imported.Endpoints, 2)
	assert.Equal(t, "custom.example.com", imported.Endpoints[0].Host)
	assert.Equal(t, uint32(443), imported.Endpoints[0].Port)
	assert.Equal(t, "https", imported.Endpoints[0].Protocol)
	assert.Equal(t, "api.internal.io", imported.Endpoints[1].Host)
}

func TestImportCustomDomainsProfile_EmptyDomains(t *testing.T) {
	fc := fake.NewClient()
	providerName, err := importCustomDomainsProfile(context.Background(), fc, "ws1", nil)
	require.NoError(t, err)
	assert.Empty(t, providerName, "empty domains should skip import")
}

func TestImportCustomDomainsProfile_ImportFailure(t *testing.T) {
	fc := fake.NewClient()
	failPC := &failingProfileClientWS{}
	client := &profileOverrideClientWS{inner: fc, profiles: failPC}

	domains := []string{"example.com"}
	providerName, err := importCustomDomainsProfile(context.Background(), client, "ws1", domains)
	require.NoError(t, err, "import failure should warn, not error")
	assert.Empty(t, providerName, "failed import returns empty provider name")
}

func TestParseHostPort_Valid(t *testing.T) {
	host, port, err := parseHostPort("example.com:443")
	require.NoError(t, err)
	assert.Equal(t, "example.com", host)
	assert.Equal(t, 443, port)
}

func TestParseHostPort_InvalidFormat(t *testing.T) {
	_, _, err := parseHostPort("no-colon")
	assert.Error(t, err)
}

func TestParseHostPort_InvalidPort(t *testing.T) {
	_, _, err := parseHostPort("host:abc")
	assert.Error(t, err)
}

func TestParseHostPort_PortOutOfRange(t *testing.T) {
	_, _, err := parseHostPort("host:99999")
	assert.Error(t, err)
}

// failingProfileClientWS is a profile client whose Import always fails.
type failingProfileClientWS struct{}

func (c *failingProfileClientWS) Get(_ context.Context, _ string) (*types.ProviderProfile, error) {
	return nil, fmt.Errorf("not found")
}
func (c *failingProfileClientWS) List(_ context.Context, _ ...v1.ListOptions) ([]*types.ProviderProfile, error) {
	return nil, nil
}
func (c *failingProfileClientWS) Import(_ context.Context, _ []types.ProfileImportItem) (*types.ImportResult, error) {
	return nil, fmt.Errorf("gateway unavailable")
}
func (c *failingProfileClientWS) Update(_ context.Context, _ string, _ uint64, _ types.ProfileImportItem) (*types.UpdateResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *failingProfileClientWS) Lint(_ context.Context, _ []types.ProfileImportItem) (*types.LintResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *failingProfileClientWS) Delete(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func infraPtr(v InfraStateValue) *InfraStateValue { return &v }
