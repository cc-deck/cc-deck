# Research: Worktree Sidebar Visibility

## R1: CWD Filter Behavior

**Decision**: Refine the `.claude/` path filter to distinguish worktree paths from other internal paths.

**Rationale**: The current filter `cwd.contains("/.claude/")` is too broad. It blocks all CWD changes inside `.claude/`, including the worktree directory. A simple additional check `!cwd.contains("/.claude/worktrees/")` carves out the exception without changing behavior for other `.claude/` paths.

**Alternatives considered**:
- Allowlist approach (check for specific allowed subdirectories): More complex, requires maintenance as `.claude/` structure evolves.
- Remove the filter entirely: Would expose internal paths like `.claude/settings.json` as CWD changes, causing spurious branch detection.

## R2: Session Serialization Compatibility

**Decision**: Use `#[serde(default)]` on the new `in_worktree` field.

**Rationale**: Existing session caches serialized without `in_worktree` will deserialize successfully with the field defaulting to `false`. This is the standard serde pattern for backward-compatible field additions, already used by other Session fields (`paused`, `done_attended`, `pending_tab_rename`).

**Alternatives considered**:
- Version migration: Overkill for a single bool field with a safe default.

## R3: Git Branch Detection in Worktrees

**Decision**: Rely on the existing `detect_git_branch()` function without modification.

**Rationale**: Git worktrees created by `.claude/worktrees/` have their own `.git` file (not directory) that points to the main repo's git directory. The `git rev-parse --abbrev-ref HEAD` command works correctly inside worktrees, returning the worktree's branch name. The existing `detect_git_branch()` function at `cc-zellij-plugin/src/git.rs` already handles this correctly since it spawns `git` as a subprocess in the worktree's CWD.

**Alternatives considered**:
- Parse `.git` file to extract branch: Fragile, duplicates git's own logic, unnecessary.

## R4: Icon Character Selection

**Decision**: Use `⌥` (U+2325, OPTION KEY) for worktree branches, `⎇` (U+2387, ALTERNATIVE KEY SYMBOL) for regular branches.

**Rationale**: Both are single-width Unicode characters that render well in terminal fonts. `⌥` is visually distinct from `⎇` (fork shape vs. angular shape) and semantically appropriate ("option/alternative workspace"). The user confirmed this choice during brainstorming.

**Alternatives considered**:
- `⑂` (U+2442, OCR FORK): Less commonly rendered in terminal fonts.
- `⋔` (U+22D4, PITCHFORK): Heavier visual weight, doesn't match sidebar aesthetic.
