package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/build/imageprobe"
	"github.com/cc-deck/cc-deck/internal/ws"
)

// --- extractTimestamp / extractQuoted (internal/cmd/domains_runtime.go) ---

func TestExtractTimestamp(t *testing.T) {
	line := `NOTICE    Mar 19 18:31:08.838 [1]: Connect`
	assert.Equal(t, "Mar 19 18:31:08.838", extractTimestamp(line))
}

func TestExtractTimestamp_NoBracket(t *testing.T) {
	assert.Equal(t, "", extractTimestamp("no bracket here"))
}

func TestExtractTimestamp_NoSpace(t *testing.T) {
	assert.Equal(t, "ABC", extractTimestamp("ABC[1]: foo"))
}

func TestExtractQuoted(t *testing.T) {
	line := `NOTICE blocked "example.com" from 10.0.0.1`
	assert.Equal(t, "example.com", extractQuoted(line))
}

func TestExtractQuoted_NoQuotes(t *testing.T) {
	assert.Equal(t, "", extractQuoted("no quotes here"))
}

func TestExtractQuoted_UnclosedQuote(t *testing.T) {
	assert.Equal(t, "", extractQuoted(`starts "but never closes`))
}

// --- validateImageRef (internal/cmd/build_probe.go) ---

func TestValidateImageRef_Valid(t *testing.T) {
	require.NoError(t, validateImageRef("docker.io/library/fedora:latest"))
}

func TestValidateImageRef_Empty(t *testing.T) {
	err := validateImageRef("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateImageRef_LeadingDash(t *testing.T) {
	err := validateImageRef("-rm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with")
}

func TestValidateImageRef_ControlChars(t *testing.T) {
	err := validateImageRef("fedora\x00latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control characters")
}

// --- resolveProbeTools / computeManifestDiff (internal/cmd/build_probe.go) ---

func TestResolveProbeTools_NoManifest(t *testing.T) {
	dir := t.TempDir()
	tools := resolveProbeTools(dir)
	// Falls back to the default merged tool set even with no manifest present.
	assert.Equal(t, imageprobe.MergeToolSets(nil), tools)
}

func TestResolveProbeTools_WithManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := "version: 1\nprobe_tools:\n  - name: git\n    version: \">=2.0\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(manifest), 0o644))

	tools := resolveProbeTools(dir)
	found := false
	for _, pt := range tools {
		if pt.Name == "git" && pt.Version == ">=2.0" {
			found = true
		}
	}
	assert.True(t, found, "expected merged tool set to include manifest-declared git tool")
}

func TestComputeManifestDiff_NoManifest(t *testing.T) {
	dir := t.TempDir()
	diffs := computeManifestDiff(dir, &imageprobe.ProbeResult{})
	assert.Nil(t, diffs)
}

func TestComputeManifestDiff_NoRequiredTools(t *testing.T) {
	dir := t.TempDir()
	manifest := "version: 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(manifest), 0o644))

	diffs := computeManifestDiff(dir, &imageprobe.ProbeResult{})
	assert.Nil(t, diffs)
}

func TestComputeManifestDiff_MissingTool(t *testing.T) {
	dir := t.TempDir()
	manifest := "version: 1\ntools:\n  - name: jq\n    install: package\nprobe_tools:\n  - name: jq\n    version: \"1.6\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(manifest), 0o644))

	result := &imageprobe.ProbeResult{
		Tools:          map[string]imageprobe.ToolInfo{},
		PackageManager: "dnf",
	}
	diffs := computeManifestDiff(dir, result)
	require.Len(t, diffs, 1)
	assert.Equal(t, "jq", diffs[0].Tool)
	assert.Equal(t, "missing", diffs[0].Status)
}

// --- optionalDirFS / exitError / resolveBuildDir (internal/cmd/build.go) ---

func TestOptionalDirFS_Empty(t *testing.T) {
	f, root := optionalDirFS("")
	assert.Nil(t, f)
	assert.Equal(t, "", root)
}

func TestOptionalDirFS_MissingDir(t *testing.T) {
	f, root := optionalDirFS(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Nil(t, f)
	assert.Equal(t, "", root)
}

func TestOptionalDirFS_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	f, root := optionalDirFS(dir)
	require.NotNil(t, f)
	assert.Equal(t, ".", root)
}

func TestExitError_PlainError(t *testing.T) {
	err := errors.New("boom")
	assert.Equal(t, err, exitError(err))
}

func TestExitError_ExitErrorPassthrough(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 3")
	runErr := cmd.Run()
	require.Error(t, runErr)

	var expected *exec.ExitError
	require.True(t, errors.As(runErr, &expected))

	got := exitError(runErr)
	assert.Equal(t, expected, got)
}

func TestResolveBuildDir_ExplicitArg(t *testing.T) {
	dir := resolveBuildDir([]string{"/some/project/.cc-deck/setup"})
	assert.Equal(t, "/some/project/.cc-deck/setup", dir)
}

func TestResolveBuildDirAndRoot_ConventionalPath(t *testing.T) {
	setupDir, root := resolveBuildDirAndRoot([]string{"/some/project/.cc-deck/setup"})
	assert.Equal(t, "/some/project/.cc-deck/setup", setupDir)
	assert.Equal(t, "/some/project", root)
}

func TestResolveBuildDirAndRoot_NonConventionalPath(t *testing.T) {
	setupDir, root := resolveBuildDirAndRoot([]string{"/tmp/custom-setup-dir"})
	assert.Equal(t, "/tmp/custom-setup-dir", setupDir)
	assert.Equal(t, "/tmp", root)
}

// --- splitCredential / buildProjectPathMap (internal/cmd/ws.go) ---

func TestSplitCredential_Valid(t *testing.T) {
	assert.Equal(t, []string{"TOKEN", "abc123"}, splitCredential("TOKEN=abc123"))
}

func TestSplitCredential_ValueContainsEquals(t *testing.T) {
	assert.Equal(t, []string{"KEY", "a=b=c"}, splitCredential("KEY=a=b=c"))
}

func TestSplitCredential_NoEquals(t *testing.T) {
	assert.Nil(t, splitCredential("NOEQUALSIGN"))
}

func TestBuildProjectPathMap(t *testing.T) {
	defs := []*ws.WorkspaceDefinition{
		{Name: "a", WorkspaceSpec: ws.WorkspaceSpec{ProjectDir: "/home/user/proj-a"}},
		{Name: "b", WorkspaceSpec: ws.WorkspaceSpec{ProjectDir: ""}},
	}
	got := buildProjectPathMap(defs)
	assert.Equal(t, map[string]string{"a": "/home/user/proj-a"}, got)
}

// --- printFlag / groupedHelp (internal/cmd/ws.go) ---

func TestPrintFlag_StringType(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("name", "", "the workspace name")
	f := fs.Lookup("name")

	var buf bytes.Buffer
	printFlag(&buf, f)
	assert.Contains(t, buf.String(), "--name string")
	assert.Contains(t, buf.String(), "the workspace name")
}

func TestPrintFlag_WithShorthand(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.IntP("count", "c", 0, "how many")
	f := fs.Lookup("count")

	var buf bytes.Buffer
	printFlag(&buf, f)
	assert.Contains(t, buf.String(), "-c, --count int")
}
