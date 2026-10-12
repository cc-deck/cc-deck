# Workspace-Centric Session Sharing UX Design

**Date:** 2026-07-24
**Feature:** 084 Zellij session sharing
**Status:** Approved design

## Purpose

Replace the standalone `cc-deck share start|status|stop` UX with sharing integrated into workspace lifecycle management. A host should be able to create, start, or attach to a local workspace with sharing enabled, issue additional independently revocable invitations, inspect sharing through normal workspace status commands, and end public access without stopping the workspace.

The design also resolves a lifecycle mismatch discovered during live acceptance: Zellij web sharing is selected when a session is created and cannot be safely enabled afterward by targeting an existing session with `zellij --session NAME options --web-sharing on`.

## Product Model

Each workspace has exactly one canonical Zellij session and three orthogonal runtime dimensions:

| Dimension | States |
|---|---|
| Infrastructure | `running`, `stopped`, `error`, or not applicable |
| Canonical session | `running` or `absent` |
| Sharing | `private`, `shared`, or `degraded` |

`start` is an idempotent convergence command. Its postcondition is a **ready workspace**: infrastructure is running and the canonical Zellij session exists. It may perform no transition, start only infrastructure, create only the missing session, or do both.

Sharing is runtime intent, not a persistent workspace preference. If a shared session ends, a later plain start or attach creates a private session unless the host explicitly supplies `--share` again.

V1 supports sharing local workspaces only and permits at most one actively shared workspace per host. Multi-backend sharing remains deferred to brainstorm 084.

## Command UX

### Create

```text
cc-deck ws new NAME
```

Creates the workspace and converges it to ready in private mode. It creates the canonical Zellij session in the background but does not attach a terminal client.

```text
cc-deck ws new NAME --share
```

Creates the workspace, starts its canonical session with web sharing enabled, establishes the public endpoint, creates initial interactive and observer invitations, and prints both invitations once. It does not attach.

```text
cc-deck ws new NAME --no-start
```

Creates the workspace without converging it to ready. For local workspaces this leaves the canonical session absent; for infrastructure-backed workspaces it leaves infrastructure stopped when the backend supports a stopped post-creation state. `--no-start` and `--share` are mutually exclusive.

### Start

```text
cc-deck ws start NAME
```

Ensures that infrastructure and the canonical session are running privately. If infrastructure is already running but the session was killed, only the session is recreated. If both already exist, the command is a successful no-op.

```text
cc-deck ws start NAME --share
```

Ensures that a newly created canonical session is share-enabled, starts the endpoint, creates initial invitations for both roles, and prints them once.

If a private canonical session already exists, `start --share` fails without replacing it and prints explicit restart guidance:

```text
Workspace "demo" already has a private session.
Restart it for sharing:
  cc-deck ws kill-session demo
  cc-deck ws start demo --share
```

If the workspace is already shared, `start --share` is an idempotent success but does not redisplay secrets.

### Attach

```text
cc-deck ws attach NAME
cc-deck attach NAME
```

Attaches to the canonical session. If the workspace is not ready, attach first delegates to the same private ensure-ready operation used by `start`, announces the work performed, and then connects.

```text
cc-deck ws attach NAME --share
cc-deck attach NAME --share
```

If readiness work is required, attach delegates to shared start, prints the initial invitations, and then connects. If the workspace already has a shared session, it attaches without redisplaying secrets. If it already has a private session, it returns the same non-destructive restart guidance as `start --share`.

Attach is the only command that implicitly starts a stopped workspace. Commands such as `invite`, `revoke`, and `exec` do not silently start infrastructure or sessions.

### Create invitations

```text
cc-deck ws invite NAME --role interactive [--name LABEL]
cc-deck ws invite NAME --role observer [--name LABEL]
```

Creates an additional independently revocable invitation for an actively shared workspace. The invitation secret is printed once and never persisted by cc-deck. If the host omits `--name`, cc-deck generates a unique, memorable random label such as `brave-otter` or `quiet-comet`.

Invitation labels and roles are safe metadata and appear in workspace status. Multiple invitations may exist for each role.

### Revoke an invitation

```text
cc-deck ws revoke NAME INVITATION_LABEL
```

Revokes one named invitation without affecting other collaborators or observers. Revoking an already-revoked label is idempotent.

### End sharing

```text
cc-deck ws unshare NAME
```

Revokes all invitation credentials and closes the public endpoint while keeping infrastructure and the canonical session running. The session remains usable locally. A repeated unshare is an idempotent success.

`cc-deck ws stop NAME` also unshares, terminates the canonical session, and stops infrastructure. It attempts all teardown actions even if one action fails.

### Remove the standalone command family

The unmerged feature's following commands are removed rather than deprecated:

```text
cc-deck share start
cc-deck share status
cc-deck share stop
```

Their lifecycle behavior moves behind workspace commands and workspace status. No compatibility alias is required because the feature has not shipped.

## Status UX

`ws list` reconciles actual backend, Zellij, and sharing state before displaying separate `INFRA`, `SESSION`, and `SHARING` columns:

```text
NAME   TYPE     INFRA    SESSION   SHARING
demo   local    -        running   shared
api    compose  running  absent    unsupported
lab    local    -        absent    private
```

`ws status` includes safe sharing details:

