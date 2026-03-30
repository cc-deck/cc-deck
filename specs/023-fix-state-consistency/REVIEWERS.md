# Review Guide: Fix State Consistency and Add Refresh Command

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-03-30

---

## What This Spec Does

Fixes three bugs where the cc-deck sidebar plugin shows inconsistent or stale session state across tabs and Zellij sessions. Also adds a manual "refresh" command so users can force-correct corrupted state without restarting.

**In scope:** Broadcasting cleanup transitions to all instances, PID-based staleness detection for metadata files, pruning dead session entries, "!" key and CLI pipe refresh command.

**Out of scope:** The WASM crash seen in screenshot 2 (deferred until after these fixes), changes to the merge/conflict resolution logic, changes to the sidebar rendering, changes to the hook event pipeline.

## Bigger Picture

cc-deck is a Zellij WASM plugin that tracks Claude Code sessions across tabs. Each tab gets its own plugin instance, all sharing a `/cache/` directory. The plugin has grown organically from single-tab to multi-tab, and the sync mechanisms haven't kept pace. These bugs represent the gap between "works for one instance" and "works for N instances."

This fix addresses the most-reported usability issue: users can't trust the sidebar indicators. After this, the remaining known issue is the occasional WASM crash (which may itself be caused by the stale state feeding unexpected data to rendering code).

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts of the spec and plan that need human judgment most.

### Understanding the approach (8 min)

