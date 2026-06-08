# Implementation Plan: Credential Transport Abstraction

**Branch**: `069-credential-transport` | **Date**: 2026-06-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/069-credential-transport/spec.md`

## Summary

Replace the hardcoded Claude-specific credential detection and injection scattered across `ws/auth.go`, `ssh/credentials.go`, and `openshell/credentials.go` with an agent-declared `CredentialSpecs()` method on the Agent interface. A shared `internal/credential` package handles resolution, validation, and transport across all six workspace types. Users select an auth mode during `ws new` (prompted if multiple modes are available), the mode is persisted in the workspace definition, and credentials are validated eagerly at workspace start.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (config), client-go v0.35.2 (K8s), `internal/agent` (agent registry), `internal/ws` (workspace management)
**Storage**: YAML files at `$XDG_CONFIG_HOME/cc-deck/workspaces.yaml` (definitions), `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state)
**Testing**: `go test ./...` via `make test`
**Target Platform**: Linux/macOS (CLI), wasm32-wasip1 (plugin, not affected by this feature)
**Project Type**: CLI tool
**Performance Goals**: Credential validation completes within 2 seconds (SC-003)
**Constraints**: Never log credential values (FR-016). All credential files use 0600 permissions (FR-017). Use `make install`/`make test`/`make lint`, never `go build` directly.
**Scale/Scope**: Single-user tool. No migration of existing workspaces needed.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Plan includes unit tests for credential package, integration tests for each workspace type. Documentation tasks cover README, CLI reference (`--auth-mode`), and configuration reference. |
| II. Interface implementations satisfy behavioral contracts | PASS | New `CredentialSpecs()` method extends the Agent interface. Existing agents (Claude, OpenCode) will implement it. Contract documented in `contracts/`. |
| III. Build and tool rules | PASS | Uses `make install`/`make test`/`make lint`. Uses `internal/xdg` for paths. Uses podman exclusively. |

## Project Structure

### Documentation (this feature)

```text
specs/069-credential-transport/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── credential-transport.md
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/
├── agent/
│   ├── agent.go              # Agent interface (add CredentialSpecs())
│   ├── claude.go             # Claude agent (add credential specs)
│   ├── opencode.go           # OpenCode agent (add credential specs)
│   └── credential_spec.go    # NEW: CredentialSpec type definitions
├── credential/               # NEW: shared credential transport package
│   ├── resolve.go            # Detect available auth modes from host env
│   ├── validate.go           # Eager validation at workspace start
│   ├── transport.go          # Inject credentials into workspace types
│   ├── resolve_test.go
│   ├── validate_test.go
│   └── transport_test.go
├── ws/
│   ├── auth.go               # REFACTOR: delegate to credential package
│   ├── container.go          # MODIFY: use credential package
│   ├── compose.go            # MODIFY: use credential package
│   ├── ssh.go                # MODIFY: use credential package
│   ├── openshell.go          # MODIFY: use credential package
│   ├── k8s_deploy.go         # MODIFY: use credential package
│   ├── k8s_credentials.go    # MODIFY: use credential package
│   ├── definition.go         # MODIFY: add AuthMode field to WorkspaceSpec
│   └── types.go              # MODIFY: add agent field to WorkspaceInstance
├── ssh/
│   └── credentials.go        # REFACTOR: delegate to credential package
├── openshell/
│   └── credentials.go        # REFACTOR: remove KnownProviderProfiles
└── cmd/
    └── ws.go                  # MODIFY: add --auth-mode flag, agent-aware flow
```

**Structure Decision**: The new `internal/credential` package centralizes credential resolution, validation, and transport. It depends on `internal/agent` for CredentialSpec data but has no dependency on workspace types. Each workspace type calls into the credential package during Create/Start.

## Design Decisions

### D-001: New package vs. extending existing

**Decision**: Create `internal/credential` as a new package.
**Rationale**: The credential resolution logic is shared across six workspace types plus SSH and OpenShell. Putting it in `ws/auth.go` would create a growing file. A dedicated package provides clear separation and testability.
**Alternatives**: Extending `ws/auth.go` (rejected: would grow too large and couple credential logic to workspace types).

### D-002: CredentialSpec lives in agent package

**Decision**: `CredentialSpec` type defined in `internal/agent/credential_spec.go`, returned by `Agent.CredentialSpecs()`.
**Rationale**: Credential specs are an intrinsic property of each agent. Defining them in the agent package avoids circular dependencies (credential package imports agent, not the reverse).
**Alternatives**: Defining in credential package (rejected: would require agent to import credential, creating coupling in the wrong direction).

### D-003: Agent field in workspace definition

**Decision**: Add an `Agent` field (string, agent name) to `WorkspaceSpec` alongside the existing `Auth` field.
**Rationale**: The workspace needs to know which agent's credential specs to use. Currently agent is implicit (all Claude). The `Auth` field becomes the selected CredentialSpec name.
**Alternatives**: Inferring agent from auth mode (rejected: multiple agents can share auth mode names).

### D-004: Gradual refactoring via delegation

**Decision**: Keep `ws/auth.go`, `ssh/credentials.go`, and `openshell/credentials.go` as thin wrappers that delegate to the new credential package during the transition, then inline/remove in a follow-up.
**Rationale**: Minimizes risk of regression in existing flows. Each workspace type can be migrated independently.
**Alternatives**: Big-bang replacement (rejected: higher regression risk for a credential-sensitive feature).

## Complexity Tracking

No constitution violations. No complexity justifications needed.
