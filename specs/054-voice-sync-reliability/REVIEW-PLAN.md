# Review Guide: Voice Sync Reliability

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-14

---

## What This Spec Does

Fixes four voice relay usability issues in the cc-deck Zellij plugin: the green voice indicator flickers at idle, it disappears momentarily when switching sessions, the voice relay TUI shows the wrong session name after switching focus, and mute state is lost after laptop sleep/wake recovery. The root causes are a heartbeat protocol that doesn't carry mute state and a session name resolver that reads the wrong pane ID field.

**In scope:** Heartbeat protocol change (carry mute suffix), controller handler restructure (change-gated dirty marking), dump-state response extension (add `focused_pane_id`), relay session name resolution (prefer focused over attended), targeted render on sidebar registration.

**Out of scope:** Voice relay protocol versioning, heartbeat interval/timeout changes, sidebar-side rendering optimizations, relay identity tracking for multiple simultaneous relays.

## Bigger Picture

This spec builds on top of [spec 053 (render pipeline stability)](../053-render-pipeline-stability/spec.md), which eliminated the dual controller root cause for sidebar flickering. With the controller stabilized, these remaining voice-specific symptoms become addressable. The voice relay integration (spec 042) and sidebar integration (spec 045) established the current architecture; this spec refines the protocol without changing the fundamental push-based model.

After this work, the major remaining voice relay issue is the attended/focused pane distinction itself. The relay currently has two signals for "which session is active," and this spec adds a priority ordering rather than unifying them. A future spec may want to collapse `attended_pane_id` and `focused_pane_id` into a single `active_session` concept, but that requires broader changes to sidebar navigation semantics.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts of the spec and plan that need human judgment most. Each section points to specific locations and frames the review as questions.

### Understanding the approach (8 min)

Read [Purpose](spec.md#purpose) and [Functional Requirements](spec.md#functional-requirements) for the core approach. As you read, consider:

- Does the decision to make every heartbeat carry mute state ([FR-001](spec.md#functional-requirements)) create unnecessary chatter on the pipe? The relay already sends `[[voice:on]]` every second, so the payload size increase is minimal (adding `:muted` or `:unmuted`), but is the 1-second resolution appropriate for mute state sync?
- The [change-gating approach in FR-006](spec.md#functional-requirements) compares previous and current state to decide whether to mark dirty. Is there a race condition risk if two state changes happen within the same timer tick?
- The spec [assumes spec 053 is fixed](spec.md#assumptions) (single controller). What happens if a user runs this code without the 053 fix? Would the heartbeat changes behave correctly with dual controllers?

### Key decisions that need your eyes (12 min)

**Heartbeat protocol change** ([Plan Change 1](plan.md#change-1-heartbeat-carries-mute-state-fr-001-fr-006))

The relay switches from bare `[[voice:on]]` to `[[voice:on:muted]]`/`[[voice:on:unmuted]]` on every tick. Backward compatibility is maintained by treating bare `voice:on` as unmuted.
- Does the backward compatibility handling ([Clarification Q1](spec.md#clarifications)) cover all real deployment scenarios? Could an older CLI and newer plugin coexist during a rolling upgrade?

**Session name resolution priority** ([Plan Change 3](plan.md#change-3-relay-prefers-focused_pane_id-for-session-name-fr-004))

The relay now prefers `focused_pane_id` over `attended_pane_id`. The attended pane represents explicit sidebar clicks; the focused pane represents terminal focus (which pane has the cursor).
- Is `focused_pane_id` always the right signal? If the user focuses a non-session pane (e.g., an editor pane, a shell), the relay would fall back to `attended_pane_id`. But if they focus a plugin pane (the sidebar itself), does `focused_pane_id` point to the sidebar? Would this cause an incorrect fallback?

**Targeted render on sidebar-hello** ([Plan Change 4](plan.md#change-4-targeted-render-on-sidebar-registration-fr-005))

New sidebars receive an immediate render payload on registration instead of waiting for the next dirty cycle.
- The targeted render builds a fresh `RenderPayload` per registration. With 14 tabs, could rapid tab creation cause 14 payload serializations in quick succession? Is the performance cost acceptable?

### Areas where I'm less certain (5 min)

- [FR-005 interaction with existing broadcast](spec.md#functional-requirements): The controller already sends a broadcast render when permissions are granted (mod.rs:115-119). If sidebar-hello arrives after this initial broadcast, the targeted render is redundant. If it arrives before, the sidebar already got data from the broadcast. I'm not certain there's actually a gap here, or if the real issue is timing between broadcast and sidebar-hello. The plan assumes the gap exists based on the symptom (indicator disappearing on tab switch), but the root cause could be elsewhere (e.g., sidebar reinitializing state on tab switch events).

- [Clarification Q2 (dual relay)](spec.md#clarifications): The "last heartbeat wins" approach was chosen for simplicity. With two relays having different mute states, the indicator would alternate every second. This is documented as acceptable ("user error"), but I'm not fully certain users won't accidentally start two relays (e.g., via tmux and a separate terminal).

- [T001 API design](tasks.md#phase-1-foundational-blocking-prerequisites): The `targeted_render()` function combines build + send. If in the future we need to send a modified payload (e.g., with a notification for the new sidebar), this API would need changing. Passing a pre-built payload might be more flexible.

### Risks and open questions (5 min)

- If the dump-state response grows with `focused_pane_id` ([FR-003](spec.md#functional-requirements)), does the JSON size increase cause issues with Zellij's pipe message size limits? The field is a single `Option<u32>`, so the impact is negligible, but has anyone verified pipe message size limits for this project?

- The [mute state recovery flow](spec.md#user-story-4---mute-state-survives-recovery-priority-p2) depends on the relay sending `[[voice:on:muted]]` after a 15-second timeout. But during laptop sleep, the relay process is also suspended. Does the relay resume heartbeats immediately on wake, or is there an additional delay? The spec's 2-second target for session name updates ([SC-003](spec.md#measurable-outcomes)) could be affected by wake-up latency.

- The [testing strategy](plan.md#testing-strategy) relies on manual verification for SC-001 through SC-004. Is there an opportunity to add automated regression tests using the existing plugin integration test infrastructure (spec 052)?

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
