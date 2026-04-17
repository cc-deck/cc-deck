package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8sPush_RequiresLocalPath(t *testing.T) {
	err := k8sPush(t.Context(), "default", "pod-0", nil, SyncOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local path is required")
}

func TestK8sPull_RequiresRemotePath(t *testing.T) {
	err := k8sPull(t.Context(), "default", "pod-0", nil, SyncOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remote path is required")
}

func TestK8sHarvest_RequiresBranch(t *testing.T) {
	err := k8sHarvest(t.Context(), "default", "pod-0", nil, HarvestOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--branch is required")
}
