# Deep Review Findings

**Date:** 2026-09-07
**Branch:** 087-workspace-sharing-endpoint
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 1 | 1 | 0 |
| Important | 8 | 8 | 0 |
| Minor | 8 | 6 | 2 |
| Notable | 7 | - | 7 |
| **Total** | **24** | **15** | **9** |

**Agents completed:** 5/6 (Goal Alignment skipped, no PR exists) + 1 external tool
**External tools:** CodeRabbit completed (9 findings, 2 in scope); Codex failed (usage limit); Copilot skipped (not installed)

## Findings

### FINDING-1
- **Severity:** Critical
- **Confidence:** 97
- **File:** cc-deck/internal/share/probe_test.go:482-492
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`TestProbeStageMatrixCoversEveryDeclaredStage` built its coverage map from a hardcoded literal list of stages rather than from the matrix it claimed to be guarding. The two lists could drift apart without any test noticing.

**Why this matters:**

SC-003 names this test as the measurement method for "the correct layer is named in 100% of layer-specific cases". If a row can be deleted from the matrix while this test still passes, the criterion is unenforceable and the guard asserts nothing at exactly the moment it would matter.

**How it was resolved:**

The matrix is now a named type built by `probeStageMatrix()`, and the exhaustiveness check derives its coverage map from the rows the sibling test actually runs. Verified by mutation: deleting the "upgrade refused" row now fails the test, where the previous version passed the same mutation.

### FINDING-2
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/share/service.go:206-211
- **Category:** production-readiness / correctness
- **Source:** production-readiness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

Two distinct bound defects in `verify()`. First, the configured budget was applied only when the caller had no deadline, so a caller with a *longer* deadline bypassed `verifyTimeout` entirely. Second, the deferred credential revocation runs on its own 5s context, so the effective wall-clock bound was `verifyTimeout + 5s`.

**Why this matters:**

FR-012 requires the whole verification to complete within a bounded, configurable time defaulting to fifteen seconds. Both defects make the real bound larger than the configured one. A bound that is quietly larger than it claims is not a bound.

**How it was resolved:**

`WithTimeout` is now applied unconditionally (it already keeps whichever deadline is sooner, so a caller with a shorter deadline still wins), and the revocation grace is reserved out of the budget with a floor so a small configured timeout cannot starve the probe. Three regression tests cover the longer-caller, shorter-caller, and total-budget cases.

**External tool analysis (CodeRabbit):**
> Update the probe context setup around probeCtx and verifyTimeout to compare the caller's deadline with s.verifyTimeout, applying the earlier deadline. Preserve the caller context when its deadline is sooner, and create a timeout context when verifyTimeout is sooner or no caller deadline exists.

### FINDING-3
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/share/service.go:264-270
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`Invite()` ran a verification but never recorded its result. On failure it returned the error without extracting the `ProbeResult`; on success the in-memory update could be discarded by a later failure before `Save`.

**Why this matters:**

FR-047 requires every completed verification to update the stored result. Without it, a user sees a failure interactively and then the next `cc-deck ws` contradicts them with a stale age. `Status()` did this correctly; `Invite()` did not.

**How it was resolved:**

`Invite` now calls `recordProbe` on both paths, and records before minting the credential so a later failure cannot discard an observation that was genuinely made. Covered by `TestInviteRecordsAFailedVerification`.

*This was a real gap in the Stage 1 compliance pass, which had scored FR-047 as compliant on the strength of `Status()` alone.*

### FINDING-4
- **Severity:** Important
- **Confidence:** 82
- **File:** cc-deck/internal/share/service.go:333-390, lock.go:19-33
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`Snapshot()` took the exclusive lifecycle lock. A concurrent `ws status` holds that lock for the length of a full network probe, so a listing would spin-poll behind it for up to ~30 seconds.

**Why this matters:**

SC-008 requires a shared listing to cost roughly what an unshared one costs. Blocking on a concurrent probe misses that by orders of magnitude, and the whole point of `Snapshot` is to be the cheap read.

**How it was resolved:**

`Snapshot` no longer takes the lock. `FileStore.Save` writes through a temp file and `os.Rename`, so a lock-free reader always sees one complete version. Covered by `TestSnapshotDoesNotWaitOnTheLifecycleLock`, which holds the lock from another goroutine and fails if the listing blocks.

### FINDING-5
- **Severity:** Important
- **Confidence:** 92
- **File:** cc-deck/internal/share/model.go (Transition), service.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`Transition()` presents itself as the authoritative state machine but governs one of five production transition sites; teardown, rollback, and reconciliation assign `State` directly.

**Why this matters:**

Reading the transition map gives an incomplete picture of what the type actually does, and adding a state would require auditing direct assignments rather than updating the map.

**How it was resolved:**

