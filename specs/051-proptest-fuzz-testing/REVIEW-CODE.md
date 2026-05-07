# Code Review: Property-Based Fuzz Testing for Sidebar State Machine

**Spec:** [spec.md](spec.md)
**Date:** 2026-05-07
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 8/8 (100%)
- Error Handling: N/A (test infrastructure, no error handling requirements)
- Edge Cases: Covered via fuzz testing (the feature itself discovers edge cases)
- Non-Functional: 1/1 (100%, performance target met)

## Detailed Review

### Functional Requirements

#### FR-001: FuzzAction covers all input paths
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:22-45`
**Status:** Compliant
**Notes:** 22 variants defined, exceeding the SC-003 target of 18. All keyboard keys, mouse clicks, toggle navigate, and session mutations are covered. F1 key is omitted because it shares the same code path as `?` (`BareKey::Char('?') | BareKey::F(1)` in `handle_navigate_key`), so KeyQuestion provides equivalent coverage.

#### FR-002: Random sequences 1-50 actions, 0-5 initial sessions
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:222-224`
**Status:** Compliant
**Notes:** `initial_session_count in 0..=5usize` and `actions in prop::collection::vec(arb_fuzz_action(), 1..=50)` match spec exactly.

#### FR-003: 5 invariants verified after every action
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:142-212`
**Status:** Compliant
**Notes:** All 5 invariants implemented: cursor bounds (INV-1), filter state consistency (INV-2), passive filter clean (INV-3), selectable matches mode (INV-4), help consistency (INV-5). Called after every action at line 242.

#### FR-004: proptest with prop_oneof!
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:49-73`
**Status:** Compliant
**Notes:** Uses `prop_oneof!` with uniform coverage across all 22 action variants.

#### FR-005: 2000 test cases per run
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:216`
**Status:** Compliant
**Notes:** `ProptestConfig { cases: 2000, .. }` configured correctly.

#### FR-006: Dedicated file in sidebar_plugin module
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs` + `mod.rs:15`
**Status:** Compliant
**Notes:** File exists, declared as `#[cfg(test)] mod fuzz_tests;` in mod.rs.

