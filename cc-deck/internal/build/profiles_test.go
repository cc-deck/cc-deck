package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildProfileManifest_AgentsAndTools(t *testing.T) {
	manifest := &Manifest{
		Agents: []string{"claude"},
		Tools:  []ToolEntry{{Name: "python"}},
	}
	matched := []PolicyComponent{
		{Match: MatchCondition{Tools: []string{"go"}}},
	}
	pm := BuildProfileManifest(manifest, matched)
	assert.Contains(t, pm.Profiles, "anthropic")
	assert.Contains(t, pm.Profiles, "claude-agent")
	assert.Contains(t, pm.Profiles, "python")
	assert.Contains(t, pm.Profiles, "golang")
	assert.Contains(t, pm.Profiles, "github")
	assert.Contains(t, pm.Profiles, "gitlab")
}

func TestBuildProfileManifest_EmptyDefaults(t *testing.T) {
	manifest := &Manifest{}
	pm := BuildProfileManifest(manifest, nil)
	assert.Contains(t, pm.Profiles, "anthropic", "defaults to claude agent")
	assert.Contains(t, pm.Profiles, "claude-agent")
	assert.Contains(t, pm.Profiles, "github")
	assert.Contains(t, pm.Profiles, "gitlab")
}

func TestBuildProfileManifest_DeterministicSorting(t *testing.T) {
	manifest := &Manifest{
		Agents: []string{"claude", "opencode"},
		Tools:  []ToolEntry{{Name: "python"}, {Name: "go"}},
	}
	pm1 := BuildProfileManifest(manifest, nil)
	pm2 := BuildProfileManifest(manifest, nil)
	assert.Equal(t, pm1.Profiles, pm2.Profiles, "repeated calls should produce identical sorted results")

	for i := 1; i < len(pm1.Profiles); i++ {
		assert.True(t, pm1.Profiles[i-1] <= pm1.Profiles[i], "profiles should be sorted")
	}
}

func TestBuildProfileManifest_WithCredentials(t *testing.T) {
	manifest := &Manifest{
		Credentials: []CredentialEntry{{Type: "vertex"}},
	}
	pm := BuildProfileManifest(manifest, nil)
	assert.Contains(t, pm.Profiles, "vertexai")
}

func TestBuildProfileManifest_MatchedComponentTools(t *testing.T) {
	manifest := &Manifest{}
	matched := []PolicyComponent{
		{Match: MatchCondition{Tools: []string{"python"}}},
		{Match: MatchCondition{Tools: []string{"node"}}},
	}
	pm := BuildProfileManifest(manifest, matched)
	assert.Contains(t, pm.Profiles, "python")
	assert.Contains(t, pm.Profiles, "nodejs")
}

func TestBuildProfileManifest_NoDuplicates(t *testing.T) {
	manifest := &Manifest{
		Tools: []ToolEntry{{Name: "python"}},
	}
	matched := []PolicyComponent{
		{Match: MatchCondition{Tools: []string{"python"}}},
	}
	pm := BuildProfileManifest(manifest, matched)
	count := 0
	for _, p := range pm.Profiles {
		if p == "python" {
			count++
		}
	}
	assert.Equal(t, 1, count, "python should appear exactly once")
}

func TestGenerateProfileManifest_WithOpenShellTarget(t *testing.T) {
	manifest := &Manifest{
		Agents: []string{"claude"},
		Tools:  []ToolEntry{{Name: "python"}},
		Targets: &TargetsConfig{
			OpenShell: &OpenShellTarget{Name: "test-sandbox"},
		},
	}
	matched := []PolicyComponent{
		{Match: MatchCondition{Tools: []string{"node"}}},
	}
	pm := GenerateProfileManifest(manifest, matched)
	require.NotNil(t, pm)
	assert.Contains(t, pm.Profiles, "anthropic")
	assert.Contains(t, pm.Profiles, "python")
	assert.Contains(t, pm.Profiles, "nodejs")
}

