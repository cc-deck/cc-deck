package openshell

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"github.com/rhuss/openshell-sdk-go/openshell/v1/fake"
	"github.com/rhuss/openshell-sdk-go/openshell/v1/types"
)

func TestResolveProfiles_SingleAgent(t *testing.T) {
	profiles := ResolveProfiles([]string{"claude"}, nil, nil)
	assert.Contains(t, profiles, "anthropic")
	assert.Contains(t, profiles, "claude-agent")
	assert.Contains(t, profiles, "github")
	assert.Contains(t, profiles, "gitlab")
}

func TestResolveProfiles_MultipleAgents(t *testing.T) {
	profiles := ResolveProfiles([]string{"claude", "opencode"}, nil, nil)
	assert.Contains(t, profiles, "anthropic")
	assert.Contains(t, profiles, "claude-agent")
	assert.Contains(t, profiles, "openai")
	assert.Contains(t, profiles, "opencode-agent")
	assert.Contains(t, profiles, "github")
	assert.Contains(t, profiles, "gitlab")
}

func TestResolveProfiles_ToolMapping(t *testing.T) {
	profiles := ResolveProfiles(nil, []string{"python", "go", "node"}, nil)
	assert.Contains(t, profiles, "python")
	assert.Contains(t, profiles, "golang")
	assert.Contains(t, profiles, "nodejs")
}

func TestResolveProfiles_ToolAliases(t *testing.T) {
	profiles1 := ResolveProfiles(nil, []string{"node"}, nil)
	profiles2 := ResolveProfiles(nil, []string{"npm"}, nil)
	assert.Contains(t, profiles1, "nodejs")
	assert.Contains(t, profiles2, "nodejs")

	profiles3 := ResolveProfiles(nil, []string{"cargo"}, nil)
	assert.Contains(t, profiles3, "rust")
}

func TestResolveProfiles_Deduplication(t *testing.T) {
	profiles := ResolveProfiles([]string{"claude", "opencode"}, nil, nil)
	count := 0
	for _, p := range profiles {
		if p == "openai" {
			count++
		}
	}
	assert.Equal(t, 1, count, "openai should appear once despite opencode mapping to it")
}

func TestResolveProfiles_AlwaysIncluded(t *testing.T) {
	profiles := ResolveProfiles(nil, nil, nil)
	assert.Contains(t, profiles, "github")
	assert.Contains(t, profiles, "gitlab")
	assert.Equal(t, 2, len(profiles), "only always-included profiles when inputs are empty")
}

func TestResolveProfiles_Credentials(t *testing.T) {
	profiles := ResolveProfiles(nil, nil, []string{"vertex"})
	assert.Contains(t, profiles, "vertexai")
}

func TestResolveProfiles_Registry(t *testing.T) {
	profiles := ResolveProfiles(nil, nil, []string{"quay"})
	assert.Contains(t, profiles, "quay")
}

func TestResolveProfiles_Sorted(t *testing.T) {
	profiles := ResolveProfiles([]string{"claude"}, []string{"python"}, nil)
	for i := 1; i < len(profiles); i++ {
		assert.True(t, profiles[i-1] <= profiles[i], "profiles should be sorted: %s > %s", profiles[i-1], profiles[i])
	}
}

func TestResolveProfiles_UnknownAgent(t *testing.T) {
	profiles := ResolveProfiles([]string{"unknown-agent"}, nil, nil)
	assert.Contains(t, profiles, "github")
	assert.Contains(t, profiles, "gitlab")
	assert.Equal(t, 2, len(profiles), "unknown agent should only have always-included profiles")
}

func TestResolveProfiles_CodexAgent(t *testing.T) {
	profiles := ResolveProfiles([]string{"codex"}, nil, nil)
	assert.Contains(t, profiles, "openai")
	assert.Contains(t, profiles, "codex-agent")
}

func TestResolveProfiles_FullCoverage(t *testing.T) {
	profiles := ResolveProfiles(
		[]string{"claude", "opencode", "codex"},
		[]string{"python", "node", "go", "rust", "docker"},
		[]string{"vertex", "quay"},
	)
	expected := []string{
		"anthropic", "claude-agent",
		"openai", "opencode-agent", "codex-agent",
		"python", "nodejs", "golang", "rust", "docker",
		"vertexai", "quay",
		"github", "gitlab",
	}
	for _, e := range expected {
		assert.Contains(t, profiles, e, "missing profile: %s", e)
	}
	assert.Equal(t, len(expected), len(profiles), "should contain exactly the expected profiles, got extra: %v", profiles)
}

func TestLookupToolProfile(t *testing.T) {
	assert.Equal(t, "python", LookupToolProfile("python"))
	assert.Equal(t, "nodejs", LookupToolProfile("node"))
	assert.Equal(t, "golang", LookupToolProfile("go"))
	assert.Equal(t, "", LookupToolProfile("unknown"))
}

