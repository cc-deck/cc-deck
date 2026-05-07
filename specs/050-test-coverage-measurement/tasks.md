# Tasks: Test Coverage Measurement and Baseline

**Input**: Design documents from `/specs/050-test-coverage-measurement/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: No test tasks included (this feature is build tooling infrastructure, not application code).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No project initialization needed. This feature modifies existing files only.

(No setup tasks required.)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Prerequisite check helper used by all Makefile coverage targets

- [x] T001 Add cargo-llvm-cov prerequisite check function to Makefile that prints actionable install instructions when the tool is missing, in `Makefile`

**Checkpoint**: Foundation ready, coverage targets can reference the prerequisite check.

---

## Phase 3: User Story 1 - Local Coverage Report (Priority: P1) MVP

**Goal**: A developer can run `make coverage` to generate and open an HTML coverage report showing line-level coverage for all Rust source files.

**Independent Test**: Run `make coverage` and verify an HTML report opens in the browser with per-file line coverage.

### Implementation for User Story 1

- [x] T002 [US1] Add `coverage` target to `Makefile` that runs `cargo llvm-cov --html` in `cc-zellij-plugin/`, opens the HTML report in the default browser, and depends on the T001 prerequisite check

**Checkpoint**: `make coverage` generates and opens an HTML coverage report.

---

## Phase 4: User Story 2 - Per-Module Coverage Summary (Priority: P2)

**Goal**: A developer can run `make coverage-summary` to see a per-module coverage table in the terminal.

**Independent Test**: Run `make coverage-summary` and verify a table showing module names, covered/total lines, and percentages is printed to stdout.

### Implementation for User Story 2

- [x] T003 [P] [US2] Add `coverage-summary` target to `Makefile` that runs `cargo llvm-cov --json` in `cc-zellij-plugin/`, pipes output through `jq` to group files by module (controller/, sidebar_plugin/, root), and prints an aligned coverage table with an aggregate total row

**Checkpoint**: `make coverage-summary` prints a per-module coverage table with totals.

---

## Phase 5: User Story 3 - CI Coverage Upload (Priority: P2)

**Goal**: The CI `rust-test` job runs tests with coverage and uploads results to Codecov with a `rust` flag.

**Independent Test**: Push a branch, verify the `rust-test` CI job generates lcov and uploads to Codecov (or skips gracefully if no token).

### Implementation for User Story 3

- [x] T004 [P] [US3] Extend the `rust-test` job in `.github/workflows/ci.yaml` to: (1) add `components: llvm-tools-preview` to the rust toolchain step, (2) add a step to install `cargo-llvm-cov` via `taiki-e/install-action@cargo-llvm-cov`, (3) replace `cargo test` with `cargo llvm-cov --lcov --output-path lcov.info`, (4) add a Codecov upload step using `codecov/codecov-action@v5` with `flags: rust`, conditional on `CODECOV_TOKEN`, matching the `go-test` job pattern

**Checkpoint**: CI `rust-test` job generates coverage and uploads to Codecov.

---

## Phase 6: User Story 4 - README Coverage Badge (Priority: P3)

**Goal**: The README displays a Codecov badge reflecting Rust coverage.

**Independent Test**: Verify the README badge URL includes the project and renders correctly.

### Implementation for User Story 4

- [x] T005 [P] [US4] Verify that the existing aggregate Codecov badge in `README.md` will automatically include Rust coverage data once uploaded. No changes needed if the badge already uses `codecov.io/gh/cc-deck/cc-deck/graph/badge.svg` (which it does). Document this in a comment in the PR description.

**Checkpoint**: Existing README badge will reflect combined Go+Rust coverage after first CI upload.

---

## Phase 7: User Story 5 - Machine-Readable Coverage Output (Priority: P3)

**Goal**: A developer can run `make coverage-json` to generate a JSON coverage file.

**Independent Test**: Run `make coverage-json` and verify a JSON file is produced that can be parsed with `jq`.

### Implementation for User Story 5

- [x] T006 [P] [US5] Add `coverage-json` target to `Makefile` that runs `cargo llvm-cov --json` in `cc-zellij-plugin/` and writes output to `cc-zellij-plugin/target/llvm-cov/coverage.json`, depends on the T001 prerequisite check

**Checkpoint**: `make coverage-json` produces a parseable JSON file.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cleanup

- [x] T007 [P] Update README.md to document coverage targets (`make coverage`, `make coverage-summary`, `make coverage-json`) in the development section, noting the `cargo-llvm-cov` prerequisite and WASM limitation

**Checkpoint**: All coverage functionality documented.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies, can start immediately
- **US1 (Phase 3)**: Depends on T001 (prerequisite check)
- **US2 (Phase 4)**: Depends on T001 (prerequisite check). Can run in parallel with US1.
- **US3 (Phase 5)**: No dependency on T001 (CI installs tools independently). Can run in parallel with US1/US2.
- **US4 (Phase 6)**: No code changes needed. Can run in parallel with all other stories.
- **US5 (Phase 7)**: Depends on T001 (prerequisite check). Can run in parallel with US1/US2.
- **Polish (Phase 8)**: Depends on US1, US2, US5 being complete (to document all targets).

### User Story Dependencies

- **US1 (P1)**: Depends on T001 only
- **US2 (P2)**: Depends on T001 only, independent of US1
- **US3 (P2)**: Independent of all other stories (CI has its own tool installation)
- **US4 (P3)**: Independent (existing badge, no changes)
- **US5 (P3)**: Depends on T001 only, independent of US1/US2

### Parallel Opportunities

- T002, T003, T004, T005, T006 can ALL run in parallel after T001 completes
- T004 (CI) has no dependency on T001 and can start immediately
- T005 (badge verification) requires no code changes and can be done anytime

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001: Prerequisite check in Makefile
2. Complete T002: `make coverage` target
3. **STOP and VALIDATE**: Run `make coverage`, verify HTML report
4. Proceed to remaining stories

### Incremental Delivery

1. T001 (foundation) -> T002 (HTML report) -> Validate MVP
2. T003 (per-module summary) -> Validate terminal output
3. T004 (CI upload) -> Push branch, validate Codecov
4. T005 (badge verification) -> Check README
5. T006 (JSON output) -> Validate with jq
6. T007 (documentation) -> Final polish

### Parallel Execution

After T001 completes, launch all story tasks in parallel:
```
Task: T002 [US1] coverage target
Task: T003 [US2] coverage-summary target
Task: T004 [US3] CI workflow
Task: T006 [US5] coverage-json target
```

---

## Notes

- All Makefile targets operate in `cc-zellij-plugin/` directory (Rust plugin)
- Coverage output goes to `cc-zellij-plugin/target/llvm-cov/` (gitignored)
- CI installs its own tools (cargo-llvm-cov, llvm-tools-preview) independently of local setup
- T005 is a verification task, not a code change (existing badge covers Rust automatically)
