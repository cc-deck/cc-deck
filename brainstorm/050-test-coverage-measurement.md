# Brainstorm: Test Coverage Measurement and Baseline

**Date:** 2026-05-06
**Status:** active

## Problem Framing

The cc-deck WASM plugin has 215 tests but no visibility into which code paths are actually covered.
During spec 049 (dead code cleanup), the deep review found that `sidebar_plugin/input.rs` (647 lines) had zero tests, a gap that was only discovered by manual review.
Without automated coverage measurement, similar gaps will go unnoticed until a regression surfaces.

### Current state

- 215 unit tests across controller/, sidebar_plugin/, pipe_handler, session, lib, perf, main
- No coverage tooling installed
- No coverage targets in the Makefile
- No CI coverage gates or trend tracking
- The project develops on macOS (Darwin), so Linux-only tools like cargo-tarpaulin are not usable

### What this enables

Coverage measurement serves three purposes:
1. **Identify gaps**: find untested modules before they cause regressions
2. **Set a floor**: establish a minimum coverage percentage that new code must maintain
3. **Track trends**: detect coverage erosion over time as features are added

## Proposed Approach

### Tool: cargo-llvm-cov

`cargo-llvm-cov` uses LLVM source-based coverage instrumentation.
It works on macOS, produces accurate line/branch coverage, and supports HTML, lcov, and JSON output formats.

Installation:
```bash
cargo install cargo-llvm-cov
rustup component add llvm-tools-preview
```

### Makefile targets

```makefile
coverage:       ## Run tests with coverage report (HTML, opens in browser)
coverage-lcov:  ## Generate lcov report for CI/editors
```

### Limitations

Tests run on native (not wasm32).
All `#[cfg(target_family = "wasm")]` code paths are unreachable during native test execution, meaning the actual WASM host function calls are never covered.
The no-op stubs in `wasm_compat.rs` and `debug.rs` get covered instead.
This is an inherent limitation of testing WASM plugins on native.

### Open Questions

- What coverage floor should we set? 70%? 80%? Or just measure and improve incrementally?
- Should coverage be enforced in CI, or is it advisory?
- Should we add lcov output for IDE integration (VS Code coverage gutters)?
- Is there value in tracking coverage per module (e.g., sidebar_plugin vs controller vs shared)?

---

## Revisit: 2026-05-07

### Decisions Made

All open questions from the initial brainstorm have been resolved:

1. **Coverage floor**: Measure first, set floor later. Run coverage to establish a baseline, then decide on a target based on the actual numbers.
2. **CI enforcement**: CI runs coverage and posts a report (via Codecov), but never blocks merges. Advisory only.
3. **lcov output**: Not needed. HTML report is sufficient for local use.
4. **Per-module tracking**: Yes. A local `make coverage-summary` target prints per-module line coverage.

### Chosen Approach: Full Coverage Platform

Three approaches were considered:

#### A: Local-only
- Makefile targets for `make coverage` (HTML) and `make coverage-json` (machine-readable)
- No CI changes
- Simplest, but no team visibility

#### B: Local + CI report
- Same local tooling as A
- CI uploads to Codecov, matching existing Go pattern in ci.yaml
- Moderate complexity

#### C: Full platform (chosen)
- Everything from B
- Codecov PR comments showing coverage diffs per PR
- README badge showing current Rust coverage percentage
- Highest setup cost, but gives the most visibility

**Why C**: The Go side already uploads to Codecov. Matching that pattern for Rust is low incremental cost. PR comments and the badge provide passive visibility that helps the team notice coverage trends without actively checking.

### Existing CI Context

The `ci.yaml` workflow already has a `go-test` job that runs coverage and uploads to Codecov. The `rust-test` job runs `cargo test` without coverage. The spec extends `rust-test` to mirror the Go pattern.

### Scope

- Tool: `cargo-llvm-cov` (LLVM source-based instrumentation, works on macOS)
- Local targets: `make coverage` (HTML), `make coverage-summary` (per-module table), `make coverage-json` (machine-readable)
- CI: extend `rust-test` job to upload lcov to Codecov with `flags: rust`
- README: add Codecov badge
- Limitation: tests run on native (not wasm32), so `#[cfg(target_family = "wasm")]` code paths are unreachable. The no-op stubs in `wasm_compat.rs` and `debug.rs` get covered instead.

### Open Threads

- Coverage floor value to be determined after first baseline measurement
- Whether to add per-module CI reporting (beyond aggregate Codecov) after seeing initial results
