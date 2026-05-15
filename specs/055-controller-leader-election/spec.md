# Feature Specification: Controller Leader Election

**Feature Branch**: `055-controller-leader-election`
**Created**: 2026-05-14
**Status**: Draft
**Input**: Brainstorm session investigating navigation mode chaos and voice indicator flicker, both caused by duplicate WASM controller instances

## Purpose

Fix two critical regressions caused by the unresolved dual-controller Zellij bug (duplicate WASM instances with plugin_id 0 and 4):

1. **Navigation mode chaos**: A single keypress produces multiple navigate broadcasts from competing controllers, causing the sidebar cursor to race through all sessions uncontrollably.
2. **Voice indicator flicker**: The `broadcast_render_all` safety net was removed to work around dual-controller interference, but this left sidebars without a fallback for missed targeted renders.

The fix adds controller leader election via the existing ping/pong pipe protocol, so only one controller is active even when Zellij loads duplicates. With a single active controller, `broadcast_render_all` can be safely restored.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stable Navigation Mode (Priority: P1)

A developer presses Alt+s to enter navigation mode. The sidebar shows an amber cursor on the next session. Each subsequent Alt+s press moves the cursor down by exactly one position. The cursor never jumps, skips, or races through sessions.

**Why this priority**: Navigation mode is completely broken with dual controllers. Every keypress causes the cursor to cycle through all 12 sessions within a second, making the feature unusable.

**Independent Test**: Press Alt+s once. Observe that exactly one session is highlighted with an amber cursor. Press Alt+s again. The cursor moves to the next session. Press Escape. The cursor disappears and the sidebar returns to passive mode.

**Acceptance Scenarios**:

1. **Given** the sidebar is in passive mode, **When** I press Alt+s, **Then** exactly one session is highlighted with an amber cursor and the cursor does not move on its own
2. **Given** the sidebar is in navigate mode with cursor at position N, **When** I press Alt+s, **Then** the cursor moves to position N+1 (wrapping to 0 at the end)
3. **Given** Zellij has loaded two controller instances, **When** I press Alt+s, **Then** only one controller forwards the navigate command and only one sidebar enters navigate mode
4. **Given** the sidebar is in navigate mode, **When** I press Escape, **Then** the sidebar returns to passive mode and focus returns to the terminal pane

---

### User Story 2 - Stable Voice Indicator (Priority: P1)

A developer running voice relay sees a stable green note in the sidebar header. The note remains visible during session switches, tab switches, and all other user actions without flickering or momentary disappearance.

**Why this priority**: The green note is the primary visual feedback for voice relay status. Flickering makes the indicator untrustworthy and distracting.

**Independent Test**: Start voice relay for a workspace. Click through five different sessions in rapid succession. The green note must never disappear during the switches.

**Acceptance Scenarios**:

1. **Given** voice relay is connected, **When** I click a different session in the sidebar, **Then** the green note remains visible throughout the transition
2. **Given** voice relay is connected, **When** I switch Zellij tabs, **Then** the green note remains visible on the new tab's sidebar immediately
3. **Given** voice relay is connected, **When** a sidebar is not yet registered in the sidebar registry, **Then** the sidebar still receives the render payload via the untargeted broadcast fallback and shows the green note

---

### User Story 3 - Transparent Leader Election (Priority: P2)

The leader election happens automatically on startup with no user intervention. The developer is unaware that multiple controller instances exist. All controller features (keybindings, renders, hooks) work as if there were only one controller.

**Why this priority**: Leader election is the mechanism, not a user-facing feature. It must be invisible.

**Independent Test**: Start cc-deck in a new Zellij session. Verify via debug log that one controller becomes leader and the other goes dormant. All keybindings (Alt+s, Alt+a, Alt+w) function correctly.

**Acceptance Scenarios**:

1. **Given** Zellij loads two controller instances, **When** both instances complete startup, **Then** the instance with the lower plugin_id becomes leader within 2 seconds
2. **Given** one controller is leader and another is dormant, **When** the leader crashes or is unloaded, **Then** the dormant controller re-activates within 60 seconds
3. **Given** a dormant controller exists, **When** it receives pipe messages (hooks, navigate, attend), **Then** it ignores all of them except controller-ping/pong

---

### Edge Cases

