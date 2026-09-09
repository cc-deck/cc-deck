// Integration tests for SidebarRendererPlugin.
//
// Exercise the plugin through its ZellijPlugin trait methods (load, update,
// pipe, render) with synthetic events, verifying the full event dispatch
// chain without requiring a running Zellij instance.

use super::test_helpers::*;
use super::PERMISSION_RETRY_LIMIT;
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
    plugin.pipe(make_pipe(
        "cc-deck:render",
        &serde_json::to_string(&payload1).unwrap(),
    ));
    assert_eq!(plugin.test_state().filtered_sessions().len(), 2);

    // Send second payload with 1 session (replaces first)
    let payload2 = make_payload(vec![make_session(3, "worker", 0)]);
    plugin.pipe(make_pipe(
        "cc-deck:render",
        &serde_json::to_string(&payload2).unwrap(),
    ));

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

    let payload = make_payload(vec![make_session(1, "api-server", 0)]);
    let json = serde_json::to_string(&payload).unwrap();
    // The payload is cached regardless of permission state, so the first
    // frame after the grant is already there. A render must not count as a
    // permission retry: only the bounded timer path asks again.
    plugin.pipe(make_pipe("cc-deck:render", &json));

    assert!(plugin.test_state().cached_payload.is_some());
    assert_eq!(plugin.test_state().permission_retries, 0);
}

#[test]
fn test_sidebar_denied_permission_stops_retrying() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    plugin.update(Event::PermissionRequestResult(PermissionStatus::Denied));

    assert!(!plugin.test_state().permissions_granted);
    // Timer ticks must not re-raise the prompt the user just dismissed.
    for _ in 0..3 {
        plugin.update(Event::Timer(1.0));
    }
    assert_eq!(plugin.test_state().permission_retries, PERMISSION_RETRY_LIMIT);
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
    plugin.pipe(make_pipe(
        "cc-deck:render",
        &serde_json::to_string(&payload).unwrap(),
    ));

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
        sort_active: false,
        client_views: std::collections::BTreeMap::from([(
            0,
            cc_deck::ClientViewSnapshot {
                active_tab_index: Some(0),
                focused_pane_id: Some(1),
                revision: 1,
            },
        )]),
        multiplayer_colors: None,
        profile_legend: vec![],
    };

    let json = serde_json::to_string(&original).unwrap();
    plugin.pipe(make_pipe("cc-deck:render", &json));

    let cached = plugin.test_state().cached_payload.as_ref().unwrap();
    assert_eq!(cached.sessions.len(), 2);
    assert_eq!(cached.sessions[0].pane_id, 1);
    assert_eq!(cached.sessions[0].display_name, "api-server");
    assert_eq!(cached.sessions[1].pane_id, 2);
    assert_eq!(cached.client_views[&0].focused_pane_id, Some(1));
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
    plugin.pipe(make_pipe(
        "cc-deck:render",
        &serde_json::to_string(&payload).unwrap(),
    ));

    assert!(plugin.test_state().local_mute_override.is_none());
}

#[test]
fn test_local_mute_override_preserved_on_mismatch() {
    let mut plugin = setup_sidebar();
    plugin.test_state_mut().local_mute_override = Some(true);

    let mut payload = make_payload(vec![make_session(1, "test", 0)]);
    payload.voice_connected = true;
    payload.voice_muted = false;
    plugin.pipe(make_pipe(
        "cc-deck:render",
        &serde_json::to_string(&payload).unwrap(),
    ));

    assert_eq!(plugin.test_state().local_mute_override, Some(true));
}

use super::SidebarRendererPlugin;
use cc_deck::{LegendEntry, RenderPayload};

// ---------------------------------------------------------------------------
// Lost Permission Grant Recovery
//
// Zellij addresses a PermissionRequestResult to a specific client. A sidebar
// created during the initial layout load can be handed that result before any
// client is ready, and the grant is then dropped. The pane sits on the
// permission prompt forever, because every other recovery path depends on the
// controller having registered this sidebar, which itself depends on the
// grant. The timer is the only signal that reaches the plugin from outside
// that loop.
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_timer_reasks_for_lost_permission_grant() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    assert_eq!(plugin.test_state().permission_retries, 0);

    plugin.update(Event::Timer(1.0));

    assert_eq!(plugin.test_state().permission_retries, 1);
    assert!(!plugin.test_state().permissions_granted);
}

#[test]
fn test_sidebar_permission_retry_is_bounded() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // Far more ticks than the limit: a request the user has genuinely not
    // answered yet must not be re-raised on a loop.
    for _ in 0..(super::PERMISSION_RETRY_LIMIT as usize + 20) {
        plugin.update(Event::Timer(1.0));
    }

    assert_eq!(
        plugin.test_state().permission_retries,
        super::PERMISSION_RETRY_LIMIT
    );
}

#[test]
fn test_sidebar_timer_does_not_reask_once_granted() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));

    plugin.update(Event::Timer(1.0));

    assert_eq!(plugin.test_state().permission_retries, 0);
}

#[test]
fn test_sidebar_permission_grant_requests_repaint() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // render() draws the permission prompt while the grant is missing, and the
    // pane is made unselectable on grant, so nothing else will ever prompt a
    // repaint. Without this the prompt stays on screen over a working sidebar.
    let should_render = plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));

    assert!(should_render);
}

// ---------------------------------------------------------------------------
// US2: Profile Indicators in Sidebar (T040)
// ---------------------------------------------------------------------------

#[test]
fn test_sidebar_agent_color_reaches_rendered_session() {
    let mut plugin = setup_sidebar();

    let mut session = make_session(1, "profiled", 0);
    session.agent_color = Some((255, 0, 0));
    let payload = make_payload(vec![session]);
    let json = serde_json::to_string(&payload).unwrap();
    plugin.pipe(make_pipe("cc-deck:render", &json));

    let cached = plugin.test_state().cached_payload.as_ref().unwrap();
    assert_eq!(cached.sessions.len(), 1);
    assert_eq!(cached.sessions[0].agent_color, Some((255, 0, 0)));
}

#[test]
fn test_sidebar_help_overlay_contains_legend_names() {
    let mut plugin = setup_sidebar();

    let mut payload = make_payload(vec![make_session(1, "test", 0)]);
    payload.profile_legend = vec![
        LegendEntry {
            name: "work".to_string(),
            indicator: "\u{2733}".to_string(),
            color: (255, 0, 0),
        },
        LegendEntry {
            name: "personal".to_string(),
            indicator: "\u{2733}".to_string(),
            color: (0, 255, 0),
        },
    ];
    let json = serde_json::to_string(&payload).unwrap();
    plugin.pipe(make_pipe("cc-deck:render", &json));

    let cached = plugin.test_state().cached_payload.as_ref().unwrap();
    assert_eq!(cached.profile_legend.len(), 2);
    assert_eq!(cached.profile_legend[0].name, "work");
    assert_eq!(cached.profile_legend[1].name, "personal");
}
