//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/ws"
)

const (
	testNamespace = "cc-deck-test"
	testImage     = "localhost/cc-deck-stub:latest"
)

func TestK8sDeployLifecycle(t *testing.T) {
	ctx := context.Background()
	store := ws.NewStateStore(t.TempDir() + "/state.yaml")
	defs := ws.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	wsName := "inttest-lifecycle"

	// Create
	raw, err := ws.NewWorkspace(ws.WorkspaceTypeK8sDeploy, wsName, store, defs)
	require.NoError(t, err)

	ke, ok := raw.(*ws.K8sDeployWorkspace)
	require.True(t, ok)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute

	err = ke.Create(ctx, ws.CreateOpts{Image: testImage})
	require.NoError(t, err, "Create should succeed")

	// Verify instance recorded.
	inst, err := store.FindInstanceByName(wsName)
	require.NoError(t, err)
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, ws.InfraStateRunning, *inst.InfraState)
	assert.Equal(t, testNamespace, inst.K8s.Namespace)

	// Stop
	err = ke.Stop(ctx)
	require.NoError(t, err, "Stop should succeed")

	inst, _ = store.FindInstanceByName(wsName)
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, ws.InfraStateStopped, *inst.InfraState)

	// Start
	err = ke.Start(ctx)
	require.NoError(t, err, "Start should succeed")

	inst, _ = store.FindInstanceByName(wsName)
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, ws.InfraStateRunning, *inst.InfraState)

	// Status
	status, err := ke.Status(ctx)
	require.NoError(t, err)
	if status.InfraState == nil {
		t.Fatal("expected infra state to be set")
	}
	assert.Equal(t, ws.InfraStateRunning, *status.InfraState)

	// Delete
	err = ke.Delete(ctx, true)
	require.NoError(t, err, "Delete should succeed")

	_, err = store.FindInstanceByName(wsName)
	assert.Error(t, err, "Instance should be removed after delete")
}

func TestK8sDeployResourceVerification(t *testing.T) {
	ctx := context.Background()
	store := ws.NewStateStore(t.TempDir() + "/state.yaml")
	defs := ws.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	wsName := "inttest-resources"

	raw, err := ws.NewWorkspace(ws.WorkspaceTypeK8sDeploy, wsName, store, defs)
	require.NoError(t, err)

	ke := raw.(*ws.K8sDeployWorkspace)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute
	ke.Credentials = map[string]string{"TEST_KEY": "test-value"}

	err = ke.Create(ctx, ws.CreateOpts{Image: testImage, Storage: ws.StorageConfig{Size: "1Gi"}})
	require.NoError(t, err)

	// Verify K8s resources exist via client.
	client, err := ws.NewK8sClient("", "")
	require.NoError(t, err)

	resName := "cc-deck-" + wsName

	// Verify StatefulSet exists.
	_, stsErr := client.ReconcileState(ctx, testNamespace, resName)
	assert.NoError(t, stsErr)

	// Cleanup.
	_ = ke.Delete(ctx, true)
}

func TestK8sDeployDuplicateConflict(t *testing.T) {
	ctx := context.Background()
	store := ws.NewStateStore(t.TempDir() + "/state.yaml")
	defs := ws.NewDefinitionStore(t.TempDir() + "/definitions.yaml")

	wsName := "inttest-dup"

	// First create.
	raw, err := ws.NewWorkspace(ws.WorkspaceTypeK8sDeploy, wsName, store, defs)
	require.NoError(t, err)
	ke := raw.(*ws.K8sDeployWorkspace)
	ke.Namespace = testNamespace
	ke.NoNetworkPolicy = true
	ke.Timeout = 3 * time.Minute

	err = ke.Create(ctx, ws.CreateOpts{Image: testImage})
	require.NoError(t, err)

	// Second create with same name should fail.
	raw2, _ := ws.NewWorkspace(ws.WorkspaceTypeK8sDeploy, wsName, store, defs)
	ke2 := raw2.(*ws.K8sDeployWorkspace)
	ke2.Namespace = testNamespace

	err = ke2.Create(ctx, ws.CreateOpts{Image: testImage})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ws.ErrNameConflict)

	// Cleanup.
	_ = ke.Delete(ctx, true)
}
