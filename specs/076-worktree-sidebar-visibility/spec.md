# Feature Specification: Worktree Sidebar Visibility

**Feature Branch**: `076-worktree-sidebar-visibility`
**Created**: 2026-07-01
**Status**: Draft
**Input**: Make worktree state visible in the cc-deck sidebar when Claude Code operates inside .claude/worktrees/

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Branch Name Updates When Entering Worktree (Priority: P1)

A developer starts a spec-driven workflow in cc-deck. The spex extension creates a worktree at `.claude/worktrees/<name>/` and Claude Code switches into it. The sidebar's branch indicator updates from `main` to the worktree's feature branch name (e.g., `076-worktree-sidebar-visibility`).

**Why this priority**: This is the core problem. Without the branch name updating, the user cannot tell which branch they are working on, leading to confusion about whether changes are landing on main or a feature branch.

**Independent Test**: Can be tested by creating a worktree, switching into it, and verifying the sidebar shows the correct branch name instead of `main`.

**Acceptance Scenarios**:

1. **Given** a session on `main` with the sidebar showing `⎇ main`, **When** Claude Code enters a worktree at `.claude/worktrees/076-fix/`, **Then** the sidebar updates to show the worktree's branch name (e.g., `076-fix`).
2. **Given** a session inside a worktree showing a feature branch, **When** Claude Code exits the worktree and returns to the main working directory, **Then** the sidebar reverts to showing `main`.

---

### User Story 2 - Worktree Icon Distinguishes from Regular Branch (Priority: P2)

When a session is operating inside a worktree, the branch icon changes from `⎇` to `⌥` so the user can visually distinguish a worktree branch from a regular branch at a glance.

**Why this priority**: The branch name alone may not make it obvious that the session is in a worktree rather than just on a different branch. The icon provides an instant visual cue without reading the branch name.

**Independent Test**: Can be tested by entering a worktree and verifying the icon changes from `⎇` to `⌥`, then exiting and verifying it reverts.

**Acceptance Scenarios**:

1. **Given** a session on `main` with icon `⎇`, **When** Claude Code enters a worktree, **Then** the icon changes to `⌥`.
2. **Given** a session in a worktree with icon `⌥`, **When** Claude Code exits the worktree, **Then** the icon reverts to `⎇`.
3. **Given** a session that switches directly between two different worktrees, **When** the CWD changes from one worktree to another, **Then** the icon remains `⌥` and the branch name updates.

---

### User Story 3 - Worktree State Persists Across Reattach (Priority: P3)

When the user detaches from Zellij and reattaches, sessions that were in a worktree still show the `⌥` icon and the correct worktree branch name.

**Why this priority**: Without persistence, reattaching would lose the worktree visual indicator, forcing the user to trigger a CWD change to restore it.

**Independent Test**: Can be tested by entering a worktree, detaching Zellij, reattaching, and verifying the sidebar still shows `⌥` with the correct branch.

**Acceptance Scenarios**:

1. **Given** a session in a worktree showing `⌥ 076-fix`, **When** the user detaches and reattaches Zellij, **Then** the sidebar still shows `⌥ 076-fix`.

---

### Edge Cases

- What happens when the CWD changes to `.claude/settings.json` or other non-worktree `.claude/` paths? The filter continues to suppress these, only `.claude/worktrees/` paths are allowed through.
- What happens when a worktree is removed while the session is active? The next CWD change to a non-worktree path clears the `in_worktree` state and reverts the icon.
- What happens when the CWD contains `.claude/worktrees/` but is not a git worktree (e.g., a manually created directory)? The path detection sets `in_worktree` based on the path pattern, but git branch detection may fail gracefully (showing no branch or the parent repo's branch).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CWD filter in `hooks.rs` MUST allow CWD changes through when the path contains `/.claude/worktrees/`, while continuing to suppress other `/.claude/` paths.
- **FR-002**: When a CWD change passes through with a `.claude/worktrees/` path, the system MUST set an `in_worktree` boolean field on the Session to `true`.
- **FR-003**: When a CWD change occurs to a path that does NOT contain `.claude/worktrees/`, the system MUST set `in_worktree` to `false`.
- **FR-004**: The git branch detection MUST trigger for `.claude/worktrees/` CWD changes, updating the displayed branch name.
- **FR-005**: The sidebar MUST render the branch icon as `⌥` (U+2325) when `in_worktree` is `true`, and `⎇` (U+2387) when `in_worktree` is `false`.
- **FR-006**: The `in_worktree` field MUST be included in the session serialization/deserialization for persistence across Zellij reattach.
- **FR-007**: The worktree icon MUST use the same color as the regular branch icon (same base color, same fading behavior).

### Key Entities

- **Session**: Existing struct tracking a Claude Code session. Gains an `in_worktree: bool` field that defaults to `false`.
- **CWD Filter**: The path-matching logic in `process_cwd_change()` that decides whether to update `working_dir` and trigger branch detection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When Claude Code enters a worktree, the sidebar branch name updates within one render cycle (the next hook event).
- **SC-002**: The worktree icon `⌥` is visually distinguishable from the regular branch icon `⎇` at the sidebar's default width.
- **SC-003**: After Zellij detach and reattach, worktree sessions retain their `⌥` icon and branch name without requiring user interaction.
- **SC-004**: CWD changes to non-worktree `.claude/` paths (e.g., `.claude/settings.json`) continue to be suppressed (no regression).

## Documentation Impact

This is a visual indicator change in the sidebar plugin. No CLI commands, configuration options, or user-facing documentation changes are needed. The README does not need updating since this is an internal rendering detail.

## Clarifications

### Session 2026-07-01

- Q: Should the `in_worktree` state persist across Zellij reattach? → A: Yes, via serde serialization (FR-006).
- Q: Should the worktree icon color differ from the regular branch icon? → A: No, same color, only shape differs (FR-007).

## Assumptions

- The CWD hook fires when Claude Code enters a worktree (this already happens, the hook payload includes the new CWD).
- Git branch detection via `detect_git_branch()` works correctly inside `.claude/worktrees/` directories (git worktrees have their own `.git` file that points to the main repo's git directory).
- The `in_worktree` field can be added to the Session struct without breaking backward compatibility with existing serialized session caches (new fields with defaults are safe in serde).
- The worktree path pattern `/.claude/worktrees/` is stable and will not change in Claude Code's worktree implementation.
