// Controller state: authoritative session store for the single-instance architecture.
//
// The controller is the sole writer of session state. Sidebars receive
// pre-computed RenderPayload via pipe and send ActionMessages back.
// This eliminates the N-instance sync protocol (cc-deck:sync, cc-deck:request)
// and the file-based metadata sync (session-meta.json).

use crate::config::PluginConfig;
use crate::perf::PerfTracker;
use crate::session::{Activity, Session};
use std::collections::{BTreeMap, HashMap, HashSet};
use zellij_tile::prelude::*;

/// PID-scoped sessions file path: `/cache/sessions-{pid}.json`.
/// Falls back to the legacy `/cache/sessions.json` when PID is 0 (native tests).
fn sessions_path(pid: u32) -> String {
    if pid == 0 {
        "/cache/sessions.json".to_string()
    } else {
        format!("/cache/sessions-{pid}.json")
    }
}

/// Legacy file paths (pre-044, no PID suffix).
const LEGACY_SESSIONS_PATH: &str = "/cache/sessions.json";
const LEGACY_META_PATH: &str = "/cache/session-meta.json";
const LEGACY_PID_PATH: &str = "/cache/zellij_pid";

/// TTL for in-flight focus protection. After this period, manifest-derived
/// focus is trusted again even if it differs from the action-set value.
const IN_FLIGHT_FOCUS_TTL_MS: u64 = 3000;

/// Metadata override to apply when a restored session is discovered via CWD matching.
#[derive(Debug, Clone)]
pub struct PendingOverride {
    pub display_name: String,
    pub paused: bool,
}

/// The authoritative state held by the controller plugin instance.
#[derive(Default)]
pub struct ControllerState {
    /// All known Claude sessions, keyed by pane_id. Single writer.
    pub sessions: BTreeMap<u32, Session>,
    /// Current tab list from TabUpdate events.
    pub tabs: Vec<TabInfo>,
    /// Current pane manifest from PaneUpdate events.
    pub pane_manifest: Option<PaneManifest>,
    /// Pane ID -> (tab_index, tab_name) mapping derived from manifest + tabs.
    pub pane_to_tab: HashMap<u32, (usize, String)>,
    /// Currently focused tab position.
    pub active_tab_index: Option<usize>,
    /// Currently focused terminal pane ID.
    pub focused_pane_id: Option<u32>,
    /// Registered sidebar instances: plugin_id -> tab_index.
    pub sidebar_registry: HashMap<u32, usize>,
    /// This controller's plugin ID (set after permissions granted).
    pub plugin_id: u32,
    /// Whether plugin permissions have been granted.
    pub permissions_granted: bool,
    /// Whether the render payload needs to be broadcast on the next timer tick.
    pub render_dirty: bool,
    /// Millisecond timestamp until which `remove_dead_sessions()` is skipped.
    pub startup_grace_until: Option<u64>,
    /// Pending metadata overrides from snapshot restore, keyed by working directory.
    pub pending_overrides: HashMap<String, Vec<PendingOverride>>,
    /// Configuration parsed from KDL layout.
    pub config: PluginConfig,
    /// Whether keybindings have been registered via reconfigure().
    pub keybindings_registered: bool,
    /// Tab count from last TabUpdate. Used to detect tab closures.
    pub last_tab_count: usize,
    /// Pane IDs with in-flight git branch detection commands.
    pub pending_git_branch: HashSet<u32>,
    /// Timestamp (ms) of the last timer-driven git branch poll.
    pub last_git_poll_ms: u64,
    /// Performance instrumentation tracker.
    pub perf: PerfTracker,
    /// Last pane_id that attend switched to, for round-robin cycling.
    pub last_attended_pane_id: Option<u32>,
    /// Timestamp (ms) of the last attend action, for rapid-cycle detection.
    pub last_attend_ms: u64,
    /// Pane IDs already visited during the current rapid-cycle sequence.
    pub attend_visited: HashSet<u32>,
    /// Whether voice relay is currently connected.
    pub voice_enabled: bool,
    /// Whether voice relay is currently muted.
    pub voice_muted: bool,
    /// Timestamp (ms) of last voice ping or voice:on message.
    pub voice_last_ping_ms: u64,
    /// Pending mute toggle from sidebar: Some(true) = mute, Some(false) = unmute.
    pub voice_mute_requested: Option<bool>,
    /// Events received before permissions were granted.
    pub pending_events: Vec<Event>,
    /// Monotonic tick counter for render coalescing.
    pub tick_count: u64,
    /// In-flight focus set by action handlers (Switch, Navigate, Attend).
    /// Protects focused_pane_id from being overwritten by stale manifest data
    /// in rebuild_pane_map() until Zellij confirms the focus change.
    /// Format: (target_pane_id, timestamp_ms). Expires after IN_FLIGHT_FOCUS_TTL_MS.
    pub in_flight_focus: Option<(u32, u64)>,
}


