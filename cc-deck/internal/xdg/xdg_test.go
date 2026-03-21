package xdg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigHome_Default(t *testing.T) {
	home, _ := os.UserHomeDir()
	// When XDG_CONFIG_HOME is not set, should use ~/.config
	result := envOrDefault("XDG_CONFIG_HOME_TEST_UNSET", ".config")
	assert.Equal(t, filepath.Join(home, ".config"), result)
}

func TestStateHome_Default(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := envOrDefault("XDG_STATE_HOME_TEST_UNSET", ".local", "state")
	assert.Equal(t, filepath.Join(home, ".local", "state"), result)
}

func TestDataHome_Default(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := envOrDefault("XDG_DATA_HOME_TEST_UNSET", ".local", "share")
	assert.Equal(t, filepath.Join(home, ".local", "share"), result)
}

func TestCacheHome_Default(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := envOrDefault("XDG_CACHE_HOME_TEST_UNSET", ".cache")
	assert.Equal(t, filepath.Join(home, ".cache"), result)
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("XDG_TEST_OVERRIDE", "/custom/path")
	result := envOrDefault("XDG_TEST_OVERRIDE", ".config")
	assert.Equal(t, "/custom/path", result)
}

func TestEnvOverride_EmptyFallsToDefault(t *testing.T) {
	t.Setenv("XDG_TEST_EMPTY", "")
	home, _ := os.UserHomeDir()
	result := envOrDefault("XDG_TEST_EMPTY", ".config")
	assert.Equal(t, filepath.Join(home, ".config"), result)
}

func TestPackageVarsAreSet(t *testing.T) {
	// Verify the package-level vars are non-empty
	assert.NotEmpty(t, ConfigHome, "ConfigHome should not be empty")
	assert.NotEmpty(t, StateHome, "StateHome should not be empty")
	assert.NotEmpty(t, DataHome, "DataHome should not be empty")
	assert.NotEmpty(t, CacheHome, "CacheHome should not be empty")
}

func TestPathsUseLinuxConvention(t *testing.T) {
	home, _ := os.UserHomeDir()
	// Unless env vars are set, paths should use ~/.config, ~/.local, ~/.cache
	// (not ~/Library/Application Support on macOS)
	if os.Getenv("XDG_CONFIG_HOME") == "" {
		assert.Equal(t, filepath.Join(home, ".config"), ConfigHome)
	}
	if os.Getenv("XDG_STATE_HOME") == "" {
		assert.Equal(t, filepath.Join(home, ".local", "state"), StateHome)
	}
}
