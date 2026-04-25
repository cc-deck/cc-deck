# Research: Workspace Lifecycle Redesign

**Date**: 2026-04-25
**Branch**: 043-workspace-lifecycle

## Decision 1: Interface Split Strategy

**Decision**: Split the current `Workspace` interface into a base `Workspace` interface (universal methods) and an `InfraManager` interface (infrastructure-only methods). Use Go type assertion at the CLI layer.

**Rationale**: The current interface forces all types to implement `Start()`/`Stop()`, but local and SSH workspaces return `ErrNotSupported`. A type assertion (`if im, ok := ws.(InfraManager); ok`) is the idiomatic Go pattern for optional capabilities. It eliminates fake no-ops and makes the type system enforce the contract.

**Alternatives considered**:
- Flat interface with no-op implementations: Requires every type to implement methods that do nothing. SSH currently returns `ErrNotSupported` which is an error, not a no-op.
- Separate command structs per type: Over-engineering for 2 methods. Type assertion is simpler.

## Decision 2: State Model Migration

**Decision**: Replace the single `State WorkspaceState` field in `WorkspaceInstance` with two fields: `InfraState *string` (nullable, only for InfraManager types) and `SessionState string` (always present).

**Rationale**: The current state model conflates infrastructure state with session state, leading to confusing labels for non-infrastructure types (local shows "available" or "stopped" when neither is meaningful). Two dimensions allow each to be tracked and displayed independently.

**Migration approach**: On `StateFile` load, if `Version < 3`, convert each instance:
- For local/ssh types: `State "running"` -> `SessionState "exists"`, `InfraState nil`
- For container/compose/k8s types: `State "running"` -> `InfraState "running"`, `SessionState "none"` (session state reconciled on next status check)
- Bump version to 3, save atomically.

**Alternatives considered**:
- Keep single state with type-aware display: Simpler storage but muddies the data model. Display logic would need to "know" what the state means per type.
- Embed state in type-specific fields: Scatters state across ContainerFields/K8sFields/etc. Makes list/status queries complex.

## Decision 3: KillSession Implementation Per Type

**Decision**: Add `KillSession()` to the base `Workspace` interface. Each type implements it using its existing session-detection mechanism:

| Type | Implementation |
|------|---------------|
| local | `zellij kill-session cc-deck-<name>` |
| container | `podman exec cc-deck-<name> zellij kill-session cc-deck-<name>` |
| compose | Same as container (session container name) |
| ssh | SSH exec: `zellij kill-session cc-deck-<name>` on remote |
| k8s-deploy | `kubectl exec cc-deck-<name>-0 -- zellij kill-session cc-deck-<name>` |

**Rationale**: Each type already has a mechanism to detect Zellij sessions (zellijSessionExists, ContainerHasZellijSession, remoteHasSession, k8sHasZellijSession). KillSession follows the same dispatch pattern.

## Decision 4: Attach Lazy-Start Flow

**Decision**: `Attach()` checks if the workspace implements `InfraManager`, and if so, checks `InfraState`. If stopped, calls `Start()` first (with existing progress output), then proceeds to session creation/reattach.

**Rationale**: This is already partially implemented in container.go and compose.go (both auto-start on attach). The change is to make this a documented pattern in the base `Attach()` contract rather than a per-type opt-in.

**Key detail**: The lazy-start must happen before session detection, since the session cannot exist inside stopped infrastructure.

## Decision 5: Stop Implies KillSession

**Decision**: `InfraManager.Stop()` must call `KillSession()` before stopping infrastructure.

**Rationale**: A Zellij session inside a container dies when the container stops. Explicitly killing it first keeps the state model clean (session_state transitions to "none" before infra_state transitions to "stopped"). Without this, the session state would become stale until the next reconciliation.

## Key Files and Changes

| File | Changes |
|------|---------|
| `interface.go` | Remove `Start()`, `Stop()` from Workspace. Add `KillSession()`. Define `InfraManager` interface. |
| `types.go` | Replace `WorkspaceState` constants. Add `InfraState`, `SessionState` fields to `WorkspaceInstance`. Remove old state constants that no longer apply (`Available`). |
| `state.go` | Add migration logic for state file version 2 -> 3. Update `Load()` to run migration. |
| `local.go` | Remove `Start()`. Rename `Stop()` behavior to `KillSession()`. Update `Attach()` to always create session with layout. Update `Status()` for two-dimensional state. |
| `container.go` | Move `Start()`/`Stop()` to InfraManager implementation. Add `KillSession()`. Update `Stop()` to call `KillSession()` first. Update `Status()`. |
| `compose.go` | Same as container.go. |
| `ssh.go` | Remove `Start()`/`Stop()` (were ErrNotSupported). Add `KillSession()`. Update `Status()`. |
| `k8s_deploy.go` | Move `Start()`/`Stop()` to InfraManager implementation. Add `KillSession()`. Update `Stop()` to call `KillSession()` first. Update `Status()`. |
| `ws.go` (cmd) | Add `ws kill-session` command. Update `ws start`/`ws stop` to type-assert InfraManager. Update `ws list`/`ws status` for two-dimensional state display. |
