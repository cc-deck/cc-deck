// T006-T007: Pipe message parsing and hook event to Activity mapping

use crate::session::{Activity, WaitReason};
use serde::Deserialize;

/// Hook event payload received from `cc-deck hook` CLI via pipe.
#[derive(Debug, Deserialize)]
pub struct HookPayload {
    pub session_id: Option<String>,
    pub pane_id: u32,
    pub hook_event_name: String,
    pub tool_name: Option<String>,
    pub cwd: Option<String>,
}

/// Pipe message types that the plugin handles.
pub enum PipeAction {
    /// Hook event from CLI (cc-deck:hook).
    HookEvent(HookPayload),
    /// State sync from another instance (cc-deck:sync).
    SyncState(String),
    /// State request from a new instance (cc-deck:request).
    RequestState,
    /// Attend action (cc-deck:attend).
    Attend,
    /// Rename action (cc-deck:rename).
    Rename,
    /// New session action (cc-deck:new).
    NewSession,
    /// Navigate action - toggle sidebar navigation mode (cc-deck:navigate).
    Navigate,
    /// Dump state action - serialize all sessions for CLI (cc-deck:dump-state).
    DumpState,
    /// Restore metadata overrides from snapshot (cc-deck:restore-meta).
    RestoreMeta(String),
    /// Toggle navigation mode (cc-deck:nav-toggle).
    NavToggle,
    /// Move cursor up in navigation mode (cc-deck:nav-up).
    NavUp,
    /// Move cursor down in navigation mode (cc-deck:nav-down).
    NavDown,
    /// Select session at cursor in navigation mode (cc-deck:nav-select).
    NavSelect,
    /// Toggle pause on cursor session (cc-deck:pause).
    Pause,
    /// Toggle help overlay (cc-deck:help).
    Help,
    /// Navigate previous - enter navigation or move cursor up (cc-deck:navigate-prev).
    NavigatePrev,
    /// Attend previous - reverse-cycle through attend tiers (cc-deck:attend-prev).
    AttendPrev,
    /// Cycle through working sessions (cc-deck:working).
    Working,
    /// Cycle through working sessions in reverse (cc-deck:working-prev).
    WorkingPrev,
    /// Force-refresh state: clear caches, broadcast active instance's state (cc-deck:refresh).
    Refresh,
    /// Voice text to inject into attended pane (cc-deck:voice).
    VoiceText(String),
    /// Voice control long-poll connection (cc-deck:voice-control).
    VoiceControl,
    /// Voice toggle from F8 keybinding (cc-deck:voice-toggle).
    VoiceToggle,
    /// Diagnostic: inject hardcoded text into focused pane (cc-deck:test-inject).
    TestInject,
    /// Unknown message.
    Unknown,
}

