# Implementation Plan: Container Environment

**Branch**: `024-container-env` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/024-container-env/spec.md`

## Summary

Implement the `container` environment type using `podman run` for single-container lifecycle management. This includes a shared `internal/podman/` interaction package, definition/state file separation (`environments.yaml` for declarations, `state.yaml` for runtime state), credential injection via podman secrets, file transfer via `podman cp`, and reconciliation via `podman inspect`. The `EnvironmentTypePodman` and `PodmanFields` types are renamed to `EnvironmentTypeContainer` and `ContainerFields`.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML parsing), client-go v0.35.2 (K8s, existing)
**Storage**: YAML files: `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (definitions), `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state)
**Testing**: `go test` via `make test`, table-driven tests with testify where used
**Target Platform**: macOS (darwin/arm64, darwin/amd64), Linux (linux/amd64, linux/arm64)
**Project Type**: CLI tool
**Performance Goals**: env create < 30s (excluding image pull), env list < 2s with reconciliation
**Constraints**: Must work with rootless podman (default on developer workstations)
**Scale/Scope**: < 10 environments per user (single-file YAML approach)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | CLI-only feature (Go), no Rust/WASM plugin changes |
| II. Plugin Installation | N/A | CLI feature, no WASM build needed |
| III. WASM Filename | N/A | No WASM changes |
| IV. Host Function Gating | N/A | No WASM changes |
| V. Zellij API Research | N/A | No Zellij plugin API usage |
| VI. Build via Makefile | PASS | Will use `make test`, `make lint` |
| VII. Simplicity | PASS | Shared `internal/podman/` is justified (compose will reuse). No premature abstractions. |
| VIII. Documentation Freshness | NOTED | Must update README, Antora docs, landing page after implementation |
| IX. Spec Tracking | NOTED | Must add to README spec table |
| X. Release Process | N/A | Not applicable during implementation |
| XI. Prose Plugin | NOTED | Must use prose plugin for all documentation text |

**Gate result**: PASS (no violations)

**Post-Phase 1 re-check**: PASS. The design introduces `internal/podman/` as a shared package (justified: compose will reuse it). No interface signature changes (type-specific options on struct fields). No new abstractions beyond what is needed.

## Project Structure

### Documentation (this feature)

```text
specs/024-container-env/
  plan.md              # This file
  research.md          # Phase 0 output
  data-model.md        # Phase 1 output
  quickstart.md        # Phase 1 output
  contracts/           # Phase 1 output
  tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/
  podman/              # NEW: Shared podman interaction layer
    podman.go          # Detection, availability check, command runner
    container.go       # Run, Start, Stop, Remove, Inspect
    volume.go          # VolumeCreate, VolumeRemove
    secret.go          # SecretCreate, SecretRemove, SecretExists
    exec.go            # Exec (interactive + non-interactive), Cp
    types.go           # RunOpts, ContainerInfo, etc.
  env/
    container.go       # NEW: ContainerEnvironment implementation
    definition.go      # NEW: Definition store (environments.yaml)
    definition_test.go # NEW: Definition store tests
    container_test.go  # NEW: ContainerEnvironment tests
    interface.go       # MODIFIED: Extend CreateOpts, add DeleteOpts
    types.go           # MODIFIED: Rename PodmanFields -> ContainerFields,
                       #   EnvironmentTypePodman -> EnvironmentTypeContainer
    factory.go         # MODIFIED: Add container case
    state.go           # MODIFIED: State schema v2 (slim instance records)
    errors.go          # MODIFIED: Add ErrPodmanNotFound
  cmd/
    env.go             # MODIFIED: Add container-specific flags
                       #   (--image, --port, --storage, --path,
                       #    --credential, --keep-volumes, --all-ports)
  config/
    config.go          # MODIFIED: Extend Defaults for container settings
```
