// Integration tests for ControllerPlugin.
//
// Exercise the plugin through its ZellijPlugin trait methods (load, update,
// pipe) with synthetic hook events, action messages, and discovery protocol
// messages. Verifies the full event dispatch chain without requiring a
// running Zellij instance.

use crate::sidebar_plugin::test_helpers::*;
use crate::session::Activity;
use cc_deck::ActionType;
use zellij_tile::prelude::*;

// ---------------------------------------------------------------------------
// User Story 2: Controller Processes Hook Events (T015-T018)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_load_and_permission_grant() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // Before permissions: not initialized
    assert!(!plugin.test_state().permissions_granted);
    assert!(plugin.test_state().sessions.is_empty());

    // Grant permissions
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    assert!(plugin.test_state().permissions_granted);
}

#[test]
fn test_controller_hook_session_start() {
    let mut plugin = setup_controller();

    let hook = make_hook_pipe("SessionStart", 42);
    plugin.pipe(hook);

    assert!(plugin.test_state().sessions.contains_key(&42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Init);
}

#[test]
fn test_controller_hook_pre_tool_use() {
    let mut plugin = setup_controller();

    // Create the session first via SessionStart
    plugin.pipe(make_hook_pipe("SessionStart", 42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Init);

    // Send PreToolUse to transition to Working
    plugin.pipe(make_hook_pipe("PreToolUse", 42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Working);
}

#[test]
fn test_controller_hook_stop() {
    let mut plugin = setup_controller();

    // Create session and move to Working
    plugin.pipe(make_hook_pipe("SessionStart", 42));
    plugin.pipe(make_hook_pipe("PreToolUse", 42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Working);

    // Send Stop to transition to Done
    plugin.pipe(make_hook_pipe("Stop", 42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Done);
}

// ---------------------------------------------------------------------------
// User Story 3: Sidebar-Controller Discovery Protocol (T019)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_sidebar_hello_registration() {
    let mut plugin = setup_controller();

    // Set up a pane manifest so the controller can find the sidebar's tab.
    // The hello handler cross-references plugin_id with the manifest.
    let mut panes = std::collections::HashMap::new();
    panes.insert(0, vec![make_pane_info_full(99, true, false)]);
    plugin.test_state_mut().pane_manifest = Some(PaneManifest { panes });

    // Send SidebarHello from plugin_id=99
    let hello = make_hello_pipe(99);
    plugin.pipe(hello);

    // Verify sidebar is registered with the correct tab index
    assert!(plugin.test_state().sidebar_registry.contains_key(&99));
    assert_eq!(plugin.test_state().sidebar_registry[&99], 0);
}

// ---------------------------------------------------------------------------
// User Story 4: Sidebar Action Message Dispatch (T021-T022)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_action_pause() {
    let mut plugin = setup_controller();

    // Create a session
    plugin.pipe(make_hook_pipe("SessionStart", 42));
    assert!(!plugin.test_state().sessions[&42].paused);

    // Send Pause action
    let pause = make_action_pipe(ActionType::Pause, Some(42), 10);
    plugin.pipe(pause);

    assert!(plugin.test_state().sessions[&42].paused);

    // Toggle back
    let unpause = make_action_pipe(ActionType::Pause, Some(42), 10);
    plugin.pipe(unpause);

    assert!(!plugin.test_state().sessions[&42].paused);
}

#[test]
fn test_controller_action_attend() {
    let mut plugin = setup_controller();

    // Create a Done session with a tab_index
    plugin.pipe(make_hook_pipe("SessionStart", 42));
    plugin.pipe(make_hook_pipe("PreToolUse", 42));
    plugin.pipe(make_hook_pipe("Stop", 42));
    assert_eq!(plugin.test_state().sessions[&42].activity, Activity::Done);
    assert!(!plugin.test_state().sessions[&42].done_attended);

    // Set tab_index so attend can find it
    plugin.test_state_mut().sessions.get_mut(&42).unwrap().tab_index = Some(0);

    // Send Attend action (via pipe, not action message, since attend is CLI-driven)
    let attend_pipe = PipeMessage {
        source: PipeSource::Cli("test".to_string()),
        name: "cc-deck:attend".to_string(),
        payload: None,
        args: std::collections::BTreeMap::new(),
        is_private: false,
    };
    plugin.pipe(attend_pipe);

    assert_eq!(plugin.test_state().last_attended_pane_id, Some(42));
    assert!(plugin.test_state().sessions[&42].done_attended);
}

// ---------------------------------------------------------------------------
// User Story 5: Permission Grant and Deferred Event Replay (T023)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_deferred_events() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // Do NOT grant permissions yet. Send events that should be queued.
    // Note: pipe() returns false before permissions are granted (controller
    // drops pipe messages). But update() with non-permission events queues them.
    plugin.update(Event::Timer(1.0));

    // Verify event was queued
    assert_eq!(plugin.test_state().pending_events.len(), 1);

    // Now grant permissions, which should replay pending events
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));

    assert!(plugin.test_state().permissions_granted);
    // Pending events should be drained after replay
    assert!(plugin.test_state().pending_events.is_empty());
}

