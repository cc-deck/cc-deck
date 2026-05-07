# Review Guide: Test Coverage Measurement and Baseline

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-07

---

## What This Spec Does

The cc-deck WASM plugin has 215 tests but no way to know which code paths they cover. This feature adds coverage measurement using `cargo-llvm-cov`, with local Makefile targets for developers and CI integration via Codecov to give the team passive visibility into coverage trends on every PR.

**In scope:** Three Makefile targets (`coverage`, `coverage-summary`, `coverage-json`), extending the CI `rust-test` job to upload lcov to Codecov with a `rust` flag, and documenting the coverage workflow in the README.

**Out of scope:** Coverage floor enforcement (advisory only, no merge blocking), per-module CI reporting beyond Codecov's aggregate, and any changes to the Rust source code itself. The coverage floor value will be determined after the first baseline measurement.

## Bigger Picture

This feature fills a gap exposed during spec 049 (dead code cleanup), where the deep review found that `sidebar_plugin/input.rs` (647 lines) had zero tests. That gap was only caught by manual review. Coverage measurement prevents similar blind spots from accumulating as the plugin grows.

The Go side of cc-deck already uploads coverage to Codecov from the `go-test` CI job with `flags: go`. This spec mirrors that pattern for Rust with `flags: rust`, so both languages feed the same Codecov dashboard. The existing aggregate README badge will automatically include Rust data once the first upload happens.

This is pure build tooling infrastructure. No application code changes, no new dependencies in `Cargo.toml`, no runtime impact. The implementation touches only `Makefile`, `.github/workflows/ci.yaml`, and `README.md`.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts that need human judgment most.

### Understanding the approach (8 min)

Read the [Chosen Approach](spec.md#clarifications) and [Functional Requirements](spec.md#functional-requirements) sections. As you read, consider:

- Is `cargo-llvm-cov` the right tool for this project, given that tests run on native (not wasm32)? The [Assumptions](spec.md#assumptions) section documents this limitation.
- Does mirroring the Go CI pattern make sense, or should Rust coverage be handled differently given the WASM compilation target?

### Key decisions that need your eyes (12 min)

**Advisory-only coverage** ([Success Criteria](spec.md#measurable-outcomes))

The spec explicitly avoids enforcing a coverage floor. CI never blocks merges based on coverage. This means coverage can erode silently if the team does not actively monitor Codecov reports.
- Question: Is advisory-only sufficient, or should there be at least a warning when coverage drops below a threshold?

**Per-module summary via jq** ([plan.md Phase 0 R2](plan.md#r2-per-module-coverage-summary))

The plan uses `cargo llvm-cov --json` piped through `jq` to produce a per-module table. The module grouping is by top-level directory (controller/, sidebar_plugin/, root files).
- Question: Is this granularity useful? Would file-level output be more actionable, or is module-level the right abstraction for spotting gaps?

**Existing badge reuse** ([plan.md R4](plan.md#r4-readme-badge-strategy))

The plan concludes that the existing aggregate Codecov badge already covers Rust once data is uploaded. No new badge is added. T005 is just a verification task.
- Question: Should there be a separate Rust-specific badge so the team can see Rust coverage independently from Go?

### Areas where I'm less certain (5 min)

- [FR-009](spec.md#functional-requirements): "Coverage measurement MUST exclude test files from the coverage percentage." `cargo-llvm-cov` excludes test code by default, but if the project has test helper modules (like `sidebar_plugin/test_helpers.rs`) that are not behind `#[cfg(test)]`, they might inflate coverage numbers. I'm not certain whether the current test helper structure would be excluded automatically.

- [tasks.md T003](tasks.md#phase-4-user-story-2---per-module-coverage-summary-priority-p2): The `jq` script to group files by module and compute coverage percentages from `cargo-llvm-cov --json` output could be fragile if the JSON schema changes between tool versions. No pinned version is specified.

### Risks and open questions (5 min)

- The CI job installs `cargo-llvm-cov` on every run. If the `taiki-e/install-action` or crate registry has an outage, does the `rust-test` job fail? The Go job has no equivalent external tool dependency.
- `cargo-llvm-cov` runs all tests natively. If any test relies on WASM-specific behavior that the no-op stubs do not replicate faithfully, coverage results could be misleading. Are the current stubs comprehensive enough?
- The Makefile `coverage-summary` target depends on `jq` being installed locally. Is `jq` a safe assumption for all developers, or should there be a prerequisite check similar to T001?

---

## Coverage Matrix

| Requirement | Implementing Tasks |
|-------------|-------------------|
| FR-001 (HTML coverage report) | T002 |
| FR-002 (per-module summary) | T003 |
| FR-003 (JSON output) | T006 |
| FR-004 (CI lcov upload) | T004 |
| FR-005 (graceful token skip) | T004 |
| FR-006 (match Go pattern) | T004 |
| FR-007 (README badge) | T005 |
| FR-008 (missing tool error) | T001 |
| FR-009 (exclude test files) | T002 (implicit in cargo-llvm-cov defaults) |
| FR-010 (aggregate total row) | T003 |

All requirements have task coverage. No gaps.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
