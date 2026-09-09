package profile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// setupTestEnv creates an isolated temp directory structure and overrides
// xdg package-level vars, HOME, and PATH so that Sync never touches the
// real filesystem. Returns the fake home directory path.
func setupTestEnv(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	dataHome := filepath.Join(tmp, "data")
	configHome := filepath.Join(tmp, "config")

	require.NoError(t, os.MkdirAll(home, 0o755))

	// Override the xdg package-level variables so every path
	// computed by Sync and Resolve lands inside our temp tree.
	origDataHome := xdg.DataHome
	origConfigHome := xdg.ConfigHome
	xdg.DataHome = dataHome
	xdg.ConfigHome = configHome
	t.Cleanup(func() {
		xdg.DataHome = origDataHome
		xdg.ConfigHome = origConfigHome
	})

	// Point HOME at the temp dir so agent.DetectConfig() does not
	// find (or touch) the real user's config directories.
	t.Setenv("HOME", home)

	// Create fake harness binaries so agent.IsInstalled() returns true.
	fakeBin := filepath.Join(tmp, "fakebin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))
	for _, name := range []string{"claude", "codex", "opencode"} {
		require.NoError(t, os.WriteFile(
			filepath.Join(fakeBin, name),
			[]byte("#!/bin/sh\n"),
			0o755,
		))
	}
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))

	return home
}

// binDir returns the wrapper output directory for the current xdg.DataHome.
func binDir() string {
	return filepath.Join(xdg.DataHome, "cc-deck", "bin")
}

// simpleClaudeProfile returns a minimal claude profile with an env-sourced API key.
func simpleClaudeProfile(envVar string) config.Profile {
	return config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{Env: envVar},
		},
	}
}

// ---------- Test cases ----------

func TestSync_FirstRunWritesWrappers(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("TEST_API_KEY", "secret-123")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": simpleClaudeProfile("TEST_API_KEY"),
		},
	}

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	assert.Contains(t, result.Written, "claude-work",
		"first sync must write the wrapper")

	_, statErr := os.Stat(filepath.Join(binDir(), "claude-work"))
	assert.NoError(t, statErr, "wrapper file must exist on disk")
}

func TestSync_SecondRunIsNoOp(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("TEST_API_KEY", "secret-123")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": simpleClaudeProfile("TEST_API_KEY"),
		},
	}

	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	assert.Empty(t, result.Written,
		"second sync must not rewrite unchanged wrappers")
	assert.Empty(t, result.Removed,
		"second sync must not remove anything")
}

func TestSync_DeletedProfileRemovesWrapper(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("KEY_A", "val-a")
	t.Setenv("KEY_B", "val-b")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"alpha": simpleClaudeProfile("KEY_A"),
			"beta":  simpleClaudeProfile("KEY_B"),
		},
	}

	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	// Remove the beta profile.
	delete(cfg.Profiles, "beta")

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	assert.Contains(t, result.Removed, "claude-beta",
		"wrapper for deleted profile must be removed")

	_, statErr := os.Stat(filepath.Join(binDir(), "claude-beta"))
	assert.True(t, os.IsNotExist(statErr),
		"removed wrapper must no longer exist on disk")

	// The surviving profile's wrapper must still be present.
	_, statErr = os.Stat(filepath.Join(binDir(), "claude-alpha"))
	assert.NoError(t, statErr,
		"surviving profile wrapper must still exist")
}

func TestSync_NonCCDeckFileUntouched(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("TEST_API_KEY", "secret-123")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": simpleClaudeProfile("TEST_API_KEY"),
		},
	}

	// First sync creates the bin dir and writes the wrapper.
	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	// Drop a foreign file into the bin dir (no cc-deck marker).
	foreignPath := filepath.Join(binDir(), "my-custom-script")
	require.NoError(t, os.WriteFile(foreignPath, []byte("#!/bin/sh\necho hello\n"), 0o755))

	// Remove the profile so stale cleanup runs.
	delete(cfg.Profiles, "work")

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	assert.Contains(t, result.Removed, "claude-work",
		"cc-deck wrapper must be removed")
	assert.NotContains(t, result.Removed, "my-custom-script",
		"foreign file must not appear in removed list")

	_, statErr := os.Stat(foreignPath)
	assert.NoError(t, statErr,
		"foreign file must still exist on disk")
}

