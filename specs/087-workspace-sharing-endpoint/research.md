# Phase 0 Research: Workspace Sharing via External Endpoint

**Feature**: 087-workspace-sharing-endpoint
**Date**: 2026-09-07

All Technical Context unknowns are resolved below. No `NEEDS CLARIFICATION` markers remain.

## R1: WebSocket upgrade check without a new dependency

**Decision**: Perform the upgrade handshake by hand over `net.Dial` or `tls.Dial`, write the HTTP
upgrade request, read the status line, assert `101 Switching Protocols`, then close. No WebSocket
library is added.

**Rationale**: `FR-007` stage five needs exactly one fact: does the endpoint pass an upgrade through.
It never sends or receives a frame, so the entire value of a WebSocket library, framing, ping/pong,
close handshakes, compression, is unused. The handshake itself is a request with four headers and a
status line check, roughly forty lines including the `Sec-WebSocket-Accept` verification. Adding a
dependency to read one status code is not a trade worth making, and the module currently has no
WebSocket dependency at all.

**Alternatives considered**:

- `github.com/coder/websocket` (formerly nhooyr). Well maintained and the modern default, but a new
  direct dependency for a single status code assertion. Rejected on cost, not on quality.
- `golang.org/x/net/websocket`. Already present as an indirect dependency, so promoting it to direct
  is cheap, but it is the legacy package the Go team steers people away from, it lacks context
  support on the dial path, and its error reporting is coarse. Stage five must attribute failures
  precisely, which is the whole point of the staged probe, so coarse errors defeat the requirement.
- Reusing `net/http` with `Upgrade` headers. `http.Client` transparently rejects `101` responses in
  a way that loses the underlying connection, so the assertion cannot be made cleanly.

**Consequence for tasks**: the probe's stage five works against a raw connection, which means the
test fake must be an `httptest.Server` with a handler that hijacks the connection. A handler that
serves pages correctly and refuses to hijack is exactly the blank-terminal reproduction the spec
requires, and it is deterministic and offline.

## R2: Distinguishing a filtered address from a missing one

**Decision**: On resolution failure, retry the lookup against a public resolver using a `net.Resolver`
with a custom `Dial` that targets a well known DNS server on port 53. If the public resolver answers
and the system resolver did not, report local filtering by name. If both fail, report the address as
not existing.

**Rationale**: This is the failure that cost a full day of diagnosis on 2026-09-06. AdGuard Home
returned `NXDOMAIN` for every `*.trycloudflare.com` name, including invented ones, while two other
resolvers on the same network answered correctly. Without the second opinion, a filtered name and a
typo are indistinguishable, and the user is sent hunting in the wrong direction.

**Alternatives considered**:

- Reading `/etc/resolv.conf` and comparing configured resolvers. Platform specific, does not work on
  macOS where resolution goes through `libresolv` and the system configuration database, and it
  detects configuration rather than behaviour.
- Reporting only that resolution failed. This is the current behaviour and it is precisely what
  failed the user.
- DNS over HTTPS to avoid port 53 being blocked. Better in hostile networks but adds an HTTP
  dependency to the DNS stage and confuses layer attribution when the HTTP path is itself broken.
  Deferred; the port 53 attempt is enough to distinguish the two cases, and if it too is blocked the
  probe reports resolution failure, which is honest.

**Consequence for tasks**: the second opinion is best effort. Its own failure must never turn a
successful primary resolution into an error, and it must sit inside the overall probe deadline.

## R3: Where the probe's fifteen second budget is spent

**Decision**: One `context.WithTimeout` created at the top of the probe and passed to all five
stages, rather than a per stage budget. Stages inherit and consume from the single deadline.

**Rationale**: `FR-012` and `SC-004` require the whole probe to be bounded, not each stage. A per
stage budget multiplies to five times the intended ceiling in the worst case, which is how a bound
becomes a hang. The existing `ZellijCLI.run` already models the correct pattern at
`internal/share/zellij.go:34`: impose a default bound only when the caller has not set a shorter
deadline. The probe follows the same convention.

**Alternatives considered**: per stage timeouts summing to fifteen seconds. Gives better attribution
for a slow stage, but the stage name already provides attribution, and the arithmetic becomes a
maintenance burden every time a stage is added.

## R4: Preserving the no-secrets-persisted invariant for the probe credential

**Decision**: The probe mints its credential with `Zellij.CreateToken(ctx, label, readOnly=true)`,
holds the secret in memory only, and revokes it through a deferred call that runs on every exit path
including panic and deadline expiry. Nothing about the probe credential enters `SharingOperation`.

