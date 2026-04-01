# Tasks: Single-Instance Architecture

**Input**: Design documents from `/specs/030-single-instance-arch/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/pipe-protocol.md

**Tests**: Unit and integration tests are included per constitution principles.

**Organization**: Tasks are grouped by user story. US1 (Responsive at Scale) and US2 (Consistent State) share implementation since both depend on the controller/sidebar split.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extract shared types to lib.rs, configure two-binary build with feature flags

- [x] T001 Update Cargo.toml with `controller` and `sidebar` feature flags and two `[[bin]]` targets in cc-zellij-plugin/Cargo.toml
- [x] T002 Create shared lib.rs with re-exported types (Session, Activity, HookPayload, PluginConfig, Notification) extracted from existing modules in cc-zellij-plugin/src/lib.rs
- [x] T003 [P] Define RenderPayload and RenderSession structs with serde Serialize/Deserialize in cc-zellij-plugin/src/lib.rs
- [x] T004 [P] Define ActionMessage, ActionType, SidebarHello, and SidebarInit structs in cc-zellij-plugin/src/lib.rs
- [x] T005 Update main.rs entry point with `#[cfg(feature = "controller")]` and `#[cfg(feature = "sidebar")]` gated ZellijPlugin registration in cc-zellij-plugin/src/main.rs
- [x] T006 Update Makefile with `build-wasm-controller` and `build-wasm-sidebar` targets, update `build-wasm` to build both in Makefile
- [x] T007 [P] Add protocol serialization roundtrip tests for all new types in cc-zellij-plugin/tests/protocol_tests.rs

**Checkpoint**: Both binaries compile (empty plugin stubs), `cargo test` passes, shared types serialize correctly

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Controller module skeleton with event subscription and state management

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T008 Create controller module directory and mod.rs with ControllerPlugin struct implementing ZellijPlugin trait (load, update, pipe stubs) in cc-zellij-plugin/src/controller/mod.rs
- [x] T009 Implement ControllerState with sessions BTreeMap, pane_manifest, pane_to_tab, tabs, sidebar_registry, render_dirty flag in cc-zellij-plugin/src/controller/state.rs
- [x] T010 [P] Create sidebar module directory and mod.rs with SidebarPlugin struct implementing ZellijPlugin trait (load, update, pipe, render stubs) in cc-zellij-plugin/src/sidebar/mod.rs
- [x] T011 [P] Implement SidebarState with cached_payload, mode, click_regions, my_tab_index, initialized flag in cc-zellij-plugin/src/sidebar/state.rs
- [x] T012 Implement controller load() subscribing to PaneUpdate, TabUpdate, Timer, RunCommandResult, PaneClosed, CommandPaneOpened and requesting permissions in cc-zellij-plugin/src/controller/mod.rs
- [x] T013 Implement sidebar load() subscribing to Mouse, Key only and requesting permissions in cc-zellij-plugin/src/sidebar/mod.rs

**Checkpoint**: Both plugins load in Zellij, subscribe to correct events, request permissions

---

## Phase 3: User Story 1+2 - Responsive Sidebar at Scale + Consistent State (Priority: P1) MVP

**Goal**: Controller processes all heavyweight events and broadcasts pre-computed render payloads. Sidebars display cached data. Session state is authoritative in the controller.

**Independent Test**: Open 15+ tabs, verify sidebar renders correctly in active tab, verify state changes (hook events, session transitions) appear in all tabs simultaneously.

### Implementation for US1+US2

