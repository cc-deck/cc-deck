# Research: Container Environment

**Feature**: 024-container-env | **Date**: 2026-03-20

## Research Topics

### R1: Podman CLI Patterns for Container Lifecycle

**Decision**: Use direct `exec.Command("podman", ...)` calls (consistent with existing codebase patterns in `local.go` and `domains_runtime.go`) wrapped in a shared `internal/podman/` package.

**Rationale**: The codebase uses `os/exec` directly without wrapper abstractions. A shared package organizes podman-specific logic but does not over-abstract. Each function maps to a single podman CLI invocation.

**Key Commands**:

| Operation | Command | Key Output |
|-----------|---------|------------|
| Run | `podman run -d --name <n> -v <vol>:/workspace --secret <s>,type=env,target=<VAR> <image> sleep infinity` | Container ID |
| Start | `podman start <name>` | - |
| Stop | `podman stop <name>` | - |
| Remove | `podman rm -f <name>` | - |
| Inspect | `podman inspect <name> --format '{{.State.Status}}'` | running/exited/stopped |
| Exec | `podman exec [-it] <name> <cmd...>` | Command output |
| Copy | `podman cp <src> <dst>` | - |
| Volume Create | `podman volume create <name>` | Volume name |
| Volume Remove | `podman volume rm <name>` | - |
| Secret Create | `podman secret create <name> -` (stdin) | Secret ID |
| Secret Remove | `podman secret rm <name>` | - |

### R2: Podman Secret Injection

**Decision**: Use `--secret <name>,type=env,target=<ENV_VAR>` to inject secrets as environment variables inside the container.

**Rationale**: Two modes exist for podman secrets: file mount (default, available at `/run/secrets/<name>`) and environment variable (`type=env`). The env var mode is preferred because Claude Code and most API clients read credentials from environment variables, not files. This avoids requiring application changes to read from a secrets directory.

**Alternatives considered**:
- File mount mode (`/run/secrets/`): Would require applications to read from files instead of env vars. Claude Code expects `ANTHROPIC_API_KEY` as an env var.
- Plain `-e` env vars: Visible in `podman inspect` output (security concern per FR-007).

### R3: Rootless Podman Detection

**Decision**: Use `podman info --format '{{.Host.Security.Rootless}}'` to detect rootless mode.

**Rationale**: Returns `true` or `false` directly. Socket path can be obtained via `podman info --format '{{.Host.RemoteSocket.Path}}'`.

**Impact**: Rootless mode affects:
- Socket path: `/run/user/$(id -u)/podman/podman.sock` vs `/run/podman/podman.sock`
- Storage paths: `~/.local/share/containers/storage/` vs `/var/lib/containers/storage/`
- Volume permissions: UID mapping may differ

For this spec, rootless detection is informational (for status display and debugging). The `podman` CLI handles rootless/rootful differences transparently; no code changes are needed based on mode.

### R4: Container State Mapping

**Decision**: Map podman container states to `EnvironmentState` as follows:

| Podman State | EnvironmentState |
|--------------|-----------------|
| `running` | `running` |
| `exited` | `stopped` |
| `stopped` | `stopped` |
| `paused` | `stopped` |
| Container not found | `error` |

**Rationale**: Podman distinguishes `exited` (process terminated) from `stopped` (explicitly stopped), but from the user's perspective both mean the container is not running. The `error` state handles the case where a container was deleted externally.

### R5: Interface Extension Strategy

**Decision**: Use struct fields on `ContainerEnvironment` for type-specific options rather than extending the `Environment` interface.

**Rationale**: The interface contract from spec 023 states: "Type-specific options are passed via type-specific extension structs embedded in the concrete implementation, not in the interface." This means:
- `CreateOpts` keeps existing fields (Image, Storage, Sync)
- `ContainerEnvironment` gets `Ports`, `Credentials`, `AllPorts` fields set by CLI before `Create()`
- `ContainerEnvironment` gets `KeepVolumes` field set by CLI before `Delete()`
- No interface signature changes needed

**Alternatives considered**:
- Extending `CreateOpts` with Ports/Credentials: Would add container-specific fields to a universal struct.
- Adding `DeleteOpts` struct: Would change the `Delete` signature for all implementations.

### R6: Definition/State Separation

**Decision**: Create a new `DefinitionStore` (parallel to `FileStateStore`) for `environments.yaml`, with a join operation for listing.

**Rationale**: The brainstorm confirms a fresh start (no migration from v1). Two stores with clear responsibilities:
- `DefinitionStore`: Human-editable declarations in `$XDG_CONFIG_HOME/cc-deck/environments.yaml`
- `FileStateStore`: Machine-managed runtime state in `$XDG_STATE_HOME/cc-deck/state.yaml` (v2 schema)

The CLI commands join both stores when displaying information (list, status).

### R7: Existing Codebase Patterns

**Findings**:
- `internal/compose/generate.go` already exists for compose YAML generation with tinyproxy sidecar
- `internal/cmd/domains_runtime.go` already calls `exec.Command("podman", ...)` for runtime operations
- No `internal/podman/` package exists yet (to be created)
- Test patterns: `t.TempDir()`, `t.Helper()`, skip when tool unavailable, `require.*`/`assert.*`
- No additional Go dependencies needed (all in go.mod already)
- Atomic state writes via temp file + rename
