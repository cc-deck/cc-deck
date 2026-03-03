use std::collections::{BTreeMap, HashMap};

use crate::config::PluginConfig;
use crate::session::Session;

/// Color palette for project groups.
pub const GROUP_COLORS: &[&str] = &[
    "blue", "green", "yellow", "magenta", "cyan", "red", "white",
];

/// A logical grouping of sessions sharing the same project.
#[derive(Debug, Clone)]
pub struct ProjectGroup {
    /// Normalized project name (lowercase)
    pub id: String,
    /// Original-case project name
    pub display_name: String,
    /// Assigned color from palette
    pub color: String,
    /// Number of active sessions in this group
    pub session_count: usize,
}

/// Main plugin state holding all runtime data.
#[derive(Default)]
pub struct PluginState {
    /// Active sessions keyed by session ID
    pub sessions: BTreeMap<u32, Session>,
    /// Project groups keyed by group ID
    pub groups: HashMap<String, ProjectGroup>,
    /// Currently focused Zellij pane ID
    pub focused_pane_id: Option<u32>,
    /// Whether the fuzzy picker is currently showing
    pub picker_active: bool,
    /// Current search text in the picker
    pub picker_query: String,
    /// Currently highlighted item index in picker
    pub picker_selected: usize,
    /// Configurable idle timeout in seconds
    pub idle_timeout_secs: u64,
    /// User configuration from KDL
    pub config: PluginConfig,
    /// Counter for session ID generation
    pub next_session_id: u32,
    /// Counter for color palette assignment
    pub next_color_index: usize,
    /// This plugin's own ID (set during load)
    pub plugin_id: u32,
}

impl PluginState {
    /// Find a session by its Zellij pane ID.
    pub fn session_by_pane_id(&self, pane_id: u32) -> Option<&Session> {
        self.sessions.values().find(|s| s.pane_id == pane_id)
    }

    /// Find a mutable session by its Zellij pane ID.
    pub fn session_by_pane_id_mut(&mut self, pane_id: u32) -> Option<&mut Session> {
        self.sessions.values_mut().find(|s| s.pane_id == pane_id)
    }

    /// Allocate the next session ID.
    pub fn next_id(&mut self) -> u32 {
        let id = self.next_session_id;
        self.next_session_id += 1;
        id
    }

    /// Get or create a project group for the given group ID.
    pub fn get_or_create_group(&mut self, group_id: &str, display_name: &str) -> String {
        if !self.groups.contains_key(group_id) {
            let color_idx = self.next_color_index % GROUP_COLORS.len();
            self.next_color_index += 1;
            self.groups.insert(
                group_id.to_string(),
                ProjectGroup {
                    id: group_id.to_string(),
                    display_name: display_name.to_string(),
                    color: GROUP_COLORS[color_idx].to_string(),
                    session_count: 0,
                },
            );
        }
        if let Some(group) = self.groups.get_mut(group_id) {
            group.session_count += 1;
        }
        group_id.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    #[test]
    fn test_default_state() {
        let state = PluginState::default();
        assert!(state.sessions.is_empty());
        assert!(state.groups.is_empty());
        assert_eq!(state.focused_pane_id, None);
        assert!(!state.picker_active);
        assert_eq!(state.next_session_id, 0);
    }

    #[test]
    fn test_next_id() {
        let mut state = PluginState::default();
        assert_eq!(state.next_id(), 0);
        assert_eq!(state.next_id(), 1);
        assert_eq!(state.next_id(), 2);
    }

    #[test]
    fn test_session_by_pane_id() {
        let mut state = PluginState::default();
        let session = Session::new(0, 42, PathBuf::from("/tmp/test"));
        state.sessions.insert(0, session);

        assert!(state.session_by_pane_id(42).is_some());
        assert!(state.session_by_pane_id(99).is_none());
    }

    #[test]
    fn test_get_or_create_group() {
        let mut state = PluginState::default();

        let group_id = state.get_or_create_group("api-server", "api-server");
        assert_eq!(group_id, "api-server");
        assert_eq!(state.groups["api-server"].color, "blue");
        assert_eq!(state.groups["api-server"].session_count, 1);

        // Adding another session to same group
        state.get_or_create_group("api-server", "api-server");
        assert_eq!(state.groups["api-server"].session_count, 2);

        // New group gets next color
        state.get_or_create_group("frontend", "frontend");
        assert_eq!(state.groups["frontend"].color, "green");
    }
}
