package build

import (
	"strings"
)

// wellKnownPaths maps tool names to additional filesystem paths beyond the
// default /usr/bin/<name>. These cover common alternative installation
// locations in OpenShell sandbox images.
var wellKnownPaths = map[string][]string{
	"cargo": {
		"/usr/local/bin/cargo",
		"/sandbox/.cargo/bin/cargo",
		"/sandbox/.rustup/toolchains/*/bin/cargo",
	},
	"rustc": {
		"/usr/local/bin/rustc",
		"/sandbox/.rustup/toolchains/*/bin/rustc",
	},
	"go": {
		"/usr/local/go/bin/go",
		"/sandbox/go/bin/go",
	},
	"node": {
		"/usr/local/bin/node",
	},
	"npm": {
		"/usr/local/bin/npm",
	},
	"npx": {
		"/usr/local/bin/npx",
	},
	"pip": {
		"/usr/local/bin/pip",
		"/sandbox/.venv/bin/pip",
	},
	"pip3": {
		"/usr/local/bin/pip3",
		"/sandbox/.venv/bin/pip3",
	},
	"uv": {
		"/usr/local/bin/uv",
		"/sandbox/.venv/bin/uv",
		"/sandbox/.local/bin/uv",
	},
	"claude": {
		"/usr/local/bin/claude",
		"/sandbox/.local/bin/claude",
		"/sandbox/.local/share/claude/**",
	},
	"git": {
		"/usr/local/bin/git",
	},
	"gh": {
		"/usr/local/bin/gh",
	},
}

// resolveBinaries populates the Binaries field on matched policy components
// that do not already have explicit binaries. For each tool in a component's
// match.Tools list, it looks up the manifest's Tools section to determine the
// install path, then adds well-known paths from the table. Components with
// existing Binaries (len > 0) are preserved as-is (explicit override).
// If manifest is nil, components are returned unchanged.
func resolveBinaries(components []PolicyComponent, manifest *Manifest) []PolicyComponent {
	if manifest == nil {
		return components
	}

	// Build a lookup from tool name to manifest ToolEntry for quick access.
	toolIndex := make(map[string]ToolEntry, len(manifest.Tools))
	for _, t := range manifest.Tools {
		toolIndex[strings.ToLower(t.Name)] = t
	}

	result := make([]PolicyComponent, len(components))
	for i, comp := range components {
		result[i] = comp

		// Skip components that already have explicit binaries.
		if len(comp.Binaries) > 0 {
			continue
		}

		// Collect binary paths from all matched tools.
		seen := make(map[string]bool)
		var paths []string

		for _, toolName := range comp.Match.Tools {
			lower := strings.ToLower(toolName)

			// Look up in manifest tools.
			if entry, ok := toolIndex[lower]; ok {
				switch entry.Install {
				case "github-release":
					if entry.InstallPath != "" {
						addPath(&paths, seen, entry.InstallPath)
					}
				default:
					// "package" or empty: default to /usr/bin/<name>
					addPath(&paths, seen, "/usr/bin/"+lower)
				}
			}

			// Add well-known paths for this tool regardless of manifest presence.
			if wkPaths, ok := wellKnownPaths[lower]; ok {
				for _, p := range wkPaths {
					addPath(&paths, seen, p)
				}
			}
		}

		if len(paths) > 0 {
			binaries := make([]PolicyBinary, len(paths))
			for j, p := range paths {
				binaries[j] = PolicyBinary{Path: p}
			}
			result[i].Binaries = binaries
		}
	}

	return result
}

// addPath appends a path to the list if it has not been seen before.
func addPath(paths *[]string, seen map[string]bool, path string) {
	if !seen[path] {
		seen[path] = true
		*paths = append(*paths, path)
	}
}
