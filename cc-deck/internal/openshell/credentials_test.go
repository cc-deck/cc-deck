package openshell

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient implements a minimal Client for testing UploadFileCredential.
type mockClient struct {
	uploadErr      error
	uploadedLocal  string
	uploadedRemote string
	execCount      int
}

func (m *mockClient) Address() string { return "localhost:17670" }
func (m *mockClient) CreateSandbox(_ context.Context, _, _, _ string, _ []string) (string, error) {
	return "", nil
}
func (m *mockClient) DeleteSandbox(_ context.Context, _ string) error { return nil }
func (m *mockClient) GetSandbox(_ context.Context, _ string) (*SandboxInfo, error) {
	return nil, nil
}
func (m *mockClient) ExecSandbox(_ context.Context, _ string, _ []string) (*ExecResult, error) {
	m.execCount++
	return &ExecResult{}, nil
}
func (m *mockClient) ExecSandboxStream(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockClient) AttachExec(_ context.Context, _ string, _ []string) error       { return nil }
func (m *mockClient) Upload(_ context.Context, _, localPath, remotePath string) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	m.uploadedLocal = localPath
	m.uploadedRemote = remotePath
	return nil
}
func (m *mockClient) Download(_ context.Context, _, _, _ string) error { return nil }
func (m *mockClient) CreateProvider(_ context.Context, _, _ string, _ bool, _ map[string]string) error {
	return nil
}
func (m *mockClient) UpdateProvider(_ context.Context, _, _ string, _ bool, _ map[string]string) error {
	return nil
}
func (m *mockClient) DeleteProvider(_ context.Context, _ string) error { return nil }
func (m *mockClient) EnsureProvider(_ context.Context, _, _ string, _ bool, _ map[string]string) error {
	return nil
}

func TestKnownProviderProfiles_AllTypesExist(t *testing.T) {
	expectedTypes := []string{"claude", "anthropic", "github", "gitlab", "openai", "nvidia", "vertex", "generic"}
	for _, typ := range expectedTypes {
		_, ok := KnownProviderProfiles[typ]
		assert.True(t, ok, "expected profile for type %q", typ)
	}
}

func TestKnownProviderProfiles_VertexHasEndpoints(t *testing.T) {
	profile := KnownProviderProfiles["vertex"]
	assert.Equal(t, "GOOGLE_APPLICATION_CREDENTIALS", profile.FileVar)
	require.Len(t, profile.Endpoints, 1)
	assert.Equal(t, "oauth2.googleapis.com", profile.Endpoints[0].Host)
	assert.Equal(t, 443, profile.Endpoints[0].Port)
}

func TestKnownProviderProfiles_GenericHasNoVars(t *testing.T) {
	profile := KnownProviderProfiles["generic"]
	assert.Empty(t, profile.DetectVars)
	assert.Empty(t, profile.RequiredVars)
	assert.Empty(t, profile.FileVar)
}

