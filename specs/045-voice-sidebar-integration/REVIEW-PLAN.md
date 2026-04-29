# Review Guide: Voice Sidebar Integration

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-04-29

---

## What This Spec Does

Adds voice relay visibility and control to the cc-deck sidebar plugin. When a developer runs voice dictation, the sidebar shows a ♫ indicator (bright when listening, dim when muted). The developer can mute/unmute from anywhere in the Zellij session via a keyboard shortcut, without switching to the voice terminal. Under the hood, this replaces raw text injection with a structured `[[command]]` protocol and removes the push-to-talk mode in favor of the simpler mute toggle.

**In scope:** Sidebar ♫ indicator, mute toggle (3 input paths: keybinding, nav mode key, click), `[[command]]` protocol for voice pipe, heartbeat for crash detection, PTT removal, voice_key config option.

**Out of scope:** Permission-state detection (deferred from 042), multilingual model support, concurrent voice relay to multiple workspaces, `--device` flag for audio input selection.

## Bigger Picture

This spec is the third in the voice relay sequence. [042-voice-relay](../../brainstorm/042-voice-relay.md) established the core audio-to-text-to-agent pipeline. [044-sidebar-session-isolation](../044-sidebar-session-isolation/) moved the plugin to a single-instance controller architecture. This spec (045) bridges those two by integrating voice state into the controller/sidebar broadcast system.

The `[[command]]` protocol introduced here is a stepping stone. Future specs may add more commands (e.g., `[[voice:pause-for-permission]]` when FR-007 from 042 is eventually implemented). The protocol's extensibility is a deliberate design choice, but the current command set is deliberately small (6 commands + plain text).

The mute toggle replaces PTT (push-to-talk), which required a long-poll pipe and a separate F8 keybinding. Mute is architecturally simpler because it piggybacks on the existing dump-state polling loop rather than holding a pipe open.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts of the spec and plan
> that need human judgment most. Each section points to specific locations and
> frames the review as questions.

### Understanding the approach (8 min)

Read [User Story 4 - Command Protocol](spec.md#user-story-4---command-protocol-priority-p2) and the [voice-command-protocol contract](contracts/voice-command-protocol.md) for the core protocol design. As you read, consider:

- Is the `[[command]]` syntax the right choice for separating control signals from dictation text? The spec assumes Whisper never produces `[[` in transcription output. Is that assumption safe enough, or should there be an escape mechanism?
- The protocol is intentionally one-directional (CLI to plugin) with the backchannel via dump-state polling. Does this asymmetry create any subtle issues for future extensibility?

### Key decisions that need your eyes (12 min)

**Mute backchannel via dump-state polling** ([Clarifications](spec.md#clarifications), [FR-012](spec.md#functional-requirements))

The sidebar-initiated mute toggle is communicated back to the voice CLI by adding a `voice_mute_requested` field to the existing dump-state response. The CLI polls this every 1 second. This means sidebar-to-CLI mute has up to 1 second latency, while CLI-to-sidebar mute is near-instant.

- Question for reviewer: Is 1 second worst-case latency for sidebar-initiated mute acceptable? Users pressing Alt+v will see the ♫ dim immediately (sidebar renders locally), but the actual audio muting happens up to 1 second later. Could this cause confusion where the user thinks they are muted but a final utterance still gets transcribed?

**Heartbeat for crash detection** ([FR-019](spec.md#functional-requirements), [Edge Cases](spec.md#edge-cases))

The voice CLI sends `[[voice:ping]]` every 5 seconds. The plugin clears voice state after 3 missed pings (15 seconds). This was chosen over a simple timeout because extended silence is indistinguishable from a crash when using a timeout.

- Question for reviewer: Is 15 seconds the right window? Too short risks false positives during system load spikes. Too long means a stale ♫ indicator lingers after a crash. The 5-second ping interval means at least one pipe call every 5 seconds regardless of dictation activity.

**PTT removal before mute addition** ([Phase 1 in tasks.md](tasks.md#phase-1-setup))

The plan removes PTT in Phase 1 before adding the mute toggle, even though PTT removal is User Story 5 (P3 priority). This is a sequencing decision to avoid conflicting code paths, since both PTT and mute control the same underlying behavior (pause audio processing).

- Question for reviewer: Is this reordering justified? It means PTT users lose functionality immediately rather than having a migration period. Are there any PTT users who would be affected?

**Click region for ♫** ([T019](tasks.md#implementation-for-user-story-1), [T027](tasks.md#implementation-for-user-story-2))

The ♫ indicator uses a sentinel pane_id (`u32::MAX - 2`) for click detection, following the existing pattern where the header uses `u32::MAX - 1`. This works but adds another magic constant.

- Question for reviewer: Is the sentinel pane_id pattern maintainable as more clickable elements are added to the header? Should there be an enum or constant set instead of raw `u32::MAX - N` values?

### Areas where I'm less certain (5 min)

- [FR-012](spec.md#functional-requirements): The `voice_mute_requested` field is `Option<bool>` in the dump-state response. The clear-on-acknowledge pattern (CLI sends `[[voice:mute]]` which clears the request) could have a race condition: if the user toggles mute twice quickly, the second toggle might be lost if the CLI hasn't polled yet. I believe the flag approach handles this correctly (each toggle flips the desired state), but the interaction between rapid toggles and the 1-second poll interval deserves scrutiny.

- [T037](tasks.md#implementation-for-user-story-4): The task says ANSI sanitization should apply to plain text but NOT to `[[command]]` messages. This is correct (commands are internal, not user-supplied), but the implementation boundary where sanitization happens vs. command detection needs care. If sanitization runs first, it could corrupt `[[` brackets. The ordering matters.

- The spec mentions `v` key in navigation mode ([FR-005](spec.md#functional-requirements)) but doesn't address what happens if a developer has a session whose display name starts with "v", since navigation mode already uses letter keys for searching. Is there a conflict?

### Risks and open questions (5 min)

- If the voice CLI crashes and restarts within the 15-second heartbeat window ([FR-019](spec.md#functional-requirements)), it sends a new `[[voice:on]]`. Does the plugin correctly handle receiving `[[voice:on]]` while `voice_enabled` is already true? The spec says it resets state, but the edge case of mute state during reconnection needs verification.

- The `[[` prefix detection assumes exact matching. What happens with malformed messages like `[[voice:on` (missing closing `]]`) or `[[ voice:on ]]` (with spaces)? The [contract](contracts/voice-command-protocol.md) specifies strict matching, but the relay code is the only sender, so this may be academic.

- The dump-state response is growing (sessions + attended_pane_id + voice_mute_requested). At what point does the 1-second polling frequency become a performance concern? Currently likely fine, but worth noting as more state gets added to this response over time.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
