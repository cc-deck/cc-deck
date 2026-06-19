package imageprobe

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

// probeOSLine matches the "os" type output.
type probeOSLine struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	IDLike  string `json:"id_like"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// probePkgMgrLine matches the "pkgmgr" type output.
type probePkgMgrLine struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// probeToolLine matches the "tool" type output.
type probeToolLine struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Present bool   `json:"present"`
}

// probeUserLine matches the "user" type output.
type probeUserLine struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	UID   int    `json:"uid"`
	Home  string `json:"home"`
	Shell string `json:"shell"`
}

// probeShellsLine matches the "shells" type output.
type probeShellsLine struct {
	Type      string   `json:"type"`
	Available []string `json:"available"`
}

// ParseProbeOutput parses JSON-per-line probe output into a ProbeResult.
// Non-JSON lines are silently skipped (container stderr noise).
func ParseProbeOutput(output string) (*ProbeResult, error) {
	result := &ProbeResult{
		Tools: make(map[string]ToolInfo),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &typeCheck); err != nil {
			continue
		}

		switch typeCheck.Type {
		case "os":
			var ol probeOSLine
			if err := json.Unmarshal([]byte(line), &ol); err != nil {
				continue
			}
			result.OS = OSInfo{
				ID:      ol.ID,
				IDLike:  ol.IDLike,
				Name:    ol.Name,
				Version: ol.Version,
			}

		case "pkgmgr":
			var pl probePkgMgrLine
			if err := json.Unmarshal([]byte(line), &pl); err != nil {
				continue
			}
			result.PackageManager = pl.Name

		case "tool":
			var tl probeToolLine
			if err := json.Unmarshal([]byte(line), &tl); err != nil {
				continue
			}
			version := extractVersion(tl.Version)
			result.Tools[tl.Name] = ToolInfo{
				Name:    tl.Name,
				Path:    tl.Path,
				Version: version,
				Present: tl.Present,
			}

		case "user":
			var ul probeUserLine
			if err := json.Unmarshal([]byte(line), &ul); err != nil {
				continue
			}
			result.User = UserInfo{
				Name:  ul.Name,
				UID:   ul.UID,
				Home:  ul.Home,
				Shell: ul.Shell,
			}

		case "shells":
			var sl probeShellsLine
			if err := json.Unmarshal([]byte(line), &sl); err != nil {
				continue
			}
			result.Shells = sl.Available
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning probe output: %w", err)
	}

	return result, nil
}

// extractVersion pulls the first semver-like string from a version
// output line (e.g., "git version 2.43.0" -> "2.43.0").
func extractVersion(raw string) string {
	v, ok := ParseVersion(raw)
	if !ok {
		return ""
	}
	return v.String()
}
