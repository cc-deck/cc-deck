# Tasks: Worktree Sidebar Visibility

**Feature**: 076-worktree-sidebar-visibility
**Generated**: 2026-07-01

## Phase 1: Session Struct

- [x] T001 Add `in_worktree: bool` field with `#[serde(default)]` to Session struct in `cc-zellij-plugin/src/session.rs`
- [x] T002 Initialize `in_worktree: false` in `Session::new()` in `cc-zellij-plugin/src/session.rs`

## Phase 2: CWD Filter Fix (User Story 1)

- [x] T003 [US1] Update CWD filter in `process_cwd_change()` to allow `.claude/worktrees/` paths through in `cc-zellij-plugin/src/controller/hooks.rs`
- [x] T004 [US1] Set `session.in_worktree = true` when CWD contains `/.claude/worktrees/` in `cc-zellij-plugin/src/controller/hooks.rs`
- [x] T005 [US1] Set `session.in_worktree = false` when CWD changes to a non-worktree path in `cc-zellij-plugin/src/controller/hooks.rs`

## Phase 3: Icon Rendering (User Story 2)

- [x] T006 [US2] Update `format_line2()` signature to accept `in_worktree: bool` in `cc-zellij-plugin/src/sidebar_plugin/render.rs`
- [x] T007 [US2] Swap branch icon from `\u{2387}` to `\u{2325}` when `in_worktree` is true in `cc-zellij-plugin/src/sidebar_plugin/render.rs`
- [x] T008 [US2] Update call site in `render_session()` to pass `session.in_worktree` to `format_line2()` in `cc-zellij-plugin/src/sidebar_plugin/render.rs`

## Phase 4: Tests

- [x] T009 [P] Add test: CWD with `/.claude/worktrees/` sets `in_worktree = true` and updates `working_dir` in `cc-zellij-plugin/src/controller/hooks.rs`
- [x] T010 [P] Add test: CWD with `/.claude/settings.json` is suppressed in `cc-zellij-plugin/src/controller/hooks.rs`
- [x] T011 [P] Add test: CWD from worktree to non-worktree clears `in_worktree` in `cc-zellij-plugin/src/controller/hooks.rs`

## Dependencies

```
T001, T002 (no dependencies, foundational)
T003, T004, T005 depend on T001
T006, T007, T008 depend on T001
T009, T010, T011 depend on T003, T004, T005
```

## Parallel Execution

- T001 and T002 can run together (same file, different locations)
- T006, T007, T008 can run in parallel with T003, T004, T005 (different files)
- T009, T010, T011 are all parallelizable (independent tests)

## Implementation Strategy

**MVP (User Story 1 only)**: T001-T005 deliver the core fix (branch name updates when entering a worktree). This is independently testable.

**Full feature**: T006-T008 add the visual icon distinction. T009-T011 add test coverage.

Total: 11 tasks across 4 phases.
