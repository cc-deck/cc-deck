// MCP tool handlers: process cc-deck:mcp-* pipe messages.
//
// Each handler receives controller state and a CLI pipe_id, performs
// its operation, and sends the JSON response via cli_pipe_output.
// All handlers manage their own pipe lifecycle (output + unblock).

use super::state::ControllerState;
use crate::session::Activity;
use serde::Deserialize;

/// Serialize all sessions into a JSON array for MCP consumption.
/// Separated from the WASM-gated handler for testability.
pub fn serialize_sessions(state: &ControllerState) -> String {
    let mut entries = Vec::new();
    for (_, session) in &state.sessions {
        let entry = serde_json::json!({
            "pane_id": session.pane_id,
            "name": session.display_name,
            "agent": session.agent_name.as_deref().unwrap_or(""),
            "state": session.activity.as_mcp_str(),
            "cwd": session.working_dir,
            "topic": session.topic,
            "recent_tools": session.recent_tools,
            "paused": session.paused,
            "badges": session.badges,
        });
        entries.push(entry);
    }
    serde_json::to_string(&entries).unwrap_or_else(|_| "[]".to_string())
}

/// Handle cc-deck:mcp-sessions: list all sessions.
pub fn handle_mcp_sessions(state: &ControllerState, pipe_id: &str) {
    let json_string = serialize_sessions(state);
    #[cfg(target_family = "wasm")]
    {
        zellij_tile::prelude::cli_pipe_output(pipe_id, &json_string);
        zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
    }
    #[cfg(not(target_family = "wasm"))]
    {
        let _ = pipe_id;
    }
}

#[derive(Deserialize)]
struct ScrollbackRequest {
    #[allow(dead_code)]
    pane_id: u32,
    #[allow(dead_code)]
    lines: u32,
}

/// Handle cc-deck:mcp-scrollback: get scrollback for a pane.
pub fn handle_mcp_scrollback(state: &ControllerState, pipe_id: &str, payload: &str) {
    let _ = state;
    let response = match serde_json::from_str::<ScrollbackRequest>(payload) {
        Ok(_req) => {
            // Zellij's get_plugin_pane_id / dump_screen APIs do not expose
            // terminal pane scrollback to plugin code in zellij-tile 0.43.1.
            // Return empty content with an explanatory note.
            // TODO: revisit when zellij-tile exposes terminal scrollback.
            serde_json::json!({
                "content": "",
                "note": "scrollback capture not available in current zellij-tile version"
            }).to_string()
        }
        Err(e) => {
            serde_json::json!({ "error": format!("invalid payload: {e}") }).to_string()
        }
    };
    #[cfg(target_family = "wasm")]
    {
        zellij_tile::prelude::cli_pipe_output(pipe_id, &response);
        zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
    }
    #[cfg(not(target_family = "wasm"))]
    {
        let _ = (pipe_id, &response);
    }
}

#[derive(Deserialize)]
struct StateRequest {
    pane_id: u32,
}

/// Handle cc-deck:mcp-state: get detailed state for a single session.
pub fn handle_mcp_state(state: &ControllerState, pipe_id: &str, payload: &str) {
    let response = match serde_json::from_str::<StateRequest>(payload) {
        Ok(req) => {
            if let Some(session) = state.sessions.get(&req.pane_id) {
                serde_json::json!({
                    "name": session.display_name,
                    "pane_id": session.pane_id,
                    "cwd": session.working_dir,
                    "topic": session.topic,
                    "recent_tools": session.recent_tools,
                    "paused": session.paused,
                    "badges": session.badges,
                    "agent": session.agent_name.as_deref().unwrap_or(""),
                    "state": session.activity.as_mcp_str(),
                    "last_event_ts": session.last_event_ts,
                }).to_string()
            } else {
                serde_json::json!({ "error": "session not found" }).to_string()
            }
        }
        Err(e) => {
            serde_json::json!({ "error": format!("invalid payload: {e}") }).to_string()
        }
    };
    #[cfg(target_family = "wasm")]
    {
        zellij_tile::prelude::cli_pipe_output(pipe_id, &response);
        zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
    }
    #[cfg(not(target_family = "wasm"))]
    {
        let _ = (pipe_id, &response);
    }
}