// ---------------------------------------------------------------------------
// Error Handling (T025, T028)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_malformed_hook_payload() {
    let mut plugin = setup_controller();

    // Send a hook pipe with malformed JSON
    let bad_hook = PipeMessage {
        source: PipeSource::Cli("test".to_string()),
        name: "cc-deck:hook".to_string(),
        payload: Some("not valid json {{{".to_string()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    };
    // Should not panic
    plugin.pipe(bad_hook);

    // State should be unchanged
    assert!(plugin.test_state().sessions.is_empty());
}

#[test]
fn test_action_message_roundtrip_through_pipe() {
    let mut plugin = setup_controller();

    // Create a session so the action has a target
    plugin.pipe(make_hook_pipe("SessionStart", 42));
    assert!(!plugin.test_state().sessions[&42].paused);

    // Construct an ActionMessage, serialize it, and send through pipe
    let msg = cc_deck::ActionMessage {
        action: ActionType::Pause,
        pane_id: Some(42),
        tab_index: None,
        value: None,
        sidebar_plugin_id: 10,
    };
    let json = serde_json::to_string(&msg).unwrap();
    let pipe = make_pipe("cc-deck:action", &json);
    plugin.pipe(pipe);

    // Verify the action was processed correctly
    assert!(plugin.test_state().sessions[&42].paused);
}

// ---------------------------------------------------------------------------
// Additional: Hook event lifecycle chain
// ---------------------------------------------------------------------------

#[test]
fn test_controller_full_lifecycle_chain() {
    let mut plugin = setup_controller();

    // SessionStart -> Init
    plugin.pipe(make_hook_pipe("SessionStart", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Init);

    // PreToolUse -> Working
    plugin.pipe(make_hook_pipe("PreToolUse", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Working);

    // PermissionRequest -> Waiting
    plugin.pipe(make_hook_pipe("PermissionRequest", 10));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    // PostToolUse -> Working (clears Waiting)
    plugin.pipe(make_hook_pipe("PostToolUse", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Working);

    // Stop -> Done
    plugin.pipe(make_hook_pipe("Stop", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Done);
}

#[test]
fn test_controller_multiple_sessions() {
    let mut plugin = setup_controller();

    // Create three sessions
    plugin.pipe(make_hook_pipe("SessionStart", 1));
    plugin.pipe(make_hook_pipe("SessionStart", 2));
    plugin.pipe(make_hook_pipe("SessionStart", 3));

    assert_eq!(plugin.test_state().sessions.len(), 3);

    // Move each to a different state
    plugin.pipe(make_hook_pipe("PreToolUse", 1));
    plugin.pipe(make_hook_pipe("Stop", 2));
    // Session 3 stays at Init

    assert_eq!(plugin.test_state().sessions[&1].activity, Activity::Working);
    assert_eq!(plugin.test_state().sessions[&2].activity, Activity::Done);
    assert_eq!(plugin.test_state().sessions[&3].activity, Activity::Init);
}

// ---------------------------------------------------------------------------
// Helper: PaneInfo construction for manifest setup
// ---------------------------------------------------------------------------

fn make_pane_info_full(id: u32, is_plugin: bool, is_focused: bool) -> PaneInfo {
    PaneInfo {
        id,
        is_plugin,
        is_focused,
        is_fullscreen: false,
        is_floating: false,
        is_suppressed: false,
        title: String::new(),
        exited: false,
        exit_status: None,
        is_held: false,
        pane_x: 0,
        pane_content_x: 0,
        pane_y: 0,
        pane_content_y: 0,
        pane_rows: 0,
        pane_content_rows: 0,
        pane_columns: 0,
        pane_content_columns: 0,
        cursor_coordinates_in_pane: None,
        terminal_command: None,
        plugin_url: None,
        is_selectable: true,
        index_in_pane_group: std::collections::BTreeMap::new(),
        default_bg: None,
        default_fg: None,
    }
}

use crate::controller::ControllerPlugin;

// ---------------------------------------------------------------------------
// Render Pipeline Stability: Defensive Guard (T007)
// ---------------------------------------------------------------------------

#[test]
fn test_controller_without_permissions_ignores_pipe() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    // Do NOT grant permissions

    let hook = make_hook_pipe("SessionStart", 42);
    plugin.pipe(hook);

    // Session should NOT be created because permissions are not granted
    assert!(plugin.test_state().sessions.is_empty());
}

// ---------------------------------------------------------------------------
// Render Pipeline Stability: Probe messages are no-ops (T008-T009)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Leader Election Protocol Tests (T009, T010, T017)
// ---------------------------------------------------------------------------

#[test]
fn test_election_dormant_default() {
    // A fresh controller starts dormant (is_leader = false)
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    // Do NOT set is_leader = true (unlike setup_controller)
    assert!(!plugin.test_state().is_leader);
}

#[test]
fn test_election_activation_after_timeout() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    assert!(!plugin.test_state().is_leader);

    // Simulate 2 timer ticks (ELECTION_TIMEOUT_TICKS = 2)
    plugin.update(Event::Timer(1.0));
    assert!(!plugin.test_state().is_leader);
    plugin.update(Event::Timer(1.0));
    assert!(plugin.test_state().is_leader);
}

#[test]
fn test_election_dormant_on_lower_id_ping() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    plugin.test_state_mut().plugin_id = 10;

    // Receive ping from lower-ID controller (ID 5)
    let ping = make_pipe("cc-deck:controller-ping", "5");
    // Dormant controller still processes controller-ping (not blocked by guard)
    plugin.pipe(ping);

    assert!(!plugin.test_state().is_leader);
    assert_eq!(plugin.test_state().leader_plugin_id, Some(5));
    assert!(plugin.test_state().last_leader_ping_ms > 0);

    // Even after timeout ticks, should not activate (leader is known)
    plugin.update(Event::Timer(1.0));
    plugin.update(Event::Timer(1.0));
    plugin.update(Event::Timer(1.0));
    assert!(!plugin.test_state().is_leader);
}

#[test]
fn test_election_dormant_ignores_pipe_messages() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    // is_leader = false (dormant)

    // Hook events should be ignored
    let hook = make_hook_pipe("SessionStart", 42);
    plugin.pipe(hook);
    assert!(plugin.test_state().sessions.is_empty());
}

