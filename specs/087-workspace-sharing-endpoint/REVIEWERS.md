# Review Guide: Workspace Sharing via External Endpoint

**Generated**: 2026-09-07 | **Spec**: [spec.md](spec.md)

## Why This Change

Workspace sharing was built around a Cloudflare quick tunnel that cc-deck launched, supervised, and
killed. A full manual exercise on 2026-09-06 established that the feature had never once worked end
to end for a human, despite a passing unit suite and a commit recording that verification had been
performed. Four separate defects surfaced. A detached guard process polled sharing status every 250
milliseconds and deadlocked the Zellij server, freezing every terminal on the machine. Every `zellij`
invocation was unbounded, so one wedged session server hung any command that touched sharing state.
Worse, a failed session probe was treated as proof the session had ended, so a single `cc-deck ws ls`
against an unresponsive server revoked live credentials and killed the tunnel while the session was
still running. And the guest simply could not connect, because cc-deck scraped a URL out of process
output and never once fetched it.

The first three were fixed on the parent branch. This change addresses the fourth and, more
importantly, removes the architecture that produced all of them.

## What Changes

cc-deck stops managing tunnels entirely. The user supplies an endpoint that is already serving, and
cc-deck verifies it through five layers before printing any invitation, records what it found, and
never tears anything down without positive evidence that the workspace session is gone.

**This is a breaking change to the configuration schema.** The `sharing.provider` key is removed
outright with no migration, which is safe only because the feature is unreleased. `sharing.endpoint`,
`sharing.endpoints`, `sharing.default`, and `sharing.verify_timeout` replace it. `cc-deck ws start
--share` is no longer self-sufficient: an endpoint must exist first. That cost was accepted
deliberately; zero-configuration sharing was considered and rejected as a requirement.

The user-visible gain is that sharing either works or tells you precisely which layer is broken. The
review-worthy gain is roughly 1,900 lines deleted against 900 added.

## How It Works

The `Provider` interface, the Cloudflare implementation, the `Process` abstraction, and every
PID-based lifecycle path are deleted. An `Endpoint` interface replaces them with `Name`, `Resolve`,
and `Probe`, and deliberately no `Start` or `Stop`. `StaticEndpoint` is the only implementation; the
interface is retained purely as a seam so a supervised endpoint later is an addition rather than a
restructuring.

The probe runs five stages fail-fast under one shared deadline: DNS, TLS, HTTP, Auth, WebSocket. Each
names itself, so a failure identifies the layer at fault. The DNS stage retries through an independent
public resolver, which is what distinguishes a locally filtered name from one that does not exist. The
Auth stage mints its own observer-role credential, uses it against the real login path, and revokes it
in a `defer` that runs on every exit path. The WebSocket stage performs the upgrade handshake by hand
over `net`/`crypto/tls` and asserts only `101`; no WebSocket library is added, because reading one
status code does not justify a dependency.

Three behavioural changes in `service.go` carry the real risk. `Start` loses its process launch and
the rollback arm that went with it. `Status` no longer routes an endpoint failure into
`reconcileLocked`, so only a confirmed-absent session can reach teardown. And `teardownLocked` gates
the web server stop on a newly persisted `WebServerOwned` flag rather than stopping it
unconditionally.

The state machine, the lifecycle lock, the step-wise persisted teardown, the invitation and label
logic, and the token lifecycle are all retained untouched. None of them caused the failures.

## When It Applies

**Applies when**:

- A developer wants to share a running workspace with someone else
- An endpoint is already serving in front of the machine, typically a long-lived named tunnel under
  `launchd` or `systemd`
- Any command that reports sharing state runs: `start --share`, `invite`, `status`, `unshare`, and
  the default listing

**Does not apply when**:

- cc-deck would need to manage a tunnel's lifecycle. Supervised endpoints are deferred until real
  usage demands them, and the `Endpoint` seam exists so that stays an addition.
- Tailnet-only sharing is wanted. That is a materially different security posture and is worth
  documenting only once properly tested.
- The guest's own network is the problem. cc-deck can verify only from its own host, and the
  documentation must say so plainly rather than implying the probe guarantees guest access.
