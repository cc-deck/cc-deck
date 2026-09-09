// T006-T007: Pipe message parsing and hook event to Activity mapping

use crate::session::{Activity, WaitReason};
use serde::Deserialize;

/// Hook event payload received from `cc-deck hook` CLI via pipe.
#[derive(Debug, Deserialize)]
pub struct HookPayload {
    #[serde(default)]
    pub agent: Option<String>,
    #[serde(default)]
    pub agent_indicator: Option<String>,
    pub session_id: Option<String>,
    pub pane_id: u32,
    pub hook_event_name: String,
    pub tool_name: Option<String>,
    pub cwd: Option<String>,
    pub agent_id: Option<String>,
    #[serde(default)]
    pub badges: Vec<String>,
    #[serde(default)]
    pub profile: Option<String>,
    #[serde(default)]
    pub profile_color: Option<String>,
}

/// Pipe message types that the plugin handles.
pub enum PipeAction {
    /// Hook event from CLI (cc-deck:hook).
    HookEvent(Box<HookPayload>),
    /// Attend action (cc-deck:attend).
    Attend,
    /// New session action (cc-deck:new).
    NewSession,
    /// Navigate action - toggle sidebar navigation mode (cc-deck:navigate).
    Navigate,
    /// Dump state action - serialize all sessions for CLI (cc-deck:dump-state).
    DumpState,
    /// Restore metadata overrides from snapshot (cc-deck:restore-meta).
    RestoreMeta(String),
    /// Toggle pause on the focused session (cc-deck:pause).
    Pause,
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
    /// Voice mute toggle from keybinding (cc-deck:voice-mute-toggle).
    VoiceMuteToggle,
    /// Sidebar requests initial render from controller (cc-deck:render-request).
    RenderRequest(u32),
    /// Unknown message.
    Unknown,
}

