package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
)

func writeMinimalConfig(t *testing.T, dir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(cfgPath, []byte(""), 0o644))
	return cfgPath
}

func TestProfileAdd_WithFlags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalConfig(t, dir)

	gf := &GlobalFlags{ConfigFile: cfgPath}
	cmd := NewProfileCmd(gf)

	cmd.SetArgs([]string{"add", "work",
		"--harness", "claude",
		"--backend", "anthropic",
		"--api-key-env", "MY_KEY",
		"--model", "claude-sonnet-5",
		"--color", "#4FC1E9",
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	p, ok := cfg.Profiles["work"]
	require.True(t, ok, "profile 'work' should exist in saved config")
	assert.Equal(t, "claude", p.Harness)
	assert.Equal(t, config.BackendAnthropic, p.Backend)
	assert.Equal(t, "claude-sonnet-5", p.Model)
	assert.Equal(t, "#4FC1E9", p.Color)
	require.NotNil(t, p.Auth)
	require.NotNil(t, p.Auth.APIKey)
	assert.Equal(t, "MY_KEY", p.Auth.APIKey.Env)
}

func TestProfileAdd_InvalidColor(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalConfig(t, dir)

	gf := &GlobalFlags{ConfigFile: cfgPath}
	cmd := NewProfileCmd(gf)

	cmd.SetArgs([]string{"add", "bad-color",
		"--harness", "claude",
		"--api-key-env", "KEY",
		"--color", "invalid",
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color must be #RRGGBB")
}

func TestProfileAdd_InvalidIcon(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalConfig(t, dir)

	gf := &GlobalFlags{ConfigFile: cfgPath}
	cmd := NewProfileCmd(gf)

	cmd.SetArgs([]string{"add", "bad-icon",
		"--harness", "claude",
		"--api-key-env", "KEY",
		"--icon", "AB",
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "icon must be a single glyph")
}

func captureProfileStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

func TestProfileList_Columns(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := &config.Config{
		DefaultProfile: "work",
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Backend: config.BackendAnthropic,
				Model:   "claude-sonnet-5",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
			"team": {
				Harness: "codex",
				Backend: config.BackendOpenAI,
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "TEAM_KEY"}},
			},
		},
	}
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0o644))

	gf := &GlobalFlags{ConfigFile: cfgPath}

	output := captureProfileStdout(t, func() {
		cmd := NewProfileCmd(gf)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"list"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "HARNESS")
	assert.Contains(t, output, "BACKEND")
	assert.Contains(t, output, "AUTH")
	assert.Contains(t, output, "MODEL")
	assert.Contains(t, output, "DEFAULT")
	assert.Contains(t, output, "work")
	assert.Contains(t, output, "team")
}

func TestProfileShow_PrintsReferences(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"secure": {
				Harness: "claude",
				Backend: config.BackendAnthropic,
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "ANTHROPIC_API_KEY"}},
			},
		},
	}
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0o644))

	gf := &GlobalFlags{ConfigFile: cfgPath}

	output := captureProfileStdout(t, func() {
		cmd := NewProfileCmd(gf)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"show", "secure"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "ANTHROPIC_API_KEY", "show should print the env var reference")
	assert.Contains(t, output, "env:")
	assert.Contains(t, output, "Wrapper:")
	assert.Contains(t, output, "claude-secure")
}

func TestProfileDelete_ClearsDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := &config.Config{
		DefaultProfile: "doomed",
		Profiles: map[string]config.Profile{
			"doomed": {
				Harness: "claude",
				Backend: config.BackendAnthropic,
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "KEY"}},
			},
		},
	}
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0o644))

	gf := &GlobalFlags{ConfigFile: cfgPath}

	captureProfileStdout(t, func() {
		cmd := NewProfileCmd(gf)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"delete", "doomed"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	reloaded, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, reloaded.DefaultProfile, "default_profile should be cleared after deleting the default")
	_, err = reloaded.GetProfile("doomed")
	assert.Error(t, err, "deleted profile should not be found")
}

// runProfileCmd executes the profile command with args against cfgPath.
func runProfileCmd(t *testing.T, cfgPath string, args ...string) error {
	t.Helper()
	cmd := NewProfileCmd(&GlobalFlags{ConfigFile: cfgPath})
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var err error
	captureProfileStdout(t, func() { err = cmd.Execute() })
	return err
}

// loadSavedProfiles reads the profiles back from cfgPath.
func loadSavedProfiles(t *testing.T, cfgPath string) map[string]config.Profile {
	t.Helper()
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var cfg config.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	return cfg.Profiles
}

func TestProfileAdd_PicksColorWhenUnset(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	require.NoError(t, runProfileCmd(t, cfgPath, "add", "first", "--harness", "claude", "--api-key-env", "K1"))
	saved := loadSavedProfiles(t, cfgPath)
	assert.Equal(t, profile.Pick(nil), saved["first"].Color, "first add writes the first picked palette color")
}

