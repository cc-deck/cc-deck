# Review Guide: CLI Command Restructuring (#027)

## Quick Summary

This feature restructures the cc-deck CLI by promoting six daily commands (attach, list, status, start, stop, logs) to the top level, removing six legacy K8s commands, and organizing help output into four named groups.

## What to Review

### Spec (spec.md)
- 4 user stories with Given/When/Then acceptance scenarios
- 14 functional requirements, 6 success criteria
- Clear scope boundaries (Out of Scope section)

### Plan (plan.md)
- Constitution check: all 14 principles evaluated
- Source code structure: 25 files deleted, 1 new file, 3 modified
- Research-backed decisions on Cobra groups and RunE sharing

### Tasks (tasks.md)
- 28 tasks across 5 phases
- Phase 1 (US4): K8s removal (foundational, 9 tasks)
- Phase 2 (US1): Command promotion (9 tasks)
- Phase 3 (US2): Help grouping (3 tasks)
- Phase 4 (US3): Compatibility tests (2 tasks)
- Phase 5: Documentation (5 tasks)

### Contract (contracts/command-hierarchy.md)
- Complete command surface definition
- Behavioral contract for promoted commands (output/flag/argument/completion parity)
- Expected help output format

## Key Review Points

1. **Shared constructor pattern** (research.md Decision 2): Is `newXxxCmdCore(gf)` the right approach for dual-path commands? Alternatives considered.

2. **K8s removal scope**: 25 files removed, `k8s.io/client-go` dropped. Verify nothing depends on these packages beyond what was identified.

3. **Help group naming**: "Daily", "Session", "Environment", "Setup". Do these labels communicate clearly?

4. **`profile.go` modification**: K8s Secret validation removed. Profile CRUD stays. Verify this does not break credential workflows.

## Review Checklist

- [ ] Spec requirements are testable and unambiguous
- [ ] Plan covers all spec requirements (see coverage matrix in plan review)
- [ ] Task dependencies are correct
- [ ] Parallel task markers are safe (no same-file conflicts)
- [ ] Constitution compliance is complete
- [ ] Out of Scope items are appropriate
