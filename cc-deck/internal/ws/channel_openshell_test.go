package ws

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenShellDataChannel_Push_NoLocalPath(t *testing.T) {
	ch := &openShellDataChannel{ws: &OpenShellWorkspace{name: "test-ws"}}
	err := ch.Push(context.Background(), SyncOpts{})
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Equal(t, "data", chErr.Channel)
	assert.Equal(t, "push", chErr.Op)
	assert.Equal(t, "test-ws", chErr.Workspace)
}

func TestOpenShellDataChannel_Push_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	ch := &openShellDataChannel{ws: &OpenShellWorkspace{name: "test-ws", store: store}}
	err := ch.Push(context.Background(), SyncOpts{LocalPath: "/tmp/src"})
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Contains(t, chErr.Summary, "no sandbox")
}

func TestOpenShellDataChannel_Pull_NoRemotePath(t *testing.T) {
	ch := &openShellDataChannel{ws: &OpenShellWorkspace{name: "test-ws"}}
	err := ch.Pull(context.Background(), SyncOpts{})
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Equal(t, "data", chErr.Channel)
	assert.Equal(t, "pull", chErr.Op)
}

func TestOpenShellDataChannel_Pull_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	ch := &openShellDataChannel{ws: &OpenShellWorkspace{name: "test-ws", store: store}}
	err := ch.Pull(context.Background(), SyncOpts{RemotePath: "/sandbox"})
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Contains(t, chErr.Summary, "no sandbox")
}

func TestOpenShellDataChannel_PushBytes_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	ch := &openShellDataChannel{ws: &OpenShellWorkspace{name: "test-ws", store: store}}
	err := ch.PushBytes(context.Background(), []byte("data"), "/remote/path")
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Equal(t, "push-bytes", chErr.Op)
}

func TestOpenShellGitChannel_Fetch_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	ch := &openShellGitChannel{ws: &OpenShellWorkspace{name: "test-ws", store: store}}
	err := ch.Fetch(context.Background(), HarvestOpts{})
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Equal(t, "git", chErr.Channel)
	assert.Equal(t, "fetch", chErr.Op)
}

func TestOpenShellGitChannel_Push_NoSandbox(t *testing.T) {
	store := newTestStore(t)
	ch := &openShellGitChannel{ws: &OpenShellWorkspace{name: "test-ws", store: store}}
	err := ch.Push(context.Background())
	assert.Error(t, err)
	var chErr *ChannelError
	assert.True(t, errors.As(err, &chErr))
	assert.Equal(t, "git", chErr.Channel)
	assert.Equal(t, "push", chErr.Op)
}

func TestBuildExtOpenShellURL(t *testing.T) {
	url := buildExtOpenShellURL("gw:8080", "sb-123", "/sandbox")
	assert.Equal(t, "ext::openshell sandbox exec --gateway gw:8080 sb-123 -- %S /sandbox", url)
}

func TestPipeChannel_UsesExecPipeChannel(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	ch, err := w.PipeChannel(context.Background())
	assert.NoError(t, err)
	_, ok := ch.(*execPipeChannel)
	assert.True(t, ok, "PipeChannel should return an execPipeChannel")
}

func TestDataChannel_LazyInit(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	ch1, err := w.DataChannel(context.Background())
	assert.NoError(t, err)
	ch2, err := w.DataChannel(context.Background())
	assert.NoError(t, err)
	assert.Same(t, ch1, ch2, "DataChannel should return the same instance")
}

func TestGitChannel_LazyInit(t *testing.T) {
	w := &OpenShellWorkspace{name: "test-ws"}
	ch1, err := w.GitChannel(context.Background())
	assert.NoError(t, err)
	ch2, err := w.GitChannel(context.Background())
	assert.NoError(t, err)
	assert.Same(t, ch1, ch2, "GitChannel should return the same instance")
}