func TestResolveDefaultEnvVars_KnownTypes(t *testing.T) {
	tests := []struct {
		credType string
		expected []string
	}{
		{"claude", []string{"ANTHROPIC_API_KEY"}},
		{"anthropic", []string{"ANTHROPIC_API_KEY"}},
		{"github", []string{"GITHUB_TOKEN", "GH_TOKEN"}},
		{"gitlab", []string{"GITLAB_TOKEN", "GLAB_TOKEN"}},
		{"openai", []string{"OPENAI_API_KEY"}},
		{"nvidia", []string{"NVIDIA_API_KEY"}},
		{"vertex", []string{"GOOGLE_APPLICATION_CREDENTIALS", "ANTHROPIC_VERTEX_PROJECT_ID", "CLOUD_ML_REGION"}},
	}

	for _, tt := range tests {
		t.Run(tt.credType, func(t *testing.T) {
			result := ResolveDefaultEnvVars(tt.credType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveDefaultEnvVars_Generic(t *testing.T) {
	result := ResolveDefaultEnvVars("generic")
	assert.Nil(t, result)
}

func TestResolveDefaultEnvVars_Unknown(t *testing.T) {
	result := ResolveDefaultEnvVars("unknown-provider")
	assert.Nil(t, result)
}

func TestResolveCredentials_APIKeyPresent(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")

	entries := []CredentialInput{
		{Type: "claude"},
	}

	configs := ResolveCredentials(entries, "test-ws")
	require.Len(t, configs, 1)
	assert.Equal(t, "cc-deck-test-ws-claude", configs[0].Name)
	assert.Equal(t, "claude", configs[0].Type)
	assert.True(t, configs[0].FromExisting)
	assert.Nil(t, configs[0].Credentials)
}

func TestResolveCredentials_APIKeyMissing(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	entries := []CredentialInput{
		{Type: "claude"},
	}

	configs := ResolveCredentials(entries, "test-ws")
	assert.Empty(t, configs)
}

func TestResolveCredentials_MultipleTypes(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("GITHUB_TOKEN", "ghp-test")

	entries := []CredentialInput{
		{Type: "claude"},
		{Type: "github"},
	}

	configs := ResolveCredentials(entries, "myws")
	require.Len(t, configs, 2)
	assert.Equal(t, "cc-deck-myws-claude", configs[0].Name)
	assert.Equal(t, "cc-deck-myws-github", configs[1].Name)
}

func TestResolveCredentials_GenericType(t *testing.T) {
	t.Setenv("CUSTOM_API_KEY", "custom-value")

	entries := []CredentialInput{
		{Type: "generic", EnvVars: []string{"CUSTOM_API_KEY"}},
	}

	configs := ResolveCredentials(entries, "ws")
	require.Len(t, configs, 1)
	assert.Equal(t, "cc-deck-ws-generic", configs[0].Name)
	assert.Equal(t, "generic", configs[0].Type)
	assert.False(t, configs[0].FromExisting)
	assert.Equal(t, "custom-value", configs[0].Credentials["CUSTOM_API_KEY"])
}

func TestResolveCredentials_UnknownTypeFallsBackToGeneric(t *testing.T) {
	t.Setenv("MY_CUSTOM_KEY", "value123")

	entries := []CredentialInput{
		{Type: "custom-service", EnvVars: []string{"MY_CUSTOM_KEY"}},
	}

	configs := ResolveCredentials(entries, "ws")
	require.Len(t, configs, 1)
	assert.Equal(t, "custom-service", configs[0].Type)
	assert.False(t, configs[0].FromExisting)
	assert.Equal(t, "value123", configs[0].Credentials["MY_CUSTOM_KEY"])
}

func TestResolveCredentials_VertexWithFile(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/tmp/sa.json")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-project")

	entries := []CredentialInput{
		{Type: "vertex"},
	}

	configs := ResolveCredentials(entries, "ws")
	require.Len(t, configs, 1)
	assert.Equal(t, "vertex", configs[0].Type)
	assert.Equal(t, "GOOGLE_APPLICATION_CREDENTIALS", configs[0].FileVar)
	assert.Equal(t, "/tmp/sa.json", configs[0].FilePath)
}

func TestResolveCredentials_ExplicitEnvVarsOverrideDefaults(t *testing.T) {
	t.Setenv("MY_CLAUDE_KEY", "custom-key")

	entries := []CredentialInput{
		{Type: "claude", EnvVars: []string{"MY_CLAUDE_KEY"}},
	}

	// MY_CLAUDE_KEY is not in the RequiredVars for claude, so it won't
	// pass the required check. But ANTHROPIC_API_KEY is checked.
	// Actually: the required check uses profile.RequiredVars, not the entry's EnvVars.
	// So with ANTHROPIC_API_KEY unset, it should be skipped.
	t.Setenv("ANTHROPIC_API_KEY", "")
	configs := ResolveCredentials(entries, "ws")
	assert.Empty(t, configs)

	// With ANTHROPIC_API_KEY set, it should pass.
	t.Setenv("ANTHROPIC_API_KEY", "real-key")
	configs = ResolveCredentials(entries, "ws")
	require.Len(t, configs, 1)
	assert.True(t, configs[0].FromExisting)
}

func TestResolveCredentials_GithubEitherTokenWorks(t *testing.T) {
	// Only GH_TOKEN set, GITHUB_TOKEN not set.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token-value")

	entries := []CredentialInput{
		{Type: "github"},
	}

	configs := ResolveCredentials(entries, "ws")
	require.Len(t, configs, 1)
	assert.Equal(t, "github", configs[0].Type)
}

func TestDetectCredentials_FindsSetVars(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("GITHUB_TOKEN", "ghp-test")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")

	detected := DetectCredentials()
	require.Len(t, detected, 2)
	assert.Equal(t, "claude", detected[0].Type)
	assert.Equal(t, "github", detected[1].Type)
}

func TestDetectCredentials_NoneSet(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")

	detected := DetectCredentials()
	assert.Empty(t, detected)
}

func TestDetectCredentials_DeduplicatesGithubTokens(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghp-1")
	t.Setenv("GH_TOKEN", "gh-2")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")

	detected := DetectCredentials()
	// Should only detect github once, not twice.
	githubCount := 0
	for _, d := range detected {
		if d.Type == "github" {
			githubCount++
		}
	}
	assert.Equal(t, 1, githubCount)
}

func TestUploadFileCredential_FileNotFound(t *testing.T) {
	ctx := context.Background()
	client := &mockClient{}

	err := UploadFileCredential(ctx, client, "sb-123", "/nonexistent/path/sa.json", "/sandbox/.config/gcloud/credentials.json", "GOOGLE_APPLICATION_CREDENTIALS")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUploadFileCredential_UploadError(t *testing.T) {
	ctx := context.Background()

	// Create a temp file to upload.
	tmpFile, err := os.CreateTemp("", "test-cred-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(`{"type": "service_account"}`)
	tmpFile.Close()

	client := &mockClient{
		uploadErr: fmt.Errorf("gateway unreachable"),
	}

	err = UploadFileCredential(ctx, client, "sb-123", tmpFile.Name(), "/sandbox/.config/gcloud/credentials.json", "GOOGLE_APPLICATION_CREDENTIALS")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uploading credential file")
}

func TestUploadFileCredential_Success(t *testing.T) {
	ctx := context.Background()

	tmpFile, err := os.CreateTemp("", "test-cred-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(`{"type": "service_account"}`)
	tmpFile.Close()

	client := &mockClient{}

	err = UploadFileCredential(ctx, client, "sb-123", tmpFile.Name(), "/sandbox/.config/gcloud/credentials.json", "GOOGLE_APPLICATION_CREDENTIALS")
	assert.NoError(t, err)
	assert.Equal(t, tmpFile.Name(), client.uploadedLocal)
	assert.Equal(t, "/sandbox/.config/gcloud/credentials.json", client.uploadedRemote)
	assert.Equal(t, 2, client.execCount) // .bashrc + .zshrc
}

func TestDetectCredentials_VertexDetection(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/path/to/sa.json")
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")

	detected := DetectCredentials()
	require.Len(t, detected, 1)
	assert.Equal(t, "vertex", detected[0].Type)
	assert.Equal(t, "GOOGLE_APPLICATION_CREDENTIALS", detected[0].File)
}
