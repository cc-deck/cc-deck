package ws

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore is defined in state_test.go and shared across test files.

func TestLocalWorkspace_Type(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "test", store: store}
	assert.Equal(t, WorkspaceTypeLocal, env.Type())
}

func TestLocalWorkspace_Name(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "my-project", store: store}
	assert.Equal(t, "my-project", env.Name())
}

func TestLocalWorkspace_CreateAddsInstance(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalWorkspace{name: "test-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	inst, err := store.FindInstanceByName("test-env")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeLocal, inst.Type)
	assert.Equal(t, WorkspaceStateRunning, inst.State)
}

func TestLocalWorkspace_CreateRejectsDuplicate(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalWorkspace{name: "dup-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	err = env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestLocalWorkspace_CreateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "INVALID", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidName))
}

func TestLocalWorkspace_ExecReturnsNotSupported(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "test", store: store}

	err := env.Exec(context.Background(), []string{"ls"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotSupported))
}

func TestLocalWorkspace_DeleteRemovesInstance(t *testing.T) {
	store := newTestStore(t)

	// Manually add an instance (bypassing Create to avoid zellij dependency).
	inst := &WorkspaceInstance{
		Name:  "del-env",
		Type:  WorkspaceTypeLocal,
		State: WorkspaceStateUnknown,
	}
	require.NoError(t, store.AddInstance(inst))

	env := &LocalWorkspace{name: "del-env", store: store}
	err := env.Delete(context.Background(), true)
	require.NoError(t, err)

	_, err = store.FindInstanceByName("del-env")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestNewWorkspace_Local(t *testing.T) {
	store := newTestStore(t)
	env, err := NewWorkspace(WorkspaceTypeLocal, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, WorkspaceTypeLocal, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewWorkspace_Container(t *testing.T) {
	store := newTestStore(t)
	env, err := NewWorkspace(WorkspaceTypeContainer, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, WorkspaceTypeContainer, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewWorkspace_UnimplementedType(t *testing.T) {
	store := newTestStore(t)
	_, err := NewWorkspace("k8s-sandbox", "test", store, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestNewWorkspace_K8sDeploy(t *testing.T) {
	store := newTestStore(t)
	e, err := NewWorkspace("k8s-deploy", "test", store, nil)
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeK8sDeploy, e.Type())
	assert.Equal(t, "test", e.Name())
}
