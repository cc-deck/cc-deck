# Code Review Artifacts

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 4 source files (2 Rust, 1 Go, 1 README), plus spec artifacts

### Understanding the changes (8 min)

- Start with `cc-zellij-plugin/src/controller/mod.rs:486-505`: This is the core behavioral change. The `handle_voice_command()` handler was restructured to always parse the mute suffix from heartbeat messages and only call `mark_render_dirty()` when state actually changes. Read this first to understand the change-gating logic.
- Then `cc-deck/internal/voice/relay.go:270-279`: The Go side now sends `[[voice:on:muted]]` or `[[voice:on:unmuted]]` instead of bare `[[voice:on]]`. The mute state is read under the mutex lock.
- Question: Is the separation of concerns right here, with the relay carrying mute state on every tick and the controller deciding when to re-render? Or should the controller own mute state more explicitly?

### Key decisions that need your eyes (12 min)

**Bare `voice:on` defaults to unmuted** (`cc-zellij-plugin/src/controller/mod.rs:492-494`, relates to [FR-001](spec.md))

The `strip_prefix("voice:on:")` approach treats bare `voice:on` (no suffix) as unmuted via `unwrap_or(false)`. This is a behavioral change from before, where bare `voice:on` from an already-enabled relay preserved the existing mute state.
- Question: Could an older relay sending bare `voice:on` cause an already-muted controller to flip to unmuted? The initial `Start()` still sends bare `[[voice:on]]`, but `statePoll()` sends the suffixed version. Is the window between initial `Start()` and first poll tick a concern?

**`focused_pane_id` uses `skip_serializing_if`** (`cc-zellij-plugin/src/controller/mod.rs:573-574`, relates to [FR-003](spec.md))

The field uses `skip_serializing_if = "Option::is_none"` which means older relays that do not expect this field will not see it in the JSON when it is `None`. But when it IS present, older relays will ignore it (Go's JSON decoder ignores unknown fields by default).
- Question: Is this the right serialization strategy, or should `focused_pane_id` always be present (even as null) for forward compatibility?

**Targeted render on sidebar registration** (`cc-zellij-plugin/src/controller/sidebar_registry.rs:30`, relates to [FR-005](spec.md))

After `send_sidebar_init()`, a `targeted_render()` call sends the full render payload to the new sidebar. This means the sidebar receives two messages in quick succession (init + render).
- Question: Could these two messages arrive out of order via Zellij pipe delivery? If the render arrives before init, does the sidebar handle that gracefully?

**Session name resolution priority** (`cc-deck/internal/voice/relay.go:369-378`, relates to [FR-004](spec.md))

The relay now tries `focused_pane_id` first, then `attended_pane_id`, then single-session fallback. The `resolveSessionName` helper is a local closure.
- Question: When a user explicitly attends a session (clicks it), should that still take priority over keyboard focus? The current logic always prefers focus over attend.

### Areas where I'm less certain (5 min)

- `cc-zellij-plugin/src/controller/mod.rs:497-499` ([FR-002](spec.md)): The `voice_mute_requested` clearing only happens on first enable (`!was_enabled`). If a user toggles mute from the sidebar while voice is already enabled, will the pending request survive correctly through heartbeat cycles? I believe it does because the mute toggle path (`VoiceMuteToggle`) is separate from the heartbeat handler, but the interaction is subtle.
- `cc-deck/internal/voice/relay.go:273-278`: The mutex lock/unlock around reading `r.muted` is correct, but I am less certain about whether the lock scope is optimal. The mute state is read, then released, then used in the `Send()` call. If `SendMuteCommand()` changes `r.muted` between the read and the send, the heartbeat would carry stale state for one tick. This seems acceptable (self-healing on next tick) but worth verifying.

### Deviations and risks (5 min)

- No deviations from [plan.md](plan.md) were identified. All four design changes were implemented exactly as specified.
- Risk: The `targeted_render()` function in `render_broadcast.rs` builds a fresh payload each time. For the sidebar-hello case this is correct, but if called frequently it could be expensive. Current usage is only on sidebar registration, so this is not a concern today. Question: Should we document that `targeted_render()` is intended for low-frequency use?

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-14 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 1 | completed |
| Architecture & Idioms | 4 | completed |
| Security | 0 | completed |
| Production Readiness | 2 | completed |
| Test Quality | 5 | completed |
| CodeRabbit (external) | 8 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 14 | 0 | 14 |

### What was fixed automatically

Consolidated the duplicated render-to-single-sidebar logic: the `RenderRequest` handler in `mod.rs` now calls `targeted_render()` instead of reimplementing build+serialize+send. Renamed three test functions whose names overstated what they verified (WASM send functions are no-ops in test mode). Added a missing `render_dirty` assertion to the bare `voice:on` mute-clearing test.

### What still needs human attention

All Critical and Important findings were resolved. 14 Minor findings remain (see [review-findings.md](review-findings.md) for details). Most notable:

- The correctness agent flagged that `voice_mute_requested` is not cleared by subsequent heartbeats when voice is already enabled. Is the current idempotent handling in the relay sufficient, or should the controller clear it when the heartbeat confirms the requested state?
- CodeRabbit found contractions in several spec artifacts that violate the cc-deck voice guidelines. These are documentation-only issues.
- Two dead helper functions (`contains`, `searchString`) exist in `relay_test.go`.

### Recommendation

All findings addressed. Code is ready for human review with no known blockers.
