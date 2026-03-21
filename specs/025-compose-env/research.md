# Research: Compose Environment

**Feature**: 025-compose-env | **Date**: 2026-03-21

## Summary

All design decisions were resolved during the interactive brainstorming session (brainstorm/029-compose-environment.md). No NEEDS CLARIFICATION items remain. This document records the resolved decisions, rationale, and alternatives considered.

## R-01: Code Reuse Strategy

**Decision**: Shared helper functions + compose uses `internal/podman` directly.

**Rationale**: Compose and container environments share auth detection and Zellij session checking logic, but differ significantly in lifecycle management (compose CLI vs single-container podman commands). Extracting shared helpers avoids duplication while keeping the implementations independent.

**Alternatives considered**:
- Embedding ContainerEnvironment inside ComposeEnvironment: Rejected because the lifecycle methods differ too much. Compose uses `podman-compose up/down/start/stop` while container uses `podman run/start/stop/rm`. Embedding would create a confusing delegation chain.
- Copy-paste the shared code: Rejected due to maintenance burden. Auth detection logic should remain in one place.

## R-02: Credential Injection Mechanism

**Decision**: Use `.cc-deck/.env` file for environment variable credentials; use bind mounts via compose volumes for file-based credentials (e.g., ADC files).

**Rationale**: The container type injects credentials via `podman secret create` + `--secret` flags. Compose has its own secrets mechanism, but it adds complexity. The `.env` file approach is compose-native (`env_file: [".env"]`), simpler, and achieves the same end result (credentials available inside the container). File-based credentials are copied to `.cc-deck/secrets/` and mounted via compose volumes. The detection logic (what credentials to inject) is identical to the container type via shared helpers.

**Alternatives considered**:
- Using compose-level `secrets:` declaration: More complex, requires podman secret creation before compose up, and the compose secrets syntax varies between runtimes.
- Using podman secrets directly with `--secret` on compose services: Not portable across compose runtimes.

## R-03: Storage Default

**Decision**: Host-path bind mount is the default storage.

**Rationale**: Compose environments are project-local. The user is already in the project directory. Bind mounting provides immediate bidirectional file sync without explicit push/pull. This is the natural model for project-local development.

**Alternatives considered**:
- Named volume as default (matching container type): Rejected because the project-local model implies the user wants to work on their existing project files, not start fresh.
- Rsync-based sync: Over-engineered for the current use case. Bind mounts work well for typical project sizes.

## R-04: Generated Files Location

**Decision**: `.cc-deck/` subdirectory within the project directory.

**Rationale**: Project-local storage keeps all generated artifacts visible and co-located with the project. The `.cc-deck/` directory is gitignored. On delete, the entire directory is removed, leaving no orphaned files.

**Alternatives considered**:
- `$XDG_STATE_HOME/cc-deck/compose/<name>/`: Would separate generated files from the project. Rejected because the compose runtime needs the files at a known relative path for volume mounts.
- `.cc-mux/` or other name: `.cc-deck/` matches the project name and is consistent with the dot-directory convention.

## R-05: Compose Runtime Detection

**Decision**: Auto-detect with `podman-compose` preferred. Detection order: `podman-compose` > `docker compose` (v2) > `docker-compose` (v1 legacy).

**Rationale**: The project uses podman exclusively, so `podman-compose` is the natural default. Fallback to docker compose supports users who have that installed instead.

**Alternatives considered**:
- Only support podman-compose: Too restrictive. Some users have docker compose installed.
- Only support docker compose: Contradicts the project's podman-first philosophy.

## R-06: EnvironmentInstance Type Field

**Decision**: Add `Type EnvironmentType` field to `EnvironmentInstance`.

**Rationale**: The current `EnvironmentInstance` struct (v2 state) has no `Type` field, relying on the presence of a `Container` or `K8s` field to infer the type. With compose environments also using v2 instances, explicit type storage is cleaner and more robust. Existing instances without a Type field default to "container" for backwards compatibility.

**Alternatives considered**:
- Infer type from which Fields struct is populated: Works but fragile. A compose instance with `Compose: nil` (error state) would be unclassifiable.
- Look up the definition store: Adds an I/O dependency to type resolution and fails if the definition is deleted before the instance.

## R-07: Push/Pull for Compose

**Decision**: Use `podman exec` + tar pipe for push/pull operations.

**Rationale**: `podman cp` does not work with compose service names (only container names/IDs). Using exec + tar is the same mechanism used by K8s environments and is reliable across all container runtimes. For bind-mount storage, push/pull is redundant (files are already synced), but should still work correctly for the named-volume case.

**Alternatives considered**:
- Use `podman cp` with the container name directly: Works if we know the container name, but the tar approach is more consistent with the rest of the codebase.
- No-op for bind mounts: Could confuse users. Better to have a consistent interface.

## R-08: Reconciliation Strategy

**Decision**: Compose environment reconciliation uses `podman inspect` on the session container name, identical to container type.

**Rationale**: The compose CLI does not provide a clean way to check project status. `podman inspect cc-deck-<name>` on the session container gives us running/stopped/missing state, which is sufficient for reconciliation. The proxy sidecar state is secondary; if the session container is running, we report running.

**Alternatives considered**:
- `podman-compose ps`: Output parsing is fragile and format varies by version.
- Check both session and proxy containers: Over-complex. If session is running, the environment is usable.
