# Implementation Plan: Workspace-Centric Zellij Session Sharing

**Branch**: `084-zellij-session-sharing` | **Date**: 2026-07-24 | **Spec**: [spec.md](spec.md)

## Summary

Replace the unshipped standalone sharing CLI with local-workspace sharing integrated into workspace creation, readiness, attachment, invitation management, status, and teardown. A reusable readiness operation independently converges infrastructure and one canonical Zellij session. Existing provider, locking, escaping, state, guard, and teardown foundations remain in `internal/share`, generalized for workspace identity and multiple named invitations.

## Command Surface

```text
cc-deck ws new NAME [--share | --no-start]
cc-deck ws start NAME [--share]
cc-deck ws attach NAME [--share]
cc-deck ws invite NAME --role interactive|observer [--name LABEL]
cc-deck ws revoke NAME INVITATION_LABEL
cc-deck ws unshare NAME
cc-deck ws list
cc-deck ws status NAME
```

Sharing is local-only and only one workspace may be shared per host. `--share` creates a missing canonical session with `zellij --layout cc-deck attach -b NAME options --web-sharing on`; it never replaces a private running session. Raw invitation secrets print once and are never persisted. If the shared session dies, the guard tears sharing down and an ordinary restart is private.

## Architecture

### Workspace readiness

`ws.EnsureReady` reads workspace status once, starts non-running infrastructure when applicable, then creates the absent canonical session through `SessionManager`. Infrastructure and session are independent dimensions, so already-ready work is not repeated. Web sharing is passed only at session creation and is rejected for non-local backends before any backend command runs.

```go
type SessionStartOptions struct { WebSharing bool }
type SessionStartResult struct { Created bool; Name string }
type SessionManager interface {
    EnsureSession(context.Context, SessionStartOptions) (SessionStartResult, error)
}
type ReadyOptions struct { Share bool }
type ReadyResult struct {
    InfrastructureStarted bool
    SessionCreated bool
    SessionName string
}
```

Every backend extracts its existing non-interactive session-existence/background-creation work from `Attach` into `EnsureSession`. `Attach` then ensures readiness before performing terminal replacement or interactive transport.

### Sharing lifecycle

The sharing service accepts a workspace and canonical session identity. Start requires that session to exist before creating the Zellij web server, credentials, provider endpoint, or guard. Initial start creates one interactive and one observer invitation. Later invites create independent credentials under the lifecycle lock; revoke targets one label and persists a tombstone. Stop attempts provider shutdown, every active credential revocation, web-server shutdown, guard cleanup, and state cleanup even after partial failures.

```go
type StartRequest struct { Workspace, Session, Provider string }
type InviteRequest struct { Label string; Role InvitationRole }
type Service interface {
    Start(context.Context, StartRequest) ([]Invitation, error)
    Invite(context.Context, InviteRequest) (Invitation, error)
    Revoke(context.Context, string) (SharingStatus, error)
    Status(context.Context) (SharingStatus, error)
    Stop(context.Context) (SharingStatus, error)
}
```

The Zellij adapter validates capabilities, checks canonical-session health, creates/revokes tokens, and manages the web server. It does not attempt the invalid mutation of an existing session's web-sharing option. The detached hidden `ws share-guard` watches both provider and canonical-session health and invokes the normal once-locking stop path without recursive lock acquisition.

### Command orchestration

Workspace commands are thin adapters over readiness and sharing services. Shared startup runs readiness first and starts sharing second. If sharing startup fails after creating a new share-enabled session, the command kills only that newly created session. An existing private session is preserved and produces actionable restart instructions. Workspace stop attempts unshare, session shutdown, and infrastructure shutdown, aggregates residual errors, and does not abandon later cleanup actions.

List/status reconcile safe sharing metadata into command output. Local inactive workspaces are `private`, healthy active ones `shared`, and operations with residuals `degraded`; non-local workspaces are `unsupported`. Output contains endpoint and invitation labels/roles, never tokens.

## Data and Security Constraints

- Persist invitation `label`, `role`, `state`, and `created_at`; never persist raw tokens.
- Generate memorable adjective-noun labels with `crypto/rand`, collision retry, and bounded exhaustion.
- Print trusted-control and terminal certificate warnings before one-time secrets.
- Serialize lifecycle actions with the existing cross-process lock.
- Preserve 0700 state directories, 0600 atomic files, provider encryption, and explicit terminal TLS-risk disclosure.
- Never kill a private session merely because sharing was requested.

## Verification

Each implementation task follows red/green tests and a focused package suite. Final focused verification covers `internal/share`, `internal/ws`, `internal/cmd`, and `cmd/cc-deck`, followed by `make lint` and command help. Live T032 remains open until the ten-repetition four-client matrix passes. Repository-wide `make test`/`make verify` remain T034 and link brainstorm 090 until their unrelated failures are repaired.
