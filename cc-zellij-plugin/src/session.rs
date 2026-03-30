// T002: Session struct with Activity enum and state transition logic

use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

pub fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

pub fn unix_now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as u64)
        .unwrap_or(0)
}

/// Why a session is in a waiting state.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum WaitReason {
    /// Session blocked on user permission decision (critical, blocking).
    Permission,
    /// Session paused with informational notification (soft, non-blocking).
    Notification,
}

/// Current activity state of a Claude Code session.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub enum Activity {
    #[default]
    Init,
    Working,
    Waiting(WaitReason),
    Idle,
    Done,
    AgentDone,
}


impl Activity {
    /// Activity indicator character for sidebar rendering.
    pub fn indicator(&self) -> &str {
        match self {
            Activity::Init | Activity::Idle => "○",
            Activity::Working => "●",
            Activity::Waiting(_) => "⚠",
            Activity::Done | Activity::AgentDone => "✓",
        }
    }

    /// RGB color for the activity indicator.
    pub fn color(&self) -> (u8, u8, u8) {
        match self {
            Activity::Init | Activity::Idle => (180, 175, 195),
            Activity::Working => (180, 140, 255),
            Activity::Waiting(WaitReason::Permission) => (255, 60, 60),
            Activity::Waiting(WaitReason::Notification) => (255, 180, 60),
            Activity::Done => (80, 200, 120),
            Activity::AgentDone => (80, 180, 100),
        }
    }

    /// Whether this state represents "needs human attention".
    pub fn is_waiting(&self) -> bool {
        matches!(self, Activity::Waiting(_))
    }
}

/// A single Claude Code session running in a Zellij tab.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub pane_id: u32,
    pub session_id: String,
    pub display_name: String,
    pub activity: Activity,
    pub tab_index: Option<usize>,
    pub tab_name: Option<String>,
    pub working_dir: Option<String>,
    pub git_branch: Option<String>,
    pub last_event_ts: u64,
    pub manually_renamed: bool,
    #[serde(default)]
    pub paused: bool,
    /// Timestamp of last user metadata change (rename, pause toggle).
    /// Separate from last_event_ts which tracks hook events.
    #[serde(default)]
    pub meta_ts: u64,
    /// Whether this Done/AgentDone session has been attended already.
    /// Once attended, it drops to the Idle tier for subsequent attend presses.
    /// Reset when activity transitions away from Done/AgentDone.
    #[serde(default)]
    pub done_attended: bool,
    /// Whether the tab rename was deferred because tab_index was not yet
    /// available when the display name was first set. Cleared after the
    /// rename is issued in rebuild_pane_map.
    #[serde(default)]
    pub pending_tab_rename: bool,
}

impl Session {
    pub fn new(pane_id: u32, session_id: String) -> Self {
        Self {
            pane_id,
            session_id,
            display_name: format!("session-{pane_id}"),
            activity: Activity::Init,
            tab_index: None,
            tab_name: None,
            working_dir: None,
            git_branch: None,
            last_event_ts: unix_now(),
            manually_renamed: false,
            paused: false,
            meta_ts: 0,
            done_attended: false,
            pending_tab_rename: false,
        }
    }

    /// Transition to a new activity state based on a hook event.
    /// Returns true if the state actually changed.
    pub fn transition(&mut self, new_activity: Activity) -> bool {
        // Waiting can only transition to Working or Done, not back to Idle
        if matches!(self.activity, Activity::Waiting(_)) {
            match new_activity {
                Activity::Working | Activity::Done | Activity::AgentDone => {}
                // Allow upgrading Notification wait to Permission wait
                Activity::Waiting(_) => {}
                _ => return false,
            }
        }

        // Suppress AgentDone when the session is actively Working.
        // SubagentStop fires when a subagent finishes, but the parent
        // agent typically continues immediately with the next tool call.
        // Transitioning to AgentDone here causes a brief green checkmark
        // flicker before the next PreToolUse arrives. Only show AgentDone
        // when the session is idle/init/done (i.e., the subagent was the
        // final action).
        if matches!(new_activity, Activity::AgentDone) && matches!(self.activity, Activity::Working) {
            self.last_event_ts = unix_now();
            return false;
        }

        if self.activity != new_activity {
            // Reset done_attended when leaving Done/AgentDone
            if matches!(self.activity, Activity::Done | Activity::AgentDone)
                && !matches!(new_activity, Activity::Done | Activity::AgentDone)
            {
                self.done_attended = false;
            }
            self.activity = new_activity;
            self.last_event_ts = unix_now();
            true
        } else {
            self.last_event_ts = unix_now();
            false
        }
    }

    /// Elapsed seconds since last activity change.
    pub fn elapsed_secs(&self) -> u64 {
        unix_now().saturating_sub(self.last_event_ts)
    }
}

/// Generate a unique display name by appending a numeric suffix if needed.
pub fn deduplicate_name(base: &str, existing_names: &[&str]) -> String {
    if !existing_names.contains(&base) {
        return base.to_string();
    }
    for i in 2.. {
        let candidate = format!("{base}-{i}");
        if !existing_names.iter().any(|n| *n == candidate) {
            return candidate;
        }
    }
    unreachable!()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_activity_indicators() {
        assert_eq!(Activity::Working.indicator(), "●");
        assert_eq!(Activity::Waiting(WaitReason::Permission).indicator(), "⚠");
        assert_eq!(Activity::Idle.indicator(), "○");
        assert_eq!(Activity::Done.indicator(), "✓");
    }

    #[test]
    fn test_waiting_transition_restrictions() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Waiting(WaitReason::Permission);
        session.last_event_ts = unix_now();

        // Waiting should not transition to Idle
        assert!(!session.transition(Activity::Idle));
        assert_eq!(session.activity, Activity::Waiting(WaitReason::Permission));

        // Waiting should transition to Working
        assert!(session.transition(Activity::Working));
        assert_eq!(session.activity, Activity::Working);
    }

    #[test]
    fn test_waiting_transition_to_done() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Waiting(WaitReason::Permission);
        session.last_event_ts = unix_now();

        assert!(session.transition(Activity::Done));
        assert_eq!(session.activity, Activity::Done);
    }

    #[test]
    fn test_agent_done_suppressed_when_working() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Working;

        // AgentDone should be suppressed when Working
        assert!(!session.transition(Activity::AgentDone));
        assert_eq!(session.activity, Activity::Working);
    }

    #[test]
    fn test_agent_done_allowed_when_idle() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Idle;

        // AgentDone should be allowed when Idle
        assert!(session.transition(Activity::AgentDone));
        assert_eq!(session.activity, Activity::AgentDone);
    }

    #[test]
    fn test_deduplicate_name() {
        assert_eq!(deduplicate_name("api", &[]), "api");
        assert_eq!(deduplicate_name("api", &["api"]), "api-2");
        assert_eq!(deduplicate_name("api", &["api", "api-2"]), "api-3");
    }

    #[test]
    fn test_deduplicate_name_no_conflict() {
        assert_eq!(deduplicate_name("frontend", &["api", "backend"]), "frontend");
    }
}
