package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectRuntime_FindsPodmanOnPath(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "podman")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir)

	path, err := DetectRuntime()
	require.NoError(t, err)
	assert.Equal(t, stub, path)
}

func TestDetectRuntime_ErrorsWhenPodmanMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := DetectRuntime()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podman not found")
}
