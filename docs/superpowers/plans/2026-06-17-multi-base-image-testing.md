# Multi-Base-Image Testing & Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make cc-deck's build pipeline testable against multiple base images per target type, with a tiered e2e probe suite and a discovery skill for tracking upstream changes.

**Architecture:** A `base-images.yaml` config file at the repo root lists known base images grouped by target type. A new `registry.go` package loads this file and provides default resolution. The existing `Manifest.BaseImage()` and `Manifest.OpenShellBaseImage()` methods fall back to the registry instead of hardcoded constants. A Go e2e test suite builds images from each base and runs probe checks. A Claude Code skill discovers new/updated images via `skopeo inspect` and GitHub API.

**Tech Stack:** Go 1.25 (existing), gopkg.in/yaml.v3 (existing), podman (container builds), skopeo (image inspection), testify (existing test dep)

---

### Task 1: Create `base-images.yaml` with current defaults

**Files:**
- Create: `base-images.yaml`
- Modify: `.gitignore`

- [ ] **Step 1: Create the base images registry file**

```yaml
# Base image registry for cc-deck build pipeline.
# Each entry lists a base image that cc-deck can build on top of.
# The `default: true` entry is used when no `base:` is specified in the manifest.
openshell:
  - name: nvidia-upstream
    ref: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    default: true
container:
  - name: fedora-41
    ref: registry.fedoraproject.org/fedora:41
    default: true
```

Write this to `base-images.yaml` at the repo root.

- [ ] **Step 2: Add digest tracking file to `.gitignore`**

Append this line to `.gitignore`:

```
# Base image digest tracking (local state)
.base-images-digests.json
```

- [ ] **Step 3: Verify the file parses correctly**

Run: `yq '.' base-images.yaml`

Expected: The YAML is echoed back with no errors.

- [ ] **Step 4: Commit**

```bash
git add base-images.yaml .gitignore
git commit -m "feat: add base-images.yaml registry with current defaults"
```

---

### Task 2: Implement `registry.go` with types and loader

**Files:**
- Create: `cc-deck/internal/build/registry.go`
- Create: `cc-deck/internal/build/registry_test.go`

- [ ] **Step 1: Write the failing tests for `LoadBaseImageRegistry`**

Create `cc-deck/internal/build/registry_test.go`:

```go
package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBaseImageRegistry(t *testing.T) {
	content := `openshell:
  - name: nvidia-upstream
    ref: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    default: true
  - name: rh-ubi
    ref: quay.io/aipcc/openshell-base:latest
container:
  - name: fedora-41
    ref: registry.fedoraproject.org/fedora:41
    default: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	reg, err := LoadBaseImageRegistry(path)
	require.NoError(t, err)
	assert.Len(t, reg.OpenShell, 2)
	assert.Len(t, reg.Container, 1)
	assert.Equal(t, "nvidia-upstream", reg.OpenShell[0].Name)
	assert.Equal(t, "ghcr.io/nvidia/openshell-community/sandboxes/base:latest", reg.OpenShell[0].Ref)
	assert.True(t, reg.OpenShell[0].Default)
	assert.False(t, reg.OpenShell[1].Default)
}

func TestLoadBaseImageRegistry_FileNotFound(t *testing.T) {
	_, err := LoadBaseImageRegistry("/nonexistent/base-images.yaml")
	assert.Error(t, err)
}

func TestLoadBaseImageRegistry_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml:"), 0o644))

	_, err := LoadBaseImageRegistry(path)
	assert.Error(t, err)
}

func TestBaseImageRegistry_DefaultRef(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest", Default: false},
			{Name: "b", Ref: "img-b:latest", Default: true},
		},
		Container: []BaseImageEntry{
			{Name: "c", Ref: "img-c:latest", Default: true},
		},
	}

	assert.Equal(t, "img-b:latest", reg.DefaultRef("openshell"))
	assert.Equal(t, "img-c:latest", reg.DefaultRef("container"))
	assert.Equal(t, "", reg.DefaultRef("unknown"))
}

func TestBaseImageRegistry_DefaultRef_NoDefault(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest"},
		},
	}
	assert.Equal(t, "", reg.DefaultRef("openshell"))
}

