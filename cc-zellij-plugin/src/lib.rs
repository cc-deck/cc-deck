// cc-deck shared types for controller and sidebar plugins
//
// New protocol types used for controller-sidebar communication.
// Existing modules (session, config, etc.) remain in the binary crate (main.rs)
// until Phase 2+ migrates them here.

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Render payload: controller -> sidebar via cc-deck:render pipe
// ---------------------------------------------------------------------------

/// Pre-computed display data for a single session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RenderSession {
    pub pane_id: u32,
    pub display_name: String,
    pub activity_label: String,
    pub indicator: String,
    pub color: (u8, u8, u8),
    pub git_branch: Option<String>,
    pub tab_index: usize,
    pub paused: bool,
    pub done_attended: bool,
}

/// Complete render payload broadcast by the controller to all sidebars.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RenderPayload {
    pub sessions: Vec<RenderSession>,
    pub focused_pane_id: Option<u32>,
    pub active_tab_index: usize,
    pub notification: Option<String>,
    #[serde(default)]
    pub notification_expiry: Option<u64>,
    pub total: usize,
    pub waiting: usize,
    pub working: usize,
    pub idle: usize,
    pub controller_plugin_id: u32,
    #[serde(default)]
    pub voice_connected: bool,
    #[serde(default)]
    pub voice_muted: bool,
}

// ---------------------------------------------------------------------------
// Action message: sidebar -> controller via cc-deck:action pipe
// ---------------------------------------------------------------------------

/// Types of actions a sidebar can request from the controller.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ActionType {
    Switch,
    Rename,
    Delete,
    Pause,
    Attend,
    AttendPrev,
    Working,
    WorkingPrev,
    Navigate,
    NewSession,
    Refresh,
    VoiceMute,
}

/// A user-initiated action sent from a sidebar to the controller.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ActionMessage {
    pub action: ActionType,
    pub pane_id: Option<u32>,
    pub tab_index: Option<usize>,
    pub value: Option<String>,
    pub sidebar_plugin_id: u32,
}

// ---------------------------------------------------------------------------
// Sidebar discovery protocol
// ---------------------------------------------------------------------------

/// Sent from sidebar to controller during registration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SidebarHello {
    pub plugin_id: u32,
}

/// Sent from controller to sidebar with tab assignment.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SidebarInit {
    pub tab_index: usize,
    pub controller_plugin_id: u32,
}

#[cfg(test)]
mod protocol_tests {
    use super::*;

    #[test]
    fn test_render_payload_roundtrip() {
        let payload = RenderPayload {
            sessions: vec![RenderSession {
                pane_id: 1,
                display_name: "api-server".into(),
                activity_label: "Working".into(),
                indicator: "●".into(),
                color: (180, 140, 255),
                git_branch: Some("main".into()),
                tab_index: 0,
                paused: false,
                done_attended: false,
            }],
            focused_pane_id: Some(1),
            active_tab_index: 0,
            notification: None,
            notification_expiry: None,
            total: 1,
            waiting: 0,
            working: 1,
            idle: 0,
            controller_plugin_id: 42,
            voice_connected: false,
            voice_muted: false,
        };
        let json = serde_json::to_string(&payload).unwrap();
        let restored: RenderPayload = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.sessions.len(), 1);
        assert_eq!(restored.sessions[0].pane_id, 1);
        assert_eq!(restored.sessions[0].display_name, "api-server");
        assert_eq!(restored.controller_plugin_id, 42);
    }

    #[test]
    fn test_action_message_roundtrip() {
        let msg = ActionMessage {
            action: ActionType::Switch,
            pane_id: Some(5),
            tab_index: Some(2),
            value: None,
            sidebar_plugin_id: 10,
        };
        let json = serde_json::to_string(&msg).unwrap();
        let restored: ActionMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.pane_id, Some(5));
        assert_eq!(restored.sidebar_plugin_id, 10);
    }

    #[test]
    fn test_action_message_rename_roundtrip() {
        let msg = ActionMessage {
            action: ActionType::Rename,
            pane_id: Some(3),
            tab_index: None,
            value: Some("my-session".into()),
            sidebar_plugin_id: 7,
        };
        let json = serde_json::to_string(&msg).unwrap();
        let restored: ActionMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.value, Some("my-session".into()));
    }

    #[test]
    fn test_sidebar_hello_roundtrip() {
        let hello = SidebarHello { plugin_id: 99 };
        let json = serde_json::to_string(&hello).unwrap();
        let restored: SidebarHello = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.plugin_id, 99);
    }

    #[test]
    fn test_sidebar_init_roundtrip() {
        let init = SidebarInit {
            tab_index: 3,
            controller_plugin_id: 42,
        };
        let json = serde_json::to_string(&init).unwrap();
        let restored: SidebarInit = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.tab_index, 3);
        assert_eq!(restored.controller_plugin_id, 42);
    }

    #[test]
    fn test_render_payload_voice_fields() {
        let payload = RenderPayload {
            sessions: vec![],
            focused_pane_id: None,
            active_tab_index: 0,
            notification: None,
            notification_expiry: None,
            total: 0,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
            voice_connected: true,
            voice_muted: false,
        };
        let json = serde_json::to_string(&payload).unwrap();
        let restored: RenderPayload = serde_json::from_str(&json).unwrap();
        assert!(restored.voice_connected);
        assert!(!restored.voice_muted);
    }

    #[test]
    fn test_render_payload_voice_backwards_compat() {
        // Deserialize payload without voice fields (defaults to false)
        let json = r#"{"sessions":[],"focused_pane_id":null,"active_tab_index":0,"notification":null,"notification_expiry":null,"total":0,"waiting":0,"working":0,"idle":0,"controller_plugin_id":1}"#;
        let restored: RenderPayload = serde_json::from_str(json).unwrap();
        assert!(!restored.voice_connected);
        assert!(!restored.voice_muted);
    }

    #[test]
    fn test_render_payload_empty_sessions() {
        let payload = RenderPayload {
            sessions: vec![],
            focused_pane_id: None,
            active_tab_index: 0,
            notification: Some("No sessions".into()),
            notification_expiry: Some(1000),
            total: 0,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
            voice_connected: false,
            voice_muted: false,
        };
        let json = serde_json::to_string(&payload).unwrap();
        let restored: RenderPayload = serde_json::from_str(&json).unwrap();
        assert!(restored.sessions.is_empty());
        assert_eq!(restored.notification, Some("No sessions".into()));
    }

    #[test]
    fn test_all_action_types_serialize() {
        let types = vec![
            ActionType::Switch,
            ActionType::Rename,
            ActionType::Delete,
            ActionType::Pause,
            ActionType::Attend,
            ActionType::AttendPrev,
            ActionType::Working,
            ActionType::WorkingPrev,
            ActionType::Navigate,
            ActionType::NewSession,
            ActionType::Refresh,
            ActionType::VoiceMute,
        ];
        for action in types {
            let msg = ActionMessage {
                action,
                pane_id: None,
                tab_index: None,
                value: None,
                sidebar_plugin_id: 1,
            };
            let json = serde_json::to_string(&msg).unwrap();
            let _: ActionMessage = serde_json::from_str(&json).unwrap();
        }
    }
}