func TestLookupCredentialProfile_APIWithClaude(t *testing.T) {
	assert.Equal(t, "anthropic", LookupCredentialProfile("api", "claude"))
}

func TestLookupCredentialProfile_APIWithOpenCode(t *testing.T) {
	assert.Equal(t, "openai", LookupCredentialProfile("api", "opencode"))
}

func TestLookupCredentialProfile_APIWithUnknownAgent(t *testing.T) {
	assert.Equal(t, "anthropic", LookupCredentialProfile("api", "unknown"))
}

func TestLookupCredentialProfile_Vertex(t *testing.T) {
	assert.Equal(t, "vertexai", LookupCredentialProfile("vertex", "claude"))
}

func TestLookupCredentialProfile_Registry(t *testing.T) {
	assert.Equal(t, "quay", LookupCredentialProfile("quay", "claude"))
}

func TestLookupCredentialProfile_Unknown(t *testing.T) {
	assert.Equal(t, "", LookupCredentialProfile("bedrock", "claude"))
}

func TestSanitizeWorkspaceName_Simple(t *testing.T) {
	assert.Equal(t, "my-workspace", SanitizeWorkspaceName("my-workspace"))
}

func TestSanitizeWorkspaceName_Uppercase(t *testing.T) {
	assert.Equal(t, "my-workspace", SanitizeWorkspaceName("My-Workspace"))
}

func TestSanitizeWorkspaceName_SpecialChars(t *testing.T) {
	assert.Equal(t, "my-cool-ws", SanitizeWorkspaceName("my_cool.ws!"))
}

func TestSanitizeWorkspaceName_Truncation(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz"
	result := SanitizeWorkspaceName(long)
	assert.LessOrEqual(t, len(result), 50)
}

func TestSanitizeWorkspaceName_Empty(t *testing.T) {
	assert.Equal(t, "ws", SanitizeWorkspaceName(""))
}

func TestSanitizeWorkspaceName_AllInvalid(t *testing.T) {
	assert.Equal(t, "ws", SanitizeWorkspaceName("!!!"))
}

func TestSanitizeWorkspaceName_MultiDashes(t *testing.T) {
	assert.Equal(t, "a-b", SanitizeWorkspaceName("a---b"))
}

// profileOverrideClient wraps a fake client but overrides the Providers()
// method to use a custom profile client that supports Get and Import.
type profileOverrideClient struct {
	inner    v1.ClientInterface
	profiles v1.ProfileInterface
}

func (c *profileOverrideClient) Sandboxes() v1.SandboxInterface  { return c.inner.Sandboxes() }
func (c *profileOverrideClient) Providers() v1.ProviderInterface {
	return &profileOverrideProviderClient{inner: c.inner.Providers(), profiles: c.profiles}
}
func (c *profileOverrideClient) Services() v1.ServiceInterface { return c.inner.Services() }
func (c *profileOverrideClient) Exec() v1.ExecInterface        { return c.inner.Exec() }
func (c *profileOverrideClient) Files() v1.FileInterface       { return c.inner.Files() }
func (c *profileOverrideClient) Health() v1.HealthInterface    { return c.inner.Health() }
func (c *profileOverrideClient) SSH() v1.SSHInterface          { return c.inner.SSH() }
func (c *profileOverrideClient) TCP() v1.TCPInterface          { return c.inner.TCP() }
func (c *profileOverrideClient) Config() v1.ConfigInterface    { return c.inner.Config() }
func (c *profileOverrideClient) Policy() v1.PolicyInterface    { return c.inner.Policy() }
func (c *profileOverrideClient) Close() error                  { return c.inner.Close() }

type profileOverrideProviderClient struct {
	inner    v1.ProviderInterface
	profiles v1.ProfileInterface
}

func (c *profileOverrideProviderClient) Create(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Create(ctx, p)
}
func (c *profileOverrideProviderClient) Get(ctx context.Context, name string) (*types.Provider, error) {
	return c.inner.Get(ctx, name)
}
func (c *profileOverrideProviderClient) List(ctx context.Context, opts ...v1.ListOptions) ([]*types.Provider, error) {
	return c.inner.List(ctx, opts...)
}
func (c *profileOverrideProviderClient) Update(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Update(ctx, p)
}
func (c *profileOverrideProviderClient) Delete(ctx context.Context, name string) error {
	return c.inner.Delete(ctx, name)
}
func (c *profileOverrideProviderClient) Ensure(ctx context.Context, p *types.Provider) (*types.Provider, error) {
	return c.inner.Ensure(ctx, p)
}
func (c *profileOverrideProviderClient) Profiles() v1.ProfileInterface { return c.profiles }
func (c *profileOverrideProviderClient) Refresh() v1.RefreshInterface  { return c.inner.Refresh() }

