package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerDataForTarget_Container(t *testing.T) {
	m := &Manifest{}
	data := ContainerDataForTarget(m, "container")
	require.NotNil(t, data)
	assert.Equal(t, "container", data.Target)
	assert.Equal(t, "dev", data.User)
	assert.Equal(t, "/home/dev", data.HomeDir)
	assert.Equal(t, "container", data.ContextDir)
	assert.Equal(t, DefaultBaseImage, data.BaseImage)
	assert.Equal(t, "zsh", data.Shell)
}

func TestContainerDataForTarget_OpenShell(t *testing.T) {
	m := &Manifest{
		Settings: &SettingsConfig{Shell: "bash"},
	}
	data := ContainerDataForTarget(m, "openshell")
	require.NotNil(t, data)
	assert.Equal(t, "openshell", data.Target)
	assert.Equal(t, "sandbox", data.User)
	assert.Equal(t, "/sandbox", data.HomeDir)
	assert.Equal(t, "openshell", data.ContextDir)
	assert.Equal(t, DefaultOpenShellBaseImage, data.BaseImage)
	assert.Equal(t, "bash", data.Shell)
}

func TestContainerDataForTarget_Invalid(t *testing.T) {
	m := &Manifest{}
	assert.Nil(t, ContainerDataForTarget(m, "invalid"))
}

// --- ResolveToolPaths tests ---

func TestResolveToolPaths_GoTool(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "Go >= 1.25.0"}},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Equal(t, []string{"/usr/local/go/bin"}, paths)
}

func TestResolveToolPaths_RustCargo(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "Rust stable (edition 2021)"}},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Equal(t, []string{"/sandbox/.cargo/bin"}, paths)
}

func TestResolveToolPaths_CargoAlias(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "cargo"}},
	}
	paths := ResolveToolPaths(m, "/home/dev")
	assert.Equal(t, []string{"/home/dev/.cargo/bin"}, paths)
}

func TestResolveToolPaths_NoMatches(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "Node.js 20"}},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Empty(t, paths)
}

func TestResolveToolPaths_Deduplication(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{
			{Name: "cargo"},
			{Name: "Rust stable"},
		},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	// Both "cargo" and "rust" map to the same path; it should appear only once.
	assert.Equal(t, []string{"/sandbox/.cargo/bin"}, paths)
}

func TestResolveToolPaths_CaseInsensitive(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "GO >= 1.25"}},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Equal(t, []string{"/usr/local/go/bin"}, paths)
}

func TestResolveToolPaths_HomeDirSubstitution_Container(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "cargo"}},
	}
	paths := ResolveToolPaths(m, "/home/dev")
	assert.Equal(t, []string{"/home/dev/.cargo/bin"}, paths)
}

func TestResolveToolPaths_HomeDirSubstitution_OpenShell(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "cargo"}},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Equal(t, []string{"/sandbox/.cargo/bin"}, paths)
}

func TestResolveToolPaths_MultipleTools(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{
			{Name: "Go >= 1.25.0"},
			{Name: "Rust stable (edition 2021)"},
		},
	}
	paths := ResolveToolPaths(m, "/sandbox")
	// Sorted deterministically.
	assert.Equal(t, []string{"/sandbox/.cargo/bin", "/usr/local/go/bin"}, paths)
}

func TestResolveToolPaths_NilManifest(t *testing.T) {
	paths := ResolveToolPaths(nil, "/sandbox")
	assert.Nil(t, paths)
}

func TestResolveToolPaths_EmptyTools(t *testing.T) {
	m := &Manifest{}
	paths := ResolveToolPaths(m, "/sandbox")
	assert.Nil(t, paths)
}

func TestContainerDataForTarget_ToolPaths(t *testing.T) {
	m := &Manifest{
		Tools: []ToolEntry{{Name: "Go >= 1.25.0"}},
	}
	data := ContainerDataForTarget(m, "container")
	require.NotNil(t, data)
	assert.Equal(t, []string{"/usr/local/go/bin"}, data.ToolPaths)

	data = ContainerDataForTarget(m, "openshell")
	require.NotNil(t, data)
	assert.Equal(t, []string{"/usr/local/go/bin"}, data.ToolPaths)
}

func TestContainerDataForTarget_CustomBaseImage(t *testing.T) {
	m := &Manifest{
		Targets: &TargetsConfig{
			Container: &ContainerTarget{Name: "test", Base: "custom:latest"},
		},
	}
	data := ContainerDataForTarget(m, "container")
	assert.Equal(t, "custom:latest", data.BaseImage)
}

