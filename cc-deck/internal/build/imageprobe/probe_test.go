package imageprobe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateProbeScript_ContainsOSDetection(t *testing.T) {
	script := GenerateProbeScript(nil)
	assert.Contains(t, script, "/etc/os-release")
	assert.Contains(t, script, `"type":"os"`)
}

func TestGenerateProbeScript_ContainsPackageManagerDetection(t *testing.T) {
	script := GenerateProbeScript(nil)
	assert.Contains(t, script, `"type":"pkgmgr"`)
	assert.Contains(t, script, "dnf")
	assert.Contains(t, script, "apt-get")
	assert.Contains(t, script, "apk")
	assert.Contains(t, script, "yum")
}

func TestGenerateProbeScript_ContainsToolChecks(t *testing.T) {
	tools := []ProbeToolEntry{
		{Name: "git"},
		{Name: "python3", Version: "3.12"},
	}
	script := GenerateProbeScript(tools)
	assert.Contains(t, script, `command -v git`)
	assert.Contains(t, script, `command -v python3`)
	assert.Contains(t, script, `"type":"tool"`)
}

func TestGenerateProbeScript_ContainsUserInfo(t *testing.T) {
	script := GenerateProbeScript(nil)
	assert.Contains(t, script, `"type":"user"`)
	assert.Contains(t, script, "id -un")
}

func TestGenerateProbeScript_ContainsShellAvailability(t *testing.T) {
	script := GenerateProbeScript(nil)
	assert.Contains(t, script, `"type":"shells"`)
	assert.Contains(t, script, "bash")
	assert.Contains(t, script, "zsh")
}

func TestGenerateProbeScript_DefaultTools(t *testing.T) {
	tools := MergeToolSets(nil)
	script := GenerateProbeScript(tools)
	for _, name := range DefaultTools {
		assert.Contains(t, script, name, "script should check for default tool %s", name)
	}
}

func TestGenerateProbeScript_ManifestOverrideTools(t *testing.T) {
	tools := MergeToolSets([]ProbeToolEntry{
		{Name: "rustc", Version: "1.78"},
	})
	script := GenerateProbeScript(tools)
	assert.Contains(t, script, "rustc")
}

func TestGenerateProbeScript_IsPOSIXShell(t *testing.T) {
	script := GenerateProbeScript(nil)
	require.True(t, strings.HasPrefix(script, "#!/bin/sh\n"))
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git", "git"},
		{"python3", "python3"},
		{"fd-find", "fd-find"},
		{"some;bad", "somebad"},
		{"tool$(cmd)", "toolcmd"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, shellEscape(tt.input))
	}
}