/// Parse a pipe message name into an action.
/// Handles PID-scoped sync/request messages (cc-deck:sync:{pid}, cc-deck:request:{pid})
/// as well as legacy names without PID suffix.
pub fn parse_pipe_message(name: &str, payload: Option<&str>) -> PipeAction {
    // Check PID-scoped sync/request messages first
    if crate::sync::is_sync_message(name) {
        return PipeAction::SyncState(payload.unwrap_or("").to_string());
    }
    if crate::sync::is_request_message(name) {
        return PipeAction::RequestState;
    }

    match name {
        "cc-deck:hook" => {
            if let Some(payload_str) = payload {
                match serde_json::from_str::<HookPayload>(payload_str) {
                    Ok(hook) => PipeAction::HookEvent(hook),
                    Err(_) => PipeAction::Unknown,
                }
            } else {
                PipeAction::Unknown
            }
        }
        "cc-deck:attend" => PipeAction::Attend,
        "cc-deck:rename" => PipeAction::Rename,
        "cc-deck:new" => PipeAction::NewSession,
        "cc-deck:navigate" | "navigate" => PipeAction::Navigate,
        "cc-deck:dump-state" => PipeAction::DumpState,
        "cc-deck:restore-meta" => {
            PipeAction::RestoreMeta(payload.unwrap_or("").to_string())
        }
        "cc-deck:nav-toggle" => PipeAction::NavToggle,
        "cc-deck:nav-up" => PipeAction::NavUp,
        "cc-deck:nav-down" => PipeAction::NavDown,
        "cc-deck:nav-select" => PipeAction::NavSelect,
        "cc-deck:pause" => PipeAction::Pause,
        "cc-deck:help" => PipeAction::Help,
        "cc-deck:navigate-prev" => PipeAction::NavigatePrev,
        "cc-deck:attend-prev" => PipeAction::AttendPrev,
        "cc-deck:working" => PipeAction::Working,
        "cc-deck:working-prev" => PipeAction::WorkingPrev,
        "cc-deck:refresh" => PipeAction::Refresh,
        "cc-deck:voice" => PipeAction::VoiceText(payload.unwrap_or("").to_string()),
        "cc-deck:voice-control" => PipeAction::VoiceControl,
        "cc-deck:voice-toggle" => PipeAction::VoiceToggle,
        "cc-deck:test-inject" => PipeAction::TestInject,
        _ => PipeAction::Unknown,
    }
}

/// Map a Claude Code hook event name to an Activity state.
/// Returns None for events that should not change the activity (e.g., Notification).
pub fn hook_event_to_activity(event: &str, _tool_name: Option<&str>) -> Option<Activity> {
    match event {
        "SessionStart" => Some(Activity::Init),
        "PreToolUse" | "PostToolUse" | "PostToolUseFailure" | "UserPromptSubmit" | "SubagentStart" => Some(Activity::Working),
        "PermissionRequest" => Some(Activity::Waiting(WaitReason::Permission)),
        "Stop" => Some(Activity::Done),
        "SubagentStop" => Some(Activity::AgentDone),
        // Notification is informational (e.g., "task complete"), not a blocking state.
        // Just refresh the timestamp, don't change activity.
        "Notification" => None,
        // SessionEnd is handled separately (removes session entirely)
        "SessionEnd" => None,
        _ => None,
    }
}

