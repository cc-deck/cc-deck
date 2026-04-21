# Implementation Plan: Environment-to-Workspace Internal Rename

**Branch**: `040-env-to-workspace-rename` | **Date**: 2026-04-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/040-env-to-workspace-rename/spec.md`

## Summary

Rename all internal Go types, constants, functions, and the package path from "environment"/"env" terminology to "workspace"/"ws" to match the CLI's existing `ws` command vocabulary. Also rename the config file from `environments.yaml` to `workspaces.yaml`, the YAML key from `environments:` to `workspaces:`, the env var from `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE`, and update all user-facing error messages and build command descriptions. No backward compatibility.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3, client-go v0.35.2  
**Storage**: YAML files (`workspaces.yaml` for definitions, `state.yaml` for runtime state)  
**Testing**: `make test` (Go test), `make lint` (Go vet + clippy)  
**Target Platform**: Linux/macOS CLI  
**Project Type**: CLI tool  
**Performance Goals**: N/A (mechanical rename)  
**Constraints**: Must not rename Docker Compose `Environment` field or OS env var references  
**Scale/Scope**: 41 files in `internal/env/`, 5 consumer files, ~218 package references, ~25 error messages, ~10 build descriptions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Go CLI only, no plugin changes |
| II. Plugin Installation | N/A | No plugin changes |
| III. WASM Filename | N/A | No WASM changes |
| IV. WASM Host Function Gating | N/A | No WASM changes |
| V. Zellij API Research | N/A | No Zellij API usage |
| VI. Build via Makefile Only | PASS | Will use `make test`, `make lint` |
| VII. Interface Behavioral Contracts | PASS | Interface renamed but behavior unchanged |
| VIII. Simplicity | PASS | Pure rename, no new abstractions |
| IX. Documentation Freshness | DEFERRED | Docs update out of scope per spec assumptions |
| X. Spec Tracking in README | PASS | Will update spec table |
| XI. Release Process | N/A | No release |
| XII. Prose Plugin | N/A | No docs content created |
| XIII. XDG Paths | PASS | Path logic unchanged, only variable name changes |
| XIV. No Dotfile Nesting | N/A | No dotfiles affected |

## Project Structure

### Documentation (this feature)

```text
specs/040-env-to-workspace-rename/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (affected areas)

```text
cc-deck/internal/
├── env/           # RENAME TO ws/ (41 files, package env -> package ws)
│   ├── types.go          # EnvironmentType/State/Instance -> Workspace*
│   ├── interface.go      # Environment interface -> Workspace
│   ├── definition.go     # EnvironmentDefinition, DefinitionFile, env var, filename
│   ├── factory.go        # NewEnvironment -> NewWorkspace
│   ├── state.go          # Error messages, AllProjectEnvironmentNames
│   ├── validate.go       # ValidateEnvName -> ValidateWsName
│   ├── local.go          # LocalEnvironment -> LocalWorkspace
│   ├── container.go      # ContainerEnvironment -> ContainerWorkspace
│   ├── compose.go        # ComposeEnvironment -> ComposeWorkspace
│   ├── ssh.go            # SSHEnvironment -> SSHWorkspace
│   ├── k8s_deploy.go     # K8sDeployEnvironment -> K8sDeployWorkspace
│   └── *_test.go         # All test files: update types + error assertions
├── cmd/
│   ├── ws.go             # 120 env.* references -> ws.*
│   ├── ws_new_test.go    # 68 env.* references + CC_DECK_DEFINITIONS_FILE
│   ├── ws_resolve_test.go
│   ├── ws_prune_test.go
│   ├── compose_smoke_test.go
│   ├── ws_integration_test.go
│   └── build.go          # "AI-driven environment configuration" string
├── build/commands/
│   ├── cc-deck.build.md  # "Build environment" -> "Build workspace"
│   └── cc-deck.capture.md # "Capture environment" -> "Capture workspace"
├── build/templates/
│   └── build.yaml.tmpl   # "environment" references in template text
├── integration/
│   └── k8s_deploy_test.go # 24 env.* references
└── e2e/
    └── ws_test.go         # CC_DECK_STATE_FILE (unchanged, no env rename)
```

**Structure Decision**: No structural changes beyond renaming `internal/env/` to `internal/ws/`.

## Implementation Strategy

The rename is mechanical and can be executed in three ordered phases:

### Phase 1: Package rename and type/constant/function renames

Move `internal/env/` to `internal/ws/`. Rename all Go identifiers (types, constants, functions, methods) from Environment/Env prefix to Workspace/Ws prefix within the package. Change `package env` to `package ws` in all files. This is the bulk of the work (41 files).

**Approach**: Use `git mv` for directory rename, then sed/IDE refactor for identifier renames within files. The package-internal references update automatically with the package rename; only the exported identifier names need explicit renaming.

**DO NOT rename**: `EnvironmentDefinition.Env` field, Docker Compose `Environment` field, OS env var references, `composeEnvFile` constant.

### Phase 2: Update all consumers

Update all import paths from `internal/env` to `internal/ws` and all `env.` qualifiers to `ws.` in the 5 consumer files. Update the renamed type/constant/function references.

### Phase 3: Config, env var, strings, and descriptions

- Change `definitionFileName` constant to `"workspaces.yaml"`
- Change `DefinitionFile.Environments` field to `Workspaces` with `yaml:"workspaces"` tag
- Change `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE`
- Update all user-facing error messages (~25 occurrences)
- Update build command descriptions in `.md` files and `build.yaml.tmpl`
- Update CLI help text in `ws.go`

### Phase 4: Test updates and verification

- Update test assertions that check error message strings
- Update test references to `CC_DECK_DEFINITIONS_FILE` -> `CC_DECK_WORKSPACES_FILE`
- Update test references to `environments.yaml` -> `workspaces.yaml`
- Update test inline YAML with `environments:` -> `workspaces:` key
- Run `make test` and `make lint`

## Research Basis

Research was conducted by 3 parallel agents exploring: (1) complete type/constant/function inventory in `internal/env/`, (2) all import consumers across the codebase, (3) all user-facing strings, config references, and env vars. Findings are consolidated in [research.md](research.md).