func TestBaseImageRegistry_EntriesForTarget(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest"},
			{Name: "b", Ref: "img-b:latest"},
		},
	}

	entries := reg.EntriesForTarget("openshell")
	assert.Len(t, entries, 2)

	entries = reg.EntriesForTarget("container")
	assert.Nil(t, entries)

	entries = reg.EntriesForTarget("unknown")
	assert.Nil(t, entries)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cc-deck && go test -run "TestLoadBaseImageRegistry|TestBaseImageRegistry" -v ./internal/build/`

Expected: Compilation error, `LoadBaseImageRegistry` and `BaseImageRegistry` undefined.

- [ ] **Step 3: Implement the registry types and loader**

Create `cc-deck/internal/build/registry.go`:

```go
package build

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BaseImageEntry describes a single base image in the registry.
type BaseImageEntry struct {
	Name    string `yaml:"name"`
	Ref     string `yaml:"ref"`
	Default bool   `yaml:"default,omitempty"`
}

// BaseImageRegistry is the top-level structure of base-images.yaml.
type BaseImageRegistry struct {
	OpenShell []BaseImageEntry `yaml:"openshell,omitempty"`
	Container []BaseImageEntry `yaml:"container,omitempty"`
}

// LoadBaseImageRegistry reads and parses a base-images.yaml file.
func LoadBaseImageRegistry(path string) (*BaseImageRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading base image registry: %w", err)
	}
	var reg BaseImageRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing base image registry: %w", err)
	}
	return &reg, nil
}

// DefaultRef returns the ref of the default entry for the given target type.
// Returns an empty string if no default is set or the target is unknown.
func (r *BaseImageRegistry) DefaultRef(target string) string {
	for _, e := range r.EntriesForTarget(target) {
		if e.Default {
			return e.Ref
		}
	}
	return ""
}

