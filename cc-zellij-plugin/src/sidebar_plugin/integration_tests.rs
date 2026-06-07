// Integration tests for SidebarRendererPlugin.
//
// Exercise the plugin through its ZellijPlugin trait methods (load, update,
// pipe, render) with synthetic events, verifying the full event dispatch
// chain without requiring a running Zellij instance.

use super::test_helpers::*;
use zellij_tile::prelude::*;

// ---------------------------------------------------------------------------
// User Story 1: Sidebar Receives Render Payload (T011-T014)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_load_and_permission_grant() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // Before permissions: not initialized, no permissions
    assert!(!plugin.test_state().permissions_granted);
    assert!(!plugin.test_state().initialized);

    // Grant permissions
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    assert!(plugin.test_state().permissions_granted);
}

#[test]
fn test_sidebar_receives_render_payload() {
    let mut plugin = setup_sidebar();

    let payload = make_payload(vec![
        make_session(1, "api-server", 0),
        make_session(2, "frontend", 1),
        make_session(3, "worker", 2),
    ]);
    let json = serde_json::to_string(&payload).unwrap();
    let should_render = plugin.pipe(make_pipe("cc-deck:render", &json));

    assert!(should_render);
    assert!(plugin.test_state().initialized);
    let sessions = plugin.test_state().filtered_sessions();
    assert_eq!(sessions.len(), 3);
    assert_eq!(sessions[0].display_name, "api-server");
    assert_eq!(sessions[1].display_name, "frontend");
    assert_eq!(sessions[2].display_name, "worker");
}

#[test]
fn test_sidebar_payload_replacement() {
    let mut plugin = setup_sidebar();

    // Send first payload with 2 sessions
    let payload1 = make_payload(vec![
        make_session(1, "api-server", 0),
        make_session(2, "frontend", 1),
    ]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload1).unwrap()));
    assert_eq!(plugin.test_state().filtered_sessions().len(), 2);

    // Send second payload with 1 session (replaces first)
    let payload2 = make_payload(vec![
        make_session(3, "worker", 0),
    ]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload2).unwrap()));

    let sessions = plugin.test_state().filtered_sessions();
    assert_eq!(sessions.len(), 1);
    assert_eq!(sessions[0].display_name, "worker");
    assert_eq!(sessions[0].pane_id, 3);
}

#[test]
fn test_sidebar_render_before_permissions() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    // Do NOT grant permissions

    let payload = make_payload(vec![
        make_session(1, "api-server", 0),
    ]);
    let json = serde_json::to_string(&payload).unwrap();
    // Pipe still processes render payload (it re-requests permissions and
    // stores the payload), but the sidebar is not "initialized" via
    // permissions_granted alone. The render payload processing happens
    // regardless of permission state for cc-deck:render.
    plugin.pipe(make_pipe("cc-deck:render", &json));

    // The sidebar stores the payload even without permissions (it re-requests
    // them on each render pipe). Verify it was stored.
    assert!(plugin.test_state().cached_payload.is_some());
}

// ---------------------------------------------------------------------------
// User Story 3: Sidebar-Controller Discovery Protocol (T020)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_init_assigns_tab() {
    let mut plugin = setup_sidebar();

    assert!(plugin.test_state().my_tab_index.is_none());

    let init_pipe = make_init_pipe(2, 42);
    plugin.pipe(init_pipe);

    assert_eq!(plugin.test_state().my_tab_index, Some(2));
    assert_eq!(plugin.test_state().controller_plugin_id, Some(42));
}

// ---------------------------------------------------------------------------
// User Story 6: Sidebar Mode Transitions (T029)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_navigate_mode_via_pipe() {
    let mut plugin = setup_sidebar();

    // Send a render payload first so we have sessions to navigate
    let payload = make_payload(vec![
        make_session(1, "api-server", 0),
        make_session(2, "frontend", 0),
    ]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));

    // Set the sidebar tab index so it responds to navigate messages
    plugin.pipe(make_init_pipe(0, 1));

    // Send navigate message targeting tab 0 (where our sidebar is)
    let nav_json = r#"{"active_tab_index":0,"direction":"forward"}"#;
    let should_render = plugin.pipe(make_pipe("cc-deck:navigate", nav_json));

    assert!(should_render);
    assert!(plugin.test_state().mode.is_navigating());
}

// ---------------------------------------------------------------------------
// Error Handling and Edge Cases (T024, T026, T027)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_malformed_pipe_message() {
    let mut plugin = setup_sidebar();

    // Send malformed JSON as render payload
    let should_render = plugin.pipe(make_pipe("cc-deck:render", "not valid json {{{"));

    assert!(!should_render);
    // State should be unchanged (no cached payload)
    assert!(plugin.test_state().cached_payload.is_none());
    assert!(!plugin.test_state().initialized);
}

#[test]
fn test_sidebar_empty_payload() {
    let mut plugin = setup_sidebar();

    let payload = make_payload(vec![]);
    let json = serde_json::to_string(&payload).unwrap();
    let should_render = plugin.pipe(make_pipe("cc-deck:render", &json));

    assert!(should_render);
    assert!(plugin.test_state().initialized);
    assert!(plugin.test_state().filtered_sessions().is_empty());
}

