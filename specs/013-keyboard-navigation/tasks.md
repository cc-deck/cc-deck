# Tasks: Keyboard Navigation & Global Shortcuts

**Input**: Design documents from `/specs/013-keyboard-navigation/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/pipe-protocol.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Foundational data model changes needed by all user stories

- [ ] T001 (cc-mux-0jp.1) [P] Add `WaitReason` enum and change `Activity::Waiting` to `Activity::Waiting(WaitReason)` in cc-zellij-plugin/src/session.rs
- [ ] T002 (cc-mux-0jp.2) [P] Add navigation state fields (`navigation_mode`, `cursor_index`, `filter_state`, `delete_confirm`) to PluginState in cc-zellij-plugin/src/state.rs
- [ ] T003 (cc-mux-0jp.3) [P] Add `navigate_key` and `attend_key` config fields to PluginConfig in cc-zellij-plugin/src/config.rs
- [ ] T004 (cc-mux-0jp.4) Update `hook_event_to_activity` mapping for `PermissionRequest` and `Notification` with WaitReason in cc-zellij-plugin/src/pipe_handler.rs
- [ ] T005 (cc-mux-0jp.5) Fix all compiler errors from `Activity::Waiting` → `Activity::Waiting(WaitReason)` change across all .rs files (session.rs, state.rs, attend.rs, sidebar.rs, main.rs)

**Checkpoint**: Project compiles with new data model. All existing tests pass.

---

## Phase 2: User Story 2 - Global Shortcut Registration (Priority: P1)

**Goal**: Register `Alt+s` and `Alt+a` keybindings at plugin load so they send pipe messages to the sidebar.

**Independent Test**: Start Zellij with cc-deck layout. Press `Alt+s` and verify a pipe message reaches the plugin (check debug.log for a PIPE entry with name "navigate"). Press `Alt+a` and verify attend pipe message arrives.

### Implementation for User Story 2

- [ ] T006 (cc-mux-xf6.1) [US2] Add `reconfigure()` call after `PermissionStatus::Granted` to register keybindings via KDL MessagePlugin syntax in cc-zellij-plugin/src/main.rs
- [ ] T007 (cc-mux-xf6.2) [US2] Add `cc-deck:navigate` to `PipeAction` enum and `parse_pipe_message` function in cc-zellij-plugin/src/pipe_handler.rs
- [ ] T008 (cc-mux-xf6.3) [US2] Handle `PipeAction::Navigate` in the `pipe()` method: toggle `navigation_mode`, call `set_selectable`/`focus_plugin_pane` in cc-zellij-plugin/src/main.rs
- [ ] T009 (cc-mux-xf6.4) [US2] Ensure only the active tab's sidebar instance responds to navigate/attend messages (check `self.active_tab_index` matches this instance's tab) in cc-zellij-plugin/src/main.rs

**Checkpoint**: Global shortcuts registered. `Alt+s` toggles `set_selectable`. Debug log shows pipe messages arriving.

---

## Phase 3: User Story 1 - Sidebar Keyboard Navigation (Priority: P1)

**Goal**: Cursor-based session list navigation with `j`/`k`/arrows, `Enter` to switch, `Esc` to exit.

**Independent Test**: Start Zellij, create 2+ Claude sessions, press `Alt+s`, use arrows to move cursor, press `Enter` to switch. Verify correct tab/pane gets focus.

### Implementation for User Story 1

- [ ] T010 (cc-mux-k1j.1) [US1] Add cursor rendering (`▶` prefix) to `render_sidebar` and `render_session_entry` in cc-zellij-plugin/src/sidebar.rs
- [ ] T011 (cc-mux-k1j.2) [US1] Add navigation mode key handler: `j`/`k`/`↑`/`↓` for cursor movement with wrapping in cc-zellij-plugin/src/main.rs
- [ ] T012 (cc-mux-k1j.3) [US1] Handle `Enter` key: switch to cursor session via `switch_tab_to` + `focus_terminal_pane`, exit navigation mode in cc-zellij-plugin/src/main.rs
- [ ] T013 (cc-mux-k1j.4) [US1] Handle `Esc` key: exit navigation mode, `set_selectable(false)`, `focus_terminal_pane` to return focus in cc-zellij-plugin/src/main.rs
- [ ] T014 (cc-mux-k1j.5) [US1] Handle `g`/`Home` and `G`/`End` keys for jump to first/last session in cc-zellij-plugin/src/main.rs
- [ ] T015 (cc-mux-k1j.6) [US1] Handle navigate shortcut as toggle: if already in navigation mode, exit it in cc-zellij-plugin/src/main.rs
- [ ] T016 (cc-mux-k1j.7) [US1] Preserve cursor position by pane_id when session list changes during navigation mode in cc-zellij-plugin/src/main.rs

**Checkpoint**: Full keyboard navigation works. Cursor visible, movement correct, Enter switches, Esc exits.

---

## Phase 4: User Story 3 - Smart Attend (Priority: P2)

**Goal**: Enhanced attend algorithm with tiered priority: Permission > Notification > idle (tab order). Skip current session.

**Independent Test**: Create sessions in different states. Press `Alt+a` repeatedly. Verify PermissionRequest sessions are focused first, then Notification, then idle (tab order, top-to-bottom).

### Implementation for User Story 3

- [ ] T017 (cc-mux-nd2.1) [US3] Implement tiered priority algorithm in `perform_attend` replacing `oldest_waiting_session` in cc-zellij-plugin/src/attend.rs
- [ ] T018 (cc-mux-nd2.2) [US3] Add skip-current-session logic to attend (skip focused pane_id unless it's the only session) in cc-zellij-plugin/src/attend.rs
- [ ] T019 (cc-mux-nd2.3) [US3] Update `AttendResult` to include "All sessions busy" variant in cc-zellij-plugin/src/attend.rs
- [ ] T020 (cc-mux-nd2.4) [P] [US3] Add unit tests for tiered attend priority: Permission > Notification > idle ordering in cc-zellij-plugin/src/attend.rs
- [ ] T021 (cc-mux-nd2.5) [US3] If attend is triggered during navigation mode, exit navigation mode after switching in cc-zellij-plugin/src/main.rs

**Checkpoint**: Smart attend cycles through sessions in correct priority order.

---

## Phase 5: User Story 4 - Contextual Actions (Rename, Delete) (Priority: P2)

**Goal**: Press `r` to rename cursor session, `d` to delete with `[y/N]` confirmation.

**Independent Test**: Enter navigation mode, move cursor to a session, press `r`, type new name, press Enter. Verify rename. Press `d`, press `y`, verify session closes.

### Implementation for User Story 4

- [ ] T022 (cc-mux-abj.1) [US4] Handle `r` key in navigation mode: start rename for cursor session (not active session) using existing rename flow in cc-zellij-plugin/src/main.rs
- [ ] T023 (cc-mux-abj.2) [US4] After rename completes or cancels, return to navigation mode (not passive mode) in cc-zellij-plugin/src/main.rs
- [ ] T024 (cc-mux-abj.3) [US4] Handle `d` key: set `delete_confirm = Some(cursor_pane_id)` and render confirmation in cc-zellij-plugin/src/main.rs
- [ ] T025 (cc-mux-abj.4) [US4] Render inline delete confirmation `Delete "name"? [y/N]` replacing session display lines in cc-zellij-plugin/src/sidebar.rs
- [ ] T026 (cc-mux-abj.5) [US4] Handle `y` key in delete confirmation: close command pane + tab if sole session, move cursor in cc-zellij-plugin/src/main.rs
- [ ] T027 (cc-mux-abj.6) [US4] Handle any other key in delete confirmation: cancel and return to navigation mode in cc-zellij-plugin/src/main.rs

**Checkpoint**: Rename and delete work from cursor position in navigation mode.

---

## Phase 6: User Story 5 - Search/Filter Sessions (Priority: P3)

**Goal**: Press `/` to enter search mode, type to filter sessions by name, Enter to confirm, Esc to clear.

**Independent Test**: Create 5+ sessions. Enter navigation mode, press `/`, type partial name. Verify list filters in real-time. Press Enter, verify cursor on first match.

### Implementation for User Story 5

- [ ] T028 (cc-mux-imx.1) [US5] Handle `/` key in navigation mode: set `filter_state = Some(FilterState::default())`, enter search sub-mode in cc-zellij-plugin/src/main.rs
- [ ] T029 (cc-mux-imx.2) [US5] Handle character input, backspace, arrows in filter mode (reuse rename key handling pattern) in cc-zellij-plugin/src/main.rs
- [ ] T030 (cc-mux-imx.3) [US5] Compute filtered session list from `filter_state.input_buffer` (case-insensitive substring match) in cc-zellij-plugin/src/sidebar.rs
- [ ] T031 (cc-mux-imx.4) [US5] Render search input at bottom of sidebar replacing [+] button row in cc-zellij-plugin/src/sidebar.rs
- [ ] T032 (cc-mux-imx.5) [US5] Handle Enter in search mode: apply filter, move cursor to first match, show "No matches" if empty in cc-zellij-plugin/src/main.rs
- [ ] T033 (cc-mux-imx.6) [US5] Handle Esc in search mode: clear filter, return to unfiltered navigation mode in cc-zellij-plugin/src/main.rs

**Checkpoint**: Search/filter works in navigation mode. Typing filters list, Enter confirms, Esc clears.

---

## Phase 7: User Story 6 - New Session from Navigation Mode (Priority: P3)

**Goal**: Press `n` in navigation mode to create a new session (same as [+] button).

**Independent Test**: Enter navigation mode, press `n`. Verify new tab with claude auto-started.

### Implementation for User Story 6

- [ ] T034 (cc-mux-ald.1) [US6] Handle `n` key in navigation mode: call existing `create_new_session_tab` + auto-start logic, exit navigation mode in cc-zellij-plugin/src/main.rs

**Checkpoint**: New session creation works from keyboard.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [ ] T035 (cc-mux-j74.1) Run `cargo clippy --target wasm32-wasip1 -- -D warnings` and fix any issues in cc-zellij-plugin/
- [ ] T036 (cc-mux-j74.2) Verify full e2e flow: install, navigate, attend, rename, delete, search, new session, exit
- [ ] T037 (cc-mux-j74.3) Run quickstart.md validation: build, install, test all keyboard shortcuts
- [ ] T038 (cc-mux-j74.4) Update existing tests for `Activity::Waiting(WaitReason)` change across all test functions in cc-zellij-plugin/src/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **US2 (Phase 2)**: Depends on Phase 1 (config fields + pipe action needed)
- **US1 (Phase 3)**: Depends on Phase 2 (global shortcut must work to enter navigation mode)
- **US3 (Phase 4)**: Depends on Phase 1 (WaitReason enum needed for priority tiers)
- **US4 (Phase 5)**: Depends on Phase 3 (navigation mode must work for cursor-based actions)
- **US5 (Phase 6)**: Depends on Phase 3 (navigation mode must work for search sub-mode)
- **US6 (Phase 7)**: Depends on Phase 3 (navigation mode must work for `n` key)
- **Polish (Phase 8)**: Depends on all phases

### User Story Dependencies

- **US2 (Global Shortcuts)**: Independent after Phase 1. Must complete before US1.
- **US1 (Navigation)**: Depends on US2 for activation mechanism.
- **US3 (Smart Attend)**: Independent after Phase 1. Can run in parallel with US1.
- **US4 (Rename/Delete)**: Depends on US1 (needs navigation mode).
- **US5 (Search)**: Depends on US1 (needs navigation mode).
- **US6 (New Session)**: Depends on US1 (needs navigation mode).

### Within Each User Story

- Models/enums before services/logic
- Core implementation before integration (wiring into main.rs)
- Rendering changes in sidebar.rs after state changes

### Parallel Opportunities

- T001, T002, T003 can run in parallel (different files)
- US3 (Smart Attend) can run in parallel with US1 (Navigation) after Phase 1
- T020 (attend tests) can run in parallel with T017-T019

---

## Implementation Strategy

### MVP First (US2 + US1)

1. Complete Phase 1: Setup (data model changes)
2. Complete Phase 2: US2 (global shortcuts registered)
3. Complete Phase 3: US1 (navigation mode works)
4. **STOP and VALIDATE**: Navigate between sessions with keyboard only
5. This is the MVP: keyboard-driven session switching

### Incremental Delivery

1. Setup + US2 + US1 → Core keyboard navigation (MVP)
2. Add US3 → Smart attend with priorities
3. Add US4 → Rename/delete from keyboard
4. Add US5 → Search/filter sessions
5. Add US6 → New session from keyboard
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- The `Activity::Waiting(WaitReason)` change (T001) is a breaking change that requires T005 to fix all callers


<!-- SDD-TRAIT:beads -->
## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