- Lifecycle state
- Public endpoint
- Invitation labels and roles
- Guard health
- Named residual resources when degraded

Neither command displays raw token values.

## Lifecycle Architecture

### Central readiness operation

A workspace orchestration layer owns one operation conceptually equivalent to:

```text
EnsureReady(workspace, private | shared)
```

`ws new`, `ws start`, and `ws attach` use this operation rather than duplicating lifecycle decisions. It always reconciles actual state before choosing transitions.

For a shared start, the orchestration is:

1. Resolve the workspace and verify that its backend is local.
2. Reconcile actual canonical-session and sharing state.
3. Reject an existing private session or a different actively shared workspace.
4. Create the canonical Zellij session in the background with web sharing enabled at creation time.
5. Start or reuse the local Zellij web server.
6. Start the Cloudflare exposure provider and wait for readiness.
7. Create initial interactive and observer credentials with unique random labels.
8. Start the lifecycle guard.
9. Persist only safe lifecycle metadata.
10. Print invitations only after every required resource is ready.

Persisted metadata includes workspace identity, canonical-session identity, provider and endpoint handles, invitation labels and roles, guard identity, timestamps, and residual warnings. Raw invitation secrets are never persisted.

### Backend boundary

V1 accepts sharing operations only for local workspaces. For container, compose, SSH, Kubernetes, and OpenShell workspaces, `--share`, `invite`, `revoke`, and `unshare` return an actionable unsupported-backend error. The command shape remains suitable for adding backend-specific exposure later without changing user vocabulary.

### One shared workspace

Only one workspace may be shared on a host at a time. A request to share another workspace fails and names the currently shared workspace. It never silently unshares or switches the active workspace.

## Session Death and Recovery

Killing Zellij with `Ctrl+q` is a normal state transition, not an infrastructure failure.

For a private session:

```text
Infrastructure: running
Session:        absent
Sharing:        private
```

The next plain `start` or `attach` recreates a private canonical session.

For a shared session, the lifecycle guard watches canonical-session existence as well as provider health. Session death triggers complete sharing teardown: revoke every known credential, close the endpoint, and mark sharing private. A later plain start or attach remains private. The host must explicitly request `--share` again.

If teardown cannot be confirmed, sharing becomes degraded. New invitations and new sharing operations are prohibited until reconciliation succeeds. `ws status`, `ws unshare`, `ws stop`, and later lifecycle commands identify and retry every residual action.

## Transactionality and Error Handling

Shared startup is transactional. Invitations are not printed until the session, web server, provider endpoint, initial credentials, and lifecycle guard are all ready. A failure rolls back resources in reverse creation order.

Teardown is exhaustive rather than fail-fast. It attempts credential revocation, endpoint closure, web-server reconciliation, session shutdown, and infrastructure shutdown as applicable, collecting every residual failure.

Notable command behavior:

- `invite` requires an active, healthy shared workspace.
- `revoke` requires a known invitation label but is idempotent after successful revocation.
- `unshare` is idempotent when private.
- A running private session is never killed implicitly to satisfy `--share`.
- Unsupported backends fail before creating tokens, endpoints, or other sharing resources.
- Status and errors never expose raw secrets.

## Implementation Reuse

The existing feature-084 sharing package remains the foundation for:

- Exposure-provider abstraction and Cloudflare implementation
- Invitation construction and escaping
- Guard process and identity validation
- Restricted state persistence and locking
- Transactional rollback and exhaustive teardown

The standalone CLI adapter is replaced by workspace commands. The Zellij adapter is changed from attempting to mutate an existing named session to creating the canonical session with sharing selected at session creation.

Implementation occurs in the existing `084-zellij-session-sharing` worktree.

## Verification

### State and orchestration tests

- Cover every meaningful infrastructure/session/sharing combination.
- Verify `start` converges only the missing dimensions.
- Verify session death followed by plain start or attach recreates privately.
- Verify attach is the only implicit-start command.

### CLI tests

- Verify `new`, `start`, and `attach` delegation with and without `--share`.
- Verify `--share` conflicts with `--no-start`.
- Verify visible announcements when attach performs readiness work.
- Verify non-destructive errors for running private sessions.
- Verify unsupported-backend messages.
- Verify standalone `share` commands are absent.
- Verify list/status never expose secrets.

### Sharing lifecycle tests

- Verify multiple invitations per role and unique random fallback labels.
- Verify individual revocation and complete unshare.
- Verify one-shared-workspace enforcement.
- Verify shared-session death triggers guard teardown.
- Verify transactional rollback and degraded recovery at every failure point.

### Live acceptance

Revise feature task T032 around the workspace-centric flow and execute ten measured repetitions with:

- One local workspace
- Two interactive clients and two observers
- Initial and additional invitations
- Individual revocation
- Complete unshare
- Session death through `Ctrl+q`
- Private-by-default recreation
- Old-token rejection
- Start, attach, stop, and restart transitions

Repository-wide verification task T034 remains deferred under verification-hardening brainstorm 090.

## Deferred Scope

- Sharing non-local workspace backends
- Multiple concurrently shared workspaces
- Persistent sharing policy
- Redisplaying or persisting invitation secrets
- Synchronized observer focus or presenter-following behavior
- Terminal TLS hardening and guaranteed cleanup after uncatchable process loss
- Repository-wide test-tier repairs tracked by brainstorm 090
