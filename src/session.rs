use std::path::PathBuf;
use std::time::Duration;

/// Activity status of a Claude Code session.
#[derive(Debug, Clone, PartialEq)]
pub enum SessionStatus {
    /// Claude is generating output or using tools
    Working,
    /// Claude needs user input (permission, question)
    Waiting,
    /// No activity for the configured timeout
    Idle(Duration),
    /// Claude finished responding, awaiting next prompt
    Done,
    /// Claude process terminated with an exit code
    Exited(i32),
    /// No hook data received (hooks not configured)
    Unknown,
}

impl SessionStatus {
    /// Returns a short label for status bar display.
    pub fn label(&self) -> &str {
        match self {
            SessionStatus::Working => "working",
            SessionStatus::Waiting => "waiting",
            SessionStatus::Idle(_) => "idle",
            SessionStatus::Done => "done",
            SessionStatus::Exited(_) => "exited",
            SessionStatus::Unknown => "?",
        }
    }

    /// Returns a single-character indicator for compact display.
    pub fn indicator(&self) -> &str {
        match self {
            SessionStatus::Working => "⚡",
            SessionStatus::Waiting => "⏳",
            SessionStatus::Idle(_) => "💤",
            SessionStatus::Done => "✓",
            SessionStatus::Exited(code) if *code == 0 => "✓",
            SessionStatus::Exited(_) => "✗",
            SessionStatus::Unknown => "?",
        }
    }
}

/// A running Claude Code session managed by cc-deck.
#[derive(Debug, Clone)]
pub struct Session {
    /// Unique session identifier (sequential)
    pub id: u32,
    /// Zellij terminal pane ID
    pub pane_id: u32,
    /// Current display name (auto-detected or manually set)
    pub display_name: String,
    /// Auto-detected name (git repo or directory basename)
    pub auto_name: String,
    /// Whether the user manually renamed this session
    pub is_name_manual: bool,
    /// Absolute path to session's working directory
    pub working_dir: PathBuf,
    /// Current activity status
    pub status: SessionStatus,
    /// Project group identifier (normalized repo/dir name)
    pub group_id: String,
    /// Elapsed seconds since session was created
    pub created_at_secs: u64,
    /// Elapsed seconds since last activity
    pub last_activity_secs: u64,
    /// Exit code if Claude process has terminated
    pub exit_code: Option<i32>,
}

impl Session {
    /// Create a new session with the given parameters.
    pub fn new(id: u32, pane_id: u32, working_dir: PathBuf) -> Self {
        let dir_name = working_dir
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_else(|| "unnamed".to_string());
        let group_id = dir_name.to_lowercase();

        Self {
            id,
            pane_id,
            display_name: dir_name.clone(),
            auto_name: dir_name,
            is_name_manual: false,
            working_dir,
            status: SessionStatus::Unknown,
            group_id,
            created_at_secs: 0,
            last_activity_secs: 0,
            exit_code: None,
        }
    }

    /// Transition session status based on an incoming event.
    pub fn transition_status(&mut self, new_status: SessionStatus) {
        // Don't allow transitions from Exited (terminal state)
        if matches!(self.status, SessionStatus::Exited(_)) {
            return;
        }

        match &new_status {
            SessionStatus::Working => {
                self.status = SessionStatus::Working;
                self.last_activity_secs = 0;
            }
            SessionStatus::Waiting => {
                self.status = SessionStatus::Waiting;
                self.last_activity_secs = 0;
            }
            SessionStatus::Done => {
                self.status = SessionStatus::Done;
                self.last_activity_secs = 0;
            }
            SessionStatus::Idle(duration) => {
                // Only transition to Idle from Done or Unknown
                if matches!(self.status, SessionStatus::Done | SessionStatus::Unknown) {
                    self.status = SessionStatus::Idle(*duration);
                }
            }
            SessionStatus::Exited(code) => {
                self.exit_code = Some(*code);
                self.status = SessionStatus::Exited(*code);
            }
            SessionStatus::Unknown => {
                // Only set Unknown on initial creation
            }
        }
    }

    /// Set the display name, either from auto-detection or manual rename.
    pub fn set_name(&mut self, name: String, manual: bool) {
        self.display_name = name;
        self.is_name_manual = manual;
    }

    /// Set the auto-detected name (from git repo or directory).
    /// Only updates display_name if the user hasn't manually renamed.
    pub fn set_auto_name(&mut self, name: String) {
        self.auto_name = name.clone();
        self.group_id = name.to_lowercase();
        if !self.is_name_manual {
            self.display_name = name;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_new_session() {
        let session = Session::new(1, 42, PathBuf::from("/home/user/projects/api-server"));
        assert_eq!(session.id, 1);
        assert_eq!(session.pane_id, 42);
        assert_eq!(session.display_name, "api-server");
        assert_eq!(session.auto_name, "api-server");
        assert_eq!(session.group_id, "api-server");
        assert!(!session.is_name_manual);
        assert_eq!(session.status, SessionStatus::Unknown);
        assert_eq!(session.exit_code, None);
    }

    #[test]
    fn test_status_transition_working() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.transition_status(SessionStatus::Working);
        assert_eq!(session.status, SessionStatus::Working);
    }

    #[test]
    fn test_status_transition_working_to_done() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.transition_status(SessionStatus::Working);
        session.transition_status(SessionStatus::Done);
        assert_eq!(session.status, SessionStatus::Done);
    }

    #[test]
    fn test_status_transition_done_to_idle() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.transition_status(SessionStatus::Done);
        session.transition_status(SessionStatus::Idle(Duration::from_secs(300)));
        assert!(matches!(session.status, SessionStatus::Idle(_)));
    }

    #[test]
    fn test_status_transition_working_blocks_idle() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.transition_status(SessionStatus::Working);
        session.transition_status(SessionStatus::Idle(Duration::from_secs(300)));
        // Should remain Working, not transition to Idle
        assert_eq!(session.status, SessionStatus::Working);
    }

    #[test]
    fn test_exited_is_terminal() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.transition_status(SessionStatus::Exited(0));
        assert_eq!(session.exit_code, Some(0));
        // Further transitions should be blocked
        session.transition_status(SessionStatus::Working);
        assert!(matches!(session.status, SessionStatus::Exited(0)));
    }

    #[test]
    fn test_manual_rename() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.set_name("my-custom-name".to_string(), true);
        assert_eq!(session.display_name, "my-custom-name");
        assert!(session.is_name_manual);

        // Auto-name should not override manual name
        session.set_auto_name("auto-detected".to_string());
        assert_eq!(session.display_name, "my-custom-name");
        assert_eq!(session.auto_name, "auto-detected");
    }

    #[test]
    fn test_auto_name_updates_display() {
        let mut session = Session::new(1, 42, PathBuf::from("/tmp/test"));
        session.set_auto_name("detected-repo".to_string());
        assert_eq!(session.display_name, "detected-repo");
        assert_eq!(session.group_id, "detected-repo");
    }

    #[test]
    fn test_status_indicators() {
        assert_eq!(SessionStatus::Working.label(), "working");
        assert_eq!(SessionStatus::Waiting.indicator(), "⏳");
        assert_eq!(SessionStatus::Done.indicator(), "✓");
        assert_eq!(SessionStatus::Exited(1).indicator(), "✗");
        assert_eq!(SessionStatus::Exited(0).indicator(), "✓");
    }
}
