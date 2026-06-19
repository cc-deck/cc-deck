package imageprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeToolDiff_Present(t *testing.T) {
	probed := map[string]ToolInfo{
		"git": {Name: "git", Path: "/usr/bin/git", Version: "2.43.0", Present: true},
	}
	required := []ProbeToolEntry{
		{Name: "git", Version: "2.40"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "present", diffs[0].Status)
	assert.Equal(t, "2.43.0", diffs[0].Installed)
}

func TestComputeToolDiff_Missing(t *testing.T) {
	probed := map[string]ToolInfo{}
	required := []ProbeToolEntry{
		{Name: "go", Version: "1.25"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "missing", diffs[0].Status)
	assert.Equal(t, "package", diffs[0].InstallMethod, "should default to package when pkg manager available")
}

func TestComputeToolDiff_MissingNoPkgMgr(t *testing.T) {
	probed := map[string]ToolInfo{}
	required := []ProbeToolEntry{
		{Name: "go", Version: "1.25"},
	}

	diffs := ComputeToolDiff(probed, required, "")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "missing", diffs[0].Status)
	assert.Equal(t, "binary", diffs[0].InstallMethod, "should use binary when no pkg manager")
}

func TestComputeToolDiff_Incompatible(t *testing.T) {
	probed := map[string]ToolInfo{
		"node": {Name: "node", Path: "/usr/bin/node", Version: "18.0.0", Present: true},
	}
	required := []ProbeToolEntry{
		{Name: "node", Version: "22.0"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "incompatible", diffs[0].Status)
	assert.Equal(t, "18.0.0", diffs[0].Installed)
	assert.Equal(t, "shadow", diffs[0].InstallMethod)
}

func TestComputeToolDiff_NotPresent(t *testing.T) {
	probed := map[string]ToolInfo{
		"go": {Name: "go", Path: "", Version: "", Present: false},
	}
	required := []ProbeToolEntry{
		{Name: "go", Version: "1.25"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "missing", diffs[0].Status)
}

func TestComputeToolDiff_NoVersionRequired(t *testing.T) {
	probed := map[string]ToolInfo{
		"curl": {Name: "curl", Path: "/usr/bin/curl", Version: "8.2.1", Present: true},
	}
	required := []ProbeToolEntry{
		{Name: "curl"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 1)
	assert.Equal(t, "present", diffs[0].Status)
}

func TestComputeToolDiff_MultipleMixed(t *testing.T) {
	probed := map[string]ToolInfo{
		"git":     {Name: "git", Path: "/usr/bin/git", Version: "2.43.0", Present: true},
		"python3": {Name: "python3", Path: "/usr/bin/python3", Version: "3.12.4", Present: true},
	}
	required := []ProbeToolEntry{
		{Name: "git", Version: "2.40"},
		{Name: "go", Version: "1.25"},
		{Name: "python3", Version: "3.14"},
	}

	diffs := ComputeToolDiff(probed, required, "dnf")
	assert.Len(t, diffs, 3)

	byName := make(map[string]ToolDiff)
	for _, d := range diffs {
		byName[d.Tool] = d
	}

	assert.Equal(t, "present", byName["git"].Status)
	assert.Equal(t, "missing", byName["go"].Status)
	assert.Equal(t, "incompatible", byName["python3"].Status)
}
