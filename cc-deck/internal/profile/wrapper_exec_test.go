package profile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
)

// execWrapper renders the Claude wrapper for p, runs it with a stub claude
// binary that prints its environment, and returns stdout, stderr and the
// exit code. env is the complete environment of the wrapper process.
func execWrapper(t *testing.T, name string, p config.Profile, home string, env []string) (string, string, int) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}

	a := agent.Get("claude")
	require.NotNil(t, a)
	tr, ok := profile.Lookup("claude")
	require.True(t, ok)
	rp, err := profile.Resolve(name, p, a)
	require.NoError(t, err)
	ws, err := tr.Render(rp)
	require.NoError(t, err)

	dir := t.TempDir()
	wrapper := filepath.Join(dir, ws.Name)
	require.NoError(t, os.WriteFile(wrapper, ws.Content, 0o755))
	stubDir := filepath.Join(dir, "stub")
	require.NoError(t, os.MkdirAll(stubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stubDir, "claude"), []byte("#!/bin/sh\necho STUB_STARTED\nenv\n"), 0o755))

	cmd := exec.Command(sh, wrapper, "--resume", "abc")
	cmd.Env = append([]string{"HOME=" + home, "PATH=" + stubDir + ":/usr/bin:/bin"}, env...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return stdout.String(), stderr.String(), code
}

// envLines splits env output into a set of KEY=VALUE lines.
func envLines(out string) map[string]bool {
	set := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		set[l] = true
	}
	return set
}

func hasKey(out, key string) bool {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, key+"=") {
			return true
		}
	}
	return false
}

func tokenProfile(src config.CredentialSource) config.Profile {
	return config.Profile{
		Harness: "claude",
		Color:   "#E27C5A",
		Auth:    &config.AuthConfig{OAuthToken: &src},
	}
}

func TestWrapperExec_TokenProfileClearsInheritedCredentials(t *testing.T) {
	// Acceptance 1a and FR-007a: a stray API key, Vertex switch or icon in
	// the calling shell does not reach the harness.
	out, stderr, code := execWrapper(t, "private", tokenProfile(config.CredentialSource{Env: "CLAUDE_TOKEN_PRIVATE"}), t.TempDir(), []string{
		"CLAUDE_TOKEN_PRIVATE=tok-123",
		"ANTHROPIC_API_KEY=leak",
		"ANTHROPIC_AUTH_TOKEN=leak",
		"CLAUDE_CODE_USE_VERTEX=1",
		"CC_DECK_PROFILE_ICON=Z",
	})
	require.Equal(t, 0, code, "stderr: %s", stderr)

	lines := envLines(out)
	assert.True(t, lines["STUB_STARTED"])
	assert.True(t, lines["CLAUDE_CODE_OAUTH_TOKEN=tok-123"])
	assert.True(t, lines["CC_DECK_PROFILE=private"])
	assert.True(t, lines["CC_DECK_PROFILE_COLOR=#E27C5A"])
	assert.True(t, lines["CC_DECK_PROFILE_ICON="], "inherited icon must be replaced by the empty export")
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_USE_VERTEX"} {
		assert.False(t, hasKey(out, k), "%s must be cleared", k)
	}
}

func TestWrapperExec_MissingTokenExitsBeforeHarness(t *testing.T) {
	// FR-013: exit 1, name profile and variable, never start the harness.
	out, stderr, code := execWrapper(t, "private", tokenProfile(config.CredentialSource{Env: "CLAUDE_TOKEN_PRIVATE"}), t.TempDir(), nil)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "cc-deck profile 'private'")
	assert.Contains(t, stderr, "CLAUDE_TOKEN_PRIVATE")
	assert.NotContains(t, out, "STUB_STARTED")
}

func TestWrapperExec_TokenFileExportsContentWithoutNewline(t *testing.T) {
	home := t.TempDir()
	credDir := filepath.Join(home, ".config", "cc-deck", "profiles", "claude", "private")
	require.NoError(t, os.MkdirAll(credDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, "oauth_token"), []byte("tok-file\n"), 0o600))

	out, stderr, code := execWrapper(t, "private", tokenProfile(config.CredentialSource{File: "~/secret-token"}), home, nil)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.True(t, envLines(out)["CLAUDE_CODE_OAUTH_TOKEN=tok-file"], "command substitution strips the trailing newline")
}

func TestWrapperExec_MissingTokenFileExits(t *testing.T) {
	out, stderr, code := execWrapper(t, "private", tokenProfile(config.CredentialSource{File: "~/secret-token"}), t.TempDir(), nil)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "cc-deck profile 'private'")
	assert.NotContains(t, out, "STUB_STARTED")
}

func TestWrapperExec_LoginProfileClearsAllAuthVars(t *testing.T) {
	// A login profile owns no auth variable, so every inherited one is
	// cleared and the harness still starts.
	out, _, code := execWrapper(t, "home", config.Profile{Harness: "claude", Color: "#7CE25A", Auth: &config.AuthConfig{Login: true}}, t.TempDir(), []string{
		"ANTHROPIC_API_KEY=leak", "ANTHROPIC_AUTH_TOKEN=leak", "CLAUDE_CODE_OAUTH_TOKEN=leak", "CLAUDE_CODE_USE_VERTEX=1", "CLAUDE_CODE_USE_BEDROCK=1",
	})
	require.Equal(t, 0, code)
	assert.True(t, envLines(out)["STUB_STARTED"])
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_BEDROCK"} {
		assert.False(t, hasKey(out, k))
	}
}
