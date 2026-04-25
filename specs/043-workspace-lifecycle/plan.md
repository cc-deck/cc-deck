# Implementation Plan: Workspace Lifecycle Redesign

**Branch**: `043-workspace-lifecycle` | **Date**: 2026-04-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/043-workspace-lifecycle/spec.md`

## Summary

Redesign workspace lifecycle to separate infrastructure management from Zellij session management. Split the Workspace interface into a base interface (all types) and an InfraManager interface (container/compose/k8s only). Add `kill-session` command, make `attach` always lazy, and introduce a two-dimensional state model (infra_state + session_state).

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), adrg/xdg replacement via internal/xdg (XDG paths)
**Storage**: YAML state file at `~/.local/state/cc-deck/state.yaml`
**Testing**: Go stdlib testing + testify v1.11.1
**Target Platform**: Linux, macOS (CLI tool)
**Project Type**: CLI tool
**Constraints**: Must use `make install`, `make test`, `make lint` (constitution Principle VI)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | CLI-only change, no plugin changes |
| II. Plugin Installation | N/A | No plugin changes |
| III. WASM Filename | N/A | No WASM changes |
| IV. WASM Host Function Gating | N/A | No WASM changes |
| V. Zellij API Research Order | N/A | Using existing Zellij interactions |
| VI. Build via Makefile Only | PASS | Will use make test, make lint |
| VII. Interface Behavioral Contracts | PASS | New contract documented in contracts/workspace-interface.md |
| VIII. Simplicity | PASS | Two interfaces justified by type safety. No premature abstractions. |
| IX. Documentation Freshness | PENDING | README, CLI reference, user guide updates required |
| X. Spec Tracking in README | PENDING | Will add spec to README table |
| XI. Release Process | N/A | No release changes |
| XII. Prose Plugin | PENDING | Documentation must use prose plugin |
| XIII. XDG Paths | PASS | Using existing internal/xdg package |
| XIV. No Dotfile Nesting | N/A | No dotfile changes |

## Project Structure

### Documentation (this feature)

```text
specs/043-workspace-lifecycle/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/
│   └── workspace-interface.md  # Behavioral contract
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output (speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/ws/
├── interface.go         # Workspace + InfraManager interfaces
├── types.go             # State constants, WorkspaceInstance struct
├── state.go             # StateStore with migration logic
├── local.go             # LocalWorkspace (Workspace only)
├── container.go         # ContainerWorkspace (Workspace + InfraManager)
├── compose.go           # ComposeWorkspace (Workspace + InfraManager)
├── ssh.go               # SSHWorkspace (Workspace only)
├── k8s_deploy.go        # K8sDeployWorkspace (Workspace + InfraManager)
└── state_test.go        # Migration tests

cc-deck/internal/cmd/
└── ws.go                # CLI commands (add kill-session, update start/stop/list/status)
```

**Structure Decision**: No new files needed. All changes are modifications to existing files in `internal/ws/` and `internal/cmd/`.

## Complexity Tracking

No constitution violations to justify.

## Implementation Phases

### Phase 1: Interface and State Model (Foundation)

**Goal**: Define the new interfaces and state model. Everything else builds on this.

**Tasks**:

1. **Update interface.go**: Remove `Start()` and `Stop()` from `Workspace` interface. Add `KillSession(ctx context.Context) error`. Define new `InfraManager` interface with `Start()` and `Stop()`.

2. **Update types.go**: Replace single `State WorkspaceState` field in `WorkspaceInstance` with `InfraState *string` and `SessionState string`. Update state constants: keep `running`, `stopped`, `error` for InfraState. Define `none`, `exists` for SessionState. Remove `available`, `creating`, `unknown`.

3. **Update state.go**: Add migration logic in `Load()` for version 2 -> 3. Map old single `state` to new two-dimensional fields per rules in data-model.md. Bump state file version to 3.

4. **Add state migration tests**: Test migration from version 2 state files with all workspace types and state combinations.

**Verification**: `make test` passes with migration tests.

### Phase 2: Workspace Type Implementations

**Goal**: Update each workspace type to implement the new interfaces.

**Tasks**:

5. **Update local.go**: Remove `Start()` method. Rename current `Stop()` logic to `KillSession()`. Ensure `Attach()` always creates session with cc-deck layout when no session exists (fix the original bug). Update `Status()` to return two-dimensional state. Update reconciliation.

6. **Update container.go**: Keep `Start()`/`Stop()` as InfraManager implementation. Add `KillSession()` using `podman exec ... zellij kill-session`. Update `Stop()` to call `KillSession()` before stopping container. Ensure `Attach()` lazy-starts if stopped. Update `Status()` for two-dimensional state. Update reconciliation.

7. **Update compose.go**: Same pattern as container.go. Keep `Start()`/`Stop()` as InfraManager. Add `KillSession()`. Update `Stop()` to call `KillSession()` first. Update `Status()` and reconciliation.

8. **Update ssh.go**: Remove `Start()`/`Stop()` (were `ErrNotSupported`). Add `KillSession()` using SSH exec `zellij kill-session`. Update `Status()` for two-dimensional state. Update reconciliation.

9. **Update k8s_deploy.go**: Keep `Start()`/`Stop()` as InfraManager. Add `KillSession()` using `kubectl exec ... zellij kill-session`. Update `Stop()` to call `KillSession()` first. Update `Status()` and reconciliation.

**Verification**: `make test` passes. `make lint` passes (interface satisfaction checked by compiler).

### Phase 3: CLI Layer

**Goal**: Update CLI commands to use the new interfaces.

**Tasks**:

10. **Add `ws kill-session` command**: New cobra command in ws.go. Resolves workspace, calls `KillSession()`. Add to the lifecycle command group.

11. **Update `ws start` command**: Type-assert `InfraManager`. If not implemented, print warning with suggestion to use `ws attach`. If implemented, call `Start()`.

12. **Update `ws stop` command**: Type-assert `InfraManager`. If not implemented, print warning with suggestion to use `ws kill-session`. If implemented, call `Stop()`.

13. **Update `ws list` and `ws status`**: Display two-dimensional state. For non-InfraManager types, show only session state. For InfraManager types, show both infra_state and session_state.

**Verification**: Manual testing of all commands with local and container workspaces.

### Phase 4: Documentation

**Goal**: Update all documentation for the lifecycle changes.

**Tasks**:

14. **Update README.md**: Update workspace command reference. Add lifecycle state diagram. Update spec tracking table with 043 entry.

15. **Update CLI reference** (`docs/modules/reference/pages/cli.adoc`): Add `ws kill-session` command. Update `ws start`, `ws stop` descriptions. Document type-specific behavior.

16. **Update user guide**: Add workspace lifecycle state diagram showing the flow per workspace type. Update any existing workspace guides that reference the old start/stop semantics.

**Verification**: Documentation review via prose plugin.

### Phase 5: Testing and Verification

**Goal**: Ensure all acceptance scenarios pass.

**Tasks**:

17. **Unit tests for state migration**: Cover all migration paths (version 2 -> 3, all type/state combinations).

18. **Unit tests for interface compliance**: Verify InfraManager is only implemented by container, compose, k8s-deploy. Verify KillSession exists on all types.

19. **Integration testing**: Manual test with local workspace (attach creates session with layout, kill-session kills it, re-attach creates fresh). Manual test with container workspace (stop kills session then stops container, attach lazy-starts and creates session).

**Verification**: `make test` and `make lint` pass. Manual verification of acceptance scenarios from spec.
