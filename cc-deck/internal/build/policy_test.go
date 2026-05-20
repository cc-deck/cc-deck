package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	assert.Equal(t, 1, p.Version)

	require.NotNil(t, p.FilesystemPolicy)
	assert.True(t, p.FilesystemPolicy.IncludeWorkdir)
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/usr")
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/lib")
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/proc")
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/etc")
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/var/log")
	assert.Contains(t, p.FilesystemPolicy.ReadWrite, "/sandbox")
	assert.Contains(t, p.FilesystemPolicy.ReadWrite, "/tmp")

	require.NotNil(t, p.Landlock)
	assert.Equal(t, "best_effort", p.Landlock.Compatibility)

	require.NotNil(t, p.Process)
	assert.Equal(t, "sandbox", p.Process.RunAsUser)
	assert.Equal(t, "sandbox", p.Process.RunAsGroup)

	assert.Contains(t, p.NetworkPolicies, "claude_code")
	assert.Contains(t, p.NetworkPolicies, "github")
}

func TestGeneratePolicy_EmptyDomains(t *testing.T) {
	m := &Manifest{Version: 3}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	assert.Equal(t, 1, p.Version)
	assert.Contains(t, p.NetworkPolicies, "claude_code")
	require.NotNil(t, p.FilesystemPolicy)
	assert.True(t, p.FilesystemPolicy.IncludeWorkdir)
	assert.Contains(t, p.FilesystemPolicy.ReadOnly, "/usr")
	require.NotNil(t, p.Landlock)
	assert.Equal(t, "best_effort", p.Landlock.Compatibility)
	require.NotNil(t, p.Process)
	assert.Equal(t, "sandbox", p.Process.RunAsUser)
}

func TestGeneratePolicy_WithDomains(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"api.anthropic.com", "github.com"},
		},
	}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	np, ok := p.NetworkPolicies["api_anthropic_com"]
	require.True(t, ok)
	assert.Equal(t, "api.anthropic.com", np.Name)
	require.Len(t, np.Endpoints, 1)
	assert.Equal(t, "api.anthropic.com", np.Endpoints[0].Host)
	assert.Equal(t, 443, np.Endpoints[0].Port)

	np2, ok := p.NetworkPolicies["github_com"]
	require.True(t, ok)
	assert.Equal(t, "github.com", np2.Name)
}

func TestMarshalPolicy(t *testing.T) {
	p := DefaultPolicy()
	data, err := MarshalPolicy(p)
	require.NoError(t, err)

	var parsed PolicyFile
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	assert.Equal(t, 1, parsed.Version)
	require.NotNil(t, parsed.FilesystemPolicy)
	assert.True(t, parsed.FilesystemPolicy.IncludeWorkdir)
	assert.Equal(t, p.FilesystemPolicy.ReadOnly, parsed.FilesystemPolicy.ReadOnly)
	assert.Equal(t, p.FilesystemPolicy.ReadWrite, parsed.FilesystemPolicy.ReadWrite)
	require.NotNil(t, parsed.Landlock)
	assert.Equal(t, "best_effort", parsed.Landlock.Compatibility)
	require.NotNil(t, parsed.Process)
	assert.Equal(t, "sandbox", parsed.Process.RunAsUser)
	assert.Equal(t, "sandbox", parsed.Process.RunAsGroup)
}

func TestMergePolicy_NilOverrides(t *testing.T) {
	base := DefaultPolicy()
	result := MergePolicy(base, nil)
	assert.Equal(t, base, result)
}

func TestMergePolicy_OverrideFilesystemPolicy(t *testing.T) {
	base := DefaultPolicy()
	overrides := &OpenShellPolicy{
		FilesystemPolicy: &FilesystemPolicy{
			IncludeWorkdir: false,
			ReadOnly:       []string{"/custom"},
			ReadWrite:      []string{"/data"},
		},
	}

	result := MergePolicy(base, overrides)
	assert.False(t, result.FilesystemPolicy.IncludeWorkdir)
	assert.Equal(t, []string{"/custom"}, result.FilesystemPolicy.ReadOnly)
	assert.Equal(t, []string{"/data"}, result.FilesystemPolicy.ReadWrite)
	assert.Equal(t, "best_effort", result.Landlock.Compatibility)
}

func TestMergePolicy_OverrideProcess(t *testing.T) {
	base := DefaultPolicy()
	overrides := &OpenShellPolicy{
		Process: &ProcessConfig{
			RunAsUser:  "root",
			RunAsGroup: "root",
		},
	}

	result := MergePolicy(base, overrides)
	assert.Equal(t, "root", result.Process.RunAsUser)
	assert.Equal(t, "root", result.Process.RunAsGroup)
}

func TestMergePolicy_NetworkOverrideReplaces(t *testing.T) {
	base := DefaultPolicy()
	base.NetworkPolicies["github_com"] = NetworkPolicy{
		Name:      "github.com",
		Endpoints: []PolicyEndpoint{{Host: "github.com", Port: 443}},
	}
	base.NetworkPolicies["api_anthropic_com"] = NetworkPolicy{
		Name:      "api.anthropic.com",
		Endpoints: []PolicyEndpoint{{Host: "api.anthropic.com", Port: 443}},
	}

	overrides := &OpenShellPolicy{
		NetworkPolicies: map[string]NetworkPolicy{
			"git_hosting": {
				Name:      "git-hosting",
				Endpoints: []PolicyEndpoint{{Host: "github.com", Port: 443}},
				Binaries:  []PolicyBinary{{Path: "/usr/bin/git"}},
			},
		},
	}

	result := MergePolicy(base, overrides)

	_, hasOldGithub := result.NetworkPolicies["github_com"]
	assert.False(t, hasOldGithub, "auto-generated github_com should be replaced")

	np, hasOverride := result.NetworkPolicies["git_hosting"]
	assert.True(t, hasOverride)
	assert.Equal(t, "git-hosting", np.Name)
	require.Len(t, np.Binaries, 1)
	assert.Equal(t, "/usr/bin/git", np.Binaries[0].Path)

	_, hasAnthropic := result.NetworkPolicies["api_anthropic_com"]
	assert.True(t, hasAnthropic, "non-overridden entry should be preserved")
}