// EntriesForTarget returns all entries for the given target type.
func (r *BaseImageRegistry) EntriesForTarget(target string) []BaseImageEntry {
	switch target {
	case "openshell":
		return r.OpenShell
	case "container":
		return r.Container
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd cc-deck && go test -run "TestLoadBaseImageRegistry|TestBaseImageRegistry" -v ./internal/build/`

Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cc-deck/internal/build/registry.go cc-deck/internal/build/registry_test.go
git commit -m "feat: add BaseImageRegistry types and loader"
```

---

### Task 3: Wire manifest defaults to registry lookup

**Files:**
- Modify: `cc-deck/internal/build/manifest.go:270-291`
- Modify: `cc-deck/cmd/cc-deck/main.go:128-132`
- Modify: `cc-deck/internal/build/manifest_test.go` (update tests that reference the constants)

- [ ] **Step 1: Write a test for `ResolveDefaultBaseImage`**

Add to `cc-deck/internal/build/registry_test.go`:

```go
func TestResolveDefaultBaseImage(t *testing.T) {
	content := `openshell:
  - name: custom
    ref: custom-openshell:v1
    default: true
container:
  - name: custom
    ref: custom-container:v1
    default: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	ref := ResolveDefaultBaseImage(path, "openshell")
	assert.Equal(t, "custom-openshell:v1", ref)

	ref = ResolveDefaultBaseImage(path, "container")
	assert.Equal(t, "custom-container:v1", ref)
}

func TestResolveDefaultBaseImage_FileNotFound(t *testing.T) {
	ref := ResolveDefaultBaseImage("/nonexistent/path", "openshell")
	assert.Equal(t, "", ref)
}

func TestResolveDefaultBaseImage_NoDefault(t *testing.T) {
	content := `openshell:
  - name: test
    ref: test:latest
`
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	ref := ResolveDefaultBaseImage(path, "openshell")
	assert.Equal(t, "", ref)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cc-deck && go test -run "TestResolveDefaultBaseImage" -v ./internal/build/`

Expected: Compilation error, `ResolveDefaultBaseImage` undefined.

- [ ] **Step 3: Implement `ResolveDefaultBaseImage`**

Add to `cc-deck/internal/build/registry.go`:

```go
// ResolveDefaultBaseImage loads the registry from the given path and returns
// the default ref for the target type. Returns an empty string on any error
// (file not found, parse error, no default set), allowing callers to fall
// back to hardcoded constants.
func ResolveDefaultBaseImage(registryPath string, target string) string {
	reg, err := LoadBaseImageRegistry(registryPath)
	if err != nil {
		return ""
	}
	return reg.DefaultRef(target)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd cc-deck && go test -run "TestResolveDefaultBaseImage" -v ./internal/build/`

Expected: All 3 tests PASS.

- [ ] **Step 5: Add a `RegistryPath` variable and update default resolution in `manifest.go`**

In `cc-deck/internal/build/manifest.go`, replace the hardcoded fallbacks in `OpenShellBaseImage()` and `BaseImage()` with registry-aware lookups. The `DefaultOpenShellBaseImage` and `DefaultBaseImage` constants remain as ultimate fallbacks:

```go
// RegistryPath is the path to base-images.yaml. Set by the CLI at startup
// if the file exists. When empty, hardcoded defaults are used.
var RegistryPath string

// OpenShellBaseImage returns the OpenShell base image reference, with default.
func (m *Manifest) OpenShellBaseImage() string {
	if m.Targets != nil && m.Targets.OpenShell != nil && m.Targets.OpenShell.Base != "" {
		return m.Targets.OpenShell.Base
	}
	if RegistryPath != "" {
		if ref := ResolveDefaultBaseImage(RegistryPath, "openshell"); ref != "" {
			return ref
		}
	}
	return DefaultOpenShellBaseImage
}

// BaseImage returns the base image reference, with default.
func (m *Manifest) BaseImage() string {
	if m.Targets != nil && m.Targets.Container != nil && m.Targets.Container.Base != "" {
		return m.Targets.Container.Base
	}
	if RegistryPath != "" {
		if ref := ResolveDefaultBaseImage(RegistryPath, "container"); ref != "" {
			return ref
		}
	}
	return DefaultBaseImage
}
```

- [ ] **Step 6: Update `main.go` to set `RegistryPath` if the file exists**

In `cc-deck/cmd/cc-deck/main.go`, in the `main()` function, after the existing `ImageRegistry` override, add:

```go
// Set registry path if base-images.yaml exists alongside the binary or in repo root
if registryPath, err := findBaseImagesYAML(); err == nil {
	build.RegistryPath = registryPath
}
```

And add the finder function:

```go
func findBaseImagesYAML() (string, error) {
	// Check current working directory first (repo root during development)
	cwd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(cwd, "base-images.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("base-images.yaml not found")
}
```

- [ ] **Step 7: Run all existing tests to make sure nothing breaks**

Run: `cd cc-deck && go test ./internal/build/ -v`

Expected: All existing tests PASS. The `RegistryPath` variable defaults to empty string, so all tests using `DefaultBaseImage` / `DefaultOpenShellBaseImage` still get the hardcoded constants.

- [ ] **Step 8: Commit**

```bash
git add cc-deck/internal/build/manifest.go cc-deck/internal/build/registry.go \
       cc-deck/internal/build/registry_test.go cc-deck/cmd/cc-deck/main.go
git commit -m "feat: wire manifest defaults to base-images.yaml registry"
```

---

### Task 4: Implement Tier 1 probe suite

**Files:**
- Create: `cc-deck/internal/e2e/image_probe_test.go`
- Create: `cc-deck/internal/e2e/probe.go`
- Modify: `Makefile` (add `test-images` targets)

- [ ] **Step 1: Create the probe definitions**

Create `cc-deck/internal/e2e/probe.go`:

```go
//go:build e2e

package e2e

// ProbeCheck defines a single validation to run inside a built container.
type ProbeCheck struct {
	Name    string
	Command []string
	Check   func(exitCode int, stdout string) error
}

// ContainerProbeChecks returns the standard probes to run against a cc-deck image.
// The expectedUser and expectedHome vary by target type.
func ContainerProbeChecks(expectedUser, expectedHome, expectedShell string) []ProbeCheck {
	return []ProbeCheck{
		{
			Name:    "claude-code-binary",
			Command: []string{"claude", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "zellij-binary",
			Command: []string{"zellij", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "cc-deck-binary",
			Command: []string{"cc-deck", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "user-identity",
			Command: []string{"whoami"},
			Check:   expectOutput(expectedUser),
		},
		{
			Name:    "home-directory",
			Command: []string{"sh", "-c", "echo $HOME"},
			Check:   expectOutput(expectedHome),
		},
		{
			Name:    "shell-config",
			Command: []string{"sh", "-c", "echo $SHELL"},
			Check:   expectContains(expectedShell),
		},
		{
			Name:    "cc-session-binary",
			Command: []string{"which", "cc-session"},
			Check:   expectExitZero,
		},
		{
			Name:    "cc-setup-binary",
			Command: []string{"which", "cc-setup"},
			Check:   expectExitZero,
		},
		{
			Name:    "write-permissions",
			Command: []string{"sh", "-c", "touch $HOME/test-write && rm $HOME/test-write"},
			Check:   expectExitZero,
		},
		{
			Name:    "plugin-installed",
			Command: []string{"sh", "-c", "ls " + expectedHome + "/.config/zellij/plugins/cc_deck.wasm"},
			Check:   expectExitZero,
		},
	}
}
```

- [ ] **Step 2: Create the check helper functions**

Append to `cc-deck/internal/e2e/probe.go`:

```go
import (
	"fmt"
	"strings"
)

func expectExitZero(exitCode int, stdout string) error {
	if exitCode != 0 {
		return fmt.Errorf("expected exit 0, got %d (output: %s)", exitCode, strings.TrimSpace(stdout))
	}
	return nil
}

func expectOutput(expected string) func(int, string) error {
	return func(exitCode int, stdout string) error {
		if exitCode != 0 {
			return fmt.Errorf("expected exit 0, got %d", exitCode)
		}
		got := strings.TrimSpace(stdout)
		if got != expected {
			return fmt.Errorf("expected %q, got %q", expected, got)
		}
		return nil
	}
}

func expectContains(substr string) func(int, string) error {
	return func(exitCode int, stdout string) error {
		if exitCode != 0 {
			return fmt.Errorf("expected exit 0, got %d", exitCode)
		}
		if !strings.Contains(stdout, substr) {
			return fmt.Errorf("expected output to contain %q, got %q", substr, strings.TrimSpace(stdout))
		}
		return nil
	}
}
```

Note: Merge the `import` block with the top of the file so there is a single import block.

- [ ] **Step 3: Create the image probe test**

Create `cc-deck/internal/e2e/image_probe_test.go`:

```go
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
	registryPath := filepath.Join(repoRoot, "base-images.yaml")

	reg, err := build.LoadBaseImageRegistry(registryPath)
	require.NoError(t, err, "base-images.yaml must exist at repo root")

	targets := []struct {
		target       string
		user         string
		home         string
		shell        string
	}{
		{"openshell", "sandbox", "/sandbox", "zsh"},
		{"container", "dev", "/home/dev", "zsh"},
	}

	for _, tgt := range targets {
		entries := reg.EntriesForTarget(tgt.target)
		if len(entries) == 0 {
			continue
		}

		// Filter by BASE env var if set
		baseFilter := os.Getenv("BASE")

		for _, entry := range entries {
			if baseFilter != "" && entry.Name != baseFilter {
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

	// Build the image using the manifest template system
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

	// Assemble the Containerfile from snippets
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

	// Write Containerfile to temp dir
	buildDir := t.TempDir()
	cfPath := filepath.Join(buildDir, "Containerfile")
	require.NoError(t, os.WriteFile(cfPath, []byte(containerfile.String()), 0o644))

	// Prepare context directory with cross-compiled binaries
	contextDir := filepath.Join(buildDir, target, "context")
	require.NoError(t, os.MkdirAll(contextDir, 0o755))

	// Copy cross-compiled binaries from the build output
	for _, arch := range []string{"amd64", "arm64"} {
		src := filepath.Join(repoRoot, "cc-deck", fmt.Sprintf("cc-deck-linux-%s", arch))
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(contextDir, fmt.Sprintf("cc-deck-linux-%s", arch))
			copyFile(t, src, dst)
		}
	}

	// Build the image
	buildCmd := exec.Command("podman", "build", "-f", cfPath, "-t", imageName, buildDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		if !isDefault {
			t.Skipf("WARNING: build failed for %s: %v", entry.Name, err)
		}
		t.Fatalf("build failed: %v", err)
	}

	// Clean up image at end
	t.Cleanup(func() {
		exec.Command("podman", "rmi", "-f", imageName).Run()
	})

	// Start container
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

	// Run probes
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
```

- [ ] **Step 4: Add Make targets**

Append to the `## -- Test --------------------------------------------------` section in `Makefile`, after the `smoke` target:

```makefile
test-images: cross-cli  ## Run image probe suite against all base images
	cd cc-deck && go test -tags e2e -run TestImageProbe -v -timeout 600s -count=1 ./internal/e2e/

test-images-quick: cross-cli  ## Run image probe suite against default base images only
	cd cc-deck && BASE= go test -tags e2e -run TestImageProbe -v -timeout 300s -count=1 ./internal/e2e/
```

Note: `test-images-quick` will need a way to filter to defaults only. Update the test to check for `DEFAULTS_ONLY` env var:

Add this check at the start of the inner loop in `TestImageProbe`:

```go
if os.Getenv("DEFAULTS_ONLY") == "1" && !entry.Default {
    continue
}
```

And update the Make target:

```makefile
test-images-quick: cross-cli  ## Run image probe suite against default base images only
	cd cc-deck && DEFAULTS_ONLY=1 go test -tags e2e -run TestImageProbe -v -timeout 300s -count=1 ./internal/e2e/
```

- [ ] **Step 5: Verify the test compiles**

Run: `cd cc-deck && go vet -tags e2e ./internal/e2e/`

Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add cc-deck/internal/e2e/probe.go cc-deck/internal/e2e/image_probe_test.go Makefile
git commit -m "feat: add Tier 1 image probe suite with Make targets"
```

---

### Task 5: Create the discovery skill

**Files:**
- Create: `.claude/skills/cc-deck-base-images/SKILL.md`

- [ ] **Step 1: Create the skill directory**

```bash
mkdir -p .claude/skills/cc-deck-base-images
```

- [ ] **Step 2: Write the skill definition**

Create `.claude/skills/cc-deck-base-images/SKILL.md`:

````markdown
---
name: cc-deck.base-images
description: Check known sources for base image updates and maintain base-images.yaml
---

# Base Image Discovery

Check upstream sources for new or updated base images and help maintain `base-images.yaml`.

## Invocation

- `/cc-deck.base-images` — check all entries for updates, report findings
- `/cc-deck.base-images update` — check and offer to apply updates to `base-images.yaml`

## Steps

### 1. Load current state

Read `base-images.yaml` from the repo root. If `.base-images-digests.json` exists, load the last-known digests.

### 2. Check registry digests

For each entry in `base-images.yaml`, run:

```bash
skopeo inspect --raw docker://<ref> 2>/dev/null | jq -r '.digest // .config.digest // "unknown"'
```

If `skopeo` is not available, fall back to:

```bash
podman inspect --format '{{.Digest}}' docker://<ref> 2>/dev/null
```

Compare against stored digests. Report changes:
- "nvidia-upstream: digest changed (sha256:old... → sha256:new...)"
- "nvidia-upstream: unchanged"
- "rh-ubi-openshell: image not found (may have been renamed or removed)"

### 3. Check upstream repos for new images

Check these GitHub sources for new base image references:

```bash
# OpenShell upstream releases
gh api repos/NVIDIA/OpenShell-Community/releases/latest --jq '.tag_name'

# Red Hat agentic starter kits — look for Containerfile FROM lines
gh api repos/red-hat-data-services/agentic-starter-kits/contents/ --jq '.[].name' 2>/dev/null
```

### 4. Scan known registries

Check for new tags in known registries:

```bash
# NVIDIA OpenShell sandbox images
skopeo list-tags docker://ghcr.io/nvidia/openshell-community/sandboxes/base 2>/dev/null | jq -r '.Tags[]'

# AIPCC images (if registry exists)
skopeo list-tags docker://quay.io/aipcc/openshell-base 2>/dev/null | jq -r '.Tags[]'
```

Report any tags not currently tracked in `base-images.yaml`.

### 5. Report findings

Present a summary:
- **Digest changes** for tracked entries
- **New images** found in upstream repos or registries
- **Stale entries** that could not be found

### 6. Apply updates (if `update` argument)

If the user invoked with `update`:
1. Show proposed changes to `base-images.yaml`
2. Ask for confirmation before writing
3. Update `.base-images-digests.json` with current digests
4. Suggest running `make test-images` to validate the changes
````

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/cc-deck-base-images/
git commit -m "feat: add base image discovery skill"
```

---

### Task 6: Add Red Hat image entries (placeholder for known refs)

**Files:**
- Modify: `base-images.yaml`

This task adds the Red Hat image entries once their references are known. For now, add commented-out placeholders based on the Slack thread discussion.

- [ ] **Step 1: Add commented entries to `base-images.yaml`**

Update `base-images.yaml`:

```yaml
# Base image registry for cc-deck build pipeline.
# Each entry lists a base image that cc-deck can build on top of.
# The `default: true` entry is used when no `base:` is specified in the manifest.
openshell:
  - name: nvidia-upstream
    ref: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    default: true
  # Uncomment when AIPCC publishes UBI-based OpenShell base images:
  # - name: rh-ubi-openshell
  #   ref: quay.io/aipcc/openshell-base:latest
container:
  - name: fedora-41
    ref: registry.fedoraproject.org/fedora:41
    default: true
  # Uncomment when UBI base images are validated:
  # - name: rh-ubi9
  #   ref: registry.access.redhat.com/ubi9/ubi:latest
```

- [ ] **Step 2: Commit**

```bash
git add base-images.yaml
git commit -m "docs: add commented Red Hat base image entries for future activation"
```

---

### Task 7: Implement Tier 2 session smoke test (follow-up)

**Files:**
- Create: `cc-deck/internal/e2e/image_session_test.go`
- Modify: `Makefile` (add `test-images-session` target)

- [ ] **Step 1: Create the session smoke test**

Create `cc-deck/internal/e2e/image_session_test.go`:

```go
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
	// Require API credentials — skip if not available
	hasAnthropicKey := os.Getenv("ANTHROPIC_API_KEY") != ""
	hasVertexCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != ""
	if !hasAnthropicKey && !hasVertexCreds {
		t.Skip("No API credentials set (ANTHROPIC_API_KEY or GOOGLE_APPLICATION_CREDENTIALS), skipping session tests")
	}

	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not found, skipping session tests")
	}

	repoRoot := mustFindProjectRoot()
	registryPath := filepath.Join(repoRoot, "base-images.yaml")

	reg, err := build.LoadBaseImageRegistry(registryPath)
	require.NoError(t, err)

	// Only test default images for session tests (they're slow)
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

	// Assume the probe test already built the image
	// If not, skip
	checkCmd := exec.Command("podman", "image", "exists", imageName)
	if err := checkCmd.Run(); err != nil {
		t.Skipf("Image %s not found; run test-images first", imageName)
	}

	// Build env var args for API credentials
	envArgs := []string{}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		envArgs = append(envArgs, "-e", "ANTHROPIC_API_KEY="+key)
	}
	if creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); creds != "" {
		envArgs = append(envArgs, "-e", "GOOGLE_APPLICATION_CREDENTIALS="+creds)
	}

	// Start container
	runArgs := append([]string{"run", "-d", "--name", containerName}, envArgs...)
	runArgs = append(runArgs, imageName, "sleep", "120")
	runCmd := exec.Command("podman", runArgs...)
	require.NoError(t, runCmd.Run(), "failed to start container")

	t.Cleanup(func() {
		exec.Command("podman", "rm", "-f", containerName).Run()
	})

	// Start cc-deck run inside the container (background)
	exec.Command("podman", "exec", "-d", containerName, "cc-deck", "run").Run()

	// Wait for Zellij to start (poll for up to 60 seconds)
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

	// Check for plugin process
	var pluginOut bytes.Buffer
	pluginCmd := exec.Command("podman", "exec", containerName,
		"sh", "-c", "ls /proc/*/cmdline 2>/dev/null | head -20")
	pluginCmd.Stdout = &pluginOut
	pluginCmd.Run()

	t.Logf("Container processes after Zellij start: %s", pluginOut.String())
}
```

- [ ] **Step 2: Add Make target**

Add to the test section in `Makefile`:

```makefile
test-images-session: cross-cli  ## Run session smoke tests (requires API credentials)
	cd cc-deck && go test -tags e2e -run TestImageSession -v -timeout 600s -count=1 ./internal/e2e/
```

- [ ] **Step 3: Verify the test compiles**

Run: `cd cc-deck && go vet -tags e2e ./internal/e2e/`

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add cc-deck/internal/e2e/image_session_test.go Makefile
git commit -m "feat: add Tier 2 session smoke test for image validation"
```

---

## Self-Review Checklist

**Spec coverage:**
- [x] Base image registry (`base-images.yaml`) — Task 1, Task 6
- [x] Go types + loader — Task 2
- [x] Default resolution wiring — Task 3
- [x] Probe suite (Tier 1) — Task 4
- [x] Session smoke test (Tier 2) — Task 7
- [x] Discovery skill — Task 5
- [x] Make targets — Tasks 4, 7
- [x] Failure semantics (default=hard, non-default=warning) — Task 4 (`isDefault` flag)
- [x] `.base-images-digests.json` gitignored — Task 1

**Placeholder scan:** No TBDs, TODOs, or "implement later" remaining. All code steps have actual code.

**Type consistency:** `BaseImageEntry`, `BaseImageRegistry`, `LoadBaseImageRegistry`, `ResolveDefaultBaseImage`, `ContainerProbeChecks` — consistent across all tasks. `ProbeCheck` type fields (`Name`, `Command`, `Check`) used consistently in Task 4.
