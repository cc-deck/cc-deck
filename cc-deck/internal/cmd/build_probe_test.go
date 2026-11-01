package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/build/imageprobe"
)

func TestValidateImageRef_Valid(t *testing.T) {
	for _, ref := range []string{
		"fedora:latest",
		"ghcr.io/cc-deck/base@sha256:abcd1234",
		"ubuntu",
	} {
		assert.NoError(t, validateImageRef(ref), "ref=%q", ref)
	}
}

func TestValidateImageRef_Empty(t *testing.T) {
	err := validateImageRef("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateImageRef_LeadingDash(t *testing.T) {
	err := validateImageRef("-malicious")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestValidateImageRef_ControlCharacters(t *testing.T) {
	err := validateImageRef("fedora\n:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control characters")
}

func TestResolveProbeTools_NoManifest(t *testing.T) {
	dir := t.TempDir()

	tools := resolveProbeTools(dir)

	// With no build.yaml present, the default tool set is used unchanged.
	assert.Equal(t, imageprobe.MergeToolSets(nil), tools)
	assert.NotEmpty(t, tools)
}

func TestResolveProbeTools_WithManifestOverride(t *testing.T) {
	dir := t.TempDir()
	manifest := `version: 1
probe_tools:
  - name: git
    version: ">=2.40"
  - name: mytool
    version: "1.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(manifest), 0o644))

	tools := resolveProbeTools(dir)

	byName := make(map[string]string, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool.Version
	}
	assert.Equal(t, ">=2.40", byName["git"])
	assert.Equal(t, "1.0", byName["mytool"])
}

func TestComputeManifestDiff_NoManifest(t *testing.T) {
	dir := t.TempDir()
	result := &imageprobe.ProbeResult{Tools: map[string]imageprobe.ToolInfo{}}

	diffs := computeManifestDiff(dir, result)

	assert.Nil(t, diffs)
}

func TestComputeManifestDiff_NoToolsRequired(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte("version: 1\n"), 0o644))
	result := &imageprobe.ProbeResult{Tools: map[string]imageprobe.ToolInfo{}}

	diffs := computeManifestDiff(dir, result)

	assert.Nil(t, diffs)
}

func TestComputeManifestDiff_MissingAndPresentTools(t *testing.T) {
	dir := t.TempDir()
	manifest := `version: 1
tools:
  - name: git
  - name: jq
probe_tools:
  - name: git
    version: "2.40.0"
  - name: jq
    version: "1.6"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(manifest), 0o644))

	result := &imageprobe.ProbeResult{
		PackageManager: "dnf",
		Tools: map[string]imageprobe.ToolInfo{
			"git": {Name: "git", Present: true, Version: "2.40.0"},
		},
	}

	diffs := computeManifestDiff(dir, result)

	require.Len(t, diffs, 2)
	byTool := make(map[string]imageprobe.ToolDiff, len(diffs))
	for _, d := range diffs {
		byTool[d.Tool] = d
	}
	assert.Equal(t, "present", byTool["git"].Status)
	assert.Equal(t, "missing", byTool["jq"].Status)
	assert.Equal(t, "package", byTool["jq"].InstallMethod)
}