- What happens when both controllers have the same plugin_id? This should not happen (Zellij assigns unique IDs), but if it does, the first to broadcast a ping wins (timestamp tiebreaker).
- What happens when a third controller instance loads? It participates in the same election; highest plugin_id goes dormant.
- What happens when the leader re-broadcasts its heartbeat ping and the dormant controller receives it? The dormant controller confirms the leader is still alive and resets its re-activation timer.
- What happens during the election window (before election completes)? All controllers start dormant (no event processing, no keybinding registration). The winner activates after the 2-second probe window. During this window, keybindings and renders are inactive, which is acceptable because Zellij's own UI is typically still initializing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: On startup (after permissions granted), each controller MUST broadcast a `cc-deck:controller-ping` message containing its `plugin_id`
- **FR-002**: When a controller receives a ping from another instance, the instance with the higher `plugin_id` MUST transition to dormant state
- **FR-003**: A dormant controller MUST NOT register keybindings, broadcast renders, process hook events, or forward navigate/attend/working commands
- **FR-004**: A dormant controller MUST continue to listen for `cc-deck:controller-ping` messages to maintain its dormant state and detect leader failure
- **FR-005**: The active (leader) controller MUST re-broadcast a ping every 30 seconds as a heartbeat, so late-loading instances discover the leader and go dormant
- **FR-006**: If the leader's heartbeat stops (no ping received for 60 seconds), a dormant controller MUST re-activate and begin a new election round
- **FR-007**: `broadcast_render` MUST be restored to include the untargeted `broadcast_render_all` fallback alongside targeted sidebar sends, now that only one controller broadcasts
- **FR-008**: A dormant controller MUST ignore all pipe messages except `cc-deck:controller-ping` and `cc-deck:controller-pong` (early return, no processing)
- **FR-008a**: A dormant controller MUST skip all event processing (`TabUpdate`, `PaneUpdate`, `Timer`) via early return in `update()`. Only `PermissionRequestResult` is processed to ensure permissions are granted for future re-activation.
- **FR-009**: Only the leader controller MUST register keybindings via `reconfigure()`. A dormant controller MUST skip keybinding registration entirely.
- **FR-010**: On startup, each controller MUST begin in dormant state (pessimistic default). It MUST broadcast a ping and wait up to 2 seconds for a response. If no ping from a lower-ID instance arrives within 2 seconds, the controller activates as leader.

### Key Entities

- **ControllerState.is_leader**: Boolean flag indicating whether this controller instance is the active leader (true) or dormant (false). Defaults to false on startup (pessimistic). The controller activates only after a 2-second probe window passes with no ping from a lower-ID instance, or immediately upon winning the election.
- **ControllerState.leader_plugin_id**: The plugin_id of the known leader. Set when a ping is received from a lower-ID instance.
- **ControllerState.last_leader_ping_ms**: Timestamp of the last received leader ping. Used for the 60-second re-activation timeout.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Pressing Alt+s in navigation mode moves the cursor by exactly one position per keypress, with zero unwanted cursor jumps
- **SC-002**: The voice indicator (green note) remains visually stable during 60 seconds of idle observation and during 5 rapid session switches, with zero flicker events
- **SC-003**: Leader election completes within 2 seconds of the second controller instance loading
- **SC-004**: No increase in steady-state pipe message volume compared to a single-controller baseline (the 30-second heartbeat adds at most 2 messages per minute)
- **SC-005**: The sidebar registry contains the correct number of sidebars (one per tab, not inflated by duplicate controller discovery)

## Error Handling

- If the leader controller crashes without clean shutdown, the dormant controller detects the missing heartbeat after 60 seconds and self-promotes to leader. During the 60-second window, no keybindings or render broadcasts are active from the dormant instance.
- If a `cc-deck:controller-ping` message arrives with an unrecognized format, the controller ignores it (backward compatibility with future protocol changes).
- If the dormant controller receives a `cc-deck:controller-pong` response to a ping it did not send (stale message from a previous election), it ignores the response.

## Out of Scope

- Fixing the upstream Zellij bug that creates duplicate WASM instances (tracked in brainstorm doc `zellij-load-plugins-duplicate-instance.md`)
- Changing the existing heartbeat/timeout values for voice relay (15-second timeout, 1-second interval)
- Sidebar-side rendering optimizations beyond restoring `broadcast_render_all`
- Cleaning up stale sidebar registry entries (separate concern, may be addressed in a future spec)

## Clarifications

### Session 2026-05-14

- Q: Should controllers default to leader (optimistic) or dormant (pessimistic) on startup? -> A: Pessimistic. Start dormant, activate after 2-second probe timeout if no lower-ID ping received. Eliminates the dual-active window entirely.
- Q: Should dormant controllers continue processing TabUpdate/PaneUpdate events to stay fresh for failover? -> A: No. Skip all event processing when dormant (early return in update()). On re-activation, the controller rebuilds state from the next TabUpdate/PaneUpdate cycle.

## Assumptions

- The Zellij bug creates at most 2 controller instances (the original and one duplicate). The election protocol supports N instances but is optimized for N=2.
- `plugin_id` values are unique across all loaded plugin instances within a Zellij session.
- The `PipeSource::Plugin(id)` field in received pipe messages correctly identifies the sender's `plugin_id`.
- The existing `ControllerPing` and `ControllerPong` pipe actions in `pipe_handler.rs` can be reused for the election protocol.
- The 30-second heartbeat interval and 60-second re-activation timeout provide sufficient responsiveness for the leader-failure scenario without adding meaningful overhead.
- `broadcast_render_all` (untargeted broadcast) is safe to restore because only one controller will be active at any time.
