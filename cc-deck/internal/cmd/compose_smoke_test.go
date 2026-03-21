package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoCompose skips the test if podman-compose is not available.
func skipIfNoCompose(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("podman-compose"); err != nil {
		t.Skip("podman-compose not available, skipping compose smoke test")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping compose smoke test")
	}
}

// buildTestBinary builds the cc-deck binary for smoke testing.
// Returns the path to the built binary.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "cc-deck-test")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cc-deck")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}
	return binPath
}

// ccd runs the cc-deck CLI binary with the given args and env overrides.
func ccd(t *testing.T, bin string, env map[string]string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// setupComposeSmokeEnv creates isolated state/definition files and a project dir.
func setupComposeSmokeEnv(t *testing.T) (envVars map[string]string, projectDir string) {
	t.Helper()
	stateDir := t.TempDir()
	projectDir = t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "main.go"),
		[]byte("package main\nfunc main() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "go.mod"),
		[]byte("module test-project\n"), 0o644))

	envVars = map[string]string{
		"CC_DECK_STATE_FILE":       filepath.Join(stateDir, "state.yaml"),
		"CC_DECK_DEFINITIONS_FILE": filepath.Join(stateDir, "environments.yaml"),
	}
	return envVars, projectDir
}

func TestComposeSmokeFullLifecycle(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	// Cleanup on exit.
	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-full", "--force")
	}()

	// 1. Create
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-full",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir,
		"--credential", "MY_KEY=test123")
	require.NoError(t, err, "create failed: %s", out)
	assert.Contains(t, out, "created")

	// 2. Verify .cc-deck/ generated
	ccDeckDir := filepath.Join(projectDir, ".cc-deck")
	assert.DirExists(t, ccDeckDir)
	assert.FileExists(t, filepath.Join(ccDeckDir, "compose.yaml"))
	assert.FileExists(t, filepath.Join(ccDeckDir, ".env"))

	// 3. Verify compose.yaml content
	composeYAML, err := os.ReadFile(filepath.Join(ccDeckDir, "compose.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(composeYAML), "cc-deck-smoke-full")
	assert.Contains(t, string(composeYAML), "fedora:latest")
	assert.Contains(t, string(composeYAML), "stdin_open: true")
	assert.Contains(t, string(composeYAML), "/workspace")

	// 4. Verify container is running
	podOut, err := exec.Command("podman", "inspect", "cc-deck-smoke-full",
		"--format", "{{.State.Status}}").Output()
	require.NoError(t, err)
	assert.Equal(t, "running", strings.TrimSpace(string(podOut)))

	// 5. Verify bind mount (project files visible)
	podOut, err = exec.Command("podman", "exec", "cc-deck-smoke-full",
		"ls", "/workspace/").Output()
	require.NoError(t, err)
	assert.Contains(t, string(podOut), "main.go")

	// 6. Verify credential available inside container
	podOut, err = exec.Command("podman", "exec", "cc-deck-smoke-full",
		"sh", "-c", "echo $MY_KEY").Output()
	require.NoError(t, err)
	assert.Equal(t, "test123", strings.TrimSpace(string(podOut)))

	// 7. List shows compose type
	out, err = ccd(t, bin, envVars, "env", "list")
	require.NoError(t, err, "list failed: %s", out)
	assert.Contains(t, out, "smoke-full")
	assert.Contains(t, out, "compose")

	// 8. Exec works
	out, err = ccd(t, bin, envVars, "env", "exec", "smoke-full", "--", "cat", "/etc/os-release")
	require.NoError(t, err, "exec failed: %s", out)
	assert.Contains(t, out, "Fedora")

	// 9. Stop
	out, err = ccd(t, bin, envVars, "env", "stop", "smoke-full")
	require.NoError(t, err, "stop failed: %s", out)
	assert.Contains(t, out, "stopped")

	// 10. Start
	out, err = ccd(t, bin, envVars, "env", "start", "smoke-full")
	require.NoError(t, err, "start failed: %s", out)
	assert.Contains(t, out, "started")

	// 11. Status
	out, err = ccd(t, bin, envVars, "env", "status", "smoke-full")
	require.NoError(t, err, "status failed: %s", out)
	assert.Contains(t, out, "compose")

	// 12. Delete
	out, err = ccd(t, bin, envVars, "env", "delete", "smoke-full", "--force")
	require.NoError(t, err, "delete failed: %s", out)
	assert.Contains(t, out, "deleted")

	// 13. Verify cleanup
	_, err = os.Stat(ccDeckDir)
	assert.True(t, os.IsNotExist(err), ".cc-deck/ should be removed")

	_, err = exec.Command("podman", "inspect", "cc-deck-smoke-full").Output()
	assert.Error(t, err, "container should not exist after delete")
}

func TestComposeSmokeNetworkFiltering(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-filter", "--force")
	}()

	// Create with allowed domains.
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-filter",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir,
		"--allowed-domains", "anthropic")
	require.NoError(t, err, "create failed: %s", out)

	ccDeckDir := filepath.Join(projectDir, ".cc-deck")

	// Verify proxy config files.
	assert.FileExists(t, filepath.Join(ccDeckDir, "proxy", "tinyproxy.conf"))
	assert.FileExists(t, filepath.Join(ccDeckDir, "proxy", "whitelist"))

	whitelist, err := os.ReadFile(filepath.Join(ccDeckDir, "proxy", "whitelist"))
	require.NoError(t, err)
	assert.Contains(t, string(whitelist), "anthropic")

	tinyConf, err := os.ReadFile(filepath.Join(ccDeckDir, "proxy", "tinyproxy.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(tinyConf), "FilterDefaultDeny Yes")

	// Verify compose.yaml has proxy service.
	composeYAML, err := os.ReadFile(filepath.Join(ccDeckDir, "compose.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(composeYAML), "proxy:")
	assert.Contains(t, string(composeYAML), "HTTP_PROXY")
	assert.Contains(t, string(composeYAML), "internal:")

	// Verify proxy container is running.
	podOut, err := exec.Command("podman", "inspect", "cc-deck-smoke-filter-proxy",
		"--format", "{{.State.Status}}").Output()
	require.NoError(t, err)
	assert.Equal(t, "running", strings.TrimSpace(string(podOut)))
}

func TestComposeSmokeGitignore(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-gitignore", "--force")
	}()

	// Initialize git repo.
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = projectDir
	require.NoError(t, gitCmd.Run())

	// Create with --gitignore.
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-gitignore",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir,
		"--gitignore")
	require.NoError(t, err, "create failed: %s", out)

	// Verify .gitignore contains .cc-deck/
	gitignore, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), ".cc-deck/")
}

func TestComposeSmokeDeleteRefusesRunning(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-nodelete", "--force")
	}()

	_, err := ccd(t, bin, envVars, "env", "create", "smoke-nodelete",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir)
	require.NoError(t, err)

	// Delete without --force should fail.
	out, err := ccd(t, bin, envVars, "env", "delete", "smoke-nodelete")
	assert.Error(t, err, "delete without --force should fail")
	assert.Contains(t, out, "running")
}

func TestComposeSmokeBindMountSync(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-sync", "--force")
	}()

	_, err := ccd(t, bin, envVars, "env", "create", "smoke-sync",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir)
	require.NoError(t, err)

	// Host → Container: create file on host, verify inside container.
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "host-file.txt"),
		[]byte("from host"), 0o644))

	podOut, err := exec.Command("podman", "exec", "cc-deck-smoke-sync",
		"cat", "/workspace/host-file.txt").Output()
	require.NoError(t, err)
	assert.Equal(t, "from host", string(podOut))

	// Container → Host: create file in container, verify on host.
	_, err = exec.Command("podman", "exec", "cc-deck-smoke-sync",
		"sh", "-c", "echo 'from container' > /workspace/container-file.txt").Output()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "container-file.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "from container")
}
