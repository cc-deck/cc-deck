// cc-deck shared types for controller and sidebar plugins
//
// New protocol types used for controller-sidebar communication.
// Existing modules (session, config, etc.) remain in the binary crate (main.rs)
// until Phase 2+ migrates them here.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use zellij_tile::prelude::PermissionType;

/// Every Zellij permission the plugin needs, in either role.
///
/// Both roles request the same set on purpose: Zellij caches grants by plugin
/// URL, and the two roles share one binary, so the sidebar's dialog is what
/// grants the background controller. The Go CLI seeds the same set into
/// Zellij's `permissions.kdl`; a test there keeps the two lists identical.
pub const REQUIRED_PERMISSIONS: [PermissionType; 7] = [
    PermissionType::ReadApplicationState,
    PermissionType::ChangeApplicationState,
    PermissionType::RunCommands,
    PermissionType::ReadCliPipes,
    PermissionType::MessageAndLaunchOtherPlugins,
    PermissionType::Reconfigure,
    PermissionType::WriteToStdin,
];

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
    #[serde(default)]
    pub badges: Vec<String>,
    #[serde(default)]
    pub agent_indicator: Option<String>,
    /// Whether this session is operating inside a `.claude/worktrees/` directory.
    #[serde(default)]
    pub in_worktree: bool,
}

/// Complete render payload broadcast by the controller to all sidebars.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RenderPayload {
    pub sessions: Vec<RenderSession>,
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
    #[serde(default)]
    pub show_agent_indicators: bool,
    #[serde(default)]
    pub sort_active: bool,
    /// Per-client presentation state. Presence, active highlighting and local
    /// ordering are all derived from this single map.
    #[serde(default)]
    pub client_views: BTreeMap<u16, ClientViewSnapshot>,
    #[serde(default)]
    pub multiplayer_colors: Option<Vec<(u8, u8, u8)>>,
}

/// Render-safe projection of a connected client's state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClientViewSnapshot {
    pub active_tab_index: Option<usize>,
    pub focused_pane_id: Option<u32>,
    pub revision: u64,
}

// ---------------------------------------------------------------------------
// Action message: sidebar -> controller via cc-deck:action pipe
// ---------------------------------------------------------------------------

/// Types of actions a sidebar can request from the controller.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ActionType {
    Rename,
    Delete,
    Pause,
    Attend,
    AttendPrev,
    Working,
    WorkingPrev,
    NewSession,
    Refresh,
    VoiceMute,
    Sort,
    MoveUp,
    MoveDown,
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
    /// The Zellij client that owns this sidebar instance.
    /// Defaults to 0 for backward compatibility with older plugin versions
    /// that do not include this field in the hello payload.
    #[serde(default)]
    pub client_id: u16,
}

/// Sent from controller to sidebar with tab assignment.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SidebarInit {
    pub tab_index: usize,
    pub controller_plugin_id: u32,
}

// ---------------------------------------------------------------------------
// Focus report: sidebar -> controller for multiplayer presence tracking
// ---------------------------------------------------------------------------

/// Sent from sidebar to controller when a client switches session focus locally.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FocusReport {
    pub client_id: u16,
    pub pane_id: u32,
    pub tab_index: usize,
}

/// Fallback multiplayer palette when ModeUpdate hasn't delivered colors yet.
/// Matches Zellij's default multiplayer_user_colors.
pub const FALLBACK_MULTIPLAYER_COLORS: [(u8, u8, u8); 10] = [
    (255, 0, 255),   // magenta
    (0, 0, 255),     // blue
    (128, 0, 128),   // purple
    (255, 255, 0),   // yellow
    (0, 255, 255),   // cyan
    (0, 255, 0),     // green
    (255, 165, 0),   // orange
    (128, 128, 128), // gray
    (255, 192, 203), // pink
    (139, 69, 19),   // brown
];

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
                badges: vec![],
                agent_indicator: None,
                in_worktree: false,
            }],
            notification: None,
            notification_expiry: None,
            total: 1,
            waiting: 0,
            working: 1,
            idle: 0,
            controller_plugin_id: 42,
            voice_connected: false,
            voice_muted: false,
            show_agent_indicators: false,
            sort_active: false,
            client_views: BTreeMap::new(),
            multiplayer_colors: None,
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
            action: ActionType::Pause,
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
        let hello = SidebarHello {
            plugin_id: 99,
            client_id: 3,
        };
        let json = serde_json::to_string(&hello).unwrap();
        let restored: SidebarHello = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.plugin_id, 99);
        assert_eq!(restored.client_id, 3);
    }

    #[test]
    fn test_sidebar_hello_backward_compat_no_client_id() {
        // Simulate an older plugin that does not include the client_id field.
        // The #[serde(default)] attribute ensures client_id defaults to 0.
        let json = r#"{"plugin_id":42}"#;
        let restored: SidebarHello = serde_json::from_str(json).unwrap();
        assert_eq!(restored.plugin_id, 42);
        assert_eq!(restored.client_id, 0);
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
            notification: None,
            notification_expiry: None,
            total: 0,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
            voice_connected: true,
            voice_muted: false,
            show_agent_indicators: false,
            sort_active: false,
            client_views: BTreeMap::new(),
            multiplayer_colors: None,
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
            notification: Some("No sessions".into()),
            notification_expiry: Some(1000),
            total: 0,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
            voice_connected: false,
            voice_muted: false,
            show_agent_indicators: false,
            sort_active: false,
            client_views: BTreeMap::new(),
            multiplayer_colors: None,
        };
        let json = serde_json::to_string(&payload).unwrap();
        let restored: RenderPayload = serde_json::from_str(&json).unwrap();
        assert!(restored.sessions.is_empty());
        assert_eq!(restored.notification, Some("No sessions".into()));
    }

    #[test]
    fn test_all_action_types_serialize() {
        let types = vec![
            ActionType::Rename,
            ActionType::Delete,
            ActionType::Pause,
            ActionType::Attend,
            ActionType::AttendPrev,
            ActionType::Working,
            ActionType::WorkingPrev,
            ActionType::NewSession,
            ActionType::Refresh,
            ActionType::VoiceMute,
            ActionType::Sort,
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

    #[test]
    fn test_focus_report_roundtrip() {
        let report = FocusReport {
            client_id: 2,
            pane_id: 42,
            tab_index: 3,
        };
        let json = serde_json::to_string(&report).unwrap();
        let restored: FocusReport = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.client_id, 2);
        assert_eq!(restored.pane_id, 42);
        assert_eq!(restored.tab_index, 3);
    }

    #[test]
    fn test_render_payload_client_views_default() {
        let json = r#"{"sessions":[],"focused_pane_id":null,"active_tab_index":0,"notification":null,"total":0,"waiting":0,"working":0,"idle":0,"controller_plugin_id":1}"#;
        let restored: RenderPayload = serde_json::from_str(json).unwrap();
        assert!(restored.client_views.is_empty());
    }

    #[test]
    fn test_render_payload_client_views_roundtrip() {
        let payload = RenderPayload {
            sessions: vec![],
            notification: None,
            notification_expiry: None,
            total: 0,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
            voice_connected: false,
            voice_muted: false,
            show_agent_indicators: false,
            sort_active: false,
            client_views: BTreeMap::from([(
                2,
                ClientViewSnapshot {
                    active_tab_index: Some(3),
                    focused_pane_id: Some(42),
                    revision: 4,
                },
            )]),
            multiplayer_colors: None,
        };
        let json = serde_json::to_string(&payload).unwrap();
        let restored: RenderPayload = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.client_views[&2].focused_pane_id, Some(42));
    }
}
