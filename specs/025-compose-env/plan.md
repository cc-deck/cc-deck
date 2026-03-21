# Implementation Plan: Compose Environment

**Branch**: `025-compose-env` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/025-compose-env/spec.md`

## Summary

Add a `compose` environment type that uses `podman-compose` for multi-container orchestration. The compose environment is project-local (generated files in `.cc-deck/`), uses bind mounts by default for immediate bidirectional file sync, and optionally adds a tinyproxy sidecar for network filtering via `--allowed-domains`. The implementation reuses existing compose YAML generation, proxy config generation, domain resolution, and auth detection, while adding compose CLI lifecycle management and the `ComposeEnvironment` struct that implements the `Environment` interface.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), internal/xdg (XDG paths), internal/podman (container interaction), internal/compose (YAML generation), internal/network (domain resolution)
**Storage**: YAML files at `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (definitions) and `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state). Project-local `.cc-deck/` directory for generated compose artifacts.
**Testing**: `go test` (unit tests), integration tests requiring podman + podman-compose
**Target Platform**: macOS + Linux (CLI tool)
**Project Type**: CLI tool
**Performance Goals**: Environment creation in <30s (excluding image pull, SC-001)
**Constraints**: Must use podman exclusively (constitution), XDG paths on all platforms (constitution Principle XIII), must satisfy Environment interface behavioral contract (constitution Principle VII)
**Scale/Scope**: Single-user CLI tool, single project directory per compose environment

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Go CLI only, no Rust plugin changes |
| II. Plugin Installation | N/A | Not modifying plugin |
| III. WASM Filename Convention | N/A | Not modifying plugin |
| IV. WASM Host Function Gating | N/A | Not modifying plugin |
| V. Zellij API Research Order | N/A | Reuses existing attach logic |
| VI. Build via Makefile Only | PASS | Using `make test`, `make lint` |
| VII. Interface Behavioral Contracts | PASS | Spec explicitly references environment-interface.md. All behavioral requirements apply. |
| VIII. Simplicity | PASS | Reuses existing code, minimal new abstractions |
| IX. Documentation Freshness | ACTION | Must update README, CLI reference, Antora docs |
| X. Spec Tracking in README | ACTION | Must add 025 to README spec table |
| XI. Release Process | N/A | Not releasing |
| XII. Prose Plugin for Documentation | ACTION | Must use prose plugin for all doc content |
| XIII. XDG Paths on All Platforms | PASS | Using `internal/xdg` package |

**Gate Result**: PASS. No violations. Three ACTION items tracked as implementation tasks (documentation).

**Post-Design Re-check**: PASS. Design uses existing packages, no new external dependencies, no new abstractions beyond `ComposeEnvironment` struct.

## Project Structure

### Documentation (this feature)

```text
specs/025-compose-env/
├── plan.md              # This file
├── research.md          # Phase 0: Design decisions resolved from brainstorm
├── data-model.md        # Phase 1: Entity model and state transitions
├── quickstart.md        # Phase 1: Usage guide for compose environments
├── contracts/           # Phase 1: No new contracts (implements existing Environment interface)
└── tasks.md             # Phase 2: Task breakdown (/speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/env/
├── auth.go              # NEW: Extracted shared auth detection helpers
├── compose.go           # NEW: ComposeEnvironment implementation
├── container.go         # MODIFIED: Remove auth functions (moved to auth.go)
├── types.go             # MODIFIED: Add EnvironmentTypeCompose, ComposeFields, Type field on EnvironmentInstance
├── factory.go           # MODIFIED: Add compose case
├── definition.go        # MODIFIED: Add AllowedDomains, ProjectDir fields
├── state.go             # UNCHANGED (v2 instance methods already sufficient)

cc-deck/internal/compose/
├── generate.go          # MODIFIED: Add volumes, stdin/tty, secrets directory mounts
├── proxy.go             # UNCHANGED
├── runtime.go           # NEW: Compose runtime detection (podman-compose, docker compose)

cc-deck/internal/cmd/
├── env.go               # MODIFIED: Add compose flags, update resolveEnvironment

cc-deck/internal/env/
├── compose_test.go      # NEW: Unit tests for ComposeEnvironment
├── auth_test.go         # NEW: Unit tests for extracted auth helpers

cc-deck/internal/compose/
├── runtime_test.go      # NEW: Unit tests for runtime detection
├── generate_test.go     # MODIFIED: Tests for updated generation
```

**Structure Decision**: Follows the existing Go package layout. New files are added to existing packages (`internal/env`, `internal/compose`). No new packages needed.

## Complexity Tracking

No constitution violations to justify. The design adds one new struct (`ComposeEnvironment`) and extracts shared helpers, following the established pattern.
