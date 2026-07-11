# Deep Review Findings

**Date:** 2026-07-11
**Branch:** 080-sidebar-auto-sort
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** superpowers

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 3 | 3 | 0 |
| Important | 7 | 7 | 0 |
| Minor | 0 | - | 0 |
| Notable | 0 | - | 0 |
| **Total** | **10** | **10** | **0** |

**Agents completed:** 5/5 (+ 1 external tool: CodeRabbit)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/actions.rs:33-63
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`handle_switch()` auto-unpauses a session on focus switch but never calls `state.save_sessions()`, so the unpause is lost on reattach.

**Why this matters:**
After switching to a paused session (which auto-unpauses it) and then detaching/reattaching, the session would revert to paused state, confusing the user.

**How it was resolved:**
Added `auto_unpaused` boolean flag and a conditional `state.save_sessions()` call after the auto-unpause block.

---

### FINDING-2
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/events.rs:221-229
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The startup grace drain loop removes unconfirmed sessions from `sessions` and `sort_order` but not from `auto_sort_tail`, leaving stale pane IDs.

**Why this matters:**
Stale pane IDs in `auto_sort_tail` could cause the render payload builder to reference non-existent sessions, producing incorrect display ordering.

**How it was resolved:**
Added `state.auto_sort_tail.retain(|&p| p != pane_id)` inside the drain loop.

---

### FINDING-3
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:114
- **Category:** correctness
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Paused sessions in the render payload were not sorted by `tab_index`, so their display order depended on HashMap iteration order, which is non-deterministic.

**Why this matters:**
Paused sessions would appear in arbitrary order in the sidebar's paused zone, making the UI confusing and unstable across renders.

**How it was resolved:**
Added `paused.sort_by_key(|s| s.tab_index)` after the auto_sort_tail handling block.

---

### FINDING-4
- **Severity:** Critical
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:657-682
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`test_auto_sort_tail_moves_unpaused_to_end_of_active` was tautological: session 30 had `tab_index = Some(2)`, so it would naturally sort last even without `auto_sort_tail`. The test could never fail.

**Why this matters:**
A tautological test provides false confidence. If the `auto_sort_tail` logic were removed entirely, this test would still pass.

**How it was resolved:**
Changed session 30 to `tab_index = Some(0)` (naturally first), session 10 to `Some(1)`, and session 20 to `Some(2)`. Now the test verifies that `auto_sort_tail` actually moves session 30 from first to last in the active zone.

---

### FINDING-5
- **Severity:** Critical
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/actions.rs:704-719
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`test_handle_pause_toggle` tested the pause/unpause toggle but never asserted on `auto_sort_tail`, leaving FR-002 (recently-unpaused tracking) untested through this code path.

**Why this matters:**
The `handle_pause` function manages `auto_sort_tail` on both pause and unpause transitions. Without assertions, regressions in this logic would go undetected.

**How it was resolved:**
Added `assert!(!state.auto_sort_tail.contains(&42))` after pause (verifying removal) and `assert!(state.auto_sort_tail.contains(&42))` after unpause (verifying addition).

---

### FINDING-6
- **Severity:** Critical
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/actions.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
No test existed for `handle_switch()` auto-unpause behavior (the session should be unpaused and added to `auto_sort_tail` when the user switches to it).

**Why this matters:**
The auto-unpause-on-switch feature (FR-003) was untested. A regression could silently break the user experience of resuming paused sessions.

**How it was resolved:**
Added `test_handle_switch_auto_unpauses_and_populates_tail` test that creates a paused session, switches to it, and asserts both unpause and `auto_sort_tail` membership.

---

### FINDING-7
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/actions.rs:132-137
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`test_handle_delete` did not verify that `auto_sort_tail` is cleaned when a session is deleted.

**Why this matters:**
If `handle_delete` failed to clean `auto_sort_tail`, stale pane IDs would accumulate and cause incorrect rendering.

**How it was resolved:**
Added `test_handle_delete_cleans_auto_sort_tail` test with two sessions in `auto_sort_tail`, deleting one and verifying only the other remains.

---

### FINDING-8
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/state.rs:229-276
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`remove_dead_sessions()` cleans `auto_sort_tail` (line 268) but had no test coverage for this behavior.

**Why this matters:**
Dead session cleanup is a critical path. Without test coverage, a regression removing the `auto_sort_tail` cleanup would go undetected.

**How it was resolved:**
Added `test_remove_dead_sessions_cleans_auto_sort_tail` test that sets up two sessions in `auto_sort_tail`, marks one as exited, and verifies only the living session remains.

---

### FINDING-9
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/state.rs:303-338
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`cleanup_stale_sessions()` removes auto-paused sessions from `auto_sort_tail` (lines 334-336) but had no test coverage.

**Why this matters:**
Auto-pause is a timer-driven background process. Without test coverage, a regression could leave stale entries in `auto_sort_tail` for sessions that were silently paused.

**How it was resolved:**
Added `test_cleanup_stale_sessions_cleans_auto_sort_tail` test that creates an idle session with `auto_pause_secs = 10` and old timestamp, triggers cleanup, and verifies the session is paused and removed from `auto_sort_tail`.

---

### FINDING-10
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/actions.rs:298-328
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`handle_sort()` clears `auto_sort_tail` (line 327) and excludes paused sessions from `sort_order` when `auto_sort` is enabled (lines 309-311), but neither behavior had test coverage.

**Why this matters:**
The interaction between manual sort (S key) and auto-sort is a key behavioral contract. Without tests, regressions in either the tail clearing or paused exclusion logic would go undetected.

**How it was resolved:**
Added two tests: `test_handle_sort_clears_auto_sort_tail` (verifies tail is emptied after sort) and `test_handle_sort_excludes_paused_when_auto_sort` (verifies paused sessions are excluded from sort_order).
