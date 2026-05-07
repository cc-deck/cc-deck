# Data Model: Test Coverage Measurement and Baseline

**Date**: 2026-05-07
**Branch**: `050-test-coverage-measurement`

## Overview

This feature introduces no persistent data entities. All coverage data is transient output from build tooling. This document describes the output artifacts for reference.

## Output Artifacts

### HTML Coverage Report

- **Location**: `cc-zellij-plugin/target/llvm-cov/html/`
- **Format**: Static HTML files with per-file source annotation
- **Lifecycle**: Generated on demand by `make coverage`, overwritten on each run
- **Gitignored**: Yes (inside `target/`)

### lcov Report

- **Location**: `cc-zellij-plugin/target/llvm-cov/lcov.info`
- **Format**: Standard lcov tracefile
- **Lifecycle**: Generated for CI upload, overwritten on each run
- **Gitignored**: Yes (inside `target/`)

### JSON Coverage Data

- **Location**: `cc-zellij-plugin/target/llvm-cov/coverage.json`
- **Format**: cargo-llvm-cov JSON export format
- **Lifecycle**: Generated on demand by `make coverage-json`, overwritten on each run
- **Gitignored**: Yes (inside `target/`)

## External State

### Codecov

- Coverage data is uploaded to Codecov with `flags: rust` alongside existing `flags: go`
- Codecov aggregates both flags for the overall project badge
- No local state is stored for Codecov integration
