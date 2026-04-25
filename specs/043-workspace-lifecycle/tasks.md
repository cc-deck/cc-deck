# Tasks: Workspace Lifecycle Redesign

**Input**: Design documents from `/specs/043-workspace-lifecycle/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/workspace-interface.md

**Tests**: Unit tests for state migration and interface compliance are included (spec requires tests per constitution).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No new project setup needed. This is a refactoring of existing code. Phase reserved for any preliminary cleanup.

(No tasks in this phase)

---

## Phase 2: Foundational (Interface and State Model)

**Purpose**: Define the new interfaces and state model. ALL user stories depend on this.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T001 Update Workspace interface: remove `Start()` and `Stop()`, add `KillSession(ctx context.Context) error` in cc-deck/internal/ws/interface.go
- [X] T002 Define `InfraManager` interface with `Start(ctx context.Context) error` and `Stop(ctx context.Context) error` in cc-deck/internal/ws/interface.go
- [X] T003 Update `WorkspaceStatus` struct: replace single `State` with `InfraState *string` and `SessionState string` in cc-deck/internal/ws/interface.go
- [X] T004 Update state constants in cc-deck/internal/ws/types.go: add InfraState values (running/stopped/error), SessionState values (none/exists), remove obsolete constants (available/creating/unknown)
- [X] T005 Update `WorkspaceInstance` struct: replace `State WorkspaceState` with `InfraState *string` and `SessionState string` with YAML tags in cc-deck/internal/ws/types.go
- [X] T006 Add state file migration logic (version 2 -> 3) in `Load()` method of cc-deck/internal/ws/state.go per data-model.md migration rules
- [X] T007 [P] Add unit tests for state migration: all type/state combinations (local running, container stopped, ssh error, etc.) in cc-deck/internal/ws/state_test.go

**Checkpoint**: Interfaces defined, state model updated, migration tested. Code will not compile yet (implementations need updating).

---

## Phase 3: User Story 1 - Attach with correct layout (Priority: P1) MVP

**Goal**: Fix the original bug: `ws attach` on a local workspace always creates a session with the cc-deck layout.

**Independent Test**: Create a local workspace, run `ws attach`, verify the cc-deck sidebar plugin is visible.

### Implementation for User Story 1

- [X] T008 [US1] Update `LocalWorkspace` to remove `Start()` method and rename `Stop()` to `KillSession()` in cc-deck/internal/ws/local.go
- [X] T009 [US1] Update `LocalWorkspace.Attach()`: always create session with `--layout cc-deck` when no session exists, reattach when session exists, in cc-deck/internal/ws/local.go
- [X] T010 [US1] Update `LocalWorkspace.Status()` to return two-dimensional state (InfraState nil, SessionState none/exists) in cc-deck/internal/ws/local.go
- [X] T011 [US1] Update `ReconcileLocalWorkspaces()` to set SessionState based on zellij session detection in cc-deck/internal/ws/local.go
- [X] T012 [US1] Update `LocalWorkspace.Delete()` to call `KillSession()` instead of inline session kill in cc-deck/internal/ws/local.go

**Checkpoint**: Local workspaces attach with correct layout. `make test` and `make lint` pass.

---

## Phase 4: User Story 2 - Kill session without affecting infrastructure (Priority: P1)

**Goal**: Add `kill-session` command that kills only the Zellij session for any workspace type.

**Independent Test**: Attach to a container workspace, run `ws kill-session`, verify container still running, re-attach gets fresh session.

### Implementation for User Story 2

- [X] T013 [P] [US2] Add `KillSession()` to `ContainerWorkspace` using `podman exec ... zellij kill-session` in cc-deck/internal/ws/container.go
- [X] T014 [P] [US2] Add `KillSession()` to `ComposeWorkspace` using session container exec in cc-deck/internal/ws/compose.go
- [X] T015 [P] [US2] Add `KillSession()` to `SSHWorkspace` using SSH exec `zellij kill-session` in cc-deck/internal/ws/ssh.go
- [X] T016 [P] [US2] Add `KillSession()` to `K8sDeployWorkspace` using `kubectl exec ... zellij kill-session` in cc-deck/internal/ws/k8s_deploy.go
- [X] T017 [US2] Add `ws kill-session` cobra command in cc-deck/internal/cmd/ws.go: resolve workspace, call `KillSession()`

**Checkpoint**: `ws kill-session` works for all workspace types. Container/infra stays running.

---

## Phase 5: User Story 3 - Lazy attach starts infrastructure (Priority: P2)

**Goal**: `ws attach` on a stopped container workspace automatically starts infra, then creates session and attaches.

**Independent Test**: Stop a container workspace, run `ws attach`, verify container starts and session created with layout.

### Implementation for User Story 3

- [X] T018 [US3] Ensure `ContainerWorkspace.Attach()` lazy-starts via InfraManager type assertion and existing progress output in cc-deck/internal/ws/container.go
- [X] T019 [P] [US3] Ensure `ComposeWorkspace.Attach()` lazy-starts via same pattern in cc-deck/internal/ws/compose.go
- [X] T020 [P] [US3] Ensure `K8sDeployWorkspace.Attach()` lazy-starts (scale up if scaled to 0) in cc-deck/internal/ws/k8s_deploy.go
- [X] T021 [US3] Update all Attach() implementations to update SessionState to "exists" after session creation in cc-deck/internal/ws/{container,compose,ssh,k8s_deploy}.go

**Checkpoint**: `ws attach` works from any state for all InfraManager types. Single command to go from stopped to attached.

---

## Phase 6: User Story 4 - Infrastructure start/stop (Priority: P2)

**Goal**: `ws start`/`ws stop` manage infrastructure only. Warn on non-applicable types.

**Independent Test**: Run `ws stop` on container (kills session + stops container), `ws start` (starts container), `ws start` on local (shows warning).

### Implementation for User Story 4

- [X] T022 [P] [US4] Move `Start()` and `Stop()` on `ContainerWorkspace` to satisfy `InfraManager` interface, update `Stop()` to call `KillSession()` first in cc-deck/internal/ws/container.go
- [X] T023 [P] [US4] Move `Start()` and `Stop()` on `ComposeWorkspace` to satisfy `InfraManager`, update `Stop()` to call `KillSession()` first in cc-deck/internal/ws/compose.go
- [X] T024 [P] [US4] Move `Start()` and `Stop()` on `K8sDeployWorkspace` to satisfy `InfraManager`, update `Stop()` to call `KillSession()` first in cc-deck/internal/ws/k8s_deploy.go
- [X] T025 [US4] Remove `Start()` and `Stop()` from `SSHWorkspace` (were `ErrNotSupported`) in cc-deck/internal/ws/ssh.go
- [X] T026 [US4] Update `ws start` command: type-assert `InfraManager`, print warning if not implemented in cc-deck/internal/cmd/ws.go
- [X] T027 [US4] Update `ws stop` command: type-assert `InfraManager`, print warning if not implemented in cc-deck/internal/cmd/ws.go

**Checkpoint**: `ws start`/`ws stop` work correctly for InfraManager types, warn for others. `make test` and `make lint` pass.

---

## Phase 7: User Story 5 - Two-dimensional state display (Priority: P3)

**Goal**: `ws list` and `ws status` show clear, type-appropriate state.

**Independent Test**: Create workspaces of different types in various states, verify `ws list` output.

### Implementation for User Story 5

- [X] T028 [P] [US5] Update `ContainerWorkspace.Status()` to return InfraState + SessionState in cc-deck/internal/ws/container.go
- [X] T029 [P] [US5] Update `ComposeWorkspace.Status()` to return InfraState + SessionState in cc-deck/internal/ws/compose.go
- [X] T030 [P] [US5] Update `SSHWorkspace.Status()` to return InfraState nil + SessionState in cc-deck/internal/ws/ssh.go
- [X] T031 [P] [US5] Update `K8sDeployWorkspace.Status()` to return InfraState + SessionState in cc-deck/internal/ws/k8s_deploy.go
- [X] T032 [US5] Update `ws list` display logic: show type-appropriate state columns in cc-deck/internal/cmd/ws.go
- [X] T033 [US5] Update `ws status` display logic for two-dimensional state in cc-deck/internal/cmd/ws.go
- [X] T034 [P] [US5] Update all reconciliation functions to set both InfraState and SessionState in cc-deck/internal/ws/{container,compose,ssh,k8s_deploy}.go

**Checkpoint**: All user stories functional. `ws list` shows correct state for all workspace types.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, final verification.

- [X] T035 [P] Update README.md: workspace command reference, lifecycle description, add 043 to spec tracking table
- [X] T036 [P] Update CLI reference in docs/modules/reference/pages/cli.adoc: add `ws kill-session`, update `ws start`/`ws stop` descriptions
- [X] T037 Add workspace lifecycle state diagram to user guide documentation
- [X] T038 Update behavioral contract for Workspace interface in specs/043-workspace-lifecycle/contracts/workspace-interface.md and cross-reference from code
- [X] T039 [P] Add unit tests for InfraManager type assertion: verify container/compose/k8s implement it, local/ssh do not in cc-deck/internal/ws/interface_test.go
- [X] T040 Run `make test` and `make lint` for final verification

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 2 (Foundational)**: No dependencies, start immediately. BLOCKS all user stories.
- **Phase 3 (US1)**: Depends on Phase 2 completion
- **Phase 4 (US2)**: Depends on Phase 2 completion. Can run in parallel with US1.
- **Phase 5 (US3)**: Depends on Phase 2. Can run in parallel with US1/US2 but benefits from US4 being done first.
- **Phase 6 (US4)**: Depends on Phase 2. Can run in parallel with US1/US2.
- **Phase 7 (US5)**: Depends on Phase 2. Can run in parallel with other stories.
- **Phase 8 (Polish)**: Depends on all user stories being complete.

### User Story Dependencies

- **US1 (Attach layout)**: Independent. MVP story.
- **US2 (Kill session)**: Independent. Can be done in parallel with US1.
- **US3 (Lazy attach)**: Benefits from US4 (InfraManager move) but can be done independently.
- **US4 (Start/stop)**: Independent.
- **US5 (State display)**: Depends on state model changes from Phase 2, benefits from all other stories being complete.

### Within Each User Story

- Interface changes (Phase 2) before implementations
- KillSession before Stop (Stop calls KillSession)
- Status updates after state model changes
- CLI commands after workspace implementations

### Parallel Opportunities

- T013, T014, T015, T016 (KillSession for each type) can all run in parallel
- T022, T023, T024 (InfraManager for each type) can all run in parallel
- T028, T029, T030, T031 (Status updates) can all run in parallel
- T035, T036, T039 (docs and tests) can run in parallel
- US1 and US2 can be worked on simultaneously after Phase 2

---

## Parallel Example: User Story 2

```bash
# Launch all KillSession implementations in parallel (different files):
Task: "Add KillSession() to ContainerWorkspace in cc-deck/internal/ws/container.go"
Task: "Add KillSession() to ComposeWorkspace in cc-deck/internal/ws/compose.go"
Task: "Add KillSession() to SSHWorkspace in cc-deck/internal/ws/ssh.go"
Task: "Add KillSession() to K8sDeployWorkspace in cc-deck/internal/ws/k8s_deploy.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (interfaces + state model + migration)
2. Complete Phase 3: User Story 1 (local attach with layout fix)
3. **STOP and VALIDATE**: Test local workspace attach independently
4. This alone fixes the original bug that motivated the redesign

### Incremental Delivery

1. Phase 2: Foundation ready (code may not compile until at least one type is updated)
2. Phase 3: US1 (local attach fix) -> Test -> The original bug is fixed
3. Phase 4: US2 (kill-session) -> Test -> Users can reset sessions
4. Phase 5+6: US3+US4 (lazy attach + start/stop) -> Test -> Full lifecycle works
5. Phase 7: US5 (state display) -> Test -> Clean UX
6. Phase 8: Polish -> Documentation complete

### Recommended Approach

Since this is a refactoring that touches the core interface, the practical approach is:
1. Do Phase 2 (foundation) fully
2. Update ALL workspace types together (US1+US2+US4 combined) since they share the same interface change
3. Then US3 (lazy attach) and US5 (state display)
4. Finally documentation

This avoids a state where the code does not compile because some types implement the old interface while others implement the new one.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Commit after each task or logical group
- Use `make test` and `make lint` after each phase
- Use `make install` only when ready for manual testing (constitution Principle II)
- All documentation must use the prose plugin with cc-deck voice profile (constitution Principle XII)