// testProfileClient implements ProfileInterface with an in-memory store.
type testProfileClient struct {
	profiles map[string]*types.ProviderProfile
	imported []*types.ImportResult
}

func newTestProfileClient(existing ...string) *testProfileClient {
	c := &testProfileClient{profiles: make(map[string]*types.ProviderProfile)}
	for _, id := range existing {
		c.profiles[id] = &types.ProviderProfile{ID: id, DisplayName: id}
	}
	return c
}

func (c *testProfileClient) List(_ context.Context, _ ...v1.ListOptions) ([]*types.ProviderProfile, error) {
	var result []*types.ProviderProfile
	for _, p := range c.profiles {
		result = append(result, p)
	}
	return result, nil
}

func (c *testProfileClient) Get(_ context.Context, id string) (*types.ProviderProfile, error) {
	p, ok := c.profiles[id]
	if !ok {
		return nil, &types.StatusError{Code: types.ErrorNotFound, Message: "profile not found: " + id}
	}
	return p, nil
}

func (c *testProfileClient) Import(_ context.Context, items []types.ProfileImportItem) (*types.ImportResult, error) {
	result := &types.ImportResult{Imported: true}
	for _, item := range items {
		p := item.Profile
		c.profiles[p.ID] = &p
		result.Profiles = append(result.Profiles, p)
	}
	c.imported = append(c.imported, result)
	return result, nil
}

func (c *testProfileClient) Update(_ context.Context, _ string, _ uint64, _ types.ProfileImportItem) (*types.UpdateResult, error) {
	return &types.UpdateResult{Updated: true}, nil
}

func (c *testProfileClient) Lint(_ context.Context, _ []types.ProfileImportItem) (*types.LintResult, error) {
	return &types.LintResult{Valid: true}, nil
}

func (c *testProfileClient) Delete(_ context.Context, id string) (bool, error) {
	_, ok := c.profiles[id]
	if ok {
		delete(c.profiles, id)
	}
	return ok, nil
}

// errorProfileClient returns a fixed error from Get (simulates transient failures).
type errorProfileClient struct {
	err error
}

func (c *errorProfileClient) List(_ context.Context, _ ...v1.ListOptions) ([]*types.ProviderProfile, error) {
	return nil, c.err
}
func (c *errorProfileClient) Get(_ context.Context, _ string) (*types.ProviderProfile, error) {
	return nil, c.err
}
func (c *errorProfileClient) Import(_ context.Context, _ []types.ProfileImportItem) (*types.ImportResult, error) {
	return nil, c.err
}
func (c *errorProfileClient) Update(_ context.Context, _ string, _ uint64, _ types.ProfileImportItem) (*types.UpdateResult, error) {
	return nil, c.err
}
func (c *errorProfileClient) Lint(_ context.Context, _ []types.ProfileImportItem) (*types.LintResult, error) {
	return nil, c.err
}
func (c *errorProfileClient) Delete(_ context.Context, _ string) (bool, error) {
	return false, c.err
}

func TestVerifyProfiles_AllFound(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("anthropic", "claude-agent", "python")
	client := &profileOverrideClient{inner: fc, profiles: pc}

	verified, missing, err := VerifyProfiles(context.Background(), client, []string{"anthropic", "claude-agent", "python"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"anthropic", "claude-agent", "python"}, verified)
	assert.Empty(t, missing)
}

func TestVerifyProfiles_SomeMissing(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("anthropic")
	client := &profileOverrideClient{inner: fc, profiles: pc}

	verified, missing, err := VerifyProfiles(context.Background(), client, []string{"anthropic", "unknown-profile"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"anthropic"}, verified)
	assert.Equal(t, []string{"unknown-profile"}, missing)
}

func TestVerifyProfiles_AllMissing(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient()
	client := &profileOverrideClient{inner: fc, profiles: pc}

	verified, missing, err := VerifyProfiles(context.Background(), client, []string{"no-such", "also-missing"})
	assert.NoError(t, err)
	assert.Empty(t, verified)
	assert.Equal(t, []string{"no-such", "also-missing"}, missing)
}

func TestVerifyProfiles_EmptyInput(t *testing.T) {
	fc := fake.NewClient()
	pc := newTestProfileClient("anthropic")
	client := &profileOverrideClient{inner: fc, profiles: pc}

	verified, missing, err := VerifyProfiles(context.Background(), client, nil)
	assert.NoError(t, err)
	assert.Empty(t, verified)
	assert.Empty(t, missing)
}

func TestVerifyProfiles_TransientError(t *testing.T) {
	fc := fake.NewClient()
	pc := &errorProfileClient{err: fmt.Errorf("connection refused")}
	client := &profileOverrideClient{inner: fc, profiles: pc}

	_, _, err := VerifyProfiles(context.Background(), client, []string{"anthropic"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verifying profile")
	assert.Contains(t, err.Error(), "connection refused")
}
