# Plan Review: Environment-to-Workspace Internal Rename

**Date**: 2026-04-21  
**Branch**: `040-env-to-workspace-rename`  
**Reviewer**: Automated plan review

## Coverage Matrix

Maps each spec requirement (FR) and success criterion (SC) to plan tasks.

| Requirement | Description | Tasks | Coverage |
|-------------|-------------|-------|----------|
| FR-001 | Rename Environment-prefixed types to Workspace | T003-T014, T020 | FULL |
| FR-002 | Rename internal/env/ to internal/ws/ | T001, T002 | FULL |
| FR-003 | Update all import paths | T015-T019 | FULL |
| FR-004 | Config file + YAML key rename | T005, T022, T026, T027 | FULL |
| FR-005 | Env var CC_DECK_DEFINITIONS_FILE -> CC_DECK_WORKSPACES_FILE | T023-T025, T028 | FULL |
| FR-006 | Build command descriptions | T030-T033 | FULL |
| FR-007 | Error/log messages | T034-T041 | FULL |
| FR-008 | Tests pass | T021, T029, T042 | FULL |
| FR-009 | Linting passes | T021, T042 | FULL |
| SC-001 | Zero Environment-prefixed types | T043 | FULL |
| SC-002 | make test passes | T021, T029, T042 | FULL |
| SC-003 | make lint passes | T021, T042 | FULL |
| SC-004 | workspaces.yaml with workspaces: key | T005, T022, T026 | FULL |
| SC-005 | Build descriptions use "workspace" | T030-T033 | FULL |
| SC-006 | No "environment" in error messages | T034-T041, T043 | FULL |

**Coverage verdict**: All 9 functional requirements and 6 success criteria are fully covered by tasks.

## Red Flag Scan

| Check | Status | Notes |
|-------|--------|-------|
| Uncovered requirements | PASS | All FRs mapped to tasks |
| Missing verification tasks | PASS | T021, T029, T042, T043 provide verification at each stage |
| Circular dependencies | PASS | Linear phase chain: Setup -> Foundational -> US1 -> US2/US3 -> Polish |
| Over-complex phases | PASS | 6 phases for a mechanical rename is appropriate |
| Missing exclusion guards | PASS | "DO NOT rename" list in research.md, repeated in tasks Phase 2 header |
| Test regression risk | LOW | Tests are updated alongside code; 3 verification checkpoints (T021, T029, T042) |
| Data migration risk | NONE | No backward compatibility per clarification; clean break |
| Constitution violations | PASS | All principles checked; IX (Doc Freshness) explicitly deferred per spec |

## Task Quality Assessment

### Format Compliance

| Check | Result |
|-------|--------|
| All tasks have checkbox `- [ ]` | PASS (43/43) |
| All tasks have sequential ID (T001-T043) | PASS |
| All tasks have file paths | PASS (42/43; T014 uses wildcard `*.go` which is acceptable for cross-reference sweep) |
| User story tasks have [US] labels | PASS (T015-T021=US1, T022-T029=US2, T030-T033=US3) |
| Setup/Foundational tasks have NO story labels | PASS |
| [P] markers only on parallelizable tasks | PASS |

### Task Specificity

| Rating | Count | Examples |
|--------|-------|---------|
| Highly specific (exact files + line numbers) | 28 | T030, T031, T034-T040 |
| Specific (exact files, no line numbers) | 13 | T003-T013, T015-T019 |
| Sweep tasks (wildcard/cross-cutting) | 2 | T014 (cross-refs), T041 (test assertions) |

**Sweep task risk**: T014 and T041 are intentionally broad because they catch residual references after the specific renames. This is correct for a mechanical rename. An implementer should grep for old names after completing the specific tasks.

### Parallelism Assessment

- Phase 2: 12 tasks, all marked [P] (different files within the package)
- Phase 3: 5 of 7 tasks marked [P] (different consumer files)
- Phase 4: 3 of 8 tasks marked [P] (different test files)
- Phase 5: 4 of 4 tasks marked [P] (all different files)
- Phase 6: 8 of 10 tasks marked [P] (different source files)

**Verdict**: Good parallelism. Maximum theoretical speedup ~4x with 4 agents in Phase 2.

## Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Accidental rename of Compose `Environment` field | Medium | Explicitly called out in exclusion list (research.md, tasks Phase 2 header, plan) |
| Missed "environment" string in user-facing output | Low | T043 performs final grep verification |
| Test assertions checking old error message strings fail | Low | T041 updates test assertions; T042 runs full test suite |
| `git mv` doesn't preserve file history cleanly | Low | Cosmetic concern only; `git log --follow` still works |

## Issues Found

### Issue 1: T005 combines structural rename with config rename (Severity: Low)

T005 renames `DefinitionFile.Environments` to `DefinitionFile.Workspaces` with `yaml:"workspaces"` tag. This is both a Go identifier rename (Phase 2 concern) AND a config format change (Phase 4/US2 concern). The YAML tag change is bundled into the foundational phase rather than US2.

**Impact**: Minor. Since there's no backward compatibility, changing the YAML tag early doesn't cause problems. The Phase 4 tasks (T022, T026, T027) handle the corresponding test updates. The only risk is that between Phase 2 and Phase 4 completion, tests with inline `environments:` YAML might fail. However, T020 (Phase 3) updates test files within `internal/ws/`, which should catch this.

**Recommendation**: Acceptable as-is. If preferred, the `yaml:"workspaces"` tag change could be deferred to T022 alongside the filename constant change, but this adds coordination complexity for no practical benefit.

### Issue 2: Missing README spec table update (Severity: Low)

Constitution Principle X requires updating the spec table in README.md. No task covers this.

**Recommendation**: Add to Phase 6 or handle at commit time. Not blocking since it's a one-line table entry.

## Verdict

**APPROVED**: The plan and tasks are well-structured, complete, and ready for implementation. All requirements are covered, task format is compliant, and the phased approach with verification checkpoints is sound. The two minor issues noted above are non-blocking.

## Reviewer Guidance

When reviewing the spec PR, focus on:

1. **Exclusion list correctness**: Verify the "DO NOT rename" list in research.md covers all Docker Compose and OS env var references
2. **YAML tag timing**: T005 changes `yaml:"workspaces"` in Phase 2; confirm test updates in T020/T027 handle this consistently
3. **Error message completeness**: The ~25 error message count is approximate; implementation should grep for completeness
4. **Spec assumptions**: "No backward compatibility" is the key user decision driving simplicity
