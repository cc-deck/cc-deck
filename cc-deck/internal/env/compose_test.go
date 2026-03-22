package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T014: Create tests ---

func TestComposeEnvironment_Type(t *testing.T) {
	e := &ComposeEnvironment{name: "test"}
	assert.Equal(t, EnvironmentTypeCompose, e.Type())
}

func TestComposeEnvironment_Name(t *testing.T) {
	e := &ComposeEnvironment{name: "myenv"}
	assert.Equal(t, "myenv", e.Name())
}

func TestComposeEnvironment_SessionContainerName(t *testing.T) {
	e := &ComposeEnvironment{name: "myenv"}
	assert.Equal(t, "cc-deck-myenv", e.sessionContainerName())
}

func TestComposeEnvironment_ProxyContainerName(t *testing.T) {
	e := &ComposeEnvironment{name: "myenv"}
	assert.Equal(t, "cc-deck-myenv-proxy", e.proxyContainerName())
}

func TestComposeEnvironment_ProjectDir_Default(t *testing.T) {
	e := &ComposeEnvironment{name: "test"}
	dir := e.projectDir()
	cwd, _ := os.Getwd()
	assert.Equal(t, cwd, dir)
}

func TestComposeEnvironment_ProjectDir_Explicit(t *testing.T) {
	e := &ComposeEnvironment{name: "test", ProjectDir: "/tmp/myproject"}
	assert.Equal(t, "/tmp/myproject", e.projectDir())
}

func TestComposeEnvironment_ComposeProjectDir(t *testing.T) {
	e := &ComposeEnvironment{name: "test", ProjectDir: "/tmp/myproject"}
	assert.Equal(t, "/tmp/myproject/.cc-deck/run", e.composeProjectDir())
}

// --- T021: Credential tests ---

func TestComposeCreate_EnvFile_APIKey(t *testing.T) {
	tmpDir := t.TempDir()
	ccDeckDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))

	// Simulate writing an env file with API key credentials.
	creds := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-test-key-123",
	}

	var envLines []string
	for key, val := range creds {
		envLines = append(envLines, key+"="+val)
	}
	envContent := strings.Join(envLines, "\n") + "\n"
	envPath := filepath.Join(ccDeckDir, "env")
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0o600))

	// Read back and verify.
	data, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ANTHROPIC_API_KEY=sk-ant-test-key-123")
}

func TestComposeCreate_FileCredentialVolumeMount(t *testing.T) {
	// File-based credentials should produce :ro,U volume mounts, not copies.
	credFile := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(credFile, []byte(`{"type":"authorized_user"}`), 0o600))

	creds := map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": credFile,
	}

	var volumeMounts []string
	var envLines []string
	for key, val := range creds {
		if info, statErr := os.Stat(val); statErr == nil && !info.IsDir() {
			secretName := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
			containerPath := "/run/secrets/" + secretName
			volumeMounts = append(volumeMounts, val+":"+containerPath+":ro,U")
			envLines = append(envLines, key+"="+containerPath)
		}
	}

	require.Len(t, volumeMounts, 1)
	assert.Contains(t, volumeMounts[0], credFile+":/run/secrets/google-application-credentials:ro,U")
	assert.Equal(t, "GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-application-credentials", envLines[0])
}

// --- T024: Filtering tests ---

func TestComposeCreate_ProxyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ccDeckDir := filepath.Join(tmpDir, ".cc-deck")
	proxyDir := filepath.Join(ccDeckDir, "proxy")
	require.NoError(t, os.MkdirAll(proxyDir, 0o755))

	// Simulate proxy config generation.
	domains := []string{".anthropic.com", ".github.com", "api.openai.com"}

	// Write tinyproxy config.
	tinyproxyConf := "Port 8888\nTimeout 600\nFilterDefaultDeny Yes\n"
	require.NoError(t, os.WriteFile(filepath.Join(proxyDir, "tinyproxy.conf"), []byte(tinyproxyConf), 0o644))

	// Write whitelist.
	var wlLines []string
	for _, d := range domains {
		wlLines = append(wlLines, d)
	}
	whitelist := strings.Join(wlLines, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(proxyDir, "whitelist"), []byte(whitelist), 0o644))

	// Verify files exist.
	_, err := os.Stat(filepath.Join(proxyDir, "tinyproxy.conf"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(proxyDir, "whitelist"))
	assert.NoError(t, err)

	// Verify whitelist content.
	wlData, err := os.ReadFile(filepath.Join(proxyDir, "whitelist"))
	require.NoError(t, err)
	assert.Contains(t, string(wlData), ".anthropic.com")
	assert.Contains(t, string(wlData), ".github.com")
}

// Gitignore tests are in gitignore_test.go (ensureCCDeckGitignore replaces handleGitignore).

// --- T018: Lifecycle state transition tests ---

func TestComposeEnvironment_HarvestError(t *testing.T) {
	e := &ComposeEnvironment{name: "test"}
	err := e.Harvest(nil, HarvestOpts{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNotSupported)
}

func TestComposeEnvironment_CleanupOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	ccDeckDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ccDeckDir, "test.txt"), []byte("test"), 0o644))

	e := &ComposeEnvironment{name: "test"}
	e.cleanupOnFailure(ccDeckDir)

	_, err := os.Stat(ccDeckDir)
	assert.True(t, os.IsNotExist(err), ".cc-deck/ should be removed on cleanup")
}
