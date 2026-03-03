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
    /// Monotonic counter for MRU ordering (incremented on each focus change)
    pub mru_counter: u64,
    /// Pending sessions: session_id -> cwd, waiting for CommandPaneOpened
    pub pending_sessions: BTreeMap<u32, std::path::PathBuf>,
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

    /// Prepare a new session for creation and return the session ID.
    ///
    /// Records the session as "pending" until we receive CommandPaneOpened
    /// with the matching session_id in its context. The caller is responsible
    /// for calling `open_command_pane` with the returned session_id.
    pub fn prepare_session(&mut self, cwd: std::path::PathBuf) -> u32 {
        let session_id = self.next_id();
        self.pending_sessions.insert(session_id, cwd);
        session_id
    }

    /// Register a session when its command pane has been opened by Zellij.
    pub fn register_session(&mut self, pane_id: u32, context: BTreeMap<String, String>) {
        let session_id = context
            .get("session_id")
            .and_then(|s| s.parse::<u32>().ok());

        let cwd = session_id
            .and_then(|id| self.pending_sessions.remove(&id))
            .unwrap_or_else(|| std::path::PathBuf::from("."));

        let id = session_id.unwrap_or_else(|| self.next_id());
        let mut session = Session::new(id, pane_id, cwd.clone());

        // Update MRU timestamp
        self.mru_counter += 1;
        session.last_activity_secs = self.mru_counter;

        // Create or join project group
        let group_id = session.group_id.clone();
        let display_name = session.display_name.clone();
        self.get_or_create_group(&group_id, &display_name);

        self.sessions.insert(id, session);
    }

    /// Update MRU timestamp for a session when it gains focus.
    pub fn touch_session_mru(&mut self, pane_id: u32) {
        self.mru_counter += 1;
        let counter = self.mru_counter;
        if let Some(session) = self.session_by_pane_id_mut(pane_id) {
            session.last_activity_secs = counter;
        }
    }

    /// Remove a session by its pane ID, cleaning up the associated project group.
    pub fn remove_session_by_pane(&mut self, pane_id: u32) {
        let session_id = self
            .sessions
            .iter()
            .find(|(_, s)| s.pane_id == pane_id)
            .map(|(id, _)| *id);
        if let Some(id) = session_id {
            if let Some(session) = self.sessions.remove(&id) {
                if let Some(group) = self.groups.get_mut(&session.group_id) {
                    group.session_count = group.session_count.saturating_sub(1);
                    if group.session_count == 0 {
                        self.groups.remove(&session.group_id);
                    }
                }
            }
        }
    }

    /// Get the Nth session (1-indexed) for direct switching.
    pub fn session_pane_by_index(&self, index: usize) -> Option<u32> {
        self.sessions
            .values()
            .nth(index.saturating_sub(1))
            .map(|s| s.pane_id)
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

    #[test]
    fn test_prepare_session() {
        let mut state = PluginState::default();
        let id = state.prepare_session(PathBuf::from("/tmp/project"));
        assert_eq!(id, 0);
        assert!(state.pending_sessions.contains_key(&0));
        assert_eq!(
            state.pending_sessions[&0],
            PathBuf::from("/tmp/project")
        );
    }

    #[test]
    fn test_register_session() {
        let mut state = PluginState::default();
        let id = state.prepare_session(PathBuf::from("/home/user/my-project"));

        let context = BTreeMap::from([("session_id".to_string(), id.to_string())]);
        state.register_session(42, context);

        assert!(state.sessions.contains_key(&0));
        let session = &state.sessions[&0];
        assert_eq!(session.pane_id, 42);
        assert_eq!(session.display_name, "my-project");
        assert!(state.pending_sessions.is_empty());
        assert!(state.groups.contains_key("my-project"));
    }

    #[test]
    fn test_touch_session_mru() {
        let mut state = PluginState::default();
        let session = Session::new(0, 42, PathBuf::from("/tmp/test"));
        state.sessions.insert(0, session);

        state.touch_session_mru(42);
        assert_eq!(state.sessions[&0].last_activity_secs, 1);

        state.touch_session_mru(42);
        assert_eq!(state.sessions[&0].last_activity_secs, 2);
    }

    #[test]
    fn test_session_pane_by_index() {
        let mut state = PluginState::default();
        state.sessions.insert(0, Session::new(0, 10, PathBuf::from("/a")));
        state.sessions.insert(1, Session::new(1, 20, PathBuf::from("/b")));
        state.sessions.insert(2, Session::new(2, 30, PathBuf::from("/c")));

        assert_eq!(state.session_pane_by_index(1), Some(10));
        assert_eq!(state.session_pane_by_index(2), Some(20));
        assert_eq!(state.session_pane_by_index(3), Some(30));
        assert_eq!(state.session_pane_by_index(4), None);
    }

    #[test]
    fn test_remove_session_by_pane() {
        let mut state = PluginState::default();
        state.sessions.insert(0, Session::new(0, 42, PathBuf::from("/tmp/test")));
        state.get_or_create_group("test", "test");

        state.remove_session_by_pane(42);
        assert!(state.sessions.is_empty());
        // Group should be removed since session count reached 0
        assert!(!state.groups.contains_key("test"));
    }

    #[test]
    fn test_remove_session_preserves_other_sessions() {
        let mut state = PluginState::default();
        state.sessions.insert(0, Session::new(0, 42, PathBuf::from("/tmp/a")));
        state.sessions.insert(1, Session::new(1, 43, PathBuf::from("/tmp/b")));

        state.remove_session_by_pane(42);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&1));
    }
}
