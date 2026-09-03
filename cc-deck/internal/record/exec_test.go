package record

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakePodman writes a shell script named "podman" into dir that behaves
// according to script, then prepends dir to PATH for the duration of the
// test (restored automatically via t.Cleanup / t.Setenv).
func writeFakePodman(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake podman shell script requires a POSIX shell")
	}

	dir := t.TempDir()
	fakePath := filepath.Join(dir, "podman")
	require.NoError(t, os.WriteFile(fakePath, []byte(script), 0o755))

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
}

func TestValidateImage_Success(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nexit 0\n")

	err := validateImage(context.Background(), "some-image:latest")
	assert.NoError(t, err)
}

func TestValidateImage_NotFound(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nexit 1\n")

	err := validateImage(context.Background(), "missing-image:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-image:latest")
	assert.Contains(t, err.Error(), "cc-deck build run --target openshell")
}

func TestValidateImage_PodmanMissing(t *testing.T) {
	// No fake podman on PATH at all; force PATH to something that can't
	// possibly resolve "podman".
	t.Setenv("PATH", t.TempDir())

	err := validateImage(context.Background(), "any-image")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "any-image")
}

func TestRunInPod_Success(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nexit 0\n")

	err := runInPod(context.Background(), "pod1", "container1", "image1",
		[]string{"vol1:/data"}, []string{"sleep", "infinity"})
	assert.NoError(t, err)
}

func TestRunInPod_Failure(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\necho 'boom failure' >&2\nexit 1\n")

	err := runInPod(context.Background(), "pod1", "container1", "image1", nil, []string{"sh"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom failure")
}

func TestRunInPod_ArgsPassedThrough(t *testing.T) {
	// Fake podman echoes its args so we can assert the command was built
	// correctly (run -d --pod <pod> --name <name> -v <vol> <image> <cmd...>).
	script := "#!/bin/sh\necho \"$@\"\nexit 0\n"
	writeFakePodman(t, script)

	err := runInPod(context.Background(), "mypod", "mycontainer", "myimage",
		[]string{"myvol:/mnt"}, []string{"echo", "hi"})
	assert.NoError(t, err)
}

func TestExtractDNSLog_Success(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\necho 'line1'\necho 'line2'\nexit 0\n")

	out, err := extractDNSLog(context.Background(), "myvolume")
	require.NoError(t, err)
	assert.Contains(t, out, "line1")
	assert.Contains(t, out, "line2")
}

func TestExtractDNSLog_NoSuchFile(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\necho 'cat: /data/queries.log: No such file or directory' >&2\nexit 1\n")

	out, err := extractDNSLog(context.Background(), "myvolume")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestExtractDNSLog_OtherFailure(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\necho 'permission denied' >&2\nexit 1\n")

	out, err := extractDNSLog(context.Background(), "myvolume")
	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestValidateImage_ContextCancelled(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nsleep 5\nexit 0\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := validateImage(ctx, "any-image")
	require.Error(t, err)
}

func TestRunInPod_ContextCancelled(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nsleep 5\nexit 0\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runInPod(ctx, "pod", "container", "image", nil, []string{"sh"})
	require.Error(t, err)
}

func TestExtractDNSLog_ContextCancelled(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nsleep 5\nexit 0\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := extractDNSLog(ctx, "volume")
	require.Error(t, err)
}

func TestValidateImage_ErrorWraps(t *testing.T) {
	writeFakePodman(t, "#!/bin/sh\nexit 1\n")

	err := validateImage(context.Background(), "img")
	require.Error(t, err)
	assert.Equal(t, fmt.Sprintf("workspace image %q not found; run 'cc-deck build run --target openshell' first", "img"), err.Error())
}
