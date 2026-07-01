# Implementation Plan: Worktree Sidebar Visibility

**Branch**: `076-worktree-sidebar-visibility` | **Date**: 2026-07-01 | **Spec**: [spec.md](spec.md)

## Summary

Make worktree state visible in the cc-deck sidebar by fixing the CWD filter to allow `.claude/worktrees/` paths through, adding an `in_worktree` bool to Session, and swapping the branch icon from `⎇` to `⌥` when in a worktree.

## Technical Context

**Language/Version**: Rust (stable, edition 2021, wasm32-wasip1 target)
**Framework**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: WASI `/cache/` directory for persistent state (sessions.json)
**Build**: `make install` (per constitution III)

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests for CWD filter and icon rendering. No user-facing docs needed (internal rendering change, per spec Documentation Impact). |
| II. Interface contracts | N/A | No new interfaces or backends. |
| III. Build and tool rules | PASS | Use `make install`, `make test`, `make lint`. |

## Implementation Phases

### Phase 1: CWD Filter Fix (FR-001, FR-002, FR-003, FR-004)

**File**: `cc-zellij-plugin/src/controller/hooks.rs`

Update `process_cwd_change()` at line 278:

```
Current:  let is_worktree_cwd = cwd.contains("/.claude/");
Changed:  let is_worktree_cwd = cwd.contains("/.claude/") && !cwd.contains("/.claude/worktrees/");
```

This allows `.claude/worktrees/` paths through while continuing to suppress other `.claude/` paths.

After the existing `working_dir` assignment (line 287), add worktree state detection:

```rust
if let Some(s) = state.sessions.get_mut(&pane_id) {
    s.in_worktree = cwd.contains("/.claude/worktrees/");
}
```

For non-worktree CWD changes, the `in_worktree` field is set to `false` (FR-003). This happens naturally since the above code runs for every allowed CWD change.

Git branch detection already triggers after `working_dir` is updated (existing `detect_git_branch` call at line 320). No changes needed for FR-004.

### Phase 2: Session Struct (FR-006)

**File**: `cc-zellij-plugin/src/session.rs`

Add `in_worktree: bool` field to the `Session` struct after `git_branch`:

```rust
#[serde(default)]
pub in_worktree: bool,
```

The `#[serde(default)]` attribute ensures backward compatibility with existing serialized session caches (new fields default to `false`).

Initialize to `false` in `Session::new()`.

### Phase 3: Icon Rendering (FR-005, FR-007)

**File**: `cc-zellij-plugin/src/sidebar_plugin/render.rs`

Update `format_line2()` to accept the `in_worktree` flag and swap the icon:

```rust
let branch_icon = if in_worktree { "\u{2325}" } else { "\u{2387}" };
```

Replace the hardcoded `\u{2387}` references with `branch_icon`. The color parameter is already passed through and unchanged (FR-007).

Update the call site in `render_session()` to pass `session.in_worktree`.

### Phase 4: Tests

**File**: `cc-zellij-plugin/src/controller/hooks.rs` (or new test module)

Add unit tests:
1. CWD with `/.claude/worktrees/076-fix/` sets `in_worktree = true` and updates `working_dir`
2. CWD with `/.claude/settings.json` is suppressed (no `working_dir` update)
3. CWD change from worktree to non-worktree path sets `in_worktree = false`
4. CWD change between two worktrees keeps `in_worktree = true` and updates branch

## Key Decisions

- **Path detection over git detection**: Use `cwd.contains("/.claude/worktrees/")` rather than checking `.git` file type. Simpler, faster, and the path pattern is stable.
- **Single bool over enum**: `in_worktree: bool` is sufficient since there are only two states (in worktree or not). No need for a richer enum.
- **Same color, different shape**: The worktree icon uses identical coloring to the branch icon, only the Unicode character differs. This keeps the visual weight consistent.
