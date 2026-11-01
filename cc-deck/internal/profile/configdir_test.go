package profile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrepareSharedDir_MissingDefaultDir verifies that a missing defaultDir
// is treated as "nothing to share": configDir is still created, but no
// error or warnings are produced.
func TestPrepareSharedDir_MissingDefaultDir(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "does-not-exist")

	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	info, statErr := os.Stat(configDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
}

// TestPrepareSharedDir_SymlinksNonIsolatedEntries verifies that entries in
// defaultDir that are not isolated get symlinked into configDir, while
// isolated entries are skipped entirely.
func TestPrepareSharedDir_SymlinksNonIsolatedEntries(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "shared.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "secret.json"), []byte("{}"), 0644))

	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, []string{"secret.json"})
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// The non-isolated entry is symlinked.
	sharedLink := filepath.Join(configDir, "shared.json")
	target, readErr := os.Readlink(sharedLink)
	require.NoError(t, readErr)
	assert.Equal(t, filepath.Join("..", "default", "shared.json"), target)

	// The isolated entry is never created.
	_, statErr := os.Lstat(filepath.Join(configDir, "secret.json"))
	assert.True(t, os.IsNotExist(statErr))
}

// TestPrepareSharedDir_IdempotentReRun verifies that running PrepareSharedDir
// twice with the same arguments does not error and leaves the correct
// symlink in place (no repointing, no duplication).
func TestPrepareSharedDir_IdempotentReRun(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "settings.json"), []byte("{}"), 0644))

	_, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)

	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	target, readErr := os.Readlink(filepath.Join(configDir, "settings.json"))
	require.NoError(t, readErr)
	assert.Equal(t, filepath.Join("..", "default", "settings.json"), target)
}

// TestPrepareSharedDir_RemovesLinkForNewlyIsolatedEntry verifies that an
// entry linked by an earlier sync, which has since become isolated, has its
// symlink removed (but the target in defaultDir is left untouched).
func TestPrepareSharedDir_RemovesLinkForNewlyIsolatedEntry(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "creds.json"), []byte("{}"), 0644))

	// First sync: not yet isolated, so it gets linked.
	_, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	_, readErr := os.Readlink(filepath.Join(configDir, "creds.json"))
	require.NoError(t, readErr)

	// Second sync: now isolated, the link must be removed.
	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, []string{"creds.json"})
	require.NoError(t, err)
	assert.Empty(t, warnings)

	_, statErr := os.Lstat(filepath.Join(configDir, "creds.json"))
	assert.True(t, os.IsNotExist(statErr))

	// The target in defaultDir must still exist.
	_, targetErr := os.Stat(filepath.Join(defaultDir, "creds.json"))
	assert.NoError(t, targetErr)
}

// TestPrepareSharedDir_RepointsStaleSymlink verifies that a symlink pointing
// at a different, incorrect target is removed and recreated to point at the
// expected target.
func TestPrepareSharedDir_RepointsStaleSymlink(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "settings.json"), []byte("{}"), 0644))

	stalePath := filepath.Join(configDir, "settings.json")
	require.NoError(t, os.Symlink("/some/stale/target", stalePath))

	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	target, readErr := os.Readlink(stalePath)
	require.NoError(t, readErr)
	assert.Equal(t, filepath.Join("..", "default", "settings.json"), target)
}

// TestPrepareSharedDir_WarnsOnRegularFile verifies that a real (non-symlink)
// file already present at the link path is left untouched and produces a
// warning instead of an error.
func TestPrepareSharedDir_WarnsOnRegularFile(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	defaultDir := filepath.Join(root, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "settings.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("real"), 0644))

	warnings, err := profile.PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "settings.json")
	assert.Contains(t, warnings[0], "not a symlink")

	content, readErr := os.ReadFile(filepath.Join(configDir, "settings.json"))
	require.NoError(t, readErr)
	assert.Equal(t, "real", string(content))
}
