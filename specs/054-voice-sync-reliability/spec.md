# Feature Specification: Voice Sync Reliability

**Feature Branch**: `054-voice-sync-reliability`
**Created**: 2026-05-14
**Status**: Draft
**Input**: Brainstorm session and user discussion about voice indicator sync issues

## Purpose

Fix four voice sync reliability issues that cause the voice indicator (green note) to flicker, disappear on session switch, show wrong session names in the voice relay TUI, and lose mute state after recovery. The root cause is that the voice heartbeat protocol carries insufficient state, and the relay's session tracking uses a stale signal (`last_attended_pane_id` instead of `focused_pane_id`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stable Voice Indicator (Priority: P1)

A developer running voice relay sees a stable green note in the sidebar header. The note appears when voice relay connects and stays visible without flickering until voice relay stops. The note never blinks, disappears momentarily, or shows incorrect mute state during normal operation.

**Why this priority**: The green note is the primary visual feedback for voice relay status. Flickering makes the indicator untrustworthy and distracting.

**Independent Test**: Start voice relay for a workspace. Observe the sidebar for 60 seconds with no interaction. The green note must remain stable with no visual changes.

**Acceptance Scenarios**:

1. **Given** voice relay is running and connected, **When** no user interaction occurs for 60 seconds, **Then** the green note remains steadily visible in the sidebar header with no flickering
2. **Given** voice relay is running and muted, **When** no user interaction occurs for 30 seconds, **Then** the dim note remains steadily visible with no flickering
3. **Given** voice relay connects, **When** the sidebar receives the first render payload, **Then** the green note appears and stays visible from that point forward
4. **Given** voice relay disconnects (process stops), **When** the heartbeat timeout expires (15 seconds), **Then** the green note disappears exactly once and stays gone

---

### User Story 2 - Stable Indicator During Session Switch (Priority: P1)

A developer clicks a different session in the sidebar or switches Zellij tabs. The green note remains visible throughout the switch with no momentary disappearance.

**Why this priority**: Session switching is a frequent action. The note disappearing on every switch undermines confidence in the voice relay connection.

**Independent Test**: Start voice relay. Click through five different sessions in rapid succession. The green note must never disappear during the switches.

**Acceptance Scenarios**:

1. **Given** voice relay is connected and green note is visible, **When** I click a different session in the sidebar, **Then** the green note remains visible throughout the transition
2. **Given** voice relay is connected, **When** I switch Zellij tabs rapidly (5 tabs in 3 seconds), **Then** the green note remains visible on every tab's sidebar
3. **Given** voice relay is connected, **When** a new sidebar registers after tab switch, **Then** the new sidebar shows the green note immediately (within 1 render cycle)

---

### User Story 3 - Accurate Session Name in Voice Relay (Priority: P2)

A developer switches focus between Claude Code sessions. The voice relay TUI updates to show the name of the currently focused session within 2 seconds.

**Why this priority**: The session name tells the developer where dictated text will be injected. A stale name causes confusion about which session receives input.

**Independent Test**: Start voice relay. Focus on session "api-server". The relay TUI shows "api-server". Switch focus to session "frontend". Within 2 seconds, the relay TUI shows "frontend".

**Acceptance Scenarios**:

1. **Given** voice relay is connected and showing session "A", **When** I click session "B" in the sidebar, **Then** the relay TUI shows "B" within 2 seconds
2. **Given** voice relay is connected, **When** I switch Zellij tabs to a tab with a different session, **Then** the relay TUI updates to show that session's name within 2 seconds
3. **Given** multiple sessions exist but none is explicitly attended, **When** I focus a pane that is a tracked session, **Then** the relay TUI shows that session's name

---

### User Story 4 - Mute State Survives Recovery (Priority: P2)

A developer mutes voice relay. If the plugin's voice state times out and recovers (e.g., after laptop sleep/wake), the mute state is preserved. The developer does not need to re-mute after recovery.

**Why this priority**: Losing mute state after sleep/wake is a usability annoyance that requires manual intervention.

**Independent Test**: Mute voice relay. Simulate a heartbeat timeout (wait 15+ seconds without heartbeat). Resume heartbeat. The relay should reconnect as muted, and the sidebar should show the dim note.

**Acceptance Scenarios**:

1. **Given** voice relay is muted, **When** the plugin's voice state times out and the relay reconnects, **Then** the sidebar shows a dim note (muted state preserved)
2. **Given** voice relay is muted, **When** the relay sends its heartbeat ping, **Then** the ping carries the current mute state
3. **Given** voice relay is unmuted, **When** the plugin's voice state times out and the relay reconnects, **Then** the sidebar shows a bright green note (unmuted state preserved)

