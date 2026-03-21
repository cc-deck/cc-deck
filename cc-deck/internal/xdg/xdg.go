// Package xdg provides XDG Base Directory paths using the Linux convention
// on all platforms (including macOS). Environment variable overrides are
// respected when set.
package xdg

import (
	"os"
	"path/filepath"
)

var (
	// ConfigHome is $XDG_CONFIG_HOME or ~/.config.
	ConfigHome = envOrDefault("XDG_CONFIG_HOME", ".config")

	// StateHome is $XDG_STATE_HOME or ~/.local/state.
	StateHome = envOrDefault("XDG_STATE_HOME", ".local", "state")

	// DataHome is $XDG_DATA_HOME or ~/.local/share.
	DataHome = envOrDefault("XDG_DATA_HOME", ".local", "share")

	// CacheHome is $XDG_CACHE_HOME or ~/.cache.
	CacheHome = envOrDefault("XDG_CACHE_HOME", ".cache")
)

func envOrDefault(envVar string, subPaths ...string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(append([]string{home}, subPaths...)...)
}
