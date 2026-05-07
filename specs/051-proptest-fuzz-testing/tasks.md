# Tasks: Property-Based Fuzz Testing for Sidebar State Machine

**Input**: Design documents from `specs/051-proptest-fuzz-testing/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: This feature IS a test feature. The fuzz tests themselves are the deliverable.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add proptest dependency and create module structure

- [x] T001 Add `proptest = "1"` to `[dev-dependencies]` in `cc-zellij-plugin/Cargo.toml`
- [x] T002 Add `#[cfg(test)] mod fuzz_tests;` declaration to `cc-zellij-plugin/src/sidebar_plugin/mod.rs`
- [x] T003 Create empty `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs` with module doc comment and imports

**Checkpoint**: `make test` passes with empty fuzz_tests module

---

## Phase 2: User Story 1 - Discover Unknown State Machine Bugs (Priority: P1)

**Goal**: Generate random action sequences and verify state invariants after every action

**Independent Test**: Run `cargo test -p cc-deck fuzz` and verify 2000 cases pass without invariant violations

### Implementation for User Story 1

- [x] T004 [US1] Define `FuzzAction` enum with all 22 variants (KeyJ, KeyK, KeyEnter, KeyEsc, KeyD, KeyR, KeyP, KeySlash, KeyQuestion, KeyM, KeyN, KeyBigR, KeyY, KeyBigY, KeyBackspace, ArbitraryChar, ToggleNavigate, ToggleNavigatePrev, LeftClick, RightClick, AddSession, RemoveSession) in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T005 [US1] Implement proptest `Arbitrary` strategy for `FuzzAction` using `prop_oneof!` with uniform weighting across all variants in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T006 [US1] Implement `apply_action` function that maps each `FuzzAction` variant to the corresponding handler call (`handle_key`, `handle_mouse`, `toggle_navigate`, `toggle_navigate_prev`, or payload mutation) in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T007 [US1] Implement `build_click_regions` helper that constructs valid click regions from the current session list (3-row spacing, header sentinel at row 0) in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T008 [US1] Implement `check_invariants` function verifying all 5 invariants (cursor bounds, filter state consistency, passive filter clean, selectable matches mode, help consistency) in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T009 [US1] Implement the main `proptest!` test function `test_sidebar_invariants` with `ProptestConfig { cases: 2000, .. }`, generating 0-5 initial sessions and 1-50 action sequences, applying actions and checking invariants after each in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T010 [US1] Run `make test` and verify fuzz tests pass. If invariant violations are found, fix the bugs in the sidebar state machine code (modes.rs, input.rs, state.rs) before proceeding

**Checkpoint**: `cargo test -p cc-deck fuzz` runs 2000 cases in <10 seconds with no invariant violations

---

## Phase 3: User Story 2 - Preserve Regression Seeds (Priority: P2)

**Goal**: Ensure regression seeds are stored and replayed correctly

**Independent Test**: Verify proptest regression seed files are at the correct path for the new module

### Implementation for User Story 2

- [x] T011 [US2] Evaluate existing seeds in `cc-zellij-plugin/proptest-regressions/fuzz_tests.txt` for compatibility with new test module path and FuzzAction signature; document findings as a comment in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T012 [US2] Verify proptest creates regression files at the correct path (should be `cc-zellij-plugin/proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt`) by checking proptest config or running a deliberate failure test

**Checkpoint**: Regression seed infrastructure is verified and documented

---

## Phase 4: User Story 3 - Fast Feedback (Priority: P3)

**Goal**: Verify fuzz tests complete within performance target

**Independent Test**: Time `make test` and verify fuzz test portion completes in <10 seconds

### Implementation for User Story 3

- [x] T013 [US3] Run `make test` with timing and verify fuzz tests complete in <10 seconds. If too slow, adjust `ProptestConfig.cases` or sequence length bounds in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`
- [x] T014 [US3] Run `make lint` and fix any clippy warnings in `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs`

**Checkpoint**: Full test suite passes, fuzz tests run within performance budget, no lint warnings

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [x] T015 Verify all existing tests still pass with `make test` (no regressions)
- [x] T016 Run quickstart.md validation: execute the commands documented in quickstart.md and verify they work

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **User Story 1 (Phase 2)**: Depends on Phase 1 completion
- **User Story 2 (Phase 3)**: Depends on Phase 2 (needs the fuzz test to exist)
- **User Story 3 (Phase 4)**: Depends on Phase 2 (needs the fuzz test to exist)
- **Polish (Phase 5)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Depends on Setup only. Core deliverable.
- **User Story 2 (P2)**: Depends on US1 (needs fuzz test to exist for seed path verification)
- **User Story 3 (P3)**: Depends on US1 (needs fuzz test to exist for timing). Can run in parallel with US2.

### Parallel Opportunities

- T011 and T013 can run in parallel (both depend on US1 but work on different concerns)
- T014 can run in parallel with T012

---

## Parallel Example: User Story 1

```bash
# All tasks in US1 are sequential (single file, each builds on prior work)
# No parallelism within US1 - but US2 and US3 can run in parallel after US1 completes
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: User Story 1 (T004-T010)
3. **STOP and VALIDATE**: Run `cargo test -p cc-deck fuzz` independently
4. If any invariant violations found, fix the bugs before proceeding

### Incremental Delivery

1. Setup + US1 complete -> Core fuzz testing works (MVP)
2. Add US2 -> Regression seeds verified
3. Add US3 -> Performance validated, lint clean
4. Polish -> Full validation complete

---

## Notes

- All US1 tasks are in a single file (fuzz_tests.rs), so they cannot be parallelized
- T010 may require fixing bugs found by the fuzz tests. These fixes are in different files (modes.rs, input.rs, state.rs) and are part of the deliverable
- The feature spec explicitly states this is test infrastructure, so no README/docs updates needed per constitution
