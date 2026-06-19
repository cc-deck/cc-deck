package imageprobe

import "time"

// ProbeResult holds the complete output of probing a base image.
type ProbeResult struct {
	ImageRef       string            `json:"image_ref"`
	ImageDigest    string            `json:"image_digest"`
	Timestamp      time.Time         `json:"timestamp"`
	OS             OSInfo            `json:"os"`
	PackageManager string            `json:"package_manager"`
	Tools          map[string]ToolInfo `json:"tools"`
	User           UserInfo          `json:"user"`
	Shells         []string          `json:"shells"`
	DurationMS     int64             `json:"duration_ms"`
}

// OSInfo holds OS identification from /etc/os-release.
type OSInfo struct {
	ID      string `json:"id"`
	IDLike  string `json:"id_like"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolInfo describes a single tool found (or not) in the base image.
type ToolInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Present bool   `json:"present"`
}

// UserInfo holds the default user configuration inside the image.
type UserInfo struct {
	Name  string `json:"name"`
	UID   int    `json:"uid"`
	Home  string `json:"home"`
	Shell string `json:"shell"`
}

// ToolDiff represents the comparison of a single tool between probe
// results and manifest requirements.
type ToolDiff struct {
	Tool          string `json:"tool"`
	Required      string `json:"required_version,omitempty"`
	Installed     string `json:"installed_version,omitempty"`
	Status        string `json:"status"`
	InstallMethod string `json:"install_method,omitempty"`
}

// ProbeCache is the on-disk structure for cached probe results.
type ProbeCache struct {
	Version int                    `json:"version"`
	Entries map[string]ProbeResult `json:"entries"`
}
