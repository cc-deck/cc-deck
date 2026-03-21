# Review Guide: 025-sidebar-state-refresh

## Summary

This spec adds a startup grace period to the cc-deck Zellij plugin to prevent a race condition where restored cached sessions are immediately wiped by `remove_dead_sessions()` before the pane manifest fully populates after reattach.

## Review Focus Areas

### 1. Race Condition Fix (Primary)

**Files**: `spec.md` (FR-001, FR-002), `research.md` (R1, R2), `data-model.md`

- Verify the root cause analysis is correct: PaneUpdate delivers incomplete manifests during startup
- Evaluate whether a 3-second grace period is appropriate (research.md R1)
- Check that the grace period starts at the right point (permission grant, not load)
- Confirm that the existing empty-manifest guard at `state.rs:167-171` is correctly identified as insufficient

### 2. Regression Safety

**Files**: `spec.md` (FR-003 through FR-005), `tasks.md` (T006, T008)

- Verify that no existing behavior is modified outside the grace period
- Check that session persistence, sync protocol, and metadata sync are explicitly preserved
- Confirm the fresh-start scenario (no cache) is tested

### 3. Simplicity

**Files**: `plan.md` (Constitution Check VIII), `data-model.md`

- Verify that the solution is truly minimal (one field, one guard)
- No new cache files, pipe messages, configuration options, or abstractions
- No changes to session.rs, sync.rs, sidebar.rs, config.rs

### 4. Task Completeness

**Files**: `tasks.md`

- 11 tasks total, 5 for MVP (T001-T005)
- Each user story has clear independent test criteria
- Unit tests cover: grace period active, grace period expired, empty cache, stale cleanup
- Live validation via quickstart.md (T010)

## Key Decisions to Validate

| Decision | Location | Question |
|----------|----------|----------|
| 3-second grace period | research.md R1 | Is 3s the right balance between reliability and responsiveness? |
| Time-based vs event-count | research.md R2 | Is wall-clock time simpler than counting PaneUpdate events? |
| Only `remove_dead_sessions` deferred | research.md R3 | Should any other cleanup be deferred during startup? |
| `Option<u64>` timestamp | data-model.md | Is this the simplest representation? (Alternative: bool + separate timer) |

## Spec Artifacts

| Artifact | Purpose |
|----------|---------|
| `spec.md` | Requirements, user stories, acceptance criteria |
| `plan.md` | Technical context, constitution check, project structure |
| `research.md` | Design decisions with rationale and alternatives |
| `data-model.md` | Single new field, state transition diagram |
| `quickstart.md` | Manual test procedures for live validation |
| `tasks.md` | 11 tasks organized by user story |
