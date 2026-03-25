# Tasks: Sidebar State Refresh on Reattach

**Input**: Design documents from `/specs/025-sidebar-state-refresh/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Unit tests for the grace period logic. Live testing via quickstart.md (manual, requires Zellij session).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add the grace period field to plugin state

- [x] T001 Add `startup_grace_until: Option<u64>` field to `PluginState` in `cc-zellij-plugin/src/state.rs`

---

## Phase 2: User Story 1 - Reattach Preserves Session List (Priority: P1) MVP

**Goal**: After reattaching, the sidebar shows cached sessions instead of "No Claude sessions". Dead session cleanup is deferred during a 3-second grace period after permission grant.

**Independent Test**: Start two Claude Code panes, detach, reattach, verify sidebar shows both sessions within 1 second.

### Implementation for User Story 1

- [x] T002 [US1] Set `startup_grace_until` to `unix_now_ms() + 3000` at permission grant in `cc-zellij-plugin/src/main.rs` (after session restore, around line 252)
- [x] T003 [US1] Guard `remove_dead_sessions()` call in PaneUpdate handler to skip when `startup_grace_until` is active (current time < grace deadline) in `cc-zellij-plugin/src/main.rs` (around line 720)
- [x] T004 [US1] Add unit test: `remove_dead_sessions` is skipped during grace period in `cc-zellij-plugin/src/state.rs`
- [x] T005 [US1] Add unit test: `remove_dead_sessions` runs normally after grace period expires in `cc-zellij-plugin/src/state.rs`

**Checkpoint**: Reattach preserves session list. Stale entries persist until grace period expires, then are cleaned up on next PaneUpdate.

---

## Phase 3: User Story 2 - Fresh Start with No Cache (Priority: P2)

**Goal**: Verify that a fresh session with no cached state works identically to existing behavior. The grace period has no effect when the cache is empty.

**Independent Test**: Clear cached state, start fresh session, open Claude Code pane, verify sidebar picks it up via hook events.

### Implementation for User Story 2

- [x] T006 [US2] Add unit test: grace period does not interfere with empty session map (no sessions to protect, hook events still add sessions normally) in `cc-zellij-plugin/src/state.rs`

**Checkpoint**: Fresh sessions work identically to pre-feature behavior.

---

## Phase 4: User Story 3 - Stale Session Cleanup After Reattach (Priority: P2)

**Goal**: After grace period expires, stale cached entries for panes that no longer exist are removed on the next PaneUpdate.

**Independent Test**: Start a pane, detach, kill the pane externally, reattach, verify the stale entry is removed within a few seconds.

### Implementation for User Story 3

- [x] T007 [US3] Add unit test: after grace period, `remove_dead_sessions` removes cached entries whose pane IDs are absent from the manifest in `cc-zellij-plugin/src/state.rs`

**Checkpoint**: Stale entries are cleaned up after grace period. Combined with US1, the full reattach lifecycle works correctly.

---

## Phase 5: User Story 4 - State Reconciliation After Reattach (Priority: P3)

**Goal**: Cached activity states (e.g., "Working") update when new hook events arrive. This is already handled by existing hook event flow; no new code needed.

**Independent Test**: Start a pane in "Working" state, detach, wait for it to finish, reattach, verify sidebar updates when next hook event fires.

### Implementation for User Story 4

No implementation tasks. The existing `Session::transition()` method and hook event handling already update activity states as events arrive. The grace period does not affect hook event processing.

**Checkpoint**: Existing hook event flow naturally reconciles stale activity states. Verified by existing tests in `session.rs`.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T008 Run `make test` to verify all existing tests pass with the new field
- [x] T009 Run `make lint` (cargo clippy) to verify no new warnings
- [x] T010 Run quickstart.md live validation (manual: detach/reattach test with Zellij) -- verified in daily use
- [x] T011 Update README.md spec table with 025-sidebar-state-refresh entry

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, start immediately
- **Phase 2 (US1)**: Depends on Phase 1 (T001)
- **Phase 3 (US2)**: Depends on Phase 2 (T002, T003)
- **Phase 4 (US3)**: Depends on Phase 2 (T002, T003)
- **Phase 5 (US4)**: No implementation needed; verification only
- **Phase 6 (Polish)**: Depends on all previous phases

### User Story Dependencies

- **US1 (P1)**: Core implementation; all other stories depend on this
- **US2 (P2)**: Can be tested after US1 is complete
- **US3 (P2)**: Can be tested after US1 is complete; parallel with US2
- **US4 (P3)**: No new code; existing behavior verified

### Within User Story 1

- T002 and T003 are sequential (T003 depends on T002 to know what to guard)
- T004 and T005 can run in parallel after T003

### Parallel Opportunities

- T004 and T005 can run in parallel (different test functions, same file)
- US2 (T006) and US3 (T007) can run in parallel after US1 is complete
- T008 and T009 can run in parallel (different tools)

---

## Parallel Example: User Story 1

```bash
# Sequential: T001 → T002 → T003
# Then parallel:
Task: "Unit test: remove_dead_sessions skipped during grace period in state.rs"
Task: "Unit test: remove_dead_sessions runs after grace period in state.rs"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Add field (T001)
2. Complete Phase 2: Set grace timestamp + guard cleanup (T002, T003, T004, T005)
3. **STOP and VALIDATE**: Test reattach preserves sessions
4. This is the minimum viable fix for the reported bug

### Incremental Delivery

1. T001-T005: Core fix (MVP, resolves the reported bug)
2. T006-T007: Regression and edge case tests
3. T008-T011: Polish, lint, live validation, docs