- [x] T014 [US1] Implement controller event handlers for TabUpdate (tab tracking, active tab detection, keybinding registration via reconfigure()) and PaneUpdate (pane manifest, focus tracking, pane_to_tab map) in cc-zellij-plugin/src/controller/events.rs
- [x] T015 [US1] Implement controller Timer handler with render coalescing (100ms debounce), stale session cleanup, and git branch polling in cc-zellij-plugin/src/controller/events.rs
- [x] T016 [US1] Implement controller hook event processing (create/update sessions, activity transitions, CWD tracking, pending overrides) extracted from current pipe handler in cc-zellij-plugin/src/controller/hooks.rs
- [x] T017 [US2] Implement render_broadcast: build RenderPayload from ControllerState sessions, broadcast via pipe_message_to_plugin with cc-deck:render message name in cc-zellij-plugin/src/controller/render_broadcast.rs
- [x] T018 [US2] Implement controller persistence: single-writer save_sessions to /cache/sessions.json, restore_sessions on startup with PID-based stale detection in cc-zellij-plugin/src/controller/state.rs
- [x] T019 [US1] Implement sidebar pipe handler for cc-deck:render: deserialize RenderPayload, cache locally, trigger render if on active tab in cc-zellij-plugin/src/sidebar/mod.rs
- [x] T020 [US1] Implement sidebar render() producing ANSI output from cached RenderPayload (session list with indicators, colors, git branch, header with counts) adapted from current sidebar.rs in cc-zellij-plugin/src/sidebar/render.rs
- [x] T021 [US2] Implement sidebar "loading" state display when no render payload received, and "controller unavailable" timeout state when render payloads stop arriving in cc-zellij-plugin/src/sidebar/render.rs
- [x] T022 [P] [US1] Write controller state machine tests for event processing (TabUpdate, PaneUpdate, Timer, hook events) in cc-zellij-plugin/tests/controller_tests.rs
- [x] T023 [P] [US2] Write integration tests for controller-sidebar render payload flow (serialize, broadcast, deserialize, render) in cc-zellij-plugin/tests/integration_tests.rs

**Checkpoint**: Controller processes events and broadcasts render payloads. Sidebar displays session list from cached payload. Single source of truth, no sync needed.

---

## Phase 4: User Story 3 - Sidebar Interaction Without Cross-Tab Interference (Priority: P2)

**Goal**: All interactive modes (navigate, filter, rename, delete confirm, help) work locally in each sidebar without affecting other tabs.

**Independent Test**: Enter navigate mode in Tab 1, apply a filter, switch to Tab 2 (passive mode), return to Tab 1 and confirm filter is preserved.

### Implementation for US3

- [x] T024 [US3] Implement SidebarMode state machine (Passive, Navigate, NavigateFilter, NavigateDeleteConfirm, NavigateRename, RenamePassive, Help) with transitions extracted from current state.rs in cc-zellij-plugin/src/sidebar/modes.rs
- [x] T025 [US3] Implement sidebar Key event handler with mode-aware routing (navigate: j/k/Enter/Esc/d/r, filter: text input, rename: text input, help: any key dismiss) in cc-zellij-plugin/src/sidebar/input.rs
- [x] T026 [US3] Implement sidebar Mouse event handler with click region hit testing (single-click switch, double-click rename, right-click rename) in cc-zellij-plugin/src/sidebar/input.rs
- [x] T027 [US3] Implement inline rename editing with text input, cursor movement, and confirmation/cancel adapted from current rename.rs in cc-zellij-plugin/src/sidebar/rename.rs
- [x] T028 [US3] Implement sidebar action forwarding: on user confirmation (Enter to switch, 'd'+y to delete, rename complete, pause toggle), send ActionMessage via cc-deck:action pipe to controller in cc-zellij-plugin/src/sidebar/input.rs
- [x] T029 [US3] Implement controller action processing: receive ActionMessage, validate target session exists, execute action (switch_tab_to, focus_terminal_pane, rename, delete, pause), broadcast updated render payload in cc-zellij-plugin/src/controller/actions.rs
- [x] T030 [US3] Implement local session filtering in sidebar: filter cached RenderPayload sessions by text, rebuild click regions from filtered list, re-render in cc-zellij-plugin/src/sidebar/input.rs
- [x] T031 [US3] Implement sidebar-local notifications for mode transitions ("Navigate mode", "Filter: /text", rename feedback, delete confirmation) in cc-zellij-plugin/src/sidebar/render.rs
- [x] T032 [P] [US3] Write sidebar mode transition tests (all transitions, grace period, focus loss auto-exit) in cc-zellij-plugin/tests/sidebar_tests.rs

**Checkpoint**: All interactive modes work locally per sidebar. Actions forward to controller. Filter operates on cached data.

---

## Phase 5: User Story 4 - Automatic Sidebar Discovery (Priority: P2)

**Goal**: New sidebars auto-register with controller and receive tab index assignment. Tab closure cleans up registrations. Tab reindexing works after tab add/remove.

