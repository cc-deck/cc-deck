package credential

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_AllPresent(t *testing.T) {
	t.Setenv("VAL_KEY", "secret")

	spec := agent.CredentialSpec{
		Name: "api",
		EnvVars: []agent.EnvVarSpec{
			{Name: "VAL_KEY", Required: true},
		},
	}

	err := Validate(spec, false)
	assert.NoError(t, err)
}

func TestValidate_MissingEnvVar(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "api",
		EnvVars: []agent.EnvVarSpec{
			{Name: "MISSING_KEY", Required: true},
		},
	}

	err := Validate(spec, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING_KEY")
	assert.Contains(t, err.Error(), "api")
}

func TestValidate_MissingFile(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "vertex",
		FileCredential: &agent.FileCredentialSpec{
			EnvVar:      "GOOGLE_CREDS",
			DefaultPath: "/nonexistent/path/creds.json",
			Required:    true,
		},
	}

	err := Validate(spec, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_CREDS")
}

func TestValidate_FilePresent(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))
	t.Setenv("VAL_FILE", credFile)

	spec := agent.CredentialSpec{
		Name: "vertex",
		FileCredential: &agent.FileCredentialSpec{
			EnvVar:   "VAL_FILE",
			Required: true,
		},
	}

	err := Validate(spec, false)
	assert.NoError(t, err)
}

func TestValidate_ExternalCredentials(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "api",
		EnvVars: []agent.EnvVarSpec{
			{Name: "DEFINITELY_MISSING", Required: true},
		},
	}

	err := Validate(spec, true)
	assert.NoError(t, err)
}

func TestValidate_FixedValueSkipped(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "vertex",
		EnvVars: []agent.EnvVarSpec{
			{Name: "USE_VERTEX", FixedValue: "1", Required: true},
		},
	}

	err := Validate(spec, false)
	assert.NoError(t, err)
}

func TestValidate_MultiMissing(t *testing.T) {
	spec := agent.CredentialSpec{
		Name: "bedrock",
		EnvVars: []agent.EnvVarSpec{
			{Name: "AWS_REGION", Required: true},
			{Name: "AWS_KEY", Required: true},
		},
	}

	err := Validate(spec, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS_REGION")
	assert.Contains(t, err.Error(), "AWS_KEY")
}
