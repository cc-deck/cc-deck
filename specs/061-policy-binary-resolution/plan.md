# Implementation Plan: Policy Binary Resolution

**Branch**: `061-policy-binary-resolution` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/061-policy-binary-resolution/spec.md`

## Summary

Resolve binary paths at policy assembly time by cross-referencing policy component `match.tools` with the manifest's `tools` section and a well-known paths table. This replaces hardcoded binaries in embedded components, making catalog components installation-independent while ensuring the OpenShell supervisor knows which processes can access each endpoint.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: gopkg.in/yaml.v3 (YAML parsing, existing)
**Storage**: N/A (in-memory resolution during policy assembly)
**Testing**: `go test ./internal/build/...`
**Target Platform**: Linux/macOS CLI
**Project Type**: CLI tool (internal library change)
**Performance Goals**: <10ms resolution overhead (SC-003)
**Constraints**: No new dependencies, no manifest format changes

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Tests for resolution logic, README update for behavior change |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | PASS | Uses `make test`/`make lint`, no direct builds |

## Project Structure

### Documentation (this feature)

```text
specs/061-policy-binary-resolution/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cc-deck/internal/build/
├── policy.go              # MODIFIED: add resolution call in AssemblePolicy()
├── policy_binaries.go     # NEW: well-known paths table + resolveBinaries()
├── policy_binaries_test.go # NEW: tests for resolution logic
├── policy_test.go         # MODIFIED: add integration tests
└── policies/
    ├── go.yaml            # MODIFIED: remove binaries
    ├── rust.yaml           # MODIFIED: remove binaries
    ├── node.yaml           # MODIFIED: remove binaries
    ├── python.yaml         # MODIFIED: remove binaries
    ├── claude-code.yaml    # UNCHANGED: explicit binaries kept
    ├── git-hosting.yaml    # UNCHANGED: explicit binaries kept
    └── vertex-ai.yaml      # UNCHANGED: explicit binaries kept
```

**Structure Decision**: New file `policy_binaries.go` in the existing `internal/build/` package. Keeps resolution logic separate from the main assembly flow while staying in the same package for access to internal types.

## Design Decisions

### D1: Resolution Function Signature

```go
func resolveBinaries(components []PolicyComponent, manifest *Manifest) []PolicyComponent
```

Takes the matched components and manifest, returns components with binaries populated. Called in `AssemblePolicy()` between component matching and map building.

### D2: Well-Known Paths Table

Package-level `var wellKnownPaths = map[string][]string{...}` in `policy_binaries.go`. Each entry maps a tool name to additional path strings beyond `/usr/bin/<name>`. Includes glob patterns where appropriate (e.g., `/sandbox/.rustup/toolchains/*/bin/cargo`). Note: the table stores raw path strings; the resolution function converts them to `[]PolicyBinary` structs when setting `component.Binaries`.

### D3: Resolution Priority

1. If component has explicit `Binaries` (len > 0): skip, preserve as-is
2. Otherwise: resolve from manifest + well-known paths table
3. Deduplicate resolved paths

### D4: Embedded Component Changes

Remove `binaries:` from go.yaml, rust.yaml, node.yaml, python.yaml. These become pure endpoint definitions. Binary paths are resolved dynamically at assembly time.
