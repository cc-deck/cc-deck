# Implementation Plan: Zellij Session Sharing

**Branch**: `084-zellij-session-sharing` | **Date**: 2026-07-24 | **Spec**: [spec.md](spec.md)

## Summary

Add `cc-deck share start|status|stop` for one local Zellij session. A new internal sharing package owns transactional startup and rollback, invitation construction, 0600 XDG runtime state, stale-state reconciliation, Zellij web/token operations, and a provider contract. V1 implements Cloudflare Quick Tunnel and validates the provider contract with deterministic fakes. Terminal TLS bypass and best-effort crash cleanup remain documented V1 risks tracked by brainstorm 089.

## Technical Context

**Language/Version**: Go version from `cc-deck/go.mod`

**Primary Dependencies**: Cobra, `internal/xdg`, external Zellij >=0.44.3, external `cloudflared`, Go standard process APIs

**Storage**: Atomic YAML under `internal/xdg.StateHome/cc-deck/`, directory 0700 and file 0600; raw tokens never persisted

**Testing**: Co-located Go tests with testify, fake command runners, shared provider contract suite, PATH-based CLI integration, manual real-tool acceptance

**Target Platform**: Existing cc-deck platforms with Zellij and cloudflared

**Project Type**: Go CLI and local process integration

**Performance Goals**: Invitations in 15 seconds when endpoint readiness is <=10 seconds; normal stop disconnects clients within 5 seconds

**Constraints**: One active operation; two shared role tokens; status suppresses secrets; transactional rollback; experimental terminal `--insecure`; crash cleanup best effort

**Scale/Scope**: One local session, at least two collaborators and two observers, one production provider plus test implementation

## Constitution Check

*GATE: Passed before and after design.*

- Tests cover service, state, rollback, escaping, provider contract, CLI, and manual acceptance.
- Documentation covers README, CLI reference, configuration reference, Antora guide/nav, and prose-profile validation.
- Verification uses `make test`, `make lint`, and `make verify`; direct build commands are prohibited.
- Runtime paths use `internal/xdg`. No workspace behavior contract changes and no container runtime is required.

## Global Constraints

- Zellij 0.44.3 is the minimum baseline, but authoritative capability probes MUST reject incompatible builds.
- Only one sharing operation may be active per host, with exactly one interactive and one observer credential.
- Raw token values MUST NOT be persisted or redisplayed after initial start output.
- The experimental terminal certificate bypass MUST be disclosed before its command is revealed.
- Provider traffic remains encrypted; V1 server-identity bypass is limited to the explicitly warned terminal client.
- Crash cleanup is best effort with no completion bound; later lifecycle commands MUST reconcile residuals.
- Verification MUST use `make test`, `make lint`, and `make verify`, never direct build commands.
- Delivery includes README, CLI reference, configuration reference, Antora guide/nav, and prose-profile validation.

## Interfaces

```go
type Process interface {
    PID() int
    Wait() error
    Signal(os.Signal) error
    Kill() error
}
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) (stdout []byte, err error)
    Start(ctx context.Context, name string, args ...string) (Process, error)
}
type ProviderHandle struct { PID int; Metadata map[string]string }
type ProviderStatus struct { State string; EndpointURL string; Diagnostic string }

type Provider interface {
    Name() string
    Validate(ctx context.Context) error
    Start(ctx context.Context, localURL string) (ProviderHandle, error)
    Ready(ctx context.Context, handle ProviderHandle) (ProviderStatus, error)
    Status(ctx context.Context, handle ProviderHandle) (ProviderStatus, error)
    Stop(ctx context.Context, handle ProviderHandle) error
}

type Zellij interface {
    ValidateCapabilities(ctx context.Context) error
    ResolveSession(ctx context.Context, requested string) (string, error)
    ShareSession(ctx context.Context, session string) error
    UnshareSession(ctx context.Context, session string) error
    CreateToken(ctx context.Context, label string, readOnly bool) (string, error)
    RevokeToken(ctx context.Context, label string) error
    EnsureWebServer(ctx context.Context) (localURL string, started bool, err error)
    StopWebServer(ctx context.Context) error
}

type Store interface {
    WithLock(ctx context.Context, fn func() error) error
    Load() (*SharingOperation, error)
    Save(*SharingOperation) error
    Remove() error
}

type Service interface {
    Start(ctx context.Context, req StartRequest) (InvitationSet, error)
    Status(ctx context.Context) (SharingStatus, error)
    Stop(ctx context.Context) (SharingStatus, error)
}
type Guard interface {
    Start(ctx context.Context, operationID string) (GuardHandle, error)
    Disarm(ctx context.Context, GuardHandle) error
}
type GuardHandle struct { PID int; OperationID string; Ready bool }
```

