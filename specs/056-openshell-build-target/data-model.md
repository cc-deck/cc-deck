# Data Model: OpenShell Build Target

## Entities

### OpenShellTarget

Extension of `TargetsConfig` for OpenShell sandbox images.

```go
// OpenShellTarget describes the OpenShell sandbox image to build.
type OpenShellTarget struct {
    Name     string              `yaml:"name"`
    Tag      string              `yaml:"tag,omitempty"`
    Base     string              `yaml:"base,omitempty"`
    Registry string              `yaml:"registry,omitempty"`
    Policy   *OpenShellPolicy    `yaml:"policy,omitempty"`
}
```

**Fields**:
- `Name` (required): Image name for the built sandbox image
- `Tag` (optional, default: `latest`): Image tag
- `Base` (optional, default: `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`): Base image
- `Registry` (optional): Registry prefix for push support
- `Policy` (optional): Explicit policy overrides

**Location**: `cc-deck/internal/build/manifest.go`

### OpenShellPolicy

Explicit policy overrides that merge with auto-generated policy from `network.allowed_domains`.

```go
// OpenShellPolicy defines explicit OpenShell policy overrides.
type OpenShellPolicy struct {
    FilesystemPolicy *FilesystemPolicy         `yaml:"filesystem_policy,omitempty"`
    Landlock         *LandlockConfig            `yaml:"landlock,omitempty"`
    Process          *ProcessConfig             `yaml:"process,omitempty"`
    NetworkPolicies  map[string]NetworkPolicy   `yaml:"network_policies,omitempty"`
}

// FilesystemPolicy defines read-only and read-write filesystem paths.
type FilesystemPolicy struct {
    IncludeWorkdir bool     `yaml:"include_workdir,omitempty"`
    ReadOnly       []string `yaml:"read_only,omitempty"`
    ReadWrite      []string `yaml:"read_write,omitempty"`
}

// LandlockConfig holds Landlock LSM settings.
type LandlockConfig struct {
    Compatibility string `yaml:"compatibility,omitempty"`
}

// ProcessConfig defines sandbox process execution settings.
type ProcessConfig struct {
    RunAsUser  string `yaml:"run_as_user,omitempty"`
    RunAsGroup string `yaml:"run_as_group,omitempty"`
}

// NetworkPolicy defines a named set of endpoint/binary restrictions.
type NetworkPolicy struct {
    Name      string           `yaml:"name"`
    Endpoints []PolicyEndpoint `yaml:"endpoints"`
    Binaries  []PolicyBinary   `yaml:"binaries,omitempty"`
}

// PolicyEndpoint is a host:port pair for network access control.
type PolicyEndpoint struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

// PolicyBinary restricts network access to a specific binary path.
type PolicyBinary struct {
    Path string `yaml:"path"`
}
```

**Location**: `cc-deck/internal/build/policy.go`

### TargetsConfig (modified)

```go
// TargetsConfig holds per-target configuration.
type TargetsConfig struct {
    Container *ContainerTarget  `yaml:"container,omitempty"`
    SSH       *SSHTarget        `yaml:"ssh,omitempty"`
    OpenShell *OpenShellTarget  `yaml:"openshell,omitempty"`  // NEW
}
```

**Location**: `cc-deck/internal/build/manifest.go`

### BinaryMapping (build-time only)

Not a persistent struct. During Containerfile generation, the AI-driven `/cc-deck.build` command builds up a mapping of tool names to binary paths as it writes install instructions. This mapping is consumed when generating `policy.yaml`.

Well-known defaults:

| Tool | Install Method | Binary Path |
|------|---------------|-------------|
| git | package (dnf) | `/usr/bin/git` |
| node/nodejs | package (dnf) | `/usr/bin/node` |
| python3 | package (dnf) | `/usr/bin/python3` |
| go | package (dnf) | `/usr/bin/go` |
| Claude Code | native installer | `/usr/local/bin/claude` |
| npm globals | npm install -g | `/usr/local/bin/<name>` |
| github-release | curl/tar | per `install_path` field |

## Relationships

```text
Manifest
└── Targets
    ├── Container (existing)
    ├── SSH (existing)
    └── OpenShell (NEW)
        ├── image config (name, tag, base, registry)
        └── Policy (optional overrides)
            ├── filesystem_policy
            ├── landlock
            ├── process
            └── network_policies (keyed by slug)
                ├── name
                ├── endpoints[] (host, port)
                └── binaries[] (path)

Network.AllowedDomains ──(auto-generates)──> PolicyFile.network_policies
OpenShellPolicy.NetworkPolicies ──(overrides)──> PolicyFile.network_policies
```

## Validation Rules

1. `OpenShellTarget.Name` is required when `targets.openshell` is present
2. `OpenShellPolicy.NetworkPolicies` entries must have at least one endpoint
3. Policy merge: explicit entries override auto-generated entries matched by endpoint host
4. `Base` defaults to `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`
5. `Tag` defaults to `latest`

## State Transitions

N/A. The build target is stateless, producing artifacts from the manifest.
