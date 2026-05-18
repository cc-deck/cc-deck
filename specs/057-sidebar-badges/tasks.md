# Tasks: Configurable Sidebar Badges

**Input**: Design documents from `/specs/057-sidebar-badges/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Add badge data structures across the Go CLI and Rust plugin

- [x] T001 [P] Add BadgeRule struct and Badges field to Config in cc-deck/internal/config/config.go
- [x] T002 [P] Create badge evaluation package with Evaluate function in cc-deck/internal/badge/badge.go
- [x] T003 [P] Add badges field (Vec<String>) to HookPayload in cc-zellij-plugin/src/pipe_handler.rs
- [x] T004 [P] Add badges field (Vec<String>) to Session in cc-zellij-plugin/src/session.rs
- [x] T005 [P] Add badges field (Vec<String>) to RenderSession in cc-zellij-plugin/src/lib.rs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Wire badge data through the hook pipeline end-to-end

- [x] T006 Add badges field to pipePayload and hookPayload structs in cc-deck/internal/cmd/hook.go
- [x] T007 Call badge.Evaluate in runHook and populate pipePayload.Badges in cc-deck/internal/cmd/hook.go
- [x] T008 Store HookPayload.badges on Session in process_hook in cc-zellij-plugin/src/controller/hooks.rs
- [x] T009 Copy Session.badges to RenderSession.badges in build_render_payload in cc-zellij-plugin/src/controller/render_broadcast.rs

**Checkpoint**: Badge data flows from CLI config through hook payload to plugin render payload. No visible output yet.

---

## Phase 3: User Story 1 - Spex Pipeline Badge (Priority: P1)

**Goal**: Show a spex ship/flow badge emoji on sidebar line 2 when .specify/.spex-state exists

**Independent Test**: Configure a badge rule for .specify/.spex-state, create the file with {"mode": "ship"}, fire a hook event, verify sidebar renders the ship emoji.

### Implementation for User Story 1

- [x] T010 [US1] Implement dot-path JSON extraction (extractDotPath function) in cc-deck/internal/badge/badge.go
- [x] T011 [US1] Implement full Evaluate function (file read, extract, map to emoji) in cc-deck/internal/badge/badge.go
- [x] T012 [US1] Add unit tests for badge evaluation (dot-path, value mapping, defaults, errors) in cc-deck/internal/badge/badge_test.go
- [x] T013 [US1] Render badges on line 2 before branch icon in render_session_entry in cc-zellij-plugin/src/sidebar_plugin/render.rs
- [x] T014 [US1] Add unit test for badge rendering in cc-zellij-plugin/src/sidebar_plugin/render.rs

**Checkpoint**: Single badge (spex pipeline) renders on sidebar line 2.

---

## Phase 4: User Story 2 - Multiple Simultaneous Badges (Priority: P2)

**Goal**: Display multiple badges when multiple state files exist

**Independent Test**: Configure two badge rules, create both state files, verify both emojis appear in config order.

### Implementation for User Story 2

- [x] T015 [US2] Add unit test for multiple badge evaluation (multiple rules, partial matches) in cc-deck/internal/badge/badge_test.go
- [x] T016 [US2] Add unit test for multiple badge rendering on line 2 in cc-zellij-plugin/src/sidebar_plugin/render.rs

**Checkpoint**: Multiple badges render correctly in configuration order.

---

## Phase 5: User Story 3 - Silent Failure (Priority: P2)

**Goal**: Badge evaluation silently skips when files are missing, invalid, or paths don't resolve

**Independent Test**: Configure badge rules for nonexistent files, invalid JSON, and unmatched paths; verify no badges appear and no errors.

### Implementation for User Story 3

- [x] T017 [US3] Add unit tests for error cases (missing file, invalid JSON, bad dot-path, no match no default) in cc-deck/internal/badge/badge_test.go

**Checkpoint**: All error cases handled silently.

---

## Phase 6: User Story 4 - Badge Configuration (Priority: P3)

**Goal**: Users can define badge rules in config.yaml and have them parsed correctly

**Independent Test**: Write config with badge rules, verify parsing and round-trip.

### Implementation for User Story 4

- [x] T018 [US4] Add config parsing test for badges section in cc-deck/internal/config/config_test.go (or badge_test.go)
- [x] T019 [US4] Add test for nested dot-path extraction (e.g., .result.outcome) in cc-deck/internal/badge/badge_test.go
- [x] T020 [US4] Add test for invalid/missing badge rule fields (skip silently) in cc-deck/internal/badge/badge_test.go

**Checkpoint**: Configuration parsing is robust and well-tested.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, integration test, cleanup

- [x] T021 [P] Add integration test for full badge pipeline (hook with badges through to render) in cc-zellij-plugin/src/controller/integration_tests.rs
- [x] T022 [P] Add HookPayload deserialization test with badges field in cc-zellij-plugin/src/pipe_handler.rs
- [x] T023 [P] Update configuration reference docs for badges section in docs/modules/reference/pages/configuration.adoc
- [x] T024 [P] Update README.md with badge configuration example

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, all tasks parallelizable
- **Foundational (Phase 2)**: Depends on Setup; T006-T007 depend on T001-T002; T008-T009 depend on T003-T005
- **User Story 1 (Phase 3)**: Depends on Foundational phase
- **User Story 2 (Phase 4)**: Depends on US1 (rendering logic must exist)
- **User Story 3 (Phase 5)**: Can run in parallel with US2 (tests error paths in badge.go)
- **User Story 4 (Phase 6)**: Can run in parallel with US2/US3 (tests config parsing)
- **Polish (Phase 7)**: Depends on all user stories

### Parallel Opportunities

- T001-T005 (all Setup) can run in parallel
- T015-T020 (US2, US3, US4 tests) can mostly run in parallel after US1
- T021-T024 (Polish) can all run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (add data structures)
2. Complete Phase 2: Foundational (wire pipeline)
3. Complete Phase 3: User Story 1 (evaluate + render single badge)
4. **STOP and VALIDATE**: Test with a real .specify/.spex-state file

### Incremental Delivery

1. Setup + Foundational: badge data flows end-to-end
2. US1: single badge works (MVP)
3. US2: multiple badges display
4. US3: error handling confirmed
5. US4: config parsing robust
6. Polish: docs, integration tests

---

## Notes

- [P] tasks = different files, no dependencies
- Constitution requires tests + docs with every feature
- Use `make test` and `make lint`, never `go build` or `cargo build` directly
- Badge evaluation is in Go (CLI side), rendering in Rust (plugin side)