`SharingOperation` contains the fields in data-model.md; `StartRequest` contains session/provider; `InvitationSet` contains four one-time strings and warnings; `SharingStatus` contains lifecycle state, safe endpoint/session/provider fields, and residuals without tokens. Errors retain residuals and never token values. Stop operations are idempotent.

After active state is atomically persisted, `share start` launches a detached hidden `cc-deck share guard --operation <id>`. The guard acquires the operation lock only long enough to reload state and verify that its operation identity is still current, then releases the lock before signaling readiness or watching provider exit and supported termination signals. On either event it invokes the normal `Service.Stop`, which reacquires the lifecycle lock exactly once. Normal `share stop` acquires the lock, marks the operation stopping, signals/disarms the guard, and performs teardown without waiting while holding a lock needed by the guard. The guard never calls a lock-taking service while already holding the lock. Uncatchable guard death cannot run cleanup and is reconciled by the next lifecycle command, matching the accepted V1 limitation.

## Project Structure

```text
specs/084-zellij-session-sharing/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/{cli,provider}.md
└── tasks.md

cc-deck/
├── cmd/cc-deck/main.go                 # register share command
├── internal/cmd/share.go               # Cobra input/output only
├── internal/cmd/share_test.go          # CLI and secret-output contract
├── internal/share/model.go             # domain types and state transitions
├── internal/share/service.go           # start/status/stop saga and reconciliation
├── internal/share/state.go             # atomic 0600 store
├── internal/share/lock.go              # cross-process serialization
├── internal/share/guard.go             # best-effort controller-loss teardown attempt
├── internal/share/invitation.go        # URL and shell-safe invitations
├── internal/share/zellij.go            # capability and web/token/session adapter
├── internal/share/provider.go           # provider and runner interfaces/registry
├── internal/share/cloudflare.go         # Cloudflare Quick Tunnel provider
├── internal/share/*_test.go             # unit and contract suites
├── internal/config/config.go            # sharing configuration schema/defaults
├── internal/config/validate.go          # sharing validation
└── internal/config/validate_test.go     # validation cases

README.md
docs/modules/reference/pages/{cli,configuration}.adoc
docs/modules/using/pages/sharing.adoc
docs/modules/using/nav.adoc
```

**Structure Decision**: Keep Cobra wiring thin and isolate lifecycle behavior in `internal/share`. All external commands use injected interfaces so failure and rollback behavior is deterministic in tests.

## Design Decisions

1. Startup validates prerequisites, reconciles stale state, enables only the selected session, creates both tokens, starts the provider, awaits readiness, persists state, then reveals invitations. Failure compensates in reverse order.
2. Status probes persisted non-secret metadata and health; token values are never stored or redisplayed.
3. A cross-process lifecycle lock serializes separate Cobra invocations. Stop always attempts provider stop, both revocations, and session unsharing, collecting residual failures into degraded state.
4. Browser URLs use URL-path escaping; terminal commands use shell-safe quoting and label `--insecure` experimental.
5. Cloudflare endpoint discovery is timeout-bound and malformed output causes rollback.

## Complexity Tracking

No constitution violations. The provider abstraction is an explicit V1 requirement and has a shared contract suite.
