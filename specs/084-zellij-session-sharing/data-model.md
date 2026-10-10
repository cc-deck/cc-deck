# Data Model: Workspace-Centric Zellij Session Sharing

## WorkspaceSharingState

The reconciled command-output state is `private`, `shared`, or `degraded`. Non-local workspaces render `unsupported` because sharing is local-only.

- `private`: no active sharing operation exists for the local workspace.
- `shared`: endpoint, canonical session, lifecycle guard, and persisted invitation metadata are healthy.
- `degraded`: cleanup or health reconciliation found residual exposure or could not confirm a safe state.

This is derived status attached to list/status output, not persisted in workspace definitions.

## SharingOperation

Fields: opaque operation ID, workspace name, canonical session name, provider name, endpoint URL, provider process metadata, lifecycle state, guard identity, timestamps, residual resources, and an invitation list.

Only one operation may exist per host. Lifecycle states remain `starting`, `active`, `stopping`, and `degraded`. Startup failure compensates toward absence; unresolved cleanup remains degraded.

## InvitationRecord

Each persisted invitation contains:

- `label`: unique memorable label within the operation
- `role`: `interactive` or `observer`
- `state`: `active` or `revoked`
- `created_at`: creation timestamp

There may be multiple invitations for either role. Revoked records remain as tombstones so repeated revocation is idempotent. No raw token is ever persisted.

## Invitation

Ephemeral output for one credential: label, role, browser URL, terminal command, and applicable warnings. The raw token exists only in this one-time output.

## ProviderStatus

Provider key, `starting|ready|stopped|failed|unknown`, safe diagnostic, public endpoint when ready, and process metadata.
