# Plan Review: Workspace Lifecycle Redesign

**Date**: 2026-04-25
**Branch**: 043-workspace-lifecycle
**Reviewer**: Automated plan review

## Coverage Matrix

| Spec Requirement | Plan Phase | Task(s) | Status |
|-----------------|------------|---------|--------|
| FR-001 (Workspace interface with Attach, KillSession, Delete, Status) | Phase 2 | T001, T003 | Covered |
| FR-002 (InfraManager interface with Start, Stop) | Phase 2 | T002 | Covered |
| FR-003 (Attach lazy-starts infrastructure) | Phase 5 (US3) | T018, T019, T020 | Covered |
| FR-004 (Attach creates session with layout) | Phase 3 (US1) | T009 | Covered |
| FR-005 (Attach reattaches to existing session) | Phase 3 (US1) | T009 | Covered |
| FR-006 (KillSession kills only session) | Phase 4 (US2) | T013-T016 | Covered |
| FR-007 (Stop kills session first, then infra) | Phase 6 (US4) | T022-T024 | Covered |
| FR-008 (Start starts infra only) | Phase 6 (US4) | T022-T024 | Covered |
| FR-009 (Warning for non-InfraManager start/stop) | Phase 6 (US4) | T026, T027 | Covered |
| FR-010 (Two-dimensional state model) | Phase 2 | T004, T005 | Covered |
| FR-011 (InfraState null for non-InfraManager) | Phase 2 | T005 | Covered |
| FR-012 (SessionState: none/exists) | Phase 2 | T004 | Covered |
| FR-013 (Independent reconciliation) | Phase 3,7 (US1,US5) | T011, T034 | Covered |
| FR-014 (Type-appropriate state display) | Phase 7 (US5) | T032, T033 | Covered |
| FR-015 (Delete kills session + stops infra + cleanup) | Phase 3 (US1) | T012 | Covered |
| FR-016 (Remove Start/Stop from Workspace interface) | Phase 2,3 | T001, T008 | Covered |
| FR-017 (New ws kill-session CLI command) | Phase 4 (US2) | T017 | Covered |
| FR-018 (State file migration v2 -> v3) | Phase 2 | T006, T007 | Covered |

**Coverage**: 18/18 requirements covered (100%)

## Red Flags

### Flag 1: Compile-Breaking Intermediate State (LOW RISK)

The plan acknowledges that after Phase 2 (interface changes), the code will not compile until workspace types are updated. The "Recommended Approach" section in tasks.md correctly suggests doing Phase 2 + all type updates together. This is well-handled.

**Mitigation**: The implementation notes explicitly call this out and suggest combining US1+US2+US4 with the foundational phase to reach a compilable state quickly.

### Flag 2: Task Overlap Between Stories (LOW RISK)

Some tasks touch the same files across different user stories (e.g., container.go is modified in US2, US3, US4, and US5). This means the "parallel by story" model from the template is less applicable here. In practice, these will likely be done sequentially per file.

**Mitigation**: The recommended approach in tasks.md already acknowledges this and suggests combining related changes.

### Flag 3: Delete Behavior Update Only in US1 (INFO)

FR-015 (Delete kills session + stops infra) is only covered by T012 (local.go Delete). Container, compose, ssh, and k8s-deploy Delete methods also need updating to call KillSession() instead of inline session kills.

**Recommendation**: Add explicit delete-update tasks for container, compose, ssh, and k8s-deploy, or expand T012 to cover all types.

## Task Quality

| Criterion | Status | Notes |
|-----------|--------|-------|
| All tasks have IDs | PASS | T001-T040 sequential |
| All tasks have file paths | PASS | Every task names specific files |
| Story labels on story tasks | PASS | US1-US5 correctly applied |
| No story labels on foundational/polish tasks | PASS | Phase 2 and 8 correctly unlabeled |
| [P] markers only on truly parallelizable tasks | PASS | Different files, no shared state |
| Checkpoints after each phase | PASS | Every phase has a checkpoint |
| Dependencies documented | PASS | Phase and story dependencies clear |

## Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| VI. Build via Makefile | PASS | Plan uses make test, make lint |
| VII. Interface Contracts | PASS | Behavioral contract written at contracts/workspace-interface.md |
| VIII. Simplicity | PASS | Two interfaces justified, no over-engineering |
| IX. Documentation Freshness | PASS | Tasks T035-T037 cover README, CLI ref, user guide |
| X. Spec Tracking | PASS | T035 includes README spec table update |
| XII. Prose Plugin | NOTED | Documentation tasks should use prose plugin (mentioned in Notes) |

## Recommendations

1. **Add Delete update tasks for non-local types** (Flag 3): T012 only covers local.go. Add tasks or expand scope to cover container, compose, ssh, k8s-deploy Delete methods.
2. **Consider batching Phase 2 + US1 + US4**: As the recommended approach suggests, do the interface split and all type implementations together to avoid a non-compilable state.
3. **Manual testing plan**: Phase 3 (CLI) verification says "manual testing." Consider writing a brief test script or checklist for the manual verification steps.

## Verdict

**READY FOR IMPLEMENTATION** with one minor gap (Delete coverage for non-local types). The plan is well-structured, has full requirement coverage, and the implementation strategy section correctly identifies the practical order of work.