Documented rather than restructured. The direct assignments are all pre-existing (verified: they appear as context lines in this feature's diff), and the plan explicitly retained the state machine as not being a source of the failures. `Transition` now says it governs the start path only and that teardown deliberately bypasses it.

### FINDING-6
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/share/probe_test.go:496-529
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

The "on deadline expiry" case in the credential lifecycle test returns early when nothing was minted, so it could pass without asserting anything about revocation under a deadline.

**Why this matters:**

FR-046 requires that a probe which times out still revokes what it minted. The test was supposed to prove it and instead proved nothing whenever the short budget expired before the auth stage.

**How it was resolved:**

Added `stallControlChannel` to the endpoint fake and `TestProbeRevokesItsCredentialWhenTheDeadlineStrikesAfterMinting`, which answers every earlier stage and then stalls on the control channel so the credential always exists when the budget runs out.

### FINDING-7
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/share/probe_test.go, service_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

SC-013 says no credential may be "written to disk", but every assertion checked a Go struct's `%+v` through an in-memory store fake. A serializer bug would not be caught.

**Why this matters:**

The gap is between what the test proved (the model has no secret field) and what the criterion requires (the file on disk has no secret).

**How it was resolved:**

`TestNoCredentialEverReachesTheStateFileOnDisk` runs a real share through the real `FileStore` and reads the bytes back, asserting no secret is present and that the file is mode 0600.

### FINDING-8
- **Severity:** Important
- **Confidence:** 82
- **File:** cc-deck/internal/cmd/ws_share_test.go:413-436
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`require.Zero(t, recorder.probeCalls)` was a tautology: the counter is only incremented by `Status()`, so it proved the listing chose `Snapshot`, not that no probing is reachable.

**Why this matters:**

FR-017 forbids network access during a listing. A counter on a fake cannot detect a future change that routes listings back through verification.

**How it was resolved:**

Added `TestListingNeverReachesTheVerifyingRead`, whose service calls `t.Fatal` if a listing ever invokes `Status`. The weaker counter assertion is kept alongside it.

### FINDING-9
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/cmd/ws.go:1414-1417
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

The residual de-duplication used `len(residuals) == 0`, which drops a genuinely distinct status error whenever unrelated residuals are already present.

**How it was resolved:**

Now compares content with `slices.Contains` rather than assuming emptiness.

**External tool analysis (CodeRabbit):**
> The residual handling around Status must preserve a distinct status error even when unrelated residuals already exist. Update the condition to append err.Error() only when that exact error is not already present in residuals, rather than checking len(residuals) == 0.

### FINDING-10
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/share/probe.go:307-314
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

The session cookie is concatenated into a hand-built HTTP request. CRLF injection protection rested entirely on Go's cookie parser three call frames away.

**Why this matters:**

The cookie value comes from the endpoint, which is not necessarily well behaved. The defense was implicit and would vanish silently if the cookie were ever obtained differently.

**How it was resolved:**

The one place that could be injected now checks for control bytes itself and fails the auth stage if any are present.

### FINDING-11
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-deck/internal/share/probe.go:51-54
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`Probe` is a public interface method with no intrinsic bound; the no-hang guarantee relied on `verify()` always being in the call chain.

**How it was resolved:**

`Probe` now imposes `DefaultVerifyTimeout` when the caller supplies no deadline.

### FINDING-12
- **Severity:** Minor
- **Confidence:** 95
- **File:** cc-deck/internal/share/service.go:27-29
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

`DefaultVerifyTimeout` was declared independently in both `config` and `share`, with a comment claiming they match and nothing enforcing it.

**How it was resolved:**

The share constant is now an alias of `config.DefaultVerifyTimeout`, making the coupling structural.

### FINDING-13
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/share/probe_test.go:547-570
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

The C-9 no-secrets test covered only the three stages that run after a credential exists, leaving the reasoning about DNS and TLS implicit.

**How it was resolved:**

`TestProbeDiagnosticsCarryNoSecretsBeforeACredentialExists` covers both, and asserts that no credential was minted at those stages, which is what makes the claim explicit rather than assumed.

### FINDING-14
- **Severity:** Minor (from a Notable)
- **File:** cc-deck/internal/share/probe.go:319-322
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

A comment claimed a "HEAD-shaped request object" prevented body reads on a 101, but the method is GET and the real reason is that Go treats 101 as a protocol switch regardless of method.

**How it was resolved:**

Comment corrected. A misleading comment in the WebSocket path carries elevated risk of a future mistake.

### FINDING-15
- **Severity:** Minor (from a Notable)
- **File:** cc-deck/internal/share/probe.go:29, docs/modules/using/pages/sharing.adoc
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**

On resolution failure the probe queries Cloudflare's resolver at `1.1.1.1`, sending the endpoint hostname off the machine. Neither the code comment nor the documentation named the third party or disclosed the outbound query.

**How it was resolved:**

The constant now names Cloudflare and explains when it is consulted, and the sharing guide carries a NOTE admonition disclosing the behaviour for audited networks. Making the resolver configurable remains open (see Remaining).

## Remaining Findings

Two Minor findings were deliberately not auto-fixed. Neither gates the review.

### REMAINING-1: file naming no longer matches contents
- **Severity:** Minor
- **File:** cc-deck/internal/share/provider.go, provider_contract_test.go
- **Source:** architecture-agent

`provider.go` contains no provider; it holds the package interfaces. `provider_contract_test.go` tests the `Endpoint` contract.

**Why not fixed:** plan.md names these files explicitly in its file tree (`provider.go REWRITE`), so renaming them departs from the approved plan and adds rename churn to a diff that is already large. The rename is correct and cheap, but it is the user's call whether to take it now or in a follow-up.

### REMAINING-2: probe credential revocation failure is silent
- **Severity:** Minor
- **File:** cc-deck/internal/share/probe.go:138-140
- **Source:** security-agent, production-readiness-agent (both)

`_ = e.zellij.RevokeToken(...)` discards the error. A systematically broken revocation would accumulate live credentials with no signal.

**Why not fixed:** the suggested remedies either add a `RevocationFailed` field to `ProbeResult` (which is a persisted model change, and the model is fixed by data-model.md) or introduce logging into a package that currently has none. Both are spec-touching decisions rather than review fixes. The revocation itself is correct and runs on every exit path; only its failure is unobservable.

## Notable Observations

Captured to `brainstorm/idea-inbox.md` for future brainstorming. Not fixed, not gating.

### NOTABLE-1
- **File:** cc-deck/internal/share/endpoint.go:41
- **Category:** architecture
- **Description:** `StaticEndpoint.Name()` returns the implementation kind while `EndpointRef.Name` carries the configured endpoint name. Two meanings of "name" in one package.
- **Rationale:** worth a rename to `Kind()` if a second `Endpoint` implementation ever arrives.

### NOTABLE-2
- **File:** cc-deck/internal/share/invitation.go:14-26
- **Category:** architecture
- **Description:** `BuildInvitations` (plural) and `InvitationSet` have no production caller and are kept alive only by their own tests.
- **Rationale:** pre-existing dead code, not introduced by this feature, but tests over dead code give false coverage confidence.

### NOTABLE-3
- **File:** cc-deck/internal/share/service.go (recordProbe, reconcileLocked)
- **Category:** correctness
- **Description:** Both discard the `store.Save` error, so a disk failure silently loses an observation.
- **Rationale:** deliberate (an observation must not become a state change, and cleanup must not be blocked by persistence), but it means FR-047's "MUST update" is best-effort under disk error.

### NOTABLE-4
- **File:** cc-deck/internal/share/probe.go, zellij.go
- **Category:** production-readiness
- **Description:** Every `ws status` mints and revokes a Zellij token, costing two subprocess spawns, so status will never be sub-second even when healthy.
- **Rationale:** an accepted trade for storing no standing secret, recorded in the spec's assumptions.

### NOTABLE-5
- **File:** cc-deck/internal/share/e2e_test.go:51-59
- **Category:** test-quality
- **Description:** All three end to end tests skip when zellij is absent, which is the only coverage of the real network stack and real binary.
- **Rationale:** intentional and documented, but CI without Zellij loses the coverage that caught the `web_client_id` disagreement.

### NOTABLE-6
- **File:** cc-deck/internal/cmd/ws_share_test.go:225-240
- **Category:** test-quality
- **Description:** `stubWorkspace` embeds a nil interface, so an unexpected call panics rather than failing with a message.
- **Rationale:** deliberate, and the panic is itself the assertion, but the failure message would be poor.

### NOTABLE-7
- **File:** cc-deck/internal/share/probe.go:142
- **Category:** security
- **Description:** The credential's in-memory lifetime is not scoped to the smallest possible window; `bytes.NewReader` holds the login body.
- **Rationale:** acceptable for a CLI; redaction coverage was confirmed complete on every diagnostic path.

## Post-Fix Spec Coverage

The fix round removed no production code (only a conditional and a lock acquisition), but the coverage check was run anyway because FINDING-3 was itself a dropped-requirement finding.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-012 bounded, configurable | service.go verify() + 3 regression tests | ✓ (strengthened) |
| FR-017 listing no network | service.go Snapshot() lock-free + guard test | ✓ (strengthened) |
| FR-046 revoke on timeout | probe.go defer + deterministic stall test | ✓ (strengthened) |
| FR-047 record every verification | Status() and now Invite() | ✓ (was a gap, now fixed) |
| SC-003 stage attribution | probeStageMatrix() + derived exhaustiveness guard | ✓ (was unenforceable, now mutation-verified) |
| SC-013 no credential on disk | disk-level assertion on real FileStore | ✓ (strengthened) |

All other requirements unchanged from the Stage 1 pass.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1 | make test | 1 | 16 | pre-existing only |

The 16 failures are identical to the T001 baseline recorded before any work began: 9 `TestComposeSmoke*` (podman environment) and 7 `TestVoiceRelay_*` (reads real user configuration). No new failures. `make lint` passes clean.

The end to end tests against a real Zellij 0.45.1 web server pass after the fix round, confirming the bound reservation and the cookie control-character check did not break the real handshake.
