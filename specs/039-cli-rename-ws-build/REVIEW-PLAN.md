# Plan Review: 039-cli-rename-ws-build

**Reviewer**: spex:review-plan
**Date**: 2026-04-18
**Artifacts reviewed**: spec.md, plan.md, tasks.md, research.md, contracts/cli-commands.md

## Coverage Matrix

| Requirement | Plan Section | Task(s) | Status |
|-------------|-------------|---------|--------|
| FR-001: env → ws with workspace alias | Phase 1 | T001, T005 | COVERED |
| FR-002: setup → build | Phase 1, Phase 2 | T003, T007 | COVERED |
| FR-003: config parent (plugin, profile, domains, completion) | Phase 1 | T009, T010 | COVERED |
| FR-004: create → new, delete → kill | Phase 1 | T005, T006 | COVERED |
| FR-005: ls alias for list | Phase 1 | T005 (retains existing alias) | COVERED |
| FR-006: promote attach, ls, exec | Phase 1 | T002, T011 | COVERED |
| FR-007: demote status, start, stop, logs | Phase 1 | T002, T012 | COVERED |
| FR-008: hide hook from help | Phase 1 | T012 | COVERED |
| FR-009: snapshot, version, hook behavior unchanged | Implicit | T011, T012 | COVERED |
| FR-010: preserve operation behavior | All phases | All tasks | COVERED |
| FR-011: no config/state/YAML changes | data-model.md | N/A (constraint) | COVERED |
| FR-012: help text uses new names | Phases 1-4 | T005, T007, T008 | COVERED |
| FR-013: ws subcommand tree complete | Phase 1 | T001, T005 | COVERED |
| SC-001: ws operations work | Phase 3 | T005, T006 | COVERED |
| SC-002: build operations work | Phase 4 | T007, T008 | COVERED |
| SC-003: config operations work | Phase 5 | T009, T010 | COVERED |
| SC-004: help output structure | Phase 6 | T011, T012 | COVERED |
| SC-006: tests pass | Phase 7 | T013-T022 | COVERED |
| SC-007: documentation updated | Phase 8 | T023-T029 | COVERED |
| Edge: old command error handling | Clarifications | N/A (Cobra default) | COVERED |
| Edge: demoted command error | Clarifications | N/A (Cobra default) | COVERED |
| Edge: doc references updated | Phase 8 | T023-T029 | COVERED |

**Coverage**: 21/21 requirements mapped. No gaps.

## Red Flag Scan

### Potential Issues

| # | Flag | Severity | Assessment |
|---|------|----------|------------|
| 1 | T001 is massive (1833-line file rename with ~30 function renames) | Medium | Acceptable for a rename. The task is mechanical (find-replace). Breaking it up would create partial-compile states. Keep as single task but note it is the highest-risk task. |
| 2 | Phase 2 checkpoint says "code does not compile yet" | Low | Intentional. The foundational phase renames files but main.go still imports old names. Phase 6 (T011) fixes main.go. This is a valid approach for a coordinated rename. |
| 3 | T026 bundles 5 files into one task | Low | All 5 files get identical find-replace patterns. Splitting would create 5 near-identical tasks with no benefit. Acceptable. |
| 4 | No task for Makefile/CI updates | None | Verified: Makefile references build targets (`make test`, `make lint`), not command names. No CI files reference `env` or `setup` command strings. No gap. |
| 5 | `cc-deck-setup.yaml` → `cc-deck-build.yaml` rename affects existing user projects | Medium | Users with existing `cc-deck-setup.yaml` files need migration. This is correctly scoped as a rename in T008 but no migration guidance task exists. Since the spec assumes pre-1.0 with no deprecation period, this is acceptable. Add a note in the README about the filename change. |

### No Critical Red Flags Found

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Every task has file paths | PASS | All tasks reference specific files |
| Tasks are independently actionable | PASS | Each task can be executed by an LLM without additional context |
| No circular dependencies | PASS | Linear phase progression with clear ordering |
| Parallel opportunities identified | PASS | 16/29 tasks marked [P] |
| Checkpoints defined | PASS | Every phase has a checkpoint |
| Verification gates exist | PASS | T022 runs `make test` + `make lint` |
| Story labels present | PASS | All story-phase tasks have [US*] labels |

## NFR Validation

| NFR | Coverage | Notes |
|-----|----------|-------|
| No behavioral changes | PASS | FR-010 is a constraint, not a task. Verified by existing test suite (T022). |
| No data/config changes | PASS | FR-011 constraint. data-model.md confirms only manifest filename changes. |
| Constitution IX (docs) | PASS | Phase 8 covers README, cli.adoc, Antora, walkthroughs |
| Constitution X (spec table) | PASS | T024 explicitly includes spec table update |
| Constitution XII (prose plugin) | PASS | Phase 8 header notes prose plugin requirement |

## Recommendations

1. **Add migration note to T024** (README update): mention that `cc-deck-setup.yaml` has been renamed to `cc-deck-build.yaml` for users with existing build manifests.
2. **Consider consolidating T001 and T005**: T001 renames env.go to ws.go and updates function names. T005 updates subcommand Use fields in the same file. Since both touch ws.go, they could be one task. Current split is acceptable for incremental progress, but the implementer should do them back-to-back.

## Verdict

**PASS**: Plan and tasks are ready for implementation. Full spec coverage, no critical red flags, clear dependency ordering, good parallel opportunities. The plan is well-suited for the rename's mechanical nature.

**Suggested approach**: Execute phases sequentially (2 → 3 → 4 → 5 → 6 → 7 → 8), using parallel agents for Phase 7 (9 test tasks) and Phase 8 (7 doc tasks). Total estimated effort: medium (one session for code, one for docs).
