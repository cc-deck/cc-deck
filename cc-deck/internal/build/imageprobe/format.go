package imageprobe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FormatTable renders a ProbeResult as a human-readable summary table.
func FormatTable(result *ProbeResult, cached bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Base Image Probe: %s\n", result.ImageRef))
	sb.WriteString(strings.Repeat("━", 52) + "\n\n")

	sb.WriteString(fmt.Sprintf("  OS:              %s\n", result.OS.Name))
	sb.WriteString(fmt.Sprintf("  Package Manager: %s\n", result.PackageManager))
	sb.WriteString(fmt.Sprintf("  User:            %s (uid=%d, %s)\n", result.User.Name, result.User.UID, result.User.Home))
	sb.WriteString(fmt.Sprintf("  Shells:          %s\n", strings.Join(result.Shells, ", ")))

	sb.WriteString("\n  Tools:\n")

	names := sortedToolNames(result.Tools)
	for _, name := range names {
		tool := result.Tools[name]
		if tool.Present {
			ver := tool.Version
			if ver == "" {
				ver = "(unknown)"
			}
			sb.WriteString(fmt.Sprintf("  ✓ %-14s %-10s %s\n", name, ver, tool.Path))
		} else {
			sb.WriteString(fmt.Sprintf("  ✗ %-14s -          not found\n", name))
		}
	}

	source := "fresh"
	if cached {
		source = "cached"
	}
	sb.WriteString(fmt.Sprintf("\n  Probed in %.1fs (%s)\n", float64(result.DurationMS)/1000, source))

	return sb.String()
}

// FormatDiff appends a tool diff section to a table output.
func FormatDiff(diffs []ToolDiff) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n  Tool Diff (vs manifest requirements):\n")

	for _, d := range diffs {
		switch d.Status {
		case "present":
			sb.WriteString(fmt.Sprintf("  ✓ %-14s %s ≥ %s (required)    → skip\n",
				d.Tool, d.Installed, d.Required))
		case "missing":
			req := d.Required
			if req == "" {
				req = "any"
			}
			sb.WriteString(fmt.Sprintf("  ✗ %-14s -      (%s required)        → install\n",
				d.Tool, req))
		case "incompatible":
			sb.WriteString(fmt.Sprintf("  ~ %-14s %s < %s (required)        → shadow in /usr/local/bin\n",
				d.Tool, d.Installed, d.Required))
		}
	}

	return sb.String()
}

// jsonOutput is the JSON output schema with a cached flag.
type jsonOutput struct {
	ImageRef       string             `json:"image_ref"`
	ImageDigest    string             `json:"image_digest"`
	Cached         bool               `json:"cached"`
	OS             OSInfo             `json:"os"`
	PackageManager string             `json:"package_manager"`
	Tools          map[string]ToolInfo `json:"tools"`
	User           UserInfo           `json:"user"`
	Shells         []string           `json:"shells"`
	DurationMS     int64              `json:"duration_ms"`
}

// FormatJSON marshals a ProbeResult as JSON with a cached boolean field.
func FormatJSON(result *ProbeResult, cached bool) (string, error) {
	out := jsonOutput{
		ImageRef:       result.ImageRef,
		ImageDigest:    result.ImageDigest,
		Cached:         cached,
		OS:             result.OS,
		PackageManager: result.PackageManager,
		Tools:          result.Tools,
		User:           result.User,
		Shells:         result.Shells,
		DurationMS:     result.DurationMS,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling probe result: %w", err)
	}

	return string(data), nil
}

func sortedToolNames(tools map[string]ToolInfo) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
