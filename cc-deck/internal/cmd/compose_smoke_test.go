package cmd_test

import (
	"fmt"
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

	// 2. Verify .cc-deck/run/ generated
	ccDeckRunDir := filepath.Join(projectDir, ".cc-deck", "run")
	assert.DirExists(t, ccDeckRunDir)
	assert.FileExists(t, filepath.Join(ccDeckRunDir, "compose.yaml"))
	assert.FileExists(t, filepath.Join(ccDeckRunDir, "env"))

	// 3. Verify compose.yaml content
	composeYAML, err := os.ReadFile(filepath.Join(ccDeckRunDir, "compose.yaml"))
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
	_, err = os.Stat(filepath.Join(projectDir, ".cc-deck", "run"))
	assert.True(t, os.IsNotExist(err), ".cc-deck/run/ should be removed")

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

	ccDeckRunDir := filepath.Join(projectDir, ".cc-deck", "run")

	// Verify proxy config files.
	assert.FileExists(t, filepath.Join(ccDeckRunDir, "proxy", "tinyproxy.conf"))
	assert.FileExists(t, filepath.Join(ccDeckRunDir, "proxy", "whitelist"))

	whitelist, err := os.ReadFile(filepath.Join(ccDeckRunDir, "proxy", "whitelist"))
	require.NoError(t, err)
	assert.Contains(t, string(whitelist), "anthropic")

	tinyConf, err := os.ReadFile(filepath.Join(ccDeckRunDir, "proxy", "tinyproxy.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(tinyConf), "FilterDefaultDeny Yes")

	// Verify compose.yaml has proxy service.
	composeYAML, err := os.ReadFile(filepath.Join(ccDeckRunDir, "compose.yaml"))
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

// nonRootTestImage builds a minimal test image with a non-root user.
// Returns the image name. The image is removed on test cleanup.
func nonRootTestImage(t *testing.T) string {
	t.Helper()
	imageName := fmt.Sprintf("localhost/cc-deck-test-nonroot:%d", os.Getpid())
	containerfile := filepath.Join(t.TempDir(), "Containerfile")
	require.NoError(t, os.WriteFile(containerfile, []byte(
		"FROM fedora:latest\nRUN useradd -m devuser\nUSER devuser\n"), 0o644))
	out, err := exec.Command("podman", "build", "-t", imageName,
		"-f", containerfile, filepath.Dir(containerfile)).CombinedOutput()
	require.NoError(t, err, "failed to build non-root test image: %s", out)
	t.Cleanup(func() {
		_ = exec.Command("podman", "rmi", imageName).Run()
	})
	return imageName
}

func TestComposeSmokeWritePermissionsNonRoot(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)
	image := nonRootTestImage(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-perms", "--force")
	}()

	_, err := ccd(t, bin, envVars, "env", "create", "smoke-perms",
		"--type", "compose",
		"--image", image,
		"--path", projectDir)
	require.NoError(t, err)

	// Verify the container user is non-root.
	podOut, err := exec.Command("podman", "exec", "cc-deck-smoke-perms",
		"id", "-u").Output()
	require.NoError(t, err)
	uid := strings.TrimSpace(string(podOut))
	assert.NotEqual(t, "0", uid, "container should run as non-root user")

	// Container should be able to write to /workspace (the bind mount).
	podOut, err = exec.Command("podman", "exec", "cc-deck-smoke-perms",
		"sh", "-c", "echo 'write-test' > /workspace/nonroot-write.txt && cat /workspace/nonroot-write.txt").CombinedOutput()
	require.NoError(t, err, "non-root user should be able to write to /workspace: %s", podOut)
	assert.Contains(t, string(podOut), "write-test")

	// Verify file exists on host.
	data, err := os.ReadFile(filepath.Join(projectDir, "nonroot-write.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "write-test")
}

func TestComposeSmokeNamedVolume(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-vol", "--force")
	}()

	// Create with named-volume storage.
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-vol",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir,
		"--storage", "named-volume")
	require.NoError(t, err, "create with named-volume failed: %s", out)

	// Verify compose.yaml declares the volume as external.
	composeYAML, err := os.ReadFile(filepath.Join(projectDir, ".cc-deck", "run", "compose.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(composeYAML), "external: true",
		"compose.yaml should declare external volume")

	// Verify container is running.
	podOut, err := exec.Command("podman", "inspect", "cc-deck-smoke-vol",
		"--format", "{{.State.Status}}").Output()
	require.NoError(t, err)
	assert.Equal(t, "running", strings.TrimSpace(string(podOut)))

	// Verify volume exists.
	podOut, err = exec.Command("podman", "volume", "inspect",
		"cc-deck-smoke-vol-data").Output()
	require.NoError(t, err, "named volume should exist")

	// Write data, stop, start, verify data persists.
	_, err = exec.Command("podman", "exec", "cc-deck-smoke-vol",
		"sh", "-c", "echo 'persist-test' > /workspace/persist.txt").CombinedOutput()
	require.NoError(t, err)

	_, err = ccd(t, bin, envVars, "env", "stop", "smoke-vol")
	require.NoError(t, err)

	_, err = ccd(t, bin, envVars, "env", "start", "smoke-vol")
	require.NoError(t, err)

	podOut, err = exec.Command("podman", "exec", "cc-deck-smoke-vol",
		"cat", "/workspace/persist.txt").Output()
	require.NoError(t, err)
	assert.Contains(t, string(podOut), "persist-test",
		"data should persist across stop/start")
}

func TestComposeSmokeRecreateAfterDelete(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-recreate", "--force")
	}()

	// First create.
	_, err := ccd(t, bin, envVars, "env", "create", "smoke-recreate",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir)
	require.NoError(t, err)

	// Verify running.
	podOut, err := exec.Command("podman", "inspect", "cc-deck-smoke-recreate",
		"--format", "{{.State.Status}}").Output()
	require.NoError(t, err)
	assert.Equal(t, "running", strings.TrimSpace(string(podOut)))

	// Delete.
	_, err = ccd(t, bin, envVars, "env", "delete", "smoke-recreate", "--force")
	require.NoError(t, err)

	// Second create should succeed (no stale resource conflicts).
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-recreate",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir)
	require.NoError(t, err, "re-create after delete should succeed: %s", out)
	assert.Contains(t, out, "created")

	// Verify running again.
	podOut, err = exec.Command("podman", "inspect", "cc-deck-smoke-recreate",
		"--format", "{{.State.Status}}").Output()
	require.NoError(t, err)
	assert.Equal(t, "running", strings.TrimSpace(string(podOut)))
}

func TestComposeSmokeDuplicateNameFailsFast(t *testing.T) {
	skipIfNoCompose(t)
	bin := buildTestBinary(t)
	envVars, projectDir := setupComposeSmokeEnv(t)

	defer func() {
		ccd(t, bin, envVars, "env", "delete", "smoke-dup", "--force")
	}()

	// First create should succeed.
	_, err := ccd(t, bin, envVars, "env", "create", "smoke-dup",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir)
	require.NoError(t, err)

	// Second create should fail fast.
	projectDir2 := t.TempDir()
	out, err := ccd(t, bin, envVars, "env", "create", "smoke-dup",
		"--type", "compose",
		"--image", "fedora:latest",
		"--path", projectDir2)
	assert.Error(t, err, "duplicate create should fail")
	assert.Contains(t, out, "already exists", "should report name conflict")

	// No .cc-deck/ directory should be created in the second project dir.
	_, statErr := os.Stat(filepath.Join(projectDir2, ".cc-deck"))
	assert.True(t, os.IsNotExist(statErr),
		".cc-deck/ should NOT be created when duplicate name is rejected")
}
