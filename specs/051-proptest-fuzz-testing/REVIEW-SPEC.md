# Spec Review: Property-Based Fuzz Testing for Sidebar State Machine

**Spec:** specs/051-proptest-fuzz-testing/spec.md
**Date:** 2026-05-07
**Reviewer:** Claude (spex:review-spec)

## Overall Assessment

**Status:** SOUND

**Summary:** The specification is thorough, well-scoped, and directly implementable. All mandatory sections are complete with concrete, testable requirements. Minor constitution alignment notes below.

## Completeness: 5/5

### Structure
- All required sections present (User Scenarios, Requirements, Success Criteria, Assumptions)
- Edge cases enumerated with 5 specific scenarios
- Key entities clearly defined

### Coverage
- All 8 functional requirements are specific and numbered
- 5 measurable success criteria defined
- Scope boundaries explicit (sidebar-only, proptest-only)
- Open questions from brainstorm all resolved in Assumptions

**Issues:** None

## Clarity: 5/5

### Language Quality
- Requirements use "MUST" consistently (no weak "should"/"might")
- Invariants are precisely defined with formulas (e.g., `cursor_index < max(1, filtered_sessions.len())`)
- No vague terms or placeholders

**Ambiguities Found:** None

## Implementability: 5/5

### Plan Generation
- File location specified (sidebar_plugin module, fuzz_tests.rs)
- All 7 SidebarMode variants and their transitions are well-understood from the codebase
- Test helper reuse specified (FR-007)
- proptest configuration parameters concrete (2000 cases, 1-50 sequence length)
- WASM stubs already exist for non-WASM test builds

**Issues:** None

## Testability: 5/5

### Verification
- SC-001: Run 2000 cases without violations (pass/fail)
- SC-002: Timing measurement (<10s)
- SC-003: Count FuzzAction variants (>=18)
- SC-004: Count invariant checks (>=5)
- SC-005: Existing test suite regression check

**Issues:** None

## Constitution Alignment

- **Principle I (tests + docs)**: This feature IS test infrastructure. No user-facing changes, so README/CLI reference/Antora docs updates are not applicable. No new config options or file locations introduced.
- **Principle III (build rules)**: The spec references `cargo test fuzz` in acceptance scenarios. Implementation should use `make test` per constitution. This is a spec-level phrasing choice (describing what conceptually runs), not an implementation directive; the plan phase should ensure `make test` is used.

**Violations:** None (minor phrasing note above is informational, not a violation)

## Recommendations

### Critical (Must Fix Before Implementation)
(none)

### Important (Should Fix)
(none)

### Optional (Nice to Have)
- [ ] Consider adding a 6th invariant: when in NavigateDeleteConfirm, the stored pane_id should reference a session that exists in the current list (or was recently removed). This would catch stale delete confirmations.

## Conclusion

The spec is sound and ready for implementation planning. Requirements are concrete, testable, and well-scoped. The feature builds on proven prior art (previous proptest suite that found real bugs) with a clear target (the current SidebarMode state machine).

**Ready for implementation:** Yes

**Next steps:** Proceed to `/speckit-plan`
