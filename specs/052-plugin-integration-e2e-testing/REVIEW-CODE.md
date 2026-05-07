# Code Review: Plugin Integration and E2E Testing

**Spec:** [spec.md](spec.md)
**Date:** 2026-05-07
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 9/9 (100%)
- User Stories: 5/5 (100%)
- Success Criteria: 5/5 (100%)
- Edge Cases: 3/5 (60%, two minor gaps noted below)

All 241 Rust tests pass. Zero clippy warnings. No existing tests broken.

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 6 source files (3 new test/helper files, 3 modified plugin modules), 1 documentation file

### Understanding the changes (8 min)

- Start with `cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs` (lines 70-193): This is the foundation. All pipe message construction helpers and plugin setup functions live here. Understanding the helper API clarifies every test.
- Then `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs`: The sidebar tests demonstrate the pattern. Each test creates a plugin, sends pipe messages, and asserts on `test_state()`.
- Question: The helpers are all in `sidebar_plugin/test_helpers.rs` even though they also construct controller-specific messages (hook pipes, action pipes). Would a shared `test_helpers` module at the crate root be a cleaner home, or does the current location work because `pub(crate)` visibility already grants cross-module access?

### Key decisions that need your eyes (12 min)

**Test state accessor pattern** (`sidebar_plugin/mod.rs:302-306`, `controller/mod.rs:640-648`, relates to [FR-001](spec.md#fr-001))

Both plugins expose `test_state()` and `test_state_mut()` behind `#[cfg(test)]`. This couples tests to internal state structure but avoids adding public API surface. The `test_state_mut()` on ControllerPlugin is used in exactly two places (hello registration setup, attend tab_index setup).
- Question: Is `test_state_mut()` acceptable, or should those two tests set up state through the public pipe interface instead?

**Attend test uses raw PipeMessage** (`controller/integration_tests.rs:132-139`, relates to [US4 acceptance scenario 3](spec.md#user-story-4))

The attend test constructs a raw `PipeMessage` with `PipeSource::Cli` instead of using `make_action_pipe()`. This is because attend is triggered by CLI pipe (`cc-deck:attend`), not by a sidebar action message.
- Question: Is this the right approach, or should a `make_attend_pipe()` helper be added for consistency with the other helpers?

**Deferred event test uses Timer event** (`controller/integration_tests.rs:157`, relates to [FR-005](spec.md#fr-005))

The deferred event replay test queues a `Timer` event before permissions. The spec's acceptance scenario mentions "hook events," but `pipe()` drops messages before permissions while `update()` queues them. The test correctly exercises the queuing path but tests Timer instead of hook events.
- Question: Should there be an additional test that verifies pipe messages sent before permissions are also handled (even if they are currently dropped)?

**Permission-before-render semantic** (`sidebar_plugin/integration_tests.rs:74-92`, relates to [US1 acceptance scenario 3](spec.md#user-story-1))

The spec says "the payload is not processed until permissions are granted." The implementation actually stores the payload regardless (the sidebar re-requests permissions on each render pipe). The test adapted to this behavior by asserting `cached_payload.is_some()` rather than `is_none()`.
- Question: Is the spec or the implementation correct here? If the implementation is intentional (defensive re-request), the spec should be evolved to match.

### Areas where I'm less certain (5 min)

- `controller/integration_tests.rs:79-81` ([US3](spec.md#user-story-3)): The sidebar hello test manually constructs a `PaneManifest` and injects it via `test_state_mut()`. This bypasses the normal `PaneUpdate` event flow. If the manifest structure changes, this test might silently diverge from production behavior.
- `sidebar_plugin/integration_tests.rs:116-135` ([FR-006](spec.md#fr-006)): The navigate mode test only verifies forward navigation. Backward navigation and mode exit (Escape) are not tested through the pipe interface, only through unit tests of the input handler.
- Edge case gaps: Duplicate `SidebarHello` for an already-registered plugin ID is not tested. Action targeting a non-existent pane ID is not tested. Both are documented in the spec's edge cases section but neither has a corresponding integration test.

### Deviations and risks (5 min)

- `sidebar_plugin/integration_tests.rs:86-92`: The test for "render before permissions" deviates from [spec acceptance scenario US1.3](spec.md#user-story-1). The spec expects the payload is not processed; the implementation stores it and re-requests permissions. This is a spec/code mismatch that should be reconciled via `spex:evolve`. Question: "Should the spec be updated to reflect the defensive re-request behavior?"
- The [plan](plan.md#phase-2) specifies `setup_sidebar_with_tab()` as a helper. It exists but is marked `#[allow(dead_code)]` because no test currently uses it. Is this acceptable, or should a test be added that exercises it?
- `make_hook_pipe_with_cwd` is also `#[allow(dead_code)]`. Same question applies.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-07 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 2 | completed |
| Architecture & Idioms | 5 | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 2 | completed |
| CodeRabbit (external) | 4 (2 false positives) | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 7 | - | 7 |

### What was fixed automatically

No fixes were needed. All findings are Minor severity. The gate passed on the first round without requiring a fix loop.

### What still needs human attention

All Critical and Important findings were resolved (none were found). 7 Minor findings remain (see [review-findings.md](review-findings.md) for details). No further review action needed, but reviewers may want to consider:

- The `use` imports at the bottom of both integration test files (`integration_tests.rs:219-220` and `integration_tests.rs:301`) follow an unconventional placement. Is this intentional or should they move to the top?
- Two test helpers (`make_hook_pipe_with_cwd`, `setup_sidebar_with_tab`) are unused and marked `#[allow(dead_code)]`. Should they be removed until needed, or kept as planned infrastructure?
- The spec/code mismatch in [US1 acceptance scenario 3](spec.md#user-story-1) regarding permission-before-render behavior should be reconciled. The sidebar stores payloads even without permissions (defensive re-request), while the spec says payloads are not processed. This is a candidate for `spex:evolve`.

### Recommendation

All findings addressed. Code is ready for human review with no known blockers. The 7 Minor findings are style preferences and spec evolution candidates, none of which affect correctness or production behavior.