func TestProfileAdd_SequentialColorsAreDistinct(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	for _, name := range []string{"one", "two", "three"} {
		require.NoError(t, runProfileCmd(t, cfgPath, "add", name, "--harness", "claude", "--api-key-env", "K_"+name))
	}
	saved := loadSavedProfiles(t, cfgPath)
	colors := []string{saved["one"].Color, saved["two"].Color, saved["three"].Color}
	for i := range colors {
		require.NotEmpty(t, colors[i])
		for j := i + 1; j < len(colors); j++ {
			assert.False(t, profile.Similar(colors[i], colors[j]),
				"colors %s and %s must not be similar", colors[i], colors[j])
		}
	}
}

func TestProfileAdd_ExplicitColorKeptVerbatim(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	require.NoError(t, runProfileCmd(t, cfgPath, "add", "mine", "--harness", "claude", "--api-key-env", "K", "--color", "#abcdef"))
	assert.Equal(t, "#abcdef", loadSavedProfiles(t, cfgPath)["mine"].Color)
}

func TestProfileAdd_OAuthTokenEnv(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	require.NoError(t, runProfileCmd(t, cfgPath, "add", "private", "--harness", "claude", "--oauth-token-env", "CLAUDE_TOKEN_PRIVATE"))
	p := loadSavedProfiles(t, cfgPath)["private"]
	require.NotNil(t, p.Auth)
	require.NotNil(t, p.Auth.OAuthToken)
	assert.Equal(t, "CLAUDE_TOKEN_PRIVATE", p.Auth.OAuthToken.Env)
	assert.Nil(t, p.Auth.APIKey)
	assert.False(t, p.Auth.Login)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "oauth_token:")
	assert.Contains(t, string(data), "env: CLAUDE_TOKEN_PRIVATE")
}

func TestProfileAdd_OAuthTokenFileWithAPIKeyRejected(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	err := runProfileCmd(t, cfgPath, "add", "both", "--harness", "claude",
		"--oauth-token-file", "/tmp/token", "--api-key-env", "K")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the others can be")
	assert.Empty(t, loadSavedProfiles(t, cfgPath))
}

func TestProfileAdd_OAuthTokenForCodexFailsValidation(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())

	err := runProfileCmd(t, cfgPath, "add", "team", "--harness", "codex", "--oauth-token-env", "T")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.oauth_token requires harness claude and backend anthropic")
	assert.Empty(t, loadSavedProfiles(t, cfgPath))
}

func TestProfileListAndShow_OAuthToken(t *testing.T) {
	cfgPath := writeMinimalConfig(t, t.TempDir())
	require.NoError(t, runProfileCmd(t, cfgPath, "add", "private", "--harness", "claude", "--oauth-token-env", "CLAUDE_TOKEN_PRIVATE"))

	t.Setenv("CLAUDE_TOKEN_PRIVATE", "sk-ant-oat-NEVER-PRINTED")

	gf := &GlobalFlags{ConfigFile: cfgPath}
	list := captureProfileStdout(t, func() { require.NoError(t, runProfileList(gf)) })
	assert.Contains(t, list, "token:env:CLAUDE_TOKEN_PRIVATE")

	show := captureProfileStdout(t, func() { require.NoError(t, runProfileShow("private", gf)) })
	assert.Contains(t, show, "token:env:CLAUDE_TOKEN_PRIVATE")
	assert.NotContains(t, show, "sk-ant-oat-NEVER-PRINTED")
	assert.NotContains(t, list, "sk-ant-oat-NEVER-PRINTED")
}

func TestProfileAdd_AvoidsDerivedColorOfExistingProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`profiles:
  work:
    harness: claude
    auth:
      api_key: {env: K}
`), 0o644))

	require.NoError(t, runProfileCmd(t, cfgPath, "add", "second", "--harness", "claude", "--api-key-env", "K2"))
	saved := loadSavedProfiles(t, cfgPath)
	assert.False(t, profile.Similar(saved["second"].Color, profile.Derive("work")),
		"picked %s collides with the derived color of work (%s)", saved["second"].Color, profile.Derive("work"))
	assert.Empty(t, saved["work"].Color, "add must not write colors into existing profiles")
}

func TestProfileAdd_RefusesWhenExistingColorInvalid(t *testing.T) {
	// A hand-edited invalid color on an existing profile fails cross-profile
	// validation, so add writes nothing rather than picking around it.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	original := []byte("profiles:\n  work:\n    harness: claude\n    auth:\n      api_key: {env: K}\n    color: \"red\"\n")
	require.NoError(t, os.WriteFile(cfgPath, original, 0o644))

	err := runProfileCmd(t, cfgPath, "add", "second", "--harness", "claude", "--api-key-env", "K2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `profile "work": color must be #RRGGBB`)
	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, after)
}
