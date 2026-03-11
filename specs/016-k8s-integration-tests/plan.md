# Implementation Plan: K8s Integration Tests

**Branch**: `016-k8s-integration-tests` | **Date**: 2026-03-11 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/016-k8s-integration-tests/spec.md`

## Summary

Add integration tests for the cc-deck Kubernetes CLI that verify the full deploy/list/delete lifecycle against a kind cluster. Tests run in parallel using testify, with a GitHub Actions CI workflow for automated regression detection. A minimal stub container image replaces the real Claude Code image for testing.

## Technical Context

**Language/Version**: Go 1.22+ (go.mod specifies 1.25)
**Primary Dependencies**: k8s.io/client-go v0.35.2, github.com/stretchr/testify (new), cobra (existing)
**Storage**: N/A (tests create/delete K8s resources)
**Testing**: `go test -tags integration` with testify assert/require
**Target Platform**: Linux (CI), macOS (local dev)
**Project Type**: CLI (test infrastructure)
**Performance Goals**: All tests complete within 2 minutes (parallel execution)
**Constraints**: kind cluster required, stub image must be loaded
**Scale/Scope**: 8+ test cases, 1 CI workflow

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Go-only feature, no Rust/WASM |
| II. Plugin Installation | N/A | Test infrastructure, not plugin |
| III. WASM Filename | N/A | No WASM involved |
| IV. WASM Host Function Gating | N/A | No WASM involved |
| V. Zellij API Research Order | N/A | No Zellij API usage |
| VI. Simplicity | PASS | Direct function calls, no abstractions beyond testEnv |

No violations. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/016-k8s-integration-tests/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cc-deck/
├── test/
│   └── Containerfile.stub       # Minimal stub image for integration tests
└── internal/
    └── integration/
        ├── integration_test.go  # TestMain + all test cases
        └── helpers_test.go      # testEnv, assertions, cleanup utilities

.github/
└── workflows/
    └── integration.yaml         # CI workflow for kind + tests
```

**Structure Decision**: Integration tests live in `cc-deck/internal/integration/` as a separate package. The stub Containerfile lives in `cc-deck/test/` following Go convention for test fixtures. The CI workflow lives in `.github/workflows/` at the repository root.