**Rationale**: `FR-045` and `FR-046` come directly from the clarification session, and the existing
`TestStartRevealsBothRolesOnlyAfterReadinessAndPersistsNoSecrets` already guards the broader
invariant that no secret reaches disk. A `defer` is the only construct that survives the early
returns a five stage fail-fast probe necessarily has.

**Risk noted**: this mints and revokes a token on every `ws status` call. Two consequences to verify
during implementation. First, `zellij web --create-token` rejects being combined with `--token-name`,
which is why `ZellijCLI.CreateToken` ignores its label argument entirely
(`internal/share/zellij.go:122`); the probe must therefore revoke by the name Zellij returns, not by
a name it chose. Second, the mint and revoke pair are two bounded `zellij` calls inside the probe
deadline, so the deadline must accommodate them alongside the five network stages.

**Alternatives considered**: a reusable per share probe credential. Cheaper per probe but requires
persisting a secret, which the user explicitly rejected during clarification.

## R5: Removing teardown from the endpoint failure path

**Decision**: `Status` calls `reconcileLocked` only when the canonical session is confirmed absent.
The endpoint probe result never reaches `reconcileLocked`. A failing probe updates the stored
verification result and returns a degraded status without any cleanup.

**Rationale**: This is the largest behavioural change in the feature and the source of the worst
observed defect. Today `Status` at `internal/share/service.go:331` falls through to
`reconcileLocked` whenever the provider is not healthy, and `reconcileLocked` calls
`teardownLocked`, which revokes credentials and stops the web server. So a single `cc-deck ws ls`
against an unhealthy endpoint destroys a live share. `FR-021` and `FR-022` forbid this outright.
`reportUnverified` at `internal/share/service.go:382` already implements the correct behaviour for
the unresponsive-Zellij case and its doc comment already states the reasoning; the fix generalizes
that path rather than inventing one.

**Alternatives considered**: keeping teardown on repeated failures, for example after N consecutive
bad probes. Rejected: it reintroduces the same destruction with a delay, requires persisting a
failure counter, and an endpoint being down is still not evidence that the session ended.

## R6: Web server ownership

**Decision**: Persist a `WebServerOwned` boolean on `SharingOperation`, set from the `started` return
value that `Zellij.EnsureWebServer` already produces, and gate the teardown step on it.

**Rationale**: The ownership signal already exists and is already used correctly for rollback at
`internal/share/service.go:127`, but it is discarded rather than persisted, so `teardownLocked` at
`internal/share/service.go:453` stops the web server unconditionally. A user who was already running
`zellij web` loses it on unshare. The fix is to persist a flag that is already computed, not to infer
ownership later, which matches the spec's rule that ownership is recorded when the operation begins.

**Alternatives considered**: probing whether the web server was pre-existing at teardown time.
Rejected as inference after the fact, which is the pattern that produced the PID-based defects this
redesign removes.

## R7: Configuration schema replacement

**Decision**: Replace `SharingConfig.Provider` with `Endpoint`, `Endpoints`, `Default`, and
`VerifyTimeout`. Delete `DefaultSharingProvider`, `Config.SharingProvider()`, and `validateSharing`'s
provider check, replacing the latter with validation of the new keys. No migration path.

**Rationale**: The feature is unreleased, and the user confirmed backwards compatibility is
irrelevant. Retaining a dead `provider` key would invite configurations that silently do nothing.
Validation moves to the new shape: an address must parse as an absolute URL with an `http` or `https`
scheme, a declared default must name a configured endpoint, and the timeout must be positive.

**Alternatives considered**: accepting `provider` with a deprecation warning. Rejected because there
is no released version to deprecate from.

## R8: Test strategy for the end to end acceptance test

**Decision**: One `TestMain`-gated integration test that skips when `zellij` is absent, following the
existing compose smoke test pattern. It starts a real Zellij web server through the existing
`EnsureWebServer`, creates a real background session, points a static endpoint at the local web
server address, and runs the real five stage probe. Unit level stage failures are covered separately
by an `httptest.Server` that can fail at each stage independently.

**Rationale**: `SC-002` is explicit that the blank terminal case must be caught by an automated test.
The lesson recorded in the source design is that a complete unit suite passed while the feature had
never worked for a human, so the unit fakes alone are insufficient. Pointing the endpoint at the
local web server means the test exercises the real contract without needing a tunnel, which keeps it
runnable in continuous integration.

**Critical constraint**: any Zellij command that creates a session must run as
`env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID zellij ...`. Inside an existing session,
`zellij --layout NAME attach -b SESSION` adds a tab to the current session, exits zero, and creates
nothing. This is the same defect as the unguarded `EnsureSession` folded into this feature, and it
will silently corrupt the test's own environment if missed.

**Alternatives considered**: a fully mocked end to end test. That is what exists today, and it passed
while the feature was broken.
