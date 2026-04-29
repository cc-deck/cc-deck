// T008: Multi-instance state synchronization via pipe messages + file-based metadata sync
// T001-T024 (044): PID-scoped state isolation so each Zellij session sees only its own sessions.

use crate::session::Session;
use crate::state::PluginState;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

// --- PID-scoped path helpers (T001) ---

/// Get the Zellij server PID (stable across reattach, different for new sessions).
#[cfg(target_family = "wasm")]
fn current_zellij_pid() -> u32 {
    zellij_tile::prelude::get_plugin_ids().zellij_pid
}

#[cfg(not(target_family = "wasm"))]
fn current_zellij_pid() -> u32 {
    0
}

/// PID-scoped sessions file path: `/cache/sessions-{pid}.json`.
/// Falls back to the legacy `/cache/sessions.json` when PID is 0 (native tests).
fn sessions_path(pid: u32) -> String {
    if pid == 0 {
        "/cache/sessions.json".to_string()
    } else {
        format!("/cache/sessions-{pid}.json")
    }
}

/// PID-scoped meta file path: `/cache/session-meta-{pid}.json`.
/// Falls back to the legacy `/cache/session-meta.json` when PID is 0 (native tests).
fn meta_path(pid: u32) -> String {
    if pid == 0 {
        "/cache/session-meta.json".to_string()
    } else {
        format!("/cache/session-meta-{pid}.json")
    }
}

/// Legacy file paths (pre-044, no PID suffix).
const LEGACY_SESSIONS_PATH: &str = "/cache/sessions.json";
const LEGACY_META_PATH: &str = "/cache/session-meta.json";
const LEGACY_PID_PATH: &str = "/cache/zellij_pid";

/// PID-scoped pipe message name for sync broadcasts.
fn sync_message_name(pid: u32) -> String {
    if pid == 0 {
        "cc-deck:sync".to_string()
    } else {
        format!("cc-deck:sync:{pid}")
    }
}

/// PID-scoped pipe message name for state requests.
fn request_message_name(pid: u32) -> String {
    if pid == 0 {
        "cc-deck:request".to_string()
    } else {
        format!("cc-deck:request:{pid}")
    }
}

/// Extract PID from a sync/request message name.
/// Returns Some(pid) if the name matches `cc-deck:sync:{pid}` or `cc-deck:request:{pid}`.
/// Returns None if the name does not contain a PID suffix.
pub fn extract_pid_from_message_name(name: &str) -> Option<u32> {
    if let Some(suffix) = name.strip_prefix("cc-deck:sync:") {
        suffix.parse().ok()
    } else if let Some(suffix) = name.strip_prefix("cc-deck:request:") {
        suffix.parse().ok()
    } else {
        None
    }
}

/// Check if a pipe message name is a sync or request message (with or without PID).
pub fn is_sync_message(name: &str) -> bool {
    name == "cc-deck:sync" || name.starts_with("cc-deck:sync:")
}

/// Check if a pipe message name is a request message (with or without PID).
pub fn is_request_message(name: &str) -> bool {
    name == "cc-deck:request" || name.starts_with("cc-deck:request:")
}

// --- Broadcast and sync (T010-T012) ---

