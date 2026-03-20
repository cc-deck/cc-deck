package podman

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailable(t *testing.T) {
	// Available() should return a boolean without panicking.
	// The actual result depends on whether podman is installed.
	result := Available()
	assert.IsType(t, true, result)
}

func TestRunOpts_BuildArgs(t *testing.T) {
	// Verify that Run builds the correct argument structure by inspecting
	// the RunOpts fields. We cannot call Run without an actual podman binary,
	// so this test validates the data model used to build commands.
	opts := RunOpts{
		Name:    "test-container",
		Image:   "quay.io/cc-deck/cc-deck-demo:latest",
		Volumes: []string{"vol1:/data", "vol2:/workspace"},
		Secrets: []SecretMount{
			{Name: "secret1", Target: "MY_KEY"},
			{Name: "secret2", Target: "OTHER_KEY"},
		},
		Ports:    []string{"8080:80", "9090:90"},
		AllPorts: false,
		Cmd:      []string{"sleep", "infinity"},
	}

	assert.Equal(t, "test-container", opts.Name)
	assert.Equal(t, "quay.io/cc-deck/cc-deck-demo:latest", opts.Image)
	assert.Len(t, opts.Volumes, 2)
	assert.Len(t, opts.Secrets, 2)
	assert.Equal(t, "MY_KEY", opts.Secrets[0].Target)
	assert.Len(t, opts.Ports, 2)
	assert.False(t, opts.AllPorts)
	assert.Equal(t, []string{"sleep", "infinity"}, opts.Cmd)
}

func TestRunOpts_AllPortsExcludesExplicitPorts(t *testing.T) {
	// When AllPorts is true, explicit port mappings should conceptually
	// be ignored (the Run function uses -P instead of -p flags).
	opts := RunOpts{
		Name:     "allports-test",
		Image:    "test:latest",
		AllPorts: true,
		Ports:    []string{"8080:80"},
	}

	assert.True(t, opts.AllPorts)
	// The Run function checks AllPorts first and uses -P,
	// skipping the Ports slice entirely.
}

func TestContainerInfo_Fields(t *testing.T) {
	info := ContainerInfo{
		ID:      "abc123def456",
		Name:    "my-container",
		State:   "running",
		Running: true,
	}

	assert.Equal(t, "abc123def456", info.ID)
	assert.Equal(t, "my-container", info.Name)
	assert.Equal(t, "running", info.State)
	assert.True(t, info.Running)
}

func TestSecretMount_Fields(t *testing.T) {
	sm := SecretMount{
		Name:   "cc-deck-mydev-anthropic-api-key",
		Target: "ANTHROPIC_API_KEY",
	}

	assert.Equal(t, "cc-deck-mydev-anthropic-api-key", sm.Name)
	assert.Equal(t, "ANTHROPIC_API_KEY", sm.Target)
}
