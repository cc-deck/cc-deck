# Code Review Guide

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 5 source files (state.rs, mod.rs, events.rs, render_broadcast.rs, integration_tests.rs), 1 test helper (test_helpers.rs), 1 documentation file (README.md)

### Understanding the changes (8 min)

- Start with `cc-zellij-plugin/src/controller/state.rs`: Four new fields added to ControllerState (`is_leader`, `leader_plugin_id`, `last_leader_ping_ms`, `election_ticks`) plus three constants. This is the data model for the entire feature.
- Then `cc-zellij-plugin/src/controller/mod.rs`: Two dormant guards (one in `pipe()`, one in `update()`) plus the ping/pong handler and startup ping broadcast. This is the core behavior change.
- Question: Are the dormant guards placed at the right level? They block at the trait method level rather than inside individual handlers. Does this create any risk of blocking something that should get through?

### Key decisions that need your eyes (12 min)

**Dormant guard allows Timer events through** (`cc-zellij-plugin/src/controller/mod.rs:136-141`, relates to [FR-008a](spec.md#functional-requirements))

The dormant guard in `update()` blocks all events except Timer (needed for election tick counting and re-activation checks). The `handle_timer` function returns early after election logic when dormant, skipping all leader-only work. The spec's FR-008a lists Timer among events to skip, but FR-006 and FR-010 require Timer for the election to work.
- Question: Is the early return at `events.rs:185` sufficient to prevent dormant controllers from doing leader-only timer work, or should the election logic be extracted into a separate function?

**Ping/pong handler reuses same code path for both** (`cc-zellij-plugin/src/controller/mod.rs:428-451`)

ControllerPing and ControllerPong are handled identically. The spec mentions ControllerPong for backward compatibility, but the election protocol only uses pings.
- Question: Should ControllerPong be treated differently (perhaps ignored entirely), or is treating them identically the safer approach?

**broadcast_render_all is called on every render** (`cc-zellij-plugin/src/controller/render_broadcast.rs:103`)

The untargeted broadcast is sent after every targeted render cycle. With leader election ensuring only one active controller, this is safe but adds one extra pipe message per render.
- Question: Should the untargeted broadcast only fire when the sidebar registry is empty or recently changed, or is the simplicity of always broadcasting worth the minor overhead?

**setup_controller test helper sets is_leader = true** (`cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs:183`)

All existing integration tests bypass the election protocol by setting `is_leader = true` directly. Election-specific tests create controllers manually without the helper.
- Question: Does this test strategy provide adequate coverage, or should there be a `setup_controller_dormant()` helper for more systematic election testing?

### Areas where I'm less certain (5 min)

- `cc-zellij-plugin/src/controller/mod.rs:117-125` ([FR-001](spec.md#functional-requirements)): The startup ping is broadcast after replaying pending events. If a pending Timer event triggers election logic before the ping, the election_ticks counter could be off by one. In practice this is unlikely since permissions are granted very early, but the ordering dependency is subtle.
- `cc-zellij-plugin/src/controller/events.rs:163-170` ([FR-006](spec.md#functional-requirements)): The leader failure check uses `unix_now_ms()` which is wall clock time. If the system clock jumps backward (NTP correction), the `saturating_sub` prevents underflow but could extend the timeout window. WASI clocks may behave differently than system clocks.
- `cc-zellij-plugin/src/controller/mod.rs:443-445`: When a higher-ID controller pings us, we respond with our own ping. If both controllers are simultaneously in the startup phase, this creates a ping-pong exchange. The protocol converges (lower ID always wins), but there is no deduplication of outgoing pings.

### Deviations and risks (5 min)

- `cc-zellij-plugin/src/controller/mod.rs:136-141`: The dormant guard allows Timer events through, which deviates from the literal text of [FR-008a](spec.md#functional-requirements) ("skip all event processing... Timer"). This is necessary for [FR-006](spec.md#functional-requirements) and [FR-010](spec.md#functional-requirements) to function. The election logic returns early before any leader-only timer work. Question: "Should FR-008a be updated to explicitly note that Timer is partially processed for election purposes?"
- No other deviations from [plan.md](plan.md) were identified.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-15 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 2 | completed |
| Architecture & Idioms | 4 | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 3 | completed |
| CodeRabbit (external) | 1 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 5 | - | 5 |

### What was fixed automatically

Fixed a CLI pipe hang where the dormant guard returned early without unblocking CLI-sourced pipe messages (correctness agent, confirmed by CodeRabbit). Fixed the leader failure re-activation path to broadcast a ping on self-promotion, preventing simultaneous dual-leader activation after a 60-second timeout. Consolidated duplicate `broadcast_controller_ping` functions into a single shared implementation. Added three missing tests for leader demotion, leader failure recovery, and no-payload ping handling.

### What still needs human attention

All Critical and Important findings were resolved. 5 Minor findings remain (see [review-findings.md](review-findings.md) for details). No further review action needed, but reviewers may want to check the Minor findings during code review.

- The double-send pattern in `render_broadcast.rs` (targeted + untargeted broadcast) is by design per [FR-007](spec.md), but adds N extra pipe messages at steady state. Worth revisiting if sidebar counts grow significantly.
- `ControllerPong` is never sent by any code path. Is it worth keeping for forward compatibility, or should it be removed to reduce confusion?

### Recommendation

All findings addressed. Code is ready for human review with no known blockers.
