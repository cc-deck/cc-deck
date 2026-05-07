# Feature Specification: Test Coverage Measurement and Baseline

**Feature Branch**: `050-test-coverage-measurement`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Test Coverage Measurement and Baseline for cc-deck WASM Plugin"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Local Coverage Report (Priority: P1)

A developer wants to see which lines of the Rust plugin code are covered by existing tests so they can identify untested modules before writing new features or fixing bugs.

**Why this priority**: Coverage visibility is the foundational purpose of this feature. Without a local report, none of the other stories deliver value.

**Independent Test**: Run `make coverage` in the project root and verify that an HTML coverage report opens in the browser showing per-file line coverage for all Rust source files.

**Acceptance Scenarios**:

1. **Given** the developer has `cargo-llvm-cov` and `llvm-tools-preview` installed, **When** they run `make coverage`, **Then** an HTML coverage report is generated and opened in the default browser showing line-level coverage for every Rust source file.
2. **Given** the developer does not have `cargo-llvm-cov` installed, **When** they run `make coverage`, **Then** the build fails with a clear error message explaining how to install the required tooling.
3. **Given** the developer runs `make coverage`, **When** the report is generated, **Then** it excludes test files themselves from the coverage percentage (only production code is measured).

---

### User Story 2 - Per-Module Coverage Summary (Priority: P2)

A developer wants a quick terminal overview of coverage broken down by module so they can spot low-coverage areas without opening a browser.

**Why this priority**: A terminal summary provides fast feedback during development and is useful in CI logs. It builds on the same tooling as P1 but adds a different output format.

**Independent Test**: Run `make coverage-summary` and verify that a table is printed to stdout showing line coverage percentages for each Rust module (controller, sidebar_plugin, pipe_handler, session, etc.).

**Acceptance Scenarios**:

1. **Given** the developer runs `make coverage-summary`, **When** the tests complete, **Then** a table is printed to stdout showing the module name and line coverage percentage for each top-level module in the plugin.
2. **Given** the developer runs `make coverage-summary`, **When** the output is generated, **Then** the table includes an aggregate total row showing overall coverage.

---

### User Story 3 - CI Coverage Upload (Priority: P2)

A team member opens a pull request that modifies Rust plugin code. The CI pipeline runs tests with coverage instrumentation and uploads results to Codecov, where the team can see coverage diffs in the PR.

**Why this priority**: CI coverage provides passive team visibility and catches coverage regressions on every PR. Equal priority with P2 because both deliver distinct value on top of P1.

**Independent Test**: Push a branch with Rust changes, verify the CI `rust-test` job uploads coverage to Codecov, and confirm a Codecov comment appears on the PR showing coverage diffs.

**Acceptance Scenarios**:

1. **Given** a PR is opened with Rust changes, **When** the CI pipeline runs, **Then** the `rust-test` job runs tests with coverage instrumentation and generates an lcov report.
2. **Given** the `rust-test` job generates an lcov report, **When** a Codecov token is configured, **Then** the report is uploaded to Codecov with the `rust` flag to separate it from Go coverage.
3. **Given** the Codecov token is not configured, **When** the CI pipeline runs, **Then** coverage upload is skipped gracefully without failing the build.
4. **Given** coverage is uploaded to Codecov, **When** a developer views the PR, **Then** Codecov posts a comment showing the coverage diff (lines added/removed and impact on overall coverage).

---

### User Story 4 - README Coverage Badge (Priority: P3)

A visitor to the repository can see the current Rust test coverage percentage at a glance via a badge in the README.

**Why this priority**: The badge is purely informational and depends on CI upload (P2) working first. Low effort but lowest independent value.

**Independent Test**: Check that the README contains a Codecov badge image that links to the Codecov dashboard and displays the current Rust coverage percentage.

**Acceptance Scenarios**:

1. **Given** CI has uploaded Rust coverage to Codecov at least once, **When** a visitor views the README, **Then** they see a badge showing the current Rust coverage percentage.
2. **Given** Codecov has no data yet, **When** a visitor views the README, **Then** the badge displays gracefully (shows "unknown" or similar rather than a broken image).

---

### User Story 5 - Machine-Readable Coverage Output (Priority: P3)

A developer or script needs coverage data in a machine-readable format for further processing, scripting, or integration with other tools.

