# Research: Test Coverage Measurement and Baseline

**Date**: 2026-05-07
**Branch**: `050-test-coverage-measurement`

## R1: cargo-llvm-cov Tool Selection

**Decision**: Use `cargo-llvm-cov` for coverage instrumentation
**Rationale**: LLVM source-based instrumentation works on both macOS (developer platform) and Linux (CI). Produces accurate line-level coverage. Supports HTML, lcov, and JSON output formats. Widely adopted in the Rust ecosystem with active maintenance.

**Alternatives considered**:
- `cargo-tarpaulin`: Linux-only, incompatible with macOS development workflow
- `grcov`: Requires manual profraw file management, more setup complexity

**Installation**: `cargo install cargo-llvm-cov` + `rustup component add llvm-tools-preview`

**Key commands**:
- `cargo llvm-cov --html --output-dir target/llvm-cov/html` - HTML report
- `cargo llvm-cov --lcov --output-path target/llvm-cov/lcov.info` - lcov for Codecov
- `cargo llvm-cov --json --output-path target/llvm-cov/coverage.json` - JSON for scripting

## R2: Per-Module Coverage Summary

**Decision**: Parse `cargo llvm-cov --json` output with `jq` to group by module
**Rationale**: JSON output includes per-file line/region/branch counts. Files map to modules via directory structure. Using `jq` keeps the solution dependency-free.

**Module mapping**:
| Module | Directory | Files |
|--------|-----------|-------|
| controller | `src/controller/` | actions, events, hooks, mod, render_broadcast, sidebar_registry, state |
| sidebar_plugin | `src/sidebar_plugin/` | input, mod, modes, render, rename, state, test_helpers |
| root | `src/` (top-level) | main, lib, session, pipe_handler, config, perf, git, debug, wasm_compat |

**Output format**: Aligned table with module name, lines covered, total lines, percentage.

## R3: CI Integration Pattern

**Decision**: Mirror `go-test` job's Codecov pattern in `rust-test`
**Rationale**: Consistency with existing CI. The Go job already validates the pattern works.

**Changes to `rust-test` job**:
1. Add `components: llvm-tools-preview` to `dtolnay/rust-toolchain@stable`
2. Add step to install `cargo-llvm-cov` via `taiki-e/install-action@cargo-llvm-cov`
3. Replace `cargo test` with `cargo llvm-cov --lcov --output-path lcov.info`
4. Add Codecov upload step with `flags: rust`, conditional on `CODECOV_TOKEN`

## R4: README Badge Strategy

**Decision**: Keep existing aggregate Codecov badge unchanged
**Rationale**: The badge at `codecov.io/gh/cc-deck/cc-deck/graph/badge.svg` automatically includes all uploaded flags. Once Rust coverage is uploaded, the aggregate percentage updates to include both Go and Rust data. No additional badge needed.

**Flag-specific badge** (documented for future use):
`https://codecov.io/gh/cc-deck/cc-deck/branch/main/graph/badge.svg?flag=rust`

## R5: Error Handling for Missing Tools

**Decision**: Guard each Makefile coverage target with `command -v cargo-llvm-cov`
**Rationale**: Provides actionable error message instead of cryptic make failure.

**Error message template**:
```
cargo-llvm-cov is not installed.
Install it with: cargo install cargo-llvm-cov
Also ensure llvm-tools-preview is installed: rustup component add llvm-tools-preview
```
