# Deep Review Findings

**Date:** 2026-05-07
**Branch:** 051-proptest-fuzz-testing
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 4 | - | 4 |
| **Total** | **4** | **0** | **4** |

**Agents completed:** 5/5 (+ 1 external tool: CodeRabbit)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:11
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (cosmetic, not blocking)

**What is wrong:**
The module doc comment states that new seeds will be written to `proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt`, but proptest actually writes them to `proptest-regressions/sidebar_plugin/fuzz_tests.txt` (based on the module path, not the test function name within it).

**Why this matters:**
A developer reading the comment and looking for seeds at the documented path would not find them. Low impact since proptest manages seeds automatically, but the comment is misleading.

**How it could be resolved:**
Update line 11 to reference `proptest-regressions/sidebar_plugin/fuzz_tests.txt` instead of `proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt`.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:22-45
- **Category:** test-quality
- **Source:** test-quality-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** remaining (covered implicitly)

**What is wrong:**
The `FuzzAction` enum omits `BareKey::F(1)` (F1 key), which IS handled in `handle_navigate_key` (line sharing the same match arm as `BareKey::Char('?')`). The fuzz test does not have a KeyF1 variant.

**Why this matters:**
Low impact. The `?` key and F1 share the exact same code path (`state.mode.toggle_help()`), so the KeyQuestion variant provides equivalent coverage. Adding a KeyF1 variant would not exercise any new code.

**How it could be resolved:**
Optionally add a `KeyF1` variant to `FuzzAction` for completeness, mapping to `bare(BareKey::F(1))`. Not required since coverage is already implicit.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs:68-69
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (acceptable trade-off)

**What is wrong:**
Mouse click row values are generated from `0..20usize`, but with at most 5 initial sessions and 3-row spacing, valid session rows are only at positions 2, 5, 8, 11, 14. Most generated clicks hit empty space and become no-ops in the mouse handler.

**Why this matters:**
Mouse interaction testing effectiveness is diluted. The fuzz test is mostly verifying that out-of-range clicks are handled gracefully rather than testing actual session selection via clicks.

**How it could be resolved:**
Optionally weight mouse click generation toward valid session rows based on the current session count. The current approach is still valid since graceful handling of out-of-range clicks is valuable, and the 2000-case budget provides enough samples to occasionally hit valid rows.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 80
- **File:** specs/051-proptest-fuzz-testing/REVIEW-CODE.md:23
- **Category:** external
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining (documentation, not source code)

**What is wrong:**
The REVIEW-CODE.md note states "F1 key is correctly omitted as the codebase has no F1 handler," but `handle_navigate_key` in input.rs does handle `BareKey::F(1)` in the same match arm as `BareKey::Char('?')`.

**Why this matters:**
The review documentation contains an inaccurate claim about the codebase. A reviewer trusting this note would have a wrong mental model.

**How it could be resolved:**
Update the REVIEW-CODE.md note to say F1 is handled via the same code path as `?` and that omitting KeyF1 is acceptable because coverage is implicit.

**External tool analysis (CodeRabbit):**
> CodeRabbit identified this inaccuracy and correctly noted that `BareKey::F(1)` shares a match arm with `BareKey::Char('?')` in `handle_navigate_key`, calling `state.mode.toggle_help()`.

## Remaining Findings

All 4 remaining findings are Minor severity. No Critical or Important findings were identified by any agent or external tool. The Minor findings are cosmetic (comment inaccuracy) or represent acceptable trade-offs in test coverage (mouse row range, implicit F1 coverage).
