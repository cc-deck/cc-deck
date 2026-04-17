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

func TestValidateSyncPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"clean absolute", "/workspace/src", false},
		{"clean relative", "src/main", false},
		{"dot current", ".", false},
		{"traversal", "../../../etc/passwd", true},
		{"embedded traversal relative", "workspace/../../../etc", true},
		{"simple parent", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSyncPath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "directory traversal")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestK8sPush_RejectsTraversalPath(t *testing.T) {
	err := k8sPush(t.Context(), "default", "pod-0", nil, SyncOpts{
		LocalPath:  "/tmp/safe",
		RemotePath: "../../etc",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")
}

func TestK8sPull_RejectsTraversalPath(t *testing.T) {
	err := k8sPull(t.Context(), "default", "pod-0", nil, SyncOpts{
		RemotePath: "/workspace",
		LocalPath:  "../../../tmp",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")
}