func TestGenerateProfileManifest_NoOpenShellTarget(t *testing.T) {
	manifest := &Manifest{
		Agents: []string{"claude"},
		Targets: &TargetsConfig{
			Container: &ContainerTarget{Name: "test-container"},
		},
	}
	pm := GenerateProfileManifest(manifest, nil)
	assert.Nil(t, pm, "non-OpenShell targets should return nil")
}

func TestGenerateProfileManifest_NilTargets(t *testing.T) {
	manifest := &Manifest{Agents: []string{"claude"}}
	pm := GenerateProfileManifest(manifest, nil)
	assert.Nil(t, pm)
}

func TestGenerateProfileManifest_UnionMerge(t *testing.T) {
	manifest := &Manifest{
		Tools: []ToolEntry{{Name: "python"}},
		Targets: &TargetsConfig{
			OpenShell: &OpenShellTarget{Name: "test"},
		},
	}
	matched := []PolicyComponent{
		{Match: MatchCondition{Tools: []string{"node"}}},
	}
	pm := GenerateProfileManifest(manifest, matched)
	require.NotNil(t, pm)
	assert.Contains(t, pm.Profiles, "python", "manifest tool")
	assert.Contains(t, pm.Profiles, "nodejs", "detected tool")

	count := 0
	for _, p := range pm.Profiles {
		if p == "python" {
			count++
		}
	}
	assert.Equal(t, 1, count, "no duplicates after union merge")
}

func TestBuildProfileManifest_WithCustomDomains(t *testing.T) {
	manifest := &Manifest{
		Network: &NetworkConfig{
			AllowedDomains:         []string{"custom.example.com"},
			AllowedDomainsPerAgent: map[string][]string{"claude": {"agent-specific.io"}},
		},
	}
	pm := BuildProfileManifest(manifest, nil)
	assert.Contains(t, pm.CustomDomains, "custom.example.com")
	assert.Contains(t, pm.CustomDomains, "agent-specific.io")
}

func TestBuildProfileManifest_CustomDomainsDedup(t *testing.T) {
	manifest := &Manifest{
		Network: &NetworkConfig{
			AllowedDomains:         []string{"shared.com"},
			AllowedDomainsPerAgent: map[string][]string{"claude": {"shared.com", "unique.io"}},
		},
	}
	pm := BuildProfileManifest(manifest, nil)
	count := 0
	for _, d := range pm.CustomDomains {
		if d == "shared.com" {
			count++
		}
	}
	assert.Equal(t, 1, count, "shared.com should appear exactly once")
	assert.Contains(t, pm.CustomDomains, "unique.io")
}

func TestBuildProfileManifest_WithMCPEntries(t *testing.T) {
	manifest := &Manifest{
		MCP: []MCPEntry{
			{Name: "mcp-a", Endpoint: "host-a:9090"},
			{Name: "mcp-b", Endpoint: "host-b:9091"},
			{Name: "mcp-no-endpoint"},
		},
	}
	pm := BuildProfileManifest(manifest, nil)
	assert.Len(t, pm.MCP, 2, "entries without endpoints should be excluded")
	assert.Equal(t, "mcp-a", pm.MCP[0].Name)
	assert.Equal(t, "host-a:9090", pm.MCP[0].Endpoint)
	assert.Equal(t, "mcp-b", pm.MCP[1].Name)
}

func TestMarshalAndParseProfileManifest(t *testing.T) {
	pm := &ProfileManifest{
		Profiles: []string{"anthropic", "claude-agent", "github", "gitlab", "python"},
		MCP: []MCPManifestEntry{
			{Name: "test-mcp", Endpoint: "localhost:8080"},
		},
		AgentBinaries: []string{"/usr/bin/claude"},
	}
	data, err := MarshalProfileManifest(pm)
	require.NoError(t, err)

	parsed, err := ParseProfileManifest(data)
	require.NoError(t, err)
	assert.Equal(t, pm.Profiles, parsed.Profiles)
	assert.Len(t, parsed.MCP, 1)
	assert.Equal(t, "test-mcp", parsed.MCP[0].Name)
	assert.Equal(t, "localhost:8080", parsed.MCP[0].Endpoint)
	assert.Equal(t, []string{"/usr/bin/claude"}, parsed.AgentBinaries)
}