#[test]
fn test_election_leader_processes_pipes() {
    let mut plugin = setup_controller();
    // setup_controller sets is_leader = true

    let hook = make_hook_pipe("SessionStart", 42);
    plugin.pipe(hook);
    assert!(plugin.test_state().sessions.contains_key(&42));
}

#[test]
fn test_election_dual_controllers_navigation() {
    // T010: Two controllers, only the leader processes navigate messages
    let mut leader = setup_controller();
    leader.test_state_mut().plugin_id = 0;
    leader.test_state_mut().is_leader = true;

    let mut dormant = ControllerPlugin::default();
    dormant.load(std::collections::BTreeMap::new());
    dormant.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    dormant.test_state_mut().plugin_id = 4;
    // Simulate receiving ping from lower ID
    let ping = make_pipe("cc-deck:controller-ping", "0");
    dormant.pipe(ping);
    assert!(!dormant.test_state().is_leader);

    // Navigate message should be ignored by dormant controller
    let nav = make_pipe("cc-deck:navigate", "");
    dormant.pipe(nav);
    // No panic, no state change (dormant guard blocks it)

    // Navigate message should be processed by leader (no panic in non-WASM)
    let nav2 = PipeMessage {
        source: PipeSource::Cli("test".to_string()),
        name: "cc-deck:navigate".to_string(),
        payload: None,
        args: std::collections::BTreeMap::new(),
        is_private: false,
    };
    leader.pipe(nav2);
}

#[test]
fn test_election_full_flow() {
    // T017: Simulate full election with two controllers
    let mut ctrl_low = ControllerPlugin::default();
    ctrl_low.load(std::collections::BTreeMap::new());
    ctrl_low.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    ctrl_low.test_state_mut().plugin_id = 0;

    let mut ctrl_high = ControllerPlugin::default();
    ctrl_high.load(std::collections::BTreeMap::new());
    ctrl_high.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    ctrl_high.test_state_mut().plugin_id = 4;

    // Both are dormant initially
    assert!(!ctrl_low.test_state().is_leader);
    assert!(!ctrl_high.test_state().is_leader);

    // High-ID receives ping from low-ID: goes/stays dormant
    let ping_from_low = make_pipe("cc-deck:controller-ping", "0");
    ctrl_high.pipe(ping_from_low);
    assert!(!ctrl_high.test_state().is_leader);
    assert_eq!(ctrl_high.test_state().leader_plugin_id, Some(0));

    // Low-ID receives ping from high-ID: responds with own ping (lower wins)
    let ping_from_high = make_pipe("cc-deck:controller-ping", "4");
    ctrl_low.pipe(ping_from_high);
    // Low-ID should NOT go dormant (it has the lower ID)
    assert!(ctrl_low.test_state().leader_plugin_id.is_none());

    // Low-ID: election timeout fires, activates as leader
    ctrl_low.update(Event::Timer(1.0));
    ctrl_low.update(Event::Timer(1.0));
    assert!(ctrl_low.test_state().is_leader);

    // High-ID stays dormant
    ctrl_high.update(Event::Timer(1.0));
    ctrl_high.update(Event::Timer(1.0));
    ctrl_high.update(Event::Timer(1.0));
    assert!(!ctrl_high.test_state().is_leader);
}

