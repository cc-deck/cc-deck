package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func writeTestConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, cfg.Save(path))
	return path
}

func TestRunProfileList_Empty(t *testing.T) {
	path := writeTestConfig(t, &config.Config{})
	gf := &GlobalFlags{ConfigFile: path, Output: ""}

	out := captureStdout(t, func() {
		err := runProfileList(gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No profiles configured")
}

func TestRunProfileList_TableFormat(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "work",
		Profiles: map[string]config.Profile{
			"work": {Backend: config.BackendAnthropic, APIKeySecret: "sec"},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path, Output: ""}

	out := captureStdout(t, func() {
		err := runProfileList(gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "work")
	assert.Contains(t, out, "anthropic")
	assert.Contains(t, out, "*")
}

func TestRunProfileList_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "vtx",
		Profiles: map[string]config.Profile{
			"vtx": {Backend: config.BackendVertex, Project: "proj", Region: "us-central1"},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path, Output: "json"}

	out := captureStdout(t, func() {
		err := runProfileList(gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, `"name": "vtx"`)
	assert.Contains(t, out, `"backend": "vertex"`)
	assert.Contains(t, out, `"default": true`)
}

func TestRunProfileList_YAMLFormat(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"anthro": {Backend: config.BackendAnthropic, APIKeySecret: "sec"},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path, Output: "yaml"}

	out := captureStdout(t, func() {
		err := runProfileList(gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "name: anthro")
	assert.Contains(t, out, "backend: anthropic")
}

func TestRunProfileList_LoadError(t *testing.T) {
	// Pointing ConfigFile at a directory makes os.ReadFile fail with a
	// non-NotExist error, which config.Load propagates as an error.
	gf := &GlobalFlags{ConfigFile: t.TempDir()}
	err := runProfileList(gf)
	assert.Error(t, err)
}

func TestRunProfileUse_SetsDefault(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"a": {Backend: config.BackendAnthropic, APIKeySecret: "x"},
			"b": {Backend: config.BackendAnthropic, APIKeySecret: "y"},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path}

	out := captureStdout(t, func() {
		err := runProfileUse("b", gf)
		require.NoError(t, err)
	})
	assert.Contains(t, out, `Default profile set to "b"`)

	reloaded, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "b", reloaded.DefaultProfile)
}

func TestRunProfileUse_UnknownProfile(t *testing.T) {
	path := writeTestConfig(t, &config.Config{})
	gf := &GlobalFlags{ConfigFile: path}

	err := runProfileUse("missing", gf)
	assert.Error(t, err)
}

func TestRunProfileShow_Anthropic(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "work",
		Profiles: map[string]config.Profile{
			"work": {
				Backend:      config.BackendAnthropic,
				APIKeySecret: "my-secret",
				Model:        "claude-opus",
				Permissions:  "default",
			},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path}

	out := captureStdout(t, func() {
		err := runProfileShow("work", gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Profile: work (default)")
	assert.Contains(t, out, "Backend:  anthropic")
	assert.Contains(t, out, "API Key Secret:  my-secret")
	assert.Contains(t, out, "Model:    claude-opus")
	assert.Contains(t, out, "Permissions:  default")
}

func TestRunProfileShow_VertexWorkloadIdentity(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"gcp": {
				Backend: config.BackendVertex,
				Project: "my-proj",
				Region:  "us-central1",
			},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path}

	out := captureStdout(t, func() {
		err := runProfileShow("gcp", gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Project:  my-proj")
	assert.Contains(t, out, "Region:   us-central1")
	assert.Contains(t, out, "Credentials:  Workload Identity")
	assert.NotContains(t, out, "(default)")
}

func TestRunProfileShow_VertexWithCredentialsSecret(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"gcp": {
				Backend:           config.BackendVertex,
				Project:           "my-proj",
				Region:            "us-central1",
				CredentialsSecret: "gcp-creds",
			},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path}

	out := captureStdout(t, func() {
		err := runProfileShow("gcp", gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Credentials Secret:  gcp-creds")
}

func TestRunProfileShow_GitCredentialAndEgress(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"full": {
				Backend:             config.BackendAnthropic,
				APIKeySecret:        "secret",
				AllowedEgress:       []string{"github.com", "pypi.org"},
				GitCredentialType:   "ssh",
				GitCredentialSecret: "git-secret",
			},
		},
	}
	path := writeTestConfig(t, cfg)
	gf := &GlobalFlags{ConfigFile: path}

	out := captureStdout(t, func() {
		err := runProfileShow("full", gf)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Allowed Egress:  [github.com pypi.org]")
	assert.Contains(t, out, "Git Credential Type:  ssh")
	assert.Contains(t, out, "Git Credential Secret:  git-secret")
}

func TestRunProfileShow_UnknownProfile(t *testing.T) {
	path := writeTestConfig(t, &config.Config{})
	gf := &GlobalFlags{ConfigFile: path}

	err := runProfileShow("nope", gf)
	assert.Error(t, err)
}

func TestNewProfileCmd_HasSubcommands(t *testing.T) {
	gf := &GlobalFlags{}
	cmd := NewProfileCmd(gf)
	assert.Equal(t, "profile", cmd.Use)

	names := make([]string, 0)
	for _, c := range cmd.Commands() {
		names = append(names, c.Name())
	}
	assert.ElementsMatch(t, []string{"add", "list", "use", "show"}, names)
}
