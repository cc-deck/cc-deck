package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStampPolicyLabel_DaemonUnavailable exercises the daemon.Image failure
// path by pointing DOCKER_HOST at a socket that cannot exist, forcing a
// connection failure regardless of whether a real container runtime is
// present in the test environment.
func TestStampPolicyLabel_DaemonUnavailable(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/path/to/docker.sock")

	err := StampPolicyLabel("some-image:latest", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading image")
}
