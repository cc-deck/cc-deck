# Behavioral Contract: Workspace Interface (Revised)

**Date**: 2026-04-25
**Spec**: 043-workspace-lifecycle
**Constitution Reference**: Principle VII (Interface Behavioral Contracts)

## Workspace Interface

All workspace types MUST implement these behaviors.

### Attach(ctx) error

1. If the workspace implements `InfraManager` and `InfraState` is "stopped", call `Start(ctx)` first. If Start fails, return the error without creating a session.
2. Check if a Zellij session exists (type-specific detection).
3. If no session exists, create one with the cc-deck layout. If the layout is not available, fall back to default layout and log a warning.
4. If a session exists, reattach to it (preserving all panes and tabs).
5. Update `LastAttached` timestamp.
6. Update `SessionState` to "exists".
7. Nested session detection: if already inside a Zellij session (ZELLIJ env var set for local, equivalent check for remote types), refuse to attach and return an error.

### KillSession(ctx) error

1. Check if a Zellij session exists (type-specific detection).
2. If no session exists, return nil with a user-facing message ("No session to kill").
3. If a session exists, kill it (type-specific kill mechanism).
4. Update `SessionState` to "none".
5. MUST NOT affect infrastructure state. If the workspace implements InfraManager, infra_state remains unchanged.

### Delete(ctx, force) error

1. If a Zellij session exists, kill it (call KillSession).
2. If the workspace implements InfraManager and infrastructure is running, stop it (call Stop).
3. Remove all workspace resources (type-specific cleanup).
4. Remove the workspace instance from the state store.
5. Remove the workspace definition (if separate from instance).
6. Without `force`: refuse to delete if infrastructure is running (for InfraManager types).
7. With `force`: proceed regardless of state.

### Status(ctx) (*WorkspaceStatus, error)

1. Check actual infrastructure state (type-specific, e.g., podman inspect, kubectl get). Set InfraState accordingly. For non-InfraManager types, InfraState is nil.
2. Check if a Zellij session exists. Set SessionState to "exists" or "none".
3. If stored state differs from actual state, update the stored state (reconciliation).
4. Return the current status with both dimensions.

## InfraManager Interface

Only workspace types that manage compute infrastructure implement these.

### Start(ctx) error

1. Start the underlying infrastructure (type-specific: podman start, kubectl scale, compose up).
2. Wait for the infrastructure to become ready (with existing progress output).
3. Update `InfraState` to "running".
4. MUST NOT create a Zellij session. Session creation is the responsibility of Attach().

### Stop(ctx) error

1. Call `KillSession(ctx)` to kill any existing Zellij session. Ignore errors (session may not exist).
2. Stop the underlying infrastructure (type-specific: podman stop, kubectl scale to 0, compose stop).
3. Update `InfraState` to "stopped".
4. Update `SessionState` to "none".

## Type-Specific Implementation Notes

### Local
- Not InfraManager. Only session lifecycle.
- Session detection: `zellij list-sessions -n` parsed locally.
- Session kill: `zellij kill-session cc-deck-<name>`.
- Session create: `zellij attach cc-deck-<name> --create-background --layout cc-deck`.

### Container
- InfraManager. Both infrastructure and session lifecycle.
- Session detection: `podman exec cc-deck-<name> zellij list-sessions -n`.
- Session kill: `podman exec cc-deck-<name> zellij kill-session cc-deck-<name>`.
- Infra start: `podman start cc-deck-<name>`.
- Infra stop: `podman stop cc-deck-<name>`.

### Compose
- InfraManager. Same session patterns as Container (via session container).
- Infra start: `podman-compose start` (or fallback).
- Infra stop: `podman-compose stop` (or fallback).

### SSH
- Not InfraManager. Only session lifecycle.
- Session detection: SSH exec `zellij list-sessions -n | grep cc-deck-<name>`.
- Session kill: SSH exec `zellij kill-session cc-deck-<name>`.

### K8s-Deploy
- InfraManager. Both infrastructure and session lifecycle.
- Session detection: `kubectl exec cc-deck-<name>-0 -- zellij list-sessions -n`.
- Session kill: `kubectl exec cc-deck-<name>-0 -- zellij kill-session cc-deck-<name>`.
- Infra start: Scale StatefulSet to 1, wait for pod ready.
- Infra stop: Scale StatefulSet to 0.
