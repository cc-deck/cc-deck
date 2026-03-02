# Tasks: cc-deck

**Input**: Design documents from `/specs/001-cc-deck/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in spec. Test tasks omitted.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and Rust/WASM build environment

- [ ] T001 (cc-mux-dz4.1) Initialize Rust project with `cargo init --lib` and configure Cargo.toml for wasm32-wasip1 target with zellij-tile and serde dependencies in Cargo.toml
- [ ] T002 (cc-mux-dz4.2) [P] Create Zellij development layout for hot-reload plugin testing in zellij-dev.kdl
- [ ] T003 (cc-mux-dz4.3) [P] Create production Zellij layout with cc-deck status bar pane in zellij-layout.kdl
- [ ] T004 (cc-mux-dz4.4) Verify build pipeline: `cargo build --target wasm32-wasip1` produces loadable WASM in target/wasm32-wasip1/debug/cc_deck.wasm

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core plugin infrastructure that MUST be complete before any user story

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 (cc-mux-mrs.1) Implement ZellijPlugin trait skeleton (load, update, render, pipe) with event subscriptions in src/main.rs
- [ ] T006 (cc-mux-mrs.2) [P] Define PluginConfig struct with defaults and KDL config parsing in src/config.rs
- [ ] T007 (cc-mux-mrs.3) [P] Define Session struct, SessionStatus enum, and state transition logic in src/session.rs
- [ ] T008 (cc-mux-mrs.4) [P] Define PluginState struct with session storage (BTreeMap), focused pane tracking, and mode state in src/state.rs
- [ ] T009 (cc-mux-mrs.5) Implement pipe message parser for `cc-deck::EVENT_TYPE::PANE_ID` format with validation and error handling in src/pipe_handler.rs
- [ ] T010 (cc-mux-mrs.6) Implement keybinding registration via `reconfigure` API using Ctrl+Shift modifiers in src/keybindings.rs

**Checkpoint**: Foundation ready. Plugin loads in Zellij, registers keybindings, and handles pipe messages.

---

## Phase 3: User Story 1 - Launch and Switch Between Sessions (Priority: P1) MVP

**Goal**: Create Claude Code sessions and switch between them via a fuzzy picker overlay

**Independent Test**: Launch cc-deck, create 2+ sessions with Ctrl+Shift+N, switch between them with Ctrl+Shift+T fuzzy picker

### Implementation for User Story 1

- [ ] T011 (cc-mux-oov.1) [US1] Implement new session creation: prompt for directory path, spawn `claude` via `open_command_pane` with context dict, track in PluginState in src/state.rs
- [ ] T012 (cc-mux-oov.2) [US1] Handle `CommandPaneOpened` and `CommandPaneExited` events to register/update session lifecycle in src/main.rs
- [ ] T013 (cc-mux-oov.3) [US1] Implement basic status bar rendering showing session names with focused highlight in src/status_bar.rs
- [ ] T014 (cc-mux-oov.4) [US1] Implement fuzzy picker UI: search input, filtered session list, keyboard navigation (arrows, Enter, Escape) in src/picker.rs
- [ ] T015 (cc-mux-oov.5) [US1] Implement fuzzy string matching algorithm for incremental search filtering in src/picker.rs
- [ ] T016 (cc-mux-oov.6) [US1] Wire Ctrl+Shift+T pipe message to toggle picker mode (show floating picker, intercept keys, switch focus on selection) in src/main.rs
- [ ] T017 (cc-mux-oov.7) [US1] Wire Ctrl+Shift+N pipe message to trigger new session creation flow in src/main.rs
- [ ] T018 (cc-mux-oov.8) [US1] Implement `focus_terminal_pane` call on session selection and Ctrl+Shift+1-9 direct switching in src/main.rs
- [ ] T019 (cc-mux-oov.9) [US1] Implement MRU ordering in picker: track last-focused timestamps, sort picker list by most recently used in src/state.rs

**Checkpoint**: Can launch cc-deck, create multiple Claude sessions, and switch between them via fuzzy search. MVP is functional.

---

## Phase 4: User Story 2 - Auto-Named Sessions (Priority: P1)

**Goal**: Sessions automatically named from git repository, with manual rename support

**Independent Test**: Create session in a git repo directory, verify name matches repo name. Rename via Ctrl+Shift+R.

### Implementation for User Story 2

- [ ] T020 (cc-mux-38q.1) [US2] Implement git repo detection via `run_command("git", ["rev-parse", "--show-toplevel"])` with async result handling in src/git.rs
- [ ] T021 (cc-mux-38q.2) [US2] Handle `RunCommandResult` event to extract repo name from stdout and set session display_name in src/main.rs
- [ ] T022 (cc-mux-38q.3) [US2] Implement directory basename fallback when git detection returns non-zero exit code in src/git.rs
- [ ] T023 (cc-mux-38q.4) [US2] Implement duplicate name detection and numeric suffix assignment (e.g., "api-server-2") in src/state.rs
- [ ] T024 (cc-mux-38q.5) [US2] Implement manual rename flow: Ctrl+Shift+R triggers text input overlay, updates session display_name and sets is_name_manual flag in src/main.rs
- [ ] T025 (cc-mux-38q.6) [US2] Update status bar and picker to display auto-detected or manual names in src/status_bar.rs and src/picker.rs

**Checkpoint**: Sessions auto-named from git repos. Manual rename works. Duplicate names get suffixes.

---

## Phase 5: User Story 3 - Activity Status Awareness (Priority: P2)

**Goal**: Show working/waiting/idle status for each session based on Claude Code hook events

**Independent Test**: Configure Claude Code hooks (per contracts/claude-hooks.md), start a session, verify status transitions in status bar

### Implementation for User Story 3

- [ ] T026 (cc-mux-32d.1) [US3] Extend pipe_handler to route `working`, `waiting`, `done` events to session status updates with pane ID correlation in src/pipe_handler.rs
- [ ] T027 (cc-mux-32d.2) [US3] Implement SessionStatus state machine transitions (Working->Done->Idle, any->Waiting, any->Exited) in src/session.rs
- [ ] T028 (cc-mux-32d.3) [US3] Implement timer-based idle detection using `set_timeout` and tracking time since last `done` event in src/main.rs
- [ ] T029 (cc-mux-32d.4) [US3] Implement fallback mode: when no pipe messages received for a session, use PaneUpdate title changes as basic activity signal in src/main.rs
- [ ] T030 (cc-mux-32d.5) [US3] Add status indicators (icons/symbols) to status bar rendering for each session state in src/status_bar.rs
- [ ] T031 (cc-mux-32d.6) [US3] Add status indicators to fuzzy picker session list entries in src/picker.rs

**Checkpoint**: Status bar shows working/waiting/idle/done per session. Fallback mode works without hooks.

---

## Phase 6: User Story 4 - Session Grouping by Project (Priority: P2)

**Goal**: Sessions grouped by project with distinct colors for visual identification

**Independent Test**: Create sessions in 2+ different repos, verify distinct color coding in status bar and picker

### Implementation for User Story 4

- [ ] T032 (cc-mux-443.1) [P] [US4] Implement ProjectGroup struct with color assignment from fixed palette in src/group.rs
- [ ] T033 (cc-mux-443.2) [US4] Implement automatic group creation/assignment when sessions are created, keyed by normalized repo/directory name in src/state.rs
- [ ] T034 (cc-mux-443.3) [US4] Update status bar rendering to apply group colors to session tab backgrounds in src/status_bar.rs
- [ ] T035 (cc-mux-443.4) [US4] Update fuzzy picker to display group color indicators alongside session entries in src/picker.rs

**Checkpoint**: Sessions visually grouped by project color in both status bar and picker.

---

## Phase 7: User Story 5 - Recent Sessions (Priority: P3)

**Goal**: Remember recently used directories for quick re-launch after restart

**Independent Test**: Create sessions, restart cc-deck, verify recent directories appear in new-session picker

### Implementation for User Story 5

- [ ] T036 (cc-mux-6hq.1) [P] [US5] Implement RecentEntries struct with LRU logic (add, evict, lookup) and serde serialization in src/recent.rs
- [ ] T037 (cc-mux-6hq.2) [US5] Implement persistence: save to `/cache/recent.json` on session create, load on plugin startup in src/recent.rs
- [ ] T038 (cc-mux-6hq.3) [US5] Integrate recent entries into new-session creation flow: show recent directories as suggestions before directory input in src/main.rs
- [ ] T039 (cc-mux-6hq.4) [US5] Handle corrupted/missing recent file gracefully (default to empty list, no crash) in src/recent.rs

**Checkpoint**: Recent directories persist across restarts. New session flow shows suggestions.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Error handling, edge cases, and refinement

- [ ] T040 (cc-mux-wfp.1) [P] Implement error display in status bar when `claude` binary not found during session creation in src/status_bar.rs
- [ ] T041 (cc-mux-wfp.2) [P] Handle rapid picker toggling (debounce Ctrl+Shift+T, prevent orphan floating panes) in src/main.rs
- [ ] T042 (cc-mux-wfp.3) [P] Handle terminal resize events for status bar truncation and picker dimension recalculation in src/status_bar.rs and src/picker.rs
- [ ] T043 (cc-mux-wfp.4) [P] Implement status bar overflow indicator when session count exceeds display width in src/status_bar.rs
- [ ] T044 (cc-mux-wfp.5) Implement Ctrl+Shift+X session close with confirmation prompt in src/main.rs
- [ ] T045 (cc-mux-wfp.6) Validate Zellij dev layout and hot-reload workflow end-to-end in zellij-dev.kdl
- [ ] T046 (cc-mux-wfp.7) Run quickstart.md validation: full setup-to-usage flow on clean environment

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup (Phase 1) completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational (Phase 2), this is the MVP
- **US2 (Phase 4)**: Depends on Phase 2. Can run in parallel with US1 but integrates session naming
- **US3 (Phase 5)**: Depends on Phase 2. Requires pipe_handler from Phase 2 and status bar from US1
- **US4 (Phase 6)**: Depends on Phase 2. Requires session model from US1 for group assignment
- **US5 (Phase 7)**: Depends on Phase 2. Requires new-session flow from US1
- **Polish (Phase 8)**: Depends on US1-US4 being complete

### User Story Dependencies

- **US1 (P1)**: After Phase 2. No dependencies on other stories. **This is the MVP.**
- **US2 (P1)**: After Phase 2. Enhances US1 session naming but is independently testable.
- **US3 (P2)**: After Phase 2. Enhances US1 status bar but is independently testable.
- **US4 (P2)**: After Phase 2. Enhances US1 visual display but is independently testable.
- **US5 (P3)**: After Phase 2. Enhances US1 new-session flow but is independently testable.

### Within Each User Story

- State/model changes before UI rendering
- Core logic before integration with main.rs
- Feature complete before moving to next story

### Parallel Opportunities

- T002 + T003 (dev and prod layouts)
- T006 + T007 + T008 (config, session, state structs)
- T032 + T036 (group model and recent model, different stories)
- All T040-T043 (polish tasks touch different files)
- US3 and US4 can proceed in parallel (different concerns)

---

## Parallel Example: User Story 1

```bash
# After Phase 2 foundational tasks complete:

# These can run in parallel (different files):
Task: "T013 [US1] Basic status bar rendering in src/status_bar.rs"
Task: "T014 [US1] Fuzzy picker UI in src/picker.rs"

# Then sequential (depends on above):
Task: "T016 [US1] Wire Ctrl+Shift+T to picker in src/main.rs"
Task: "T018 [US1] Focus switching on selection in src/main.rs"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T010)
3. Complete Phase 3: User Story 1 (T011-T019)
4. **STOP and VALIDATE**: Can create sessions and switch between them
5. This alone solves the core pain point

### Incremental Delivery

1. Setup + Foundational -> Plugin loads and registers keybindings
2. Add US1 -> Create and switch sessions (MVP!)
3. Add US2 -> Sessions auto-named from git repos
4. Add US3 -> Activity status indicators
5. Add US4 -> Color-coded project grouping
6. Add US5 -> Recent session memory
7. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

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
