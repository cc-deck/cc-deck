# Brainstorm: Container and Compose Environments

**Date:** 2026-03-20
**Status:** Brainstorm (ready for spec)
**Depends on:** 023-env-interface (Environment interface, state.yaml, CLI)
**Supersedes:** Podman sections of 023-execution-environments.md

## Core Decision: Two Environment Types

The original brainstorm (023) defined a single "Podman" type. After discussion, this splits into two types with different complexity levels:

| Type | Mechanism | Network filtering | Sidecars (MCP, proxy) | Storage |
|------|-----------|-------------------|----------------------|---------|
| `container` | `podman run` | No | No | Single volume or bind mount |
| `compose` | `podman-compose` (default) | Yes (tinyproxy sidecar) | Yes (future MCP containers) | Multiple volumes |

Not every environment type has the same capabilities. Operations that are not supported for a given type return a clear error explaining the limitation and suggesting the alternative type.

### Naming Rationale

- **`container`**: Focuses on what it is (a single container), not the tool. Reads naturally: `cc-deck env create mydev --type container`.
- **`compose`**: Describes the orchestration mechanism. Runtime-agnostic: works with `podman-compose` (default) or `docker compose` (override). The compose type enables multi-container setups (proxy sidecar, future MCP containers).

### Compose Runtime Detection

Default to `podman-compose` (with dash). Allow override via config:

```yaml
# ~/.config/cc-deck/config.yaml
defaults:
  compose-runtime: podman-compose   # default
  # alternatives: docker-compose, "docker compose"
```

Auto-detection order if not configured:
1. `podman-compose` (preferred)
2. `docker compose` (v2 plugin)
3. `docker-compose` (legacy standalone)

Cache the detected runtime for the session.

## Architecture: Definition/State Separation

### Problem

Spec 023's `state.yaml` mixes environment definitions (image, storage config, ports) with runtime state (container ID, timestamps, current state). This makes the file fragile to hand-edit and clutters definitions with transient data.

### Solution

Split into two files:

**Environment definitions** (human-editable, version-controllable):
```yaml
# $XDG_CONFIG_HOME/cc-deck/environments.yaml
version: 1
environments:
  - name: my-project
    type: container
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage:
      type: named-volume
    ports: []

  - name: eval-env
    type: compose
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage:
      type: named-volume
    allowed-domains: [anthropic, github, npm]
    ports:
      - "8082:8082"

  - name: backend
    type: k8s-deploy
    image: quay.io/cc-deck/cc-deck-demo:latest
    profile: anthropic-prod
    storage:
      type: pvc
      size: 10Gi
```

**Environment state** (machine-managed):
```yaml
# $XDG_STATE_HOME/cc-deck/state.yaml
version: 2
instances:
  - name: my-project
    state: running
    created_at: 2026-03-20T10:00:00Z
    last_attached: 2026-03-20T15:30:00Z
    container:
      container_id: abc123def456
      container_name: cc-deck-my-project

  - name: eval-env
    state: stopped
    created_at: 2026-03-19T08:00:00Z
    last_attached: 2026-03-19T16:00:00Z
    container:
      container_id: def789abc012
      container_name: cc-deck-eval-env
```

### Workflow

1. `cc-deck env create my-project --type container --image ...` writes to **both** files
2. User can hand-edit `environments.yaml` (change image, add ports)
3. `cc-deck env list` joins definitions + state for display
4. `cc-deck env delete` removes from both
5. `cc-deck env status` reads definition (for config display) + state (for runtime status)

### Migration

No migration needed. Nobody uses `state.yaml` in production yet (spec 023 just merged). The state.yaml schema version bumps to 2 with the slimmed-down structure.

### Templates (Deferred)

A future spec can add reusable environment templates:

```yaml
templates:
  dev-container:
    type: container
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage: { type: named-volume }

# Usage: cc-deck env create my-project --template dev-container
```

This layers naturally on top of the definition/state separation.

## Shared Code: `internal/podman/` Package

