package compose

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailable_FindsRuntime(t *testing.T) {
	// This test only passes if podman-compose is installed.
	// Skip if not available for CI environments without it.
	path, err := Available()
	if err != nil {
		t.Skip("no compose runtime available in PATH")
	}
	assert.NotEmpty(t, path)
}

// writeFakeBin creates an executable script named `name` in dir. If script is
// empty, the binary simply exits 0 with no output.
func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bin scripts require a POSIX shell")
	}
	if script == "" {
		script = "#!/bin/sh\nexit 0\n"
	}
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

func TestAvailable_PrefersPodmanCompose(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "podman-compose", "")
	writeFakeBin(t, dir, "docker", "#!/bin/sh\necho 'Docker Compose version v2.0.0'\nexit 0\n")
	t.Setenv("PATH", dir)

	path, err := Available()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "podman-compose"), path)
}

func TestAvailable_FallsBackToDockerComposePlugin(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "docker", "#!/bin/sh\necho 'Docker Compose version v2.0.0'\nexit 0\n")
	t.Setenv("PATH", dir)

	path, err := Available()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "docker")+" compose", path)
}

func TestAvailable_DockerPresentButComposeUnsupported(t *testing.T) {
	dir := t.TempDir()
	// docker exists but "docker compose version" fails / has no useful output.
	writeFakeBin(t, dir, "docker", "#!/bin/sh\nexit 1\n")
	writeFakeBin(t, dir, "docker-compose", "")
	t.Setenv("PATH", dir)

	path, err := Available()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "docker-compose"), path)
}

func TestAvailable_FallsBackToLegacyDockerCompose(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "docker-compose", "")
	t.Setenv("PATH", dir)

	path, err := Available()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "docker-compose"), path)
}

func TestAvailable_NoneFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	path, err := Available()
	assert.Error(t, err)
	assert.Empty(t, path)
}

func TestRuntimeCmd_SingleBinary(t *testing.T) {
	parts := RuntimeCmd("/usr/bin/podman-compose")
	assert.Equal(t, []string{"/usr/bin/podman-compose"}, parts)
}

func TestRuntimeCmd_PluginStyle(t *testing.T) {
	parts := RuntimeCmd("/usr/bin/docker compose")
	assert.Equal(t, []string{"/usr/bin/docker", "compose"}, parts)
}