/// Parse a pipe message name into an action.
pub fn parse_pipe_message(name: &str, payload: Option<&str>) -> PipeAction {
    match name {
        "cc-deck:hook" => {
            if let Some(payload_str) = payload {
                match serde_json::from_str::<HookPayload>(payload_str) {
                    Ok(hook) => PipeAction::HookEvent(Box::new(hook)),
                    Err(_) => PipeAction::Unknown,
                }
            } else {
                PipeAction::Unknown
            }
        }
        "cc-deck:attend" => PipeAction::Attend,
        "cc-deck:new" => PipeAction::NewSession,
        "cc-deck:navigate" | "navigate" => PipeAction::Navigate,
        "cc-deck:dump-state" => PipeAction::DumpState,
        "cc-deck:restore-meta" => {
            PipeAction::RestoreMeta(payload.unwrap_or("").to_string())
        }
        "cc-deck:pause" => PipeAction::Pause,
        "cc-deck:navigate-prev" => PipeAction::NavigatePrev,
        "cc-deck:attend-prev" => PipeAction::AttendPrev,
        "cc-deck:working" => PipeAction::Working,
        "cc-deck:working-prev" => PipeAction::WorkingPrev,
        "cc-deck:refresh" => PipeAction::Refresh,
        "cc-deck:voice" => PipeAction::VoiceText(payload.unwrap_or("").to_string()),
        "cc-deck:voice-mute-toggle" => PipeAction::VoiceMuteToggle,
        "cc-deck:render-request" => {
            payload.and_then(|p| p.parse::<u32>().ok())
                .map(PipeAction::RenderRequest)
                .unwrap_or(PipeAction::Unknown)
        }
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
        // PermissionReply signals the user answered a permission prompt.
        // Handled specially in hooks.rs: decrements pending_permissions counter
        // and only transitions to Working when all prompts are answered.
        "PermissionReply" => Some(Activity::Working),
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
        assert_eq!(hook_event_to_activity("PermissionReply", None), Some(Activity::Working));
        assert_eq!(hook_event_to_activity("Stop", None), Some(Activity::Done));
        assert_eq!(hook_event_to_activity("SubagentStop", None), Some(Activity::AgentDone));
        assert_eq!(hook_event_to_activity("Notification", None), None);
        assert_eq!(hook_event_to_activity("SessionEnd", None), None);
    }

    #[test]
    fn test_parse_hook_payload_with_agent_id() {
        let json = r#"{"session_id":"abc","pane_id":42,"hook_event_name":"PostToolUse","tool_name":"Bash","agent_id":"sub-123"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert_eq!(payload.agent_id.as_deref(), Some("sub-123"));
    }

    #[test]
    fn test_parse_hook_payload_without_agent_id() {
        let json = r#"{"pane_id":1,"hook_event_name":"PostToolUse"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert!(payload.agent_id.is_none());
    }

    #[test]
    fn test_parse_hook_payload_with_agent() {
        let json = r#"{"pane_id":42,"hook_event_name":"PreToolUse","agent":"opencode"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert_eq!(payload.agent.as_deref(), Some("opencode"));
    }

    #[test]
    fn test_parse_hook_payload_without_agent() {
        let json = r#"{"pane_id":1,"hook_event_name":"Stop"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert!(payload.agent.is_none());
    }

    #[test]
    fn test_parse_hook_payload_with_badges() {
        let json = r#"{"pane_id":42,"hook_event_name":"PreToolUse","badges":["🚢","✅"]}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert_eq!(payload.badges, vec!["🚢", "✅"]);
    }

    #[test]
    fn test_parse_hook_payload_without_badges() {
        let json = r#"{"pane_id":1,"hook_event_name":"PostToolUse"}"#;
        let payload: HookPayload = serde_json::from_str(json).unwrap();
        assert!(payload.badges.is_empty());
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

        assert!(matches!(parse_pipe_message("cc-deck:attend", None), PipeAction::Attend));
        assert!(matches!(parse_pipe_message("cc-deck:new", None), PipeAction::NewSession));
        assert!(matches!(parse_pipe_message("cc-deck:dump-state", None), PipeAction::DumpState));
        assert!(matches!(parse_pipe_message("unknown", None), PipeAction::Unknown));
    }

    #[test]
    fn test_parse_nav_and_control_commands() {
        assert!(matches!(parse_pipe_message("cc-deck:pause", None), PipeAction::Pause));
        assert!(matches!(parse_pipe_message("cc-deck:navigate-prev", None), PipeAction::NavigatePrev));
        assert!(matches!(parse_pipe_message("cc-deck:attend-prev", None), PipeAction::AttendPrev));
    }

    #[test]
    fn test_retired_pipe_names_are_unknown() {
        // Names from the pre-controller sync protocol and sidebar-local
        // navigation are no longer part of the interface.
        for name in [
            "cc-deck:sync",
            "cc-deck:sync:12345",
            "cc-deck:request",
            "cc-deck:nav-toggle",
            "cc-deck:nav-up",
            "cc-deck:help",
            "cc-deck:rename",
            "cc-deck:test-inject",
        ] {
            assert!(
                matches!(parse_pipe_message(name, None), PipeAction::Unknown),
                "{name} should be unknown"
            );
        }
    }

    #[test]
    fn test_parse_refresh_command() {
        assert!(matches!(parse_pipe_message("cc-deck:refresh", None), PipeAction::Refresh));
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
        assert!(matches!(parse_pipe_message("cc-deck:voice-mute-toggle", None), PipeAction::VoiceMuteToggle));
        assert!(matches!(parse_pipe_message("cc-deck:voice-control", None), PipeAction::Unknown));
        assert!(matches!(parse_pipe_message("cc-deck:voice-toggle", None), PipeAction::Unknown));
    }

    #[test]
    fn test_parse_render_request() {
        match parse_pipe_message("cc-deck:render-request", Some("55")) {
            PipeAction::RenderRequest(id) => assert_eq!(id, 55),
            _ => panic!("expected RenderRequest"),
        }
        assert!(matches!(parse_pipe_message("cc-deck:render-request", None), PipeAction::Unknown));
    }

    #[test]
    fn test_focus_report_handled_before_parse() {
        // cc-deck:focus-report is handled directly in controller/mod.rs
        // before parse_pipe_message runs, so it maps to Unknown here.
        assert!(matches!(parse_pipe_message("cc-deck:focus-report", None), PipeAction::Unknown));
    }
}
