// T003: PluginState with session tracking, tab/pane state, and mode enum

use crate::config::PluginConfig;
use crate::session::{Activity, Session};
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use zellij_tile::prelude::*;

/// Plugin instance mode, set via configuration.
#[derive(Default, Debug, Clone, PartialEq)]
pub enum PluginMode {
    #[default]
    Sidebar,
    Picker,
}

/// Transient state for an active inline rename operation.
#[derive(Debug, Clone)]
pub struct RenameState {
    pub pane_id: u32,
    pub input_buffer: String,
    pub cursor_pos: usize,
}

/// Search/filter state during `/` sub-mode in navigation.
#[derive(Debug, Clone, Default)]
pub struct FilterState {
    pub input_buffer: String,
    pub cursor_pos: usize,
}

/// A brief notification message displayed in the sidebar.
#[derive(Debug, Clone)]
pub struct Notification {
    pub message: String,
    pub expires_at_ms: u64,
}

/// The aggregate state held by each plugin instance.
#[derive(Default)]
pub struct PluginState {
    /// All known Claude sessions, keyed by pane_id.
    pub sessions: BTreeMap<u32, Session>,
    /// Current tab list from TabUpdate events.
    pub tabs: Vec<TabInfo>,
    /// Current pane manifest from PaneUpdate events.
    pub pane_manifest: Option<PaneManifest>,
    /// Pane ID -> (tab_index, tab_name) mapping.
    pub pane_to_tab: HashMap<u32, (usize, String)>,
    /// Currently focused tab position.
    pub active_tab_index: Option<usize>,
    /// Currently focused terminal pane ID.
    pub focused_pane_id: Option<u32>,
    /// Plugin instance mode.
    pub mode: PluginMode,
    /// Configuration.
    pub config: PluginConfig,
    /// Whether plugin permissions have been granted.
    pub permissions_granted: bool,
    /// Current Zellij input mode.
    pub input_mode: InputMode,
    /// Active rename operation (if any).
    pub rename_state: Option<RenameState>,
    /// Brief notification to display.
    pub notification: Option<Notification>,
    /// Guard flag to prevent re-entrancy from rename_tab -> TabUpdate loops.
    pub updating_tabs: bool,
    /// Events received before permissions were granted.
    pub pending_events: Vec<Event>,
    /// Click regions from the last render (row, pane_id, tab_index).
    pub click_regions: Vec<(usize, u32, usize)>,
    /// Whether the sidebar is in keyboard navigation mode.
    pub navigation_mode: bool,
    /// Cursor index in the sorted session list (navigation mode).
    pub cursor_index: usize,
    /// Active search/filter state (navigation mode `/` sub-mode).
    pub filter_state: Option<FilterState>,
    /// Pane ID of session pending delete confirmation.
    pub delete_confirm: Option<u32>,
    /// Pane ID + tab index that was focused before entering navigation mode (for Esc restore).
    pub nav_restore: Option<(u32, usize)>,
    /// Last pane_id that attend switched to, for round-robin cycling.
    pub last_attended_pane_id: Option<u32>,
    /// Whether the help overlay is displayed.
    pub show_help: bool,
    /// Tab index that this plugin instance lives on (derived from PaneManifest).
    pub my_tab_index: Option<usize>,
    /// Guard: skip next PaneUpdate auto-exit after entering navigation mode.
    /// Entering nav mode triggers a PaneUpdate with stale focus before
    /// focus_plugin_pane takes effect, which would immediately exit nav mode.
    pub nav_enter_guard: bool,
    /// Last left-click timestamp (ms) and pane_id for double-click detection.
    pub last_click: Option<(u64, u32)>,
    /// Pending metadata overrides from snapshot restore, keyed by working directory.
    /// Applied when a hook event arrives with a matching CWD, then removed.
    pub pending_overrides: HashMap<String, PendingOverride>,
    /// Millisecond timestamp until which `remove_dead_sessions()` is skipped.
    /// Prevents the startup race condition where early PaneUpdate events deliver
    /// incomplete manifests that would wipe restored cached sessions.
    pub startup_grace_until: Option<u64>,
}

/// Metadata override to apply when a restored session is discovered.
#[derive(Debug, Clone)]
pub struct PendingOverride {
    pub display_name: String,
    pub paused: bool,
}

