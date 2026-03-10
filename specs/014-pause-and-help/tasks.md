# Tasks: Session Pause Mode & Keyboard Help

**Input**: Design documents from `/specs/014-pause-and-help/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Organization**: Tasks grouped by user story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

## Phase 1: Setup

**Purpose**: Data model changes needed by both user stories

- [ ] T001 (cc-mux-3qw.1) [P] Add `paused: bool` field (default false) to Session struct in cc-zellij-plugin/src/session.rs
- [ ] T002 (cc-mux-3qw.2) [P] Add `show_help: bool` field (default false) to PluginState in cc-zellij-plugin/src/state.rs

**Checkpoint**: Project compiles with new fields. All existing tests pass.

---

## Phase 2: User Story 1 - Pause/Unpause Sessions (Priority: P1)

**Goal**: Toggle pause on sessions to exclude them from attend cycling.

**Independent Test**: Create 3 sessions, pause one with `p`, press `Alt+a` repeatedly, verify paused session is skipped.

### Implementation for User Story 1

- [ ] T003 (cc-mux-qro.1) [US1] Add `p` key handler in navigation mode: toggle `paused` flag on cursor session, broadcast state in cc-zellij-plugin/src/main.rs
- [ ] T004 (cc-mux-qro.2) [US1] Filter out paused sessions from attend candidate list (all tiers) in cc-zellij-plugin/src/attend.rs
- [ ] T005 (cc-mux-qro.3) [US1] Update attend to show "No sessions available" when all candidates are paused or working in cc-zellij-plugin/src/main.rs
- [ ] T006 (cc-mux-qro.4) [US1] Render paused sessions with ⏸ icon and dimmed grey name in cc-zellij-plugin/src/sidebar.rs
- [ ] T007 (cc-mux-qro.5) [P] [US1] Add unit test for attend filtering paused sessions in cc-zellij-plugin/src/attend.rs

**Checkpoint**: Pause toggle works. Paused sessions show ⏸ icon, are skipped by attend.

---

## Phase 3: User Story 2 - Keyboard Help Overlay (Priority: P2)

**Goal**: Press `?` to show a help screen listing all shortcuts.

**Independent Test**: Enter navigation mode, press `?`, verify help shown, press any key to dismiss.

### Implementation for User Story 2

- [ ] T008 (cc-mux-snw.1) [US2] Add `?` key handler in navigation mode: set `show_help = true` in cc-zellij-plugin/src/main.rs
- [ ] T009 (cc-mux-snw.2) [US2] Add help dismiss handler: any key when `show_help` is true sets it to false in cc-zellij-plugin/src/main.rs
- [ ] T010 (cc-mux-snw.3) [US2] Render help overlay in render_sidebar when `show_help` is true, replacing session list in cc-zellij-plugin/src/sidebar.rs

**Checkpoint**: Help overlay shows all shortcuts, any key dismisses.

---

## Phase 4: Polish & Cross-Cutting Concerns

- [ ] T011 (cc-mux-exw.1) Run `cargo clippy --target wasm32-wasip1 -- -D warnings` and fix any issues in cc-zellij-plugin/
- [ ] T012 (cc-mux-exw.2) Verify all tests pass (existing + new attend test)
- [ ] T013 (cc-mux-exw.3) Run quickstart.md validation: build, install, test pause and help

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **US1 (Phase 2)**: Depends on Phase 1 (paused field needed)
- **US2 (Phase 3)**: Depends on Phase 1 (show_help field needed)
- **Polish (Phase 4)**: Depends on all phases

### Parallel Opportunities

- T001, T002 can run in parallel (different files)
- US1 and US2 can run in parallel after Phase 1 (independent features)
- T007 can run in parallel with T003-T006

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (2 tasks)
2. Complete Phase 2: US1 Pause (5 tasks)
3. **STOP and VALIDATE**: Pause works, attend skips paused
4. This is the MVP

### Incremental Delivery

1. Setup + US1 → Pause mode (MVP)
2. Add US2 → Help overlay
3. Polish → Clippy, tests, validation


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
