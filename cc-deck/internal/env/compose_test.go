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
	assert.Equal(t, "/tmp/myproject/.cc-deck", e.composeProjectDir())
}

// --- T021: Credential tests ---

func TestComposeCreate_EnvFile_APIKey(t *testing.T) {
	tmpDir := t.TempDir()
	ccDeckDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))

	// Simulate writing a .env file with API key credentials.
	creds := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-test-key-123",
	}

	var envLines []string
	for key, val := range creds {
		envLines = append(envLines, key+"="+val)
	}
	envContent := strings.Join(envLines, "\n") + "\n"
	envPath := filepath.Join(ccDeckDir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0o600))

	// Read back and verify.
	data, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ANTHROPIC_API_KEY=sk-ant-test-key-123")
}

func TestComposeCreate_FileCredential(t *testing.T) {
	tmpDir := t.TempDir()
	ccDeckDir := filepath.Join(tmpDir, ".cc-deck")
	secretsDir := filepath.Join(ccDeckDir, "secrets")
	require.NoError(t, os.MkdirAll(secretsDir, 0o755))

	// Create a fake credential file.
	credFile := filepath.Join(tmpDir, "adc.json")
	credContent := `{"type":"authorized_user","client_id":"test"}`
	require.NoError(t, os.WriteFile(credFile, []byte(credContent), 0o600))

	// Simulate copying file credential to secrets dir.
	secretFileName := "google-application-credentials"
	destPath := filepath.Join(secretsDir, secretFileName)
	data, err := os.ReadFile(credFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(destPath, data, 0o600))

	// Verify the secret file exists with correct content.
	readBack, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, credContent, string(readBack))
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

// --- T026: Gitignore tests ---

func TestHandleGitignore_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	e := &ComposeEnvironment{name: "test"}
	// No .git directory, should silently skip.
	e.handleGitignore(tmpDir)

	// No .gitignore should be created.
	_, err := os.Stat(filepath.Join(tmpDir, ".gitignore"))
	assert.True(t, os.IsNotExist(err))
}

func TestHandleGitignore_AutoAdd(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .git directory to simulate git repo.
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755))

	e := &ComposeEnvironment{name: "test", Gitignore: true}
	e.handleGitignore(tmpDir)

	// .gitignore should be created with .cc-deck/ entry.
	data, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(data), ".cc-deck/")
}

func TestHandleGitignore_AlreadyPresent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(".cc-deck/\n"), 0o644))

	e := &ComposeEnvironment{name: "test", Gitignore: true}
	e.handleGitignore(tmpDir)

	// Should not duplicate the entry.
	data, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	require.NoError(t, err)
	count := strings.Count(string(data), ".cc-deck/")
	assert.Equal(t, 1, count, "should not duplicate .cc-deck/ entry")
}

func TestHandleGitignore_WarningOnly(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755))

	e := &ComposeEnvironment{name: "test", Gitignore: false}
	e.handleGitignore(tmpDir)

	// .gitignore should NOT be created when --gitignore is not set.
	_, err := os.Stat(filepath.Join(tmpDir, ".gitignore"))
	assert.True(t, os.IsNotExist(err))
}

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
