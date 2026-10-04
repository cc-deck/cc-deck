package imageprobe

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writePodmanStub creates a fake "podman" executable on PATH that runs the
// given shell script body, allowing RunProbe/ResolveDigest to be exercised
// without a real container runtime.
func writePodmanStub(t *testing.T, script string) {
	t.Helper()
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "podman")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\n"+script), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
}

func TestRunProbe_ParsesSuccessfulOutput(t *testing.T) {
	writePodmanStub(t, `echo '{"type":"os","id":"fedora","id_like":"","name":"Fedora","version":"41"}'
echo '{"type":"tool","name":"git","path":"/usr/bin/git","version":"2.40","present":true}'
exit 0
`)

	result, err := RunProbe(context.Background(), "example.com/image:latest", []ProbeToolEntry{{Name: "git"}})
	require.NoError(t, err)
	assert.Equal(t, "fedora", result.OS.ID)
	assert.Equal(t, "example.com/image:latest", result.ImageRef)
	git, ok := result.Tools["git"]
	require.True(t, ok)
	assert.True(t, git.Present)
	assert.False(t, result.Timestamp.IsZero())
	assert.GreaterOrEqual(t, result.DurationMS, int64(0))
}

func TestRunProbe_CommandFailureReturnsError(t *testing.T) {
	writePodmanStub(t, "exit 1\n")

	_, err := RunProbe(context.Background(), "example.com/image:latest", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe container")
}

func TestResolveDigest_FoundViaInspect(t *testing.T) {
	writePodmanStub(t, `if [ "$1" = "inspect" ]; then
  echo "sha256:abc123"
  exit 0
fi
exit 1
`)

	digest, err := ResolveDigest(context.Background(), "example.com/image:latest")
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc123", digest)
}

func TestResolveDigest_PullsAndRetriesWhenNotPresent(t *testing.T) {
	writePodmanStub(t, `if [ "$1" = "inspect" ]; then
  echo "<no value>"
  exit 0
fi
if [ "$1" = "pull" ]; then
  exit 0
fi
exit 1
`)

	digest, err := ResolveDigest(context.Background(), "example.com/image:latest")
	require.Error(t, err)
	assert.Empty(t, digest)
	assert.Contains(t, err.Error(), "no digest found")
}

func TestResolveDigest_PullFailureReturnsError(t *testing.T) {
	writePodmanStub(t, `if [ "$1" = "inspect" ]; then
  exit 1
fi
if [ "$1" = "pull" ]; then
  echo "pull failed" >&2
  exit 1
fi
exit 1
`)

	_, err := ResolveDigest(context.Background(), "example.com/image:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulling image")
}

func TestResolveDigest_InspectAfterPullFails(t *testing.T) {
	writePodmanStub(t, `if [ "$1" = "inspect" ]; then
  exit 1
fi
if [ "$1" = "pull" ]; then
  exit 0
fi
exit 1
`)

	_, err := ResolveDigest(context.Background(), "example.com/image:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inspecting image")
}
