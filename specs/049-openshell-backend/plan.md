# Implementation Plan: OpenShell Backend for cc-deck

**Branch**: `049-openshell-backend` | **Date**: 2026-04-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/049-openshell-backend/spec.md`

## Summary

Add a new `openshell` workspace type to cc-deck that runs Claude Code sessions inside OpenShell sandboxes. The backend communicates with the OpenShell gateway via gRPC, provisions sandboxes with a custom image containing Zellij, and uses SSH tunnels for session attach and file sync. Targets the Podman compute driver for local development.

## Technical Context

**Language/Version**: Go 1.25 (cc-deck CLI)
**Primary Dependencies**: OpenShell gRPC proto (Go client codegen), existing cc-deck ws package
**Storage**: cc-deck FileStateStore (`~/.local/state/cc-deck/state.yaml`) + DefinitionStore (`~/.config/cc-deck/`)
**Testing**: Go standard testing + testify v1.11.1, cc-deck behavioral contract tests
**Target Platform**: Linux, macOS (developer workstations with Podman)
**Project Type**: CLI tool backend (new workspace type in existing Go project)
**Performance Goals**: Sandbox create <30s, attach <5s, 100MB file sync <30s
**Constraints**: OpenShell gateway must be running locally with Podman driver. gRPC proto files pinned to a specific gateway release tag and vendored into cc-deck. All gRPC call outcomes logged at debug level (FR-014). Credentials handled by gateway provider mechanism, not by cc-deck.
**Scale/Scope**: Single developer workstation, 1-5 concurrent sandboxes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**I. Tests and documentation**: Plan includes test tasks (Phase 9, T030-T035) and documentation task (T028 workspace definition schema). README, CLI reference, Antora guide page, and configuration reference updates needed as part of implementation. **PASS** (tasks exist; documentation deliverables tracked).

**II. Interface behavioral contracts**: This feature implements a new backend for the `Workspace` and `InfraManager` interfaces. Contract document exists at `contracts/workspace-interface.md`. Existing implementations (SSH, container, k8s-deploy) were reviewed during spec creation. **PASS**.

**III. Build and tool rules**: Plan uses `make test`/`make lint` (not direct `go build`). XDG paths use `internal/xdg`. Container runtime is Podman. Proto codegen uses `protoc` (acceptable, not a Go build). **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/049-openshell-backend/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── workspace-interface.md
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (cc-deck repository)

```text
cc-deck/internal/ws/
├── openshell.go              # OpenShellWorkspace struct + Workspace interface impl
├── openshell_test.go         # Unit tests
├── channel_openshell.go      # PipeChannel, DataChannel, GitChannel for OpenShell
├── channel_openshell_test.go # Channel tests
├── types.go                  # (modify) Add WorkspaceTypeOpenShell
├── factory.go                # (modify) Add openshell case to NewWorkspace

cc-deck/internal/openshell/
├── client.go                 # gRPC client wrapper for OpenShell gateway
├── client_test.go            # Client tests
├── proto/                    # Generated Go code from OpenShell proto files
│   └── *.pb.go

cc-deck/internal/cmd/
├── ws.go                     # (modify) Add openshell type validation and attach flow

cc-deck/build/
├── Dockerfile.openshell      # Sandbox image with Zellij + Claude Code
```

**Structure Decision**: Follows cc-deck's existing pattern. New workspace type in `internal/ws/`. New gRPC client package in `internal/openshell/`. Sandbox image Dockerfile in `build/`.

## Complexity Tracking

No constitution violations to justify.
