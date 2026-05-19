# Implementation Plan: OpenShell Credential Injection

**Branch**: `058-openshell-credential-injection` | **Date**: 2026-05-19 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/058-openshell-credential-injection/spec.md`

## Summary

Bridge cc-deck's credential system to OpenShell's provider architecture. Add a `credentials` section to `build.yaml`, detect credentials during capture, create OpenShell providers at workspace creation time, and handle file-based credentials (Vertex) via runtime upload. Extends the manifest data model, OpenShell client interface, workspace Create flow, capture command, and policy generation.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), internal/openshell (OpenShell client)
**Storage**: YAML files (`build.yaml` manifest, `workspaces.yaml` definitions)
**Testing**: `go test` via `make test`, testify v1.11.1
**Target Platform**: macOS/Linux host, OpenShell gateway (local or remote)
**Project Type**: CLI tool
**Constraints**: No new external dependencies. Reuse existing credential patterns from `internal/ssh/credentials.go`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + documentation | PASS | Unit tests for new credential types, CLI reference for `credentials` section, config reference update planned |
| II. Interface contracts | PASS | Extending `openshell.Client` interface with provider methods; existing implementations updated |
| III. Build and tool rules | PASS | Using `make test`, `make lint`; `internal/xdg` for paths; `podman` only |

## Project Structure

### Documentation (this feature)

```text
specs/058-openshell-credential-injection/
├── plan.md              # This file
├── research.md          # Phase 0: integration research
├── data-model.md        # Phase 1: credential entry model
├── contracts/           # Phase 1: provider client contract
└── tasks.md             # Phase 2: task breakdown
```

### Source Code (repository root)

```text
cc-deck/internal/
├── build/
│   ├── manifest.go          # ADD: CredentialEntry type, Credentials field
│   ├── policy.go            # MODIFY: add Vertex GCP endpoints
│   └── commands/
│       └── cc-deck.capture.md  # MODIFY: add credential detection step
├── openshell/
│   ├── iface.go             # MODIFY: add provider management methods
│   ├── client.go            # MODIFY: implement provider CLI wrapping
│   └── credentials.go       # NEW: credential-to-provider mapping
├── ws/
│   ├── openshell.go         # MODIFY: provider creation in Create flow
│   └── definition.go        # (no changes, credentials already in WorkspaceSpec)
└── ssh/
    └── credentials.go       # REFERENCE: reuse BuildCredentialSet pattern
```

**Structure Decision**: Extends existing packages. New file `openshell/credentials.go` contains the credential-to-provider mapping logic (detecting auth mode, mapping env vars to provider types, handling file uploads). This keeps the OpenShell client focused on CLI wrapping while credential logic is self-contained.

## Complexity Tracking

No constitution violations. All changes follow existing patterns.
