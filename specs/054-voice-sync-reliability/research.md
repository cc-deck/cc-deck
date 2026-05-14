# Research: Voice Sync Reliability

## R-001: Voice heartbeat handler behavior

**Decision**: Restructure `handle_voice_command()` to parse mute suffix on every heartbeat, not just on first enable.

**Rationale**: The current code (mod.rs:490-496) only sets `voice_muted` inside `if !was_enabled`, meaning subsequent heartbeats never update mute state. This causes mute state loss after timeout/recovery: when the relay reconnects after a 15-second timeout, the first `voice:on` re-enables voice but uses whatever mute default the controller has (false), ignoring the relay's actual mute state.

**Alternatives considered**:
- Separate `voice:mute-sync` message on reconnect: Rejected because the heartbeat already fires every second and adding another message type increases protocol complexity.
- Stateful mute tracking in the relay with explicit re-sync: Rejected because the heartbeat suffix approach is simpler and self-healing (every tick carries current state).

## R-002: dump-state response field addition

**Decision**: Add `focused_pane_id` to `DumpStateResponse` alongside existing `attended_pane_id`.

**Rationale**: The relay currently uses `attended_pane_id` for session name resolution, but this only updates on explicit "attend" actions from the sidebar. `focused_pane_id` updates on tab switches and pane focus changes, which better reflects where the user is actually working.

**Alternatives considered**:
- Replace `attended_pane_id` with `focused_pane_id`: Rejected because `attended_pane_id` represents intentional user navigation (clicking a session in the sidebar) and may differ from terminal focus (which could be a non-session pane). Both signals are valuable.
- Add a combined `active_session_id` field: Rejected because the resolution logic belongs in the relay (consumer), not the controller (provider). The controller should expose raw state.

## R-003: Sidebar-hello targeted render delivery

**Decision**: Send a targeted render payload to newly registered sidebars in `handle_sidebar_hello()`.

**Rationale**: The current flow has a gap: sidebars send hello AFTER receiving their first broadcast render, but if a new tab opens between render broadcasts, the new sidebar's hello response (sidebar-init) doesn't include session/voice data. The sidebar shows stale cached data until the next dirty render. Sending a targeted render on registration closes this gap.

**Alternatives considered**:
- Mark render dirty on sidebar-hello: Rejected because this broadcasts to ALL sidebars unnecessarily. Only the new sidebar needs the payload.
- Sidebar requests render via a new message type: Rejected because the controller already has all the state needed to push proactively.

## R-004: Render broadcast visibility for send_render_to_plugin

**Decision**: Add a `targeted_render()` public function in `render_broadcast.rs` that builds and sends a render payload to a single plugin_id.

**Rationale**: `send_render_to_plugin()` is currently a WASM-gated private helper. Rather than exposing internals, a clean public API that combines build + send is more maintainable. The sidebar_registry module can call this without knowing about serialization details.

**Alternatives considered**:
- Make `send_render_to_plugin()` public: Rejected because it only sends pre-serialized JSON, requiring the caller to handle serialization. The combined function is a better API boundary.
- Move render logic into sidebar_registry: Rejected because render broadcasting is a separate concern from sidebar discovery.