func TestSync_WrapperPassesShellSyntaxCheck(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("TEST_API_KEY", "secret-123")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": simpleClaudeProfile("TEST_API_KEY"),
		},
	}

	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	wrapperPath := filepath.Join(binDir(), "claude-work")
	// sh -n performs a syntax check without executing the script.
	cmd := exec.Command("sh", "-n", wrapperPath)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err,
		"wrapper must pass sh -n syntax check; output: %s", string(out))
}

func TestSync_WrapperContainsNoCredentialValue(t *testing.T) {
	home := setupTestEnv(t)

	secretValue := "super-secret-api-key-value-12345"
	t.Setenv("TEST_API_KEY", secretValue)

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": simpleClaudeProfile("TEST_API_KEY"),
		},
	}

	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(binDir(), "claude-work"))
	require.NoError(t, err)

	assert.NotContains(t, string(content), secretValue,
		"wrapper must reference the env var name, not the credential value")

	// The wrapper should reference the env var by name.
	assert.Contains(t, string(content), "TEST_API_KEY",
		"wrapper must contain the env var name")
}

func TestSync_MissingEnvVarReportedAsWarning(t *testing.T) {
	home := setupTestEnv(t)
	// Deliberately do NOT set MISSING_VAR in the environment.

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"broken": simpleClaudeProfile("MISSING_VAR"),
		},
	}

	result, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	// The wrapper should still be written (credential errors are non-fatal).
	assert.Contains(t, result.Written, "claude-broken",
		"wrapper must still be written despite missing credential")

	// A warning must report the missing variable.
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "MISSING_VAR") {
			found = true
			break
		}
	}
	assert.True(t, found,
		"Warnings must mention the missing env var; got: %v", result.Warnings)
}

func TestSync_NoBareHarnessBinaryInBinDir(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("KEY_A", "val")
	t.Setenv("KEY_B", "val")
	t.Setenv("KEY_C", "val")

	// Create profiles for all three harnesses.
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"cc": simpleClaudeProfile("KEY_A"),
			"cx": {
				Harness: "codex",
				Auth: &config.AuthConfig{
					APIKey: &config.CredentialSource{Env: "KEY_B"},
				},
			},
			"oc": {
				Harness: "opencode",
				Auth: &config.AuthConfig{
					APIKey: &config.CredentialSource{Env: "KEY_C"},
				},
			},
		},
	}

	_, err := profile.Sync(cfg, home)
	require.NoError(t, err)

	// FR-012: no file in the bin dir should be named after a bare harness binary.
	bareNames := []string{"claude", "codex", "opencode"}
	entries, err := os.ReadDir(binDir())
	require.NoError(t, err)

	for _, entry := range entries {
		for _, bare := range bareNames {
			assert.NotEqual(t, bare, entry.Name(),
				"bin dir must never contain a file named %q (FR-012)", bare)
		}
	}
}

func TestSync_TenProfilesUnderTwoSeconds(t *testing.T) {
	home := setupTestEnv(t)

	profiles := make(map[string]config.Profile, 10)
	for i := 0; i < 10; i++ {
		envVar := "PERF_KEY_" + strings.ToUpper(string(rune('A'+i)))
		t.Setenv(envVar, "value")
		name := "profile" + string(rune('a'+i))
		profiles[name] = simpleClaudeProfile(envVar)
	}

	cfg := &config.Config{Profiles: profiles}

	start := time.Now()
	result, err := profile.Sync(cfg, home)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, result.Written, 10,
		"all 10 wrappers must be written")
	assert.Less(t, elapsed, 2*time.Second,
		"syncing 10 profiles must complete in under 2 seconds; took %s", elapsed)
}
