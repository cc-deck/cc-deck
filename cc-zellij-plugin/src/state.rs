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
        // Refresh tab info on all sessions
        for session in self.sessions.values_mut() {
            if let Some((idx, name)) = self.pane_to_tab.get(&session.pane_id) {
                session.tab_index = Some(*idx);
                session.tab_name = Some(name.clone());
            }
        }
    }

    /// Remove sessions whose panes no longer exist.
    pub fn remove_dead_sessions(&mut self) -> bool {
        let before = self.sessions.len();
        if before == 0 {
            return false;
        }
        if self.pane_to_tab.is_empty() && self.pane_manifest.is_some() {
            return false;
        }
        if self.pane_manifest.is_some() {
            let session_keys: Vec<_> = self.sessions.keys().cloned().collect();
            let pane_keys: Vec<_> = self.pane_to_tab.keys().cloned().collect();
            self.sessions
                .retain(|pane_id, _| self.pane_to_tab.contains_key(pane_id));
            if self.sessions.len() != before {
                crate::debug_log(&format!("CLEANUP removed: session_keys={:?} pane_keys={:?} remaining={}",
                    session_keys, pane_keys, self.sessions.len()));
            }
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
}

