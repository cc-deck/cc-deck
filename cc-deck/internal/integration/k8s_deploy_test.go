//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/env"
)

const (
	testNamespace = "cc-deck-test"
	testImage     = "localhost/cc-deck-stub:latest"
)

func TestK8sDeployLifecycle(t *testing.T) {
	ctx := context.Background()
	store := env.NewStateStore(t.TempDir() + "/state.yaml")
	defs := env.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	envName := "inttest-lifecycle"

	// Create
	raw, err := env.NewEnvironment(env.EnvironmentTypeK8sDeploy, envName, store, defs)
	require.NoError(t, err)

	ke, ok := raw.(*env.K8sDeployEnvironment)
	require.True(t, ok)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute

	err = ke.Create(ctx, env.CreateOpts{Image: testImage})
	require.NoError(t, err, "Create should succeed")

	// Verify instance recorded.
	inst, err := store.FindInstanceByName(envName)
	require.NoError(t, err)
	assert.Equal(t, env.EnvironmentStateRunning, inst.State)
	assert.Equal(t, testNamespace, inst.K8s.Namespace)

	// Stop
	err = ke.Stop(ctx)
	require.NoError(t, err, "Stop should succeed")

	inst, _ = store.FindInstanceByName(envName)
	assert.Equal(t, env.EnvironmentStateStopped, inst.State)

	// Start
	err = ke.Start(ctx)
	require.NoError(t, err, "Start should succeed")

	inst, _ = store.FindInstanceByName(envName)
	assert.Equal(t, env.EnvironmentStateRunning, inst.State)

	// Status
	status, err := ke.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, env.EnvironmentStateRunning, status.State)

	// Delete
	err = ke.Delete(ctx, true)
	require.NoError(t, err, "Delete should succeed")

	_, err = store.FindInstanceByName(envName)
	assert.Error(t, err, "Instance should be removed after delete")
}

func TestK8sDeployResourceVerification(t *testing.T) {
	ctx := context.Background()
	store := env.NewStateStore(t.TempDir() + "/state.yaml")
	defs := env.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	envName := "inttest-resources"

	raw, err := env.NewEnvironment(env.EnvironmentTypeK8sDeploy, envName, store, defs)
	require.NoError(t, err)

	ke := raw.(*env.K8sDeployEnvironment)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute
	ke.Credentials = map[string]string{"TEST_KEY": "test-value"}

	err = ke.Create(ctx, env.CreateOpts{Image: testImage, Storage: env.StorageConfig{Size: "1Gi"}})
	require.NoError(t, err)

	// Verify K8s resources exist via client.
	client, err := env.NewK8sClient("", "")
	require.NoError(t, err)

	resName := "cc-deck-" + envName

	// Verify StatefulSet exists.
	_, stsErr := client.ReconcileState(ctx, testNamespace, resName)
	assert.NoError(t, stsErr)

	// Cleanup.
	_ = ke.Delete(ctx, true)
}

func TestK8sDeployDuplicateConflict(t *testing.T) {
	ctx := context.Background()
	store := env.NewStateStore(t.TempDir() + "/state.yaml")
	defs := env.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	envName := "inttest-dup"

	// First create.
	raw, err := env.NewEnvironment(env.EnvironmentTypeK8sDeploy, envName, store, defs)
	require.NoError(t, err)
	ke := raw.(*env.K8sDeployEnvironment)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute

	err = ke.Create(ctx, env.CreateOpts{Image: testImage})
	require.NoError(t, err)

	// Second create with same name should fail.
	raw2, _ := env.NewEnvironment(env.EnvironmentTypeK8sDeploy, envName, store, defs)
	ke2 := raw2.(*env.K8sDeployEnvironment)
	ke2.Namespace = testNamespace

	err = ke2.Create(ctx, env.CreateOpts{Image: testImage})
	assert.Error(t, err)
	assert.ErrorIs(t, err, env.ErrNameConflict)

	// Cleanup.
	_ = ke.Delete(ctx, true)
}
