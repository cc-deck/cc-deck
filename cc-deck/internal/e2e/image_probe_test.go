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

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/stretchr/testify/require"
)

func TestImageProbe(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not found, skipping image probe tests")
	}

	repoRoot := mustFindProjectRoot()
	registryPath := filepath.Join(repoRoot, "..", "base-images.yaml")

	reg, err := build.LoadBaseImageRegistry(registryPath)
	require.NoError(t, err, "base-images.yaml must exist at repo root")

	targets := []struct {
		target string
		user   string
		home   string
		shell  string
	}{
		{"openshell", "sandbox", "/sandbox", "zsh"},
		{"container", "dev", "/home/dev", "zsh"},
	}

	for _, tgt := range targets {
		entries := reg.EntriesForTarget(tgt.target)
		if len(entries) == 0 {
			continue
		}

		baseFilter := os.Getenv("BASE")

		for _, entry := range entries {
			if baseFilter != "" && entry.Name != baseFilter {
				continue
			}
			if os.Getenv("DEFAULTS_ONLY") == "1" && !entry.Default {
				continue
			}

			t.Run(fmt.Sprintf("%s/%s", tgt.target, entry.Name), func(t *testing.T) {
				if !entry.Default {
					t.Log("Non-default base image; failures are warnings, not errors")
				}

				imageName := fmt.Sprintf("cc-deck-probe-%s-%s:test", tgt.target, entry.Name)

				buildAndProbe(t, repoRoot, tgt.target, entry, imageName,
					tgt.user, tgt.home, tgt.shell, entry.Default)
			})
		}
	}
}

func buildAndProbe(t *testing.T, repoRoot, target string, entry build.BaseImageEntry,
	imageName, user, home, shell string, isDefault bool) {
	t.Helper()

	m := &build.Manifest{Version: 3}
	data := build.ContainerDataForTarget(m, target)
	if data == nil {
		t.Fatalf("unsupported target: %s", target)
	}
	data.BaseImage = entry.Ref

	snippets, err := build.RenderContainerfileSnippets(data)
	if err != nil {
		if !isDefault {
			t.Skipf("WARNING: template render failed for %s: %v", entry.Name, err)
		}
		t.Fatalf("template render failed: %v", err)
	}

	var containerfile strings.Builder
	for _, name := range []string{
		"01-header", "02-user-setup", "03-mandatory-stack",
		"04-openshell-extras", "05-shell-finalize",
		"055-openshell-policy", "06-footer",
	} {
		if s, ok := snippets[name]; ok {
			containerfile.WriteString(s)
		}
	}

	buildDir := t.TempDir()
	cfPath := filepath.Join(buildDir, "Containerfile")
	require.NoError(t, os.WriteFile(cfPath, []byte(containerfile.String()), 0o644))

	contextDir := filepath.Join(buildDir, target, "context")
	require.NoError(t, os.MkdirAll(contextDir, 0o755))

	for _, arch := range []string{"amd64", "arm64"} {
		src := filepath.Join(repoRoot, "cc-deck", fmt.Sprintf("cc-deck-linux-%s", arch))
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(contextDir, fmt.Sprintf("cc-deck-linux-%s", arch))
			copyFile(t, src, dst)
		}
	}

	buildCmd := exec.Command("podman", "build", "-f", cfPath, "-t", imageName, buildDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		if !isDefault {
			t.Skipf("WARNING: build failed for %s: %v", entry.Name, err)
		}
		t.Fatalf("build failed: %v", err)
	}

	t.Cleanup(func() {
		exec.Command("podman", "rmi", "-f", imageName).Run()
	})

	containerName := fmt.Sprintf("cc-deck-probe-%s-%s", target, entry.Name)
	runCmd := exec.Command("podman", "run", "-d", "--name", containerName,
		imageName, "sleep", "300")
	var runOut bytes.Buffer
	runCmd.Stdout = &runOut
	runCmd.Stderr = os.Stderr
	require.NoError(t, runCmd.Run(), "failed to start container")

	t.Cleanup(func() {
		exec.Command("podman", "rm", "-f", containerName).Run()
	})

	checks := ContainerProbeChecks(user, home, shell)
	for _, check := range checks {
		t.Run(check.Name, func(t *testing.T) {
			args := append([]string{"exec", containerName}, check.Command...)
			cmd := exec.Command("podman", args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			exitCode := 0
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					if !isDefault {
						t.Skipf("WARNING: probe exec failed for %s/%s: %v",
							entry.Name, check.Name, err)
					}
					t.Fatalf("exec failed: %v", err)
				}
			}

			if err := check.Check(exitCode, stdout.String()); err != nil {
				if !isDefault {
					t.Logf("WARNING: probe %s failed (non-default, non-fatal): %v", check.Name, err)
					return
				}
				t.Errorf("probe %s failed: %v", check.Name, err)
			}
		})
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, data, 0o755))
}
