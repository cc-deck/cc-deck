# Tasks: cc-deck Sidebar Plugin

**Input**: Design documents from `/specs/012-sidebar-plugin/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are included inline where they directly validate core logic (Rust unit tests via `cargo test`). No separate test phase.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Project structure and shared infrastructure for the plugin rebuild

- [x] T001 Update Cargo.toml with version 0.2.0 and correct dependencies in cc-zellij-plugin/Cargo.toml
- [x] T002 [P] Create Session struct with Activity enum and state transition logic in cc-zellij-plugin/src/session.rs
- [x] T003 [P] Create PluginState struct with session tracking (BTreeMap), tab/pane state, and mode enum in cc-zellij-plugin/src/state.rs
- [x] T004 [P] Create plugin configuration parsing (sidebar width, mode, keybindings) from KDL in cc-zellij-plugin/src/config.rs
- [x] T005 Implement ZellijPlugin trait (load, update, pipe, render) with event dispatch in cc-zellij-plugin/src/main.rs

**Checkpoint**: Plugin compiles and loads in Zellij (renders placeholder sidebar, requests permissions)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Pipe handling and state sync that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Implement pipe message parsing for cc-deck:hook with JSON payload extraction in cc-zellij-plugin/src/pipe_handler.rs
- [x] T007 Implement hook event to Activity state transition mapping in cc-zellij-plugin/src/pipe_handler.rs
- [x] T008 [P] Implement multi-instance state sync (cc-deck:sync broadcast, cc-deck:request, merge logic) in cc-zellij-plugin/src/sync.rs
- [x] T009 [P] Implement async git repo and branch detection via run_command() in cc-zellij-plugin/src/git.rs
- [x] T010 Wire pipe handler and sync into main.rs event dispatch (pipe method, RunCommandResult handling) in cc-zellij-plugin/src/main.rs

**Checkpoint**: Plugin receives hook events via pipe, transitions session state, syncs across instances

---

## Phase 3: User Story 1 - See Claude Activity at a Glance (Priority: P1) MVP

**Goal**: Sidebar renders a vertical list of Claude sessions with real-time activity indicators. Users can click to switch tabs. State is synchronized across all sidebar instances.

**Independent Test**: Install plugin in a layout, open multiple tabs, send hook pipe messages manually, verify sidebar shows sessions with correct indicators and click-to-switch works.

### Implementation for User Story 1

- [x] T011 [US1] Implement sidebar rendering: vertical session list with activity indicators, active tab highlighting, elapsed time, name truncation, overflow indicators in cc-zellij-plugin/src/sidebar.rs
- [x] T012 [US1] Implement empty state display (no sessions message with instructions) in cc-zellij-plugin/src/sidebar.rs
- [x] T013 [US1] Implement mouse click handling for click-to-switch (Mouse::LeftClick event, row-to-session mapping, switch_tab_to) in cc-zellij-plugin/src/sidebar.rs
- [x] T014 [US1] Implement pane-to-tab mapping from TabUpdate and PaneUpdate events for session-to-tab association in cc-zellij-plugin/src/state.rs
- [x] T015 [US1] Implement session auto-detection from hook events and session removal on pane close (PaneClosed/PaneUpdate) in cc-zellij-plugin/src/state.rs
- [x] T016 [US1] Implement session auto-naming from git detection results and directory basename fallback with duplicate suffix logic in cc-zellij-plugin/src/state.rs
- [x] T017 [US1] Wire sidebar rendering into main.rs render method, TabUpdate/PaneUpdate into update method in cc-zellij-plugin/src/main.rs

**Checkpoint**: Sidebar displays sessions with live activity indicators, click switches tabs, state syncs across tabs

---

## Phase 4: User Story 2 - Install and Configure cc-deck (Priority: P1)

**Goal**: Single `cc-deck install` command places WASM, layout, and hooks. Safe settings.json management with timestamped backup.

**Independent Test**: Run `cc-deck install` on clean system, verify all files placed, backup created, `zellij --layout cc-deck` shows sidebar.

### Implementation for User Story 2

- [x] T018 [P] [US2] Implement timestamped backup logic (create backup before modify, --skip-backup flag) in cc-deck/internal/plugin/backup.go
- [x] T019 [P] [US2] Implement settings.json hook management (read, add cc-deck hooks, remove cc-deck hooks, preserve other content) in cc-deck/internal/plugin/hooks.go
- [x] T020 [US2] Update install command to copy WASM, generate and install cc-deck layout (tab_template with sidebar), register hooks with backup in cc-deck/internal/plugin/install.go
- [x] T021 [US2] Generate cc-deck KDL layout with tab_template containing sidebar plugin pane and compact-bar in cc-deck/internal/plugin/layout.go
- [x] T022 [US2] Update plugin status command to report hook registration state in cc-deck/internal/cmd/plugin.go

**Checkpoint**: `cc-deck install` and `cc-deck plugin status` work end-to-end

---

## Phase 5: User Story 3 - Hook Integration (Priority: P1)

**Goal**: `cc-deck hook` Go command receives Claude Code hook JSON, forwards to Zellij plugin via pipe. Silent on all errors.

**Independent Test**: Echo sample hook JSON to `cc-deck hook` stdin with ZELLIJ_PANE_ID set, verify `zellij pipe` invoked. Test with Zellij not running, verify silent exit.

### Implementation for User Story 3

- [x] T023 [US3] Implement `cc-deck hook` subcommand: read stdin JSON, parse hook payload, read ZELLIJ_PANE_ID env, forward via `zellij pipe --name cc-deck:hook`, silent failure on all errors in cc-deck/internal/cmd/hook.go
- [x] T024 [US3] Register hook subcommand in root command in cc-deck/cmd/cc-deck/main.go

**Checkpoint**: Full hook flow works: Claude Code -> cc-deck hook -> zellij pipe -> sidebar plugin updates

---

## Phase 6: User Story 4 - Attend: Jump to Waiting Session (Priority: P2)

**Goal**: Single keystroke jumps to the oldest waiting session. Inline notification when no sessions waiting.

**Independent Test**: Set up two sessions (one working, one waiting via pipe), press attend key, verify focus switches to waiting tab.

### Implementation for User Story 4

- [x] T025 [P] [US4] Implement attend action: scan for Waiting sessions, find oldest, switch_tab_to() in cc-zellij-plugin/src/attend.rs
- [x] T026 [P] [US4] Implement inline notification display (brief message with auto-dismiss via timer) in cc-zellij-plugin/src/notification.rs
- [x] T027 [US4] Wire attend pipe message (cc-deck:attend) into pipe handler and notification rendering into sidebar in cc-zellij-plugin/src/main.rs
- [x] T028 [US4] Register attend keybinding via reconfigure() with configurable key in cc-zellij-plugin/src/config.rs

**Checkpoint**: Attend key finds and jumps to waiting sessions, shows notification when none waiting

---

## Phase 7: User Story 5 - Session Rename (Priority: P2)

**Goal**: Inline text input in sidebar for renaming sessions. Updates both sidebar and Zellij tab title.

**Independent Test**: Trigger rename on a session, type new name, press Enter, verify sidebar and tab title update.

### Implementation for User Story 5

- [x] T029 [US5] Implement inline rename: RenameState, key event handling (chars, Enter, Escape, Backspace), cursor management in cc-zellij-plugin/src/rename.rs
- [x] T030 [US5] Implement rename completion: update display_name, rename_tab(), duplicate name suffix logic, re-entrancy guard for TabUpdate in cc-zellij-plugin/src/rename.rs
- [x] T031 [US5] Wire rename pipe message (cc-deck:rename) and Key events during rename mode into main.rs dispatch in cc-zellij-plugin/src/main.rs
- [x] T032 [US5] Render rename input overlay in sidebar when rename is active in cc-zellij-plugin/src/sidebar.rs
- [x] T033 [US5] Register rename keybinding via reconfigure() with configurable key in cc-zellij-plugin/src/config.rs

**Checkpoint**: Sessions can be renamed inline, tab titles update, duplicates get suffixes

---

## Phase 8: User Story 6 - Session Creation (Priority: P2)

**Goal**: New Claude session from keybinding or sidebar click. Auto-named from git detection.

**Independent Test**: Trigger new session action, verify new tab opens with Claude running, sidebar shows new session with auto-detected name.

### Implementation for User Story 6

- [x] T034 [US6] Implement new session action: open_command_pane() with claude command, track pending session for auto-naming in cc-zellij-plugin/src/state.rs
- [x] T035 [US6] Handle CommandPaneOpened event to register new session and trigger git detection in cc-zellij-plugin/src/main.rs
- [x] T036 [US6] Add clickable [+] New button rendering in sidebar action bar with mouse hit region in cc-zellij-plugin/src/sidebar.rs
- [x] T037 [US6] Wire cc-deck:new pipe message and register new-session keybinding via reconfigure() in cc-zellij-plugin/src/config.rs

**Checkpoint**: New sessions can be created from keyboard or sidebar, auto-named, appear in session list

---

## Phase 9: User Story 7 - Uninstall cc-deck (Priority: P2)

**Goal**: Safe removal of cc-deck hooks, plugin, and layout files with backup.

**Independent Test**: Run `cc-deck uninstall` after install, verify hooks removed from settings.json, WASM and layout deleted, other settings preserved.

### Implementation for User Story 7

- [x] T038 [US7] Implement safe uninstall: backup settings.json, remove only cc-deck hooks, delete WASM and layout files, --skip-backup flag in cc-deck/internal/plugin/remove.go
- [x] T039 [US7] Handle "not installed" case gracefully (no-op with message) in cc-deck/internal/plugin/remove.go

**Checkpoint**: `cc-deck uninstall` safely removes all artifacts without data loss

---

## Phase 10: Polish and Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [x] T040 Verify full end-to-end flow: install (including re-install idempotency and config.kdl non-modification), start Zellij, create Claude sessions, see activity, attend, rename, create new, uninstall
- [x] T041 [P] Update Makefile dev target with cc-deck layout for development workflow in Makefile
- [x] T042 [P] Run quickstart.md validation: build, install, run, test hook, uninstall
- [x] T043 Run `cargo clippy -- -D warnings` and fix any linter issues in cc-zellij-plugin/

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001-T005 complete)
- **US1 (Phase 3)**: Depends on Phase 2 (pipe handler + sync working)
- **US2 (Phase 4)**: No plugin dependency, can start after Phase 1 (Go-only work)
- **US3 (Phase 5)**: No plugin dependency, can start after Phase 1 (Go-only work)
- **US4-US6 (Phases 6-8)**: Depend on Phase 3 (sidebar rendering working)
- **US7 (Phase 9)**: Depends on Phase 4 (install exists to be reversed)
- **Polish (Phase 10)**: Depends on all phases complete

### Parallel Opportunities

- **Phase 1**: T002, T003, T004 can run in parallel (separate files)
- **Phase 2**: T008, T009 can run in parallel (separate files)
- **Phase 4 + Phase 5**: US2 (install) and US3 (hook) are Go-only and can run in parallel with US1 (Rust sidebar)
- **Phase 6**: T025, T026 can run in parallel (attend.rs and notification.rs)
- **Phase 4**: T018, T019 can run in parallel (backup.go and hooks.go)

### Within Each User Story

- Models/structs before services/logic
- Core implementation before integration (wiring into main.rs)
- Keybinding registration last (depends on action being implemented)

---

## Parallel Example: Phases 3, 4, 5 Concurrently

```bash
# These three stories can be developed in parallel after Phase 2:

