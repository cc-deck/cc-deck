# Workspace Sharing: Endpoint Redesign

Date: 2026-09-06
Status: Approved, pending implementation plan
Supersedes: the provider portions of `2026-07-24-workspace-sharing-ux-design.md`

## Context

Workspace sharing was built around a Cloudflare quick tunnel that cc-deck launches, supervises, and kills. The feature is unreleased. A full manual exercise on 2026-09-06 established that it had never worked end to end for a human, and produced four distinct failures.

1. A detached guard process polled `service.Status()` every 250 milliseconds. Each poll took the lifecycle lock and ran `zellij` commands, which deadlocked the Zellij server and froze every terminal on the machine. The guard was removed in `a165b2c`.
2. Every `zellij` invocation was unbounded. A wedged session server accepts connections and never answers, so one bad session hung any cc-deck command that inspected sharing state. Bounded in `14d44e7`.
3. `Status()` treated a failed session probe as proof the session had ended, so a single `cc-deck ws ls` against an unresponsive server revoked live credentials, killed the tunnel, and deleted the share state while the session was still running. Fixed in `14d44e7`.
4. The guest could not connect. The endpoint hostname was blocked by a DNS filter on the host network, and cc-deck had no way to detect this because readiness only scraped the tunnel process output for a URL and never fetched that URL.

A controlled experiment cleared the components that were initially suspected. Background session creation works, the cc-deck layout works, and the Zellij web client works. The failure was in the exposure layer and in how cc-deck reasoned about it.

Two further observations shaped this design. A Cloudflare quick tunnel degraded into serving HTTP 530 within about forty minutes while `cloudflared` remained alive and reported healthy, which means process liveness is not a valid health signal. And `*.trycloudflare.com` is blocked by common DNS filter lists, confirmed locally with AdGuard Home returning NXDOMAIN for every subdomain including invented ones, which means a randomly generated quick tunnel hostname may be unreachable for the host, the guest, or both.

## Decisions

### cc-deck does not manage tunnels

cc-deck never starts, supervises, or stops a tunnel. The user provides an endpoint that is already serving, and cc-deck verifies it, issues invitations against it, and monitors it.

This removes the entire class of failure that produced items 1 through 3 above: no polling supervisor, no process identity validation, no killing by PID, no rollback of a spawned process, no residual class for a tunnel that may still be running.

The decision is also correct operationally rather than merely convenient. The endpoint most users should run is a named Cloudflare tunnel, and a named tunnel is designed to run as a long lived service under launchd or systemd. Starting and killing one per share is the wrong model for that tool even if the supervision code were flawless. The only exposure mechanism that genuinely wants per share lifecycle is the quick tunnel, which is the one this design demotes to demonstration use.

The cost is accepted deliberately: `cc-deck ws start --share` is no longer self sufficient, and an endpoint must exist first. Zero configuration was explicitly not a requirement.

### The Endpoint interface is kept as a seam

Although v1 has exactly one implementation, the interface is retained so that adding a supervised endpoint later is an addition rather than an architectural reopening.

```go
// Endpoint is a public URL plus proof that it serves the session.
type Endpoint interface {
    Name() string
    Resolve(ctx context.Context) (EndpointRef, error)
    Probe(ctx context.Context, ref EndpointRef) (ProbeResult, error)
}
```

`Resolve` reads configuration and returns an `EndpointRef` carrying the base URL. `Probe` verifies the contract described below. There is no `Start`, no `Stop`, and no process handle anywhere in the sharing package.

### Exposure mechanisms are documentation, not code

`cloudflare-named`, `cloudflare-quick`, Tailscale, and SSH reverse tunnels are no longer provider implementations. They are recipes in the user guide for producing a URL that satisfies the endpoint contract. This keeps cc-deck out of the business of tracking the command line flags and output formats of five external tools.

## The endpoint contract

An endpoint is a reverse proxy in front of the Zellij web server, by default `http://127.0.0.1:8082`. The contract was derived by reading the Zellij 0.45.1 web client rather than from documentation.

| Path | Kind | Purpose |
|------|------|---------|
| `/` and `/assets/*` | static | the web client itself |
| `POST /command/login` | JSON | exchanges an invitation token for a `session_token` cookie |
| `GET /info/version` | JSON | client handshake |
| `/ws/control` | WebSocket | control channel |
| `/ws/terminal/<session>` | WebSocket | terminal data |

Requirements that proxies commonly violate:

- **WebSocket upgrades must pass through on both socket paths.** A proxy that forwards HTTP but drops `Upgrade` and `Connection` headers produces a page that loads and a terminal that never fills, which is the blank screen observed during testing.
- **Cookies must survive in both directions.** Login sets `session_token=...; HttpOnly; SameSite=Strict; Path=/`. Because the cookie is `SameSite=Strict`, the origin the guest browses must be the origin that issued it.
- **Paths must not be rewritten.** The session is selected by URL path.
- **Responses must not be buffered, and the idle timeout must be long.** Terminal output is a stream and the sockets are long lived. A default sixty second idle timeout terminates sessions during use.
- **TLS should terminate at the endpoint** for any endpoint reachable outside a trusted network.

## Verification

Verification is internal to the commands that need it. There is no dedicated check command, because that would add a third way to do what two commands already must do.

The probe runs five stages in order and stops at the first failure. Each stage names itself so the resulting error identifies the layer at fault.