func TestRenderSnippets_Container(t *testing.T) {
	data := &ContainerfileData{
		Target:     "container",
		User:       "dev",
		HomeDir:    "/home/dev",
		ContextDir: "container",
		BaseImage:  "quay.io/cc-deck/cc-deck-base:latest",
		Shell:      "zsh",
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)
	assert.Len(t, snippets, len(snippetOrder))

	// Header has FROM and TARGETARCH.
	assert.Contains(t, snippets["01-header"], "FROM quay.io/cc-deck/cc-deck-base:latest")
	assert.Contains(t, snippets["01-header"], "ARG TARGETARCH")
	assert.Contains(t, snippets["01-header"], "--target container")

	// User setup present for container.
	assert.Contains(t, snippets["02-user-setup"], "useradd")
	assert.Contains(t, snippets["02-user-setup"], "/bin/zsh")

	// Mandatory stack uses container paths.
	assert.Contains(t, snippets["03-mandatory-stack"], "container/context/cc-deck-linux")
	assert.Contains(t, snippets["03-mandatory-stack"], "/home/dev/.claude")
	assert.Contains(t, snippets["03-mandatory-stack"], "HOME=/home/dev")
	assert.Contains(t, snippets["03-mandatory-stack"], "claude.ai/install.sh")

	// OpenShell-specific snippets are empty for container (no shim, no shell finalize).
	assert.Equal(t, "\n", snippets["04-openshell-extras"])
	assert.NotContains(t, snippets["04-openshell-extras"], "getifaddrs")
	assert.Equal(t, "\n", snippets["05-shell-finalize"])

	// Footer has container CMD.
	assert.Contains(t, snippets["06-footer"], `CMD ["sleep", "infinity"]`)
	assert.NotContains(t, snippets["06-footer"], "ENTRYPOINT")
	assert.NotContains(t, snippets["06-footer"], "chown -R")
}

func TestRenderSnippets_OpenShell(t *testing.T) {
	data := &ContainerfileData{
		Target:     "openshell",
		User:       "sandbox",
		HomeDir:    "/sandbox",
		ContextDir: "openshell",
		BaseImage:  "ghcr.io/nvidia/openshell-community/sandboxes/base:latest",
		Shell:      "zsh",
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)

	// Header references openshell.
	assert.Contains(t, snippets["01-header"], "--target openshell")

	// No user setup for openshell (base image has sandbox user).
	assert.Equal(t, "\n", snippets["02-user-setup"])

	// Mandatory stack uses sandbox paths.
	assert.Contains(t, snippets["03-mandatory-stack"], "openshell/context/cc-deck-linux")
	assert.Contains(t, snippets["03-mandatory-stack"], "/sandbox/.claude")
	assert.Contains(t, snippets["03-mandatory-stack"], "HOME=/sandbox")

	// OpenShell extras present (policy COPY moved to 055-openshell-policy).
	assert.Contains(t, snippets["04-openshell-extras"], "skills")
	assert.Contains(t, snippets["04-openshell-extras"], `ENV SHELL="/bin/zsh"`)
	assert.Contains(t, snippets["04-openshell-extras"], "getifaddrs_shim")
	assert.Contains(t, snippets["04-openshell-extras"], "/etc/ld.so.preload")
	assert.NotContains(t, snippets["04-openshell-extras"], "policy.yaml")

	// Shell finalize has starship and Zellij auto-start.
	assert.Contains(t, snippets["05-shell-finalize"], "starship init")
	assert.Contains(t, snippets["05-shell-finalize"], "exec zellij --layout cc-deck")
	assert.Contains(t, snippets["05-shell-finalize"], "FINAL SHELL SETUP")

	// Policy COPY is in its own late snippet for two-pass cache efficiency.
	assert.Contains(t, snippets["055-openshell-policy"], "policy.yaml")
	assert.Contains(t, snippets["055-openshell-policy"], "/etc/openshell")

	// Footer has chown and ENTRYPOINT.
	assert.Contains(t, snippets["06-footer"], "chown -R sandbox:sandbox /sandbox")
	assert.Contains(t, snippets["06-footer"], `ENTRYPOINT ["/bin/bash"]`)
	assert.NotContains(t, snippets["06-footer"], "sleep")
}

func TestRenderSnippets_WithToolPaths(t *testing.T) {
	data := &ContainerfileData{
		Target:     "openshell",
		User:       "sandbox",
		HomeDir:    "/sandbox",
		ContextDir: "openshell",
		BaseImage:  "ghcr.io/nvidia/openshell-community/sandboxes/base:latest",
		Shell:      "zsh",
		ToolPaths:  []string{"/sandbox/.cargo/bin", "/usr/local/go/bin"},
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)

	shellFinalize := snippets["05-shell-finalize"]
	// The PATH prepend block should be present.
	assert.Contains(t, shellFinalize, "Tool PATH restoration")
	assert.Contains(t, shellFinalize, `sed -i '1i export PATH="/sandbox/.cargo/bin:/usr/local/go/bin:$PATH"'`)
	assert.Contains(t, shellFinalize, "/sandbox/.bashrc")
	assert.Contains(t, shellFinalize, "/sandbox/.zshrc")
	// Starship and Zellij should still be present (openshell target).
	assert.Contains(t, shellFinalize, "starship init")
}

