# Deep Review Findings

**Date:** 2026-07-25
**Branch:** 084-zellij-session-sharing
**Rounds:** 3
**Gate Outcome:** PASS
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|------:|------:|----------:|
| Critical | 1 | 1 | 0 |
| Important | 13 | 13 | 0 |
| Minor | 6 | - | 6 |
| Notable | 2 | - | 2 |
| **Total** | **22** | **14** | **8** |

**Agents completed:** 5/5 (+ Codex and CodeRabbit external)
**Agents failed:** Codex's own test subprocess could not access its default Go cache, but its review completed.

## Findings

### FINDING-1 — workspace stop could leave public access active
- **Severity:** Critical
- **Confidence:** 95
- **File:** `cc-deck/internal/cmd/ws.go:1648`
- **Category:** production-readiness
- **Source:** production-readiness agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

`ws stop` discarded sharing-service and status errors, then reported success after killing the workspace even when endpoint and credentials could remain active. Stop now aggregates sharing errors, attempts reconciliation before workspace shutdown, and prints completion only when all teardown actions succeed.

### FINDING-2 — workspace-scoped commands mutated another workspace
- **Severity:** Important
- **Confidence:** 100
- **File:** `cc-deck/internal/cmd/ws_share.go:127`
- **Category:** correctness/security
- **Source:** correctness agent (also: architecture, security, production-readiness, test-quality, Codex external)
- **Round found:** 1
- **Resolution:** fixed (round 1)

Invite discarded its resolved workspace, while revoke and unshare never enforced one. Workspace identity is now carried into `Invite`, `Revoke`, and `Stop` and checked under the lifecycle lock. Cobra-level regression coverage verifies all three commands pass the resolved workspace.

### FINDING-3 — list/status bypassed lifecycle reconciliation
- **Severity:** Important
- **Confidence:** 95
- **File:** `cc-deck/internal/cmd/ws.go:1294`
- **Category:** correctness/architecture/security
- **Source:** correctness agent (also: architecture, security, production-readiness)
- **Round found:** 1
- **Resolution:** fixed (round 1)

Presentation code loaded persisted YAML directly, so dead sessions/providers could remain reported as shared without cleanup. List and status now project the result of `SharingService.Status`, including degraded residuals after reconciliation.

### FINDING-4 — structured list omitted sharing metadata
- **Severity:** Important
- **Confidence:** 95
- **File:** `cc-deck/internal/cmd/ws.go:1036`
- **Category:** correctness
- **Source:** correctness agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

JSON/YAML list entries lacked sharing state, endpoint, invitation summaries, and residuals. These safe fields are now emitted, with a regression test confirming labels/roles appear and secrets do not.

### FINDING-5 — container creation ensured a session before the container existed
- **Severity:** Important
- **Confidence:** 100
- **File:** `cc-deck/internal/ws/container.go:246`
- **Category:** external/correctness
- **Source:** Codex external
- **Round found:** 1
- **Resolution:** fixed (round 1)

Every new container workspace attempted `podman exec` before `podman run`. Session creation now occurs after the container starts, and the new container is removed if session creation fails.

**External tool analysis (Codex):** The old ordering either failed for a missing container or created a session in an orphan that was immediately removed.

### FINDING-6 — recovery depended on current provider configuration
- **Severity:** Important
- **Confidence:** 95
- **File:** `cc-deck/internal/cmd/ws_share.go:17`
- **Category:** security/production-readiness
- **Source:** security agent (also: correctness)
- **Round found:** 2
- **Resolution:** fixed (round 2)

Malformed or changed configuration could prevent cleanup of an already-persisted Cloudflare operation. Configuration selection is now applied only to new starts; V1 lifecycle recovery always constructs the persisted Cloudflare adapter.

### FINDING-7 — teardown retries repeated completed safety actions
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/share/service.go:437`
- **Category:** production-readiness
- **Source:** production-readiness agent
- **Round found:** 1
- **Resolution:** fixed (round 3)

Partial cleanup did not checkpoint successful credential, endpoint, or web-server actions, so retries could remain degraded on already-stopped resources. Cleanup progress is persisted per resource and retry tests verify completed actions are skipped.

### FINDING-8 — detached guard operations were unbounded
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/share/guard.go:145`
- **Category:** production-readiness/security
- **Source:** production-readiness agent (also: security)
- **Round found:** 1
- **Resolution:** fixed (round 3)

Provider/status probes and cleanup used unbounded background contexts. Guard probes now have two-second deadlines and cleanup has an independent ten-second deadline. A status timeout invokes bounded teardown instead of abandoning an unknown active operation.

### FINDING-9 — sharing CLI wiring lacked regression coverage
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/cmd/ws_share_test.go:43`
- **Category:** test-quality
- **Source:** test-quality agent
- **Round found:** 1
- **Resolution:** fixed (round 3)

Service tests could not detect the original Cobra argument-discard bug. New command-layer tests exercise invite, revoke, and unshare resolution and verify the intended workspace reaches the service boundary.

### FINDING-10 — sharing view output lacked regression coverage
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/cmd/ws_share_test.go:78`
- **Category:** test-quality
- **Source:** test-quality agent
- **Round found:** 2
- **Resolution:** fixed (round 3)

Structured sharing output is now tested for degraded state, endpoint, invitation label/role, residuals, and secret omission through the production projection path.

