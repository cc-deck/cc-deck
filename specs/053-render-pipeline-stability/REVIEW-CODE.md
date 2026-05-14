# Code Review Report: Render Pipeline Stability

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 10 Rust source files across controller, sidebar_plugin,
and shared modules. No new files created, all modifications to existing code.

### Understanding the changes (8 min)

- Start with `cc-zellij-plugin/src/controller/mod.rs`: This is the core of the
  change. The `update()` and `pipe()` handlers now have defensive guards and the
  startup probe protocol (ping/pong). Read the `PermissionRequestResult` handler
  first (the probe broadcast), then the `ControllerPing`/`ControllerPong` match
  arms in `pipe()`.
- Then `cc-zellij-plugin/src/controller/events.rs`: The conditional
  `mark_render_dirty()` in `handle_tab_update` is the CPU optimization. Compare
  old (unconditional) vs new (conditional on 4 change signals).
- Question: Is the set of "change signals" (tab count, active tab, dead removal,
  stale transition) complete, or are there tab update scenarios that should also
  trigger a render but are now silently dropped?

### Key decisions that need your eyes (12 min)

**Lower plugin_id wins the probe** (`controller/mod.rs:430-458`, relates to
[FR-006](spec.md#functional-requirements))

The startup probe uses plugin_id comparison as a deterministic tiebreaker: the
controller with the lower plugin_id survives, the higher self-disables. This
assumes Zellij assigns plugin_ids in load order and the "first loaded" controller
is the legitimate one.
- Question: Is plugin_id ordering a reliable proxy for "which controller should
  survive"? Could a layout change assign a higher plugin_id to the legitimate
  controller?

**Disabled controllers still process probes** (`controller/mod.rs:151-158`)

A disabled controller can still receive and respond to ping/pong messages. This
ensures late-arriving controllers can discover the winner even after the initial
probe exchange.
- Question: Could this create a liveness issue where a disabled controller keeps
  responding to probes indefinitely, or is the one-shot nature of the probe
  sufficient?

**Push-on-discovery vs. pull-on-timer** (`controller/events.rs:97-110`,
relates to [FR-008](spec.md#functional-requirements))

New sidebars get an immediate targeted render when discovered via PaneManifest,
plus a fallback one-shot render request after 3 timer ticks. This dual approach
covers both the happy path (controller sees the sidebar first) and the edge case
(sidebar loads before PaneUpdate).
- Question: Is there a race condition where a sidebar receives the push-on-discovery
  render AND sends a render-request, causing two renders within 3 seconds?
  (Functionally harmless, but worth understanding.)

**Buffered debug logging** (`debug.rs:1-85`)

Debug logging now buffers up to 50 lines in a `thread_local!` `RefCell<Vec<String>>`
and flushes on timer tick or capacity. The original code used `unsafe static mut`
for the enabled flag, which is preserved.
- Question: The `unsafe static mut DEBUG_ENABLED` is pre-existing, but is there a
  path to eliminating it now that we have `thread_local!` for the buffer? Or does
  the flag need to be checked before the thread_local access for performance?

### Areas where I'm less certain (5 min)

- `controller/mod.rs:155-158`: The probe exemption (`is_probe` check) ensures
  disabled controllers respond to pings. I'm not 100% certain this covers the
  case where a third controller instance appears after the first two have already
  resolved. The probe is one-shot (broadcast on permission grant), so the third
  instance would ping, the winner responds with pong, and the third self-disables.
  This should work, but the three-controller scenario is untested.

- `sidebar_plugin/mod.rs:85-104`: The sidebar unsubscribes from Timer implicitly
  by returning early when `initialized` is true. However, Zellij may still deliver
  Timer events to the sidebar since it subscribed. The early return is cheap, but
  a true unsubscribe would be cleaner if the Zellij API supports it.

- `controller/events.rs:42-51`: The conditional dirty marking removes the
  unconditional `mark_render_dirty()` that previously guaranteed sidebars would
  eventually converge. If a render-relevant change happens that is not captured
  by the four conditions, sidebars could show stale data until the next 5-tick
  fade check.

### Deviations and risks (5 min)

- `controller/render_broadcast.rs:93-108`: [Plan Phase 5](plan.md#phase-5-debug-and-profiling-improvements-fr-003-fr-004)
  specified recording `render:broadcast` with serialization timing via PerfTracker.
  The implementation records the event via `perf.record_raw` in `flush_render` but
  logs serialization timing via `debug_log` in `broadcast_render` rather than
  through PerfTracker. This is because `broadcast_render` takes `&ControllerState`
  (shared reference), preventing mutation of `perf`. The deviation is minor since
  the timing data is still captured in debug logs when profiling is relevant.
  Question: Should `broadcast_render` take `&mut ControllerState` to enable direct
  PerfTracker recording?

- No other deviations from [plan.md](plan.md) were identified.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-14 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 4 | completed |
| Architecture & Idioms | 4 | completed |
| Security | 1 | completed |
| Production Readiness | 6 | completed |
| Test Quality | 7 | completed |
| CodeRabbit (external) | 16 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 9 | 2 | 7 |

### What was fixed automatically

Two Important correctness issues were resolved in round 1:

1. **Sidebar render request fallback was unreachable** (correctness agent):
   The one-shot render request required knowing the controller's plugin_id, but
   in the exact scenario it targets (no render received), the ID was unknown.
   Fixed by allowing broadcast when the target is unknown.

2. **Fade throttle never stopped broadcasting** (correctness agent): The 5-tick
   fade re-render fired whenever ANY Idle session existed, even after fading
   completed. This violated [SC-003](spec.md#success-criteria). Fixed by adding
   elapsed-time checks so fully-faded sessions no longer trigger re-renders.

Two Minor fixes were also applied: self-ping guard (skip self-delivered broadcast
pings) and redundant condition removal in the sidebar timer handler.

### What still needs human attention

All Critical and Important findings were resolved. 7 Minor findings remain
(see [review-findings.md](review-findings.md) for details). Reviewers may
want to check these during code review:

- Should `discover_sidebars_from_manifest` filter by plugin URL to avoid
  registering third-party plugins as sidebars?
- Is the ms-resolution serialization timing metric (`render_broadcast.rs:79`)
  useful, or should it be removed or relabeled?
- Should the sidebar timer stop rescheduling after a maximum number of ticks
  to avoid indefinite polling when the controller is unreachable?

### Recommendation

All findings addressed. Code is ready for human review with no known blockers.
7 Minor findings remain for consideration during code review but are not blocking.
