// T008: Multi-instance state synchronization via pipe messages

use crate::session::Session;
use crate::state::PluginState;
use std::collections::BTreeMap;

/// Broadcast current session state to all plugin instances.
#[cfg(target_family = "wasm")]
pub fn broadcast_state(state: &PluginState) {
    use zellij_tile::prelude::*;
    let payload = serde_json::to_string(&state.sessions).unwrap_or_default();
    let mut msg = MessageToPlugin::new("cc-deck:sync");
    msg.message_payload = Some(payload);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
pub fn broadcast_state(_state: &PluginState) {
    // No-op in native tests
}

/// Request state from other plugin instances (called on load).
#[cfg(target_family = "wasm")]
pub fn request_state() {
    use zellij_tile::prelude::*;
    pipe_message_to_plugin(MessageToPlugin::new("cc-deck:request"));
}

#[cfg(not(target_family = "wasm"))]
pub fn request_state() {
    // No-op in native tests
}

/// Handle incoming sync payload: merge sessions from another instance.
/// Returns true if state changed (needs re-render).
pub fn handle_sync(state: &mut PluginState, payload: &str) -> bool {
    let incoming: BTreeMap<u32, Session> = match serde_json::from_str(payload) {
        Ok(s) => s,
        Err(_) => return false,
    };
    if incoming.is_empty() {
        return false;
    }
    let before_count = state.sessions.len();
    state.merge_sessions(incoming);
    // Always re-render after sync (state may have updated even if count unchanged)
    state.sessions.len() != before_count || true
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;

    fn make_session(pane_id: u32, name: &str, ts: u64) -> Session {
        let mut s = Session::new(pane_id, format!("session-{pane_id}"));
        s.display_name = name.to_string();
        s.last_event_ts = ts;
        s
    }

    #[test]
    fn test_handle_sync_empty_payload() {
        let mut state = PluginState::default();
        assert!(!handle_sync(&mut state, "{}"));
        assert!(!handle_sync(&mut state, "not json"));
    }

    #[test]
    fn test_handle_sync_merges_new_sessions() {
        let mut state = PluginState::default();
        let mut incoming = BTreeMap::new();
        incoming.insert(1, make_session(1, "api", 100));
        incoming.insert(2, make_session(2, "web", 200));
        let payload = serde_json::to_string(&incoming).unwrap();

        assert!(handle_sync(&mut state, &payload));
        assert_eq!(state.sessions.len(), 2);
        assert_eq!(state.sessions[&1].display_name, "api");
    }

    #[test]
    fn test_handle_sync_newer_wins() {
        let mut state = PluginState::default();
        state.sessions.insert(1, make_session(1, "old-name", 100));

        let mut incoming = BTreeMap::new();
        incoming.insert(1, make_session(1, "new-name", 200));
        let payload = serde_json::to_string(&incoming).unwrap();

        handle_sync(&mut state, &payload);
        assert_eq!(state.sessions[&1].display_name, "new-name");
    }

    #[test]
    fn test_handle_sync_older_ignored() {
        let mut state = PluginState::default();
        state.sessions.insert(1, make_session(1, "current", 200));

        let mut incoming = BTreeMap::new();
        incoming.insert(1, make_session(1, "stale", 100));
        let payload = serde_json::to_string(&incoming).unwrap();

        handle_sync(&mut state, &payload);
        assert_eq!(state.sessions[&1].display_name, "current");
    }
}
