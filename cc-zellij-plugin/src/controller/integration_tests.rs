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

/// End to end reproduction of the reported bug: an agent running outside
/// Zellij delivered hook events naming a pane from an earlier run. Driven
/// through the real pipe entry point, the session must not survive.
#[test]
fn test_controller_phantom_hook_is_evicted_end_to_end() {
    let mut plugin = setup_controller();

    // The controller knows about pane 10. Pane 99 does not exist.
    let mut panes = std::collections::HashMap::new();
    panes.insert(0, vec![make_pane_info_full(10, false, false)]);
    plugin.test_state_mut().pane_manifest = Some(PaneManifest { panes });

    plugin.pipe(make_hook_pipe("SessionStart", 99));

    assert!(
        plugin.test_state().is_hidden(99),
        "a pane the manifest never showed must be withheld from the sidebar"
    );
    assert!(
        !crate::controller::render_broadcast::build_render_payload(plugin.test_state())
            .sessions
            .iter()
            .any(|s| s.pane_id == 99),
        "and must not appear in the render payload"
    );

    // Expire the deadline and let the sweep settle it.
    plugin
        .test_state_mut()
        .unconfirmed_panes
        .get_mut(&99)
        .unwrap()
        .deadline_ms = 0;
    plugin.test_state_mut().sweep_quarantine();

    assert!(
        !plugin.test_state().sessions.contains_key(&99),
        "the phantom must not outlive its deadline"
    );
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
    assert_eq!(plugin.test_state().sidebar_registry[&99], (0, 0));
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

    // Give the controller a pane manifest containing pane 42. Without one, the
    // session stays quarantined and is withheld from the attend candidates,
    // which is the point of the quarantine rather than a defect.
    let mut panes = std::collections::HashMap::new();
    panes.insert(0, vec![make_pane_info_full(42, false, false)]);
    plugin.test_state_mut().pane_manifest = Some(PaneManifest { panes });

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
    //
    // Deliberately not a Timer: while unpermissioned the timer is claimed by
    // the lost-grant retry and is not queued. Every other event still is.
    plugin.update(Event::TabUpdate(vec![]));

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
fn test_controller_waiting_not_cleared_by_subagent_events() {
    let mut plugin = setup_controller();

    // Start session and enter Waiting
    plugin.pipe(make_hook_pipe("SessionStart", 10));
    plugin.pipe(make_hook_pipe("PermissionRequest", 10));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    // Subagent tool events should NOT clear Waiting
    plugin.pipe(make_hook_pipe_with_agent_id("PreToolUse", 10, "sub-1"));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    plugin.pipe(make_hook_pipe_with_agent_id("PostToolUse", 10, "sub-1"));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    // Main agent PostToolUse SHOULD clear Waiting
    plugin.pipe(make_hook_pipe("PostToolUse", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Working);
}

#[test]
fn test_controller_badges_stored_from_hook() {
    let mut plugin = setup_controller();

    plugin.pipe(make_hook_pipe("SessionStart", 10));
    assert!(plugin.test_state().sessions[&10].badges.is_empty());

    plugin.pipe(make_hook_pipe_with_badges("PreToolUse", 10, &["🚢", "✅"]));
    assert_eq!(plugin.test_state().sessions[&10].badges, vec!["🚢", "✅"]);

    // Badges update on each hook event
    plugin.pipe(make_hook_pipe_with_badges("PostToolUse", 10, &["🌊"]));
    assert_eq!(plugin.test_state().sessions[&10].badges, vec!["🌊"]);

    // Hook without badges clears them
    plugin.pipe(make_hook_pipe("PostToolUse", 10));
    assert!(plugin.test_state().sessions[&10].badges.is_empty());
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

#[test]
fn test_controller_processes_hooks_immediately_after_grant() {
    // No election, no dormant window: the first hook after the grant lands.
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));

    plugin.pipe(make_hook_pipe("SessionStart", 42));

    assert!(plugin.test_state().sessions.contains_key(&42));
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

// ---------------------------------------------------------------------------
// Lost Permission Grant Recovery
//
// The controller is a background plugin with no client of its own, so the
// permission result Zellij addresses to a client is the likeliest of all to be
// dropped. While the grant is missing, pipe() discards every message, so a
// sidebar's hello never registers and no render is ever broadcast.
// ---------------------------------------------------------------------------

#[test]
fn test_controller_timer_reasks_for_lost_permission_grant() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    assert_eq!(plugin.test_state().permission_retries, 0);

    plugin.update(Event::Timer(1.0));

    assert_eq!(plugin.test_state().permission_retries, 1);
    assert!(!plugin.test_state().permissions_granted);
}

#[test]
fn test_controller_permission_retry_is_bounded() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    for _ in 0..(crate::sidebar_plugin::PERMISSION_RETRY_LIMIT as usize + 20) {
        plugin.update(Event::Timer(1.0));
    }

    assert_eq!(
        plugin.test_state().permission_retries,
        crate::sidebar_plugin::PERMISSION_RETRY_LIMIT
    );
}

#[test]
fn test_controller_timer_does_not_reask_once_granted() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));

    plugin.update(Event::Timer(1.0));

    assert_eq!(plugin.test_state().permission_retries, 0);
}

#[test]
fn test_controller_still_queues_non_timer_events_while_unpermissioned() {
    let mut plugin = ControllerPlugin::default();
    plugin.load(std::collections::BTreeMap::new());

    // The retry hooks into the timer only. Every other event must still be
    // queued for replay once the grant lands, exactly as before.
    plugin.update(Event::TabUpdate(vec![]));

    assert_eq!(plugin.test_state().pending_events.len(), 1);
    assert_eq!(plugin.test_state().permission_retries, 0);
}