Read [User Story 1](spec.md#user-story-1---session-activity-indicators-stay-consistent-across-all-tabs-priority-p1) and [User Story 2](spec.md#user-story-2---no-ghost-state-from-previous-zellij-sessions-priority-p1) for the two P1 bugs. As you read, consider:

- Are these truly the two highest-priority bugs, or does the metadata accumulation (US3) cause more real-world confusion than the Done-to-Idle timing issue?
- Is the 30-second done_timeout the right value? The spec takes it as given, but should it be configurable or shorter?
- Does the "broadcast on cleanup" approach create any risk of message storms in sessions with many tabs (10+)?

### Key decisions that need your eyes (12 min)

**Broadcast vs. file-based sync for cleanup transitions** ([research.md, topic 1](research.md))

The plan changes `save_sessions()` to `broadcast_and_save()` for timer-driven transitions. This means every 10-second timer tick that transitions a Done session will send a pipe broadcast to all instances.

- Question for reviewer: With 10+ tabs, could the broadcast overhead from cleanup transitions cause noticeable latency? The broadcasts are small (JSON of session map), but Zellij's WASM pipe mechanism has limits.

**Hybrid refresh approach** ([research.md, topic 4](research.md))

The refresh command clears file caches but keeps in-memory state, treating the active instance as authoritative. An alternative would be to clear everything and let sessions rediscover via hook events.

- Question for reviewer: Is trusting the active instance's in-memory state always safe? Could there be scenarios where in-memory state is itself corrupted (e.g., after a partial broadcast)?

**PID-based staleness for session-meta.json** ([plan.md, Bug Fix 2](plan.md#bug-fix-2-clear-session-metajson-on-pid-mismatch-fr-003))

The fix adds a single `remove_file` call to the existing PID-mismatch branch. This is minimal but relies on `restore_sessions()` being the only entry point for cache loading.

- Question for reviewer: Is there any code path where `apply_session_meta()` could run before `restore_sessions()` has a chance to clear stale files? (The timer starts in `load()`, permissions are requested, and `restore_sessions()` runs on permission grant.)

### Areas where I'm less certain (5 min)

- [FR-004 / prune implementation](spec.md#functional-requirements): The prune function reads, filters, and writes session-meta.json. If another instance writes to the file between the read and write, the second write wins and could lose the first instance's updates. This is the same non-atomic pattern used everywhere else in the codebase, but it's worth noting. In practice, only the active-tab instance writes metadata (renames, pauses), so collisions should be rare.

- [US4 refresh via "!" key](spec.md#user-story-4---user-can-force-refresh-sidebar-state-priority-p2): The refresh handler in the pipe() method and the key handler in handle_navigation_key() share the same logic. The plan notes "consider extracting a helper," but the tasks don't explicitly create one. During implementation, the developer will need to decide whether to duplicate ~8 lines or extract a method.

### Risks and open questions (5 min)

- If `broadcast_and_save` is called more frequently (now also on cleanup transitions), does Zellij's internal pipe buffer have a size limit that could be hit with many sessions? The [pipe documentation for Zellij 0.43](https://zellij.dev/documentation/plugin-api-pipes) doesn't mention explicit limits, but real-world behavior may differ.
- The spec assumes pane IDs are globally unique within a session but can be reused across sessions. Is this actually guaranteed by Zellij, or is it an implementation detail that could change?
- After implementing all four fixes, should we add a debug log entry when `prune_session_meta` removes entries, to help diagnose future state issues?

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 4 source files in `cc-zellij-plugin/src/`: main.rs, sync.rs, pipe_handler.rs, sidebar.rs

### Understanding the changes (8 min)

- Start with `sync.rs`: This has two structural changes: (1) `META_PATH` removal added to `restore_sessions()` PID-mismatch branch (line 134), and (2) the new `prune_session_meta()` function (lines 247-264). These are the foundational data-layer changes.
- Then `main.rs`: Three change areas: Timer handler (lines 1278-1286) switches to broadcast, PaneClosed handler (line 1421) adds prune call, and the two new refresh handlers (pipe at 1053-1071, key at 1746-1759).
- Question: Is the change from `save_sessions` to `broadcast_and_save` in the Timer handler the right granularity? It broadcasts on every cleanup tick that transitions any session, which is correct but increases message volume compared to the previous disk-only approach.

### Key decisions that need your eyes (12 min)

**Duplicated refresh logic** (`main.rs:1053-1071` pipe handler + `main.rs:1746-1759` key handler, relates to [FR-005](spec.md#functional-requirements) and [FR-006](spec.md#functional-requirements))

Both the pipe handler and the "!" key handler contain identical refresh logic (~8 lines: remove 3 files, reset hash, broadcast, notification). A helper method was considered but not extracted.

- Question: Is the duplication acceptable given the small size, or should a `perform_refresh()` method be extracted? The duplication means a future change to refresh logic requires updating two locations.

**Hardcoded cache paths vs constants** (`main.rs:1058-1060` and `main.rs:1748-1750`)

The refresh handlers use hardcoded strings `"/cache/sessions.json"` and `"/cache/session-meta.json"` instead of the `SESSIONS_PATH` and `META_PATH` constants defined in `sync.rs` (which are module-private).

- Question: Should these constants be made `pub(crate)` and shared, or is the hardcoded approach acceptable given the paths are stable?

**Prune call sites** (`main.rs:1280`, `main.rs:1233`, `main.rs:1421`, relates to [FR-004](spec.md#functional-requirements))

`prune_session_meta()` is called at three points: post-startup-grace cleanup, PaneUpdate dead session removal, and PaneClosed. All three guard on `removed == true`.

- Question: Is it correct that the PaneUpdate path (line 1233) prunes on every dead session removal? With high tab churn, this means reading and rewriting session-meta.json on every PaneUpdate that removes a session. Is the I/O acceptable?

### Areas where I'm less certain (5 min)

- `main.rs:1062` and `main.rs:1751`: Resetting `last_meta_content_hash` to 0 after refresh is necessary so the next `apply_session_meta()` call re-reads the file. Without this, the hash comparison would skip the re-read. But if `broadcast_and_save` writes sessions.json and the next timer tick calls `apply_session_meta`, could there be a window where the meta file doesn't exist yet (just deleted) and `apply_session_meta` returns false? I believe this is fine (returns false = no change = correct), but worth verifying.

- `pipe_handler.rs:53`: The `Refresh` variant doesn't carry a payload, which is correct for current behavior. But if refresh ever needs parameters (e.g., "refresh only sessions" vs "refresh everything"), the variant would need to change.

### Deviations and risks (5 min)

No deviations from [plan.md](plan.md) were identified. All changes match the planned approach exactly:
- `save_sessions` changed to `broadcast_and_save` as planned
- `META_PATH` removal added to PID-mismatch branch as planned
- `prune_session_meta` function signature and call sites match plan
- Refresh handler behavior matches plan (clear caches, keep in-memory, broadcast, notify)

One risk to note: the plan suggested "consider extracting a `perform_refresh()` helper" but this was implemented as duplication. This is a code quality note, not a spec deviation.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-03-30 | **Rounds:** 0/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 0 | completed |
| Architecture & Idioms | 2 | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 0 | completed |
| CodeRabbit (external) | 0 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 2 | - | 2 |

### What was fixed automatically

No fixes needed. Zero Critical or Important findings.

### What still needs human attention

All Critical and Important findings were resolved (none existed). 2 Minor findings remain (see [review-findings.md](review-findings.md) for details):

- The refresh logic is duplicated between the pipe handler and the "!" key handler (~8 identical lines). Should a `perform_refresh()` helper be extracted?
- Hardcoded cache paths in the refresh handlers could use the constants from sync.rs if made `pub(crate)`. Worth the refactoring?

### Recommendation

All findings addressed. Code is ready for human review with no known blockers. The 2 Minor findings are code quality improvements that can be addressed in a follow-up cleanup pass.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
