# Brainstorm: Worktree Visibility in Sidebar

**Date:** 2026-07-01
**Status:** active

## Problem Framing

When Claude Code operates inside a worktree (`.claude/worktrees/<name>/`), the sidebar shows the original branch name (e.g., `main`) instead of the worktree's branch. The user cannot distinguish whether a session is on the main working tree or in a worktree. This happened because worktree management changed from parallel directories (where the directory name was visible) to subdirectories under `.claude/worktrees/`, and the sidebar's CWD filter (`hooks.rs:278`) suppresses all paths containing `/.claude/`.

## Approaches Considered

### A: Fix CWD filter + swap branch icon (Recommended)

Allow `.claude/worktrees/` CWD changes through the filter (only suppress other `.claude/` paths), detect worktree state from the path, and render a different branch icon.

- Pros: minimal changes (one filter fix, one bool on Session, one icon swap), no new protocols, no cc-spex changes needed
- Cons: requires the CWD hook to fire when entering a worktree (already happens, just gets filtered)

### B: New hook payload field from cc-spex

cc-spex sends a `worktree: true` field in the hook payload, sidebar reads it.

- Pros: explicit signal, no path parsing
- Cons: requires cc-spex changes, new protocol field, tight coupling between cc-spex and cc-deck

### C: Session name suffix

Append the worktree name to the session label (e.g., `cc-deck [076-fix]`).

- Pros: very visible
- Cons: takes horizontal space in narrow sidebar, clutters the session name, needs cleanup when leaving worktree

## Decision

Approach A: Fix the CWD filter to allow `.claude/worktrees/` paths through, detect worktree state from the path, store an `in_worktree` bool on the Session, and swap the branch icon from `⎇` to `⌥` when in a worktree.

## Key Requirements

- The CWD filter at `hooks.rs:278` must distinguish `.claude/worktrees/` (allow through) from other `.claude/` paths (keep filtering)
- The git branch detection must trigger when entering a worktree, so the branch name updates to the worktree branch
- The branch icon changes from `⎇` (U+2387) to `⌥` (U+2325) when the session is in a worktree
- When leaving the worktree (CWD changes back to the main working tree), the icon reverts to `⎇`
- No changes needed to cc-spex or the hook protocol

## Open Questions

- Should the `in_worktree` state persist across Zellij reattach (via the session cache), or be re-detected from the CWD?
- Should the worktree icon color differ from the regular branch color, or just the shape?