### FINDING-11 — invitation labels allow terminal control characters
- **Severity:** Minor
- **Confidence:** 80
- **File:** `cc-deck/internal/share/service.go:257`
- **Category:** security
- **Source:** security agent
- **Round found:** 1
- **Resolution:** remaining (non-gating)

User-provided labels are argument-safe but may contain newlines or terminal control sequences. A future hardening change should constrain labels to a short printable pattern.

### FINDING-12 — production provider construction bypasses the registry
- **Severity:** Minor
- **Confidence:** 80
- **File:** `cc-deck/internal/share/provider.go:59`
- **Category:** architecture
- **Source:** architecture agent
- **Round found:** 1
- **Resolution:** remaining (non-gating)

V1 directly composes Cloudflare while `ProviderRegistry` is test-oriented. This is acceptable for the single-provider boundary but should be consolidated before adding another provider.

### FINDING-13 — duplicate invitation aggregate API
- **Severity:** Minor
- **Confidence:** 75
- **File:** `cc-deck/internal/share/invitation.go:14`
- **Category:** architecture
- **Source:** architecture agent
- **Round found:** 1
- **Resolution:** remaining (non-gating)

`InvitationSet`/`BuildInvitations` duplicate the production role-aware invitation path and can drift.

### FINDING-14 — provider states are untyped strings
- **Severity:** Minor
- **Confidence:** 75
- **File:** `cc-deck/internal/share/provider.go:24`
- **Category:** architecture
- **Source:** architecture agent
- **Round found:** 1
- **Resolution:** remaining (non-gating)

Provider lifecycle comparisons repeat string literals. A typed state enum would make future provider evolution safer.

### FINDING-15 — lock test covers goroutines, not helper processes
- **Severity:** Minor
- **Confidence:** 75
- **File:** `cc-deck/internal/share/lock_test.go:12`
- **Category:** test-quality
- **Source:** test-quality agent
- **Round found:** 1
- **Resolution:** remaining (non-gating)

The filesystem lock is exercised concurrently in-process; a helper-process test would strengthen cross-command coverage.

### FINDING-16 — failed container startup reused a canceled context for cleanup
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/ws/container.go:249`
- **Category:** external/production-readiness
- **Source:** CodeRabbit
- **Resolution:** fixed (post-round external verification)

Container removal after session-start failure now uses an independent bounded cleanup context and preserves the original startup error.

### FINDING-17 — newly created container persisted a missing session state
- **Severity:** Important
- **Confidence:** 95
- **File:** `cc-deck/internal/ws/container.go:309`
- **Category:** external/correctness
- **Source:** CodeRabbit
- **Resolution:** fixed (post-round external verification)

`EnsureSession` ran before the instance existed, so its state update was necessarily skipped. The subsequently persisted instance now records `SessionStateExists`.

### FINDING-18 — guard teardown was not scoped to its persisted workspace
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/share/guard.go:136`
- **Category:** external/security
- **Source:** CodeRabbit
- **Resolution:** fixed (post-round external verification)

The guard now retains the validated operation workspace and supplies it to `Service.Stop`, preventing it from tearing down a different workspace operation after the validation lock is released.

### FINDING-19 — canceled fingerprint probes looked like a vanished process
- **Severity:** Important
- **Confidence:** 90
- **File:** `cc-deck/internal/share/guard.go:120`
- **Category:** external/correctness
- **Source:** CodeRabbit
- **Resolution:** fixed (post-round external verification)

Fingerprint probing now returns context cancellation/deadline errors before applying the genuine process-gone fallback. A regression test covers the distinction.

### FINDING-20 — table rendering test depended on host sharing state
- **Severity:** Minor
- **Confidence:** 85
- **File:** `cc-deck/internal/cmd/ws_new_test.go:291`
- **Category:** external/test-quality
- **Source:** CodeRabbit
- **Resolution:** fixed (post-round external verification)

The test now uses an isolated `CC_DECK_SHARE_STATE_FILE`.

### Rejected external suggestion — validate current provider for mutations

CodeRabbit suggested applying current configuration validation to invite, revoke, and unshare. This was rejected because lifecycle recovery must be driven by the persisted active operation. Applying current configuration would make credential revocation and endpoint teardown fail after configuration drift or corruption—the exact fail-closed defect fixed in round 2.

## Notable Observations

### NOTABLE-1 — live observer acceptance remains an empirical follow-up

The in-repository observer acceptance test uses a controlled behavioral transport rather than a live Zellij/Cloudflare endpoint. The already-recorded session-sharing hardening brainstorm covers live browser/terminal role verification, so this was not duplicated in the idea inbox.

### NOTABLE-2 — external provider parity needs a live-runtime tier

The shared provider contract covers controlled implementations, while full Cloudflare parity requires optional external infrastructure. This belongs with the existing verification-test hardening follow-up rather than the default unit tier.

## Test Suite Results

| Round | Test Command | Result | Status |
|------:|--------------|--------|--------|
| 1 | focused `go test` for share/ws/cmd paths | 0 | passed |
| 2 | focused `go test` for share/ws/cmd paths | 0 | passed |
| 3 | `go test -timeout 90s ./internal/share ./internal/ws ./internal/cmd` | 1 | feature packages passed; existing Podman compose smoke tests could not access the sandboxed Podman socket |
| 3 | same command with `-skip '^TestComposeSmoke'` | 0 | passed |

The failing external compose tier is pre-existing verification debt tracked by the existing verification-test hardening brainstorm; no feature regression was identified.
