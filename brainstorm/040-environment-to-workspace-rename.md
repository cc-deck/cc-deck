# 040: Environment-to-Workspace Internal Rename

## Status: brainstorm

## Problem

The internal Go types, constants, and file names use "Environment" terminology (e.g., `EnvironmentDefinition`, `EnvironmentInstance`, `EnvironmentType`, `environments.yaml`) while the CLI already uses "workspace" (`ws new`, `ws attach`, `ws list`). This creates a cognitive disconnect between the user-facing vocabulary and the codebase.

Additionally, the global definition file is named `environments.yaml`, which conflicts with the broader rename from `env` to `ws` commands (brainstorm 039).

## Scope

Pure mechanical rename with no logic changes. The goal is to align internal naming with the CLI's "workspace" terminology.

## Rename Table

| Current | New |
|---------|-----|
| `environments.yaml` (config file) | `workspaces.yaml` |
| `definitionFileName = "environments.yaml"` | `definitionFileName = "workspaces.yaml"` |
| `CC_DECK_DEFINITIONS_FILE` (env var) | `CC_DECK_WORKSPACES_FILE` |
| `EnvironmentDefinition` | `WorkspaceDefinition` |
| `EnvironmentInstance` | `WorkspaceInstance` |
| `EnvironmentType` | `WorkspaceType` |
| `EnvironmentTypeLocal` | `WorkspaceTypeLocal` |
| `EnvironmentTypeContainer` | `WorkspaceTypeContainer` |
| `EnvironmentTypeCompose` | `WorkspaceTypeCompose` |
| `EnvironmentTypeK8sDeploy` | `WorkspaceTypeK8sDeploy` |
| `EnvironmentTypeK8sSandbox` | `WorkspaceTypeK8sSandbox` |
| `EnvironmentTypeSSH` | `WorkspaceTypeSSH` |
| `EnvironmentState` | `WorkspaceState` |
| `EnvironmentStateRunning` | `WorkspaceStateRunning` |
| `EnvironmentStateStopped` | `WorkspaceStateStopped` |
| `EnvironmentStateAvailable` | `WorkspaceStateAvailable` |
| `EnvironmentStateCreating` | `WorkspaceStateCreating` |
| `EnvironmentStateError` | `WorkspaceStateError` |
| `EnvironmentStateUnknown` | `WorkspaceStateUnknown` |
| `EnvironmentStatus` (interface return) | `WorkspaceStatus` |
| `DefinitionStore` | `WorkspaceStore` |
| `DefinitionFile` | `WorkspaceFile` |
| `Environment` (interface) | `Workspace` (interface) |
| `NewEnvironment` (factory) | `NewWorkspace` |
| `LocalEnvironment` | `LocalWorkspace` |
| `ContainerEnvironment` | `ContainerWorkspace` |
| `ComposeEnvironment` | `ComposeWorkspace` |
| `SSHEnvironment` | `SSHWorkspace` |
| `K8sDeployEnvironment` | `K8sDeployWorkspace` |
| `K8sSandboxEnvironment` | `K8sSandboxWorkspace` |
| `ValidateEnvName` | `ValidateWorkspaceName` |
| `ErrNameConflict` | (keep, generic) |
| `ErrNotFound` | (keep, generic) |

## Affected Files (Go source, ~370 occurrences)

### internal/env/ (core types, ~302 occurrences)
- `types.go` - All type definitions and constants
- `definition.go` - DefinitionStore, DefinitionFile
- `state.go` - Instance references
- `interface.go` - Environment interface
- `factory.go` - NewEnvironment
- `container.go` - ContainerEnvironment
- `compose.go` - ComposeEnvironment
- `ssh.go` - SSHEnvironment
- `k8s_deploy.go` - K8sDeployEnvironment
- `local.go` - LocalEnvironment
- `migrate.go` - Migration references
- `project_status.go` - ProjectStatusFile
- `remote_bg.go` - Remote background color
- `k8s_client.go` - K8s client helpers
- All corresponding `*_test.go` files

### internal/cmd/ (~70 occurrences)
- `ws.go` - All command implementations
- `ws_new_test.go` - Creation tests
- `ws_resolve_test.go` - Resolution tests

### internal/integration/ (~3 occurrences)
- `k8s_deploy_test.go`

## Migration

The config file rename (`environments.yaml` to `workspaces.yaml`) needs backward compatibility:
1. On load, check for `workspaces.yaml` first
2. If not found, fall back to `environments.yaml`
3. On first write, use `workspaces.yaml` (auto-migrate)
4. Remove fallback after one release cycle

## Implementation Steps

1. Rename all Go types and constants (sed/IDE refactor)
2. Rename config file constant and add migration fallback
3. Update env var name with backward-compatible fallback
4. Run `make test && make lint` to verify
5. Update docs/specs references (separate commit, lower priority)

## Verification

- `make test` passes
- `make lint` passes
- Existing `environments.yaml` is auto-loaded via fallback
- New writes create `workspaces.yaml`
- All CLI commands work unchanged (they already use `ws`)

## Out of Scope

- CLI command rename (`env` to `ws`) is covered in brainstorm 039
- Centralization of definitions is covered in brainstorm 041
- Documentation and spec file updates (can follow later)
