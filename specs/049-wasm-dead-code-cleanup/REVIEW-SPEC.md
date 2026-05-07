# Spec Review: WASM Plugin Dead Code Removal and Code Health

**Spec:** specs/049-wasm-dead-code-cleanup/spec.md
**Date:** 2026-05-06
**Reviewer:** Claude (spex:review-spec)

## Overall Assessment

**Status:** SOUND

**Summary:** The spec is well-structured, specific, and directly implementable. All requirements are testable with clear acceptance criteria. The scope is tightly bounded to code removal and reorganization with no functional changes.

## Completeness: 5/5

### Structure
- All required sections present (User Scenarios, Requirements, Success Criteria)
- Assumptions section included with reasonable defaults
- Edge cases explicitly identified

### Coverage
- All 14 functional requirements are specific and verifiable
- Error/edge cases covered (shared helpers, exposed dead code, test coverage gaps)
- 7 measurable success criteria defined
- 3 prioritized user stories with 11 total acceptance scenarios

**Issues:** None.

## Clarity: 5/5

### Language Quality
- All requirements use "MUST" (no ambiguous "should" or "might")
- File names and module names are precise
- No placeholder text or TBD markers

**Ambiguities Found:** None.

## Implementability: 5/5

### Plan Generation
- Can generate a clear implementation plan from this spec
- File locations are precisely identified (specific .rs files, line ranges)
- Dependencies between steps are clear (audit before delete, relocate before delete)
- Scope is manageable (mostly deletions with targeted moves)

**Issues:** None.

## Testability: 5/5

### Verification
- SC-001 through SC-007 are all objectively measurable
- Line counts, file existence checks, grep for banned identifiers, cargo test/clippy output
- Binary size measurement is documented as informational (no hard target, which is correct given LTO uncertainty)

**Issues:** None.

## Constitution Alignment

- **Principle I (Tests and documentation):** This is a refactoring with no user-facing changes, so no README/CLI/Antora documentation updates are needed. The spec correctly requires porting unique test scenarios before deleting legacy tests (FR-005, FR-006). No new commands, flags, or config options are introduced.
- **Principle II (Interface contracts):** Not applicable. No new interface implementations.
- **Principle III (Build and tool rules):** The spec references `cargo test` and `cargo clippy` in success criteria. Per constitution, these should use `make test` and `make lint` instead.

**Violations:**
- SC-004 and SC-005 reference `cargo test` and `cargo clippy` directly. The constitution requires using `make test` and `make lint`. This is a minor wording issue in the spec; the intent is correct. Recommend updating the success criteria to reference the Makefile targets.

## Recommendations

### Important (Should Fix)
- [ ] Update SC-004 to reference `make test` instead of `cargo test` (constitution principle III)
- [ ] Update SC-005 to reference `make lint` instead of `cargo clippy` (constitution principle III)

### Optional (Nice to Have)
- [ ] Consider adding a success criterion for compile-time improvement (before/after build time measurement), since removing 4,500 lines of dead code should noticeably speed up compilation

## Conclusion

The spec is sound and nearly ready for implementation. The only issue is a minor constitution alignment point: success criteria should reference `make test`/`make lint` rather than direct cargo commands. This does not affect implementability.

**Ready for implementation:** Yes, after minor wording fixes.

**Next steps:**
1. Fix the two success criteria references (cargo -> make)
2. Proceed to `/speckit-plan`
