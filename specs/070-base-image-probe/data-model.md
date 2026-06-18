# Data Model: Base Image Probe

## Entities

### ProbeResult

Represents the complete output of probing a base image.

```go
type ProbeResult struct {
    ImageRef       string            `json:"image_ref"`
    ImageDigest    string            `json:"image_digest"`
    Timestamp      time.Time         `json:"timestamp"`
    OS             OSInfo            `json:"os"`
    PackageManager string            `json:"package_manager"` // "dnf", "apt-get", "apk", "yum", ""
    Tools          map[string]ToolInfo `json:"tools"`          // keyed by canonical tool name
    User           UserInfo          `json:"user"`
    Shells         []string          `json:"shells"`          // available shells (bash, zsh, sh)
    Duration       time.Duration     `json:"duration_ms"`
}
```

### OSInfo

```go
type OSInfo struct {
    ID      string `json:"id"`       // from /etc/os-release: "fedora", "ubuntu", "rhel"
    IDLike  string `json:"id_like"`  // from /etc/os-release: "fedora rhel", "debian"
    Name    string `json:"name"`     // human-readable: "Fedora Linux 41"
    Version string `json:"version"`  // "41", "24.04", "9.4"
}
```

### ToolInfo

```go
type ToolInfo struct {
    Name    string `json:"name"`              // canonical name: "git", "python3", "go"
    Path    string `json:"path"`              // absolute path: "/usr/bin/git"
    Version string `json:"version,omitempty"` // parsed version: "2.43.0"
    Present bool   `json:"present"`           // whether the tool was found
}
```

### UserInfo

```go
type UserInfo struct {
    Name    string `json:"name"`     // default non-root user name
    UID     int    `json:"uid"`
    Home    string `json:"home"`     // home directory path
    Shell   string `json:"shell"`    // default shell
}
```

### ProbeCache

File: `<setup-dir>/probe-cache.json`

```go
type ProbeCache struct {
    Version int                    `json:"version"` // schema version, starts at 1
    Entries map[string]ProbeResult `json:"entries"`  // keyed by image ref
}
```

Cache key: image reference string (e.g., `registry.fedoraproject.org/fedora:41`)
Invalidation: compare stored `image_digest` with current digest from `podman inspect`

### ToolDiff

Result of comparing probe results against manifest requirements.

```go
type ToolDiff struct {
    Tool           string `json:"tool"`
    Required       string `json:"required_version,omitempty"` // from manifest
    Installed      string `json:"installed_version,omitempty"` // from probe
    Status         string `json:"status"` // "present", "missing", "incompatible"
    InstallMethod  string `json:"install_method,omitempty"` // "package", "github-release", "skip"
}
```

Status values:
- `present`: tool exists at a compatible version, skip installation
- `missing`: tool not found, install required
- `incompatible`: tool exists but wrong version, shadow via `/usr/local/bin`

## Relationships

```
Manifest.Tools  ─┐
                  ├─> ToolDiff[] ──> Containerfile install steps
ProbeResult.Tools┘

ProbeResult ──cached-in──> ProbeCache.Entries[imageRef]
ProbeCache  ──stored-at──> <setup-dir>/probe-cache.json
```

## State Transitions

```
ProbeCache state for an image ref:

  [No Entry] ──probe──> [Cached]
  [Cached] ──digest unchanged──> [Cache Hit] (reuse)
  [Cached] ──digest changed──> [Stale] ──re-probe──> [Cached]
  [Cache Hit] ──build succeeds──> [Cached] (no change)
```
