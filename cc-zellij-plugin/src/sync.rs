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

/// Serialize sessions once, then broadcast via pipe AND save to disk.
/// Avoids the double serialization of calling broadcast_state + save_sessions separately.
#[cfg(target_family = "wasm")]
pub fn broadcast_and_save(state: &PluginState) {
    use zellij_tile::prelude::*;
    let json = serde_json::to_string(&state.sessions).unwrap_or_default();
    // Broadcast via pipe
    let mut msg = MessageToPlugin::new("cc-deck:sync");
    msg.message_payload = Some(json.clone());
    pipe_message_to_plugin(msg);
    // Save to disk (reusing same JSON)
    let _ = std::fs::write(SESSIONS_PATH, &json);
    let pid = current_zellij_pid();
    if pid != 0 {
        let _ = std::fs::write(PID_PATH, pid.to_string());
    }
}

#[cfg(not(target_family = "wasm"))]
pub fn broadcast_and_save(_state: &PluginState) {
    // No-op in native tests
}

/// Immediate sync: cancel any pending debounce, broadcast and save now.
/// Use this for user-initiated actions (delete, rename, pause) where
/// the state change must be visible to other instances immediately.
pub fn sync_now(state: &mut PluginState) {
    state.sync_dirty = false;
    broadcast_and_save(state);
}

/// Flush debounced sync if dirty: broadcast and save, then clear the flag.
pub fn flush_if_dirty(state: &mut PluginState) {
    if state.sync_dirty {
        sync_now(state);
    }
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
    // Skip save_sessions here: the broadcasting instance already called
    // broadcast_and_save() which persists the authoritative state. Saving
    // on every receiver caused N redundant disk writes per sync, which
    // overwhelmed Zellij's WASM runtime with 5+ instances after restore.
    state.merge_sessions(incoming)
}

// --- Full session state persistence via WASI /cache/ ---

const SESSIONS_PATH: &str = "/cache/sessions.json";
const PID_PATH: &str = "/cache/zellij_pid";

/// Get the Zellij server PID (stable across reattach, different for new sessions).
#[cfg(target_family = "wasm")]
fn current_zellij_pid() -> u32 {
    zellij_tile::prelude::get_plugin_ids().zellij_pid
}

#[cfg(not(target_family = "wasm"))]
fn current_zellij_pid() -> u32 {
    0
}

/// Persist full session state to disk for reattach recovery.
pub fn save_sessions(sessions: &BTreeMap<u32, Session>) {
    if let Ok(json) = serde_json::to_string(sessions) {
        let _ = std::fs::write(SESSIONS_PATH, json);
    }
    // Track which Zellij session owns this cache so we can detect
    // stale caches from a previous session on next startup.
    let pid = current_zellij_pid();
    if pid != 0 {
        let _ = std::fs::write(PID_PATH, pid.to_string());
    }
}

