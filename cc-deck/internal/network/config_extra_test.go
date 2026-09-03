package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

func TestUserConfigPath_UsesXDGConfigHome(t *testing.T) {
	orig := xdg.ConfigHome
	xdg.ConfigHome = "/tmp/xdg-config-test"
	defer func() { xdg.ConfigHome = orig }()

	got := UserConfigPath()
	want := filepath.Join("/tmp/xdg-config-test", "cc-deck", "domains.yaml")
	assert.Equal(t, want, got)
}

func TestLoadUserConfig_MissingFile(t *testing.T) {
	orig := xdg.ConfigHome
	xdg.ConfigHome = t.TempDir()
	defer func() { xdg.ConfigHome = orig }()

	groups, err := LoadUserConfig()
	require.NoError(t, err)
	assert.Nil(t, groups)
}

func TestLoadUserConfig_ValidFile(t *testing.T) {
	orig := xdg.ConfigHome
	dir := t.TempDir()
	xdg.ConfigHome = dir
	defer func() { xdg.ConfigHome = orig }()

	confDir := filepath.Join(dir, "cc-deck")
	require.NoError(t, os.MkdirAll(confDir, 0o755))
	content := []byte("custom:\n  domains:\n    - example.internal.corp\n")
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "domains.yaml"), content, 0o644))

	groups, err := LoadUserConfig()
	require.NoError(t, err)
	require.NotNil(t, groups)

	custom, ok := groups["custom"]
	require.True(t, ok)
	assert.Contains(t, custom.Domains, "example.internal.corp")
}

func TestLoadUserConfigFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml"), 0o644))

	groups, err := LoadUserConfigFrom(path)
	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestUnknownGroupError_Error(t *testing.T) {
	err := &UnknownGroupError{
		Name:      "bogus",
		Available: []string{"python", "golang"},
	}
	msg := err.Error()
	assert.Contains(t, msg, "bogus")
	assert.Contains(t, msg, "python")
	assert.Contains(t, msg, "golang")
}
