# Research: 025-sidebar-state-refresh

**Date**: 2026-03-21

## R1: Grace Period Duration

**Decision**: 3 seconds after permission grant.

**Rationale**: The timer interval is 10 seconds (default), and PaneUpdate events fire frequently during startup. Empirical testing shows Zellij delivers complete pane manifests within 1-2 PaneUpdate cycles after reattach. A 3-second window provides a comfortable margin while keeping stale entries visible for only a brief moment (FR-006 requires "a few seconds").

**Alternatives considered**:
- **1 second**: Too aggressive; on slower systems the manifest may still be incomplete.
- **5 seconds**: Unnecessarily long; stale entries would be visible for too long.
- **Event-count based** (e.g., after N PaneUpdate events): More complex, harder to reason about, and N would still be arbitrary. Time-based is simpler and predictable.
- **Content-stability based** (consecutive identical manifests): Adds complexity (must store and diff previous manifests). The manifest can change for legitimate reasons (new pane opened) which would reset the stability counter inappropriately.

## R2: Implementation Approach for Grace Period

**Decision**: Add a `startup_grace_until` field (Option<u64>, milliseconds) to `PluginState`. Set it to `unix_now_ms() + 3000` at permission grant. In `PaneUpdate` handler, skip `remove_dead_sessions()` while `unix_now_ms() < startup_grace_until`.

**Rationale**: A single timestamp field is the simplest possible implementation. No new timers, no new events, no state machine. The check is a single comparison in the hot path.

**Alternatives considered**:
- **Boolean flag + separate timer**: Would require coordinating a one-shot timer with the existing periodic timer. More complex for no benefit.
- **PaneUpdate counter**: Count PaneUpdate events and skip the first N. Less predictable than wall-clock time; N events may arrive in quick succession or slowly depending on session complexity.
- **Deferred event queue**: Queue PaneUpdate events during grace period and replay after. Over-engineered; the only thing we need to defer is `remove_dead_sessions`, not the entire PaneUpdate processing (we still want `rebuild_pane_map` to run for tab info).

## R3: Interaction with Existing Cleanup Mechanisms

**Decision**: The grace period only affects `remove_dead_sessions()`. All other cleanup mechanisms continue operating normally.

**Rationale**:
- `cleanup_stale_sessions()` (timer-based Done-to-Idle transition) operates on activity states, not pane existence. No interaction with the grace period.
- `sync::apply_session_meta()` applies metadata overrides. No interaction.
- Session removal via hook events (session end) is user-initiated and should not be deferred.
- `rebuild_pane_map()` must run on every PaneUpdate (even during grace period) to keep tab info current.

## R4: Zellij PaneUpdate Behavior on Reattach

**Decision**: Trust the spec's assumption that PaneUpdate eventually delivers a complete manifest, based on existing code behavior.

**Rationale**: The existing `remove_dead_sessions` code at `state.rs:167-171` already has a guard for empty manifests, confirming that transient incomplete states during startup are a known Zellij behavior. The code comment says "could be a transient state during startup." The grace period extends this protection from "completely empty" to "partially populated."
