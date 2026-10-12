# Contract: Endpoint

**Feature**: 087-workspace-sharing-endpoint

Two contracts are specified here. The first is what a user's reverse proxy must satisfy. The second
is the behavioural contract every `Endpoint` implementation must satisfy, in the sense of
constitution Principle II.

## Part 1: What a user's endpoint must satisfy

An endpoint is a reverse proxy in front of the Zellij web server, by default `http://127.0.0.1:8082`.
Derived by reading the Zellij 0.45.1 web client, not from documentation.

| Path | Kind | Purpose |
|------|------|---------|
| `/` and `/assets/*` | static | the web client itself |
| `POST /command/login` | JSON | exchanges a token for a `session_token` cookie |
| `GET /info/version` | JSON | client handshake |
| `/ws/control` | WebSocket | control channel |
| `/ws/terminal/<session>` | WebSocket | terminal data |

Requirements proxies commonly violate:

- **WebSocket upgrades must pass on both socket paths.** Forwarding HTTP while dropping `Upgrade`
  and `Connection` produces a page that loads and a terminal that never fills. This is the reported
  defect.
- **Cookies must survive in both directions.** Login sets
  `session_token=...; HttpOnly; SameSite=Strict; Path=/`. Because it is `SameSite=Strict`, the origin
  the guest browses must be the origin that issued it.
- **Paths must not be rewritten.** The session is selected by URL path.
- **Responses must not be buffered and the idle timeout must be long.** A sixty second default idle
  timeout terminates terminals during use.
- **TLS should terminate at the endpoint** for anything reachable outside a trusted network.

## Part 2: Behavioural contract for `Endpoint` implementations

```go
type Endpoint interface {
    Name() string
    Resolve(ctx context.Context) (EndpointRef, error)
    Probe(ctx context.Context, ref EndpointRef, session string) (ProbeResult, error)
}
```

### C-1: No lifecycle

An implementation MUST NOT start, stop, signal, restart, or supervise any process, and MUST NOT hold
a process handle or PID. Verified by `provider_contract_test.go` rewritten against `Endpoint`.
(FR-001)

### C-2: `Resolve` is pure with respect to the network

`Resolve` reads configuration and returns a reference. It MUST NOT perform network access, so that
callers who only need an address never pay for a probe. (FR-017)

### C-3: `Probe` runs five stages in order and stops at the first failure

Order: DNS, TLS, HTTP, Auth, WebSocket. On failure the returned `ProbeResult` MUST carry `OK=false`
and the `FailedAt` stage that failed. On success it MUST carry `OK=true` and an empty `FailedAt`.
`CheckedAt` MUST be set in both cases. (FR-007, FR-008)

### C-4: `Probe` respects a single deadline

The entire probe MUST honour the caller's context deadline. It MUST NOT create per stage deadlines
that can sum beyond it. It MUST return within that deadline even when the endpoint accepts
connections and never answers. (FR-012, FR-032, SC-004)

### C-5: `Probe` distinguishes filtered from missing

When the name fails to resolve through the system resolver but resolves through an independent public
resolver, the diagnostic MUST identify local filtering rather than a missing address. Failure of the
second opinion MUST NOT convert a successful primary resolution into an error. (FR-009)

### C-6: `Probe` mints and revokes its own credential

`Probe` MUST authenticate with a credential it minted for that probe, scoped to the least privileged
role. It MUST revoke that credential on every exit path, including failure, timeout, and panic. It
MUST NOT use, consume, or invalidate a credential issued to a person, and MUST NOT persist its own.
(FR-010, FR-044, FR-045, FR-046)

### C-7: `Probe` does not disturb the session

`Probe` MUST open `/ws/control` and MUST NOT open a terminal socket, attach, or write to the session.
(FR-011)

### C-8: `Probe` never mutates sharing state

`Probe` MUST NOT revoke a person's credential, stop a process, delete state, or trigger teardown,
whatever its outcome. Recording the result is the caller's job and is an observation only.
(FR-021, FR-022, FR-047)

### C-9: Diagnostics carry no secrets

`ProbeResult.Diagnostic` MUST NOT contain a token, cookie value, or `Authorization` header.

## Test matrix

Every row is a distinct `httptest.Server` behaviour and is offline and deterministic.

| Case | Server behaviour | Expected `FailedAt` |
|------|------------------|---------------------|
| Healthy | full contract | none, `OK=true` |
| Name missing | address does not resolve anywhere | `dns` |
| Name filtered | system resolver fails, public resolver answers | `dns`, diagnostic names filtering |
| Bad certificate | TLS handshake fails validation | `tls` |
| Wrong origin | serves 200 but not the Zellij client | `http` |
| Degraded tunnel | returns 530 while reachable | `http` |
| Login body dropped | `POST /command/login` returns 400 | `auth` |
| `Set-Cookie` stripped | login returns 200 with no cookie | `auth` |
| **Upgrade refused** | serves pages correctly, refuses to hijack | `websocket` |
| Silent endpoint | accepts connections, never answers | deadline, bounded |

The bold row is the reported failure and the one `SC-002` requires to fail the build.
