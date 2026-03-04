use std::collections::{BTreeMap, HashMap};

use crate::config::PluginConfig;
use crate::group::{ProjectGroup, GROUP_COLORS};
use crate::recent::RecentEntries;
use crate::session::Session;

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
    /// Whether the rename text input is active
    pub rename_active: bool,
    /// Buffer for the rename text input
    pub rename_buffer: String,
    /// Recently used session directories (LRU cache)
    pub recent: RecentEntries,
    /// Transient error message displayed in the status bar
    pub error_message: Option<String>,
    /// Countdown counter for clearing the error message (decremented on each timer tick)
    pub error_clear_counter: u8,
    /// Whether a close-session confirmation is active
    pub close_confirm_active: bool,
    /// Pane ID of the session pending close confirmation
    pub close_target_pane_id: Option<u32>,
    /// Cooldown counter for picker toggle debounce (decremented on each timer tick)
    pub picker_toggle_cooldown: u8,
}

impl PluginState {
    /// Generate an ISO 8601 timestamp from the current system time.
    ///
    /// Returns seconds since the Unix epoch as a simple numeric string.
    /// In a WASI environment, full date formatting libraries are not available,
    /// so we use epoch seconds as a sortable timestamp.
    pub fn iso_timestamp() -> String {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_secs().to_string())
            .unwrap_or_else(|_| "0".to_string())
    }

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
    ///
    /// Returns the session ID so the caller can trigger git detection.
    pub fn register_session(&mut self, pane_id: u32, context: BTreeMap<String, String>) -> Option<u32> {
        let session_id = context
            .get("session_id")
            .and_then(|s| s.parse::<u32>().ok());

        let cwd = session_id
            .and_then(|id| self.pending_sessions.remove(&id))
            .unwrap_or_else(|| std::path::PathBuf::from("."));

        let id = session_id.unwrap_or_else(|| self.next_id());
        let mut session = Session::new(id, pane_id, cwd.clone());

        // Apply duplicate name detection for the initial directory-based name
        let unique_name = self.unique_display_name(&session.display_name, Some(id));
        if unique_name != session.display_name {
            session.display_name = unique_name.clone();
            session.auto_name = unique_name;
        }

        // Update MRU timestamp
        self.mru_counter += 1;
        session.last_activity_secs = self.mru_counter;

        // Create or join project group
        let group_id = session.group_id.clone();
        let display_name = session.display_name.clone();
        self.get_or_create_group(&group_id, &display_name);

        self.sessions.insert(id, session);
        Some(id)
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

    /// Handle a key event while the close confirmation is active.
    ///
    /// 'y' confirms the close, 'n' or Escape cancels.
    #[cfg(target_arch = "wasm32")]
    pub fn handle_close_confirm_key(&mut self, key: zellij_tile::prelude::KeyWithModifier) {
        use zellij_tile::prelude::*;
        match key.bare_key {
            BareKey::Char('y') | BareKey::Char('Y') => {
                if let Some(pane_id) = self.close_target_pane_id {
                    close_terminal_pane(pane_id);
                    self.remove_session_by_pane(pane_id);
                }
                self.close_confirm_active = false;
                self.close_target_pane_id = None;
                // Return focus to next available pane
                if let Some(prev_pane) = self.focused_pane_id {
                    focus_terminal_pane(prev_pane, true);
                }
            }
            BareKey::Char('n') | BareKey::Char('N') | BareKey::Esc => {
                self.close_confirm_active = false;
                self.close_target_pane_id = None;
                if let Some(prev_pane) = self.focused_pane_id {
                    focus_terminal_pane(prev_pane, true);
                }
            }
            _ => {} // Ignore other keys
        }
    }

    /// Handle a key event while the rename prompt is active.
    ///
    /// Processes character input, backspace, Enter (apply), and Escape (cancel).
    #[cfg(target_arch = "wasm32")]
    pub fn handle_rename_key(&mut self, key: zellij_tile::prelude::KeyWithModifier) {
        use zellij_tile::prelude::*;
        match key.bare_key {
            BareKey::Esc => {
                self.rename_active = false;
                self.rename_buffer.clear();
                // Return focus to the previously focused terminal pane
                if let Some(prev_pane) = self.focused_pane_id {
                    focus_terminal_pane(prev_pane, true);
                }
            }
            BareKey::Enter => {
                let new_name = self.rename_buffer.trim().to_string();
                if !new_name.is_empty() {
                    if let Some(pane_id) = self.focused_pane_id {
                        // Apply the rename to the focused session
                        let unique_name = {
                            // Find the session ID for exclusion
                            let session_id = self
                                .sessions
                                .values()
                                .find(|s| s.pane_id == pane_id)
                                .map(|s| s.id);
                            self.unique_display_name(&new_name, session_id)
                        };
                        if let Some(session) = self.session_by_pane_id_mut(pane_id) {
                            session.set_name(unique_name, true);
                        }
                    }
                }
                self.rename_active = false;
                self.rename_buffer.clear();
                if let Some(prev_pane) = self.focused_pane_id {
                    focus_terminal_pane(prev_pane, true);
                }
            }
            BareKey::Backspace => {
                self.rename_buffer.pop();
            }
            BareKey::Char(c) => {
                self.rename_buffer.push(c);
            }
            _ => {}
        }
    }

    /// Get the Nth session (1-indexed) for direct switching.
    pub fn session_pane_by_index(&self, index: usize) -> Option<u32> {
        self.sessions
            .values()
            .nth(index.saturating_sub(1))
            .map(|s| s.pane_id)
    }

    /// Generate a unique display name, appending a numeric suffix if needed.
    ///
    /// If `base_name` is already used by another session (excluding `exclude_id`),
    /// appends "-2", "-3", etc. until a unique name is found.
    pub fn unique_display_name(&self, base_name: &str, exclude_id: Option<u32>) -> String {
        let is_taken = |name: &str| -> bool {
            self.sessions.values().any(|s| {
                s.display_name == name && (exclude_id != Some(s.id))
            })
        };

        if !is_taken(base_name) {
            return base_name.to_string();
        }

        let mut suffix = 2;
        loop {
            let candidate = format!("{}-{}", base_name, suffix);
            if !is_taken(&candidate) {
                return candidate;
            }
            suffix += 1;
        }
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
        assert!(state.recent.is_empty());
    }

    #[test]
    fn test_iso_timestamp() {
        let ts = PluginState::iso_timestamp();
        // Should be a valid numeric string (epoch seconds)
        let secs: u64 = ts.parse().expect("timestamp should be numeric");
        assert!(secs > 0);
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
    fn test_unique_display_name_no_conflict() {
        let state = PluginState::default();
        assert_eq!(state.unique_display_name("api-server", None), "api-server");
    }

    #[test]
    fn test_unique_display_name_with_conflict() {
        let mut state = PluginState::default();
        let mut session = Session::new(0, 10, PathBuf::from("/tmp/api-server"));
        session.display_name = "api-server".to_string();
        state.sessions.insert(0, session);

        // New name should get suffix "-2"
        assert_eq!(
            state.unique_display_name("api-server", None),
            "api-server-2"
        );
    }

    #[test]
    fn test_unique_display_name_with_multiple_conflicts() {
        let mut state = PluginState::default();

        let mut s1 = Session::new(0, 10, PathBuf::from("/a"));
        s1.display_name = "api-server".to_string();
        state.sessions.insert(0, s1);

        let mut s2 = Session::new(1, 11, PathBuf::from("/b"));
        s2.display_name = "api-server-2".to_string();
        state.sessions.insert(1, s2);

        assert_eq!(
            state.unique_display_name("api-server", None),
            "api-server-3"
        );
    }

    #[test]
    fn test_unique_display_name_excludes_own_session() {
        let mut state = PluginState::default();

        let mut session = Session::new(0, 10, PathBuf::from("/tmp/api-server"));
        session.display_name = "api-server".to_string();
        state.sessions.insert(0, session);

        // When excluding session 0, "api-server" should be available
        assert_eq!(
            state.unique_display_name("api-server", Some(0)),
            "api-server"
        );
    }

    #[test]
    fn test_register_session_returns_session_id() {
        let mut state = PluginState::default();
        let id = state.prepare_session(PathBuf::from("/home/user/my-project"));
        let context = BTreeMap::from([("session_id".to_string(), id.to_string())]);
        let result = state.register_session(42, context);
        assert_eq!(result, Some(0));
    }

    #[test]
    fn test_register_session_deduplicates_names() {
        let mut state = PluginState::default();

        // Register first session
        let id1 = state.prepare_session(PathBuf::from("/home/user/my-project"));
        let ctx1 = BTreeMap::from([("session_id".to_string(), id1.to_string())]);
        state.register_session(42, ctx1);

        // Register second session with same directory name
        let id2 = state.prepare_session(PathBuf::from("/other/path/my-project"));
        let ctx2 = BTreeMap::from([("session_id".to_string(), id2.to_string())]);
        state.register_session(43, ctx2);

        assert_eq!(state.sessions[&0].display_name, "my-project");
        assert_eq!(state.sessions[&1].display_name, "my-project-2");
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

    #[test]
    fn test_default_state_new_fields() {
        let state = PluginState::default();
        assert!(state.error_message.is_none());
        assert_eq!(state.error_clear_counter, 0);
        assert!(!state.close_confirm_active);
        assert!(state.close_target_pane_id.is_none());
        assert_eq!(state.picker_toggle_cooldown, 0);
    }

    #[test]
    fn test_error_message_set_and_clear() {
        let mut state = PluginState::default();
        state.error_message = Some("test error".to_string());
        state.error_clear_counter = 3;

        // Simulate timer ticks
        state.error_clear_counter -= 1;
        assert_eq!(state.error_clear_counter, 2);
        assert!(state.error_message.is_some());

        state.error_clear_counter -= 1;
        state.error_clear_counter -= 1;
        assert_eq!(state.error_clear_counter, 0);
        // Caller would clear the message when counter reaches 0
        state.error_message = None;
        assert!(state.error_message.is_none());
    }

    #[test]
    fn test_close_confirm_state() {
        let mut state = PluginState::default();
        state.sessions.insert(0, Session::new(0, 42, PathBuf::from("/tmp/test")));

        // Initiate close confirmation
        state.close_confirm_active = true;
        state.close_target_pane_id = Some(42);
        assert!(state.close_confirm_active);
        assert_eq!(state.close_target_pane_id, Some(42));

        // Cancel
        state.close_confirm_active = false;
        state.close_target_pane_id = None;
        assert!(!state.close_confirm_active);
    }

    #[test]
    fn test_picker_toggle_cooldown() {
        let mut state = PluginState::default();
        assert_eq!(state.picker_toggle_cooldown, 0);

        // Simulate setting cooldown on toggle
        state.picker_toggle_cooldown = 3;

        // Simulate timer ticks decrementing cooldown
        state.picker_toggle_cooldown -= 1;
        assert_eq!(state.picker_toggle_cooldown, 2);

        state.picker_toggle_cooldown -= 1;
        assert_eq!(state.picker_toggle_cooldown, 1);

        state.picker_toggle_cooldown -= 1;
        assert_eq!(state.picker_toggle_cooldown, 0);
    }

    #[test]
    fn test_close_confirm_removes_session() {
        let mut state = PluginState::default();
        state.sessions.insert(0, Session::new(0, 42, PathBuf::from("/tmp/test")));
        state.get_or_create_group("test", "test");

        state.close_confirm_active = true;
        state.close_target_pane_id = Some(42);

        // Simulate confirming close
        state.remove_session_by_pane(42);
        state.close_confirm_active = false;
        state.close_target_pane_id = None;

        assert!(state.sessions.is_empty());
        assert!(!state.close_confirm_active);
    }
}
