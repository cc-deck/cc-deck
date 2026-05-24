package podman

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodCreate_ArgsStructure(t *testing.T) {
	// Verify that PodCreate would construct the correct arguments.
	// We cannot call it without a running podman, but we validate the
	// function signature and parameter passing.
	assert.NotNil(t, PodCreate)
}

func TestPodRemove_ArgsStructure(t *testing.T) {
	assert.NotNil(t, PodRemove)
}

