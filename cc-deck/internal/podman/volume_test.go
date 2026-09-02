package podman

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVolumeCreate_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := VolumeCreate(context.Background(), "myvol")
	assert.NoError(t, err)
	assert.Equal(t, "volume create myvol\n", readArgsLog(t, dir))
}

func TestVolumeCreate_AlreadyExists(t *testing.T) {
	fakePodman(t, `echo "Error: volume myvol already exists" >&2; exit 1`)
	err := VolumeCreate(context.Background(), "myvol")
	assert.NoError(t, err)
}

func TestVolumeCreate_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := VolumeCreate(context.Background(), "myvol")
	assert.Error(t, err)
}

func TestVolumeRemove_Idempotent(t *testing.T) {
	fakePodman(t, `echo "Error: no such volume myvol" >&2; exit 1`)
	err := VolumeRemove(context.Background(), "myvol")
	assert.NoError(t, err)
}

func TestVolumeRemove_NotFoundVariant(t *testing.T) {
	fakePodman(t, `echo "volume myvol not found" >&2; exit 1`)
	err := VolumeRemove(context.Background(), "myvol")
	assert.NoError(t, err)
}

func TestVolumeRemove_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := VolumeRemove(context.Background(), "myvol")
	assert.Error(t, err)
}

func TestVolumeRemove_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := VolumeRemove(context.Background(), "myvol")
	assert.NoError(t, err)
	assert.Equal(t, "volume rm myvol\n", readArgsLog(t, dir))
}

func TestVolumeExists_True(t *testing.T) {
	fakePodman(t, `exit 0`)
	assert.True(t, VolumeExists(context.Background(), "myvol"))
}

func TestVolumeExists_False(t *testing.T) {
	fakePodman(t, `exit 1`)
	assert.False(t, VolumeExists(context.Background(), "myvol"))
}
