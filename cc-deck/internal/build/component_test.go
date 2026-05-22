package build

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFS(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

// T014: LoadComponentsFromFS tests

func TestLoadComponentsFromFS_ValidYAML(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/test.yaml": `
key: test_key
name: Test Component
match:
  always: true
endpoints:
  - host: example.com
    port: 443
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, warnings)
	require.Len(t, comps, 1)
	assert.Equal(t, "test_key", comps[0].Key)
	assert.Equal(t, "Test Component", comps[0].Name)
	assert.True(t, comps[0].Match.Always)
	require.Len(t, comps[0].Endpoints, 1)
	assert.Equal(t, "example.com", comps[0].Endpoints[0].Host)
	assert.Equal(t, 443, comps[0].Endpoints[0].Port)
}

func TestLoadComponentsFromFS_MultipleFiles(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/a.yaml": `
key: alpha
name: Alpha
match:
  always: true
endpoints:
  - host: alpha.com
    port: 443
`,
		"policies/b.yaml": `
key: beta
name: Beta
match:
  tools:
    - go
endpoints:
  - host: beta.com
    port: 8080
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, warnings)
	require.Len(t, comps, 2)
}

func TestLoadComponentsFromFS_InvalidYAML(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/bad.yaml": `invalid: [yaml: broken`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, comps)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Error(), "parsing bad.yaml")
}

func TestLoadComponentsFromFS_MissingRequiredFields(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/nokey.yaml": `
name: No Key
match:
  always: true
endpoints:
  - host: example.com
    port: 443
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, comps)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Error(), "key is required")
}

