package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/env"
)

func TestResolveEnvironmentName_FromSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create project-local definition.
	def := &env.EnvironmentDefinition{Name: "deep-project", Type: env.EnvironmentTypeLocal}
	require.NoError(t, env.SaveProjectDefinition(tmpDir, def))

	// Create a deep subdirectory.
	subDir := filepath.Join(tmpDir, "src", "pkg", "internal")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(subDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

	store := env.NewStateStore(stateFile)
	name, _, err := resolveEnvironmentName(nil, store)
	require.NoError(t, err)
	assert.Equal(t, "deep-project", name)

	// Verify project was auto-registered.
	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)
}

func TestResolveEnvironmentName_FailsWithoutConfig(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	store := env.NewStateStore(filepath.Join(tmpDir, "state.yaml"))
	_, _, err := resolveEnvironmentName(nil, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no environment name specified")
}

func TestResolveEnvironmentName_ExplicitNameTakesPrecedence(t *testing.T) {
	store := env.NewStateStore(filepath.Join(t.TempDir(), "state.yaml"))
	name, _, err := resolveEnvironmentName([]string{"explicit-name"}, store)
	require.NoError(t, err)
	assert.Equal(t, "explicit-name", name)
}
