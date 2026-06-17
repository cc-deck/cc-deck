package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUncommentTargets_ContainerOnly(t *testing.T) {
	input := `version: 2

# targets:
#   container:
#     name: test
#     base: fedora:41
#
#   ssh:
#     host: dev@server
`
	result := uncommentTargets(input, true, false, false)

	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  container:")
	assert.Contains(t, result, "    name: test")
	// SSH should remain commented
	assert.Contains(t, result, "#   ssh:")
}

func TestUncommentTargets_SSHOnly(t *testing.T) {
	input := `version: 2

# targets:
#   container:
#     name: test
#
#   ssh:
#     host: dev@server
#     # port: 22
`
	result := uncommentTargets(input, false, true, false)

	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  ssh:")
	assert.Contains(t, result, "    host: dev@server")
	// Container should remain commented
	assert.Contains(t, result, "#   container:")
}

func TestUncommentTargets_Both(t *testing.T) {
	input := `version: 2

# targets:
#   container:
#     name: test
#
#   ssh:
#     host: dev@server
`
	result := uncommentTargets(input, true, true, false)

	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  container:")
	assert.Contains(t, result, "    name: test")
	assert.Contains(t, result, "  ssh:")
	assert.Contains(t, result, "    host: dev@server")
	// No commented target headers should remain
	assert.NotContains(t, result, "# targets:")
	assert.NotContains(t, result, "#   container:")
	assert.NotContains(t, result, "#   ssh:")
}

func TestUncommentTargets_Neither(t *testing.T) {
	input := `version: 2

# targets:
#   container:
#     name: test
#
#   ssh:
#     host: dev@server
`
	result := uncommentTargets(input, false, false, false)

	// Nothing should change
	assert.Equal(t, input, result)
}

func TestUncommentTargets_PreservesNonTargetContent(t *testing.T) {
	input := `version: 2

# tools:
#   - go

# targets:
#   container:
#     name: test
`
	result := uncommentTargets(input, true, false, false)

	// Tools section should remain commented (not part of targets block)
	assert.Contains(t, result, "# tools:")
	// Targets should be uncommented
	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  container:")
}

func TestUncommentLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#     name: value", "    name: value"},
		{"#no-space", "no-space"},
		{"no-hash", "no-hash"},
		{"#", ""},
		{"# single", "single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, uncommentLine(tt.input))
		})
	}
}

func TestScaffoldSSHRoles(t *testing.T) {
	dir := t.TempDir()

	err := scaffoldSSHRoles(dir)
	require.NoError(t, err)

	expectedRoles := []string{"base", "tools", "zellij", "claude", "cc_deck", "plugins", "shell_config", "mcp"}
	for _, role := range expectedRoles {
		tasksMain := filepath.Join(dir, "roles", role, "tasks", "main.yml")
		assert.FileExists(t, tasksMain, "missing tasks/main.yml for role %s", role)

		defaultsMain := filepath.Join(dir, "roles", role, "defaults", "main.yml")
		assert.FileExists(t, defaultsMain, "missing defaults/main.yml for role %s", role)
	}

	// Verify group_vars directory
	assert.DirExists(t, filepath.Join(dir, "group_vars"))

	// Verify site.yml contains all role names
	siteContent, err := os.ReadFile(filepath.Join(dir, "site.yml"))
	require.NoError(t, err)
	for _, role := range expectedRoles {
		assert.Contains(t, string(siteContent), "- "+role)
	}

	// Verify inventory.ini has the expected group header
	invContent, err := os.ReadFile(filepath.Join(dir, "inventory.ini"))
	require.NoError(t, err)
	assert.Contains(t, string(invContent), "[setup_targets]")
}

func TestContainsTarget(t *testing.T) {
	assert.True(t, containsTarget([]string{"container", "ssh"}, "container"))
	assert.True(t, containsTarget([]string{"container", "ssh"}, "ssh"))
	assert.False(t, containsTarget([]string{"container"}, "ssh"))
	assert.False(t, containsTarget([]string{}, "container"))
	assert.False(t, containsTarget(nil, "ssh"))
}

func TestInitSetupDir_CreatesManifest(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")
	projectRoot := dir

	err := InitSetupDir(setupDir, projectRoot, false, nil)
	require.NoError(t, err)

	// Manifest should exist
	manifestPath := filepath.Join(setupDir, "build.yaml")
	assert.FileExists(t, manifestPath)

	// Should be valid YAML with version 2
	content, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "version: 3")

	// .gitignore should exist
	assert.FileExists(t, filepath.Join(setupDir, ".gitignore"))

	// Commands should be installed
	commandsDir := filepath.Join(projectRoot, ".claude", "commands")
	matches, _ := filepath.Glob(filepath.Join(commandsDir, "cc-deck.*.md"))
	assert.NotEmpty(t, matches, "no Claude commands installed")
}

