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
    /// Unknown message.
    Unknown,
}

/// Parse a pipe message name into an action.
pub fn parse_pipe_message(name: &str, payload: Option<&str>) -> PipeAction {
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
        "cc-deck:sync" => {
            PipeAction::SyncState(payload.unwrap_or("").to_string())
        }
        "cc-deck:request" => PipeAction::RequestState,
        "cc-deck:attend" => PipeAction::Attend,
        "cc-deck:rename" => PipeAction::Rename,
        "cc-deck:new" => PipeAction::NewSession,
        "cc-deck:navigate" | "navigate" => PipeAction::Navigate,
        "cc-deck:dump-state" => PipeAction::DumpState,
        _ => PipeAction::Unknown,
    }
}

/// Map a Claude Code hook event name to an Activity state.
/// Returns None for events that should not change the activity (e.g., Notification).
pub fn hook_event_to_activity(event: &str, tool_name: Option<&str>) -> Option<Activity> {
    match event {
        "SessionStart" => Some(Activity::Init),
        "PreToolUse" => {
            let name = tool_name.unwrap_or("").to_string();
            Some(Activity::ToolUse(name))
        }
        "PostToolUse" | "PostToolUseFailure" | "UserPromptSubmit" => Some(Activity::Working),
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
        assert_eq!(hook_event_to_activity("PreToolUse", Some("Bash")), Some(Activity::ToolUse("Bash".into())));
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
    fn test_parse_malformed_hook() {
        assert!(matches!(parse_pipe_message("cc-deck:hook", Some("not json")), PipeAction::Unknown));
        assert!(matches!(parse_pipe_message("cc-deck:hook", None), PipeAction::Unknown));
    }
}
