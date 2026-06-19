package imageprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProbeOutput_FullOutput(t *testing.T) {
	output := `{"type":"os","id":"fedora","id_like":"","name":"Fedora Linux 41","version":"41"}
{"type":"pkgmgr","name":"dnf","path":"/usr/bin/dnf"}
{"type":"tool","name":"git","path":"/usr/bin/git","version":"git version 2.43.0","present":true}
{"type":"tool","name":"go","path":"","version":"","present":false}
{"type":"user","name":"root","uid":0,"home":"/root","shell":"/bin/bash"}
{"type":"shells","available":["bash","sh"]}
`
	result, err := ParseProbeOutput(output)
	require.NoError(t, err)

	assert.Equal(t, "fedora", result.OS.ID)
	assert.Equal(t, "Fedora Linux 41", result.OS.Name)
	assert.Equal(t, "41", result.OS.Version)
	assert.Equal(t, "dnf", result.PackageManager)

	git, ok := result.Tools["git"]
	require.True(t, ok)
	assert.True(t, git.Present)
	assert.Equal(t, "/usr/bin/git", git.Path)
	assert.Equal(t, "2.43.0", git.Version)

	goTool, ok := result.Tools["go"]
	require.True(t, ok)
	assert.False(t, goTool.Present)

	assert.Equal(t, "root", result.User.Name)
	assert.Equal(t, 0, result.User.UID)
	assert.Equal(t, []string{"bash", "sh"}, result.Shells)
}

func TestParseProbeOutput_SkipsNonJSON(t *testing.T) {
	output := `WARNING: some container noise
{"type":"os","id":"ubuntu","id_like":"debian","name":"Ubuntu 24.04","version":"24.04"}
random stderr garbage
{"type":"pkgmgr","name":"apt-get","path":"/usr/bin/apt-get"}
`
	result, err := ParseProbeOutput(output)
	require.NoError(t, err)

	assert.Equal(t, "ubuntu", result.OS.ID)
	assert.Equal(t, "debian", result.OS.IDLike)
	assert.Equal(t, "apt-get", result.PackageManager)
}

func TestParseProbeOutput_EmptyOutput(t *testing.T) {
	result, err := ParseProbeOutput("")
	require.NoError(t, err)
	assert.NotNil(t, result.Tools)
	assert.Empty(t, result.Tools)
}

func TestParseProbeOutput_VersionExtraction(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantVer string
	}{
		{
			name:    "git version format",
			line:    `{"type":"tool","name":"git","path":"/usr/bin/git","version":"git version 2.43.0","present":true}`,
			wantVer: "2.43.0",
		},
		{
			name:    "python format",
			line:    `{"type":"tool","name":"python3","path":"/usr/bin/python3","version":"Python 3.12.4","present":true}`,
			wantVer: "3.12.4",
		},
		{
			name:    "major.minor only",
			line:    `{"type":"tool","name":"jq","path":"/usr/bin/jq","version":"jq-1.7","present":true}`,
			wantVer: "1.7.0",
		},
		{
			name:    "no version string",
			line:    `{"type":"tool","name":"make","path":"/usr/bin/make","version":"","present":true}`,
			wantVer: "",
		},
		{
			name:    "go version format",
			line:    `{"type":"tool","name":"go","path":"/usr/local/go/bin/go","version":"go version go1.22.5 linux/amd64","present":true}`,
			wantVer: "1.22.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseProbeOutput(tt.line)
			require.NoError(t, err)
			for _, tool := range result.Tools {
				assert.Equal(t, tt.wantVer, tool.Version)
			}
		})
	}
}

func TestParseProbeOutput_UBIImage(t *testing.T) {
	output := `{"type":"os","id":"rhel","id_like":"fedora","name":"Red Hat Enterprise Linux 9.4","version":"9.4"}
{"type":"pkgmgr","name":"dnf","path":"/usr/bin/dnf"}
{"type":"tool","name":"curl","path":"/usr/bin/curl","version":"curl 8.2.1","present":true}
{"type":"user","name":"root","uid":0,"home":"/root","shell":"/bin/bash"}
{"type":"shells","available":["bash","sh"]}
`
	result, err := ParseProbeOutput(output)
	require.NoError(t, err)

	assert.Equal(t, "rhel", result.OS.ID)
	assert.Equal(t, "fedora", result.OS.IDLike)
	assert.Equal(t, "dnf", result.PackageManager)
}
