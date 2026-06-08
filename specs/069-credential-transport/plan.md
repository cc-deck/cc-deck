# Implementation Plan: Credential Transport Abstraction

**Branch**: `069-credential-transport` | **Date**: 2026-06-08 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/069-credential-transport/spec.md`

## Summary

Replace the hardcoded Claude-specific credential detection scattered across `ws/auth.go`, `ssh/credentials.go`, and `openshell/credentials.go` with an agent-declared `CredentialSpecs()` method on the Agent interface. A shared `internal/credential` package handles resolution, validation, and transport across all six workspace types.

**Revised model (2026-06-08)**: Workspaces are not tied to a single agent. The credential system scans ALL registered agents, detects ALL available credentials, and injects everything. No conflict resolution or exclusion mechanism; all available modes are injected.

### What Already Exists (from initial implementation)

The following are already implemented and tested:
- `CredentialSpec`, `EnvVarSpec`, `FileCredentialSpec`, `Endpoint` types in `internal/agent/credential_spec.go`
- `CredentialSpecs()` on Agent interface, implemented for Claude (api/vertex/bedrock) and OpenCode (openai/anthropic)
- `internal/credential` package: `Detect()`, `Resolve()`, `Validate()`, transport functions (InjectContainer, InjectSSH, InjectK8s, InjectOpenShell)
- Dual-path credential injection in container.go, compose.go, ssh.go, k8s_deploy.go (new path + legacy fallback)
- AUTH column in `ws ls -v` (verbose only)
- `ws update --auth-mode` for switching

### What Needs to Change

The current implementation assumes a 1:1 workspace-to-agent binding (`--agent` flag, single `AuthMode` field). The revised model needs:
1. Replace single-agent detection with detect-all-agents scan
2. Remove `agent`/`auth-mode` fields from workspace definition
3. Remove `--agent` and `--auth-mode` flags from `ws new`
4. Inject all detected credentials without prompting

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (config), client-go v0.35.2 (K8s), `internal/agent` (agent registry), `internal/ws` (workspace management)
**Storage**: YAML files at `$XDG_CONFIG_HOME/cc-deck/workspaces.yaml` (definitions), `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state)
**Testing**: `go test ./...` via `make test`
**Target Platform**: Linux/macOS (CLI), wasm32-wasip1 (plugin, not affected by this feature)
**Project Type**: CLI tool
**Performance Goals**: Credential detection and validation completes within 2 seconds (SC-003)
**Constraints**: Never log credential values (FR-016). All credential files use 0600 permissions (FR-017). Use `make install`/`make test`/`make lint`, never `go build` directly.
**Scale/Scope**: Single-user tool. No migration of existing workspaces needed.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Existing tests for credential package. New tests needed for detect-all logic, conflict detection, and opt-out. Documentation tasks cover README, CLI reference, and config reference. |
| II. Interface implementations satisfy behavioral contracts | PASS | `CredentialSpecs()` method already on Agent interface and implemented. Contract documented in `contracts/`. |
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
│   ├── agent.go              # Agent interface (CredentialSpecs() already added)
│   ├── claude.go             # Claude agent (credential specs already added)
│   ├── opencode.go           # OpenCode agent (credential specs already added)
│   └── credential_spec.go   # CredentialSpec types (already exists)
├── credential/               # Shared credential package (already exists)
│   ├── resolve.go            # Detect(), Resolve(), DetectAll(), MergeCredentials()
│   ├── validate.go           # Validate() (MODIFY: validate all non-excluded)
│   ├── transport.go          # Inject* functions (no changes needed)
│   ├── resolve_test.go       # (MODIFY: add DetectAll tests)
│   ├── validate_test.go      # (MODIFY: add multi-mode validation tests)
│   └── transport_test.go
├── ws/
│   ├── auth.go               # DEPRECATED: delegate to credential package
│   ├── container.go          # MODIFY: use DetectAll instead of single-agent
│   ├── compose.go            # MODIFY: use DetectAll instead of single-agent
│   ├── ssh.go                # MODIFY: use DetectAll instead of single-agent
│   ├── k8s_deploy.go         # MODIFY: use DetectAll instead of single-agent
│   ├── openshell.go          # MODIFY: use DetectAll instead of single-agent
│   ├── definition.go         # MODIFY: remove agent/auth-mode fields
│   └── types.go              # MODIFY: remove agent field from WorkspaceInstance
├── ssh/
│   └── credentials.go        # REFACTOR: delegate to credential package
├── openshell/
│   └── credentials.go        # REFACTOR: delegate to credential package
└── cmd/
    └── ws.go                  # MODIFY: remove --agent and --auth-mode flags
```

**Structure Decision**: The `internal/credential` package centralizes credential resolution, validation, and transport. The detect-all model adds a `DetectAll()` function that scans all agents and a `MergeCredentials()` function that combines results. Each workspace type calls `DetectAll()` once and injects the merged result.

## Design Decisions

### D-001: Detect-all/opt-out model (revised from single-agent)

**Decision**: At workspace creation, scan all registered agents, detect all available credentials, inject everything. Conflicts from same-agent modes resolved via opt-out.
**Rationale**: A workspace can host multiple agents simultaneously. The user shouldn't need to specify which agent's credentials to inject. The common case (one set of credentials) requires zero user interaction.
**Alternatives**: Single-agent binding (rejected after user feedback: workspaces are multi-agent).

### D-002: No conflict resolution

**Decision**: All detected modes are injected without conflict resolution or prompting. Same-agent modes with multiple available credentials (e.g., Claude "api" and "vertex") are all injected.
**Rationale**: Agents like OpenCode can hold multiple API keys simultaneously. Keeping all credentials available maximizes flexibility. If contradictory env vars cause issues, agents handle it internally. Exclusion/conflict resolution can be added later if needed.

### D-004: Gradual refactoring via delegation (kept from v1)

**Decision**: Keep `ws/auth.go`, `ssh/credentials.go`, and `openshell/credentials.go` as thin wrappers during the transition, then inline/remove in a follow-up.
**Rationale**: Minimizes regression risk. Each workspace type can be migrated independently.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
