# Tasks: Plugin Bugfixes

**Input**: Design documents from `/specs/010-plugin-bugfixes/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Not explicitly requested. Test tasks omitted (US5 covers automated integration tests as a user story).

**Organization**: Tasks grouped by user story. US1 and US2 are both P1 but independent.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add new fields to existing structs needed by all user stories

- [ ] T001 (cc-mux-n4t.1) Add `tab_index: Option<usize>` field to Session struct in cc-zellij-plugin/src/session.rs
- [ ] T002 (cc-mux-n4t.2) Add `tab_pane_mapping: HashMap<usize, Vec<u32>>` and `pending_auto_start: Option<(u32, PathBuf)>` fields to PluginState struct in cc-zellij-plugin/src/state.rs
- [ ] T003 (cc-mux-n4t.3) Add `tab_title()` method to Session that returns `format!("{} {}", self.status.indicator(), self.display_name)` in cc-zellij-plugin/src/session.rs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Tab-to-session mapping infrastructure used by all user stories

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 (cc-mux-053.1) Implement TabUpdate handler in the `update()` method: extract `Vec<TabInfo>`, rebuild tab-to-session mapping by matching session pane IDs against `PaneManifest` data, update `session.tab_index` for each tracked session in cc-zellij-plugin/src/main.rs
- [ ] T005 (cc-mux-053.2) Implement PaneUpdate handler extension: rebuild `tab_pane_mapping` from `PaneManifest.panes` HashMap (tab index -> pane IDs) on every PaneUpdate event in cc-zellij-plugin/src/main.rs
- [ ] T006 (cc-mux-053.3) Add helper method `update_tab_title(&self, session: &Session)` to PluginState that calls `rename_tab(tab_index as u32, &session.tab_title())` when `tab_index` is Some, in cc-zellij-plugin/src/state.rs

**Checkpoint**: Tab mapping infrastructure ready, `update_tab_title` callable from any event handler

---

## Phase 3: User Story 1 - Dynamic Tab Titles (Priority: P1) MVP

**Goal**: Tab titles in the Zellij tab bar show status icon + project name, updated on every status change.

**Independent Test**: Create two sessions via `zellij pipe --name new_session`, simulate status changes via `zellij pipe --name "cc-deck::working::PANE_ID"`, verify tab bar titles update.

### Implementation

- [ ] T007 (cc-mux-08g.1) [US1] Call `update_tab_title()` after every status change in the pipe message handler (working/waiting/done transitions) in cc-zellij-plugin/src/main.rs
- [ ] T008 (cc-mux-08g.2) [US1] Call `update_tab_title()` after git detection completes (RunCommandResult handler) when display_name changes in cc-zellij-plugin/src/main.rs
- [ ] T009 (cc-mux-08g.3) [US1] Call `update_tab_title()` after manual rename confirmation in the Key event handler in cc-zellij-plugin/src/main.rs
- [ ] T010 (cc-mux-08g.4) [US1] Call `update_tab_title()` after idle timeout transition in the Timer event handler in cc-zellij-plugin/src/main.rs
- [ ] T011 (cc-mux-08g.5) [US1] Call `update_tab_title()` when session is first registered (register_session) to set initial tab title in cc-zellij-plugin/src/state.rs

**Checkpoint**: Tab titles show `⚡ project-name`, `⏳ project-name`, `✓ project-name`, `💤 project-name` in real time

---

## Phase 4: User Story 2 - Automatic Session Detection (Priority: P1)

**Goal**: Plugin detects Claude sessions started manually by scanning pane titles for "claude" substring.

**Independent Test**: Start `claude` manually in a Zellij pane, verify the status bar shows the session within 5 seconds.

### Implementation

- [ ] T012 (cc-mux-vt3.1) [US2] Add `detect_claude_sessions(&mut self, manifest: &PaneManifest)` method to PluginState that iterates all panes, checks `title.to_lowercase().contains("claude")` for non-plugin panes, and registers untracked panes as new sessions in cc-zellij-plugin/src/state.rs
- [ ] T013 (cc-mux-vt3.2) [US2] Handle duplicate prevention in `detect_claude_sessions`: skip panes whose `id` is already in `self.sessions` (matched by pane_id) in cc-zellij-plugin/src/state.rs
- [ ] T014 (cc-mux-vt3.3) [US2] Call `detect_claude_sessions()` from the PaneUpdate handler in `update()` after rebuilding tab_pane_mapping in cc-zellij-plugin/src/main.rs
- [ ] T015 (cc-mux-vt3.4) [US2] Trigger git repo detection for auto-detected sessions by calling `run_command` with `git rev-parse --show-toplevel` using the pane's cwd in cc-zellij-plugin/src/state.rs
- [ ] T016 (cc-mux-vt3.5) [US2] Call `update_tab_title()` for auto-detected sessions after registration and after git detection completes in cc-zellij-plugin/src/main.rs

**Checkpoint**: Manually started `claude` sessions appear in the status bar with project name and respond to status hooks

---

## Phase 5: User Story 3 - Auto-Start Claude (Priority: P2)

**Goal**: When creating a session via the plugin, Claude launches automatically in the new tab.

**Independent Test**: Run `zellij pipe --name new_session`, verify Claude starts automatically (or error message appears if claude not on PATH).

### Implementation

- [ ] T017 (cc-mux-3ay.1) [US3] Modify `new_session` pipe handler to set `pending_auto_start = Some((session_id, cwd))` after calling `new_tab()` in cc-zellij-plugin/src/main.rs
- [ ] T018 (cc-mux-3ay.2) [US3] Add auto-start detection in PaneUpdate handler: when `pending_auto_start` is set and a new terminal pane appears in the expected tab, call `focus_terminal_pane(pane_id)` then `open_command_pane_in_place(CommandToRun { path: "claude".into(), args: vec![], cwd: Some(cwd) }, context)` and clear `pending_auto_start` in cc-zellij-plugin/src/main.rs
- [ ] T019 (cc-mux-3ay.3) [US3] Handle Claude not found: in `CommandPaneExited` handler, if the exited pane matches a session with no prior activity and exit code is non-zero, set `error_message` to "Claude not found. Install: npm install -g @anthropic-ai/claude-code" with `error_clear_counter = 10` in cc-zellij-plugin/src/main.rs

**Checkpoint**: `zellij pipe --name new_session` creates a tab with Claude running. If claude is not on PATH, error message shows for 10 seconds.

---

## Phase 6: User Story 4 - Floating Picker (Priority: P2)

**Goal**: Session picker renders as a floating overlay instead of trying to render in the 1-row status bar.

**Independent Test**: With multiple sessions, run `zellij pipe --name open_picker`, verify a floating pane appears with session list, type to filter, Enter to select.

### Implementation

- [ ] T020 (cc-mux-d9s.1) [US4] Modify `open_picker` pipe handler to use `open_command_pane_floating` instead of `focus_plugin_pane`: spawn a floating command pane running a helper that displays the picker UI and sends the selection back via `zellij pipe` in cc-zellij-plugin/src/main.rs
- [ ] T021 (cc-mux-d9s.2) [US4] Create a picker helper script at cc-zellij-plugin/picker-helper.sh that receives session list as JSON argument, displays an interactive fuzzy picker using shell select/fzf, and sends the selected session ID back via `zellij pipe --name "pick_session" -- PANE_ID`
- [ ] T022 (cc-mux-d9s.3) [US4] Add `pick_session` pipe handler that receives the selected pane ID from the helper, focuses that pane, and updates MRU in cc-zellij-plugin/src/main.rs
- [ ] T023 (cc-mux-d9s.4) [US4] Handle external picker dismissal: if the floating picker pane closes without sending a `pick_session` message, detect via `PaneClosed` event and reset picker_active state in cc-zellij-plugin/src/main.rs
- [ ] T024 (cc-mux-d9s.5) [US4] Embed the picker helper script in the WASM binary or install it alongside the plugin via the cc-deck CLI `plugin install` command in cc-deck/internal/plugin/install.go

**Checkpoint**: `open_picker` shows a floating picker with fuzzy filtering, selection switches focus, Escape/close dismisses cleanly

---

## Phase 7: User Story 5 - Automated Test Suite (Priority: P3)

**Goal**: Fully automated test script that exercises all plugin features without manual interaction.

**Independent Test**: Run `./smoke_test.sh`, verify it completes with a pass/fail report in under 60 seconds.

### Implementation

- [ ] T025 (cc-mux-3hz.1) [US5] Rewrite smoke_test.sh as fully automated: launch Zellij with test session name, send pipe commands, wait for state changes, verify via `zellij action` queries, report pass/fail in smoke_test.sh
- [ ] T026 (cc-mux-3hz.2) [US5] Add test case: session creation (verify tab count increases after `new_session`) in smoke_test.sh
- [ ] T027 (cc-mux-3hz.3) [US5] Add test case: session detection (start `claude` manually, wait 5s, verify status bar pipe message indicates session count > 0) in smoke_test.sh
- [ ] T028 (cc-mux-3hz.4) [US5] Add test case: tab title updates (send status hook pipe, verify tab name changed via `zellij action query-tab-names` or layout dump) in smoke_test.sh
- [ ] T029 (cc-mux-3hz.5) [US5] Add test case: session removal and cleanup (close session, verify tab count decreases, kill test session) in smoke_test.sh

**Checkpoint**: `./smoke_test.sh` runs fully automated, reports pass/fail for each feature, cleans up test resources

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Edge case handling and build pipeline

- [ ] T030 (cc-mux-h4p.1) [P] Handle tab index shifting: when TabUpdate shows fewer tabs or reordered tabs, update all session tab_index values and re-issue rename_tab for any that changed in cc-zellij-plugin/src/main.rs
- [ ] T031 (cc-mux-h4p.2) [P] Handle rapid session creation: add a short debounce (100ms) to `pending_auto_start` to prevent race conditions when multiple `new_session` commands arrive quickly in cc-zellij-plugin/src/main.rs
- [ ] T032 (cc-mux-h4p.3) Rebuild WASM and update embedded binary: run `make build` and `cc-deck plugin install --force` to install the updated plugin in Makefile
- [ ] T033 (cc-mux-h4p.4) Run quickstart.md validation: execute all build and test steps from specs/010-plugin-bugfixes/quickstart.md end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (new struct fields)
- **US1 Tab Titles (Phase 3)**: Depends on Phase 2 (tab mapping + update_tab_title)
- **US2 Detection (Phase 4)**: Depends on Phase 2 (tab mapping for tab_index assignment)
- **US3 Auto-Start (Phase 5)**: Depends on Phase 2 (pending_auto_start field)
- **US4 Floating Picker (Phase 6)**: Depends on Phase 2 (session data for picker display)
- **US5 Tests (Phase 7)**: Depends on Phases 3-6 (tests validate the fixes)
- **Polish (Phase 8)**: Depends on all above

### User Story Dependencies

- **US1 (P1)**: Independent after Foundational. MVP target.
- **US2 (P1)**: Independent after Foundational. Can run in parallel with US1.
- **US3 (P2)**: Independent after Foundational. Benefits from US2 (detected sessions after auto-start).
- **US4 (P2)**: Independent after Foundational. Can run in parallel with US3.
- **US5 (P3)**: Depends on all US1-US4 being complete.

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (different files/methods)
- T004, T005 modify main.rs but different event handlers (can be serialized within same file)
- US1 (Phase 3) and US2 (Phase 4) can be developed in parallel
- US3 (Phase 5) and US4 (Phase 6) can be developed in parallel
- T030, T031 can run in parallel (different concerns)

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T006)
3. Complete Phase 3: US1 Tab Titles (T007-T011)
4. **STOP and VALIDATE**: Create sessions, trigger status changes, verify tab titles update

### Incremental Delivery

1. Setup + Foundational -> Infrastructure ready
2. US1 Tab Titles -> Validate tab bar shows status icons -> MVP!
3. US2 Detection -> Validate manual Claude sessions appear in status bar
4. US3 Auto-Start -> Validate plugin-created sessions launch Claude
5. US4 Floating Picker -> Validate picker renders as overlay
6. US5 Tests -> Validate automated test suite passes
7. Polish -> Edge cases, rebuild, install

---

## Notes

- All changes are to existing Rust files in cc-zellij-plugin/src/ (no new Rust files)
- The floating picker (US4) is the highest-risk item due to `open_plugin_pane_floating` not existing
- US4 uses `open_command_pane_floating` with a helper script as a workaround
- The helper script needs to be embedded or installed alongside the WASM binary
- Commit after each phase or logical group
- Build and install after each phase: `make build && cc-deck plugin install --force`

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