#[test]
fn test_render_payload_roundtrip_through_pipe() {
    let mut plugin = setup_sidebar();

    // Construct a detailed payload with all fields populated
    let original = RenderPayload {
        sessions: vec![
            make_session(1, "api-server", 0),
            make_session(2, "frontend", 1),
        ],
        focused_pane_id: Some(1),
        active_tab_index: 0,
        notification: Some("Test notification".to_string()),
        notification_expiry: Some(9999),
        total: 2,
        waiting: 0,
        working: 1,
        idle: 1,
        controller_plugin_id: 42,
        voice_connected: true,
        voice_muted: false,
        show_agent_indicators: false,
    };

    let json = serde_json::to_string(&original).unwrap();
    plugin.pipe(make_pipe("cc-deck:render", &json));

    let cached = plugin.test_state().cached_payload.as_ref().unwrap();
    assert_eq!(cached.sessions.len(), 2);
    assert_eq!(cached.sessions[0].pane_id, 1);
    assert_eq!(cached.sessions[0].display_name, "api-server");
    assert_eq!(cached.sessions[1].pane_id, 2);
    assert_eq!(cached.focused_pane_id, Some(1));
    assert_eq!(cached.active_tab_index, 0);
    assert_eq!(cached.controller_plugin_id, 42);
    assert_eq!(cached.total, 2);
    assert_eq!(cached.working, 1);
    assert!(cached.voice_connected);
    assert!(!cached.voice_muted);
}

// ---------------------------------------------------------------------------
// Additional edge case: unknown pipe names are ignored
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_unknown_pipe_ignored() {
    let mut plugin = setup_sidebar();

    let should_render = plugin.pipe(make_pipe("cc-deck:unknown-message", "{}"));
    assert!(!should_render);
}

#[test]
fn test_local_mute_override_cleared_on_disconnect() {
    let mut plugin = setup_sidebar();
    plugin.test_state_mut().local_mute_override = Some(true);

    let mut payload = make_payload(vec![make_session(1, "test", 0)]);
    payload.voice_connected = false;
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));

    assert!(plugin.test_state().local_mute_override.is_none());
}

#[test]
fn test_local_mute_override_preserved_on_mismatch() {
    let mut plugin = setup_sidebar();
    plugin.test_state_mut().local_mute_override = Some(true);

    let mut payload = make_payload(vec![make_session(1, "test", 0)]);
    payload.voice_connected = true;
    payload.voice_muted = false;
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));

    assert_eq!(plugin.test_state().local_mute_override, Some(true));
}

use super::SidebarRendererPlugin;
use cc_deck::RenderPayload;

// ---------------------------------------------------------------------------
// Render Pipeline Stability: Sidebar Render Request Fallback (T019-T021)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_sends_render_request_after_3_ticks() {
    let mut plugin = setup_sidebar();
    // Set controller_plugin_id so the request has a target
    plugin.test_state_mut().controller_plugin_id = Some(42);

    assert!(!plugin.test_state().render_request_sent);
    assert_eq!(plugin.test_state().ticks_since_init, 0);

    // Tick 1
    plugin.update(Event::Timer(1.0));
    assert!(!plugin.test_state().render_request_sent);
    assert_eq!(plugin.test_state().ticks_since_init, 1);

    // Tick 2
    plugin.update(Event::Timer(1.0));
    assert!(!plugin.test_state().render_request_sent);

    // Tick 3: should send render request
    plugin.update(Event::Timer(1.0));
    assert!(plugin.test_state().render_request_sent);
    assert_eq!(plugin.test_state().ticks_since_init, 3);
}

#[test]
fn test_sidebar_does_not_send_render_request_if_already_initialized() {
    let mut plugin = setup_sidebar();
    plugin.test_state_mut().controller_plugin_id = Some(42);

    // Receive a render payload first
    let payload = make_payload(vec![make_session(1, "api-server", 0)]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));
    assert!(plugin.test_state().initialized);

    // Even after 3+ ticks, no render request should be sent
    for _ in 0..5 {
        plugin.update(Event::Timer(1.0));
    }
    assert!(!plugin.test_state().render_request_sent);
}

#[test]
fn test_sidebar_does_not_send_render_request_more_than_once() {
    let mut plugin = setup_sidebar();
    plugin.test_state_mut().controller_plugin_id = Some(42);

    // Get past the 3-tick threshold
    for _ in 0..4 {
        plugin.update(Event::Timer(1.0));
    }
    assert!(plugin.test_state().render_request_sent);

    // Further ticks should not reset the flag
    for _ in 0..5 {
        plugin.update(Event::Timer(1.0));
    }
    assert!(plugin.test_state().render_request_sent);
}

#[test]
fn test_sidebar_sends_broadcast_render_request_without_controller_id() {
    let mut plugin = setup_sidebar();
    // controller_plugin_id is None by default

    // After 3 ticks, render request should be sent even without controller_plugin_id
    // (broadcast to any controller)
    for _ in 0..3 {
        plugin.update(Event::Timer(1.0));
    }
    assert!(plugin.test_state().render_request_sent);
}
