package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/env"
)

func TestRunEnvPrune_RemovesStaleEntry(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

	store := env.NewStateStore(stateFile)

	// Register a real project and a fake one.
	realDir := filepath.Join(tmpDir, "real-project")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	require.NoError(t, store.RegisterProject(realDir))

	fakeDir := filepath.Join(tmpDir, "deleted-project")
	require.NoError(t, os.MkdirAll(fakeDir, 0o755))
	require.NoError(t, store.RegisterProject(fakeDir))

	// Remove the fake project directory.
	require.NoError(t, os.RemoveAll(fakeDir))

	err := runEnvPrune()
	require.NoError(t, err)

	// Verify only the real project remains.
	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)
	// Path may be symlink-resolved (e.g., /private/var on macOS).
	assert.Contains(t, projects[0].Path, filepath.Base(realDir))
}

func TestRunEnvPrune_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

	// No projects registered.
	err := runEnvPrune()
	require.NoError(t, err)

	// Run again, still no error.
	err = runEnvPrune()
	require.NoError(t, err)
}
