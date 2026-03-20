package env

import (
	"context"
	"errors"
	"testing"

	"github.com/cc-deck/cc-deck/internal/podman"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerName(t *testing.T) {
	t.Helper()
	tests := []struct {
		input string
		want  string
	}{
		{"mydev", "cc-deck-mydev"},
		{"a", "cc-deck-a"},
		{"my-project-1", "cc-deck-my-project-1"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, containerName(tt.input))
	}
}

func TestVolumeName(t *testing.T) {
	t.Helper()
	tests := []struct {
		input string
		want  string
	}{
		{"mydev", "cc-deck-mydev-data"},
		{"a", "cc-deck-a-data"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, volumeName(tt.input))
	}
}

func TestSecretName(t *testing.T) {
	t.Helper()
	tests := []struct {
		envName string
		key     string
		want    string
	}{
		{"mydev", "ANTHROPIC_API_KEY", "cc-deck-mydev-anthropic-api-key"},
		{"proj", "GOOGLE_APPLICATION_CREDENTIALS", "cc-deck-proj-google-application-credentials"},
		{"a", "MY_VAR", "cc-deck-a-my-var"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, secretName(tt.envName, tt.key))
	}
}

func TestBaseNameFromPath(t *testing.T) {
	t.Helper()
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/project", "project"},
		{"project", "project"},
		{"/a/b/c/d", "d"},
		{"C:\\Users\\dev\\project", "project"},
		{"file.txt", "file.txt"},
		{"/trailing/", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, baseNameFromPath(tt.input), "baseNameFromPath(%q)", tt.input)
	}
}

func TestContainerEnvironment_Type(t *testing.T) {
	store := newTestStore(t)
	env := &ContainerEnvironment{name: "test", store: store}
	assert.Equal(t, EnvironmentTypeContainer, env.Type())
}

func TestContainerEnvironment_Name(t *testing.T) {
	store := newTestStore(t)
	env := &ContainerEnvironment{name: "my-container", store: store}
	assert.Equal(t, "my-container", env.Name())
}

func TestContainerEnvironment_CreateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	env := &ContainerEnvironment{name: "INVALID", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidName))
}

func TestContainerEnvironment_CreateRequiresPodman(t *testing.T) {
	if !podman.Available() {
		t.Skip("podman not available")
	}

	// This test validates the flow reaches the podman check.
	// With podman available, it proceeds past the check (and may fail
	// later on actual container creation, but that is expected).
	store := newTestStore(t)
	env := &ContainerEnvironment{name: "valid-name", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	// If podman IS available, the error will not be ErrPodmanNotFound.
	if err != nil {
		assert.False(t, errors.Is(err, ErrPodmanNotFound),
			"with podman available, should not get ErrPodmanNotFound")
	}
}

func TestContainerEnvironment_HarvestReturnsNotSupported(t *testing.T) {
	store := newTestStore(t)
	env := &ContainerEnvironment{name: "test", store: store}

	err := env.Harvest(context.Background(), HarvestOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotSupported))
}

func TestContainerEnvironment_DeleteBestEffort(t *testing.T) {
	if !podman.Available() {
		t.Skip("podman not available")
	}

	store := newTestStore(t)
	env := &ContainerEnvironment{name: "nonexistent-env", store: store}

	// Delete of a non-existent container with force should not panic.
	// It logs warnings but returns nil (best-effort pattern).
	err := env.Delete(context.Background(), true)
	assert.NoError(t, err)
}