Both `container` and `compose` types use podman underneath. Shared interaction layer:

```go
package podman

// Container lifecycle
func Run(opts RunOpts) (containerID string, err error)
func Start(nameOrID string) error
func Stop(nameOrID string) error
func Remove(nameOrID string, force bool) error
func Inspect(nameOrID string) (*ContainerInfo, error)

// Exec
func Exec(nameOrID string, cmd []string, interactive bool) error

// File transfer
func Cp(src, dst string) error  // podman cp

// Volumes
func VolumeCreate(name string) error
func VolumeRemove(name string) error

// Secrets
func SecretCreate(name string, data []byte) error
func SecretRemove(name string) error
func SecretExists(name string) bool

// Detection
func Available() bool  // is podman in PATH?
```

Environment implementations:
- `internal/env/container.go` uses `internal/podman/` directly
- `internal/env/compose.go` uses `internal/podman/` for inspect/secrets + compose CLI for lifecycle

## Spec 025: Container Environment (single `podman run`)

### Lifecycle

| Method | Implementation |
|--------|---------------|
| Create | Validate name, check podman available, `podman volume create` (if named-volume), `podman secret create` (credentials), `podman run -d --name cc-deck-<name> --secret ... -v ... <image> sleep infinity` |
| Start | `podman start cc-deck-<name>` |
| Stop | `podman stop cc-deck-<name>` |
| Delete | `podman rm` + optional volume cleanup + secret cleanup |
| Status | `podman inspect` for container state, `podman exec` to read pane map for sessions |
| Attach | `podman exec -it cc-deck-<name> zellij attach cc-deck --create` |
| Exec | `podman exec cc-deck-<name> <cmd>` |
| Push | `podman cp <local-path> cc-deck-<name>:/workspace/` |
| Pull | `podman cp cc-deck-<name>:/workspace/<path> <local-path>` |
| Harvest | Not supported (return error, suggest git harvest brainstorm) |

### Container Naming

Container name: `cc-deck-<env-name>` (same prefix as Zellij sessions for consistency).
Volume name: `cc-deck-<env-name>-data` (for named volumes).
Secret names: `cc-deck-<env-name>-<key>` (e.g., `cc-deck-mydev-anthropic-api-key`).

### Storage

- **Named volume** (default): `podman volume create cc-deck-<name>-data`, mounted at `/workspace`
- **Host bind mount**: `-v /absolute/path:/workspace`, specified via `--path`
- Configurable in `environments.yaml`

### Ports

- No ports exposed by default (secure default)
- Explicit: `--port 8082:8082` (repeatable flag)
- All declared: `--all-ports` (equivalent to `podman run -P`)

### Credentials

Use `podman secret` for credential injection (more secure than env vars, not visible in `podman inspect`):

```bash
# During create:
podman secret create cc-deck-mydev-anthropic-api-key <(echo "$ANTHROPIC_API_KEY")
podman run --secret cc-deck-mydev-anthropic-api-key,target=ANTHROPIC_API_KEY ...
```

Credential sources (resolution order):
1. `--credential KEY=VALUE` flag
2. Environment definition in `environments.yaml`
3. Host environment variable (auto-detect `ANTHROPIC_API_KEY`, `GOOGLE_APPLICATION_CREDENTIALS`)
4. Interactive prompt

### Reconciliation

`cc-deck env list` reconciles container environments by running `podman inspect`:
- Container exists and running: state = running
- Container exists but stopped: state = stopped
- Container does not exist: state = error (or remove stale record)

### CLI Flags for `env create --type container`

```
--image      Container image (required, or from config default)
--port       Port mapping host:container (repeatable)
--all-ports  Expose all declared ports
--storage    Storage type: named-volume (default), host-path
--path       Host path for bind mount (requires --storage host-path)
--credential KEY=VALUE (repeatable)
```

### Not Supported (clear errors)

- `--allowed-domains`: "Network filtering requires --type compose"
- Harvest: "Git harvest not available for container type. Use push/pull for file transfer."

