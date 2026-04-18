# Plan Review: 039-cli-rename-ws-build

**Reviewer**: spex:review-plan
**Date**: 2026-04-18
**Artifacts reviewed**: spec.md, plan.md, tasks.md, research.md, contracts/cli-commands.md, data-model.md, checklists/requirements.md

## Coverage Matrix

| Requirement | Plan Section | Task(s) | Status |
|-------------|-------------|---------|--------|
| FR-001: env -> ws with workspace alias | Phase 1 | T001, T005 | COVERED |
| FR-002: setup -> build | Phase 1, Phase 2 | T003, T007 | COVERED |
| FR-003: config parent (plugin, profile, domains, completion) | Phase 1 | T009, T010 | COVERED |
| FR-004: create -> new, delete -> kill | Phase 1 | T005, T006 | COVERED |
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

**Coverage**: 22/22 requirements mapped. No gaps.

## Red Flag Scan

### Issues Found

| # | Flag | Severity | Assessment |
|---|------|----------|------------|
| 1 | **Missing: template file rename** | **HIGH** | `cc-deck/internal/setup/templates/cc-deck-setup.yaml.tmpl` must be renamed to `cc-deck-build.yaml.tmpl`. Neither T004 nor T008 mentions this file. The `embed.go` file references it via `ReadFile("templates/cc-deck-setup.yaml.tmpl")`. This will cause a build failure if not addressed. |
| 2 | **Missing: embedded command docs** | **MEDIUM** | `cc-deck/internal/setup/commands/cc-deck.build.md` and `cc-deck.capture.md` contain ~15 references to `cc-deck-setup.yaml`. These are embedded Go files used for Claude Code commands. Not mentioned in any task. |
| 3 | **Missing: shell scripts** | **MEDIUM** | `cc-deck/internal/setup/scripts/validate-manifest.sh` and `update-manifest.sh` reference `cc-deck-setup.yaml` (~5 occurrences total). Not mentioned in any task. |
| 4 | **Missing: embed.go update** | **MEDIUM** | `cc-deck/internal/setup/embed.go` has `ReadFile("templates/cc-deck-setup.yaml.tmpl")` which must update to the new template filename. Not explicitly mentioned in T004. |
| 5 | T001 is massive (1833-line file rename with ~30 function renames) | Low | Acceptable for a rename. The task is mechanical (find-replace). Breaking it up would create partial-compile states. Keep as single task but note it is the highest-risk task. |
| 6 | Phase 2 checkpoint says "code does not compile yet" | Low | Intentional. The foundational phase renames files but main.go still imports old names. Phase 6 (T011) fixes main.go. This is a valid approach for a coordinated rename. |
| 7 | T026 bundles 5 files into one task | Low | All 5 files get identical find-replace patterns. Splitting would create 5 near-identical tasks with no benefit. Acceptable. |
| 8 | `cc-deck-setup.yaml` -> `cc-deck-build.yaml` affects existing user projects | Low | Users with existing `cc-deck-setup.yaml` files need migration. Since the spec assumes pre-1.0 with no deprecation period, this is acceptable. Add a note in the README about the filename change. |

### Critical Finding: Incomplete `internal/setup/` Directory Scope

The plan's T004 says "Rename directory `internal/setup/` to `internal/build/`" and "update all import paths" but only explicitly mentions `init.go`, `manifest.go`, and their test files. The actual directory contains additional files that need updating:

- `embed.go` (template filename reference)
- `runtime.go` (unknown references, needs checking)
- `templates/cc-deck-setup.yaml.tmpl` (filename rename)
- `commands/cc-deck.build.md` (~15 manifest filename refs)
- `commands/cc-deck.capture.md` (~3 manifest filename refs)
- `scripts/validate-manifest.sh` (~2 refs)
- `scripts/update-manifest.sh` (~3 refs)

**Recommendation**: Expand T004 to explicitly list all files in `internal/setup/` or add a new task T004b covering the template, commands, scripts, and embed.go files.

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Every task has file paths | PASS | All tasks reference specific files |
| Tasks are independently actionable | PASS | Each task can be executed without additional context |
| No circular dependencies | PASS | Linear phase progression with clear ordering |
| Parallel opportunities identified | PASS | 16/29 tasks marked [P] |
| Checkpoints defined | PASS | Every phase has a checkpoint |
| Verification gates exist | PASS | T022 runs `make test` + `make lint` |
| Story labels present | PASS | All story-phase tasks have [US*] labels |
| All affected files enumerated | **FAIL** | Missing template, commands, scripts, embed.go (see red flags 1-4) |

## NFR Validation

| NFR | Coverage | Notes |
|-----|----------|-------|
| No behavioral changes | PASS | FR-010 is a constraint, verified by existing test suite (T022) |
| No data/config changes | PASS | FR-011 constraint. data-model.md confirms only manifest filename changes |
| Constitution IX (docs) | PASS | Phase 8 covers README, cli.adoc, Antora, walkthroughs |
| Constitution X (spec table) | PASS | T024 explicitly includes spec table update |
| Constitution XII (prose plugin) | PASS | Phase 8 header notes prose plugin requirement |

## Recommendations

1. ~~REQUIRED: Expand T004 scope~~ **FIXED**: T004 now covers all files in `internal/setup/` including embed.go, templates, commands, and scripts.
2. ~~Add migration note to T024~~ **FIXED**: T024 now includes migration note for `cc-deck-setup.yaml` rename.
3. **Consider consolidating T001 and T005**: Both touch ws.go. Current split is acceptable for incremental progress, but the implementer should do them back-to-back.
4. ~~Domain smoke test~~ **FIXED**: Added T021b covering `cc-deck/tests/domain-smoke-test.sh`.

## Verdict

**PASS**: All blocking issues have been resolved. T004 now enumerates all files in `internal/setup/` (template, embed.go, commands, scripts). T024 includes the migration note. T021b covers the domain smoke test.

Plan is ready for implementation. Execute phases sequentially (2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8), using parallel agents for Phase 7 (9 test tasks) and Phase 8 (7 doc tasks). Total estimated effort: medium (one session for code, one for docs).