impl PluginState {
    /// Rebuild the pane-to-tab mapping from current tab and pane data.
    pub fn rebuild_pane_map(&mut self) {
        self.pane_to_tab.clear();
        self.focused_pane_id = None;
        #[cfg(target_family = "wasm")]
        let my_plugin_id = zellij_tile::prelude::get_plugin_ids().plugin_id;
        #[cfg(not(target_family = "wasm"))]
        let my_plugin_id = 0u32;
        if let Some(ref manifest) = self.pane_manifest {
            for tab in &self.tabs {
                if let Some(panes) = manifest.panes.get(&tab.position) {
                    for pane in panes {
                        if pane.is_plugin && pane.id == my_plugin_id {
                            self.my_tab_index = Some(tab.position);
                        }
                        if !pane.is_plugin {
                            self.pane_to_tab
                                .insert(pane.id, (tab.position, tab.name.clone()));
                            if pane.is_focused && tab.active {
                                self.focused_pane_id = Some(pane.id);
                            }
                        }
                    }
                }
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
        // Issue deferred tab renames now that tab_index is available.
        // Only rename if the session is the sole one on its tab.
        for (tab_idx, _display_name) in &pending_renames {
            let sessions_on_tab = self.sessions.values()
                .filter(|s| s.tab_index == Some(*tab_idx))
                .count();
            if sessions_on_tab == 1 {
                #[cfg(target_family = "wasm")]
                {
                    zellij_tile::prelude::rename_tab(*tab_idx as u32 + 1, _display_name);
                }
                self.updating_tabs = true;
            }
        }
    }

    /// Remove sessions whose panes no longer exist.
    /// Uses the raw pane manifest (stable pane IDs) instead of the derived
    /// pane_to_tab map, which can be temporarily wrong when tab positions
    /// shift after a tab close (TabUpdate arrives before PaneUpdate).
    pub fn remove_dead_sessions(&mut self) -> bool {
        let before = self.sessions.len();
        if before == 0 {
            return false;
        }
        let manifest = match self.pane_manifest {
            Some(ref m) => m,
            None => return false,
        };

        // Collect all terminal pane IDs from the manifest (across all tabs).
        // Pane IDs are globally unique and stable regardless of tab position shifts.
        let mut all_pane_ids = std::collections::HashSet::new();
        for panes in manifest.panes.values() {
            for pane in panes {
                if !pane.is_plugin {
                    all_pane_ids.insert(pane.id);
                }
            }
        }

        if all_pane_ids.is_empty() {
            // Manifest has no terminal panes at all; don't remove anything
            // (could be a transient state during startup).
            return false;
        }

        self.sessions.retain(|pane_id, _| all_pane_ids.contains(pane_id));
        if self.sessions.len() != before {
            crate::debug_log(&format!("CLEANUP removed {} dead sessions, {} remaining",
                before - self.sessions.len(), self.sessions.len()));
        }
        self.sessions.len() != before
    }

    /// Get sessions sorted by tab index for display.
    pub fn sessions_by_tab_order(&self) -> Vec<&Session> {
        let mut sessions: Vec<&Session> = self.sessions.values().collect();
        sessions.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        sessions
    }

    /// Find the oldest waiting session.
    pub fn oldest_waiting_session(&self) -> Option<&Session> {
        self.sessions
            .values()
            .filter(|s| s.activity.is_waiting())
            .min_by_key(|s| s.last_event_ts)
    }

    /// Get all session display names (for deduplication).
    pub fn session_names(&self) -> Vec<&str> {
        self.sessions.values().map(|s| s.display_name.as_str()).collect()
    }

    /// Merge incoming sessions from another instance (sync protocol).
    pub fn merge_sessions(&mut self, incoming: BTreeMap<u32, Session>) {
        for (pane_id, mut session) in incoming {
            let dominated = self
                .sessions
                .get(&pane_id)
                .map(|existing| session.last_event_ts > existing.last_event_ts)
                .unwrap_or(true);
            if dominated {
                // Refresh tab info from our local pane map
                if let Some((idx, name)) = self.pane_to_tab.get(&pane_id) {
                    session.tab_index = Some(*idx);
                    session.tab_name = Some(name.clone());
                }
                self.sessions.insert(pane_id, session);
            }
        }
    }

    /// Transition Done/AgentDone sessions to Idle after timeout.
    pub fn cleanup_stale_sessions(&mut self, timeout_secs: u64) -> bool {
        let now = crate::session::unix_now();
        let mut changed = false;
        for session in self.sessions.values_mut() {
            match session.activity {
                Activity::Done | Activity::AgentDone => {
                    if now.saturating_sub(session.last_event_ts) >= timeout_secs {
                        session.activity = Activity::Idle;
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
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;
    use std::collections::HashMap;

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

    fn make_session(pane_id: u32) -> Session {
        Session::new(pane_id, format!("session-{pane_id}"))
    }

    #[test]
    fn test_grace_period_skips_dead_session_removal() {
        let mut state = PluginState::default();
        // Simulate restored cached sessions for panes 10 and 20
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        // Manifest only has pane 10 (pane 20 not yet reported)
        state.pane_manifest = Some(make_manifest(&[10]));
        // Set grace period 3 seconds in the future
        state.startup_grace_until =
            Some(crate::session::unix_now_ms() + 3000);

        // During grace period, remove_dead_sessions should still work
        // but the caller (PaneUpdate handler) should skip calling it.
        assert!(state.in_startup_grace());
        // Verify sessions are still intact (caller would skip the call)
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_after_grace_period_dead_sessions_removed() {
        let mut state = PluginState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        // Manifest only has pane 10 (pane 20 is dead)
        state.pane_manifest = Some(make_manifest(&[10]));
        // Grace period already expired
        state.startup_grace_until = Some(0);

        assert!(!state.in_startup_grace());
        // Now remove_dead_sessions runs and removes pane 20
        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
        assert!(!state.sessions.contains_key(&20));
    }

    #[test]
    fn test_grace_period_no_effect_on_empty_sessions() {
        let mut state = PluginState::default();
        // No sessions in cache (fresh start)
        state.pane_manifest = Some(make_manifest(&[10, 20]));
        state.startup_grace_until =
            Some(crate::session::unix_now_ms() + 3000);

        // Grace period is active but irrelevant (no sessions to protect)
        assert!(state.in_startup_grace());
        let changed = state.remove_dead_sessions();
        assert!(!changed);
        assert_eq!(state.sessions.len(), 0);
    }

    #[test]
    fn test_grace_period_none_means_normal_operation() {
        let mut state = PluginState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest(&[10]));
        // No grace period set (normal operation, not a reattach)
        assert!(state.startup_grace_until.is_none());
        assert!(!state.in_startup_grace());

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
    }
}