## Spec 026: Compose Environment (`podman-compose`)

### Scope (high-level, detailed spec later)

- Reuses existing `internal/compose/` generator from spec 022
- Network filtering via tinyproxy sidecar (domain groups)
- Multiple volumes (workspace + future MCP storage volumes)
- Credentials via podman secrets (shared with container type)
- Copy sync via `podman exec` (compose does not support `podman cp` directly on service names)
- Foundation for MCP container sidecars (future spec)
- `--image`, `--port`, `--storage`, `--allowed-domains` flags

### Key Difference from Container Type

The compose type generates and manages a compose project directory:

```
$XDG_STATE_HOME/cc-deck/compose/my-project/
  compose.yaml        # generated
  .env                # credentials (if not using secrets)
  proxy/
    tinyproxy.conf    # generated (if network filtering)
    whitelist         # generated (if network filtering)
```

Lifecycle uses `podman-compose up -d`, `podman-compose down`, etc.

## Shared Type: `ContainerFields`

Replaces the existing `PodmanFields`. Used by both `container` and `compose` types in state.yaml:

```go
type ContainerFields struct {
    ContainerID   string   `yaml:"container_id,omitempty"`
    ContainerName string   `yaml:"container_name,omitempty"`
    Image         string   `yaml:"image,omitempty"`
    Ports         []string `yaml:"ports,omitempty"`
}
```

For compose, a `ComposeFields` struct may extend this later with project directory and service names.

## Config Defaults

```yaml
# ~/.config/cc-deck/config.yaml
defaults:
  compose-runtime: podman-compose
  container:
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage: named-volume
  compose:
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage: named-volume
    allowed-domains: [anthropic, github]
```

## Updated Type Enum

```go
const (
    EnvironmentTypeLocal      EnvironmentType = "local"
    EnvironmentTypeContainer  EnvironmentType = "container"
    EnvironmentTypeCompose    EnvironmentType = "compose"
    EnvironmentTypeK8sDeploy  EnvironmentType = "k8s-deploy"
    EnvironmentTypeK8sSandbox EnvironmentType = "k8s-sandbox"
)
```

The existing `EnvironmentTypePodman` is removed (nobody uses it in production yet).

## Spec Dependency Graph

```
023 (Interface + CLI)          DONE
 |
 +-- 025 (Container: podman run)        <-- next
 |    |
 |    +-- definition/state separation
 |    +-- internal/podman/ package
 |    +-- ContainerEnvironment
 |
 +-- 026 (Compose: podman-compose)      <-- after 025
 |    |
 |    +-- reuse internal/podman/
 |    +-- reuse internal/compose/
 |    +-- network filtering integration
 |    +-- MCP sidecar foundation
 |
 +-- 024 (K8s Deploy refactor)
 |    +-- K8s Sandbox (future)
 |
 +-- brainstorm: git-harvest-sync (cross-env)
```

## Resolved Questions

1. **Volume cleanup on delete**: Delete volumes by default. Provide `--keep-volumes` flag to preserve data. Users who care about persistence should use explicit flags to opt into keeping volumes.

2. **Rootless podman**: Auto-detect and adapt. Use `podman info --format '{{.Host.RemoteSocket.Path}}'` and `podman info --format '{{.Host.Security.Rootless}}'` to detect rootless mode and adjust socket paths, UID mapping, and volume permissions accordingly.

3. **Container image default**: Fall back to `quay.io/cc-deck/cc-deck-demo:latest` if no `--image` flag and no config default. Show a warning: "Using default demo image. Set a default with 'cc-deck config set container.image <image>'." This lets new users get started quickly while nudging toward explicit configuration.

4. **Zellij session name inside container**: Always `cc-deck`. Each container is isolated, so there is no collision risk. Simpler for attach: `zellij attach cc-deck --create`.

5. **State file**: Fresh start. Nobody uses v1 in production. The new schema separates definitions (environments.yaml) from state (state.yaml) cleanly.