#### FR-007: Reuse existing test helpers
**Implementation:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:17`
**Status:** Compliant
**Notes:** Imports `bare`, `make_payload`, `make_session` from `test_helpers`.

#### FR-008: Regression seeds stored and evaluated
**Implementation:** `cc-zellij-plugin/proptest-regressions/sidebar_plugin/fuzz_tests.txt` + `fuzz_tests.rs:7-11`
**Status:** Compliant
**Notes:** New seeds at correct path (2 real regression seeds found). Old seeds documented as incompatible in module doc comment (lines 7-11).

### Bug Fixes (Discovered by Fuzz Tests)

The fuzz tests discovered real bugs that required fixes in `input.rs`:

1. **Filter text leaking into RenamePassive** (`input.rs:301-304`): Right-clicking while navigating with an active filter left stale filter text when entering RenamePassive.
2. **Cursor not clamped after filter changes** (`input.rs:495,504`): Backspace and char input in filter mode could leave cursor out of bounds when filter results narrowed.
3. **Filter text not cleared on rename Esc** (`input.rs:560,573`): Escaping from RenamePassive did not clear filter text, violating the passive-filter-clean invariant.

### Extra Features (Not in Spec)

#### Help-mode inner cursor check
**Location:** `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:159-171`
**Description:** INV-1 also checks cursor bounds inside a Help-wrapped navigation mode.
**Assessment:** Helpful addition. Help wraps any mode, so checking the inner nav context is a natural extension of INV-1.
**Recommendation:** Add to spec via evolution if formalizing.

## Code Quality Notes

- Clean separation between action generation, action application, and invariant checking
- Descriptive assertion messages include step number, action, and mode for debugging
- Click region rebuild logic correctly mirrors the render layout (3-row spacing)
- `next_pane_id` counter avoids ID collisions when adding sessions

## Recommendations

### Critical (Must Fix)
- None

### Spec Evolution Candidates
- The Help-inner-cursor check could be formalized as a sub-invariant of INV-1

### Optional Improvements
- None identified

## Conclusion

The implementation achieves 100% compliance with all 8 functional requirements. The fuzz tests discovered and fixed 3 real bugs in the sidebar state machine (filter text leakage and cursor clamping). Two regression seeds are preserved for future runs. The feature is ready for verification.

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 4 source files changed (1 new test file, 1 bug-fix file, 1 config file, 1 module declaration)

### Understanding the changes (8 min)

- Start with `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`: This is the entire new feature, a single 245-line file. Read the `FuzzAction` enum (lines 22-45), then `apply_action` (lines 77-125), then `check_invariants` (lines 142-212), and finally the proptest macro at the bottom (lines 214-245). The structure is action definition, action application, invariant verification, test harness.

- Then `cc-zellij-plugin/src/sidebar_plugin/input.rs` (diff only): Three targeted bug fixes found by the fuzz tests. Each adds a `filter_text.clear()` or `preserve_cursor()` call where state cleanup was missing.

- Question: Does the `apply_action` function faithfully model how the sidebar processes real user input, or does it skip important intermediate state (e.g., render cycles, payload updates)?

### Key decisions that need your eyes (12 min)

**Click region construction** (`fuzz_tests.rs:130-138`, relates to [FR-001](spec.md#fr-001))

The fuzz test builds its own click regions with a header sentinel at row 0 and 3-row spacing per session. This mirrors the render layout but is a separate implementation. If the render layout changes, the fuzz test click regions could diverge silently.
- Question: Should click region construction be extracted into a shared helper used by both render and fuzz tests?

**Session mutation via direct payload modification** (`fuzz_tests.rs:103-124`, relates to [D2 in plan](plan.md#d2-action-application-strategy))

AddSession/RemoveSession modify `cached_payload.sessions` directly and call `preserve_cursor()`. In production, session list changes arrive via `cc-deck:render` pipe messages. The fuzz test skips the pipe message deserialization and event handling path.
- Question: Is testing the state machine in isolation sufficient, or should the test also exercise the `pipe()` method's session update path?

**Invariant 1 extended to Help-wrapped modes** (`fuzz_tests.rs:159-171`)

The cursor bounds check inspects both the top-level mode and, if Help, the inner mode. This goes beyond the spec's explicit requirement but catches a real edge case.
- Question: Is the `Help(inner)` pattern matching exhaustive enough? Could there be other wrapper modes in the future?

**Filter text clearing in RenamePassive entry** (`input.rs:301-304`, relates to acceptance scenario 3/4 in [spec](spec.md))

The fuzz test discovered that right-clicking while in NavigateFilter to rename a session left stale filter text. The fix clears `filter_text` on `enter_rename_passive`.
- Question: Are there other mode transitions that should also clear filter text but currently do not?

### Areas where I'm less certain (5 min)

- `fuzz_tests.rs:98-99`: Mouse click row values are generated as `0..20usize`, but the actual click regions may only have entries for a few rows (depending on session count). Most mouse clicks hit no region and become no-ops. Is the fuzz test getting meaningful mouse coverage, or mostly testing the "click on empty space" path?

- `fuzz_tests.rs:65`: `any::<char>()` generates arbitrary Unicode characters including control characters and multi-byte sequences. The spec mentions Unicode as an edge case. However, if `handle_key` only processes `BareKey::Char(c)`, it is unclear whether Zellij would actually deliver control characters this way. The test might be testing unreachable input paths.

- `fuzz_tests.rs:108`: `make_session(id, &format!("session-{id}"), tab)` uses a sequential tab_index derived from the session list length. In production, tab indices are assigned by Zellij and may have gaps. The fuzz test's sequential model may miss bugs triggered by non-sequential tab indices.

### Deviations and risks (5 min)

- `fuzz_tests.rs:7-11`: The [plan](plan.md#d4-regression-seed-migration) called for evaluating old seeds for migration. The implementation correctly determined they are incompatible and documented the decision in the module doc comment. The old file at `proptest-regressions/fuzz_tests.txt` is preserved. This is an acceptable deviation from "migration" to "documented incompatibility."

- The [plan](plan.md#d1-fuzzaction-enum-design) lists KeyF1 as a variant, but the implementation omits it because F1 shares the same code path as `?` (`BareKey::Char('?') | BareKey::F(1)` in `handle_navigate_key`). KeyQuestion provides equivalent coverage, so a separate KeyF1 variant is unnecessary.

- Risk: The regression seeds file (`proptest-regressions/sidebar_plugin/fuzz_tests.txt`) contains 2 real seeds from bugs found during development. If the `FuzzAction` enum changes shape in the future, these seeds will become invalid. Is there a process to detect and clean stale seeds?

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-07 | **Rounds:** 0/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 0 | completed |
| Architecture & Idioms | 1 (Minor) | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 2 (Minor) | completed |
| CodeRabbit (external) | 5 (2 Critical in specs/, 3 Minor in specs/) | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 4 | 2 | 2 |

Note: CodeRabbit reported 2 "Critical" findings, but both target `specs/tasks.md` (package name in documentation commands), not source code. These are excluded from the gate per review scope rules. The 2 fixed Minor findings were the inaccurate seed path comment in `fuzz_tests.rs:11` and the incorrect F1 handler claim in `REVIEW-CODE.md`.

### What was fixed automatically

Two Minor documentation inaccuracies were corrected during the review:
- Updated the seed file path in the `fuzz_tests.rs` module doc comment from `proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt` to the actual path `proptest-regressions/sidebar_plugin/fuzz_tests.txt`.
- Corrected the REVIEW-CODE.md claim that "no F1 handler exists" to accurately note that F1 shares a code path with `?` and is implicitly covered.

### What still needs human attention

All Critical and Important findings were resolved (none existed in source code). 2 Minor findings remain (see [review-findings.md](review-findings.md) for details). No further review action is needed, but reviewers may want to consider:

- Is the mouse click row range (0..20) providing sufficient coverage of valid session rows, or should generation be weighted toward populated rows?
- Should a KeyF1 variant be added to `FuzzAction` for explicit coverage even though `?` exercises the same code path?

### Recommendation

All findings addressed. Code is ready for human review with no known blockers. The implementation is 100% compliant with the spec, the fuzz tests discovered and fixed 3 real bugs in the sidebar state machine, and no correctness, security, or production readiness issues were found across 5 review perspectives plus CodeRabbit external validation.