**Independent Test**: Open a new tab, verify sidebar displays sessions within 2 seconds. Close a tab, verify no errors or orphaned registrations.

### Implementation for US4

- [x] T033 [US4] Implement sidebar hello handshake: on first cc-deck:render receipt, send cc-deck:sidebar-hello with own plugin_id to controller in cc-zellij-plugin/src/sidebar/mod.rs
- [x] T034 [US4] Implement controller sidebar_registry: on cc-deck:sidebar-hello, cross-reference plugin_id with PaneManifest to determine tab_index, store in HashMap, respond with cc-deck:sidebar-init in cc-zellij-plugin/src/controller/sidebar_registry.rs
- [x] T035 [US4] Implement sidebar init handler: on cc-deck:sidebar-init, store my_tab_index and controller_plugin_id, enable self-filtering for render payloads in cc-zellij-plugin/src/sidebar/mod.rs
- [x] T036 [US4] Implement tab reindexing: on TabUpdate with changed tab count, controller broadcasts cc-deck:sidebar-reindex, sidebars clear my_tab_index and re-send hello in cc-zellij-plugin/src/controller/sidebar_registry.rs
- [x] T037 [US4] Implement dead sidebar cleanup: controller removes registry entries for plugin_ids that no longer appear in PaneManifest in cc-zellij-plugin/src/controller/sidebar_registry.rs
- [x] T038 [P] [US4] Write sidebar registration tests (hello/init flow, reindex after tab add/close, dead sidebar cleanup) in cc-zellij-plugin/tests/integration_tests.rs

**Checkpoint**: Sidebars auto-register, receive tab assignments, handle tab lifecycle changes cleanly.

---

## Phase 6: User Story 5 - Hook Routing to Single Processor (Priority: P3)

**Goal**: CLI hook command targets controller directly. Sidebars never process hook events.

**Independent Test**: Trigger a Claude Code hook event and verify it is processed exactly once by the controller.

### Implementation for US5

- [x] T039 [US5] Update Go CLI embed.go: replace single `go:embed cc_deck.wasm` with two directives for `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm`, update PluginInfo with ControllerBinary and SidebarBinary fields in cc-deck/internal/plugin/embed.go
- [x] T040 [US5] Update Go CLI install.go: write both WASM files to ~/.config/zellij/plugins/, handle migration from single binary (remove old cc_deck.wasm) in cc-deck/internal/plugin/install.go
- [x] T041 [US5] Update Go CLI remove.go: remove both controller and sidebar WASM files in cc-deck/internal/plugin/remove.go
- [x] T042 [US5] Update Go CLI layout.go: add `load_plugins` block for controller in generated config.kdl, update default_tab_template plugin location to sidebar binary in cc-deck/internal/plugin/layout.go
- [x] T043 [US5] Update Go CLI hook.go: add `--plugin "file:~/.config/zellij/plugins/cc_deck_controller.wasm"` flag to zellij pipe invocation for targeted controller delivery in cc-deck/internal/cmd/hook.go
- [x] T044 [US5] Update Go CLI status.go: report both controller and sidebar binary status in cc-deck/internal/plugin/status.go
- [x] T045 [P] [US5] Write Go CLI tests for two-binary install/remove, layout generation with load_plugins, and hook targeting in cc-deck/internal/plugin/install_test.go and cc-deck/internal/cmd/hook_test.go

**Checkpoint**: CLI installs both binaries, generates correct layouts, hooks target controller directly.

---

## Phase 7: Sync Elimination

**Purpose**: Remove the entire synchronization subsystem now that the controller is the single source of truth.

- [x] T046 Delete sync.rs module entirely in cc-zellij-plugin/src/sync.rs
- [x] T047 Remove sync-related fields from state (sync_dirty, pending_overrides, last_meta_content_hash) and all sync callsites from controller event handlers in cc-zellij-plugin/src/controller/
- [x] T048 [P] Remove session-meta.json read/write, shared last_click file operations, and prune_session_meta calls from controller in cc-zellij-plugin/src/controller/
- [x] T049 [P] Remove cc-deck:sync and cc-deck:request pipe message handlers from controller (these are replaced by cc-deck:render broadcasts) in cc-zellij-plugin/src/controller/mod.rs
- [x] T050 Remove merge_sessions, broadcast_and_save, flush_if_dirty, request_state functions and replace with simple single-writer save in cc-zellij-plugin/src/controller/state.rs
- [x] T051 Verify no sync-related code remains: grep for sync_dirty, session-meta, merge_sessions, broadcast_and_save across entire codebase

