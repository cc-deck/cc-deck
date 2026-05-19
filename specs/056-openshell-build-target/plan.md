# Implementation Plan: OpenShell Build Target

**Branch**: `056-openshell-build-target` | **Date**: 2026-05-15 | **Spec**: `specs/056-openshell-build-target/spec.md`
**Input**: Feature specification from `specs/056-openshell-build-target/spec.md`

## Summary

Extend the cc-deck manifest-driven build system with an `openshell` target that generates OpenShell-compatible container images. The implementation adds an `OpenShellTarget` struct to the manifest schema, extends the `/cc-deck.build` command with a new Section C for OpenShell provisioning, and generates a `policy.yaml` from `network.allowed_domains` with per-binary scoping. The CLI's `detectRunTarget()` and `build init --target` are extended to handle the new target type.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), go:embed (template/command embedding)
**Storage**: Filesystem (generated `openshell/Containerfile`, `openshell/policy.yaml`, `openshell/build.sh` in setup directory)
**Testing**: `go test` via `make test`, testify v1.11.1
**Target Platform**: macOS (CLI host), Linux containers (build output)
**Project Type**: CLI tool extension
**Performance Goals**: N/A (build-time tooling)
**Constraints**: Must not break existing `container` and `ssh` targets. Generated policy must be valid OpenShell policy schema v1.
**Scale/Scope**: ~5 files modified in Go, 1 new command spec file, manifest template update

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests and documentation | PASS | Unit tests for new manifest structs, policy generation logic. CLI reference updated for `--target openshell`. README updated. |
| Interface behavioral contracts | N/A | No new interface implementations. Extending existing `TargetsConfig` struct. |
| Build and tool rules | PASS | Use `make test`, `make lint`. No direct `go build`. Use `podman` for image builds. |

## Project Structure

### Documentation (this feature)

```text
specs/056-openshell-build-target/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── policy-schema.md # OpenShell policy YAML contract
└── tasks.md             # Phase 2 output (NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cc-deck/
├── internal/
│   ├── build/
│   │   ├── manifest.go          # MODIFIED: Add OpenShellTarget, PolicyConfig structs
│   │   ├── manifest_test.go     # MODIFIED: Add tests for new structs
│   │   ├── init.go              # MODIFIED: Support --target openshell scaffolding
│   │   ├── init_test.go         # MODIFIED: Test openshell init
│   │   ├── embed.go             # UNCHANGED (already embeds commands/templates)
│   │   ├── policy.go            # NEW: Policy generation and merge logic
│   │   ├── policy_test.go       # NEW: Policy generation tests
│   │   ├── commands/
│   │   │   └── cc-deck.build.md # MODIFIED: Add Section C for OpenShell build
│   │   └── templates/
│   │       └── build.yaml.tmpl  # MODIFIED: Add commented openshell target section
│   ├── cmd/
│   │   └── build.go             # MODIFIED: Extend detectRunTarget, add runOpenShellBuild
│   └── openshell/
│       └── default-policy.yaml  # REFERENCE: Existing default policy template
├── docs/
│   └── modules/reference/pages/
│       └── cli.adoc             # MODIFIED: Document --target openshell
└── README.md                    # MODIFIED: Mention openshell target
```

**Structure Decision**: Follows existing project structure. New policy generation logic goes in `cc-deck/internal/build/policy.go` alongside the existing manifest handling. The build command spec is extended with a new section (Section C) following the pattern of Section A (container) and Section B (SSH).

## Complexity Tracking

No constitution violations. All changes extend existing patterns.
