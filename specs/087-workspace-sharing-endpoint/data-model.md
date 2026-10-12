# Phase 1 Data Model: Workspace Sharing via External Endpoint

**Feature**: 087-workspace-sharing-endpoint
**Date**: 2026-09-07

Current shapes live in `cc-deck/internal/share/model.go` and `cc-deck/internal/share/provider.go`.
Deltas are marked ADD, CHANGE, or REMOVE.

## SharingOperation

Persisted to `~/.local/state/cc-deck/share.yaml` at mode 0600 by `FileStore`.

| Field | Change | Notes |
|-------|--------|-------|
| `ID`, `Workspace`, `Session` | unchanged | |
| `Provider string` | REMOVE | No providers exist. The endpoint name replaces it. |
| `EndpointName string` | ADD | Which configured endpoint this share used, empty when supplied by flag. |
| `EndpointURL string` | unchanged | Now the user's address, recorded at share time. Status probes this recorded value, not the current configuration, so editing configuration never silently retargets a live share. |
| `ProviderHandle ProviderHandle` | REMOVE | Carried a PID. No process is owned. |
| `EndpointStopped bool` | REMOVE | cc-deck never stops the endpoint (FR-001, FR-026). |
| `WebServerOwned bool` | ADD | True only when `EnsureWebServer` reported that it started the server. Gates the teardown step (FR-027, R6). |
| `WebServerStopped bool` | unchanged | Teardown progress marker for crash safety. |
| `LastProbe *ProbeResult` | ADD | Nil when never verified, which is how a `--no-verify` share reports no age (FR-014, FR-018). |
| `Invitations []InvitationRecord` | unchanged | |
| `State LifecycleState` | unchanged | |
| `CreatedAt`, `UpdatedAt`, `Residuals` | unchanged | |

**Invariant**: `LastProbe` is an observation, never a state transition. Writing it must not move
`State` and must not trigger teardown (FR-047, FR-023).

## ProbeResult (ADD)

```go
type ProbeStage string

const (
    StageDNS       ProbeStage = "dns"
    StageTLS       ProbeStage = "tls"
    StageHTTP      ProbeStage = "http"
    StageAuth      ProbeStage = "auth"
    StageWebSocket ProbeStage = "websocket"
)

type ProbeResult struct {
    OK         bool       `yaml:"ok"`
    FailedAt   ProbeStage `yaml:"failed_at,omitempty"`
    Diagnostic string     `yaml:"diagnostic,omitempty"`
    CheckedAt  time.Time  `yaml:"checked_at"`
}
```

Stage order is significant and fail-fast: DNS, TLS, HTTP, Auth, WebSocket. `FailedAt` is empty when
`OK` is true. `Diagnostic` must never contain a credential.

**Validation**: `CheckedAt` is always set on a completed probe. A probe that did not run leaves
`LastProbe` nil rather than writing a zero value, because a zero timestamp would render as an age.

## LifecycleState

Unchanged values, but `StateDegraded` narrows in meaning. It remains a legal persisted state only as
a teardown marker recording that cleanup left residuals. It is no longer written because an endpoint
probe failed. A failing probe reports degraded in `SharingStatus` while `SharingOperation.State`
stays `StateActive` (FR-023, R5).

Transitions in `SharingOperation.Transition` are unchanged.

## InvitationRecord

Unchanged. No expiry field is added; invitations do not expire on a timer (FR-043). `Role` keeps its
existing `interactive` and `observer` values.

## SharingStatus

| Field | Change | Notes |
|-------|--------|-------|
| `Provider string` | REMOVE | |
| `EndpointName string` | ADD | |
| `LastProbe *ProbeResult` | ADD | What listings render, including the failing layer (FR-018). |
| everything else | unchanged | |

## Endpoint (ADD, replaces Provider)

```go
type EndpointRef struct {
    Name    string
    BaseURL string
}

type Endpoint interface {
    Name() string
    Resolve(ctx context.Context) (EndpointRef, error)
    Probe(ctx context.Context, ref EndpointRef, session string) (ProbeResult, error)
}
```

No `Start`, no `Stop`, no process handle. `StaticEndpoint` is the only implementation in this release.
The interface is retained as a seam so a supervised endpoint is a later addition rather than a
restructuring.

## Removed types

`Process`, `CommandRunner.Start`, `ProviderHandle`, `ProviderStatus`, `Provider`, `ProviderRegistry`,
and `CloudflareProvider`. `CommandRunner` keeps only `Run`.

## Configuration

```go
type SharingConfig struct {
    Provider      string            // REMOVE
    Endpoint      string            `yaml:"endpoint,omitempty"`
    Endpoints     map[string]string `yaml:"endpoints,omitempty"`
    Default       string            `yaml:"default,omitempty"`
    VerifyTimeout time.Duration     `yaml:"verify_timeout,omitempty"`
}
```

**Resolution precedence** (FR-003, FR-004): explicit `--endpoint` flag, then `--endpoint-name`, then
`Default` looked up in `Endpoints`, then the single `Endpoint`. If none resolves, refuse with
guidance listing configured names (FR-005).

**Validation**: each address parses as an absolute URL with an `http` or `https` scheme; `Default`,
when set, names a key in `Endpoints`; `VerifyTimeout` is positive, defaulting to fifteen seconds.