func TestLoadComponentsFromFS_SkipsNonYAML(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/readme.md": `# Not a component`,
		"policies/valid.yaml": `
key: valid
name: Valid
match:
  always: true
endpoints:
  - host: valid.com
    port: 443
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, warnings)
	require.Len(t, comps, 1)
	assert.Equal(t, "valid", comps[0].Key)
}

func TestLoadComponentsFromFS_MissingDirectory(t *testing.T) {
	fsys := newTestFS(map[string]string{})

	comps, warnings := LoadComponentsFromFS(fsys, "nonexistent")
	assert.Nil(t, comps)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Error(), "reading component directory")
}

// T015: ValidateComponent tests

func TestValidateComponent_Valid(t *testing.T) {
	comp := &PolicyComponent{
		Key:  "test",
		Name: "Test",
		Match: MatchCondition{
			Always: true,
		},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
}

func TestValidateComponent_EmptyKey(t *testing.T) {
	comp := &PolicyComponent{
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key is required")
}

func TestValidateComponent_EmptyName(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestValidateComponent_NoMatchConditions(t *testing.T) {
	comp := &PolicyComponent{
		Key:  "test",
		Name: "Test",
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match must have at least one field set")
}

func TestValidateComponent_EmptyEndpoints(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoints must contain at least one entry")
}

func TestValidateComponent_EndpointMissingHost(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Port: 443},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestValidateComponent_EndpointMissingPort(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com"},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port is required")
}

func TestValidateComponent_RestProtocolRequiresAccess(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "api.example.com", Port: 443, Protocol: "rest"},
		},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol=rest requires access or rules")
}

func TestValidateComponent_RestProtocolWithAccess(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "api.example.com", Port: 443, Protocol: "rest", Access: "full"},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
}

func TestValidateComponent_RestProtocolWithRules(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{
				Host:     "api.example.com",
				Port:     443,
				Protocol: "rest",
				Rules:    []PolicyRule{{Allow: &PolicyRuleMatch{Method: "GET", Path: "/"}}},
			},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
}

func TestValidateComponent_ToolsMatchIsValid(t *testing.T) {
	comp := &PolicyComponent{
		Key:  "test",
		Name: "Test",
		Match: MatchCondition{
			Tools: []string{"go"},
		},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
}

func TestValidateComponent_CredentialsMatchIsValid(t *testing.T) {
	comp := &PolicyComponent{
		Key:  "test",
		Name: "Test",
		Match: MatchCondition{
			Credentials: []string{"vertex"},
		},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
}

// T016: MatchComponent tests

func TestMatchComponent_AlwaysTrue(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Always: true},
	}
	manifest := &Manifest{Version: 3}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_AlwaysTrueWithEmptyManifest(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Always: true, Tools: []string{"nonexistent"}},
	}
	manifest := &Manifest{Version: 3}
	assert.True(t, MatchComponent(comp, manifest), "always: true should short-circuit other conditions")
}

func TestMatchComponent_ToolMatch(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"cargo"}},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_ToolMatchCaseInsensitive(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"Cargo"}},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_ToolMatchSubstring(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"rust"}},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "Rust Analyzer"}},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_ToolMatchDetectedTools(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"go"}},
	}
	manifest := &Manifest{
		Version: 3,
		Sources: []SourceEntry{
			{URL: "https://github.com/test/repo", DetectedTools: []string{"go", "python"}},
		},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_ToolNoMatch(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"ruby"}},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}
	assert.False(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_ToolORSemantics(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"ruby", "cargo"}},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "cargo"}},
	}
	assert.True(t, MatchComponent(comp, manifest), "any tool match should include component")
}

func TestMatchComponent_CredentialMatch(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Credentials: []string{"claude-vertex"}},
	}
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "claude-vertex"}},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_CredentialNoMatch(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Credentials: []string{"claude-vertex"}},
	}
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "generic"}},
	}
	assert.False(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_CredentialORSemantics(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Credentials: []string{"claude-vertex", "vertex"}},
	}
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "vertex"}},
	}
	assert.True(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_CrossFieldOR(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{
			Tools:       []string{"nonexistent"},
			Credentials: []string{"vertex"},
		},
	}
	manifest := &Manifest{
		Version:     3,
		Credentials: []CredentialEntry{{Type: "vertex"}},
	}
	assert.True(t, MatchComponent(comp, manifest), "credential match should include even if tool doesn't match")
}

func TestMatchComponent_NoConditionsNoMatch(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{},
	}
	manifest := &Manifest{
		Version: 3,
		Tools:   []ToolEntry{{Name: "go"}},
	}
	assert.False(t, MatchComponent(comp, manifest))
}

func TestMatchComponent_EmptyManifest(t *testing.T) {
	comp := &PolicyComponent{
		Match: MatchCondition{Tools: []string{"go"}},
	}
	manifest := &Manifest{Version: 3}
	assert.False(t, MatchComponent(comp, manifest))
}

// T017: ResolveComponents tests

func TestResolveComponents_SingleTier(t *testing.T) {
	tier := map[string]PolicyComponent{
		"alpha": {Key: "alpha_key", Name: "Alpha"},
		"beta":  {Key: "beta_key", Name: "Beta"},
	}

	result := ResolveComponents(tier)
	require.Len(t, result, 2)
	assert.Equal(t, "alpha_key", result[0].Key)
	assert.Equal(t, "beta_key", result[1].Key)
}

func TestResolveComponents_HigherTierReplacesLower(t *testing.T) {
	embedded := map[string]PolicyComponent{
		"rust": {Key: "pkg_rust", Name: "rust packages (embedded)", Endpoints: []PolicyEndpoint{{Host: "old.crates.io", Port: 443}}},
	}
	catalog := map[string]PolicyComponent{
		"rust": {Key: "pkg_rust", Name: "rust packages (catalog)", Endpoints: []PolicyEndpoint{{Host: "new.crates.io", Port: 443}}},
	}

	result := ResolveComponents(embedded, catalog)
	require.Len(t, result, 1)
	assert.Equal(t, "rust packages (catalog)", result[0].Name)
	assert.Equal(t, "new.crates.io", result[0].Endpoints[0].Host)
}

func TestResolveComponents_ThreeTierPrecedence(t *testing.T) {
	embedded := map[string]PolicyComponent{
		"rust": {Key: "pkg_rust", Name: "embedded"},
		"go":   {Key: "pkg_go", Name: "go-embedded"},
	}
	catalog := map[string]PolicyComponent{
		"rust": {Key: "pkg_rust", Name: "catalog"},
	}
	userLocal := map[string]PolicyComponent{
		"rust":   {Key: "pkg_rust", Name: "user-local"},
		"custom": {Key: "custom", Name: "custom-user"},
	}

	result := ResolveComponents(embedded, catalog, userLocal)
	require.Len(t, result, 3)

	byKey := make(map[string]PolicyComponent)
	for _, c := range result {
		byKey[c.Key] = c
	}
	assert.Equal(t, "user-local", byKey["pkg_rust"].Name, "user-local should win over catalog and embedded")
	assert.Equal(t, "go-embedded", byKey["pkg_go"].Name, "uncontested embedded should remain")
	assert.Equal(t, "custom-user", byKey["custom"].Name, "user-local-only component should appear")
}

func TestResolveComponents_AlphabeticalOrder(t *testing.T) {
	tier := map[string]PolicyComponent{
		"zebra":    {Key: "zebra"},
		"alpha":    {Key: "alpha"},
		"middle":   {Key: "middle"},
	}

	result := ResolveComponents(tier)
	require.Len(t, result, 3)
	assert.Equal(t, "alpha", result[0].Key)
	assert.Equal(t, "middle", result[1].Key)
	assert.Equal(t, "zebra", result[2].Key)
}

func TestResolveComponents_EmptyTiers(t *testing.T) {
	result := ResolveComponents()
	assert.Empty(t, result)
}

func TestResolveComponents_EmptyAndNonEmptyTiers(t *testing.T) {
	empty := map[string]PolicyComponent{}
	filled := map[string]PolicyComponent{
		"test": {Key: "test"},
	}

	result := ResolveComponents(empty, filled)
	require.Len(t, result, 1)
	assert.Equal(t, "test", result[0].Key)
}
