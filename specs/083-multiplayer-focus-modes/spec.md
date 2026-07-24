# Feature Specification: Multiplayer Focus Modes

**Feature Branch**: `083-multiplayer-focus-modes`
**Created**: 2026-07-21
**Revised**: 2026-07-22
**Status**: Draft
**Input**: Brainstorm 088 - Multiplayer Focus Modes
**Prerequisite**: Spec 082 - Multiplayer Plugin Resilience (client_id architecture)

## Summary

Fix the multiplayer focus bug where sidebar clicks on a secondary client control the primary client's terminal, and add presence indicators showing which sessions other clients are focused on.

The design leverages the fact that Zellij's plugin API is inherently per-client: `focus_terminal_pane()` and `switch_tab_to()` only affect the calling plugin's associated client. The bug exists because the sidebar routes Switch actions through the controller (which runs on client 1) instead of handling them locally.

## Scope

**In scope:**
- Independent focus handling (sidebar calls API locally)
- Stable auto-sort zones (focus never changes ordering)
- Presence indicators (right-aligned colored blocks on session lines)
- Theme-aware colors via `ModeUpdate` subscription
- Focus reporting to controller for activity tracking

**Out of scope (deferred):**
- Synchronized mode (pair programming, follow-the-leader)
- Per-client keybinding routing (blocked on Zellij PR #4094)
- Read-only client detection (Zellij doesn't expose this yet)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Independent Focus Control (Priority: P1)

A user shares their Zellij session with a colleague. Each person clicks sessions in their own sidebar to navigate between active coding sessions. Each click switches focus only in the clicking user's terminal, without affecting the other user's view.

**Why this priority**: This fixes the immediate bug where client 2's sidebar clicks control client 1's terminal. Without this fix, the second user's sidebar is actively harmful (it disrupts the primary user).

**Independent Test**: Start a session with cc-deck. Attach a second terminal. Click a session in the second client's sidebar. Verify only the second client's terminal switches to that session. Verify the first client's terminal is unchanged.

**Acceptance Scenarios**:

1. **Given** two clients are attached to the same Zellij session, **When** client 2 clicks a session in its sidebar, **Then** only client 2's terminal pane switches to the clicked session. Client 1's terminal is unaffected.
2. **Given** a single-client session, **When** the user clicks a session in the sidebar, **Then** focus switches exactly as before (no regression).
3. **Given** client 2 clicks a session, **When** the sidebar sends a focus-report to the controller, **Then** the controller updates its activity tracking (last-active timestamp, activity state) for the switched session.
4. **Given** client 2 uses keyboard navigation (Alt-s, j/k, Enter) in the sidebar, **When** Enter is pressed to select a session, **Then** only client 2's terminal switches focus.

---

### User Story 2 - Presence Indicators (Priority: P1)

When multiple clients are connected, each session line in the sidebar shows right-aligned colored blocks indicating which other clients are currently focused on that session. The colors match Zellij's multiplayer user colors (the same palette used in the tab bar).

**Why this priority**: In a shared session, users need spatial awareness of where other collaborators are working. Without indicators, there is no way to know which session a colleague is looking at without verbal coordination.

**Independent Test**: Attach two clients to a session. Have client 2 click on a session. Verify client 1's sidebar shows a colored block on that session line. Have client 2 switch to a different session. Verify the indicator moves.

**Acceptance Scenarios**:

1. **Given** two clients are connected, **When** client 2 focuses on session "cc-deck", **Then** client 1's sidebar shows a colored block (using client 2's multiplayer color) right-aligned on the "cc-deck" session line.
2. **Given** client 2 switches from "cc-deck" to "agent-eval-harness", **When** client 1's sidebar re-renders, **Then** the indicator moves from "cc-deck" to "agent-eval-harness".
3. **Given** three clients are connected and clients 2 and 3 both focus on the same session, **When** client 1's sidebar renders, **Then** two colored blocks appear stacked from the right on that session line.
4. **Given** a single-client session, **When** the sidebar renders, **Then** no presence indicators are shown.
5. **Given** the session name is long and indicators are present, **When** the sidebar renders, **Then** the session name truncates to make room for the indicators (indicators take priority).

---

### User Story 3 - Stable Auto-Sort Zones (Priority: P2)

All clients see stable active and paused zones. Focus changes do not affect ordering. Pausing removes a session from the active zone; reactivation appends it to that zone.

**Why this priority**: Reordering on every click destroys spatial memory and makes the active zone difficult to operate reliably.

**Independent Test**: Attach two clients and click active sessions in arbitrary order. Verify neither sidebar reorders. Pause and reactivate one session; verify it returns at the end of the active zone.

**Acceptance Scenarios**:

1. **Given** active sessions ordered [A, B, C], **When** either client clicks C then B, **Then** both sidebars retain [A, B, C].
2. **Given** active sessions [A, B, C], **When** B becomes paused, **Then** the active zone becomes [A, C].
3. **Given** B is paused, **When** B is reactivated, **Then** the active zone becomes [A, C, B].

---

### Edge Cases

- What happens when one client disconnects? The controller removes that client's entry from `client_focus` on the next `SessionUpdate` that shows a reduced client count. Presence indicators for the disconnected client disappear on the next render.
- What happens with zombie sidebars from disconnected clients? The controller ignores focus-reports from `client_id` values not in the current `connected_clients` set.
- What happens when a session name is so long that indicators overlap the icon? The session name is truncated, not the indicators. Maximum 5 indicators (covers up to 5 other clients).
- What happens before `ModeUpdate` arrives (no palette yet)? Presence indicators are not shown. The sidebar renders normally without indicators until the palette is available.
- What happens when the same client focuses a session via both click and keyboard? Both paths produce the same focus-report, so the state is consistent.

## Requirements *(mandatory)*

### Functional Requirements

**Independent Focus Handling:**
- **FR-001**: The sidebar MUST call `focus_terminal_pane()` and `switch_tab_to()` directly from its own plugin instance instead of routing Switch actions through the controller
- **FR-002**: After a local focus switch, the sidebar MUST send a `cc-deck:focus-report` pipe message to the controller containing `{ client_id, session_name }` for activity tracking and presence
- **FR-003**: The controller MUST continue updating its internal session tracking (last-active timestamp, activity state) based on received focus-reports

**Stable Auto-Sort Zones:**
- **FR-004**: Clicking or focusing a session MUST NOT change its position within the active zone
- **FR-005**: Activity changes within the active zone MUST NOT change session order
- **FR-006**: A paused session MUST leave the active zone, and a reactivated session MUST be appended to the active zone

**Presence Indicators:**
- **FR-007**: The controller MUST maintain a `client_focus: HashMap<u16, String>` mapping each client's `client_id` to the session name they last focused
- **FR-008**: The render payload MUST include `other_client_focus: Vec<(String, Vec<(u16, PaletteColor)>)>` with entries for all clients except the render target's client
- **FR-009**: The sidebar MUST render presence indicators as right-aligned colored blocks (inverted spaces) on session lines that have other-client focus
- **FR-010**: Presence indicators MUST use the `multiplayer_user_colors` from the Zellij palette (client N uses `player_(N % 10)` color, wrapping for client_ids exceeding 10)
- **FR-011**: Maximum 5 presence indicators per session line; blocks are separated by one space and stack from the right edge
- **FR-012**: Presence indicators MUST NOT be shown when only one client is connected
- **FR-013**: If the session name is too long to accommodate indicators, the name MUST truncate (indicators take rendering priority)

**Theme Integration:**
- **FR-014**: The controller MUST subscribe to `ModeUpdate` events and extract `multiplayer_user_colors` from `ModeInfo.style.colors`
- **FR-015**: The controller MUST store the palette and include relevant color values in the render payload
- **FR-016**: If `ModeUpdate` has not been received yet, presence indicators MUST NOT be rendered (graceful degradation, no hardcoded fallback)

**Backward Compatibility:**
- **FR-017**: Single-client sessions MUST behave identically to the current implementation (zero regressions)
- **FR-018**: Keyboard shortcuts (alt-s, alt-a, alt-w, shift variants, voice toggle) MUST continue to work for the primary client (controller's client)

### Known Limitations

- **KL-001**: Keyboard shortcuts only affect the primary client in multiplayer sessions. Other clients use mouse clicks for session navigation. This is due to Zellij pipe messages not carrying `client_id` (blocked on Zellij PR #4094).
- **KL-002**: Read-only client detection is not possible. All clients are treated equally for presence indicators. Read-only status is not exposed by `list_clients()`.
- **KL-003**: Stacked pane expansion is a global operation in Zellij (issue #3326). When one client changes which stacked pane is expanded, all clients on that tab have their focus moved. This is a Zellij bug, not a cc-deck issue.

### Key Entities

- **FocusReport**: Pipe message from sidebar to controller: `{ client_id: u16, session_name: String }`
- **ClientFocus**: Controller state tracking which session each client is focused on: `HashMap<u16, String>`
- **LocalActivationOrder**: Per-sidebar state tracking this client's session navigation history: `Vec<u32>` (pane IDs)
- **MultiplayerColors**: Palette data from `ModeUpdate` containing 10 player colors

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing tests pass without modification (zero regressions)
- **SC-002**: In a two-client session, clicking a session on client 2 does not change client 1's focus (verified by observation)
- **SC-003**: In a two-client session, client 1's sidebar shows a colored indicator on the session that client 2 is focused on, using client 2's multiplayer color
- **SC-004**: Switching sessions on client 2 moves the indicator on client 1's sidebar within one render cycle
- **SC-005**: In a two-client session, switching sessions on client 2 does not reorder client 1's sidebar
- **SC-006**: Single-client session performance is identical to before (no measurable overhead)
- **SC-007**: Keyboard shortcuts (alt-s, alt-a) work correctly for the primary client in multiplayer

## Documentation Requirements

- Update README.md with multiplayer focus behavior and presence indicators
- Document the keyboard shortcut limitation in multiplayer sessions
- Add or update Antora guide page for multiplayer session usage (covers independent focus and presence indicators)
- No CLI reference changes (no new commands or flags)
- No configuration reference changes (no new config options)

## Clarifications

### Session 2026-07-22

No critical ambiguities detected. All functional requirements are testable and unambiguous. The spec was revised from the original (which included synchronized mode) to focus solely on independent focus handling and presence indicators, based on research showing that Zellij's plugin API is inherently per-client.

**Deferred to implementation**: Color wrapping for `client_id > 10` (modulo 10 over the 10 available multiplayer colors). Low impact, implementation detail.

## Assumptions

- The multiplayer resilience feature (spec 082) is shipped, providing `client_id` tracking in the controller and sidebar registry
- `focus_terminal_pane()` called from a sidebar plugin on `client_id=2` correctly focuses the pane for client 2 (validated by Zellij plugin API documentation: actions implicitly target the owning client)
- `switch_tab_to()` called from a sidebar plugin similarly targets the owning client
- The controller can broadcast messages to specific sidebars by `plugin_id` (already implemented via `send_render_to_plugin`)
- `ModeUpdate` events are delivered to plugins that subscribe to them and contain the full `Style` with `multiplayer_user_colors`
- The `connected_clients` count from `SessionInfo` (via `SessionUpdate` events) is sufficient for detecting client connect/disconnect

## Error Handling

- If `focus_terminal_pane()` fails silently when called from a non-primary client (empirical risk), fall back to routing through the controller (current behavior) and log a warning
- If a focus-report arrives from a `client_id` not in the current `connected_clients` set (zombie), the controller ignores it
- If `ModeUpdate` is never received (edge case with older Zellij versions), presence indicators are simply not shown
- If the render payload's `other_client_focus` data is stale (client disconnected between render cycles), the sidebar renders stale indicators for one cycle until the next render removes them

## Research Findings

### Zellij Plugin API (per-client behavior)

Plugins are associated with a specific client via `get_plugin_ids().client_id`. API actions (`focus_terminal_pane`, `switch_tab_to`) implicitly act on the owning client. No `client_id` parameter is needed. This was confirmed by the `group_and_ungroup_panes()` function which has an explicit `for_all_clients: bool` parameter, demonstrating that per-client vs global is a deliberate distinction in the API.

### Zellij Pipe Message Limitation

Pipe messages (from `MessagePlugin` keybindings) do not include `client_id`. Zellij PR #4094 would add this, but it remains unmerged with requested changes. This blocks per-client keybinding routing. The workaround is to document that keyboard shortcuts work for the primary client only.

### Zellij Theme/Color API

`ModeUpdate` events carry `ModeInfo.style.colors` which includes `multiplayer_user_colors` with 10 player colors (`player_1` through `player_10`), each as `PaletteColor::Rgb((u8, u8, u8))` or `PaletteColor::EightBit(u8)`. These are the same colors Zellij uses in the tab bar for client indicators.

### SyncTab Clarification

`TabInfo.is_sync_panes_active` controls keystroke broadcasting to all panes in a tab (like tmux's `synchronize-panes`). It has nothing to do with multi-client tab synchronization and is not relevant to this feature.