func TestRenderSnippets_WithToolPaths_Container(t *testing.T) {
	data := &ContainerfileData{
		Target:     "container",
		User:       "dev",
		HomeDir:    "/home/dev",
		ContextDir: "container",
		BaseImage:  "quay.io/cc-deck/cc-deck-base:latest",
		Shell:      "zsh",
		ToolPaths:  []string{"/home/dev/.cargo/bin", "/usr/local/go/bin"},
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)

	shellFinalize := snippets["05-shell-finalize"]
	// Container target should also get PATH restoration.
	assert.Contains(t, shellFinalize, "Tool PATH restoration")
	assert.Contains(t, shellFinalize, `/home/dev/.cargo/bin:/usr/local/go/bin`)
	// But not starship/Zellij (openshell only).
	assert.NotContains(t, shellFinalize, "starship init")
}

func TestRenderSnippets_WithoutToolPaths(t *testing.T) {
	data := &ContainerfileData{
		Target:     "openshell",
		User:       "sandbox",
		HomeDir:    "/sandbox",
		ContextDir: "openshell",
		BaseImage:  "ghcr.io/nvidia/openshell-community/sandboxes/base:latest",
		Shell:      "zsh",
		ToolPaths:  nil,
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)

	shellFinalize := snippets["05-shell-finalize"]
	// No PATH prepend block should be generated.
	assert.NotContains(t, shellFinalize, "Tool PATH restoration")
	assert.NotContains(t, shellFinalize, "export PATH=")
	// Starship and Zellij should still be present.
	assert.Contains(t, shellFinalize, "starship init")
}

func TestRenderSnippets_EmptyToolPaths(t *testing.T) {
	data := &ContainerfileData{
		Target:     "container",
		User:       "dev",
		HomeDir:    "/home/dev",
		ContextDir: "container",
		BaseImage:  "quay.io/cc-deck/cc-deck-base:latest",
		Shell:      "zsh",
		ToolPaths:  []string{},
	}

	snippets, err := RenderContainerfileSnippets(data)
	require.NoError(t, err)

	shellFinalize := snippets["05-shell-finalize"]
	// Empty slice should not generate a PATH prepend block.
	assert.NotContains(t, shellFinalize, "Tool PATH restoration")
}

func TestSnippets_NoTemplateSyntaxLeaks(t *testing.T) {
	for _, target := range []string{"container", "openshell"} {
		t.Run(target, func(t *testing.T) {
			data := &ContainerfileData{
				Target:     target,
				User:       "testuser",
				HomeDir:    "/test",
				ContextDir: target,
				BaseImage:  "test:latest",
				Shell:      "zsh",
			}

			snippets, err := RenderContainerfileSnippets(data)
			require.NoError(t, err)

			for name, content := range snippets {
				assert.NotContains(t, content, "{{", "template syntax leaked in %s for target %s", name, target)
				assert.NotContains(t, content, "}}", "template syntax leaked in %s for target %s", name, target)
			}
		})
	}
}

func TestExtractContainerfileSnippets(t *testing.T) {
	dir := t.TempDir()
	data := &ContainerfileData{
		Target:     "openshell",
		User:       "sandbox",
		HomeDir:    "/sandbox",
		ContextDir: "openshell",
		BaseImage:  "test:latest",
		Shell:      "zsh",
	}

	err := ExtractContainerfileSnippets(dir, data)
	require.NoError(t, err)

	for _, name := range snippetOrder {
		path := filepath.Join(dir, name+".txt")
		_, statErr := os.Stat(path)
		assert.NoError(t, statErr, "expected snippet file %s", name+".txt")
	}

	// Verify a non-conditional snippet has content.
	header, err := os.ReadFile(filepath.Join(dir, "01-header.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(header), "FROM test:latest")
}

func TestMandatoryStack_BothTargets(t *testing.T) {
	containerData := &ContainerfileData{
		Target: "container", User: "dev", HomeDir: "/home/dev",
		ContextDir: "container", BaseImage: "base:latest", Shell: "zsh",
	}
	openshellData := &ContainerfileData{
		Target: "openshell", User: "sandbox", HomeDir: "/sandbox",
		ContextDir: "openshell", BaseImage: "base:latest", Shell: "zsh",
	}

	containerSnippets, err := RenderContainerfileSnippets(containerData)
	require.NoError(t, err)
	openshellSnippets, err := RenderContainerfileSnippets(openshellData)
	require.NoError(t, err)

	containerStack := containerSnippets["03-mandatory-stack"]
	openshellStack := openshellSnippets["03-mandatory-stack"]

	// Both have the same structural commands.
	assert.Contains(t, containerStack, "cc-session cc-setup")
	assert.Contains(t, openshellStack, "cc-session cc-setup")
	assert.Contains(t, containerStack, "claude.ai/install.sh")
	assert.Contains(t, openshellStack, "claude.ai/install.sh")

	// Paths differ correctly.
	assert.Contains(t, containerStack, "container/context/cc-deck-linux")
	assert.Contains(t, openshellStack, "openshell/context/cc-deck-linux")
	assert.Contains(t, containerStack, "HOME=/home/dev")
	assert.Contains(t, openshellStack, "HOME=/sandbox")
}
