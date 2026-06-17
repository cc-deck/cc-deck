//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/stretchr/testify/require"
)

func TestImageSession(t *testing.T) {
	hasAnthropicKey := os.Getenv("ANTHROPIC_API_KEY") != ""
	hasVertexCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != ""
	if !hasAnthropicKey && !hasVertexCreds {
		t.Skip("No API credentials set (ANTHROPIC_API_KEY or GOOGLE_APPLICATION_CREDENTIALS), skipping session tests")
	}

	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not found, skipping session tests")
	}

	repoRoot := mustFindProjectRoot()
	registryPath := filepath.Join(repoRoot, "..", "base-images.yaml")

	reg, err := build.LoadBaseImageRegistry(registryPath)
	require.NoError(t, err)

	for _, target := range []string{"openshell", "container"} {
		entries := reg.EntriesForTarget(target)
		for _, entry := range entries {
			if !entry.Default {
				continue
			}

			t.Run(fmt.Sprintf("session/%s/%s", target, entry.Name), func(t *testing.T) {
				sessionSmoke(t, target, entry)
			})
		}
	}
}

func sessionSmoke(t *testing.T, target string, entry build.BaseImageEntry) {
	t.Helper()

	containerName := fmt.Sprintf("cc-deck-session-%s-%s", target, entry.Name)
	imageName := fmt.Sprintf("cc-deck-probe-%s-%s:test", target, entry.Name)

	checkCmd := exec.Command("podman", "image", "exists", imageName)
	if err := checkCmd.Run(); err != nil {
		t.Skipf("Image %s not found; run test-images first", imageName)
	}

	envArgs := []string{}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		envArgs = append(envArgs, "-e", "ANTHROPIC_API_KEY="+key)
	}
	if creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); creds != "" {
		envArgs = append(envArgs, "-e", "GOOGLE_APPLICATION_CREDENTIALS="+creds)
	}

	runArgs := append([]string{"run", "-d", "--name", containerName}, envArgs...)
	runArgs = append(runArgs, imageName, "sleep", "120")
	runCmd := exec.Command("podman", runArgs...)
	require.NoError(t, runCmd.Run(), "failed to start container")

	t.Cleanup(func() {
		exec.Command("podman", "rm", "-f", containerName).Run()
	})

	exec.Command("podman", "exec", "-d", containerName, "cc-deck", "run").Run()

	deadline := time.Now().Add(60 * time.Second)
	zellijRunning := false
	for time.Now().Before(deadline) {
		var out bytes.Buffer
		cmd := exec.Command("podman", "exec", containerName, "pgrep", "-f", "zellij")
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil && strings.TrimSpace(out.String()) != "" {
			zellijRunning = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !zellijRunning {
		t.Fatal("Zellij did not start within 60 seconds")
	}

	t.Log("Zellij started successfully")

	var pluginOut bytes.Buffer
	pluginCmd := exec.Command("podman", "exec", containerName,
		"sh", "-c", "ls /proc/*/cmdline 2>/dev/null | head -20")
	pluginCmd.Stdout = &pluginOut
	pluginCmd.Run()

	t.Logf("Container processes after Zellij start: %s", pluginOut.String())
}