# Developer/Agent A: Rust sidebar (US1)
Task: "Implement sidebar rendering in cc-zellij-plugin/src/sidebar.rs"
Task: "Implement pane-to-tab mapping in cc-zellij-plugin/src/state.rs"

# Developer/Agent B: Go install (US2)
Task: "Implement backup logic in cc-deck/internal/plugin/backup.go"
Task: "Implement hooks management in cc-deck/internal/plugin/hooks.go"

# Developer/Agent C: Go hook command (US3)
Task: "Implement hook subcommand in cc-deck/internal/cmd/hook.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 + 3)

1. Complete Phase 1: Setup (plugin compiles)
2. Complete Phase 2: Foundational (pipe + sync working)
3. Complete Phases 3 + 4 + 5 in parallel: Sidebar + Install + Hook
4. **STOP and VALIDATE**: Full flow works end-to-end
5. This is the MVP: users can see Claude activity in the sidebar

### Incremental Delivery

1. MVP (US1 + US2 + US3) -> Core value delivered
2. Add US4 (Attend) -> Keyboard-driven workflow
3. Add US5 (Rename) -> Session customization
4. Add US6 (Create) -> Full session management
5. Add US7 (Uninstall) -> Complete lifecycle
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- The Rust plugin is rebuilt from scratch (only skeleton main.rs exists)
- Go CLI has existing plugin commands that are modified, not created from scratch

