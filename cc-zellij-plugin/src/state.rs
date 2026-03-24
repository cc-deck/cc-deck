// T003: PluginState with session tracking, tab/pane state, and mode enum

use crate::config::PluginConfig;
use crate::session::{Activity, Session};
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use zellij_tile::prelude::*;

/// Grace period for ignoring stale PaneUpdate events after mode entry (ms).
/// When we call focus_plugin_pane(), the next PaneUpdate may still show a
/// terminal pane as focused. This grace period prevents that stale event
/// from triggering an auto-exit.
pub const ENTER_GRACE_MS: u64 = 300;

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

/// Context shared across all navigation sub-modes.
#[derive(Debug, Clone)]
pub struct NavigateContext {
    /// Cursor position in the session list.
    pub cursor_index: usize,
    /// Pane + tab to restore on Esc.
    pub restore: Option<(u32, usize)>,
    /// Timestamp (ms) when this mode was entered, for PaneUpdate grace period.
    pub entered_at_ms: u64,
}

/// The sidebar interaction mode. Replaces the previous set of independent
/// boolean/option fields with a single enum that makes illegal state
/// combinations unrepresentable.
#[derive(Default, Debug, Clone)]
pub enum SidebarMode {
    /// Passive: sidebar displays sessions but captures no input.
    #[default]
    Passive,

    /// Cursor navigation active (amber highlight).
    Navigate(NavigateContext),

    /// Search/filter input active (sub-mode of navigate).
    NavigateFilter {
        ctx: NavigateContext,
        filter: FilterState,
    },

    /// Delete confirmation pending (sub-mode of navigate).
    NavigateDeleteConfirm {
        ctx: NavigateContext,
        pane_id: u32,
    },

    /// Inline rename within navigation (via 'r' key).
    NavigateRename {
        ctx: NavigateContext,
        rename: RenameState,
    },

    /// Rename initiated from passive mode (double-click, right-click).
    RenamePassive {
        rename: RenameState,
        /// Timestamp (ms) when rename was entered, for PaneUpdate grace period.
        entered_at_ms: u64,
    },
}


impl SidebarMode {
    /// Whether the sidebar should be selectable (captures mouse/keyboard).
    pub fn is_selectable(&self) -> bool {
        !matches!(self, SidebarMode::Passive)
    }

    /// Whether we're in any navigation sub-mode.
    pub fn is_navigating(&self) -> bool {
        matches!(
            self,
            SidebarMode::Navigate(_)
                | SidebarMode::NavigateFilter { .. }
                | SidebarMode::NavigateDeleteConfirm { .. }
                | SidebarMode::NavigateRename { .. }
        )
    }

    /// Get navigate context reference (if in any navigate sub-mode).
    pub fn nav_ctx(&self) -> Option<&NavigateContext> {
        match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    /// Get mutable navigate context reference (if in any navigate sub-mode).
    pub fn nav_ctx_mut(&mut self) -> Option<&mut NavigateContext> {
        match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    /// Get cursor index (if in a navigation sub-mode).
    pub fn cursor_index(&self) -> usize {
        self.nav_ctx().map(|ctx| ctx.cursor_index).unwrap_or(0)
    }

    /// Whether within the entry grace period (stale PaneUpdate suppression).
    /// Uses timestamp comparison instead of a boolean guard flag: if the
    /// stale PaneUpdate never arrives, the grace period expires naturally.
    pub fn in_grace_period(&self, now_ms: u64) -> bool {
        let entered = match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => ctx.entered_at_ms,
            SidebarMode::RenamePassive { entered_at_ms, .. } => *entered_at_ms,
            SidebarMode::Passive => return false,
        };
        now_ms.saturating_sub(entered) < ENTER_GRACE_MS
    }

    /// Get the rename state if currently renaming (from any parent mode).
    pub fn rename_state(&self) -> Option<&RenameState> {
        match self {
            SidebarMode::NavigateRename { rename, .. }
            | SidebarMode::RenamePassive { rename, .. } => Some(rename),
            _ => None,
        }
    }

    /// Get mutable rename state if currently renaming.
    pub fn rename_state_mut(&mut self) -> Option<&mut RenameState> {
        match self {
            SidebarMode::NavigateRename { rename, .. }
            | SidebarMode::RenamePassive { rename, .. } => Some(rename),
            _ => None,
        }
    }

    /// Get the filter state if currently filtering.
    pub fn filter_state(&self) -> Option<&FilterState> {
        match self {
            SidebarMode::NavigateFilter { filter, .. } => Some(filter),
            _ => None,
        }
    }

    /// Get the delete confirm pane_id if pending.
    pub fn delete_confirm_pane(&self) -> Option<u32> {
        match self {
            SidebarMode::NavigateDeleteConfirm { pane_id, .. } => Some(*pane_id),
            _ => None,
        }
    }
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
    /// Brief notification to display.
    pub notification: Option<Notification>,
    /// Guard flag to prevent re-entrancy from rename_tab -> TabUpdate loops.
    pub updating_tabs: bool,
    /// Events received before permissions were granted.
    pub pending_events: Vec<Event>,
    /// Click regions from the last render (row, pane_id, tab_index).
    pub click_regions: Vec<(usize, u32, usize)>,
    /// The sidebar interaction mode (single enum replaces scattered booleans).
    pub sidebar_mode: SidebarMode,
    /// Last pane_id that attend switched to, for round-robin cycling.
    pub last_attended_pane_id: Option<u32>,
    /// Whether the help overlay is displayed.
    pub show_help: bool,
    /// Tab index that this plugin instance lives on (derived from PaneManifest).
    pub my_tab_index: Option<usize>,
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

    /// Preserve cursor position by clamping after session list changes.
    pub fn preserve_cursor(&mut self) {
        if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
            let count = self.sessions.len();
            if count == 0 {
                ctx.cursor_index = 0;
            } else if ctx.cursor_index >= count {
                ctx.cursor_index = count - 1;
            }
        }
    }

