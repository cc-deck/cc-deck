package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/ws"
)

func TestResolveWorkspaceName_SingleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "only-ws",
		Type: ws.WorkspaceTypeLocal,
	}))

	store := ws.NewStateStore(stateFile)
	name, _, err := resolveWorkspaceName(nil, store)
	require.NoError(t, err)
	assert.Equal(t, "only-ws", name)
}

func TestResolveWorkspaceName_ProjectDirMatch(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	resolved, _ := filepath.EvalSymlinks(tmpDir)
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name:       "project-ws",
		Type:       ws.WorkspaceTypeLocal,
		ProjectDir: resolved,
	}))
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "other-ws",
		Type: ws.WorkspaceTypeLocal,
	}))

	store := ws.NewStateStore(stateFile)
	name, _, err := resolveWorkspaceName(nil, store)
	require.NoError(t, err)
	assert.Equal(t, "project-ws", name)
}

func TestResolveWorkspaceName_RecencySelection(t *testing.T) {
	tmpDir := t.TempDir()

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{Name: "ws-old", Type: ws.WorkspaceTypeLocal}))
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{Name: "ws-new", Type: ws.WorkspaceTypeLocal}))

	store := ws.NewStateStore(stateFile)
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	require.NoError(t, store.AddInstance(&ws.WorkspaceInstance{
		Name:         "ws-old",
		Type:         ws.WorkspaceTypeLocal,
		State:        ws.WorkspaceStateRunning,
		CreatedAt:    oldTime,
		LastAttached: &oldTime,
	}))
	require.NoError(t, store.AddInstance(&ws.WorkspaceInstance{
		Name:         "ws-new",
		Type:         ws.WorkspaceTypeLocal,
		State:        ws.WorkspaceStateRunning,
		CreatedAt:    newTime,
		LastAttached: &newTime,
	}))

	name, _, err := resolveWorkspaceName(nil, store)
	require.NoError(t, err)
	assert.Equal(t, "ws-new", name)
}

func TestResolveWorkspaceName_FailsWithNoWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	store := ws.NewStateStore(stateFile)
	_, _, err := resolveWorkspaceName(nil, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspaces found")
}

func TestResolveWorkspaceName_ExplicitNameTakesPrecedence(t *testing.T) {
	store := ws.NewStateStore(filepath.Join(t.TempDir(), "state.yaml"))
	name, _, err := resolveWorkspaceName([]string{"explicit-name"}, store)
	require.NoError(t, err)
	assert.Equal(t, "explicit-name", name)
}
