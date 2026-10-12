# Brainstorm: Session Sharing Hardening

**Date:** 2026-07-24
**Status:** parked

## Problem Framing

The Zellij session-sharing specification uncovered two security and lifecycle questions that require empirical validation before they can be treated as solved:

1. The existing Cloudflare Quick Tunnel spike found that terminal attachment required `--insecure`, which disables server-identity validation. This conflicts with safely distributing an interactive terminal invitation over the public Internet.
2. Zellij login tokens have no known built-in expiry. If the sharing controller exits uncleanly and cleanup stalls or fails, credentials and exposure may remain usable without a host present to observe or correct the failure.

These questions should be handled in a focused hardening spike rather than buried as assumptions in the main sharing implementation.

## Approaches Considered

### A: Block all session sharing until both questions are resolved

- Pros: Strongest fail-closed posture; no knowingly weakened access path ships.
- Cons: Delays the basic sharing experience even if browser access and normal teardown are otherwise usable.

### B: Ship insecure terminal attachment and best-effort cleanup

- Pros: Preserves the complete browser-plus-terminal experience immediately.
- Cons: Requires users to bypass server-identity validation and permits potentially unbounded residual access. Not suitable as an undocumented default.

### C: Separate the hardening spike from the initial delivery

- Pros: Makes the unresolved risks explicit and testable; allows the initial story to adopt a clearly bounded fallback instead of claiming unsupported guarantees.
- Cons: The initial story must deliberately narrow or qualify terminal access and crash recovery until the spike is complete.

## Decision

**Parked for a focused follow-up spike.** Record both blockers now and validate them later rather than solving them speculatively inside the main session-sharing story.

The spike must produce evidence strong enough to either confirm secure terminal attachment and fail-closed cleanup or recommend a revised product boundary.

## Key Requirements

### Secure terminal attachment

- Reproduce terminal attachment through a current Cloudflare Quick Tunnel with the project’s supported Zellij version.
- Determine why the earlier test required `--insecure` and whether that behavior still applies.
- Verify certificate hostname, chain, WebSocket upgrade, and client validation behavior without bypass flags.
- Compare at least one alternative exposure provider or endpoint configuration if Quick Tunnel cannot provide validated terminal attachment.
- Recommend whether terminal invitations can be enabled by default, require explicit risk acknowledgement, or must remain unavailable.

### Fail-closed lifecycle

- Confirm whether Zellij interactive and read-only tokens support expiry, external revocation, or safe database-level lifecycle control.
- Test controller termination, provider termination, network loss, host sleep, and forced process kill.
- Determine which sharing resources survive each failure and for how long.
- Evaluate a bounded-lifetime credential model, an independent supervisor, provider-bound endpoint lifetime, and startup reconciliation.
- Define a measurable maximum residual-access window and a recovery path when normal cleanup cannot complete.

## Open Questions

- Can current Zellij terminal attachment validate a Cloudflare Quick Tunnel certificate without `--insecure`?
- Is the earlier certificate mismatch a Zellij client issue, Quick Tunnel URL-shape issue, or endpoint configuration issue?
- Can Zellij login tokens be created with a bounded lifetime or safely expired outside the normal controller?
- Which independent component can enforce teardown if both the initiating command and sharing controller disappear?
- What reduced initial-release behavior is acceptable until hardening is complete: browser-only access, explicitly experimental terminal access, or no public sharing?