    /// Get filtered sessions in tab order (respects active filter).
    pub fn filtered_sessions_by_tab_order(&self) -> Vec<&Session> {
        let mut sessions: Vec<_> = if let Some(filter) = self.sidebar_mode.filter_state() {
            if filter.input_buffer.is_empty() {
                self.sessions.values().collect()
            } else {
                let lower = filter.input_buffer.to_lowercase();
                self.sessions.values()
                    .filter(|s| s.display_name.to_lowercase().contains(&lower))
                    .collect()
            }
        } else {
            self.sessions.values().collect()
        };
        sessions.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        sessions
    }

    /// Count sessions matching a filter string.
    pub fn filtered_session_count(&self, filter: &str) -> usize {
        if filter.is_empty() {
            return self.sessions.len();
        }
        let lower = filter.to_lowercase();
        self.sessions.values()
            .filter(|s| s.display_name.to_lowercase().contains(&lower))
            .count()
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
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest(&[10]));
        state.startup_grace_until =
            Some(crate::session::unix_now_ms() + 3000);

        assert!(state.in_startup_grace());
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_after_grace_period_dead_sessions_removed() {
        let mut state = PluginState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest(&[10]));
        state.startup_grace_until = Some(0);

        assert!(!state.in_startup_grace());
        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
        assert!(!state.sessions.contains_key(&20));
    }

    #[test]
    fn test_grace_period_no_effect_on_empty_sessions() {
        let mut state = PluginState::default();
        state.pane_manifest = Some(make_manifest(&[10, 20]));
        state.startup_grace_until =
            Some(crate::session::unix_now_ms() + 3000);

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
        assert!(state.startup_grace_until.is_none());
        assert!(!state.in_startup_grace());

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
    }

    #[test]
    fn test_sidebar_mode_passive_not_selectable() {
        assert!(!SidebarMode::Passive.is_selectable());
        assert!(!SidebarMode::Passive.is_navigating());
    }

    #[test]
    fn test_sidebar_mode_navigate_selectable() {
        let mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 0,
            restore: None,
            entered_at_ms: 0,
        });
        assert!(mode.is_selectable());
        assert!(mode.is_navigating());
        assert_eq!(mode.cursor_index(), 0);
    }

    #[test]
    fn test_sidebar_mode_grace_period() {
        let mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 0,
            restore: None,
            entered_at_ms: 1000,
        });
        // Within grace period
        assert!(mode.in_grace_period(1100));
        // After grace period
        assert!(!mode.in_grace_period(1500));
    }

    #[test]
    fn test_sidebar_mode_rename_passive_grace() {
        let mode = SidebarMode::RenamePassive {
            rename: RenameState {
                pane_id: 42,
                input_buffer: "test".into(),
                cursor_pos: 4,
            },
            entered_at_ms: 1000,
        };
        assert!(mode.is_selectable());
        assert!(!mode.is_navigating());
        assert!(mode.in_grace_period(1100));
        assert!(!mode.in_grace_period(1500));
    }

    #[test]
    fn test_sidebar_mode_passive_no_grace() {
        assert!(!SidebarMode::Passive.in_grace_period(0));
        assert!(!SidebarMode::Passive.in_grace_period(u64::MAX));
    }
}
