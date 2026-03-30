# Tasks: Session TUI (Control Plane Dashboard)

**Input**: Design documents from `/specs/031-session-tui/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Integration tests are MANDATORY per Constitution Principle IX. Unit tests for core logic, integration tests for TUI lifecycle operations.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. Only P1 user stories are included (P2/P3 ship in separate branches per spec).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go CLI**: `cc-deck/internal/` (existing project structure)
- **New TUI package**: `cc-deck/internal/tui/`
- **New command**: `cc-deck/internal/cmd/tui.go`
- **Tests**: `cc-deck/internal/tui/*_test.go`

---

## Phase 1: Setup

**Purpose**: Add bubbletea dependencies and create TUI package structure

- [ ] T001 Add bubbletea, lipgloss, and bubbles dependencies to cc-deck/go.mod
- [ ] T002 Create TUI package directory structure at cc-deck/internal/tui/
- [ ] T003 Add `tui` cobra subcommand in cc-deck/internal/cmd/tui.go and register it in the root command

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core TUI infrastructure that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Implement root bubbletea model with view routing (viewList, viewCreate, viewHelp) and Init/Update/View methods in cc-deck/internal/tui/model.go
- [ ] T005 [P] Define key bindings using bubbletea/key for all P1 views (global, list, create, help) in cc-deck/internal/tui/keys.go
- [ ] T006 [P] Define lipgloss styles for header, table rows, footer, status indicators, and confirmation dialogs in cc-deck/internal/tui/styles.go
- [ ] T007 [P] Implement envRow builder that merges FileStateStore records, instances, and definitions into a flat display model in cc-deck/internal/tui/envrow.go
- [ ] T008 [P] Implement plugin session data reader that parses sessions.json from the Zellij WASI cache path on the host filesystem, handling the Rust serde enum format for Activity in cc-deck/internal/tui/session.go
- [ ] T009 Implement status polling using tea.Tick that runs ReconcileLocalEnvs, ReconcileContainerEnvs, ReconcileComposeEnvs and rebuilds envRow list in cc-deck/internal/tui/polling.go
- [ ] T010 [P] Unit tests for envRow builder (merging records + instances + definitions) in cc-deck/internal/tui/envrow_test.go
- [ ] T011 [P] Unit tests for session.go (parsing sessions.json with all Activity variants) in cc-deck/internal/tui/session_test.go

**Checkpoint**: Foundation ready. TUI launches, displays empty view, accepts key input, polls status.

---

## Phase 3: User Story 1 - View All Environments at a Glance (Priority: P1)

**Goal**: Display a live, auto-refreshing list of all registered environments with name, type, status, session health summary, storage, last-attached time, and tags.

**Independent Test**: Launch the TUI with at least two environments (one local, one container). Verify the list displays all columns. Verify auto-refresh updates when status changes externally.

### Implementation for User Story 1

- [ ] T012 [US1] Implement list view table rendering with columns (name, type, status, sessions, storage, last attached, tags) and status indicators in cc-deck/internal/tui/list.go
- [ ] T013 [US1] Implement aggregate header showing environment counts by state (N running, N stopped, N creating) in cc-deck/internal/tui/list.go
- [ ] T014 [US1] Implement context-sensitive footer with key hints that change based on current view in cc-deck/internal/tui/list.go
- [ ] T015 [US1] Implement keyboard navigation (j/k/Up/Down, g/G for top/bottom) and cursor selection in cc-deck/internal/tui/list.go
- [ ] T016 [US1] Implement terminal resize handling (tea.WindowSizeMsg) that reflows the layout in cc-deck/internal/tui/model.go
- [ ] T017 [US1] Implement empty state message with guidance when no environments exist in cc-deck/internal/tui/list.go
- [ ] T018 [US1] Wire polling results into the list view so it auto-refreshes on tick in cc-deck/internal/tui/model.go
- [ ] T019 [US1] Integration test: verify TUI model renders environment list from a test state file in cc-deck/internal/tui/list_test.go

**Checkpoint**: User Story 1 complete. TUI shows live environment list with auto-refresh and keyboard navigation.

---

## Phase 4: User Story 2 - Attach to an Environment (Priority: P1)

**Goal**: Select an environment and press Enter to suspend the TUI, hand the terminal to Zellij (or podman exec), and resume when the user exits.

**Independent Test**: From the TUI, select a running local environment, press Enter. Verify Zellij opens. Exit Zellij. Verify TUI resumes.

### Implementation for User Story 2

- [ ] T020 [US2] Implement attach action using tea.ExecProcess for local environments (spawns `zellij attach cc-deck-<name>`) in cc-deck/internal/tui/model.go
- [ ] T021 [US2] Implement attach action for container environments (spawns `podman exec -it <container> zellij attach`) in cc-deck/internal/tui/model.go
- [ ] T022 [US2] Handle stopped environment selection: display message that environment must be started first (or offer to start it) in cc-deck/internal/tui/model.go
- [ ] T023 [US2] Refresh environment list on resume after attach (tea.Resume handler) in cc-deck/internal/tui/model.go
- [ ] T024 [US2] Unit test: verify attach command construction for local and container types in cc-deck/internal/tui/model_test.go

**Checkpoint**: User Story 2 complete. Users can attach to any running environment and resume the TUI after exiting.

---

## Phase 5: User Story 3 - Create a New Environment from the TUI (Priority: P1)

**Goal**: Press `n` to open a creation wizard. Fill in type-specific fields, confirm, create the environment, and optionally auto-attach.

**Independent Test**: From the TUI, create a local environment by providing just a name. Verify it appears in the list as "running."

### Implementation for User Story 3

- [ ] T025 [US3] Implement create wizard model with form fields: name (text input), type selector (local/container), and type-specific fields in cc-deck/internal/tui/create.go
- [ ] T026 [US3] Implement type selector (radio buttons: local, container) that dynamically shows/hides type-specific fields in cc-deck/internal/tui/create.go
- [ ] T027 [US3] Implement container-specific fields (image, storage type, source path) using bubbles text input components in cc-deck/internal/tui/create.go
- [ ] T028 [US3] Wire create wizard submission to env.NewEnvironment() + Create() using existing internal/env package in cc-deck/internal/tui/create.go
- [ ] T029 [US3] Implement auto-attach after successful creation (default behavior, suspend/resume) in cc-deck/internal/tui/create.go
- [ ] T030 [US3] Implement error display in the wizard when creation fails (invalid name, container pull error) in cc-deck/internal/tui/create.go
- [ ] T031 [US3] Unit test: verify wizard form field navigation and submission in cc-deck/internal/tui/create_test.go

**Checkpoint**: User Story 3 complete. Users can create local and container environments from the TUI.

---

## Phase 6: User Story 4 - Manage Environment Lifecycle (Priority: P1)

**Goal**: Start stopped environments, stop running environments, and delete environments with name confirmation.

**Independent Test**: Stop a running environment from the TUI. Verify status changes. Start it again. Delete with name confirmation.

### Implementation for User Story 4

- [ ] T032 [US4] Implement start action (key `S`) that calls env.Start() on the selected stopped environment in cc-deck/internal/tui/model.go
- [ ] T033 [US4] Implement stop action (key `X`) that calls env.Stop() on the selected running environment in cc-deck/internal/tui/model.go
- [ ] T034 [US4] Implement confirmation dialog model with name-typing requirement for destructive operations in cc-deck/internal/tui/confirm.go
- [ ] T035 [US4] Implement delete action (key `d`) that shows confirmation dialog, validates name match, then calls env.Delete() in cc-deck/internal/tui/model.go
- [ ] T036 [US4] Handle operation errors (display inline error message, refresh state) in cc-deck/internal/tui/model.go
- [ ] T037 [US4] Run operations in goroutines to keep UI responsive (non-blocking per spec clarification) in cc-deck/internal/tui/model.go
- [ ] T038 [US4] Unit test: verify confirmation dialog accepts/rejects name input in cc-deck/internal/tui/confirm_test.go

**Checkpoint**: User Story 4 complete. Full lifecycle management from the TUI.

---

## Phase 7: Help Overlay (Priority: P1)

**Goal**: Display a categorized key binding reference overlay accessible from any view.

**Independent Test**: Press `?` from any view. Verify the help overlay appears with all key bindings. Press Esc to dismiss.

- [ ] T039 Implement help overlay model with categorized key binding display (Navigation, Lifecycle, Display, Global) in cc-deck/internal/tui/help.go
- [ ] T040 Wire help overlay toggle (key `?` / `F1`) in the root model so it works from any view in cc-deck/internal/tui/model.go

**Checkpoint**: Help overlay complete. All P1 user stories and help are functional.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, final testing, code quality

- [ ] T041 [P] Update README.md with TUI feature description and usage examples
- [ ] T042 [P] Add `cc-deck tui` to CLI reference in docs/modules/reference/pages/cli.adoc
- [ ] T043 [P] Update spec tracking table in README.md with 031-session-tui entry
- [ ] T044 Run `make lint` and fix any linting issues across all new files
- [ ] T045 Run `make test` and verify all tests pass
- [ ] T046 Manual integration test: launch TUI, verify list view, attach to a local env, resume, create a new env, start/stop/delete, help overlay

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2. First user story, MVP foundation.
- **US2 (Phase 4)**: Depends on Phase 3 (needs list view for selection).
- **US3 (Phase 5)**: Depends on Phase 2. Can run in parallel with US2.
- **US4 (Phase 6)**: Depends on Phase 3 (needs list view for selection).
- **Help (Phase 7)**: Depends on Phase 2. Can run in parallel with US1-US4.
- **Polish (Phase 8)**: Depends on all previous phases.

### User Story Dependencies

```
Phase 2 (Foundation)
  ├── US1 (list view) ──> US2 (attach, needs list selection)
  │                   ──> US4 (lifecycle, needs list selection)
  ├── US3 (create wizard, independent of list view)
  └── Help (overlay, independent)
```

### Parallel Opportunities

Within Phase 2:
- T005, T006, T007, T008 can all run in parallel (different files)
- T010, T011 can run in parallel (test files)

US3 and US2 can be developed in parallel after Phase 2.

Help overlay (Phase 7) can be developed in parallel with any user story.

---

## Parallel Example: Phase 2 Foundation

```bash
# Launch parallel foundational tasks:
Task: "Define key bindings in cc-deck/internal/tui/keys.go"
Task: "Define lipgloss styles in cc-deck/internal/tui/styles.go"
Task: "Implement envRow builder in cc-deck/internal/tui/envrow.go"
Task: "Implement session data reader in cc-deck/internal/tui/session.go"
```

## Parallel Example: After Phase 2

```bash
# Launch US1 and Help in parallel:
Task: "Implement list view in cc-deck/internal/tui/list.go"        # US1
Task: "Implement help overlay in cc-deck/internal/tui/help.go"      # Help

# After US1 completes, launch US2 and US4 in parallel:
Task: "Implement attach action in cc-deck/internal/tui/model.go"    # US2
Task: "Implement confirmation dialog in cc-deck/internal/tui/confirm.go"  # US4
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundation (T004-T011)
3. Complete Phase 3: US1 - Environment List (T012-T019)
4. **STOP and VALIDATE**: Launch TUI, verify list displays, auto-refreshes
5. Can demo/ship as read-only dashboard

### Incremental Delivery

1. Setup + Foundation -> TUI launches with empty view
2. Add US1 (list view) -> Read-only dashboard (MVP!)
3. Add US2 (attach) -> Users can navigate to environments
4. Add US3 (create wizard) -> Users can create environments from TUI
5. Add US4 (lifecycle) -> Full control plane
6. Add Help -> Complete P1 experience
7. Polish -> Documentation, tests, cleanup

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Only P1 user stories are included. P2 (detail view, search, data transfer, tags) and P3 (notifications, status reports) ship in separate branches per spec clarification.
- Build via Makefile only (Constitution Principle VI)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
