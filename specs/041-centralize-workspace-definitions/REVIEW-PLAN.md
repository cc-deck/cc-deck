# Plan Review: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Reviewed**: 2026-04-22
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Tasks**: [tasks.md](tasks.md)

## Coverage Matrix

Every functional requirement and success criterion must map to at least one task.

| Requirement | Description | Task(s) | Status |
|-------------|-------------|---------|--------|
| FR-001 | Central store at workspaces.yaml | 1.2, 1.4 | Covered |
| FR-002 | Template format with type-keyed variants | 1.1, 1.4 | Covered |
| FR-003 | Placeholder syntax with defaults | 1.1, 1.4 | Covered |
| FR-004 | Templates support all definition fields + flag override | 1.1, 1.4 | Covered |
| FR-005 | Default name from template or directory basename | 1.1, 1.4 | Covered |
| FR-006 | Two-phase default resolution (project-dir ancestor, then global) | 1.3 | Covered |
| FR-007 | Print "Using workspace X" on auto-resolve | 1.3 | Covered |
| FR-008 | Name collision: same-type error, different-type auto-suffix | 1.2, 1.4 | Covered |
| FR-009 | Explicit name overrides template; collision still applies | 1.4 | Covered |
| FR-010 | PROJECT column replaces SOURCE in ws list | 1.5 | Covered |
| FR-011 | ws update --sync-repos reads from central store | 1.6 | Covered |
| FR-012 | Remove project registry from state.yaml | 2.1 | Covered |
| FR-013 | Remove --global and --local flags | 1.4 | Covered |
| FR-014 | Rename FindProjectConfig to FindProjectRoot | 2.2 | Covered |
| FR-015 | Remove legacy functions | 2.1 | Covered |
| FR-016 | Remove ProjectEntry type | 2.1 | Covered |
| FR-017 | Remove ProjectStatusStore and ProjectStatusFile | 2.1 | Covered |
| FR-018 | Stop creating/reading .cc-deck/status.yaml | 2.1, 2.3 | Covered |
| SC-001 | All definitions in single file | 3.2 | Covered |
| SC-002 | Template + placeholder in single command | 3.2 | Covered |
| SC-003 | ws attach selects correct workspace | 3.2 | Covered |
| SC-004 | PROJECT column correct | 3.2 | Covered |
| SC-005 | make test + make lint pass | 3.2 | Covered |
| SC-006 | Zero references to removed code | 3.2 | Covered |

**Coverage**: 18/18 FRs covered, 6/6 SCs covered. **No gaps.**

## Red Flag Scan

| # | Flag | Severity | Task | Assessment |
|---|------|----------|------|------------|
| 1 | Task 1.4 is large (~500 lines of ws new rewrite) | Medium | 1.4 | Acceptable: the rewrite is guided by clear acceptance scenarios and the existing code structure is well-understood from research. The task preserves most type-specific code and only rewrites the definition resolution and storage paths. |
| 2 | Task 1.3 and 1.4 have ordering dependency | Low | 1.3, 1.4 | Both modify `ws.go` but touch different functions (`resolveWorkspaceName` vs `runWsNew`). Can be done sequentially within the same phase. |
| 3 | Phase 2 cleanup removes code that Phase 1 stops using | Low | 2.1 | Correct ordering: Phase 1 stops calling the functions, Phase 2 deletes them. Compile errors in Phase 2 will catch any missed callers. |
| 4 | Compose `ProjectDir` semantics preserved? | Medium | 1.4 | The plan explicitly notes "Preserve: all type-specific option setting." The compose code derives paths from `ProjectDir`, and since the value remains the project root directory, compose path derivation is unchanged. Tests should verify. |
| 5 | No explicit task for `resolveWorkspace` function update | Low | 1.3 | `resolveWorkspace` (lines 1936-1964) currently searches project definitions via `ListProjects`. After Phase 2 removal of `ListProjects`, this code path must be updated to use `DefinitionStore` only. Task 2.1 covers this via "Fix all compile errors from removed types/functions." |

**No blocking red flags found.**

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Each task has clear acceptance criteria | Pass | All tasks reference specific FR/SC codes |
| Tasks are independently testable | Pass | Each task specifies test scenarios |
| File changes are explicit | Pass | All tasks list files to create/modify/delete |
| Dependencies are clear | Pass | Phase ordering (1 before 2 before 3) is explicit |
| Task granularity is appropriate | Pass | Smallest tasks (2.3, 1.6) are focused; largest (1.4) is justified by scope |
| No orphan requirements | Pass | All FRs and SCs mapped in coverage matrix |
| Constitution compliance checked | Pass | Plan includes full constitution check table |
| Research decisions documented | Pass | research.md covers 6 decisions with alternatives |

## Recommendations

1. **Task 1.4 could be split** into "template integration" and "flag/scaffolding removal" sub-tasks if the implementer prefers smaller commits. Not required.

2. **Task 2.1 should be done in one commit** to avoid intermediate compile failures from partially removed types.

3. **Consider adding a regression test** in Task 1.4 that verifies compose workspaces still derive correct paths from `project-dir` after the semantic repurposing.

## Verdict

**Ready for implementation.** All requirements covered, no blocking issues, task quality is good. The plan follows a clean bottom-up ordering (data layer first, command layer second, cleanup third) that minimizes risk.

**Suggested next step**: `/speckit-implement` (or create a spec PR first if review is desired before implementation)
