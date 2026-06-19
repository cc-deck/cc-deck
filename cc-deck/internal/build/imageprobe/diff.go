package imageprobe

// ComputeToolDiff compares probed tools against required tools and
// returns a diff for each tool. Status values:
//   - "present": tool exists at a compatible version, skip installation
//   - "missing": tool not found, install required
//   - "incompatible": tool exists but at an incompatible version, shadow install
//
// When packageManager is empty (distroless/minimal images), tools that
// would normally be installed via package manager get "binary" as the
// install method, signaling that only binary-only install paths are available.
func ComputeToolDiff(probed map[string]ToolInfo, required []ProbeToolEntry, packageManager string) []ToolDiff {
	var diffs []ToolDiff

	defaultMethod := "package"
	if packageManager == "" {
		defaultMethod = "binary"
	}

	for _, req := range required {
		info, found := probed[req.Name]
		if !found || !info.Present {
			diffs = append(diffs, ToolDiff{
				Tool:          req.Name,
				Required:      req.Version,
				Status:        "missing",
				InstallMethod: defaultMethod,
			})
			continue
		}

		if IsCompatible(info.Version, req.Version) {
			diffs = append(diffs, ToolDiff{
				Tool:      req.Name,
				Required:  req.Version,
				Installed: info.Version,
				Status:    "present",
			})
		} else {
			diffs = append(diffs, ToolDiff{
				Tool:          req.Name,
				Required:      req.Version,
				Installed:     info.Version,
				Status:        "incompatible",
				InstallMethod: "shadow",
			})
		}
	}

	return diffs
}