func TestMergePolicy_NetworkAdditive(t *testing.T) {
	base := DefaultPolicy()

	overrides := &OpenShellPolicy{
		NetworkPolicies: map[string]NetworkPolicy{
			"custom": {
				Name:      "custom-service",
				Endpoints: []PolicyEndpoint{{Host: "custom.example.com", Port: 8443}},
			},
		},
	}

	result := MergePolicy(base, overrides)

	np, ok := result.NetworkPolicies["custom"]
	assert.True(t, ok, "additive entry should be present")
	assert.Equal(t, "custom-service", np.Name)
}

func TestMergePolicy_EmptyOverrides(t *testing.T) {
	base := DefaultPolicy()
	base.NetworkPolicies["test"] = NetworkPolicy{
		Name:      "test",
		Endpoints: []PolicyEndpoint{{Host: "test.com", Port: 443}},
	}

	overrides := &OpenShellPolicy{}
	result := MergePolicy(base, overrides)

	assert.Equal(t, base.FilesystemPolicy, result.FilesystemPolicy)
	assert.Equal(t, base.Landlock, result.Landlock)
	assert.Equal(t, base.Process, result.Process)
	_, ok := result.NetworkPolicies["test"]
	assert.True(t, ok)
}

func TestGeneratePolicy_VertexCredentialAddsGCPEndpoints(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Credentials: []CredentialEntry{
			{Type: "vertex"},
		},
	}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	vertex, ok := p.NetworkPolicies["vertex_ai"]
	require.True(t, ok, "expected vertex_ai policy")
	assert.True(t, len(vertex.Endpoints) > 2, "expected multiple region endpoints")
	assert.Equal(t, "global-aiplatform.googleapis.com", vertex.Endpoints[0].Host)
	assert.Equal(t, "oauth2.googleapis.com", vertex.Endpoints[len(vertex.Endpoints)-1].Host)
}

func TestGeneratePolicy_ClaudeVertexCredentialAddsGCPEndpoints(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Credentials: []CredentialEntry{
			{Type: "claude-vertex"},
		},
	}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	vertex, ok := p.NetworkPolicies["vertex_ai"]
	require.True(t, ok, "expected vertex_ai policy")
	assert.True(t, len(vertex.Endpoints) > 2, "expected multiple region endpoints")
	assert.Equal(t, "global-aiplatform.googleapis.com", vertex.Endpoints[0].Host)
	assert.Equal(t, "oauth2.googleapis.com", vertex.Endpoints[len(vertex.Endpoints)-1].Host)
}

func TestGeneratePolicy_GenericCredentialAddsCustomEndpoints(t *testing.T) {
	m := &Manifest{
		Version: 3,
		Credentials: []CredentialEntry{
			{
				Type:    "generic",
				EnvVars: []string{"CUSTOM_KEY"},
				Endpoints: []PolicyEndpoint{
					{Host: "api.custom.com", Port: 443},
					{Host: "auth.custom.com", Port: 8443},
				},
			},
		},
	}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	ep1, ok := p.NetworkPolicies["cred_api_custom_com"]
	require.True(t, ok, "expected cred_api_custom_com policy")
	assert.Equal(t, "api.custom.com", ep1.Endpoints[0].Host)
	assert.Equal(t, 443, ep1.Endpoints[0].Port)

	ep2, ok := p.NetworkPolicies["cred_auth_custom_com"]
	require.True(t, ok, "expected cred_auth_custom_com policy")
	assert.Equal(t, "auth.custom.com", ep2.Endpoints[0].Host)
	assert.Equal(t, 8443, ep2.Endpoints[0].Port)
}

func TestGeneratePolicy_NoCredentialsHasDefaults(t *testing.T) {
	m := &Manifest{Version: 3}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	assert.Contains(t, p.NetworkPolicies, "claude_code")
	assert.Contains(t, p.NetworkPolicies, "github")
	assert.Len(t, p.NetworkPolicies, 2)
}

func TestGeneratePolicy_CredentialsAndDomainsCombined(t *testing.T) {
	t.Setenv("CLOUD_ML_REGION", "")

	m := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"api.anthropic.com"},
		},
		Credentials: []CredentialEntry{
			{Type: "vertex"},
		},
	}

	p, err := GeneratePolicy(m)
	require.NoError(t, err)

	// Should have the domain entry.
	_, hasDomain := p.NetworkPolicies["api_anthropic_com"]
	assert.True(t, hasDomain, "expected domain policy entry")

	// Should have vertex entry with region endpoints.
	vertex, hasVertex := p.NetworkPolicies["vertex_ai"]
	assert.True(t, hasVertex, "expected vertex_ai policy entry")
	assert.Equal(t, "global-aiplatform.googleapis.com", vertex.Endpoints[0].Host)
}

func TestSlugify(t *testing.T) {
	assert.Equal(t, "api_anthropic_com", slugify("api.anthropic.com"))
	assert.Equal(t, "github_com", slugify("github.com"))
	assert.Equal(t, "my_domain", slugify("my-domain"))
}
