package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/ws"
)

func TestRunWsUpdate_SyncRepos_NoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "no-repos",
		Type: ws.WorkspaceTypeSSH,
	}))

	store := ws.NewStateStore(stateFile)
	require.NoError(t, store.AddInstance(&ws.WorkspaceInstance{
		Name:  "no-repos",
		Type:  ws.WorkspaceTypeSSH,
		State: ws.WorkspaceStateRunning,
		SSH:   &ws.SSHFields{Host: "user@host"},
	}))

	err := runWsUpdate("no-repos", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no repos defined in workspace definition")
}

func TestRunWsUpdateAuthMode_Success(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "switch-ws",
		Type: ws.WorkspaceTypeContainer,
		WorkspaceSpec: ws.WorkspaceSpec{
			Agent:    "claude",
			AuthMode: "vertex",
		},
	}))
	err := runWsUpdateAuthMode("switch-ws", "api")
	require.NoError(t, err)
	updated, _ := defs.FindByName("switch-ws")
	assert.Equal(t, "api", updated.AuthMode)
}

func TestRunWsUpdateAuthMode_MissingCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "fail-ws",
		Type: ws.WorkspaceTypeContainer,
		WorkspaceSpec: ws.WorkspaceSpec{Agent: "claude", AuthMode: "vertex"},
	}))
	err := runWsUpdateAuthMode("fail-ws", "api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing credentials")
}

func TestRunWsUpdateAuthMode_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "invalid-ws",
		Type: ws.WorkspaceTypeContainer,
		WorkspaceSpec: ws.WorkspaceSpec{Agent: "claude", AuthMode: "api"},
	}))
	err := runWsUpdateAuthMode("invalid-ws", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunWsUpdate_SyncRepos_WithRepos(t *testing.T) {
	tmpDir := t.TempDir()

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "with-repos",
		Type: ws.WorkspaceTypeSSH,
		WorkspaceSpec: ws.WorkspaceSpec{
			Host: "user@host",
			Repos: []ws.RepoEntry{
				{URL: "https://github.com/org/repo.git"},
			},
		},
	}))

	store := ws.NewStateStore(stateFile)
	require.NoError(t, store.AddInstance(&ws.WorkspaceInstance{
		Name:  "with-repos",
		Type:  ws.WorkspaceTypeSSH,
		State: ws.WorkspaceStateRunning,
		SSH:   &ws.SSHFields{Host: "user@host"},
	}))

	// SyncRepos will fail due to no SSH connection, but the definition lookup
	// should succeed (the error will be from the SSH operation, not "no repos").
	err := runWsUpdate("with-repos", true)
	if err != nil {
		assert.NotContains(t, err.Error(), "no repos defined")
	}
}
