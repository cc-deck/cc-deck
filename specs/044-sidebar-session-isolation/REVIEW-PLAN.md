# Plan Review: 044-sidebar-session-isolation

**Reviewer**: Claude Code | **Date**: 2026-04-25

## Coverage

All 11 FRs have at least one task. User stories 1-3 each have dedicated phases. No gaps.

## Findings

### Important

1. **FR-007 partial coverage (T017)**: Spec requires checking process liveness first, falling back to 7-day age only if `/proc/` is unavailable. T017 description says "removes files older than 7 days" without mentioning the liveness check. The task should explicitly include both strategies.

2. **FR-011 partial coverage (T021-T022)**: Spec requires migration of `/cache/zellij_pid` (legacy PID file removal). T004 removes the constant but no task explicitly deletes the legacy `zellij_pid` file during migration. T021/T022 only cover `sessions.json` and `session-meta.json`.

### Minor

3. **T014 dependency too strict**: Tests for PID-scoped sync (T014) depend on T010-T013 completing. T014 could be written alongside implementation using TDD. Not blocking, but the dependency arrow is overly sequential.

4. **T015-T016 are test-only tasks**: They verify behavior already implemented in Phase 2. Consider merging them into Phase 2 tasks (T005, T006) to keep tests adjacent to implementation per constitution rules.

5. **Meta file cleanup missing from T017**: T017 mentions only `sessions-*.json`. Spec FR-007 also requires cleanup of `session-meta-*.json` files.

## Verdict

Plan is sound. Fix the two Important items before implementation.
