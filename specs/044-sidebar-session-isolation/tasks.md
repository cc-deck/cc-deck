# Tasks: Sidebar Session Isolation

**Feature**: 044-sidebar-session-isolation
**Generated**: 2026-04-25
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Phase 1: Setup

- [ ] T001 Extract PID helper function and path constants in cc-zellij-plugin/src/sync.rs

## Phase 2: Foundational (PID-Scoped State Files)

- [ ] T002 Replace `SESSIONS_PATH` constant with PID-scoped path function in cc-zellij-plugin/src/sync.rs
- [ ] T003 Replace `META_PATH` constant with PID-scoped path function in cc-zellij-plugin/src/sync.rs
- [ ] T004 Remove `PID_PATH` constant and `zellij_pid` file usage (PID is embedded in filenames) in cc-zellij-plugin/src/sync.rs
- [ ] T005 Update `save_sessions()` to write to PID-scoped path in cc-zellij-plugin/src/sync.rs
- [ ] T006 Update `restore_sessions()` to read from PID-scoped path and remove legacy PID check in cc-zellij-plugin/src/sync.rs
- [ ] T007 Update `broadcast_and_save()` to use PID-scoped path in cc-zellij-plugin/src/sync.rs
- [ ] T008 Update `write_session_meta()` and `apply_session_meta()` to use PID-scoped meta path in cc-zellij-plugin/src/sync.rs
- [ ] T009 Update `prune_session_meta()` to use PID-scoped meta path in cc-zellij-plugin/src/sync.rs

## Phase 3: User Story 1 - Session Isolation via Pipe Filtering (P1)

- [ ] T010 [US1] Update `broadcast_state()` to include PID in message name (`cc-deck:sync:{pid}`) in cc-zellij-plugin/src/sync.rs
- [ ] T011 [US1] Update `broadcast_and_save()` pipe message name to include PID in cc-zellij-plugin/src/sync.rs
- [ ] T012 [US1] Update `request_state()` to include PID in message name (`cc-deck:request:{pid}`) in cc-zellij-plugin/src/sync.rs
- [ ] T013 [US1] Update pipe message handler in `lib.rs` to extract PID from message name and ignore foreign PIDs in cc-zellij-plugin/src/lib.rs
- [ ] T014 [US1] Update unit tests for PID-scoped sync behavior in cc-zellij-plugin/src/sync.rs

## Phase 4: User Story 2 - Detach/Reattach Preservation (P1)

- [ ] T015 [US2] Verify `restore_sessions()` reads correct PID-scoped file on reattach (test) in cc-zellij-plugin/src/sync.rs
- [ ] T016 [US2] Verify no cross-session state leakage on concurrent reattach (test) in cc-zellij-plugin/src/sync.rs

## Phase 5: User Story 3 - Orphan Cleanup (P2)

- [ ] T017 [US3] Implement `cleanup_orphaned_state_files()` function that scans `/cache/` for `sessions-*.json` and removes files older than 7 days in cc-zellij-plugin/src/sync.rs
- [ ] T018 [US3] Call `cleanup_orphaned_state_files()` on plugin startup in cc-zellij-plugin/src/lib.rs
- [ ] T019 [US3] Call `cleanup_orphaned_state_files()` periodically via existing timer in cc-zellij-plugin/src/lib.rs
- [ ] T020 [US3] Add unit tests for orphan cleanup logic in cc-zellij-plugin/src/sync.rs

## Phase 6: Legacy Migration

- [ ] T021 Implement legacy file migration: rename `/cache/sessions.json` to `/cache/sessions-{pid}.json` on startup in cc-zellij-plugin/src/sync.rs
- [ ] T022 Implement legacy meta migration: rename `/cache/session-meta.json` to `/cache/session-meta-{pid}.json` on startup in cc-zellij-plugin/src/sync.rs
- [ ] T023 Add unit tests for legacy migration in cc-zellij-plugin/src/sync.rs

## Phase 7: Polish

- [ ] T024 Run `cargo test` and `cargo clippy` to verify all changes compile and pass in cc-zellij-plugin/
- [ ] T025 Build WASM binary and verify plugin loads correctly with `make install` in cc-zellij-plugin/

## Dependencies

```text
T001 → T002..T009 (PID helper needed first)
T002..T009 → T010..T013 (state files before pipe filtering)
T010..T013 → T014..T016 (implementation before tests)
T017..T020 independent of T010..T016
T021..T023 independent of T010..T020
T024..T025 after all implementation tasks
```

## Parallel Execution

| Group | Tasks | Rationale |
|---|---|---|
| State file paths | T002, T003, T004 | Independent constant/function changes |
| Read/write updates | T005, T006, T007, T008, T009 | Each modifies a separate function |
| Pipe message updates | T010, T011, T012 | Independent function changes |
| Orphan + Migration | T017-T020, T021-T023 | Independent feature slices |

## Implementation Strategy

**MVP**: Phases 1-3 (T001-T014). This delivers session isolation via PID-scoped files and filtered pipe messages. Two concurrent workspaces will show independent sidebars.

**Full delivery**: Add Phases 4-7 for reattach verification, orphan cleanup, legacy migration, and polish.

## Summary

- **Total tasks**: 25
- **US1 (isolation)**: 5 tasks
- **US2 (reattach)**: 2 tasks
- **US3 (cleanup)**: 4 tasks
- **Setup/foundational**: 9 tasks
- **Migration**: 3 tasks
- **Polish**: 2 tasks
