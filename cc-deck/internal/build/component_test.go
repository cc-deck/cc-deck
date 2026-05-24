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

// T003: ProbeBinaries and RuntimeGlobs field parsing and validation tests

func TestLoadComponentsFromFS_ProbeBinariesParsed(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/python.yaml": `
key: pkg_python
name: python packages
match:
  tools:
    - python
endpoints:
  - host: pypi.org
    port: 443
probe_binaries:
  - pip
  - pip3
  - uv
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, warnings)
	require.Len(t, comps, 1)
	assert.Equal(t, []string{"pip", "pip3", "uv"}, comps[0].ProbeBinaries)
}

func TestLoadComponentsFromFS_RuntimeGlobsParsed(t *testing.T) {
	fsys := newTestFS(map[string]string{
		"policies/python.yaml": `
key: pkg_python
name: python packages
match:
  tools:
    - python
endpoints:
  - host: pypi.org
    port: 443
runtime_globs:
  - /sandbox/**/bin/pip
  - /sandbox/**/bin/pip3
`,
	})

	comps, warnings := LoadComponentsFromFS(fsys, "policies")
	assert.Empty(t, warnings)
	require.Len(t, comps, 1)
	assert.Equal(t, []string{"/sandbox/**/bin/pip", "/sandbox/**/bin/pip3"}, comps[0].RuntimeGlobs)
}

func TestValidateComponent_ProbeBinariesWithSlashFails(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
		ProbeBinaries: []string{"/usr/bin/pip"},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe_binaries")
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestValidateComponent_ProbeBinariesWithShellMetacharsFails(t *testing.T) {
	cases := []string{"pip;rm", "pip$(whoami)", "pip|cat", "pip&bg", "pip`id`"}
	for _, name := range cases {
		comp := &PolicyComponent{
			Key:   "test",
			Name:  "Test",
			Match: MatchCondition{Always: true},
			Endpoints: []PolicyEndpoint{
				{Host: "example.com", Port: 443},
			},
			ProbeBinaries: []string{name},
		}
		err := ValidateComponent(comp, "test.yaml")
		require.Error(t, err, "expected error for probe_binaries entry %q", name)
		assert.Contains(t, err.Error(), "invalid characters")
	}
}

func TestValidateComponent_ProbeBinariesWithBackslashFails(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
		ProbeBinaries: []string{`pip\bin`},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestValidateComponent_RuntimeGlobsWithoutSlashFails(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
		RuntimeGlobs: []string{"sandbox/**/bin/pip"},
	}
	err := ValidateComponent(comp, "test.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime_globs")
	assert.Contains(t, err.Error(), "must start with /")
}

func TestEmbeddedComponents_ProbeBinariesAndRuntimeGlobs(t *testing.T) {
	comps, warnings := LoadEmbeddedComponents()
	assert.Empty(t, warnings)

	compsByKey := make(map[string]PolicyComponent)
	for _, stem := range []string{"python", "rust", "node", "go", "claude-code", "git-hosting", "vertex-ai"} {
		if comp, ok := comps[stem]; ok {
			compsByKey[comp.Key] = comp
		}
	}

	python := compsByKey["pkg_python"]
	assert.Equal(t, []string{"pip", "pip3", "uv", "python3"}, python.ProbeBinaries)
	assert.Contains(t, python.RuntimeGlobs, "/sandbox/**/bin/pip")
	assert.Contains(t, python.RuntimeGlobs, "/sandbox/**/bin/python3")

	rust := compsByKey["pkg_rust"]
	assert.Equal(t, []string{"cargo", "rustc"}, rust.ProbeBinaries)
	assert.Contains(t, rust.RuntimeGlobs, "/sandbox/.rustup/toolchains/*/bin/cargo")

	node := compsByKey["pkg_node"]
	assert.Equal(t, []string{"node", "npm", "npx"}, node.ProbeBinaries)
	assert.Contains(t, node.RuntimeGlobs, "/sandbox/**/node_modules/.bin/*")

	goComp := compsByKey["pkg_go"]
	assert.Equal(t, []string{"go"}, goComp.ProbeBinaries)
	assert.Contains(t, goComp.RuntimeGlobs, "/sandbox/go/bin/*")

	claude := compsByKey["claude_code"]
	assert.Empty(t, claude.ProbeBinaries)
	assert.Empty(t, claude.RuntimeGlobs)

	github := compsByKey["github"]
	assert.Empty(t, github.ProbeBinaries)
	assert.Empty(t, github.RuntimeGlobs)

	vertex := compsByKey["vertex_ai"]
	assert.Empty(t, vertex.ProbeBinaries)
	assert.Empty(t, vertex.RuntimeGlobs)
}

func TestValidateComponent_BothFieldsOmittedPasses(t *testing.T) {
	comp := &PolicyComponent{
		Key:   "test",
		Name:  "Test",
		Match: MatchCondition{Always: true},
		Endpoints: []PolicyEndpoint{
			{Host: "example.com", Port: 443},
		},
	}
	assert.NoError(t, ValidateComponent(comp, "test.yaml"))
	assert.Empty(t, comp.ProbeBinaries)
	assert.Empty(t, comp.RuntimeGlobs)
}
