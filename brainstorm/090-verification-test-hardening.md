# Brainstorm: Verification Test Hardening

**Date:** 2026-07-24
**Status:** parked

## Problem Framing

Repository-wide verification is difficult to trust and diagnose. During verification of Zellij session sharing, `make test` and therefore `make verify` appeared to stall. A timed, verbose diagnostic showed that the test process was still running, but normal Go package-output buffering concealed progress while slow tests performed external work.

Two concrete problems surfaced:

- `go test ./...` includes the compose smoke suite in `internal/cmd`. When Podman and `podman-compose` are installed, ordinary unit verification launches containers and performs external lifecycle operations. `TestComposeSmokeFullLifecycle` failed after roughly 18 seconds during the diagnostic run.
- `TestVoiceRelay_TranscribesWhileMutedAndRecording` failed because the expected transcription event did not arrive within its collection window, indicating timing-sensitive behavior or a nondeterministic test harness.

The Makefile already exposes `test-compose` as a distinct target, but the compose tests are not isolated from `test-go`. As a result, the fast default test boundary does not match the target names or user expectations. A failure can look like a hang, and feature verification becomes coupled to local Podman state.

This is existing repository verification debt rather than a defect demonstrated in the session-sharing implementation. It should be fixed separately so the feature branch does not absorb unrelated test architecture changes.

## Approaches Considered

### A: Combined Verification Hardening

- Treat unit-test determinism, external smoke-test isolation, bounded execution, and visible diagnostics as one quality boundary.
- Keep the default `make test` reliable and independent of optional external services.
- Preserve explicit targets for slower integration and compose verification.
- Pros: Addresses the user-visible “stall” and both observed causes together; produces a coherent verification contract; prevents the same ambiguity in later features.
- Cons: Broader than repairing two individual tests and requires agreement on the repository's test tiers.

### B: Separate Voice and Test-Tier Work

- Create independent efforts for voice-test determinism and compose/Makefile isolation.
- Pros: Smaller changes and clearer subsystem ownership.
- Cons: Duplicates verification context and can leave `make test` unreliable until both efforts land.

### C: Patch Only the Observed Failures

- Stabilize the voice test and repair the failing compose lifecycle assertion without changing test boundaries.
- Pros: Smallest immediate patch.
- Cons: Ordinary verification would still launch external containers, buffered output could still look stalled, and results would remain environment-dependent.

## Decision

Park Approach A for later implementation. The follow-up should define a clear, bounded verification contract rather than merely patching the two failures observed during feature 084.

No production code, tests, or Make targets are changed as part of this brainstorm. Feature 084 task T034 remains incomplete until repository-wide verification can run to completion successfully.

## Key Requirements

- `make test` and `make verify` must terminate with actionable success or failure rather than appearing indefinitely idle.
- The default Go test tier must not require Podman, `podman-compose`, network access, image pulls, or other optional external services.
- Compose smoke tests must remain available through an explicit target and must fail or skip with a clear prerequisite message.
- Test-tier names and behavior must agree: unit/default, integration, compose smoke, and end-to-end work must have unambiguous entry points.
- All tiers must use bounded timeouts appropriate to their scope and expose enough progress to identify the active package or scenario.
- The muted-and-recording voice behavior must have a deterministic test that does not depend on arbitrary wall-clock sleeps or scheduler timing.
- The compose lifecycle suite must isolate names and resources, clean up after both success and failure, and report the external command that failed without exposing secrets.
- CI and local verification must run the same default tier; optional external tiers must be selected explicitly by environments that provide their prerequisites.
- The follow-up must document how to reproduce a failure and how to run each tier independently.

## Open Questions

- Should compose smoke tests use a build tag, a separate package, an explicit environment opt-in, or a combination of these controls?
- Should `make test` remain the fast hermetic tier, with a new aggregate target for all available external tests, or should the existing target names be reorganized?
- Which timeout and progress-reporting mechanism should be the repository default without making normal output excessively noisy?
- Is the voice failure caused only by its test harness, or does it expose a real event-delivery race in the relay?
- Which external test tiers should run in CI, and on which supported operating systems and Podman configurations?
