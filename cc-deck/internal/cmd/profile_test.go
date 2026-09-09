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

func captureStdout(t *testing.T, fn func()) string {
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

	output := captureStdout(t, func() {
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

	output := captureStdout(t, func() {
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

	captureStdout(t, func() {
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