/// Check if a hook event should remove the session entirely.
pub fn is_session_end(event: &str) -> bool {
    event == "SessionEnd"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_hook_payload() {
        let json = r#"{"session_id":"abc","pane_id":42,"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/tmp"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert_eq!(payload.pane_id, 42);
        assert_eq!(payload.hook_event_name, "PreToolUse");
        assert_eq!(payload.tool_name.as_deref(), Some("Bash"));
    }

    #[test]
    fn test_parse_hook_payload_minimal() {
        let json = r#"{"pane_id":1,"hook_event_name":"Stop"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert_eq!(payload.pane_id, 1);
        assert_eq!(payload.hook_event_name, "Stop");
        assert!(payload.session_id.is_none());
        assert!(payload.tool_name.is_none());
    }

    #[test]
    fn test_hook_event_to_activity() {
        assert_eq!(hook_event_to_activity("SessionStart", None), Some(Activity::Init));
        assert_eq!(hook_event_to_activity("PreToolUse", Some("Bash")), Some(Activity::Working));
        assert_eq!(hook_event_to_activity("PostToolUse", None), Some(Activity::Working));
        assert_eq!(hook_event_to_activity("PermissionRequest", None), Some(Activity::Waiting(WaitReason::Permission)));
        assert_eq!(hook_event_to_activity("Stop", None), Some(Activity::Done));
        assert_eq!(hook_event_to_activity("SubagentStop", None), Some(Activity::AgentDone));
        assert_eq!(hook_event_to_activity("Notification", None), None);
        assert_eq!(hook_event_to_activity("SessionEnd", None), None);
    }

    #[test]
    fn test_is_session_end() {
        assert!(is_session_end("SessionEnd"));
        assert!(!is_session_end("Stop"));
        assert!(!is_session_end("SessionStart"));
    }

    #[test]
    fn test_parse_pipe_message() {
        let json = r#"{"pane_id":1,"hook_event_name":"Stop"}"#;
        match parse_pipe_message("cc-deck:hook", Some(json)) {
            PipeAction::HookEvent(h) => assert_eq!(h.hook_event_name, "Stop"),
            _ => panic!("expected HookEvent"),
        }

        assert!(matches!(parse_pipe_message("cc-deck:request", None), PipeAction::RequestState));
        assert!(matches!(parse_pipe_message("cc-deck:attend", None), PipeAction::Attend));
        assert!(matches!(parse_pipe_message("cc-deck:new", None), PipeAction::NewSession));
        assert!(matches!(parse_pipe_message("cc-deck:dump-state", None), PipeAction::DumpState));
        assert!(matches!(parse_pipe_message("unknown", None), PipeAction::Unknown));
    }

    #[test]
    fn test_parse_nav_and_control_commands() {
        assert!(matches!(parse_pipe_message("cc-deck:nav-toggle", None), PipeAction::NavToggle));
        assert!(matches!(parse_pipe_message("cc-deck:nav-up", None), PipeAction::NavUp));
        assert!(matches!(parse_pipe_message("cc-deck:nav-down", None), PipeAction::NavDown));
        assert!(matches!(parse_pipe_message("cc-deck:nav-select", None), PipeAction::NavSelect));
        assert!(matches!(parse_pipe_message("cc-deck:pause", None), PipeAction::Pause));
        assert!(matches!(parse_pipe_message("cc-deck:help", None), PipeAction::Help));
        assert!(matches!(parse_pipe_message("cc-deck:navigate-prev", None), PipeAction::NavigatePrev));
        assert!(matches!(parse_pipe_message("cc-deck:attend-prev", None), PipeAction::AttendPrev));
    }

    #[test]
    fn test_parse_refresh_command() {
        assert!(matches!(parse_pipe_message("cc-deck:refresh", None), PipeAction::Refresh));
    }

    #[test]
    fn test_parse_pid_scoped_sync_message() {
        // PID-scoped sync messages should parse as SyncState
        let payload = r#"{"1":{"pane_id":1}}"#;
        assert!(matches!(
            parse_pipe_message("cc-deck:sync:12345", Some(payload)),
            PipeAction::SyncState(_)
        ));
        // PID-scoped request messages should parse as RequestState
        assert!(matches!(
            parse_pipe_message("cc-deck:request:12345", None),
            PipeAction::RequestState
        ));
    }

    #[test]
    fn test_parse_malformed_hook() {
        assert!(matches!(parse_pipe_message("cc-deck:hook", Some("not json")), PipeAction::Unknown));
        assert!(matches!(parse_pipe_message("cc-deck:hook", None), PipeAction::Unknown));
    }

    #[test]
    fn test_parse_voice_commands() {
        match parse_pipe_message("cc-deck:voice", Some("hello world")) {
            PipeAction::VoiceText(text) => assert_eq!(text, "hello world"),
            _ => panic!("expected VoiceText"),
        }
        match parse_pipe_message("cc-deck:voice", None) {
            PipeAction::VoiceText(text) => assert_eq!(text, ""),
            _ => panic!("expected VoiceText with empty payload"),
        }
        assert!(matches!(parse_pipe_message("cc-deck:voice-control", None), PipeAction::VoiceControl));
        assert!(matches!(parse_pipe_message("cc-deck:voice-control", Some("listen")), PipeAction::VoiceControl));
        assert!(matches!(parse_pipe_message("cc-deck:voice-toggle", None), PipeAction::VoiceToggle));
    }

    #[test]
    fn test_parse_test_inject() {
        assert!(matches!(parse_pipe_message("cc-deck:test-inject", None), PipeAction::TestInject));
    }
}
