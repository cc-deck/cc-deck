# Data Model: Compose Environment

**Feature**: 025-compose-env | **Date**: 2026-03-21

## Entities

### ComposeEnvironment (new)

The core struct implementing the `Environment` interface for compose-based environments.

```go
type ComposeEnvironment struct {
    name  string
    store *FileStateStore
    defs  *DefinitionStore

    Auth           AuthMode
    Ports          []string
    AllPorts       bool
    Credentials    map[string]string
    Mounts         []string
    AllowedDomains []string // Domain groups/literals for proxy sidecar
    ProjectDir     string   // Project directory (defaults to cwd)
    Gitignore      bool     // Auto-add .cc-deck/ to .gitignore
}
```

**Relationships**: Implements `Environment` interface. Uses `FileStateStore` for runtime state, `DefinitionStore` for persistent definitions, `internal/compose` for YAML generation, `internal/podman` for container inspection, `internal/network` for domain resolution.

### ComposeFields (new, state.yaml)

Runtime state specific to compose environments, stored in the `EnvironmentInstance`.

```go
type ComposeFields struct {
    ProjectDir    string `yaml:"project_dir"`
    ContainerName string `yaml:"container_name"`
    ProxyName     string `yaml:"proxy_name,omitempty"`
}
```

**Validation**: `ProjectDir` must be an absolute path. `ContainerName` follows `cc-deck-<env-name>` convention. `ProxyName` is `cc-deck-<env-name>-proxy` (only set when network filtering is active).

### EnvironmentInstance (modified)

Add `Type` and `Compose` fields.

```go
type EnvironmentInstance struct {
    Name         string            `yaml:"name"`
    Type         EnvironmentType   `yaml:"type"`                // NEW: explicit type
    State        EnvironmentState  `yaml:"state"`
    CreatedAt    time.Time         `yaml:"created_at"`
    LastAttached *time.Time        `yaml:"last_attached,omitempty"`
    Container    *ContainerFields  `yaml:"container,omitempty"`
    Compose      *ComposeFields    `yaml:"compose,omitempty"`   // NEW
    K8s          *K8sFields        `yaml:"k8s,omitempty"`
    Sandbox      *SandboxFields    `yaml:"sandbox,omitempty"`
}
```

**Backwards compatibility**: Existing instances without a `Type` field are treated as `container` type. The YAML `omitempty` on `Type` is intentionally NOT used so the field is always written.

### EnvironmentDefinition (modified)

Add compose-specific fields.

```go
type EnvironmentDefinition struct {
    Name           string          `yaml:"name"`
    Type           EnvironmentType `yaml:"type"`
    Image          string          `yaml:"image,omitempty"`
    Auth           string          `yaml:"auth,omitempty"`
    Storage        *StorageConfig  `yaml:"storage,omitempty"`
    Ports          []string        `yaml:"ports,omitempty"`
    Credentials    []string        `yaml:"credentials,omitempty"`
    Mounts         []string        `yaml:"mounts,omitempty"`
    AllowedDomains []string        `yaml:"allowed-domains,omitempty"` // NEW
    ProjectDir     string          `yaml:"project-dir,omitempty"`     // NEW
}
```

### EnvironmentType (modified)

Add compose constant.

```go
const (
    EnvironmentTypeLocal      EnvironmentType = "local"
    EnvironmentTypeContainer  EnvironmentType = "container"
    EnvironmentTypeCompose    EnvironmentType = "compose"    // NEW
    EnvironmentTypeK8sDeploy  EnvironmentType = "k8s-deploy"
    EnvironmentTypeK8sSandbox EnvironmentType = "k8s-sandbox"
)
```

### Compose Project (filesystem)

Generated files in `.cc-deck/` within the project directory.

```text
.cc-deck/
├── compose.yaml         # Generated compose definition
├── .env                 # Generated credentials (env vars)
├── secrets/             # File-based credentials (e.g., ADC)
│   └── gcloud-adc       # Copied from host, mounted into container
└── proxy/               # Only when --allowed-domains specified
    ├── tinyproxy.conf   # Proxy configuration
    └── whitelist        # Allowed domain patterns
```

**Lifecycle**: Created on `env create`, regenerated if `.cc-deck/` already exists, removed entirely on `env delete`.

## State Transitions

```text
                    create
  (not exists) ──────────────► running
                                │  │
                          stop  │  │  start
                                ▼  │
                             stopped
                                │
                          delete│
                                ▼
                           (removed)

  Notes:
  - Attach auto-starts stopped environments
  - Delete requires --force for running environments
  - Reconciliation updates state based on podman inspect
  - Error state set when container is missing but state record exists
```

## Credential Flow

```text
Host Environment
    │
    ▼
detectAuthMode()           ◄── Shared helper (auth.go)
    │
    ▼
detectAuthCredentials()    ◄── Shared helper (auth.go)
    │
    ├── Plain values ──► .cc-deck/.env (KEY=VALUE lines)
    │                        └── Referenced via env_file in compose.yaml
    │
    └── File values ───► .cc-deck/secrets/<name> (copied from host)
                             └── Mounted via compose volumes at /run/secrets/<name>
                             └── Env var set to /run/secrets/<name>
```

## Naming Conventions

| Entity | Pattern | Example |
|--------|---------|---------|
| Session container | `cc-deck-<env-name>` | `cc-deck-mydev` |
| Proxy container | `cc-deck-<env-name>-proxy` | `cc-deck-mydev-proxy` |
| Compose project dir | `.cc-deck/` | `~/projects/my-api/.cc-deck/` |
| Zellij session | `cc-deck` (always, per container isolation) | `cc-deck` |
| Volume (named-volume mode) | `cc-deck-<env-name>-data` | `cc-deck-mydev-data` |

## Validation Rules

- **Environment name**: Must match `^[a-z0-9][a-z0-9-]*$`, max 40 characters (existing `ValidateEnvName()`)
- **Project directory**: Must exist and be writable. Defaults to cwd.
- **Compose runtime**: Must be available in PATH (`podman-compose`, `docker compose`, or `docker-compose`)
- **Domain groups**: Must be resolvable by the domain resolver. Unknown groups produce a clear error listing available groups.
- **Ports**: Format `host:container` (existing validation)
- **Storage type**: `host-path` (default for compose) or `named-volume`
