# Review Summary: Session Save and Restore

**Feature**: 015-session-save-restore
**Date**: 2026-03-10
**Status**: Ready for implementation

## Coverage Matrix

| Spec Requirement | Plan Section | Tasks | Status |
|-----------------|-------------|-------|--------|
| FR-001: save command | Source Code layout | T002, T008, T010, T012 | Covered |
| FR-002: restore command | Source Code layout | T008, T011, T012 | Covered |
| FR-003: list command | Source Code layout | T008, T015, T016 | Covered |
| FR-004: remove command | Source Code layout | T008, T018, T019 | Covered |
| FR-005: query plugin via pipe | Research R1, R6 | T003, T004, T005, T007 | Covered |
| FR-006: create tabs + resume | Research R5 | T011 | Covered |
| FR-007: progress output | Spec acceptance scenario 1.4 | T011 | Covered |
| FR-008: auto-save with cooldown | Research R2 | T013, T014 | Covered |
| FR-009: XDG config directory | Research R4 | T006 | Covered |
| FR-010: restore latest | Spec acceptance scenario 2.3 | T017 | Covered |
| FR-011: named saves persist | Data model lifecycle | T006 (auto_save flag) | Covered |
| FR-012: atomic writes | Data model state transitions | T006, T013 | Covered |

## Red Flags

None identified. The plan is straightforward and follows established project patterns.

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Each task has file path | PASS | All tasks specify exact file paths |
| Tasks are independently executable | PASS | Clear task boundaries, no ambiguity |
| Dependencies are explicit | PASS | Phase ordering + beads dependencies (13 mapped) |
| Story labels present | PASS | US1-US4 labels on all story-phase tasks |
| Parallel markers accurate | PASS | [P] only on tasks with no file conflicts |
| Checkpoint validation points | PASS | Each phase has a checkpoint description |

## Plan Strengths

- Clean two-component split: Rust plugin handles state dump, Go CLI handles file I/O and Zellij actions
- Reuses existing patterns (cobra command groups, XDG paths, pipe protocol)
- `cli_pipe_output` / `unblock_cli_pipe_input` verified in Zellij source code (constitution principle V)
- MVP-first strategy: US1 (save/restore) delivers core value independently
- All 4 user stories are independent after Phase 2

## Recommendations

1. **Start with MVP**: Phase 1 + 2 + 3 (T001-T012) delivers the core save/restore capability
2. **Auto-save (US2) next**: Highest safety-net value with just 2 tasks (T013-T014)
3. **T004 is the critical path**: The dump-state pipe handler is the foundation everything depends on. Verify `cli_pipe_output` behavior with a manual test before building the full CLI.

## Reviewer Guidance

When reviewing this spec/plan:
- Check `contracts/pipe-protocol.md` for the dump-state message format
- Check `data-model.md` for the snapshot JSON schema
- Key architectural decision: state dump via Zellij pipe (bidirectional CLI communication) rather than WASI filesystem or run_command (see research.md R1)
- The 5-minute auto-save cooldown was a clarification decision (see spec Clarifications section)
