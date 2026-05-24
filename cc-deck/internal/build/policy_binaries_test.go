package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBinaries_PackageToolResolvesToUsrBinPlusWellKnown(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_go",
			Name: "go packages",
			Match: MatchCondition{
				Tools: []string{"go"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "go", Install: "package"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)
	require.NotEmpty(t, resolved[0].Binaries)

	paths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, paths, "/usr/bin/go", "package tool should resolve to /usr/bin/<name>")
	assert.Contains(t, paths, "/usr/local/go/bin/go", "should include well-known path")
	assert.Contains(t, paths, "/sandbox/go/bin/go", "should include well-known path")
}

func TestResolveBinaries_EmptyInstallDefaultsToPackage(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_go",
			Name: "go packages",
			Match: MatchCondition{
				Tools: []string{"go"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "go"}, // Install field omitted
		},
	}

	resolved := resolveBinaries(components, manifest)
	paths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, paths, "/usr/bin/go", "empty install should default to package behavior")
}

func TestResolveBinaries_GithubReleaseUsesInstallPath(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "custom_tool",
			Name: "custom tool",
			Match: MatchCondition{
				Tools: []string{"mytool"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "mytool", Install: "github-release", Repo: "owner/mytool", InstallPath: "/usr/local/bin/mytool"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)
	require.NotEmpty(t, resolved[0].Binaries)

	paths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, paths, "/usr/local/bin/mytool", "github-release should use InstallPath")
	assert.NotContains(t, paths, "/usr/bin/mytool", "github-release should NOT add /usr/bin default")
}

func TestResolveBinaries_ExplicitBinariesPreserved(t *testing.T) {
	explicit := []PolicyBinary{
		{Path: "/opt/custom/bin/cargo"},
	}
	components := []PolicyComponent{
		{
			Key:  "pkg_rust",
			Name: "rust packages",
			Match: MatchCondition{
				Tools: []string{"cargo"},
			},
			Binaries: explicit,
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "cargo", Install: "package"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)
	require.Len(t, resolved[0].Binaries, 1, "explicit binaries should be preserved, not augmented")
	assert.Equal(t, "/opt/custom/bin/cargo", resolved[0].Binaries[0].Path)
}

func TestResolveBinaries_ToolNotInManifestSkipped(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_rust",
			Name: "rust packages",
			Match: MatchCondition{
				Tools: []string{"cargo"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{}, // cargo not in manifest
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)

	// Should still get well-known paths even if not in manifest
	paths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, paths, "/usr/local/bin/cargo", "well-known paths should be added even without manifest entry")
	assert.NotContains(t, paths, "/usr/bin/cargo", "should NOT add /usr/bin default without manifest entry")
}

func TestResolveBinaries_Deduplication(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_rust",
			Name: "rust packages",
			Match: MatchCondition{
				Tools: []string{"cargo", "rustc"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "cargo", Install: "package"},
			{Name: "rustc", Install: "package"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)

	paths := binaryPaths(resolved[0].Binaries)
	// Check that there are no duplicate paths
	seen := make(map[string]bool)
	for _, p := range paths {
		assert.False(t, seen[p], "path %q should not appear twice", p)
		seen[p] = true
	}
}

func TestResolveBinaries_MultipleComponents(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_go",
			Name: "go packages",
			Match: MatchCondition{
				Tools: []string{"go"},
			},
		},
		{
			Key:  "pkg_rust",
			Name: "rust packages",
			Match: MatchCondition{
				Tools: []string{"cargo"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "go", Install: "package"},
			{Name: "cargo", Install: "package"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 2)

	goPaths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, goPaths, "/usr/bin/go")

	rustPaths := binaryPaths(resolved[1].Binaries)
	assert.Contains(t, rustPaths, "/usr/bin/cargo")
}

func TestResolveBinaries_CaseInsensitiveLookup(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "pkg_go",
			Name: "go packages",
			Match: MatchCondition{
				Tools: []string{"Go"},
			},
		},
	}
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "go", Install: "package"},
		},
	}

	resolved := resolveBinaries(components, manifest)
	paths := binaryPaths(resolved[0].Binaries)
	assert.Contains(t, paths, "/usr/bin/go", "case-insensitive lookup should match")
}

func TestResolveBinaries_NoToolsInComponent(t *testing.T) {
	components := []PolicyComponent{
		{
			Key:  "claude_code",
			Name: "Claude Code",
			Match: MatchCondition{
				Always: true,
			},
			Binaries: []PolicyBinary{
				{Path: "/usr/local/bin/claude"},
			},
		},
	}
	manifest := &Manifest{}

	resolved := resolveBinaries(components, manifest)
	require.Len(t, resolved, 1)
	require.Len(t, resolved[0].Binaries, 1, "component with explicit binaries and no match.tools should be unchanged")
}

// binaryPaths extracts path strings from a slice of PolicyBinary.
func binaryPaths(binaries []PolicyBinary) []string {
	paths := make([]string, len(binaries))
	for i, b := range binaries {
		paths[i] = b.Path
	}
	return paths
}
