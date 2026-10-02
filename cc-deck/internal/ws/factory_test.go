package ws

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkspace_Compose(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceTypeCompose, "test", store, nil)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, WorkspaceTypeCompose, w.Type())
	_, ok := w.(*ComposeWorkspace)
	assert.True(t, ok)
}

func TestNewWorkspace_SSH(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceTypeSSH, "test", store, nil)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, WorkspaceTypeSSH, w.Type())
	_, ok := w.(*SSHWorkspace)
	assert.True(t, ok)
}

func TestNewWorkspace_OpenShell(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceTypeOpenShell, "test", store, nil)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, WorkspaceTypeOpenShell, w.Type())
	_, ok := w.(*OpenShellWorkspace)
	assert.True(t, ok)
}

func TestNewWorkspace_Unimplemented(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceTypeK8sSandbox, "test", store, nil)
	assert.Nil(t, w)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestNewWorkspace_Unknown(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceType("bogus"), "test", store, nil)
	assert.Nil(t, w)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestNewWorkspace_NilDefsPassedThrough(t *testing.T) {
	store := newTestStore(t)
	w, err := NewWorkspace(WorkspaceTypeContainer, "test", store, nil)
	require.NoError(t, err)
	cw, ok := w.(*ContainerWorkspace)
	require.True(t, ok)
	assert.Nil(t, cw.defs)
}
