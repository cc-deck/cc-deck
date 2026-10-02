package podman

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStart_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := Start(context.Background(), "c1")
	assert.NoError(t, err)
	assert.Equal(t, "start c1\n", readArgsLog(t, dir))
}

func TestStart_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := Start(context.Background(), "c1")
	assert.Error(t, err)
}

func TestStop_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := Stop(context.Background(), "c1")
	assert.NoError(t, err)
	assert.Equal(t, "stop c1\n", readArgsLog(t, dir))
}

func TestStop_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := Stop(context.Background(), "c1")
	assert.Error(t, err)
}

func TestRemove_Force(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := Remove(context.Background(), "c1", true)
	assert.NoError(t, err)
	assert.Equal(t, "rm -f c1\n", readArgsLog(t, dir))
}

func TestRemove_NoForce(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	err := Remove(context.Background(), "c1", false)
	assert.NoError(t, err)
	assert.Equal(t, "rm c1\n", readArgsLog(t, dir))
}

func TestRemove_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := Remove(context.Background(), "c1", false)
	assert.Error(t, err)
}

func TestInspect_Found(t *testing.T) {
	fakePodman(t, `printf 'running\ttrue\tabc123\tmycontainer\n'`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "running", info.State)
	assert.True(t, info.Running)
	assert.Equal(t, "abc123", info.ID)
	assert.Equal(t, "mycontainer", info.Name)
}

func TestInspect_NotRunning(t *testing.T) {
	fakePodman(t, `printf 'exited\tfalse\tabc123\tmycontainer\n'`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.False(t, info.Running)
}

func TestInspect_NotFound(t *testing.T) {
	fakePodman(t, `echo "Error: no such container mycontainer" >&2; exit 1`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.NoError(t, err)
	assert.Nil(t, info)
}

func TestInspect_NotFoundVariant(t *testing.T) {
	fakePodman(t, `echo "container mycontainer not found" >&2; exit 1`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.NoError(t, err)
	assert.Nil(t, info)
}

func TestInspect_OtherError(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestInspect_MalformedOutput(t *testing.T) {
	fakePodman(t, `printf 'onlyonefield\n'`)
	info, err := Inspect(context.Background(), "mycontainer")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "unexpected inspect output")
}

func TestRun_MinimalOpts(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"; echo "container-id-123"`)
	id, err := Run(context.Background(), RunOpts{Image: "myimage:latest"})
	assert.NoError(t, err)
	assert.Equal(t, "container-id-123", id)
	assert.Equal(t, "run -d myimage:latest sleep infinity\n", readArgsLog(t, dir))
}

func TestRun_FullOpts(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"; echo "container-id-456"`)
	opts := RunOpts{
		Name:    "mycontainer",
		Image:   "myimage:latest",
		Volumes: []string{"vol1:/data"},
		Secrets: []SecretMount{
			{Name: "s1", AsFile: true},
			{Name: "s2", Target: "MY_ENV", AsFile: false},
		},
		Envs:     []string{"FOO=bar"},
		AllPorts: true,
		Ports:    []string{"8080:80"}, // ignored because AllPorts is true
		Cmd:      []string{"echo", "hi"},
	}
	id, err := Run(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, "container-id-456", id)
	expected := "run -d --name mycontainer -v vol1:/data --secret s1 --secret s2,type=env,target=MY_ENV -e FOO=bar -P myimage:latest echo hi\n"
	assert.Equal(t, expected, readArgsLog(t, dir))
}

func TestRun_ExplicitPortsWithoutAllPorts(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"; echo "id"`)
	_, err := Run(context.Background(), RunOpts{Image: "img", Ports: []string{"8080:80", "9090:90"}})
	assert.NoError(t, err)
	assert.Equal(t, "run -d -p 8080:80 -p 9090:90 img sleep infinity\n", readArgsLog(t, dir))
}

func TestRun_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	id, err := Run(context.Background(), RunOpts{Image: "img"})
	assert.Error(t, err)
	assert.Empty(t, id)
}