#[test]
fn test_election_leader_demotion() {
    // A controller that is already leader receives a ping from a lower-ID
    // controller and must step down.
    let mut plugin = setup_controller();
    plugin.test_state_mut().plugin_id = 10;
    plugin.test_state_mut().keybindings_registered = true;
    assert!(plugin.test_state().is_leader);

    // Receive ping from lower-ID controller
    let ping = make_pipe("cc-deck:controller-ping", "2");
    plugin.pipe(ping);

    assert!(!plugin.test_state().is_leader);
    assert_eq!(plugin.test_state().leader_plugin_id, Some(2));
    assert!(!plugin.test_state().keybindings_registered);

    // Verify dormant guard now blocks hook events
    let hook = make_hook_pipe("SessionStart", 42);
    plugin.pipe(hook);
    assert!(plugin.test_state().sessions.is_empty());
}

#[test]
fn test_election_leader_failure_reactivation() {
    // Simulate a dormant controller detecting leader failure and re-activating.
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    plugin.test_state_mut().plugin_id = 4;

    // Accept leader with ID 0
    let ping = make_pipe("cc-deck:controller-ping", "0");
    plugin.pipe(ping);
    assert_eq!(plugin.test_state().leader_plugin_id, Some(0));
    assert!(!plugin.test_state().is_leader);

    // Simulate leader failure: set last_leader_ping_ms far in the past
    plugin.test_state_mut().last_leader_ping_ms = 1;

    // Fire a timer tick to trigger leader failure detection.
    // Within this tick: election_ticks increments, then leader failure
    // resets it back to 0 and clears leader_plugin_id.
    plugin.update(Event::Timer(1.0));

    assert!(plugin.test_state().leader_plugin_id.is_none());
    assert_eq!(plugin.test_state().election_ticks, 0);
    assert!(!plugin.test_state().is_leader);

    // Two more ticks needed to reach ELECTION_TIMEOUT_TICKS (2)
    plugin.update(Event::Timer(1.0));
    assert!(!plugin.test_state().is_leader);
    plugin.update(Event::Timer(1.0));
    assert!(plugin.test_state().is_leader);
}

#[test]
fn test_election_no_payload_ping_ignored() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    plugin.test_state_mut().plugin_id = 5;

    // Ping with no payload should be silently ignored
    let ping = PipeMessage {
        source: PipeSource::Plugin(1),
        name: "cc-deck:controller-ping".to_string(),
        payload: None,
        args: std::collections::BTreeMap::new(),
        is_private: false,
    };
    plugin.pipe(ping);

    assert!(plugin.test_state().leader_plugin_id.is_none());
    assert_eq!(plugin.test_state().last_leader_ping_ms, 0);
}

#[test]
fn test_election_heartbeat_resets_timer() {
    let mut dormant = ControllerPlugin::default();
    dormant.load(std::collections::BTreeMap::new());
    dormant.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    dormant.test_state_mut().plugin_id = 4;

    // Accept leadership from ID 0
    let ping = make_pipe("cc-deck:controller-ping", "0");
    dormant.pipe(ping);
    assert_eq!(dormant.test_state().leader_plugin_id, Some(0));

    let first_ping_ms = dormant.test_state().last_leader_ping_ms;

    // Simulate some time passing then another heartbeat
    std::thread::sleep(std::time::Duration::from_millis(10));
    let heartbeat = make_pipe("cc-deck:controller-ping", "0");
    dormant.pipe(heartbeat);

    assert!(dormant.test_state().last_leader_ping_ms >= first_ping_ms);
    assert!(!dormant.test_state().is_leader);
}

#[test]
fn test_voice_reconnect_resync_muted() {
    let mut plugin = setup_controller();

    plugin.pipe(make_pipe("cc-deck:voice", "[[voice:on]]"));
    assert!(plugin.test_state().voice_enabled);
    plugin.pipe(make_pipe("cc-deck:voice", "[[voice:mute]]"));
    assert!(plugin.test_state().voice_muted);

    // Simulate disconnect
    plugin.test_state_mut().voice_enabled = false;
    plugin.test_state_mut().voice_muted = false;
    plugin.test_state_mut().voice_mute_requested = None;

    // Reconnect with muted state
    plugin.pipe(make_pipe("cc-deck:voice", "[[voice:on:muted]]"));
    assert!(plugin.test_state().voice_enabled);
    assert!(plugin.test_state().voice_muted);
    assert!(plugin.test_state().voice_mute_requested.is_none());
}
