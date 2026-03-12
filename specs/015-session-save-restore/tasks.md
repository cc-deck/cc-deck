# Tasks: Session Save and Restore

**Input**: Design documents from `/specs/015-session-save-restore/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Project structure for session management feature

- [ ] T001 (cc-mux-49z.1) Create session management package directory at cc-deck/internal/session/
- [ ] T002 (cc-mux-49z.2) [P] Create Snapshot and SessionEntry structs with JSON serialization in cc-deck/internal/session/snapshot.go
- [ ] T003 (cc-mux-49z.3) [P] Add DumpState variant to PipeAction enum in cc-zellij-plugin/src/pipe_handler.rs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Plugin state dump and snapshot file I/O that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 (cc-mux-byh.1) Handle cc-deck:dump-state pipe message in cc-zellij-plugin/src/main.rs: serialize sessions as JSON, respond via cli_pipe_output/unblock_cli_pipe_input, only respond from one instance (active tab or lowest plugin_id)
- [ ] T005 (cc-mux-byh.2) Add parse_pipe_message test for dump-state in cc-zellij-plugin/src/pipe_handler.rs
- [ ] T006 (cc-mux-byh.3) Implement snapshot file I/O (load, save with atomic write, list, remove) in cc-deck/internal/session/snapshot.go using XDG config path ($XDG_CONFIG_HOME/cc-deck/sessions/)
- [ ] T007 (cc-mux-byh.4) Implement query_plugin_state function in cc-deck/internal/session/save.go: run `zellij pipe --name cc-deck:dump-state`, parse JSON response into Snapshot struct

**Checkpoint**: Plugin can dump state, CLI can read/write snapshot files

---

## Phase 3: User Story 1 - Save and Restore (Priority: P1) MVP

**Goal**: Users can explicitly save workspace state and restore it after Zellij restart

**Independent Test**: Run `cc-deck snapshot save`, restart Zellij, run `cc-deck snapshot restore`, verify tabs recreated with correct working dirs and Claude resumed

### Implementation for User Story 1

- [ ] T008 (cc-mux-ms9.1) [US1] Create cobra command group NewSnapshotCmd with save/restore/list/remove subcommands in cc-deck/internal/cmd/snapshot.go
- [ ] T009 (cc-mux-ms9.2) [US1] Register NewSnapshotCmd in cc-deck/cmd/cc-deck/main.go (add to rootCmd.AddCommand)
- [ ] T010 (cc-mux-ms9.3) [US1] Implement save command logic in cc-deck/internal/session/save.go: query plugin, generate timestamp name if unnamed, write snapshot file, print confirmation
- [ ] T011 (cc-mux-ms9.4) [US1] Implement restore command logic in cc-deck/internal/session/restore.go: load snapshot, for each session create tab via `zellij action new-tab`, write cd + claude --resume commands via `zellij action write-chars`, show progress output, handle resume failure fallback
- [ ] T012 (cc-mux-ms9.5) [US1] Wire save and restore RunE functions in cc-deck/internal/cmd/snapshot.go to call session package functions

**Checkpoint**: Save and restore fully functional for explicit use

---

## Phase 4: User Story 2 - Auto-save Safety Net (Priority: P2)

**Goal**: Hook events trigger automatic state snapshots with 5-minute cooldown and rolling retention

**Independent Test**: Start Claude sessions, trigger hook events, verify auto-save files appear in ~/.config/cc-deck/sessions/, verify rotation after 5 files

### Implementation for User Story 2

- [ ] T013 (cc-mux-07e.1) [US2] Implement auto-save logic in cc-deck/internal/session/autosave.go: check cooldown (5 min), spawn detached `cc-deck snapshot save --auto` process, use flock to prevent concurrent saves, query plugin state with 5s timeout, write auto-N.json with rotation (keep latest 5), atomic rename
- [ ] T014 (cc-mux-07e.2) [US2] Add auto-save trigger to cc-deck hook command in cc-deck/internal/cmd/hook.go: call AutoSave() (spawns background process) after zellij pipe succeeds

**Checkpoint**: Auto-save fires on hook events with cooldown and rotation

---

## Phase 5: User Story 3 - Named Saves and Listing (Priority: P2)

**Goal**: Users can save with custom names, list all snapshots, and restore specific named snapshots

**Independent Test**: Run `cc-deck snapshot save my-setup`, run `cc-deck snapshot list`, verify name/timestamp/count shown, run `cc-deck snapshot restore my-setup`

### Implementation for User Story 3

- [ ] T015 (cc-mux-psp.1) [US3] Implement list command logic in cc-deck/internal/session/snapshot.go: scan sessions directory, parse each JSON file, display name/timestamp/session-count/type table sorted by timestamp
- [ ] T016 (cc-mux-psp.2) [US3] Wire list RunE function in cc-deck/internal/cmd/snapshot.go
- [ ] T017 (cc-mux-psp.3) [US3] Update restore to select most recent snapshot (auto or named) when no name argument provided in cc-deck/internal/session/restore.go

**Checkpoint**: Named saves, listing, and argument-less restore all working

---

## Phase 6: User Story 4 - Snapshot Cleanup (Priority: P3)

**Goal**: Users can remove individual or all snapshots

**Independent Test**: Create named saves, run `cc-deck snapshot remove <name>`, verify deleted, run `cc-deck snapshot remove --all`, verify all cleared

### Implementation for User Story 4

- [ ] T018 (cc-mux-kab.1) [US4] Implement remove command logic in cc-deck/internal/session/snapshot.go: delete by name, delete all with --all flag, error with available names if not found
- [ ] T019 (cc-mux-kab.2) [US4] Wire remove RunE function with --all flag in cc-deck/internal/cmd/snapshot.go

**Checkpoint**: All CRUD operations on snapshots complete

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [ ] T020 (cc-mux-whf.1) Build and verify all tests pass: `cargo test` (plugin) and `go test ./...` (CLI)
- [ ] T021 (cc-mux-whf.2) Run `cargo clippy --target wasm32-wasip1` to verify WASM compilation
- [ ] T022 (cc-mux-whf.3) Run quickstart.md validation: manual end-to-end test of save/restore cycle

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 (plugin dump-state + snapshot I/O)
- **US2 (Phase 4)**: Depends on Phase 2 (snapshot I/O). Can run in parallel with US1.
- **US3 (Phase 5)**: Depends on Phase 2. Can run in parallel with US1/US2.
- **US4 (Phase 6)**: Depends on Phase 2. Can run in parallel with US1/US2/US3.
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Requires Phase 2 only. No dependencies on other stories.
- **US2 (P2)**: Requires Phase 2 only. Independent of US1.
- **US3 (P2)**: Requires Phase 2 only. Independent of US1/US2.
- **US4 (P3)**: Requires Phase 2 only. Independent of all others.

### Parallel Opportunities

- T002 and T003 can run in parallel (Go vs Rust, different repos)
- US1 through US4 can all start after Phase 2 (independent stories)
- Within Phase 7, T020 and T021 can run in parallel

---

## Parallel Example: Phase 1 Setup

```bash
# These can run in parallel (different languages/files):
Task: "Create Snapshot structs in cc-deck/internal/session/snapshot.go"
Task: "Add DumpState to PipeAction in cc-zellij-plugin/src/pipe_handler.rs"
```

## Parallel Example: After Phase 2

```bash
# All user stories can start simultaneously:
Task: "US1 - Implement save command in cc-deck/internal/cmd/snapshot.go"
Task: "US2 - Implement auto-save in cc-deck/internal/session/autosave.go"
Task: "US3 - Implement list in cc-deck/internal/session/snapshot.go"
Task: "US4 - Implement remove in cc-deck/internal/session/snapshot.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T007)
3. Complete Phase 3: US1 Save/Restore (T008-T012)
4. **STOP and VALIDATE**: Test save + restart + restore cycle
5. Ship MVP

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 (Save/Restore) -> Core value delivered (MVP)
3. US2 (Auto-save) -> Safety net added
4. US3 (Named saves + list) -> Workspace management
5. US4 (Cleanup) -> Housekeeping
6. Polish -> Final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
