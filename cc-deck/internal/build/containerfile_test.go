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

	// OpenShell-specific snippets are empty for container.
	assert.Equal(t, "\n", snippets["04-openshell-extras"])
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

	// OpenShell extras present.
	assert.Contains(t, snippets["04-openshell-extras"], "skills")
	assert.Contains(t, snippets["04-openshell-extras"], "policy.yaml")
	assert.Contains(t, snippets["04-openshell-extras"], `ENV SHELL="/bin/zsh"`)

	// Shell finalize has starship and Zellij auto-start.
	assert.Contains(t, snippets["05-shell-finalize"], "starship init")
	assert.Contains(t, snippets["05-shell-finalize"], "exec zellij --layout cc-deck")
	assert.Contains(t, snippets["05-shell-finalize"], "FINAL SHELL SETUP")

	// Footer has chown and ENTRYPOINT.
	assert.Contains(t, snippets["06-footer"], "chown -R sandbox:sandbox /sandbox")
	assert.Contains(t, snippets["06-footer"], `ENTRYPOINT ["/bin/bash"]`)
	assert.NotContains(t, snippets["06-footer"], "sleep")
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
