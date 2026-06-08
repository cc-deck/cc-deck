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
