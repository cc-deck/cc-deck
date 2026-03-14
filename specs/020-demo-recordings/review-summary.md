# Review Summary: 020-demo-recordings

**Reviewed**: 2026-03-14
**Artifacts**: spec.md, plan.md, tasks.md, research.md, data-model.md, contracts/pipe-commands.md, quickstart.md

## Coverage Matrix

| Spec Requirement | Plan Section | Task(s) | Status |
|-----------------|-------------|---------|--------|
| FR-001: Pipe message handlers | Project Structure, Phase 2 | T004-T007 | Covered |
| FR-002: Scene management helpers | Project Structure (runner.sh) | T003, T014, T015 | Covered |
| FR-003: Demo project setup/teardown | Phase 3 | T011, T012 | Covered |
| FR-004: Git repos with CLAUDE.md | Phase 3 | T008-T010 | Covered |
| FR-005: Recording tool integration | Phase 4 | T014 | Covered |
| FR-006: Voiceover generation | Phase 6 | T023 | Covered |
| FR-007: Output format conversion | Phase 7 | T025-T028 | Covered |
| FR-008: No OS-level key simulation | Research R1, Constraint | Validated by contract | Covered |
| FR-009: Checkpoint-based timing | Phase 4 | T015 | Covered |
| FR-010: Image builder manifest | Phase 5 | T020 | Covered |
| US1: Scripted plugin demo | Phase 4 | T013-T017 | Covered |
| US2: Pipe-based plugin control | Phase 2 | T004-T007 | Covered |
| US3: Demo projects | Phase 3 | T008-T012 | Covered |
| US4: Voiceover audio | Phase 6 | T021-T024 | Covered |
| US5: Multiple output formats | Phase 7 | T025-T028 | Covered |
| SC-001: Single-command recording | Quickstart | T016 (Makefile) | Covered |
| SC-002: Consistent recordings | Research R1 (pipe-only) | T013 (scripted) | Covered |
| SC-003: Under 60s landing clip | Phase 7 | T025 | Covered |
| SC-004: Zero key simulation | Constraint, Research R1 | Contract validation | Covered |
| SC-005: Setup < 30s | Phase 3 checkpoint | T011 | Covered |
| SC-006: Pipeline < 15 min | Performance goal | Not explicitly tested | Minor gap |
| Edge: Claude Code timeout | Spec edge cases | T015 (wait_for) | Partially covered |
| Edge: Zellij crash recovery | Spec edge cases | Not covered | Minor gap |
| Edge: TTS API unavailable | Spec edge cases | Not covered | Minor gap |

**Coverage**: 22/25 items fully covered, 3 minor gaps (edge cases, performance validation).

## Red Flags

None. The plan is well-structured with clear phase dependencies and parallel opportunities.

## Task Quality

| Criterion | Assessment |
|-----------|-----------|
| Exact file paths | All tasks specify target files |
| Testable completion | Checkpoints at each phase boundary |
| Dependency ordering | Correct: Phase 2 blocks Phase 4, Phase 3 blocks Phase 4 |
| Parallel markers | Present and accurate ([P] on independent tasks) |
| Size consistency | Tasks are appropriately scoped (single file or logical unit) |
| Story traceability | All tasks tagged with [US*] where applicable |

## Observations

1. **Strong research foundation**: The 4-agent research phase produced concrete findings with tested tool versions. The pipe API research identified exact file locations and code patterns.

2. **Constitution compliance**: All 9 gates pass. The plan correctly identifies that Makefile targets are needed (Principle VI) and that README must be updated (Principle VIII).

3. **Incremental delivery**: The MVP strategy (Phases 1-4) produces a usable demo recording before tackling voiceover and format conversion. Good prioritization.

4. **Minor gaps to address during implementation**:
   - SC-006 (pipeline under 15 min) has no explicit validation task. Add a timing check to T017 or T031.
   - Edge cases for Zellij crash recovery and TTS fallback are mentioned in the spec but not explicitly tasked. These are P3 and acceptable to defer.

## Recommendation

**APPROVE**. The spec, plan, and tasks are complete, consistent, and ready for implementation. The minor gaps are in edge case handling (P3) and do not block the MVP.

## Reviewer Guidance

When reviewing this spec PR, focus on:

1. **Pipe command contract** (`contracts/pipe-commands.md`): Verify the command names and behavior match the existing cc-deck pipe namespace conventions.
2. **Demo project design** (`data-model.md`): Confirm the three projects (Python/Go/HTML) provide sufficient variety and that CLAUDE.md tasks are completable in under 2 minutes.
3. **Recording toolchain** (`research.md` R4): Verify tool versions match what is available in the development and CI environments.
4. **Phase dependencies** (`tasks.md`): Confirm Phase 2 (pipe handlers) must complete before Phase 4 (demo scripts), and that Phases 6+7 can run in parallel.