---

### Edge Cases

- What happens when two voice relay instances connect simultaneously? Last heartbeat wins with no relay identity tracking. This converges naturally when one relay stops.
- What happens when the sidebar receives a render payload before voice relay has connected? The note should be absent (not flickering).
- What happens when a session is closed while focused? The relay should fall back to the next available session or clear the displayed name.
- What happens when all sessions are closed? The relay should show no target session name.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The voice relay MUST send `[[voice:on:muted]]` or `[[voice:on:unmuted]]` on every heartbeat tick, reflecting the relay's current mute state
- **FR-002**: The controller MUST update `voice_muted` from the heartbeat suffix on every ping and mark render dirty only when `voice_enabled` or `voice_muted` actually changes compared to the previous state
- **FR-003**: The dump-state response MUST include `focused_pane_id` alongside `attended_pane_id`
- **FR-004**: The voice relay MUST prefer `focused_pane_id` for session name resolution, falling back to `attended_pane_id` when `focused_pane_id` does not correspond to a tracked session
- **FR-005**: When a sidebar registers via sidebar-hello, the controller MUST immediately send it the current render payload (including voice state)
- **FR-006**: The `voice:on` handler MUST NOT call `mark_render_dirty()` unless voice state (`voice_enabled` or `voice_muted`) actually changed
- **FR-007**: The voice relay MUST continue to send heartbeat pings every 1 second (existing behavior, unchanged)
- **FR-008**: The controller MUST continue to timeout voice state after 15 seconds of no heartbeat (existing behavior, unchanged)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The voice indicator (green note) remains visually stable for 60 seconds of idle observation with zero flicker events
- **SC-002**: Switching between 5 sessions in rapid succession produces zero momentary disappearances of the voice indicator
- **SC-003**: The voice relay TUI updates the displayed session name within 2 seconds of focusing a different session
- **SC-004**: After a heartbeat timeout and recovery, the mute state matches the relay's actual mute state (no manual re-mute needed)
- **SC-005**: No increase in render broadcast frequency at steady state compared to current behavior (net reduction expected due to change-gated dirty marking)

## Error Handling

- If the voice relay crashes without clean disconnect, the controller continues reporting voice as enabled until the 15-second heartbeat timeout expires. Newly registered sidebars receive this state immediately (brief false positive is acceptable).
- If the controller receives a heartbeat with an unrecognized mute suffix, it treats the relay as unmuted (consistent with bare `[[voice:on]]` backward compatibility).
- If `focused_pane_id` is absent from the dump-state response, the relay falls back to `attended_pane_id` for session name resolution (FR-004).

## Out of Scope

- Voice relay protocol version negotiation or relay identity tracking
- Changing the 15-second heartbeat timeout value
- Changing the 1-second heartbeat interval
- Sidebar-side rendering optimizations for voice indicator display

## Clarifications

### Session 2026-05-14

- Q: How should the controller handle an unrecognized mute suffix on the heartbeat? → A: Treat as unmuted (same as missing suffix)
- Q: When a second voice relay connects while one is already active, how should the controller resolve the conflict? → A: Last heartbeat wins, no relay identity tracking. Two simultaneous relays is a user error; last-write-wins converges when one stops.
- Q: Should a newly registered sidebar receive voice state even if it may be stale (within the 15s timeout window)? → A: Yes, send current state immediately. Brief false positive (up to 15s) is better than a false negative.
- Q: What should the initial value of `voice_muted` be before the first heartbeat? → A: Default `false` (unmuted). Before a relay connects, `voice_enabled` is false so the muted flag is irrelevant. First heartbeat sets actual state without unnecessary dirty mark.

## Assumptions

- `voice_muted` defaults to `false` before the first heartbeat arrives (irrelevant while `voice_enabled` is `false`)
- The dual controller issue (spec 053) has been fixed and only one controller instance processes voice commands
- The 1-second polling interval for dump-state is acceptable latency for session name updates (2-second target allows for one missed poll)
- The existing 15-second heartbeat timeout is appropriate and does not need adjustment
- Zellij's pipe message delivery is reliable for targeted (plugin-id-specific) messages
- An older relay sending bare `[[voice:on]]` (without mute suffix) is treated as unmuted for backward compatibility
- Any unrecognized mute suffix (e.g., `[[voice:on:garbage]]`) is treated as unmuted, consistent with the bare `[[voice:on]]` fallback