func TestInitSetupDir_ContainerTarget(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")

	err := InitSetupDir(setupDir, dir, false, []string{"container"})
	require.NoError(t, err)

	// container/context directory should exist
	assert.DirExists(t, filepath.Join(setupDir, "container", "context"))

	// Manifest should have uncommented container section
	content, err := os.ReadFile(filepath.Join(setupDir, "build.yaml"))
	require.NoError(t, err)
	lines := string(content)
	assert.Contains(t, lines, "targets:")
	assert.Contains(t, lines, "  container:")

	// SSH roles should NOT exist
	assert.NoDirExists(t, filepath.Join(setupDir, "ssh", "roles"))
}

func TestInitSetupDir_SSHTarget(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")

	err := InitSetupDir(setupDir, dir, false, []string{"ssh"})
	require.NoError(t, err)

	// Ansible roles should exist under ssh/
	assert.DirExists(t, filepath.Join(setupDir, "ssh", "roles", "base", "tasks"))
	assert.FileExists(t, filepath.Join(setupDir, "ssh", "site.yml"))
	assert.FileExists(t, filepath.Join(setupDir, "ssh", "inventory.ini"))

	// container/context should NOT exist
	assert.NoDirExists(t, filepath.Join(setupDir, "container", "context"))
}

func TestInitSetupDir_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")

	// First init
	err := InitSetupDir(setupDir, dir, false, nil)
	require.NoError(t, err)

	// Second init without force should fail
	err = InitSetupDir(setupDir, dir, false, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")

	// With force should succeed
	err = InitSetupDir(setupDir, dir, true, nil)
	require.NoError(t, err)
}

func TestInitSetupDir_OpenShellTarget(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")

	err := InitSetupDir(setupDir, dir, false, []string{"openshell"})
	require.NoError(t, err)

	// openshell directory should exist
	assert.DirExists(t, filepath.Join(setupDir, "openshell"))

	// Manifest should have uncommented openshell section
	content, err := os.ReadFile(filepath.Join(setupDir, "build.yaml"))
	require.NoError(t, err)
	lines := string(content)
	assert.Contains(t, lines, "targets:")
	assert.Contains(t, lines, "  openshell:")

	// Container and SSH should NOT exist
	assert.NoDirExists(t, filepath.Join(setupDir, "container", "context"))
	assert.NoDirExists(t, filepath.Join(setupDir, "ssh", "roles"))
}

func TestUncommentTargets_OpenShellOnly(t *testing.T) {
	input := `version: 3

# targets:
#   container:
#     name: test
#
#   ssh:
#     host: dev@server
#
#   openshell:
#     name: my-sandbox
#     # base: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
`
	result := uncommentTargets(input, false, false, true)

	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  openshell:")
	assert.Contains(t, result, "    name: my-sandbox")
	// Container and SSH should remain commented
	assert.Contains(t, result, "#   container:")
	assert.Contains(t, result, "#   ssh:")
}

func TestUncommentTargets_AllThree(t *testing.T) {
	input := `version: 3

# targets:
#   container:
#     name: test
#
#   ssh:
#     host: dev@server
#
#   openshell:
#     name: my-sandbox
`
	result := uncommentTargets(input, true, true, true)

	assert.Contains(t, result, "targets:")
	assert.Contains(t, result, "  container:")
	assert.Contains(t, result, "  ssh:")
	assert.Contains(t, result, "  openshell:")
	assert.NotContains(t, result, "# targets:")
}

func TestContainsTarget_OpenShell(t *testing.T) {
	assert.True(t, containsTarget([]string{"openshell"}, "openshell"))
	assert.True(t, containsTarget([]string{"container", "openshell"}, "openshell"))
	assert.False(t, containsTarget([]string{"container", "ssh"}, "openshell"))
}

func TestInitSetupDir_ExtractsBaseImagesAndSkill(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")
	projectRoot := dir

	err := InitSetupDir(setupDir, projectRoot, false, nil)
	require.NoError(t, err)

	// base-images.yaml should be extracted to setup dir
	baseImagesPath := filepath.Join(setupDir, "base-images.yaml")
	assert.FileExists(t, baseImagesPath)

	// Should be valid and loadable
	reg, err := LoadBaseImageRegistry(baseImagesPath)
	require.NoError(t, err)
	assert.NotEmpty(t, reg.OpenShell, "extracted registry should have openshell entries")
	assert.NotEmpty(t, reg.Container, "extracted registry should have container entries")

	// Discovery skill should be extracted to project root's .claude/skills/
	skillPath := filepath.Join(projectRoot, ".claude", "skills", "cc-deck-base-images", "SKILL.md")
	assert.FileExists(t, skillPath)

	// Skill should contain the expected content
	skillContent, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(skillContent), "Base Image Discovery")
	assert.Contains(t, string(skillContent), "base-images.yaml")
}

func TestInitSetupDir_ManifestTargetsSectionCommented(t *testing.T) {
	dir := t.TempDir()
	setupDir := filepath.Join(dir, "setup")

	err := InitSetupDir(setupDir, dir, false, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(setupDir, "build.yaml"))
	require.NoError(t, err)

	// Without targets flag, targets section should remain commented
	lines := string(content)
	assert.Contains(t, lines, "# targets:")

	// Should NOT have an uncommented targets: line
	for _, line := range strings.Split(lines, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "targets:" {
			t.Error("targets: should be commented out when no --target flag")
		}
	}
}