**Why this priority**: JSON output enables automation but is not needed for day-to-day development. Low priority since the primary use cases are covered by HTML and terminal summary.

**Independent Test**: Run `make coverage-json` and verify that a JSON file is produced containing per-file and aggregate coverage data that can be parsed by standard JSON tools.

**Acceptance Scenarios**:

1. **Given** the developer runs `make coverage-json`, **When** the tests complete, **Then** a JSON file is written containing per-file line coverage data.
2. **Given** the JSON file is generated, **When** it is parsed with `jq`, **Then** per-file coverage percentages can be extracted.

---

### Edge Cases

- What happens when no tests exist for a module? The coverage report should still list the module with 0% coverage rather than omitting it.
- How does the system handle `#[cfg(target_family = "wasm")]` code paths? These are unreachable during native test execution. The coverage report covers the no-op stubs in `wasm_compat.rs` and `debug.rs` instead. This is a known limitation that should be documented.
- What happens when `cargo-llvm-cov` is not installed? The Makefile targets should fail with a clear error message explaining the installation steps.
- What happens if the Codecov token is missing in CI? Coverage upload should be skipped without failing the build, matching the existing Go coverage pattern.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST provide a Makefile target that runs all Rust tests with line-level coverage instrumentation and produces an HTML report.
- **FR-002**: The project MUST provide a Makefile target that prints a per-module coverage summary table to stdout.
- **FR-003**: The project MUST provide a Makefile target that generates coverage data in a machine-readable JSON format.
- **FR-004**: The CI pipeline MUST run Rust tests with coverage instrumentation and upload lcov results to Codecov with a `rust` flag.
- **FR-005**: The CI pipeline MUST skip coverage upload gracefully when the Codecov token is not configured, without failing the build.
- **FR-006**: The CI coverage upload MUST match the existing Go coverage upload pattern already present in the `go-test` job.
- **FR-007**: The README MUST include a Codecov badge showing the current Rust test coverage percentage.
- **FR-008**: Makefile coverage targets MUST fail with a clear, actionable error message when `cargo-llvm-cov` is not installed.
- **FR-009**: Coverage measurement MUST exclude test files from the coverage percentage (only production code is measured).
- **FR-010**: The per-module summary MUST include an aggregate total row showing overall coverage.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can generate an HTML coverage report with a single command (`make coverage`) and see line-level coverage for all Rust source files.
- **SC-002**: A developer can view per-module coverage percentages in the terminal with a single command (`make coverage-summary`).
- **SC-003**: Every PR that modifies Rust code receives a Codecov comment showing coverage diffs within the CI pipeline run time.
- **SC-004**: The README displays a live coverage badge that reflects the latest coverage measurement from the main branch.
- **SC-005**: Coverage reports correctly show 0% for modules with no tests rather than omitting them.
- **SC-006**: The CI build does not fail when the Codecov token is absent (graceful degradation).

## Clarifications

### Session 2026-05-07

No critical ambiguities detected. All open questions from the brainstorm were resolved before specification:
- Coverage floor: measure first, set later (deferred)
- CI enforcement: advisory only, never blocks merges
- lcov output: generated for CI upload only, not as a separate local target
- Per-module tracking: included via `make coverage-summary`

Minor details deferred to planning phase:
- Exact module granularity for the per-module summary (top-level source directories vs Rust module paths)
- Output file locations for HTML and JSON coverage artifacts
- Whether a `codecov.yml` configuration file is needed for PR comment behavior

## Assumptions

- Developers have `cargo-llvm-cov` and the `llvm-tools-preview` rustup component installed locally for local coverage targets. These are not installed automatically by the Makefile.
- The project already has a Codecov account and token configured as a repository secret (`CODECOV_TOKEN`) for CI coverage upload.
- Tests run on native targets (not wasm32). Code behind `#[cfg(target_family = "wasm")]` guards is unreachable during coverage measurement. The no-op stubs in `wasm_compat.rs` and `debug.rs` are covered instead. This is a known, documented limitation.
- Coverage measurement is advisory only. No minimum coverage threshold is enforced, and CI never blocks merges based on coverage results.
- The coverage floor value will be determined after the first baseline measurement, outside the scope of this feature.
- The existing Codecov integration for Go coverage works correctly and serves as the pattern to follow for Rust coverage.