/// Restore sessions from disk (called on load/reattach).
/// Returns an empty map if the cache belongs to a different Zellij session,
/// preventing ghost sessions when pane IDs are reused across sessions.
pub fn restore_sessions() -> BTreeMap<u32, Session> {
    let current_pid = current_zellij_pid();
    if current_pid != 0 {
        let cached_pid = std::fs::read_to_string(PID_PATH)
            .ok()
            .and_then(|s| s.trim().parse::<u32>().ok())
            .unwrap_or(0);
        if cached_pid != 0 && cached_pid != current_pid {
            // Cache belongs to a different Zellij session (stale).
            // Clear it and return empty to avoid ghost sessions from
            // coincidental pane ID collisions.
            let _ = std::fs::remove_file(SESSIONS_PATH);
            let _ = std::fs::remove_file(META_PATH);
            let _ = std::fs::remove_file(PID_PATH);
            return BTreeMap::new();
        }
    }
    match std::fs::read_to_string(SESSIONS_PATH) {
        Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
        Err(_) => BTreeMap::new(),
    }
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
/// Skips parsing if the file content hash matches the last read.
/// Returns true if any session was updated.
pub fn apply_session_meta(sessions: &mut BTreeMap<u32, Session>, last_hash: &mut u64) -> bool {
    let content = match std::fs::read_to_string(META_PATH) {
        Ok(c) => c,
        Err(_) => return false,
    };

    // Quick hash check: skip parse if content unchanged
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    let mut hasher = DefaultHasher::new();
    content.hash(&mut hasher);
    let new_hash = hasher.finish();
    if new_hash == *last_hash {
        return false;
    }
    *last_hash = new_hash;

    let meta: BTreeMap<u32, SessionMeta> = match serde_json::from_str(&content) {
        Ok(m) => m,
        Err(_) => return false,
    };
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

/// Prune session-meta.json entries for pane IDs that no longer have living sessions.
/// Called after remove_dead_sessions() and PaneClosed to prevent stale metadata
/// from being applied to new sessions with reused pane IDs.
pub fn prune_session_meta(live_sessions: &BTreeMap<u32, Session>) {
    let existing = read_session_meta_file();
    if existing.is_empty() {
        return;
    }
    let pruned: BTreeMap<u32, SessionMeta> = existing
        .into_iter()
        .filter(|(id, _)| live_sessions.contains_key(id))
        .collect();
    if pruned.is_empty() {
        let _ = std::fs::remove_file(META_PATH);
    } else if let Ok(json) = serde_json::to_string(&pruned) {
        let _ = std::fs::write(META_PATH, json);
    }
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

        // Stale sync should not trigger re-render
        assert!(!handle_sync(&mut state, &payload));
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

    #[test]
    fn test_sessions_serialize_roundtrip() {
        use crate::session::Activity;

        let mut sessions = BTreeMap::new();
        let mut s1 = make_session(1, "api-server", 100);
        s1.activity = Activity::Working;
        s1.working_dir = Some("/home/user/api".to_string());
        s1.git_branch = Some("main".to_string());
        s1.tab_index = Some(0);
        s1.manually_renamed = true;
        s1.paused = false;
        s1.meta_ts = 50;
        sessions.insert(1, s1);

        let mut s2 = make_session(2, "frontend", 200);
        s2.activity = Activity::Idle;
        s2.paused = true;
        s2.meta_ts = 150;
        s2.done_attended = true;
        sessions.insert(2, s2);

        let json = serde_json::to_string(&sessions).unwrap();
        let restored: BTreeMap<u32, Session> = serde_json::from_str(&json).unwrap();

        assert_eq!(restored.len(), 2);
        let r1 = &restored[&1];
        assert_eq!(r1.display_name, "api-server");
        assert_eq!(r1.activity, Activity::Working);
        assert_eq!(r1.working_dir.as_deref(), Some("/home/user/api"));
        assert_eq!(r1.git_branch.as_deref(), Some("main"));
        assert_eq!(r1.tab_index, Some(0));
        assert!(r1.manually_renamed);
        assert!(!r1.paused);
        assert_eq!(r1.meta_ts, 50);

        let r2 = &restored[&2];
        assert_eq!(r2.display_name, "frontend");
        assert_eq!(r2.activity, Activity::Idle);
        assert!(r2.paused);
        assert!(r2.done_attended);
    }

    #[test]
    fn test_prune_session_meta() {
        // Simulate prune logic: filter meta entries to only living sessions
        let mut meta = BTreeMap::new();
        meta.insert(1, SessionMeta {
            display_name: "alive".to_string(),
            manually_renamed: true,
            paused: false,
            meta_ts: 100,
        });
        meta.insert(2, SessionMeta {
            display_name: "dead".to_string(),
            manually_renamed: true,
            paused: false,
            meta_ts: 100,
        });
        meta.insert(3, SessionMeta {
            display_name: "also-alive".to_string(),
            manually_renamed: false,
            paused: true,
            meta_ts: 200,
        });

        let mut live_sessions = BTreeMap::new();
        live_sessions.insert(1, make_session(1, "alive", 100));
        live_sessions.insert(3, make_session(3, "also-alive", 200));

        // Prune: keep only entries for living sessions
        let pruned: BTreeMap<u32, SessionMeta> = meta
            .into_iter()
            .filter(|(id, _)| live_sessions.contains_key(id))
            .collect();

        assert_eq!(pruned.len(), 2);
        assert!(pruned.contains_key(&1));
        assert!(!pruned.contains_key(&2));
        assert!(pruned.contains_key(&3));
    }

    #[test]
    fn test_restore_invalid_json() {
        let result: BTreeMap<u32, Session> =
            serde_json::from_str("not valid json").unwrap_or_default();
        assert!(result.is_empty());
    }

    #[test]
    fn test_meta_hash_skip_logic() {
        // Verify that identical content hashes match and different content hashes differ.
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        let content_a = r#"{"1":{"display_name":"api","manually_renamed":true,"paused":false,"meta_ts":100}}"#;
        let content_b = r#"{"1":{"display_name":"renamed","manually_renamed":true,"paused":false,"meta_ts":200}}"#;

        let hash = |s: &str| -> u64 {
            let mut hasher = DefaultHasher::new();
            s.hash(&mut hasher);
            hasher.finish()
        };

        let hash_a = hash(content_a);
        let hash_a2 = hash(content_a);
        let hash_b = hash(content_b);

        // Same content produces same hash (should skip)
        assert_eq!(hash_a, hash_a2);
        // Different content produces different hash (should parse)
        assert_ne!(hash_a, hash_b);
    }

    #[test]
    fn test_broadcast_and_save_json_roundtrip() {
        // Verify the JSON produced by serializing sessions is valid and roundtrips.
        // This validates the shared serialization path used by broadcast_and_save.
        let mut sessions = BTreeMap::new();
        let mut s1 = make_session(1, "api", 100);
        s1.activity = crate::session::Activity::Working;
        s1.working_dir = Some("/home/user/api".to_string());
        sessions.insert(1, s1);
        sessions.insert(2, make_session(2, "web", 200));

        let json = serde_json::to_string(&sessions).unwrap();
        let restored: BTreeMap<u32, Session> = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.len(), 2);
        assert_eq!(restored[&1].display_name, "api");
        assert_eq!(restored[&2].display_name, "web");
        assert_eq!(restored[&1].activity, crate::session::Activity::Working);
    }
}
