package env

import (
	"context"
	"errors"
	"os"
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

// --- Auth mode detection tests ---

func TestDetectAuthMode_Vertex(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	assert.Equal(t, AuthModeVertex, DetectAuthMode())
}

func TestDetectAuthMode_Bedrock(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("ANTHROPIC_API_KEY", "")
	assert.Equal(t, AuthModeBedrock, DetectAuthMode())
}

func TestDetectAuthMode_API(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	assert.Equal(t, AuthModeAPI, DetectAuthMode())
}

func TestDetectAuthMode_None(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	assert.Equal(t, AuthModeNone, DetectAuthMode())
}

func TestDetectAuthMode_VertexPrecedence(t *testing.T) {
	// Vertex takes precedence over Bedrock and API key
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	assert.Equal(t, AuthModeVertex, DetectAuthMode())
}

func TestDetectAuthCredentials_API(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeAPI, creds)
	assert.Equal(t, "sk-ant-test-key", creds["ANTHROPIC_API_KEY"])
}

func TestDetectAuthCredentials_Vertex(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-project")
	t.Setenv("CLOUD_ML_REGION", "us-east5")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/path/to/adc.json")
	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeVertex, creds)

	assert.Equal(t, "1", creds["CLAUDE_CODE_USE_VERTEX"])
	assert.Equal(t, "my-project", creds["ANTHROPIC_VERTEX_PROJECT_ID"])
	assert.Equal(t, "us-east5", creds["CLOUD_ML_REGION"])
	assert.Equal(t, "/path/to/adc.json", creds["GOOGLE_APPLICATION_CREDENTIALS"])
}

func TestDetectAuthCredentials_Vertex_DefaultADC(t *testing.T) {
	// When GOOGLE_APPLICATION_CREDENTIALS is not set, should check for
	// the default ADC file at ~/.config/gcloud/application_default_credentials.json
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-project")
	t.Setenv("CLOUD_ML_REGION", "us-east5")

	// Create a fake ADC file in a temp dir
	tmpDir := t.TempDir()
	gcloudDir := tmpDir + "/.config/gcloud"
	require.NoError(t, os.MkdirAll(gcloudDir, 0o755))
	adcPath := gcloudDir + "/application_default_credentials.json"
	require.NoError(t, os.WriteFile(adcPath, []byte(`{"type":"authorized_user"}`), 0o644))

	// Override HOME to use the temp dir
	t.Setenv("HOME", tmpDir)

	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeVertex, creds)

	assert.Equal(t, "1", creds["CLAUDE_CODE_USE_VERTEX"])
	assert.Equal(t, adcPath, creds["GOOGLE_APPLICATION_CREDENTIALS"],
		"should auto-detect default ADC file when GOOGLE_APPLICATION_CREDENTIALS is unset")
}

func TestDetectAuthCredentials_Vertex_NoADC(t *testing.T) {
	// When neither GOOGLE_APPLICATION_CREDENTIALS nor default ADC exists
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-project")
	t.Setenv("HOME", t.TempDir()) // empty home, no gcloud dir

	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeVertex, creds)

	assert.Equal(t, "1", creds["CLAUDE_CODE_USE_VERTEX"])
	_, hasGAC := creds["GOOGLE_APPLICATION_CREDENTIALS"]
	assert.False(t, hasGAC, "should not set GOOGLE_APPLICATION_CREDENTIALS when no ADC file exists")
}

func TestDetectAuthCredentials_Bedrock(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA-test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret-test")
	t.Setenv("AWS_SESSION_TOKEN", "token-test")
	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeBedrock, creds)

	assert.Equal(t, "1", creds["CLAUDE_CODE_USE_BEDROCK"])
	assert.Equal(t, "us-east-1", creds["AWS_REGION"])
	assert.Equal(t, "AKIA-test", creds["AWS_ACCESS_KEY_ID"])
	assert.Equal(t, "secret-test", creds["AWS_SECRET_ACCESS_KEY"])
	assert.Equal(t, "token-test", creds["AWS_SESSION_TOKEN"])
}

func TestDetectAuthCredentials_ExplicitOverridesAuto(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "auto-detected-key")
	creds := map[string]string{
		"ANTHROPIC_API_KEY": "explicit-key",
	}
	DetectAuthCredentials(AuthModeAPI, creds)
	// Explicit value should NOT be overwritten
	assert.Equal(t, "explicit-key", creds["ANTHROPIC_API_KEY"])
}

func TestDetectAuthCredentials_ModelPinning(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("ANTHROPIC_DEFAULT_SONNET_MODEL", "claude-sonnet-4-20250514")
	t.Setenv("ANTHROPIC_DEFAULT_OPUS_MODEL", "")
	creds := make(map[string]string)
	DetectAuthCredentials(AuthModeAPI, creds)

	assert.Equal(t, "claude-sonnet-4-20250514", creds["ANTHROPIC_DEFAULT_SONNET_MODEL"])
	_, hasOpus := creds["ANTHROPIC_DEFAULT_OPUS_MODEL"]
	assert.False(t, hasOpus, "empty env vars should not be injected")
}

// --- Auth mode constants ---

func TestAuthModeConstants(t *testing.T) {
	assert.Equal(t, AuthMode("auto"), AuthModeAuto)
	assert.Equal(t, AuthMode("none"), AuthModeNone)
	assert.Equal(t, AuthMode("api"), AuthModeAPI)
	assert.Equal(t, AuthMode("vertex"), AuthModeVertex)
	assert.Equal(t, AuthMode("bedrock"), AuthModeBedrock)
}
