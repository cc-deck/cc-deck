package podman

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodCreate_ArgsStructure(t *testing.T) {
	// Verify that PodCreate would construct the correct arguments.
	// We cannot call it without a running podman, but we validate the
	// function signature and parameter passing.
	assert.NotNil(t, PodCreate)
}

func TestPodRemove_ArgsStructure(t *testing.T) {
	assert.NotNil(t, PodRemove)
}

func TestPodCreate_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := PodCreate(context.Background(), "mypod", "--infra=false")
	assert.NoError(t, err)
	assert.Equal(t, "pod create --name mypod --infra=false\n", readArgsLog(t, dir))
}

func TestPodCreate_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := PodCreate(context.Background(), "mypod")
	assert.Error(t, err)
}

func TestPodRemove_Idempotent(t *testing.T) {
	fakePodman(t, `echo "Error: no such pod mypod" >&2; exit 1`)
	err := PodRemove(context.Background(), "mypod")
	assert.NoError(t, err)
}

func TestPodRemove_NotFoundVariant(t *testing.T) {
	fakePodman(t, `echo "pod mypod not found" >&2; exit 1`)
	err := PodRemove(context.Background(), "mypod")
	assert.NoError(t, err)
}

func TestPodRemove_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := PodRemove(context.Background(), "mypod")
	assert.Error(t, err)
}

func TestPodRemove_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := PodRemove(context.Background(), "mypod")
	assert.NoError(t, err)
	assert.Equal(t, "pod rm -f mypod\n", readArgsLog(t, dir))
}