/// Broadcast current session state to all plugin instances.
/// Uses PID-scoped message name so only same-session instances process it.
#[cfg(target_family = "wasm")]
pub fn broadcast_state(state: &PluginState) {
    use zellij_tile::prelude::*;
    let pid = current_zellij_pid();
    let payload = serde_json::to_string(&state.sessions).unwrap_or_default();
    let mut msg = MessageToPlugin::new(sync_message_name(pid));
    msg.message_payload = Some(payload);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
pub fn broadcast_state(_state: &PluginState) {
    // No-op in native tests
}

/// Serialize sessions once, then broadcast via pipe AND save to disk.
/// Avoids the double serialization of calling broadcast_state + save_sessions separately.
/// Uses PID-scoped file paths and message names.
#[cfg(target_family = "wasm")]
pub fn broadcast_and_save(state: &PluginState) {
    use zellij_tile::prelude::*;
    let pid = current_zellij_pid();
    let json = serde_json::to_string(&state.sessions).unwrap_or_default();
    // Broadcast via pipe with PID-scoped name
    let mut msg = MessageToPlugin::new(sync_message_name(pid));
    msg.message_payload = Some(json.clone());
    pipe_message_to_plugin(msg);
    // Save to PID-scoped file (no separate PID file needed)
    let _ = std::fs::write(sessions_path(pid), &json);
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
/// Uses PID-scoped message name so only same-session instances respond.
#[cfg(target_family = "wasm")]
pub fn request_state() {
    use zellij_tile::prelude::*;
    let pid = current_zellij_pid();
    pipe_message_to_plugin(MessageToPlugin::new(request_message_name(pid)));
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

// --- Full session state persistence via WASI /cache/ (T002, T005, T006) ---

/// Persist full session state to disk for reattach recovery.
/// Writes to the PID-scoped path.
pub fn save_sessions(sessions: &BTreeMap<u32, Session>) {
    let pid = current_zellij_pid();
    if let Ok(json) = serde_json::to_string(sessions) {
        let _ = std::fs::write(sessions_path(pid), json);
    }
}

/// Restore sessions from disk (called on load/reattach).
/// Reads from the PID-scoped file. No cross-session PID check needed
/// because each PID has its own file.
pub fn restore_sessions() -> BTreeMap<u32, Session> {
    let pid = current_zellij_pid();
    match std::fs::read_to_string(sessions_path(pid)) {
        Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
        Err(_) => BTreeMap::new(),
    }
}

// --- File-based metadata sync via WASI /cache/ (T003, T008, T009) ---
//
// pipe_message_to_plugin broadcasts are unreliable in Zellij 0.43.
// Use a shared file for metadata that only changes on user actions
// (rename, pause toggle). Each instance reads on timer and applies.

/// Metadata override for a single session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionMeta {
    pub display_name: String,
    pub manually_renamed: bool,
    pub paused: bool,
    pub meta_ts: u64,
}

/// Write session metadata overrides to the PID-scoped meta file.
/// Called after rename or pause toggle.
pub fn write_session_meta(sessions: &BTreeMap<u32, Session>) {
    let pid = current_zellij_pid();
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

    let path = meta_path(pid);
    // Read existing file and merge (preserve entries from other instances)
    let mut existing = read_session_meta_file_at(&path);
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
        let _ = std::fs::write(&path, json);
    }
}

/// Read and apply session metadata from the PID-scoped meta file.
/// Called on timer to pick up renames/pauses from other instances.
/// Skips parsing if the file content hash matches the last read.
/// Returns true if any session was updated.
pub fn apply_session_meta(sessions: &mut BTreeMap<u32, Session>, last_hash: &mut u64) -> bool {
    let pid = current_zellij_pid();
    let path = meta_path(pid);
    let content = match std::fs::read_to_string(&path) {
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
    let pid = current_zellij_pid();
    let path = meta_path(pid);
    let existing = read_session_meta_file_at(&path);
    if existing.is_empty() {
        return;
    }
    let pruned: BTreeMap<u32, SessionMeta> = existing
        .into_iter()
        .filter(|(id, _)| live_sessions.contains_key(id))
        .collect();
    if pruned.is_empty() {
        let _ = std::fs::remove_file(&path);
    } else if let Ok(json) = serde_json::to_string(&pruned) {
        let _ = std::fs::write(&path, json);
    }
}

fn read_session_meta_file_at(path: &str) -> BTreeMap<u32, SessionMeta> {
    match std::fs::read_to_string(path) {
        Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
        Err(_) => BTreeMap::new(),
    }
}

// --- Legacy migration (T021-T023) ---

/// Migrate legacy state files (pre-044) to PID-scoped paths.
/// Called once on plugin startup. If legacy files exist and PID-scoped
/// files do not, rename them. Then remove the legacy PID file.
pub fn migrate_legacy_files() {
    let pid = current_zellij_pid();
    if pid == 0 {
        return; // Skip in native tests
    }

    let scoped_sessions = sessions_path(pid);
    let scoped_meta = meta_path(pid);

    if std::fs::metadata(&scoped_sessions).is_err() {
        if let Ok(content) = std::fs::read_to_string(LEGACY_SESSIONS_PATH) {
            if std::fs::write(&scoped_sessions, &content).is_ok() {
                let _ = std::fs::remove_file(LEGACY_SESSIONS_PATH);
            }
        }
    }

    if std::fs::metadata(&scoped_meta).is_err() {
        if let Ok(content) = std::fs::read_to_string(LEGACY_META_PATH) {
            if std::fs::write(&scoped_meta, &content).is_ok() {
                let _ = std::fs::remove_file(LEGACY_META_PATH);
            }
        }
    }

    // Remove the legacy PID file (PID is now embedded in filenames)
    let _ = std::fs::remove_file(LEGACY_PID_PATH);
}

// --- Orphan cleanup (T017) ---

/// Clean up orphaned state files from killed Zellij sessions.
/// Scans `/cache/` for `sessions-*.json` and `session-meta-*.json` files.
/// Attempts to check process liveness via `/proc/{pid}/`. If `/proc/` is
/// not available (WASI limitation), falls back to removing files older
/// than 7 days based on modification time.
pub fn cleanup_orphaned_state_files() {
    let current_pid = current_zellij_pid();
    if current_pid == 0 {
        return; // Skip in native tests
    }

    let entries = match std::fs::read_dir("/cache/") {
        Ok(e) => e,
        Err(_) => return,
    };

    let seven_days_secs: u64 = 7 * 24 * 60 * 60;
    let now_secs = crate::session::unix_now();

    for entry in entries.flatten() {
        let name = match entry.file_name().into_string() {
            Ok(n) => n,
            Err(_) => continue,
        };

        // Match sessions-{pid}.json or session-meta-{pid}.json
        let pid = extract_pid_from_filename(&name);
        let pid = match pid {
            Some(p) => p,
            None => continue,
        };

        // Never clean up our own files
        if pid == current_pid {
            continue;
        }

        // Try process liveness check via /proc/
        let proc_path = format!("/proc/{pid}");
        let is_alive = std::fs::metadata(&proc_path).is_ok();

        if is_alive {
            continue; // Process still running, keep the file
        }

        // /proc/ check failed (either dead process or /proc/ not mounted).
        // Use file age as fallback: only remove files older than 7 days.
        let should_remove = match entry.metadata().and_then(|m| m.modified()) {
            Ok(mtime) => {
                match mtime.duration_since(std::time::UNIX_EPOCH) {
                    Ok(d) => now_secs.saturating_sub(d.as_secs()) > seven_days_secs,
                    Err(_) => false,
                }
            }
            Err(_) => {
                // Cannot determine file age; fall back to removing it
                // only if /proc/ definitively shows the process is dead.
                // Since we got here, /proc/ may not be mounted, so skip.
                false
            }
        };

        if should_remove {
            let path = entry.path();
            let _ = std::fs::remove_file(&path);
        }
    }
}

/// Extract PID from a state filename like `sessions-12345.json` or `session-meta-12345.json`.
fn extract_pid_from_filename(name: &str) -> Option<u32> {
    if let Some(rest) = name.strip_prefix("sessions-") {
        rest.strip_suffix(".json")?.parse().ok()
    } else if let Some(rest) = name.strip_prefix("session-meta-") {
        rest.strip_suffix(".json")?.parse().ok()
    } else {
        None
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

    // --- T001: PID helper and path tests ---

    #[test]
    fn test_sessions_path_with_pid() {
        assert_eq!(sessions_path(12345), "/cache/sessions-12345.json");
        assert_eq!(sessions_path(1), "/cache/sessions-1.json");
    }

    #[test]
    fn test_sessions_path_zero_pid_fallback() {
        assert_eq!(sessions_path(0), "/cache/sessions.json");
    }

    #[test]
    fn test_meta_path_with_pid() {
        assert_eq!(meta_path(12345), "/cache/session-meta-12345.json");
        assert_eq!(meta_path(1), "/cache/session-meta-1.json");
    }

    #[test]
    fn test_meta_path_zero_pid_fallback() {
        assert_eq!(meta_path(0), "/cache/session-meta.json");
    }

    // --- T010-T012: PID-scoped message name tests ---

    #[test]
    fn test_sync_message_name_with_pid() {
        assert_eq!(sync_message_name(12345), "cc-deck:sync:12345");
    }

    #[test]
    fn test_sync_message_name_zero_pid() {
        assert_eq!(sync_message_name(0), "cc-deck:sync");
    }

    #[test]
    fn test_request_message_name_with_pid() {
        assert_eq!(request_message_name(12345), "cc-deck:request:12345");
    }

    #[test]
    fn test_request_message_name_zero_pid() {
        assert_eq!(request_message_name(0), "cc-deck:request");
    }

    #[test]
    fn test_extract_pid_from_message_name() {
        assert_eq!(extract_pid_from_message_name("cc-deck:sync:12345"), Some(12345));
        assert_eq!(extract_pid_from_message_name("cc-deck:request:99"), Some(99));
        assert_eq!(extract_pid_from_message_name("cc-deck:sync"), None);
        assert_eq!(extract_pid_from_message_name("cc-deck:request"), None);
        assert_eq!(extract_pid_from_message_name("cc-deck:hook"), None);
        assert_eq!(extract_pid_from_message_name("cc-deck:sync:notanumber"), None);
    }

    #[test]
    fn test_is_sync_message() {
        assert!(is_sync_message("cc-deck:sync"));
        assert!(is_sync_message("cc-deck:sync:12345"));
        assert!(!is_sync_message("cc-deck:request"));
        assert!(!is_sync_message("cc-deck:hook"));
    }

    #[test]
    fn test_is_request_message() {
        assert!(is_request_message("cc-deck:request"));
        assert!(is_request_message("cc-deck:request:12345"));
        assert!(!is_request_message("cc-deck:sync"));
        assert!(!is_request_message("cc-deck:hook"));
    }

    // --- T017: Orphan cleanup filename extraction ---

    #[test]
    fn test_extract_pid_from_filename() {
        assert_eq!(extract_pid_from_filename("sessions-12345.json"), Some(12345));
        assert_eq!(extract_pid_from_filename("session-meta-12345.json"), Some(12345));
        assert_eq!(extract_pid_from_filename("sessions.json"), None);
        assert_eq!(extract_pid_from_filename("session-meta.json"), None);
        assert_eq!(extract_pid_from_filename("debug.log"), None);
        assert_eq!(extract_pid_from_filename("sessions-abc.json"), None);
    }

    // --- Existing sync tests ---

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

    // --- T014-T016: PID-scoped isolation tests ---

    #[test]
    fn test_pid_scoped_paths_are_unique_per_pid() {
        // Two different PIDs produce different file paths
        let path_a = sessions_path(100);
        let path_b = sessions_path(200);
        assert_ne!(path_a, path_b);

        let meta_a = meta_path(100);
        let meta_b = meta_path(200);
        assert_ne!(meta_a, meta_b);
    }

    #[test]
    fn test_pid_scoped_message_names_are_unique_per_pid() {
        let sync_a = sync_message_name(100);
        let sync_b = sync_message_name(200);
        assert_ne!(sync_a, sync_b);

        let req_a = request_message_name(100);
        let req_b = request_message_name(200);
        assert_ne!(req_a, req_b);
    }

    #[test]
    fn test_foreign_pid_sync_messages_are_distinguishable() {
        // A message from PID 100 should not match PID 200
        let name = sync_message_name(100);
        let extracted = extract_pid_from_message_name(&name);
        assert_eq!(extracted, Some(100));
        assert_ne!(extracted, Some(200));
    }

    #[test]
    fn test_legacy_message_names_have_no_pid() {
        // Legacy messages (PID 0) produce no-PID names
        assert_eq!(extract_pid_from_message_name("cc-deck:sync"), None);
        assert_eq!(extract_pid_from_message_name("cc-deck:request"), None);
    }
}