impl ControllerState {
    /// Rebuild the pane-to-tab mapping from current tab and pane data.
    /// Derives focused_pane_id from the manifest, but respects in-flight
    /// focus set by action handlers to avoid stale manifest overwrites.
    pub fn rebuild_pane_map(&mut self) {
        self.pane_to_tab.clear();
        let mut manifest_focus: Option<u32> = None;
        if self.tabs.is_empty() {
            self.focused_pane_id = None;
            return;
        }
        if let Some(ref manifest) = self.pane_manifest {
            for tab in &self.tabs {
                if let Some(panes) = manifest.panes.get(&tab.position) {
                    for pane in panes {
                        if !pane.is_plugin {
                            self.pane_to_tab
                                .insert(pane.id, (tab.position, tab.name.clone()));
                            if pane.is_focused && tab.active {
                                manifest_focus = Some(pane.id);
                            }
                        }
                    }
                }
            }
        }

        // If an action recently set focus, protect it from stale manifests.
        // Once the manifest confirms the target, clear the in-flight guard.
        let now_ms = crate::session::unix_now_ms();
        if let Some((target, ts)) = self.in_flight_focus {
            let age_ms = now_ms.saturating_sub(ts);
            if age_ms > IN_FLIGHT_FOCUS_TTL_MS {
                // Expired: trust the manifest
                crate::debug_log(&format!(
                    "CTRL REBUILD: in_flight EXPIRED target={target} age={age_ms}ms manifest={manifest_focus:?}"
                ));
                self.in_flight_focus = None;
                self.focused_pane_id = manifest_focus;
            } else if manifest_focus == Some(target) {
                // Manifest confirmed the focus change
                crate::debug_log(&format!(
                    "CTRL REBUILD: in_flight CONFIRMED target={target} age={age_ms}ms"
                ));
                self.in_flight_focus = None;
                self.focused_pane_id = manifest_focus;
            } else {
                // Manifest is stale: keep the action-set focus
                crate::debug_log(&format!(
                    "CTRL REBUILD: in_flight STALE target={target} age={age_ms}ms manifest={manifest_focus:?}"
                ));
                self.focused_pane_id = Some(target);
            }
        } else {
            // Only update focus when manifest provides a definite value.
            // When manifest_focus is None (e.g., a plugin pane has focus during
            // navigation mode), preserve the current focused_pane_id. This
            // prevents sidebars from caching None and causing a highlight flash
            // when switching tabs (the target sidebar would render with no
            // highlight until the next broadcast arrives).
            if manifest_focus.is_some() {
                self.focused_pane_id = manifest_focus;
            }
        }
        // Refresh tab info on all sessions and process deferred tab renames.
        let mut pending_renames: Vec<(usize, String)> = Vec::new();
        for session in self.sessions.values_mut() {
            if let Some((idx, name)) = self.pane_to_tab.get(&session.pane_id) {
                session.tab_index = Some(*idx);
                session.tab_name = Some(name.clone());
                if session.pending_tab_rename {
                    session.pending_tab_rename = false;
                    pending_renames.push((*idx, session.display_name.clone()));
                }
            }
        }
        // Issue deferred tab renames for tabs with a single session.
        for (tab_idx, _display_name) in &pending_renames {
            let sessions_on_tab = self
                .sessions
                .values()
                .filter(|s| s.tab_index == Some(*tab_idx))
                .count();
            if sessions_on_tab == 1 {
                auto_rename_tab(*tab_idx, _display_name);
            }
        }
    }

    /// Remove sessions whose panes no longer exist or have exited.
    /// Uses the raw pane manifest for stable pane IDs.
    pub fn remove_dead_sessions(&mut self) -> bool {
        let before = self.sessions.len();
        if before == 0 {
            return false;
        }
        let manifest = match self.pane_manifest {
            Some(ref m) => m,
            None => return false,
        };

        let mut all_pane_ids = HashSet::new();
        let mut exited_pane_ids = HashSet::new();
        for panes in manifest.panes.values() {
            for pane in panes {
                if !pane.is_plugin {
                    all_pane_ids.insert(pane.id);
                    if pane.exited {
                        exited_pane_ids.insert(pane.id);
                    }
                }
            }
        }

        if all_pane_ids.is_empty() {
            return false;
        }

        // Only remove sessions whose pane is confirmed exited.
        // Do NOT remove sessions whose pane_id is absent from the manifest,
        // as the manifest may be temporarily incomplete during rapid updates.
        self.sessions.retain(|pane_id, _| {
            !exited_pane_ids.contains(pane_id)
        });
        if self.sessions.len() != before {
            self.pending_git_branch
                .retain(|id| self.sessions.contains_key(id));
            crate::debug_log(&format!(
                "CTRL CLEANUP removed {} dead sessions, {} remaining",
                before - self.sessions.len(),
                self.sessions.len()
            ));
        }
        self.sessions.len() != before
    }

