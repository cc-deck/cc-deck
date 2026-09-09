package profile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
)

// A Vertex profile may source both the API key and the ADC credentials from
// files. All credential checks render before all exports, so the two file
// blocks must not share a shell variable or the API key export reads the
// credentials file.
func TestClaudeRender_VertexBothFileSourcesUseDistinctVariables(t *testing.T) {
	a := agent.Get("claude")
	require.NotNil(t, a)
	tr, ok := profile.Lookup("claude")
	require.True(t, ok)

	rp := profile.ResolvedProfile{
		Name:        "work",
		Harness:     a,
		Backend:     config.BackendVertex,
		Project:     "acme-ml",
		Region:      "us-east5",
		APIKey:      &config.CredentialSource{File: "/host/api-key"},
		Credentials: &config.CredentialSource{File: "/host/adc.json"},
		BinDir:      "/tmp/test-bin",
	}

	ws, err := tr.Render(rp)
	require.NoError(t, err)
	script := string(ws.Content)

	assert.Contains(t, script, `_f_apikey="$HOME/.config/cc-deck/profiles/work/api_key"`)
	assert.Contains(t, script, `_f_creds="$HOME/.config/cc-deck/profiles/work/credentials"`)
	assert.Contains(t, script, `export ANTHROPIC_API_KEY="$(cat "$_f_apikey")"`)
	assert.Contains(t, script, `export GOOGLE_APPLICATION_CREDENTIALS="$_f_creds"`)
	assert.NotContains(t, script, `_f="`, "the shared _f variable must not be used")
}

// The profile name is single-quoted in the generated script so the export
// never depends on the name validation upstream.
func TestRender_ProfileNameIsQuoted(t *testing.T) {
	a := agent.Get("claude")
	require.NotNil(t, a)
	tr, ok := profile.Lookup("claude")
	require.True(t, ok)

	ws, err := tr.Render(profile.ResolvedProfile{Name: "work", Harness: a, Backend: config.BackendAnthropic, Login: true, BinDir: "/tmp/b"})
	require.NoError(t, err)
	lines := strings.Split(string(ws.Content), "\n")
	assert.Equal(t, "export CC_DECK_PROFILE='work'", lines[2])
}

// Codex keeps config.toml isolated per profile, so the default file must be
// carried over with the model overlaid, and a removed model must not linger.
func TestCodexPrepareConfigDir_OverlaysModelOnDefaultConfig(t *testing.T) {
	a := agent.Get("codex")
	require.NotNil(t, a)
	tr, ok := profile.Lookup("codex")
	require.True(t, ok)

	defaultDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "config.toml"), []byte(
		"model = \"old-model\"\napproval_policy = \"never\"\n\n[sandbox]\nmode = \"workspace-write\"\n"), 0o644))

	rp := profile.ResolvedProfile{
		Name:      "team",
		Harness:   a,
		Backend:   config.BackendOpenAI,
		Model:     "gpt-5",
		ConfigDir: filepath.Join(t.TempDir(), "codex"),
		BinDir:    "/tmp/b",
	}

	_, err := tr.PrepareConfigDir(rp, defaultDir)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(rp.ConfigDir, "config.toml"))
	require.NoError(t, err)
	content := string(got)
	assert.True(t, strings.HasPrefix(content, "model = \"gpt-5\"\n"), "profile model must be the top-level model:\n%s", content)
	assert.NotContains(t, content, "old-model")
	assert.Contains(t, content, "approval_policy = \"never\"")
	assert.Contains(t, content, "[sandbox]\nmode = \"workspace-write\"")

	// Removing the model from the profile clears it from the per-profile file
	// on the next sync while the inherited settings survive.
	rp.Model = ""
	_, err = tr.PrepareConfigDir(rp, defaultDir)
	require.NoError(t, err)
	got, err = os.ReadFile(filepath.Join(rp.ConfigDir, "config.toml"))
	require.NoError(t, err)
	content = string(got)
	assert.NotContains(t, content, "model =")
	assert.Contains(t, content, "approval_policy = \"never\"")
}

// A hand-edited config.yaml can carry names or credential references that
// the CLI would have rejected. Sync must skip those profiles instead of
// feeding them to the shell generator.
func TestSync_SkipsProfilesFailingValidation(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("GOOD_KEY", "secret-123")
	t.Setenv("BAD_KEY", "secret-456")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"good":      simpleClaudeProfile("GOOD_KEY"),
			"bad$(name)": simpleClaudeProfile("BAD_KEY"),
			"badenv": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: `X"; echo pwned; #`}},
			},
		},
	}

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	assert.Contains(t, result.Written, "claude-good")
	for _, w := range result.Written {
		assert.NotContains(t, w, "bad", "invalid profiles must not produce wrappers: %s", w)
	}

	skipped := map[string]string{}
	for _, s := range result.Skipped {
		skipped[s.Profile] = s.Reason
	}
	assert.Contains(t, skipped["bad$(name)"], "config validation")
	assert.Contains(t, skipped["badenv"], "config validation")
	assert.Contains(t, skipped["badenv"], "not a valid variable name")
}
