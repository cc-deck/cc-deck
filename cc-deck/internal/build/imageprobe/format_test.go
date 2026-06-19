package imageprobe

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleResult() *ProbeResult {
	return &ProbeResult{
		ImageRef:       "registry.fedoraproject.org/fedora:41",
		ImageDigest:    "sha256:abc123",
		OS:             OSInfo{ID: "fedora", Name: "Fedora Linux 41", Version: "41"},
		PackageManager: "dnf",
		Tools: map[string]ToolInfo{
			"git":     {Name: "git", Path: "/usr/bin/git", Version: "2.43.0", Present: true},
			"python3": {Name: "python3", Path: "/usr/bin/python3", Version: "3.12.4", Present: true},
			"go":      {Name: "go", Path: "", Version: "", Present: false},
		},
		User:       UserInfo{Name: "root", UID: 0, Home: "/root", Shell: "/bin/bash"},
		Shells:     []string{"bash", "sh"},
		DurationMS: 4200,
	}
}

func TestFormatTable_ContainsOSInfo(t *testing.T) {
	output := FormatTable(sampleResult(), false)
	assert.Contains(t, output, "Fedora Linux 41")
	assert.Contains(t, output, "dnf")
	assert.Contains(t, output, "root")
}

func TestFormatTable_ContainsToolCheckmarks(t *testing.T) {
	output := FormatTable(sampleResult(), false)
	assert.Contains(t, output, "✓ git")
	assert.Contains(t, output, "✓ python3")
	assert.Contains(t, output, "✗ go")
}

func TestFormatTable_ShowsCachedStatus(t *testing.T) {
	fresh := FormatTable(sampleResult(), false)
	assert.Contains(t, fresh, "(fresh)")

	cached := FormatTable(sampleResult(), true)
	assert.Contains(t, cached, "(cached)")
}

func TestFormatTable_ShowsDuration(t *testing.T) {
	output := FormatTable(sampleResult(), false)
	assert.Contains(t, output, "4.2s")
}

func TestFormatDiff_Present(t *testing.T) {
	diffs := []ToolDiff{
		{Tool: "git", Required: "2.40", Installed: "2.43.0", Status: "present"},
	}
	output := FormatDiff(diffs)
	assert.Contains(t, output, "✓ git")
	assert.Contains(t, output, "skip")
}

func TestFormatDiff_Missing(t *testing.T) {
	diffs := []ToolDiff{
		{Tool: "go", Required: "1.25", Status: "missing"},
	}
	output := FormatDiff(diffs)
	assert.Contains(t, output, "✗ go")
	assert.Contains(t, output, "install")
}

func TestFormatDiff_Incompatible(t *testing.T) {
	diffs := []ToolDiff{
		{Tool: "node", Required: "22.0", Installed: "18.0.0", Status: "incompatible"},
	}
	output := FormatDiff(diffs)
	assert.Contains(t, output, "~ node")
	assert.Contains(t, output, "shadow")
}

func TestFormatDiff_Empty(t *testing.T) {
	output := FormatDiff(nil)
	assert.Equal(t, "", output)
}

func TestFormatJSON_Schema(t *testing.T) {
	result := sampleResult()
	output, err := FormatJSON(result, false)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &parsed))

	assert.Equal(t, "registry.fedoraproject.org/fedora:41", parsed["image_ref"])
	assert.Equal(t, false, parsed["cached"])
	assert.Contains(t, parsed, "os")
	assert.Contains(t, parsed, "package_manager")
	assert.Contains(t, parsed, "tools")
	assert.Contains(t, parsed, "user")
	assert.Contains(t, parsed, "shells")
	assert.Contains(t, parsed, "duration_ms")
}

func TestFormatJSON_CachedTrue(t *testing.T) {
	result := sampleResult()
	output, err := FormatJSON(result, true)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, true, parsed["cached"])
}