    /// Get sessions sorted by tab index for display.
    pub fn sessions_by_tab_order(&self) -> Vec<&Session> {
        let mut sessions: Vec<&Session> = self.sessions.values().collect();
        sessions.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        sessions
    }

    /// Get all session display names except a given pane_id (for deduplication).
    pub fn session_names_except(&self, exclude_pane_id: u32) -> Vec<&str> {
        self.sessions
            .iter()
            .filter(|(&id, _)| id != exclude_pane_id)
            .map(|(_, s)| s.display_name.as_str())
            .collect()
    }

    /// Transition stale sessions to Idle after timeout.
    ///
    /// Done/AgentDone: transition after `timeout_secs` (show green checkmark).
    /// Working: transition after `timeout_secs` as a fallback. Claude Code's
    ///   `Stop` hook does not fire reliably on natural response completion,
    ///   so sessions can get stuck in Working after Claude finishes generating.
    ///   If no hook events arrive within the timeout, the session has finished.
    /// Waiting: NOT cleaned up. The user may take arbitrarily long to respond
    ///   to a permission prompt. Only cleared by actual hook events.
    pub fn cleanup_stale_sessions(&mut self, timeout_secs: u64) -> bool {
        let now = crate::session::unix_now();
        let auto_pause = self.config.auto_pause_secs;
        let mut changed = false;
        for session in self.sessions.values_mut() {
            match session.activity {
                Activity::Done | Activity::AgentDone => {
                    if now.saturating_sub(session.last_event_ts) >= timeout_secs {
                        session.activity = Activity::Idle;
                        changed = true;
                    }
                }
                Activity::Working => {
                    if now.saturating_sub(session.last_event_ts) >= timeout_secs {
                        session.activity = Activity::Done;
                        changed = true;
                    }
                }
                Activity::Idle if !session.paused => {
                    if auto_pause > 0
                        && now.saturating_sub(session.last_event_ts) >= auto_pause
                    {
                        session.paused = true;
                        changed = true;
                    }
                }
                _ => {}
            }
        }
        changed
    }

    /// Whether the startup grace period is currently active.
    pub fn in_startup_grace(&self) -> bool {
        self.startup_grace_until
            .map(|deadline| crate::session::unix_now_ms() < deadline)
            .unwrap_or(false)
    }

    /// Mark render payload as needing broadcast on the next timer flush.
    pub fn mark_render_dirty(&mut self) {
        self.render_dirty = true;
    }

    /// Merge incoming sessions (used for restore from cache).
    pub fn merge_sessions(&mut self, incoming: BTreeMap<u32, Session>) -> bool {
        let mut changed = false;
        for (pane_id, mut session) in incoming {
            let dominated = self
                .sessions
                .get(&pane_id)
                .map(|existing| session.last_event_ts > existing.last_event_ts)
                .unwrap_or(true);
            if dominated {
                if let Some((idx, name)) = self.pane_to_tab.get(&pane_id) {
                    session.tab_index = Some(*idx);
                    session.tab_name = Some(name.clone());
                }
                self.sessions.insert(pane_id, session);
                changed = true;
            }
        }
        changed
    }

    // --- Persistence (single-writer pattern) ---