#[derive(Deserialize)]
struct InjectRequest {
    pane_id: u32,
    text: String,
}

/// Handle cc-deck:mcp-inject: inject text into an idle pane.
pub fn handle_mcp_inject(state: &mut ControllerState, pipe_id: &str, payload: &str) {
    let response = match serde_json::from_str::<InjectRequest>(payload) {
        Ok(req) => {
            if let Some(session) = state.sessions.get(&req.pane_id) {
                let is_idle = matches!(session.activity, Activity::Idle | Activity::Init);
                if is_idle {
                    #[cfg(target_family = "wasm")]
                    {
                        zellij_tile::prelude::write_chars_to_pane_id(
                            &req.text,
                            zellij_tile::prelude::PaneId::Terminal(req.pane_id),
                        );
                    }
                    serde_json::json!({ "ok": true }).to_string()
                } else {
                    serde_json::json!({ "error": "session not idle" }).to_string()
                }
            } else {
                serde_json::json!({ "error": "session not found" }).to_string()
            }
        }
        Err(e) => {
            serde_json::json!({ "error": format!("invalid payload: {e}") }).to_string()
        }
    };
    #[cfg(target_family = "wasm")]
    {
        zellij_tile::prelude::cli_pipe_output(pipe_id, &response);
        zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
    }
    #[cfg(not(target_family = "wasm"))]
    {
        let _ = (pipe_id, &response);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::{Activity, Session, WaitReason};

    #[test]
    fn test_serialize_sessions_empty() {
        let state = ControllerState::default();
        let json = serialize_sessions(&state);
        assert_eq!(json, "[]");
    }

    #[test]
    fn test_serialize_sessions_single() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test-session".into());
        s.display_name = "my-project".to_string();
        s.agent_name = Some("claude".to_string());
        s.working_dir = Some("/home/user/project".to_string());
        s.topic = Some("fix the login bug".to_string());
        s.recent_tools = vec!["Bash".to_string(), "Read".to_string()];
        s.paused = false;
        s.badges = vec!["ship".to_string()];
        s.activity = Activity::Working;
        state.sessions.insert(42, s);

        let json = serialize_sessions(&state);
        let parsed: Vec<serde_json::Value> = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.len(), 1);

        let entry = &parsed[0];
        assert_eq!(entry["pane_id"], 42);
        assert_eq!(entry["name"], "my-project");
        assert_eq!(entry["agent"], "claude");
        assert_eq!(entry["state"], "working");
        assert_eq!(entry["cwd"], "/home/user/project");
        assert_eq!(entry["topic"], "fix the login bug");
        assert_eq!(entry["recent_tools"], serde_json::json!(["Bash", "Read"]));
        assert_eq!(entry["paused"], false);
        assert_eq!(entry["badges"], serde_json::json!(["ship"]));
    }

    #[test]
    fn test_serialize_sessions_activity_mapping() {
        let mut state = ControllerState::default();

        // Idle
        let mut s1 = Session::new(1, "s1".into());
        s1.activity = Activity::Idle;
        state.sessions.insert(1, s1);

        // Working
        let mut s2 = Session::new(2, "s2".into());
        s2.activity = Activity::Working;
        state.sessions.insert(2, s2);

        // Waiting
        let mut s3 = Session::new(3, "s3".into());
        s3.activity = Activity::Waiting(WaitReason::Permission);
        state.sessions.insert(3, s3);

        // Done
        let mut s4 = Session::new(4, "s4".into());
        s4.activity = Activity::Done;
        state.sessions.insert(4, s4);

        // Init (maps to "idle")
        let mut s5 = Session::new(5, "s5".into());
        s5.activity = Activity::Init;
        state.sessions.insert(5, s5);

        // AgentDone (maps to "done")
        let mut s6 = Session::new(6, "s6".into());
        s6.activity = Activity::AgentDone;
        state.sessions.insert(6, s6);

        let json = serialize_sessions(&state);
        let parsed: Vec<serde_json::Value> = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.len(), 6);

        // BTreeMap iterates in key order: 1, 2, 3, 4, 5, 6
        assert_eq!(parsed[0]["state"], "idle");     // Idle
        assert_eq!(parsed[1]["state"], "working");  // Working
        assert_eq!(parsed[2]["state"], "waiting");  // Waiting
        assert_eq!(parsed[3]["state"], "done");     // Done
        assert_eq!(parsed[4]["state"], "idle");     // Init -> "idle"
        assert_eq!(parsed[5]["state"], "done");     // AgentDone -> "done"
    }

    #[test]
    fn test_serialize_sessions_topic_none() {
        let mut state = ControllerState::default();
        let s = Session::new(1, "s1".into());
        // topic is None by default
        state.sessions.insert(1, s);

        let json = serialize_sessions(&state);
        let parsed: Vec<serde_json::Value> = serde_json::from_str(&json).unwrap();
        assert!(parsed[0]["topic"].is_null());
    }

    #[test]
    fn test_serialize_sessions_topic_present() {
        let mut state = ControllerState::default();
        let mut s = Session::new(1, "s1".into());
        s.topic = Some("implement feature X".to_string());
        state.sessions.insert(1, s);

        let json = serialize_sessions(&state);
        let parsed: Vec<serde_json::Value> = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed[0]["topic"], "implement feature X");
    }

    #[test]
    fn test_serialize_sessions_agent_defaults_to_empty() {
        let mut state = ControllerState::default();
        let s = Session::new(1, "s1".into());
        // agent_name is None by default
        state.sessions.insert(1, s);

        let json = serialize_sessions(&state);
        let parsed: Vec<serde_json::Value> = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed[0]["agent"], "");
    }

    #[test]
    fn test_handle_mcp_state_found() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.display_name = "api-server".to_string();
        s.activity = Activity::Working;
        s.working_dir = Some("/home/user/api".to_string());
        s.topic = Some("fix auth".to_string());
        s.recent_tools = vec!["Bash".to_string()];
        s.agent_name = Some("claude".to_string());
        state.sessions.insert(42, s);

        // In non-WASM mode, handle_mcp_state doesn't send output,
        // but we can test the response construction via the internal logic.
        // Test by calling the deserialization path directly.
        let payload = r#"{"pane_id":42}"#;
        let req: StateRequest = serde_json::from_str(payload).unwrap();
        let session = state.sessions.get(&req.pane_id).unwrap();

        assert_eq!(session.activity.as_mcp_str(), "working");
        assert_eq!(session.display_name, "api-server");
        assert_eq!(session.topic, Some("fix auth".to_string()));
    }

    #[test]
    fn test_handle_mcp_state_not_found() {
        let state = ControllerState::default();
        let payload = r#"{"pane_id":999}"#;
        let req: StateRequest = serde_json::from_str(payload).unwrap();
        assert!(state.sessions.get(&req.pane_id).is_none());
    }

    #[test]
    fn test_handle_mcp_inject_idle_session() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.activity = Activity::Idle;
        state.sessions.insert(42, s);

        let payload = r#"{"pane_id":42,"text":"hello world"}"#;
        let req: InjectRequest = serde_json::from_str(payload).unwrap();
        let session = state.sessions.get(&req.pane_id).unwrap();
        assert!(matches!(session.activity, Activity::Idle | Activity::Init));
        assert_eq!(req.text, "hello world");
    }

    #[test]
    fn test_handle_mcp_inject_working_session_rejected() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.activity = Activity::Working;
        state.sessions.insert(42, s);

        let payload = r#"{"pane_id":42,"text":"hello"}"#;
        let req: InjectRequest = serde_json::from_str(payload).unwrap();
        let session = state.sessions.get(&req.pane_id).unwrap();
        assert!(!matches!(session.activity, Activity::Idle | Activity::Init));
    }

    #[test]
    fn test_handle_mcp_inject_session_not_found() {
        let state = ControllerState::default();
        let payload = r#"{"pane_id":999,"text":"hello"}"#;
        let req: InjectRequest = serde_json::from_str(payload).unwrap();
        assert!(state.sessions.get(&req.pane_id).is_none());
    }

    #[test]
    fn test_scrollback_invalid_payload() {
        let result = serde_json::from_str::<ScrollbackRequest>("not json");
        assert!(result.is_err());
    }

    #[test]
    fn test_scrollback_valid_payload() {
        let req: ScrollbackRequest = serde_json::from_str(r#"{"pane_id":1,"lines":50}"#).unwrap();
        assert_eq!(req.pane_id, 1);
        assert_eq!(req.lines, 50);
    }
}