- Invitations need to expire on a timer. None do, and no expiry field exists. This was decided during
  clarification rather than inherited.

## Key Decisions

1. **cc-deck does not manage tunnels.** Alternatives were keeping supervision and fixing the bugs, or
   keeping it only for quick tunnels. Chosen because the endpoint most users should run is a named
   tunnel designed to live as a long-running service, so starting and killing one per share is the
   wrong operational model even with flawless supervision code. The only mechanism that genuinely
   wants per-share lifecycle is the quick tunnel, which this design demotes to demonstration use.

2. **The `Endpoint` interface is kept despite having one implementation.** The alternative was
   collapsing it into plain configuration. Retained so a supervised endpoint later is an addition
   rather than an architectural reopening. Reviewers who dislike single-implementation interfaces
   should read this as a deliberate, stated trade.

3. **Verification is a blocking gate, not a warning.** `--no-verify` exists as the explicit override.
   A dedicated `ws share-check` command was proposed and rejected as namespace pollution, since
   verification is internal to commands that already need it.

4. **No new dependency for the WebSocket stage.** `github.com/coder/websocket` and
   `golang.org/x/net/websocket` were both considered. The latter is already available as an indirect
   dependency, but its coarse errors would defeat the stage attribution the probe exists for.

5. **A skipped verification does not create a third state.** Alternatives were an explicit
   `unverified` state or reporting `degraded`. Chosen so the state model stays at two values; the
   absence of a verification age is the signal.

6. **A failed share never removes the workspace.** Even when one command created it. Sharing is
   additive; a problem in the exposure layer is not a reason to discard work the user asked for.

7. **The probe mints a credential per call rather than reusing one.** A per-share credential would be
   cheaper but would have to be stored, breaking the invariant that
   `TestStartRevealsBothRolesOnlyAfterReadinessAndPersistsNoSecrets` currently guards.

## Areas Needing Attention

**Severing teardown from the endpoint path is the highest-risk change.** `Status` currently reaches
`reconcileLocked` whenever the provider is unhealthy, and that path tears down. After this change a
genuinely dead share stays alive until either the session is confirmed gone or the user runs
`unshare`. That is intentional and is the whole point of the evidence rule, but it is a real
behavioural reversal and deserves scrutiny. The mitigation is that session absence is still checked on
every status, and `unshare` always works regardless of endpoint health.

**Minting and revoking a token on every `ws status` call.** Two extra bounded `zellij` invocations per
status, inside the probe deadline. Reasonable people could prefer a cached credential. The security
argument won, but the cost is real and worth a second opinion.

**Sixty-five tasks may read as over-decomposed.** The five probe stages are separate tasks because
each is independently reviewable and testable. If that feels excessive, the natural merge is T016
through T020 into one.

**`--no-verify` producing no distinguishing state.** The listing shows `shared` with no age. That is
subtle, and a reviewer might reasonably argue an explicit marker is clearer. This was a deliberate
choice to keep the state model at two values.

**One assumption that could be wrong**: that proxies do not distinguish credential roles, which is why
an observer-role probe is treated as proving the interactive path works. If a proxy ever inspected
token scope, stage four would pass while real interactive use failed.

## Open Questions

No open questions identified. Both ambiguities raised during clarification were resolved by the
author, and the six findings from the plan review gate were fixed rather than deferred.

Two pre-existing defects are folded into this work rather than tracked separately: the missing
`ZELLIJ` environment guard in `EnsureSession`, which lets a workspace created from inside a Zellij
session silently append a tab and report success, and the unconditional web server teardown.

## Review Checklist

- [ ] Key decisions are justified
- [ ] Breaking changes are documented with migration guidance
- [ ] Scope matches the stated boundaries
- [ ] Success criteria are achievable
- [ ] No unstated assumptions
- [ ] The evidence rule holds: no code path tears down on an inconclusive probe
- [ ] No credential, including the probe's own, can reach disk or a diagnostic string
- [ ] The listing performs zero network access
- [ ] The five documentation artifacts required by constitution Principle I ship on this branch

---

<!-- Code phase sections are appended below this line by the phase-manager command -->