| Stage | Check | Failure it catches |
|-------|-------|--------------------|
| 1. DNS | resolve the host; on failure, retry through a public resolver | a local DNS filter, reported by name when the public resolver succeeds where the configured one does not |
| 2. TLS | connect and validate the certificate | expired or mismatched certificates |
| 3. HTTP | `GET /` and confirm the Zellij web client is served | a proxy pointed at the wrong origin, or a degraded tunnel returning 5xx |
| 4. Auth | `POST /command/login`, expect a `session_token` cookie | proxies that drop POST bodies or strip `Set-Cookie` |
| 5. WebSocket | upgrade `/ws/control`, expect 101, then close | proxies that serve pages but drop upgrades |

The probe authenticates with the real invitation token so that it exercises the actual credential path. It opens `/ws/control` rather than a terminal socket so that it never disturbs the session. The whole probe is bounded by a single timeout, defaulting to fifteen seconds, so verification cannot become a new way to hang a command.

Stages 4 and 5 have no equivalent in the current implementation and are the two that would have caught the reported failure.

### Where verification runs

- `ws start --share` and `ws invite` run the full probe as a gate. Failure means no invitation is printed and the operation is rolled back. `--no-verify` overrides.
- `ws status NAME` re-runs the probe. Its result determines whether sharing reports `shared` or `degraded`.
- `ws list` never probes. A probe costs roughly a second of network work, and sharing status is already resolved once per listing rather than once per row. The listing reports the stored probe result with its age, for example `shared (verified 12m ago)`, which is honest about staleness rather than implying live knowledge.

Nothing polls in the background. A share that fails while idle is detected at the next `status` or `start`, which is the reconcile on demand model already in place after the guard was removed.

## Lifecycle and teardown

The governing rule is that cc-deck stops only what cc-deck started, and ownership is recorded in state when the operation begins rather than inferred during cleanup.

| Resource | Behavior on unshare |
|----------|---------------------|
| Invitation tokens | revoked; always created by cc-deck |
| The user endpoint | never touched |
| Zellij web server | stopped only when cc-deck started it |
| Zellij session | never touched; `unshare` is not `kill-session` |

The web server row is a fix rather than a restatement. `Start` correctly registers a rollback only when it actually started the server, but `teardownLocked` currently stops it unconditionally, so unsharing kills a web server the user was already running. A persisted `WebServerOwned` flag gates the step.

### Definite versus inconclusive evidence

Cleanup requires positive evidence that a resource is gone. This generalizes the fix committed in `14d44e7`.

- **Definite, triggers teardown.** The canonical session is confirmed absent. Credentials are revoked, preserving the original security requirement that session death ends all access.
- **Inconclusive, reports degraded and changes nothing.** Probe timeout, DNS failure, an unresponsive Zellij, or any result that fails to establish absence. Resources are left intact and the degraded state is not persisted, because a persisted degraded operation would be torn down by the next healthy call.

A failing probe never triggers teardown. An endpoint restarting is not consent to revoke credentials, and credentials are unusable while the endpoint is down.

Crash safety is unchanged. Teardown persists progress after each step, so a command killed during cleanup resumes correctly rather than orphaning resources.

## Configuration and CLI

```yaml
sharing:
  endpoint: https://dev.example.com     # single endpoint
  endpoints:                            # or named profiles
    work: https://dev.example.com
    home: https://home.example.net
  default: work
  verify_timeout: 15s
```

```bash
cc-deck ws start web --share                              # uses configured endpoint
cc-deck ws start web --share --endpoint https://x.example # override
cc-deck ws start web --share --no-verify                  # skip the gate
cc-deck ws invite web --role observer
cc-deck ws status web
cc-deck ws unshare web
```

Because the feature is unreleased, the configuration schema is redefined rather than migrated. The `sharing.provider` key is removed outright.

## Changes to existing code

| Action | Target |
|--------|--------|
| Delete | `cloudflare.go`, `cloudflare_test.go`, the `Process` interface, `CommandRunner.Start` |
| Add | `endpoint.go` (static endpoint), `probe.go` (five stage verifier) |
| Rewrite | `provider_contract_test.go` against `Endpoint` |
| Simplify | `service.go`: `Start` loses process launch and its rollback arm; `teardownLocked` loses the provider stop step |
| Extend | `model.go`: `ProviderHandle` becomes `EndpointRef`; add `WebServerOwned` and `LastProbe` |
| Wire up | `ProviderRegistry`, and delete the hardcoded provider rejection in `ws_share.go` |

The state machine, the lifecycle lock, the step wise persisted teardown, the invitation and label logic, and the token lifecycle are all retained. They were not the source of the failures.

## Testing

The probe is the new critical path. It is tested against a fake HTTP server that can fail at each stage independently, including a handler that serves the page correctly and refuses the WebSocket upgrade, which reproduces the blank terminal deterministically and offline.

One end to end test is added, and it is the most important item in this section. The lesson of 2026-09-06 is that a complete unit suite passed, and a commit recorded that verification had been performed, while the feature had never worked for a human. The test starts a real `zellij web`, creates a real background session, points a static endpoint at the local web server, and runs the real probe through all five stages. It skips when Zellij is absent, following the pattern of the existing compose smoke tests, and runs in CI where Zellij is installed. Every failure from this session would have been caught by it.

## Out of scope

Deferred to a later iteration, unblocked by this design:

- Supervised endpoints, should real usage demand that cc-deck manage a tunnel. Adding a `Supervisor` interface alongside `Endpoint` is an addition, not a restructuring.
- Tailscale Funnel and Serve recipes. Serve is the only mechanism offering tailnet only sharing rather than public exposure, which is a materially different security posture and worth documenting when it is properly tested.
- Endpoint reachability from the guest network. cc-deck can verify only from its own host. A guest behind a different DNS filter remains undetectable, and the documentation must say so plainly rather than implying the probe guarantees guest access.
