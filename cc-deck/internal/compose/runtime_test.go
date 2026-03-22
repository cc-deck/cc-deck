package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestRuntimeCmd_SingleBinary(t *testing.T) {
	parts := RuntimeCmd("/usr/bin/podman-compose")
	assert.Equal(t, []string{"/usr/bin/podman-compose"}, parts)
}

func TestRuntimeCmd_PluginStyle(t *testing.T) {
	parts := RuntimeCmd("/usr/bin/docker compose")
	assert.Equal(t, []string{"/usr/bin/docker", "compose"}, parts)
}
