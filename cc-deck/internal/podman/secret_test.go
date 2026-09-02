package podman

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretCreate_Success(t *testing.T) {
	dir := fakePodman(t, `cat > "$(dirname "$0")/stdin.log"; echo "$@" > "$(dirname "$0")/args.log"`)
	err := SecretCreate(context.Background(), "mysecret", []byte("supersecretvalue"))
	assert.NoError(t, err)
	assert.Equal(t, "secret create --replace mysecret -\n", readArgsLog(t, dir))
	data, rerr := os.ReadFile(filepath.Join(dir, "stdin.log"))
	assert.NoError(t, rerr)
	assert.Equal(t, "supersecretvalue", string(data))
}

func TestSecretCreate_Error(t *testing.T) {
	fakePodman(t, `cat > /dev/null; echo "boom" >&2; exit 1`)
	err := SecretCreate(context.Background(), "mysecret", []byte("value"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "podman secret create")
	assert.Contains(t, err.Error(), "boom")
}

func TestSecretRemove_Idempotent(t *testing.T) {
	fakePodman(t, `echo "Error: no such secret mysecret" >&2; exit 1`)
	err := SecretRemove(context.Background(), "mysecret")
	assert.NoError(t, err)
}

func TestSecretRemove_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := SecretRemove(context.Background(), "mysecret")
	assert.Error(t, err)
}

func TestSecretRemove_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := SecretRemove(context.Background(), "mysecret")
	assert.NoError(t, err)
	assert.Equal(t, "secret rm mysecret\n", readArgsLog(t, dir))
}

func TestSecretExists_True(t *testing.T) {
	fakePodman(t, `exit 0`)
	assert.True(t, SecretExists(context.Background(), "mysecret"))
}

func TestSecretExists_False(t *testing.T) {
	fakePodman(t, `exit 1`)
	assert.False(t, SecretExists(context.Background(), "mysecret"))
}
