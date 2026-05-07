use super::modes::{NavigateContext, SidebarMode};
use super::state::SidebarState;
use super::SidebarRendererPlugin;
use crate::controller::ControllerPlugin;
use cc_deck::{ActionMessage, ActionType, RenderPayload, RenderSession, SidebarHello, SidebarInit};
use std::collections::BTreeSet;
use zellij_tile::prelude::*;

pub fn make_payload(sessions: Vec<RenderSession>) -> RenderPayload {
    let total = sessions.len();
    RenderPayload {
        sessions,
        focused_pane_id: None,
        active_tab_index: 0,
        notification: None,
        notification_expiry: None,
        total,
        waiting: 0,
        working: 0,
        idle: 0,
        controller_plugin_id: 1,
        voice_connected: false,
        voice_muted: false,
    }
}

pub fn make_session(pane_id: u32, name: &str, tab_index: usize) -> RenderSession {
    RenderSession {
        pane_id,
        display_name: name.to_string(),
        activity_label: "Idle".to_string(),
        indicator: "\u{25cb}".to_string(),
        color: (180, 175, 195),
        git_branch: None,
        tab_index,
        paused: false,
        done_attended: false,
    }
}

pub fn bare(key: BareKey) -> KeyWithModifier {
    KeyWithModifier {
        bare_key: key,
        key_modifiers: BTreeSet::new(),
    }
}

pub fn make_state_with_sessions(sessions: &[(u32, &str, usize)]) -> SidebarState {
    let mut state = SidebarState::default();
    state.cached_payload = Some(make_payload(
        sessions
            .iter()
            .map(|(id, name, tab)| make_session(*id, name, *tab))
            .collect(),
    ));
    state
}

pub fn make_nav_state(sessions: &[(u32, &str, usize)], cursor: usize) -> SidebarState {
    let mut state = make_state_with_sessions(sessions);
    state.mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: cursor,
        restore_pane_id: None,
        restore_tab_index: None,
        entered_at_ms: 0,
    });
    state
}

// ---------------------------------------------------------------------------
// PipeMessage construction helpers (T005-T008)
// ---------------------------------------------------------------------------

/// Construct a generic PipeMessage from a plugin source.
pub fn make_pipe(name: &str, payload: &str) -> PipeMessage {
    PipeMessage {
        source: PipeSource::Plugin(1),
        name: name.to_string(),
        payload: Some(payload.to_string()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

/// Construct a hook event PipeMessage from CLI source.
pub fn make_hook_pipe(hook_event: &str, pane_id: u32) -> PipeMessage {
    let payload = serde_json::json!({
        "session_id": "test-session",
        "pane_id": pane_id,
        "hook_event_name": hook_event,
    });
    PipeMessage {
        source: PipeSource::Cli("test-pipe".to_string()),
        name: "cc-deck:hook".to_string(),
        payload: Some(payload.to_string()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

/// Construct a hook event PipeMessage with CWD from CLI source.
#[allow(dead_code)]
pub fn make_hook_pipe_with_cwd(hook_event: &str, pane_id: u32, cwd: &str) -> PipeMessage {
    let payload = serde_json::json!({
        "session_id": "test-session",
        "pane_id": pane_id,
        "hook_event_name": hook_event,
        "cwd": cwd,
    });
    PipeMessage {
        source: PipeSource::Cli("test-pipe".to_string()),
        name: "cc-deck:hook".to_string(),
        payload: Some(payload.to_string()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

/// Construct an action PipeMessage from a sidebar.
pub fn make_action_pipe(action: ActionType, pane_id: Option<u32>, sidebar_plugin_id: u32) -> PipeMessage {
    let msg = ActionMessage {
        action,
        pane_id,
        tab_index: None,
        value: None,
        sidebar_plugin_id,
    };
    PipeMessage {
        source: PipeSource::Plugin(sidebar_plugin_id),
        name: "cc-deck:action".to_string(),
        payload: Some(serde_json::to_string(&msg).unwrap()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

/// Construct a SidebarHello PipeMessage.
pub fn make_hello_pipe(plugin_id: u32) -> PipeMessage {
    let hello = SidebarHello { plugin_id };
    PipeMessage {
        source: PipeSource::Plugin(plugin_id),
        name: "cc-deck:sidebar-hello".to_string(),
        payload: Some(serde_json::to_string(&hello).unwrap()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

/// Construct a SidebarInit PipeMessage from the controller.
pub fn make_init_pipe(tab_index: usize, controller_plugin_id: u32) -> PipeMessage {
    let init = SidebarInit {
        tab_index,
        controller_plugin_id,
    };
    PipeMessage {
        source: PipeSource::Plugin(controller_plugin_id),
        name: "cc-deck:sidebar-init".to_string(),
        payload: Some(serde_json::to_string(&init).unwrap()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}

// ---------------------------------------------------------------------------
// Plugin setup helpers (T009-T010)
// ---------------------------------------------------------------------------

/// Create a SidebarRendererPlugin with permissions granted and ready to test.
pub fn setup_sidebar() -> SidebarRendererPlugin {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    plugin
}

/// Create a ControllerPlugin with permissions granted and ready to test.
pub fn setup_controller() -> ControllerPlugin {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    // Grant permissions (note: in non-wasm tests, restore_sessions and
    // cleanup_orphaned_state_files are no-ops because /cache/ does not exist)
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    plugin
}

/// Create a SidebarRendererPlugin with permissions granted and tab index assigned.
#[allow(dead_code)]
pub fn setup_sidebar_with_tab(tab_index: usize) -> SidebarRendererPlugin {
    let mut plugin = setup_sidebar();
    let init_pipe = make_init_pipe(tab_index, 1);
    plugin.pipe(init_pipe);
    plugin
}
