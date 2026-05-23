package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// T024: Determinism test - same manifest produces byte-identical output

func TestAssemblePolicy_Determinism(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}, {Name: "go"}},
		Credentials: []CredentialEntry{
			{Type: "claude-vertex"},
		},
	}

	policy1, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)
	data1, err := MarshalPolicy(policy1)
	require.NoError(t, err)

	policy2, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)
	data2, err := MarshalPolicy(policy2)
	require.NoError(t, err)

	assert.Equal(t, string(data1), string(data2), "same manifest must produce byte-identical output")
}

// T025: Component matching - cargo manifest matches rust.yaml endpoints

func TestAssemblePolicy_CargoMatchesRust(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}

	policy, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	rust, ok := policy.NetworkPolicies["pkg_rust"]
	require.True(t, ok, "cargo in manifest should match rust.yaml component")
	assert.Equal(t, "rust packages", rust.Name)

	var hasCratesIO bool
	for _, ep := range rust.Endpoints {
		if ep.Host == "crates.io" {
			hasCratesIO = true
		}
	}
	assert.True(t, hasCratesIO, "rust component should include crates.io endpoint")
}

// T026: Component matching - claude-vertex credential matches vertex-ai.yaml

func TestAssemblePolicy_ClaudeVertexMatchesVertexAI(t *testing.T) {
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "claude-vertex"}},
	}

	policy, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	vertex, ok := policy.NetworkPolicies["vertex_ai"]
	require.True(t, ok, "claude-vertex credential should match vertex-ai.yaml")
	assert.Equal(t, "Vertex AI", vertex.Name)
	assert.True(t, len(vertex.Endpoints) > 2, "vertex-ai should have many region endpoints")
	assert.Equal(t, "aiplatform.googleapis.com", vertex.Endpoints[0].Host)
}

func TestAssemblePolicy_VertexCredentialMatchesVertexAI(t *testing.T) {
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "vertex"}},
	}

	policy, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	_, ok := policy.NetworkPolicies["vertex_ai"]
	assert.True(t, ok, "vertex credential should also match vertex-ai.yaml")
}

// T027: Always-true components appear with empty manifest

func TestAssemblePolicy_AlwaysTrueComponentsWithEmptyManifest(t *testing.T) {
	manifest := &Manifest{Version: 3}

	policy, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	_, hasClaude := policy.NetworkPolicies["claude_code"]
	assert.True(t, hasClaude, "claude_code (always: true) should appear even with empty manifest")

	_, hasGithub := policy.NetworkPolicies["github"]
	assert.True(t, hasGithub, "github (always: true) should appear even with empty manifest")

	_, hasRust := policy.NetworkPolicies["pkg_rust"]
	assert.False(t, hasRust, "rust should NOT appear with empty manifest (no tool match)")
}

// T028: Updated existing tests using component-based approach

