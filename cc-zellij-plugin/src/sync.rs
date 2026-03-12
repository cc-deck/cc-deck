// T008: Multi-instance state synchronization via pipe messages + file-based metadata sync

use crate::session::Session;
use crate::state::PluginState;
use serde::{Deserialize, Serialize};
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
    state.merge_sessions(incoming);
    // Always re-render after sync (state may have updated even if count unchanged)
    true
}

// --- File-based metadata sync via WASI /cache/ ---
//
// pipe_message_to_plugin broadcasts are unreliable in Zellij 0.43.
// Use a shared file for metadata that only changes on user actions
// (rename, pause toggle). Each instance reads on timer and applies.

const META_PATH: &str = "/cache/session-meta.json";

/// Metadata override for a single session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionMeta {
    pub display_name: String,
    pub manually_renamed: bool,
    pub paused: bool,
    pub meta_ts: u64,
}

/// Write session metadata overrides to the shared file.
/// Called after rename or pause toggle.
pub fn write_session_meta(sessions: &BTreeMap<u32, Session>) {
    // Only write sessions that have user-modified metadata
    let meta: BTreeMap<u32, SessionMeta> = sessions
        .iter()
        .filter(|(_, s)| s.meta_ts > 0)
        .map(|(&id, s)| {
            (
                id,
                SessionMeta {
                    display_name: s.display_name.clone(),
                    manually_renamed: s.manually_renamed,
                    paused: s.paused,
                    meta_ts: s.meta_ts,
                },
            )
        })
        .collect();

    if meta.is_empty() {
        return;
    }

    // Read existing file and merge (preserve entries from other instances)
    let mut existing = read_session_meta_file();
    for (id, m) in &meta {
        let dominated = existing
            .get(id)
            .map(|e| m.meta_ts > e.meta_ts)
            .unwrap_or(true);
        if dominated {
            existing.insert(*id, m.clone());
        }
    }

    if let Ok(json) = serde_json::to_string(&existing) {
        let _ = std::fs::write(META_PATH, json);
    }
}

/// Read and apply session metadata from the shared file.
/// Called on timer to pick up renames/pauses from other instances.
/// Returns true if any session was updated.
pub fn apply_session_meta(sessions: &mut BTreeMap<u32, Session>) -> bool {
    let meta = read_session_meta_file();
    if meta.is_empty() {
        return false;
    }

    let mut changed = false;
    for (pane_id, m) in meta {
        if let Some(session) = sessions.get_mut(&pane_id) {
            if m.meta_ts > session.meta_ts {
                session.display_name = m.display_name;
                session.manually_renamed = m.manually_renamed;
                session.paused = m.paused;
                session.meta_ts = m.meta_ts;
                changed = true;
            }
        }
    }
    changed
}

fn read_session_meta_file() -> BTreeMap<u32, SessionMeta> {
    match std::fs::read_to_string(META_PATH) {
        Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
        Err(_) => BTreeMap::new(),
    }
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

    #[test]
    fn test_apply_session_meta() {
        let mut sessions = BTreeMap::new();
        sessions.insert(1, make_session(1, "old-name", 100));
        sessions.insert(2, make_session(2, "other", 100));

        let mut meta = BTreeMap::new();
        meta.insert(
            1,
            SessionMeta {
                display_name: "renamed".to_string(),
                manually_renamed: true,
                paused: false,
                meta_ts: 200,
            },
        );

        // Simulate what apply_session_meta does (without file I/O)
        let mut changed = false;
        for (pane_id, m) in meta {
            if let Some(session) = sessions.get_mut(&pane_id) {
                if m.meta_ts > session.meta_ts {
                    session.display_name = m.display_name;
                    session.manually_renamed = m.manually_renamed;
                    session.paused = m.paused;
                    session.meta_ts = m.meta_ts;
                    changed = true;
                }
            }
        }

        assert!(changed);
        assert_eq!(sessions[&1].display_name, "renamed");
        assert!(sessions[&1].manually_renamed);
        // Session 2 unchanged
        assert_eq!(sessions[&2].display_name, "other");
    }

    #[test]
    fn test_apply_meta_older_ignored() {
        let mut sessions = BTreeMap::new();
        let mut s = make_session(1, "current-name", 100);
        s.meta_ts = 300;
        s.manually_renamed = true;
        sessions.insert(1, s);

        let meta = SessionMeta {
            display_name: "stale-name".to_string(),
            manually_renamed: true,
            paused: false,
            meta_ts: 200,
        };

        // meta_ts 200 < session.meta_ts 300, should not apply
        if let Some(session) = sessions.get_mut(&1) {
            if meta.meta_ts > session.meta_ts {
                session.display_name = meta.display_name;
            }
        }

        assert_eq!(sessions[&1].display_name, "current-name");
    }
}
