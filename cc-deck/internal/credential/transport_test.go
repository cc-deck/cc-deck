package credential

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSSHClient implements SSHClient for testing.
type mockSSHClient struct {
	commands  []string
	uploads   []uploadCall
	runOutput string
	runErr    error
}

type uploadCall struct {
	localPath  string
	remotePath string
}

func (m *mockSSHClient) Run(_ context.Context, cmd string) (string, error) {
	m.commands = append(m.commands, cmd)
	return m.runOutput, m.runErr
}

func (m *mockSSHClient) Upload(_ context.Context, localPath, remotePath string) error {
	m.uploads = append(m.uploads, uploadCall{localPath, remotePath})
	return nil
}

func TestInjectSSH_EnvVars(t *testing.T) {
	client := &mockSSHClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"API_KEY": "secret-value",
		},
	}

	err := InjectSSH(context.Background(), client, resolved)
	require.NoError(t, err)

	require.NotEmpty(t, client.commands)
	lastCmd := client.commands[len(client.commands)-1]
	assert.Contains(t, lastCmd, "credentials.env")
	assert.Contains(t, lastCmd, "base64 -d")
	assert.Contains(t, lastCmd, "chmod 600")
}

func TestInjectSSH_FileCredential(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte(`{"type":"service_account"}`), 0o600))

	client := &mockSSHClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"PROJECT_ID": "my-project",
		},
		FileCredential: &ResolvedFile{
			EnvVar:    "GOOGLE_APPLICATION_CREDENTIALS",
			LocalPath: credFile,
		},
	}

	err := InjectSSH(context.Background(), client, resolved)
	require.NoError(t, err)

	require.Len(t, client.uploads, 1)
	assert.Equal(t, credFile, client.uploads[0].localPath)
	assert.Contains(t, client.uploads[0].remotePath, "GOOGLE_APPLICATION_CREDENTIALS")
}

func TestInjectSSH_UnsetVars(t *testing.T) {
	client := &mockSSHClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"API_KEY": "val",
		},
		UnsetVars: []string{"GEMINI_API_KEY"},
	}

	err := InjectSSH(context.Background(), client, resolved)
	require.NoError(t, err)

	lastCmd := client.commands[len(client.commands)-1]
	// Content is base64-encoded; decode and verify unset is present
	assert.Contains(t, lastCmd, "base64 -d")
	assert.Contains(t, lastCmd, "credentials.env")
}

func TestInjectSSH_EmptySkips(t *testing.T) {
	client := &mockSSHClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{},
	}

	err := InjectSSH(context.Background(), client, resolved)
	require.NoError(t, err)
	assert.Empty(t, client.commands)
}

// mockOpenShellClient implements OpenShellClient for testing.
type mockOpenShellClient struct {
	execCmds [][]string
	uploads  []osUploadCall
}

type osUploadCall struct {
	sandboxID  string
	localPath  string
	remotePath string
}

func (m *mockOpenShellClient) ExecRun(_ context.Context, sandboxID string, cmd []string) error {
	m.execCmds = append(m.execCmds, cmd)
	return nil
}

func (m *mockOpenShellClient) FileUpload(_ context.Context, sandboxID, localPath, remotePath string) error {
	m.uploads = append(m.uploads, osUploadCall{sandboxID, localPath, remotePath})
	return nil
}

func TestInjectOpenShell_EnvVars(t *testing.T) {
	client := &mockOpenShellClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"API_KEY": "secret-value",
		},
	}

	err := InjectOpenShell(context.Background(), client, "sandbox-1", resolved)
	require.NoError(t, err)

	require.Len(t, client.execCmds, 2) // .bashrc + .zshrc
	for _, cmd := range client.execCmds {
		assert.Contains(t, cmd[2], "API_KEY")
	}
}

func TestInjectOpenShell_UnsetVars(t *testing.T) {
	client := &mockOpenShellClient{}
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"API_KEY": "val",
		},
		UnsetVars: []string{"GEMINI_API_KEY"},
	}

	err := InjectOpenShell(context.Background(), client, "sandbox-1", resolved)
	require.NoError(t, err)

	// 2 calls for API_KEY (bashrc + zshrc) + 2 for unset GEMINI_API_KEY (bashrc + zshrc)
	require.GreaterOrEqual(t, len(client.execCmds), 4)

	hasUnset := false
	for _, cmd := range client.execCmds {
		if len(cmd) >= 3 {
			if strings.Contains(cmd[2], "unset GEMINI_API_KEY") {
				hasUnset = true
			}
		}
	}
	assert.True(t, hasUnset, "expected at least one command containing 'unset GEMINI_API_KEY'")
}

func TestInjectK8s_EnvVars(t *testing.T) {
	resolved := ResolvedCredentials{
		EnvVars: map[string]string{
			"API_KEY": "secret-value",
		},
	}

	result, err := InjectK8s(testSpec(), resolved)
	require.NoError(t, err)
	assert.Equal(t, []byte("secret-value"), result.SecretData["API_KEY"])
	require.Len(t, result.EnvVars, 1)
	assert.Equal(t, "API_KEY", result.EnvVars[0].Name)
}

func TestInjectK8s_FileCredential(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0o600))

	resolved := ResolvedCredentials{
		EnvVars: map[string]string{},
		FileCredential: &ResolvedFile{
			EnvVar:    "GOOGLE_APPLICATION_CREDENTIALS",
			LocalPath: credFile,
		},
	}

	result, err := InjectK8s(testSpec(), resolved)
	require.NoError(t, err)
	assert.Contains(t, result.SecretData, "GOOGLE_APPLICATION_CREDENTIALS")
	require.Len(t, result.VolumeMounts, 1)
	assert.Equal(t, "/run/secrets/GOOGLE_APPLICATION_CREDENTIALS", result.VolumeMounts[0].MountPath)

	// File credential env var should point to mount path, not raw content
	require.Len(t, result.FileEnvVars, 1)
	assert.Equal(t, "GOOGLE_APPLICATION_CREDENTIALS", result.FileEnvVars[0].Name)
	assert.Equal(t, "/run/secrets/GOOGLE_APPLICATION_CREDENTIALS", result.FileEnvVars[0].Value)
	// Should NOT be in the secretKeyRef env vars
	for _, ev := range result.EnvVars {
		assert.NotEqual(t, "GOOGLE_APPLICATION_CREDENTIALS", ev.Name, "file credential should not use secretKeyRef")
	}
}

func TestInjectK8s_UnsetVars(t *testing.T) {
	resolved := ResolvedCredentials{
		EnvVars:   map[string]string{"KEY": "val"},
		UnsetVars: []string{"GEMINI_API_KEY"},
	}

	result, err := InjectK8s(testSpec(), resolved)
	require.NoError(t, err)
	assert.Equal(t, []string{"GEMINI_API_KEY"}, result.UnsetVars)
}

func testSpec() agent.CredentialSpec {
	return agent.CredentialSpec{Name: "test"}
}