func TestAssemblePolicy_DefaultStructure(t *testing.T) {
	manifest := &Manifest{Version: 3}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

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

func TestAssemblePolicy_RestEndpointsHaveAccess(t *testing.T) {
	manifest := &Manifest{Version: 3}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	for key, np := range p.NetworkPolicies {
		for i, ep := range np.Endpoints {
			if ep.Protocol == "rest" {
				hasAccess := ep.Access != ""
				hasRules := len(ep.Rules) > 0
				assert.True(t, hasAccess || hasRules,
					"%s.endpoints[%d] (%s): protocol=rest requires access or rules (OpenShell 0.0.46+)",
					key, i, ep.Host)
			}
		}
	}
}

func TestAssemblePolicy_MarshalRoundTrip(t *testing.T) {
	manifest := &Manifest{Version: 3}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	data, err := MarshalPolicy(p)
	require.NoError(t, err)

	var parsed PolicyFile
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	assert.Equal(t, 1, parsed.Version)
	require.NotNil(t, parsed.FilesystemPolicy)
	assert.True(t, parsed.FilesystemPolicy.IncludeWorkdir)

	claude := parsed.NetworkPolicies["claude_code"]
	require.NotEmpty(t, claude.Endpoints)
	assert.Equal(t, "rest", claude.Endpoints[0].Protocol)
	assert.Equal(t, "full", claude.Endpoints[0].Access)

	github := parsed.NetworkPolicies["github"]
	for _, ep := range github.Endpoints {
		if ep.Host == "api.github.com" {
			assert.Equal(t, "rest", ep.Protocol)
			assert.Equal(t, "full", ep.Access)
		}
	}
}

func TestAssemblePolicy_WithDomains(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"api.anthropic.com", "github.com"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
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

func TestAssemblePolicy_GenericCredentialAddsCustomEndpoints(t *testing.T) {
	manifest := &Manifest{
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

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
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

func TestAssemblePolicy_CredentialsAndDomainsCombined(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"api.anthropic.com"},
		},
		Credentials: []CredentialEntry{
			{Type: "vertex"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	_, hasDomain := p.NetworkPolicies["api_anthropic_com"]
	assert.True(t, hasDomain, "expected domain policy entry")

	vertex, hasVertex := p.NetworkPolicies["vertex_ai"]
	assert.True(t, hasVertex, "expected vertex_ai policy entry")
	assert.Equal(t, "aiplatform.googleapis.com", vertex.Endpoints[0].Host)
}

func TestAssemblePolicy_ToolsFromSources(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Sources: []SourceEntry{
			{URL: "https://github.com/test/repo", DetectedTools: []string{"go", "python"}},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	_, hasGo := p.NetworkPolicies["pkg_go"]
	assert.True(t, hasGo, "go detected tool should match go.yaml component")

	_, hasPython := p.NetworkPolicies["pkg_python"]
	assert.True(t, hasPython, "python detected tool should match python.yaml component")
}

func TestAssemblePolicy_AllToolsCombined(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Tools: []ToolEntry{
			{Name: "cargo"},
			{Name: "go"},
			{Name: "node"},
			{Name: "python"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	assert.Contains(t, p.NetworkPolicies, "pkg_rust")
	assert.Contains(t, p.NetworkPolicies, "pkg_go")
	assert.Contains(t, p.NetworkPolicies, "pkg_node")
	assert.Contains(t, p.NetworkPolicies, "pkg_python")
	assert.Contains(t, p.NetworkPolicies, "claude_code")
	assert.Contains(t, p.NetworkPolicies, "github")
}

// MergePolicy tests (unchanged behavior)

func TestMergePolicy_NilOverrides(t *testing.T) {
	manifest := &Manifest{Version: 3}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	result := MergePolicy(base, nil)
	assert.Equal(t, base, result)
}

func TestMergePolicy_OverrideFilesystemPolicy(t *testing.T) {
	manifest := &Manifest{Version: 3}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

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
	manifest := &Manifest{Version: 3}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

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
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"github.com", "api.anthropic.com"},
		},
	}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

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
	assert.False(t, hasOldGithub, "auto-generated github_com should be replaced by override with same host")

	np, hasOverride := result.NetworkPolicies["git_hosting"]
	assert.True(t, hasOverride)
	assert.Equal(t, "git-hosting", np.Name)
	require.Len(t, np.Binaries, 1)
	assert.Equal(t, "/usr/bin/git", np.Binaries[0].Path)

	_, hasAnthropic := result.NetworkPolicies["api_anthropic_com"]
	assert.True(t, hasAnthropic, "non-overridden entry should be preserved")
}

func TestMergePolicy_NetworkAdditive(t *testing.T) {
	manifest := &Manifest{Version: 3}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

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
	manifest := &Manifest{Version: 3}
	base, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	overrides := &OpenShellPolicy{}
	result := MergePolicy(base, overrides)

	assert.Equal(t, base.FilesystemPolicy, result.FilesystemPolicy)
	assert.Equal(t, base.Landlock, result.Landlock)
	assert.Equal(t, base.Process, result.Process)
}

// T038: User-local component inclusion test

func TestAssemblePolicy_UserLocalComponentInclusion(t *testing.T) {
	userLocalDir := t.TempDir()
	customComp := `
key: internal_api
name: Internal API
match:
  always: true
endpoints:
  - host: api.internal.corp
    port: 8443
`
	require.NoError(t, os.WriteFile(filepath.Join(userLocalDir, "internal-api.yaml"), []byte(customComp), 0o644))

	manifest := &Manifest{Version: 3}

	policy, err := AssemblePolicyFromDir(manifest, "", userLocalDir)
	require.NoError(t, err)

	api, ok := policy.NetworkPolicies["internal_api"]
	require.True(t, ok, "user-local always-true component should appear in output")
	assert.Equal(t, "Internal API", api.Name)
	assert.Equal(t, "api.internal.corp", api.Endpoints[0].Host)
}

// T039: User-local precedence test

func TestAssemblePolicy_UserLocalOverridesCatalog(t *testing.T) {
	catalogDir := t.TempDir()
	catalogRust := `
key: pkg_rust
name: rust packages (catalog)
match:
  tools:
    - rust
    - cargo
endpoints:
  - host: catalog-crates.io
    port: 443
`
	require.NoError(t, os.WriteFile(filepath.Join(catalogDir, "rust.yaml"), []byte(catalogRust), 0o644))

	userLocalDir := t.TempDir()
	userRust := `
key: pkg_rust
name: rust packages (user-local)
match:
  tools:
    - rust
    - cargo
endpoints:
  - host: user-crates.io
    port: 443
`
	require.NoError(t, os.WriteFile(filepath.Join(userLocalDir, "rust.yaml"), []byte(userRust), 0o644))

	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}

	policy, err := AssemblePolicyFromDir(manifest, catalogDir, userLocalDir)
	require.NoError(t, err)

	rust, ok := policy.NetworkPolicies["pkg_rust"]
	require.True(t, ok)
	assert.Equal(t, "rust packages (user-local)", rust.Name, "user-local should override catalog")
	assert.Equal(t, "user-crates.io", rust.Endpoints[0].Host)
}

// T036: Catalog precedence test

func TestAssemblePolicy_CatalogOverridesEmbedded(t *testing.T) {
	catalogDir := t.TempDir()
	rustOverride := `
key: pkg_rust
name: rust packages (updated)
match:
  tools:
    - rust
    - cargo
endpoints:
  - host: new-crates.io
    port: 443
  - host: new-index.crates.io
    port: 443
`
	require.NoError(t, os.WriteFile(filepath.Join(catalogDir, "rust.yaml"), []byte(rustOverride), 0o644))

	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}

	policy, err := AssemblePolicyFromDir(manifest, catalogDir, "")
	require.NoError(t, err)

	rust, ok := policy.NetworkPolicies["pkg_rust"]
	require.True(t, ok)
	assert.Equal(t, "rust packages (updated)", rust.Name, "catalog version should replace embedded")
	assert.Equal(t, "new-crates.io", rust.Endpoints[0].Host, "catalog endpoints should replace embedded")
}

func TestSlugify(t *testing.T) {
	assert.Equal(t, "api_anthropic_com", slugify("api.anthropic.com"))
	assert.Equal(t, "github_com", slugify("github.com"))
	assert.Equal(t, "my-domain", slugify("my-domain"))
	assert.NotEqual(t, slugify("api.my-service.com"), slugify("api.my_service.com"),
		"slugify must not collapse hyphens into underscores to avoid collisions")
}

func TestAssemblePolicy_DomainGroupSkippedWhenCatalogCovers(t *testing.T) {
	// The embedded git-hosting.yaml has key "github" with match.always=true,
	// so it's always included. Adding "github" to AllowedDomains should NOT
	// overwrite the catalog entry.
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"github"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	np, ok := p.NetworkPolicies["github"]
	require.True(t, ok, "github policy entry must exist")
	assert.Equal(t, "GitHub", np.Name, "should retain catalog name, not group name")

	var hosts []string
	for _, ep := range np.Endpoints {
		hosts = append(hosts, ep.Host)
	}
	assert.Contains(t, hosts, "github.com", "catalog endpoint preserved")
	assert.Contains(t, hosts, "api.github.com", "catalog endpoint preserved")
	assert.NotContains(t, hosts, "github", "literal group name must not appear as hostname")
}

func TestAssemblePolicy_DomainGroupExpandedWhenNoCatalog(t *testing.T) {
	// "nodejs" has no embedded/catalog component, so it should be expanded
	// from the built-in domain group definition.
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"nodejs"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	np, ok := p.NetworkPolicies["nodejs"]
	require.True(t, ok, "nodejs policy entry must exist")

	var hosts []string
	for _, ep := range np.Endpoints {
		hosts = append(hosts, ep.Host)
	}
	assert.Contains(t, hosts, "registry.npmjs.org")
	assert.Contains(t, hosts, "npmjs.com")
	// Wildcard entries like ".npmjs.org" should be excluded.
	for _, h := range hosts {
		assert.False(t, strings.HasPrefix(h, "."),
			"wildcard domain %q should not appear in policy endpoints", h)
	}
}

func TestAssemblePolicy_LiteralDomainNotInCatalog(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"custom.internal.corp"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	np, ok := p.NetworkPolicies["custom_internal_corp"]
	require.True(t, ok)
	assert.Equal(t, "custom.internal.corp", np.Name)
	require.Len(t, np.Endpoints, 1)
	assert.Equal(t, "custom.internal.corp", np.Endpoints[0].Host)
	assert.Equal(t, 443, np.Endpoints[0].Port)
}

func TestAssemblePolicy_UnknownDomainGroupErrors(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"nonexistent_group"},
		},
	}

	_, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_group")
}

func TestAssemblePolicy_MixedGroupsAndLiterals(t *testing.T) {
	manifest := &Manifest{
		Version: 3,
		Network: &NetworkConfig{
			AllowedDomains: []string{"github", "nodejs", "api.custom.com"},
		},
	}

	p, err := AssemblePolicy(manifest, nil, "", nil, "")
	require.NoError(t, err)

	// github: covered by embedded catalog, should keep catalog entry
	gh, ok := p.NetworkPolicies["github"]
	require.True(t, ok)
	assert.Equal(t, "GitHub", gh.Name)

	// nodejs: no catalog, should be expanded from built-in group
	node, ok := p.NetworkPolicies["nodejs"]
	require.True(t, ok)
	assert.Greater(t, len(node.Endpoints), 1, "nodejs should have multiple expanded endpoints")

	// api.custom.com: literal domain
	custom, ok := p.NetworkPolicies["api_custom_com"]
	require.True(t, ok)
	require.Len(t, custom.Endpoints, 1)
	assert.Equal(t, "api.custom.com", custom.Endpoints[0].Host)
}
