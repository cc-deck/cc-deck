# Feature Specification: Property-Based Fuzz Testing for Sidebar State Machine

**Feature Branch**: `051-proptest-fuzz-testing`
**Created**: 2026-05-07
**Status**: Draft
**Input**: Brainstorm document `brainstorm/051-proptest-fuzz-testing.md`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover Unknown State Machine Bugs via Random Input Sequences (Priority: P1)

As a developer, I want to run property-based fuzz tests against the sidebar mode state machine so that I can discover edge-case bugs that hand-written unit tests miss, such as cursor positions going out of bounds after session removal or filter state leaking between mode transitions.

**Why this priority**: The sidebar has 7 mode variants and approximately 25 transitions. Unit tests verify known paths, but cannot explore the combinatorial space of multi-step input sequences. The project has prior evidence (regression seeds in `proptest-regressions/fuzz_tests.txt`) that fuzzing previously caught real bugs in cursor invariants.

**Independent Test**: Run `cargo test fuzz` in the `cc-zellij-plugin` crate. The fuzz test generates random sequences of user actions and verifies that all invariants hold after every action. A passing run with 2000 test cases confirms no invariant violations were found.

**Acceptance Scenarios**:

1. **Given** a sidebar state with 0-5 initial sessions, **When** a random sequence of 1-50 user actions is applied (keyboard keys, mouse clicks, session mutations), **Then** no panic occurs and all state invariants hold after every action.
2. **Given** a sidebar in Navigate mode with cursor at position N, **When** a session is removed reducing the list below N, **Then** the cursor is clamped to a valid position (less than the session count, or 0 if the list is empty).
3. **Given** a sidebar in NavigateFilter mode, **When** the mode transitions to any other mode, **Then** the mode-level filter state is not accessible from the new mode (no state leakage).
4. **Given** any sequence of actions that ends in Passive mode, **When** the filter_text field is inspected, **Then** it is empty.

---

### User Story 2 - Preserve Regression Seeds for Reproducibility (Priority: P2)

As a developer, I want fuzz test failures to be captured as deterministic regression seeds so that previously-discovered bugs remain covered in future test runs, even if the random seed changes.

**Why this priority**: Regression seeds are the lasting value of fuzz testing. Without them, a bug found once may never be re-tested. The project already has historical seeds from the prior fuzz suite that should be migrated to the new module path.

**Independent Test**: Place a known regression seed in the `proptest-regressions/` directory. Run `cargo test fuzz`. Verify the seed is replayed as part of the test run.

**Acceptance Scenarios**:

1. **Given** a proptest regression seed file exists at the path matching the new fuzz test module, **When** `cargo test fuzz` runs, **Then** the seed is replayed deterministically before random cases begin.
2. **Given** the old regression seeds in `proptest-regressions/fuzz_tests.txt`, **When** the new fuzz test module is created, **Then** the seeds are migrated to the correct file path for the new module (or documented as inapplicable if the test signature changed).

---

### User Story 3 - Fast Feedback in Development Workflow (Priority: P3)

As a developer, I want fuzz tests to complete within a reasonable time so that they can run as part of the regular `cargo test` cycle without slowing down the development workflow.

**Why this priority**: Fuzz tests that are too slow get skipped or removed. The test suite must remain fast enough to run on every change.

**Independent Test**: Time `cargo test fuzz` locally. Verify it completes within the target duration.

**Acceptance Scenarios**:

1. **Given** the fuzz test is configured with 2000 test cases and sequence length 1-50, **When** `cargo test fuzz` runs on a developer machine, **Then** it completes in under 10 seconds.

---

### Edge Cases

- What happens when the session list is empty and all navigation actions are applied?
- What happens when Help mode wraps another Help mode (nested Help via toggle_help while already in Help)?
- What happens when a delete confirmation targets a pane_id that no longer exists in the session list?
- What happens when filter input contains Unicode characters (multi-byte)?
- What happens when toggle_navigate is called while in RenamePassive mode?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The fuzz test MUST define a `FuzzAction` type that covers all user input paths: keyboard keys (j, k, Enter, Esc, d, r, p, /, ?, m, n, y, Y, R, Backspace, arbitrary characters), toggle navigate, toggle navigate prev, left click at arbitrary rows, right click at arbitrary rows, and session list mutations (add/remove sessions).
- **FR-002**: The fuzz test MUST generate random sequences of 1-50 `FuzzAction` values, starting from initial state with 0-5 sessions.
- **FR-003**: The fuzz test MUST verify the following invariants after every action in the sequence:
  - No panic (implicit via test execution)
  - Cursor in bounds: if in a navigation sub-mode, `cursor_index < max(1, filtered_sessions.len())`
  - Mode accessor consistency: if mode is `NavigateFilter`, then `filter_state()` returns `Some`
  - Filter text empty in Passive: `filter_text` is always empty when mode is `Passive`
  - Selectable matches mode: `is_selectable()` returns false only for `Passive` mode
- **FR-004**: The fuzz test MUST use proptest with `prop_oneof!` for action generation, providing uniform coverage across all action variants.
- **FR-005**: The fuzz test MUST be configured with 2000 test cases per run.
- **FR-006**: The fuzz test MUST live in a dedicated file within the `sidebar_plugin` module, declared as a test module in `mod.rs`.
- **FR-007**: The fuzz test MUST reuse existing test helpers for constructing sidebar state and render payload instances.
- **FR-008**: Regression seeds MUST be stored in `proptest-regressions/` and the existing seeds MUST be evaluated for migration to the new module path.

### Key Entities

- **FuzzAction**: An enum representing any possible user input to the sidebar (keyboard key, mouse click, session mutation). Each variant maps to a specific handler function in the sidebar input module.
- **SidebarState**: The aggregate state under test, containing the mode state machine, filter text, cached payload, and click regions.
- **Invariant**: A boolean property that must hold after every action. Invariant violations indicate state machine bugs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The fuzz test suite runs 2000 random action sequences without any invariant violation or panic.
- **SC-002**: The fuzz test suite completes in under 10 seconds on a developer machine.
- **SC-003**: The `FuzzAction` type covers all user-facing input paths (at least 18 distinct action variants covering keyboard, mouse, and session mutations).
- **SC-004**: At least 5 state invariants are verified after every action in every sequence.
- **SC-005**: All existing tests continue to pass after adding the fuzz tests (no regressions).

## Assumptions

- The proptest crate is available or can be added as a dev-dependency without conflicting with existing dependencies.
- The sidebar state machine functions are callable in a non-WASM test environment (WASM-gated host calls are stubbed out in conditional compilation blocks).
- Session mutations (add/remove) can be simulated by modifying the cached payload directly, followed by cursor preservation.
- The scope is limited to the sidebar mode state machine. Controller-level event handling is out of scope for this feature.
- Libfuzzer-based fuzzing (cargo-fuzz) is out of scope. This feature uses proptest exclusively for property-based testing.