**Checkpoint**: Zero sync code remains. Controller uses single-writer persistence. No merge conflicts possible.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, final integration tests, and cleanup

- [x] T052 [P] Update README.md with new architecture description and add spec 030 to feature specifications table in README.md
- [x] T053 [P] Run `make test` to verify full test suite passes (Rust + Go)
- [x] T054 Run `make lint` to verify clippy + go vet pass with no warnings
- [x] T055 Run `make install` and verify both binaries install correctly, launch Zellij with cc-deck layout, verify sidebar works with multiple tabs
- [x] T056 Run quickstart.md validation: follow the quickstart steps and verify they work end-to-end

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion, BLOCKS all user stories
- **US1+US2 (Phase 3)**: Depends on Phase 2 completion
- **US3 (Phase 4)**: Depends on Phase 3 (needs render payload to display, needs action protocol)
- **US4 (Phase 5)**: Depends on Phase 3 (needs render broadcast flow for hello trigger)
- **US5 (Phase 6)**: Depends on Phase 1 (shared types for embed), can run in parallel with Phase 3-5
- **Sync Elimination (Phase 7)**: Depends on Phase 3 (controller persistence replaces sync)
- **Polish (Phase 8)**: Depends on all prior phases

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Phase 2. No dependencies on other stories.
- **US3 (P2)**: Depends on US1+US2 (needs render payload + action protocol). Cannot run in parallel.
- **US4 (P2)**: Depends on US1+US2 (needs render broadcast for hello trigger). Can run in parallel with US3.
- **US5 (P3)**: Only depends on Phase 1 shared types. Can run in parallel with US1-US4.

### Within Each User Story

- Event handlers before render broadcast (Phase 3)
- Render before interaction (Phase 3 before Phase 4)
- Core flow before discovery protocol (Phase 3 before Phase 5)
- Tests can run in parallel with each other within a phase

### Parallel Opportunities

- T003/T004 (shared type definitions) can run in parallel
- T010/T011 (sidebar module skeleton) can run in parallel with T008/T009 (controller skeleton)
- T022/T023 (tests) can run in parallel
- Phase 6 (Go CLI) can run in parallel with Phases 3-5 (Rust plugin)
- T046/T048/T049 (sync removal tasks) can run in parallel
- T052/T053 (polish tasks) can run in parallel

---

## Parallel Example: Phase 3 (US1+US2)

```bash
# After T016 (hooks) and T014 (events) complete:
# Launch render broadcast and sidebar receive in parallel:
Task: "T017 Implement render_broadcast in controller/render_broadcast.rs"
Task: "T019 Implement sidebar pipe handler for cc-deck:render in sidebar/mod.rs"

# After render flow works, launch tests in parallel:
Task: "T022 Controller state machine tests in tests/controller_tests.rs"
Task: "T023 Integration tests for render flow in tests/integration_tests.rs"
```

---

## Implementation Strategy

### MVP First (US1+US2 Only)

1. Complete Phase 1: Setup (shared types, build infrastructure)
2. Complete Phase 2: Foundational (module skeletons, event subscriptions)
3. Complete Phase 3: US1+US2 (controller events + render broadcast + sidebar display)
4. **STOP and VALIDATE**: Build both binaries, install, launch Zellij, verify sidebar renders
5. At this point, the core performance win is achieved

### Incremental Delivery

1. Setup + Foundational: Build infrastructure ready
2. US1+US2: Core split working, performance win achieved (MVP)
3. US3: Full interaction modes restored
4. US4: Auto-discovery for new tabs
5. US5: CLI updated for two-binary model
6. Sync Elimination: Dead code removed
7. Polish: Documentation, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US2 are combined because they share the same implementation (controller/sidebar split)
- Constitution requires `make install` for all builds, `make test` for all tests
- WASM filenames use underscores: `cc_deck_controller.wasm`, `cc_deck_sidebar.wasm`
- All Zellij host functions must be `#[cfg(target_family = "wasm")]` gated
