# Implementation Plan: Deterministic Policy Generation

**Branch**: `059-deterministic-policy-generation` | **Date**: 2026-05-22 | **Spec**: `specs/059-deterministic-policy-generation/spec.md`
**Input**: Feature specification from `/specs/059-deterministic-policy-generation/spec.md`

## Summary

Replace the hardcoded Go maps and runtime policy generation in `internal/build/policy.go` with a declarative component file system. Policy components are YAML files embedded in the binary, fetched from a catalog repo, or placed locally by the user. The `build refresh` command assembles matching components into a deterministic `openshell/policy.yaml` ordered alphabetically by component name.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), go:embed (component embedding)
**Storage**: Filesystem (embedded YAML components, cached catalog, user-local overrides)
**Testing**: `go test` via `make test`, testify/assert + testify/require
**Target Platform**: Linux/macOS CLI
**Project Type**: CLI tool
**Performance Goals**: N/A (offline file assembly, sub-second)
**Constraints**: Byte-identical output for same inputs (FR-001), OpenShell 0.0.46 compliance (FR-010)
**Scale/Scope**: ~15 component files at launch, extensible

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + documentation | PASS | Plan includes unit tests for component loading, matching, assembly, and determinism. CLI reference and config reference updates planned. |
| II. Interface contracts | PASS | No new interface backend. Component file format is a new contract (documented in `contracts/`). |
| III. Build and tool rules | PASS | Uses `make test`/`make lint`. No direct `go build`. Uses `internal/xdg` for paths. No Docker references. |

No violations. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/059-deterministic-policy-generation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── component-file-format.md
└── tasks.md
```

### Source Code (repository root)

```text
cc-deck/internal/build/
├── policy.go              # Refactored: component loading + assembly (replaces hardcoded maps)
├── policy_test.go         # Refactored: component-based tests
├── policies/              # NEW: embedded component YAML files
│   ├── claude-code.yaml
│   ├── git-hosting.yaml
│   ├── rust.yaml
│   ├── go.yaml
│   ├── node.yaml
│   ├── python.yaml
│   └── vertex-ai.yaml
├── component.go           # NEW: component file schema, loading, matching
├── component_test.go      # NEW: component unit tests
├── embed.go               # MODIFIED: add //go:embed policies/*.yaml
└── catalog.go             # NEW: catalog fetching for capture command

cc-deck/internal/cmd/
├── build.go               # MODIFIED: build refresh writes openshell/policy.yaml
└── (capture command)      # MODIFIED: catalog fetch step added

tests/
└── (existing test infrastructure)
```

**Structure Decision**: Single package extension in `internal/build/`. Component files embedded alongside existing templates. No new packages needed since all policy logic already lives in `build`.
