package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/ws"
)

func TestResolveWorkspaceName_FromSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create project-local definition.
	def := &ws.WorkspaceDefinition{Name: "deep-project", Type: ws.WorkspaceTypeLocal}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, def))

	// Create a deep subdirectory.
	subDir := filepath.Join(tmpDir, "src", "pkg", "internal")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(subDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

	store := ws.NewStateStore(stateFile)
	name, _, err := resolveWorkspaceName(nil, store)
	require.NoError(t, err)
	assert.Equal(t, "deep-project", name)

	// Verify project was auto-registered.
	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)
}

func TestResolveWorkspaceName_FailsWithoutConfig(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	store := ws.NewStateStore(filepath.Join(tmpDir, "state.yaml"))
	_, _, err := resolveWorkspaceName(nil, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspace name specified")
}

func TestResolveWorkspaceName_ExplicitNameTakesPrecedence(t *testing.T) {
	store := ws.NewStateStore(filepath.Join(t.TempDir(), "state.yaml"))
	name, _, err := resolveWorkspaceName([]string{"explicit-name"}, store)
	require.NoError(t, err)
	assert.Equal(t, "explicit-name", name)
}