    /// Persist full session state to disk for reattach recovery.
    /// Only the controller calls this; sidebars never write.
    /// Writes to the PID-scoped path (no separate PID file needed).
    pub fn save_sessions(&self) {
        let pid = current_zellij_pid();
        if let Ok(json) = serde_json::to_string(&self.sessions) {
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

    /// Migrate legacy state files (pre-044) to PID-scoped paths.
    /// Called once on controller startup.
    pub fn migrate_legacy_files() {
        let pid = current_zellij_pid();
        if pid == 0 {
            return;
        }

        let scoped_sessions = sessions_path(pid);

        // Migrate sessions.json if PID-scoped file does not exist yet.
        // Only remove legacy file after successful write.
        if std::fs::metadata(&scoped_sessions).is_err() {
            if let Ok(content) = std::fs::read_to_string(LEGACY_SESSIONS_PATH) {
                if std::fs::write(&scoped_sessions, &content).is_ok() {
                    let _ = std::fs::remove_file(LEGACY_SESSIONS_PATH);
                }
            }
        }

        // Remove legacy meta and PID files (controller does not use meta file)
        let _ = std::fs::remove_file(LEGACY_META_PATH);
        let _ = std::fs::remove_file(LEGACY_PID_PATH);
    }
}

/// Get the Zellij server PID.
#[cfg(target_family = "wasm")]
fn current_zellij_pid() -> u32 {
    zellij_tile::prelude::get_plugin_ids().zellij_pid
}

#[cfg(not(target_family = "wasm"))]
fn current_zellij_pid() -> u32 {
    0
}

/// Rename a Zellij tab by index (0-based).
#[cfg(target_family = "wasm")]
fn auto_rename_tab(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn auto_rename_tab(_tab_idx: usize, _name: &str) {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::{Session, WaitReason};

    fn make_session(pane_id: u32) -> Session {
        Session::new(pane_id, format!("session-{pane_id}"))
    }

    fn make_pane_info(id: u32, is_plugin: bool) -> PaneInfo {
        PaneInfo {
            id,
            is_plugin,
            is_focused: false,
            is_fullscreen: false,
            is_floating: false,
            is_suppressed: false,
            title: String::new(),
            exited: false,
            exit_status: None,
            is_held: false,
            pane_x: 0,
            pane_content_x: 0,
            pane_y: 0,
            pane_content_y: 0,
            pane_rows: 0,
            pane_content_rows: 0,
            pane_columns: 0,
            pane_content_columns: 0,
            cursor_coordinates_in_pane: None,
            terminal_command: None,
            plugin_url: None,
            is_selectable: true,
            index_in_pane_group: std::collections::BTreeMap::new(),
            default_bg: None,
            default_fg: None,
        }
    }

    fn make_manifest(terminal_pane_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_pane_ids
            .iter()
            .map(|&id| make_pane_info(id, false))
            .collect();
        let mut map = HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    fn make_manifest_with_exited(terminal_ids: &[u32], exited_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_ids
            .iter()
            .map(|&id| {
                let mut p = make_pane_info(id, false);
                if exited_ids.contains(&id) {
                    p.exited = true;
                }
                p
            })
            .collect();
        let mut map = HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    #[test]
    fn test_remove_dead_sessions_basic() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        // Pane 20 must be present AND exited for removal (absent panes are not removed)
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
    }

    #[test]
    fn test_remove_dead_sessions_exited() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
    }

    #[test]
    fn test_startup_grace_skips_removal() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest(&[10]));
        state.startup_grace_until = Some(crate::session::unix_now_ms() + 3000);

        assert!(state.in_startup_grace());
        // Caller should check in_startup_grace() before calling remove_dead_sessions
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_cleanup_stale_sessions() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Done;
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        let changed = state.cleanup_stale_sessions(30);
        assert!(changed);
        assert_eq!(state.sessions[&10].activity, Activity::Idle);
    }

    #[test]
    fn test_cleanup_stale_working_becomes_done() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Working;
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        let changed = state.cleanup_stale_sessions(30);
        assert!(changed);
        assert_eq!(state.sessions[&10].activity, Activity::Done);
    }

    #[test]
    fn test_cleanup_never_touches_waiting_sessions() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Waiting(WaitReason::Permission);
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        // Waiting is never cleaned up by the timer, regardless of age
        let changed = state.cleanup_stale_sessions(30);
        assert!(!changed);
        assert_eq!(
            state.sessions[&10].activity,
            Activity::Waiting(WaitReason::Permission)
        );
    }

    #[test]
    fn test_merge_sessions() {
        let mut state = ControllerState::default();
        let mut incoming = BTreeMap::new();
        incoming.insert(1, {
            let mut s = make_session(1);
            s.display_name = "api".to_string();
            s.last_event_ts = 100;
            s
        });

        assert!(state.merge_sessions(incoming));
        assert_eq!(state.sessions.len(), 1);
        assert_eq!(state.sessions[&1].display_name, "api");
    }

    #[test]
    fn test_session_names_except() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1);
        s1.display_name = "api".to_string();
        state.sessions.insert(1, s1);
        let mut s2 = make_session(2);
        s2.display_name = "web".to_string();
        state.sessions.insert(2, s2);

        let names = state.session_names_except(1);
        assert_eq!(names, vec!["web"]);
    }

    #[test]
    fn test_sessions_by_tab_order() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1);
        s1.tab_index = Some(2);
        state.sessions.insert(1, s1);
        let mut s2 = make_session(2);
        s2.tab_index = Some(0);
        state.sessions.insert(2, s2);

        let ordered = state.sessions_by_tab_order();
        assert_eq!(ordered[0].pane_id, 2);
        assert_eq!(ordered[1].pane_id, 1);
    }

    #[test]
    fn test_mark_render_dirty() {
        let mut state = ControllerState::default();
        assert!(!state.render_dirty);
        state.mark_render_dirty();
        assert!(state.render_dirty);
    }
}
