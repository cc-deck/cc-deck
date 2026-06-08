package credential

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetect_AllRequiredSet(t *testing.T) {
	t.Setenv("TEST_API_KEY", "sk-test")

	specs := []agent.CredentialSpec{
		{
			Name: "api",
			EnvVars: []agent.EnvVarSpec{
				{Name: "TEST_API_KEY", Required: true},
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, "api", available[0].Spec.Name)
	assert.Equal(t, "sk-test", available[0].Values["TEST_API_KEY"])
}

func TestDetect_PartialRequired(t *testing.T) {
	t.Setenv("TEST_PROJECT_ID", "my-project")
	// TEST_REGION is not set

	specs := []agent.CredentialSpec{
		{
			Name: "vertex",
			EnvVars: []agent.EnvVarSpec{
				{Name: "TEST_PROJECT_ID", Required: true},
				{Name: "TEST_REGION", Required: true},
			},
		},
	}

	available := Detect(specs)
	assert.Empty(t, available)
}

func TestDetect_FileCredentialExists(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))

	t.Setenv("TEST_CRED_FILE", credFile)

	specs := []agent.CredentialSpec{
		{
			Name: "file-mode",
			FileCredential: &agent.FileCredentialSpec{
				EnvVar:   "TEST_CRED_FILE",
				Required: true,
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, credFile, available[0].Values["TEST_CRED_FILE"])
}

func TestDetect_FileCredentialMissing(t *testing.T) {
	t.Setenv("TEST_CRED_FILE", "/nonexistent/path/creds.json")

	specs := []agent.CredentialSpec{
		{
			Name: "file-mode",
			FileCredential: &agent.FileCredentialSpec{
				EnvVar:   "TEST_CRED_FILE",
				Required: true,
			},
		},
	}

	available := Detect(specs)
	assert.Empty(t, available)
}

func TestDetect_FileCredentialDefaultPath(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "default-creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))

	specs := []agent.CredentialSpec{
		{
			Name: "default-path",
			FileCredential: &agent.FileCredentialSpec{
				EnvVar:      "TEST_CRED_DEFAULT",
				DefaultPath: credFile,
				Required:    true,
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, credFile, available[0].Values["TEST_CRED_DEFAULT"])
}

func TestDetect_FixedValueVars(t *testing.T) {
	specs := []agent.CredentialSpec{
		{
			Name: "fixed",
			EnvVars: []agent.EnvVarSpec{
				{Name: "USE_VERTEX", FixedValue: "1"},
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, "1", available[0].Values["USE_VERTEX"])
}

func TestDetect_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Create a temp file under $HOME so tilde expansion works.
	subDir := filepath.Join(home, ".cc-deck-test-"+t.Name())
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	t.Cleanup(func() { os.RemoveAll(subDir) })

	credFile := filepath.Join(subDir, "tilde-creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))

	relPath := "~" + credFile[len(home):]

	specs := []agent.CredentialSpec{
		{
			Name: "tilde",
			FileCredential: &agent.FileCredentialSpec{
				EnvVar:      "TEST_TILDE",
				DefaultPath: relPath,
				Required:    true,
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, credFile, available[0].Values["TEST_TILDE"])
}

func TestDetect_MultipleSpecs(t *testing.T) {
	t.Setenv("TEST_KEY_A", "val-a")

	specs := []agent.CredentialSpec{
		{
			Name: "mode-a",
			EnvVars: []agent.EnvVarSpec{
				{Name: "TEST_KEY_A", Required: true},
			},
		},
		{
			Name: "mode-b",
			EnvVars: []agent.EnvVarSpec{
				{Name: "TEST_KEY_B", Required: true},
			},
		},
	}

	available := Detect(specs)
	require.Len(t, available, 1)
	assert.Equal(t, "mode-a", available[0].Spec.Name)
}

func TestResolve_EnvVarResolution(t *testing.T) {
	t.Setenv("RESOLVE_KEY", "resolved-value")
	t.Setenv("RESOLVE_OPT", "optional-value")

	spec := agent.CredentialSpec{
		Name: "test",
		EnvVars: []agent.EnvVarSpec{
			{Name: "RESOLVE_KEY", Required: true},
			{Name: "RESOLVE_OPT"},
			{Name: "RESOLVE_MISSING"},
		},
	}

	resolved := Resolve(spec)
	assert.Equal(t, "resolved-value", resolved.EnvVars["RESOLVE_KEY"])
	assert.Equal(t, "optional-value", resolved.EnvVars["RESOLVE_OPT"])
	_, hasMissing := resolved.EnvVars["RESOLVE_MISSING"]
	assert.False(t, hasMissing)
}

func TestResolve_FixedValues(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "fixed",
		EnvVars: []agent.EnvVarSpec{
			{Name: "FIXED_FLAG", FixedValue: "1"},
		},
	}

	resolved := Resolve(spec)
	assert.Equal(t, "1", resolved.EnvVars["FIXED_FLAG"])
}

func TestResolve_FileCredential(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))
	t.Setenv("RESOLVE_FILE", credFile)

	spec := agent.CredentialSpec{
		Name: "file",
		FileCredential: &agent.FileCredentialSpec{
			EnvVar:   "RESOLVE_FILE",
			Required: true,
		},
	}

	resolved := Resolve(spec)
	require.NotNil(t, resolved.FileCredential)
	assert.Equal(t, "RESOLVE_FILE", resolved.FileCredential.EnvVar)
	assert.Equal(t, credFile, resolved.FileCredential.LocalPath)
}

func TestResolve_FileCredentialDefaultFallback(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "default.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))

	spec := agent.CredentialSpec{
		Name: "default-file",
		FileCredential: &agent.FileCredentialSpec{
			EnvVar:      "RESOLVE_DEFAULT_FILE",
			DefaultPath: credFile,
			Required:    true,
		},
	}

	resolved := Resolve(spec)
	require.NotNil(t, resolved.FileCredential)
	assert.Equal(t, credFile, resolved.FileCredential.LocalPath)
}

func TestResolve_UnsetVars(t *testing.T) {
	spec := agent.CredentialSpec{
		Name:      "unset-test",
		UnsetVars: []string{"GEMINI_API_KEY"},
	}

	resolved := Resolve(spec)
	assert.Equal(t, []string{"GEMINI_API_KEY"}, resolved.UnsetVars)
}

// stubAgent is a minimal Agent for testing DetectAll and related functions.
type stubAgent struct {
	name  string
	specs []agent.CredentialSpec
}

func (s *stubAgent) Name() string                                       { return s.name }
func (s *stubAgent) DisplayName() string                                { return s.name }
func (s *stubAgent) Indicator() string                                  { return s.name[:2] }
func (s *stubAgent) IsInstalled() bool                                  { return false }
func (s *stubAgent) DetectConfig() string                               { return "" }
func (s *stubAgent) InstallHooks() error                                { return nil }
func (s *stubAgent) UninstallHooks() error                              { return nil }
func (s *stubAgent) HooksInstalled() bool                               { return false }
func (s *stubAgent) TranslateEvent(_ []byte) (*agent.NormalizedPayload, error) { return nil, nil }
func (s *stubAgent) CredentialSpecs() []agent.CredentialSpec             { return s.specs }

func registerStubAgent(t *testing.T, name string, specs []agent.CredentialSpec) {
	t.Helper()
	agent.Register(&stubAgent{name: name, specs: specs})
}

func TestDetectAll_MultipleAgents(t *testing.T) {
	agent.Reset()
	t.Cleanup(agent.Reset)

	t.Setenv("TEST_CLAUDE_KEY", "sk-claude")
	t.Setenv("TEST_OC_KEY", "sk-opencode")

	registerStubAgent(t, "claude", []agent.CredentialSpec{
		{Name: "api", EnvVars: []agent.EnvVarSpec{{Name: "TEST_CLAUDE_KEY", Required: true}}},
	})
	registerStubAgent(t, "opencode", []agent.CredentialSpec{
		{Name: "openai", EnvVars: []agent.EnvVarSpec{{Name: "TEST_OC_KEY", Required: true}}},
	})

	modes := DetectAll()
	require.Len(t, modes, 2)
	assert.Equal(t, "claude", modes[0].AgentName)
	assert.Equal(t, "api", modes[0].Spec.Name)
	assert.Equal(t, "opencode", modes[1].AgentName)
	assert.Equal(t, "openai", modes[1].Spec.Name)
}

func TestDetectAll_SingleAgent(t *testing.T) {
	agent.Reset()
	t.Cleanup(agent.Reset)

	t.Setenv("TEST_ONLY_KEY", "sk-test")

	registerStubAgent(t, "solo", []agent.CredentialSpec{
		{Name: "api", EnvVars: []agent.EnvVarSpec{{Name: "TEST_ONLY_KEY", Required: true}}},
	})

	modes := DetectAll()
	require.Len(t, modes, 1)
	assert.Equal(t, "solo", modes[0].AgentName)
}

func TestDetectAll_NoCredentials(t *testing.T) {
	agent.Reset()
	t.Cleanup(agent.Reset)

	registerStubAgent(t, "empty", []agent.CredentialSpec{
		{Name: "api", EnvVars: []agent.EnvVarSpec{{Name: "MISSING_KEY_12345", Required: true}}},
	})

	modes := DetectAll()
	assert.Empty(t, modes)
}

func TestMergeCredentials_DisjointMerge(t *testing.T) {
	modes := []DetectedMode{
		{
			AgentName: "claude",
			Spec:      agent.CredentialSpec{Name: "api"},
			Resolved: ResolvedCredentials{
				EnvVars: map[string]string{"ANTHROPIC_API_KEY": "sk-1"},
			},
		},
		{
			AgentName: "opencode",
			Spec:      agent.CredentialSpec{Name: "openai"},
			Resolved: ResolvedCredentials{
				EnvVars: map[string]string{"OPENAI_API_KEY": "sk-2"},
			},
		},
	}

	merged, err := MergeCredentials(modes)
	require.NoError(t, err)
	assert.Equal(t, "sk-1", merged.EnvVars["ANTHROPIC_API_KEY"])
	assert.Equal(t, "sk-2", merged.EnvVars["OPENAI_API_KEY"])
}

func TestMergeCredentials_SameKeySameValueDedup(t *testing.T) {
	modes := []DetectedMode{
		{
			AgentName: "a",
			Spec:      agent.CredentialSpec{Name: "m1"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{"SHARED": "val"}},
		},
		{
			AgentName: "b",
			Spec:      agent.CredentialSpec{Name: "m2"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{"SHARED": "val"}},
		},
	}

	merged, err := MergeCredentials(modes)
	require.NoError(t, err)
	assert.Equal(t, "val", merged.EnvVars["SHARED"])
}

func TestMergeCredentials_SameKeyDifferentValueError(t *testing.T) {
	modes := []DetectedMode{
		{
			AgentName: "a",
			Spec:      agent.CredentialSpec{Name: "m1"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{"KEY": "val1"}},
		},
		{
			AgentName: "b",
			Spec:      agent.CredentialSpec{Name: "m2"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{"KEY": "val2"}},
		},
	}

	_, err := MergeCredentials(modes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting env var")
}

func TestMergeCredentials_FileCredentialCollection(t *testing.T) {
	modes := []DetectedMode{
		{
			AgentName: "claude",
			Spec:      agent.CredentialSpec{Name: "vertex"},
			Resolved: ResolvedCredentials{
				EnvVars:        map[string]string{"V": "1"},
				FileCredential: &ResolvedFile{EnvVar: "GOOGLE_CREDS", LocalPath: "/path/a"},
			},
		},
		{
			AgentName: "opencode",
			Spec:      agent.CredentialSpec{Name: "gcp"},
			Resolved: ResolvedCredentials{
				EnvVars:        map[string]string{"O": "2"},
				FileCredential: &ResolvedFile{EnvVar: "OTHER_CREDS", LocalPath: "/path/b"},
			},
		},
	}

	merged, err := MergeCredentials(modes)
	require.NoError(t, err)
	require.Len(t, merged.FileCredentials, 2)
	assert.Equal(t, "GOOGLE_CREDS", merged.FileCredentials[0].EnvVar)
	assert.Equal(t, "OTHER_CREDS", merged.FileCredentials[1].EnvVar)
}

func TestMergeCredentials_EmptyInput(t *testing.T) {
	merged, err := MergeCredentials(nil)
	require.NoError(t, err)
	assert.Empty(t, merged.EnvVars)
	assert.Nil(t, merged.FileCredential)
	assert.Nil(t, merged.FileCredentials)
	assert.Nil(t, merged.UnsetVars)

	merged2, err2 := MergeCredentials([]DetectedMode{})
	require.NoError(t, err2)
	assert.Empty(t, merged2.EnvVars)
}

func TestMergeCredentials_UnsetVarsMerge(t *testing.T) {
	modes := []DetectedMode{
		{
			AgentName: "a",
			Spec:      agent.CredentialSpec{Name: "m1"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{}, UnsetVars: []string{"VAR_A"}},
		},
		{
			AgentName: "b",
			Spec:      agent.CredentialSpec{Name: "m2"},
			Resolved:  ResolvedCredentials{EnvVars: map[string]string{}, UnsetVars: []string{"VAR_B"}},
		},
	}

	merged, err := MergeCredentials(modes)
	require.NoError(t, err)
	assert.Contains(t, merged.UnsetVars, "VAR_A")
	assert.Contains(t, merged.UnsetVars, "VAR_B")
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", home},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, expandTilde(tt.input))
		})
	}
}
