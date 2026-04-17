package env

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore is defined in state_test.go and shared across test files.

func TestLocalEnvironment_Type(t *testing.T) {
	store := newTestStore(t)
	env := &LocalEnvironment{name: "test", store: store}
	assert.Equal(t, EnvironmentTypeLocal, env.Type())
}

func TestLocalEnvironment_Name(t *testing.T) {
	store := newTestStore(t)
	env := &LocalEnvironment{name: "my-project", store: store}
	assert.Equal(t, "my-project", env.Name())
}

func TestLocalEnvironment_CreateAddsInstance(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalEnvironment{name: "test-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	inst, err := store.FindInstanceByName("test-env")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentTypeLocal, inst.Type)
	assert.Equal(t, EnvironmentStateRunning, inst.State)
}

func TestLocalEnvironment_CreateRejectsDuplicate(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalEnvironment{name: "dup-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	err = env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestLocalEnvironment_CreateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	env := &LocalEnvironment{name: "INVALID", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidName))
}

func TestLocalEnvironment_ExecReturnsNotSupported(t *testing.T) {
	store := newTestStore(t)
	env := &LocalEnvironment{name: "test", store: store}

	err := env.Exec(context.Background(), []string{"ls"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotSupported))
}

func TestLocalEnvironment_DeleteRemovesInstance(t *testing.T) {
	store := newTestStore(t)

	// Manually add an instance (bypassing Create to avoid zellij dependency).
	inst := &EnvironmentInstance{
		Name:  "del-env",
		Type:  EnvironmentTypeLocal,
		State: EnvironmentStateUnknown,
	}
	require.NoError(t, store.AddInstance(inst))

	env := &LocalEnvironment{name: "del-env", store: store}
	err := env.Delete(context.Background(), true)
	require.NoError(t, err)

	_, err = store.FindInstanceByName("del-env")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestNewEnvironment_Local(t *testing.T) {
	store := newTestStore(t)
	env, err := NewEnvironment(EnvironmentTypeLocal, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, EnvironmentTypeLocal, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewEnvironment_Container(t *testing.T) {
	store := newTestStore(t)
	env, err := NewEnvironment(EnvironmentTypeContainer, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, EnvironmentTypeContainer, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewEnvironment_UnimplementedType(t *testing.T) {
	store := newTestStore(t)
	_, err := NewEnvironment("k8s-sandbox", "test", store, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestNewEnvironment_K8sDeploy(t *testing.T) {
	store := newTestStore(t)
	e, err := NewEnvironment("k8s-deploy", "test", store, nil)
	require.NoError(t, err)
	assert.Equal(t, EnvironmentTypeK8sDeploy, e.Type())
	assert.Equal(t, "test", e.Name())
}
