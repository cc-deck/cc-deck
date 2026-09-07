// Render payload broadcast: controller -> sidebars via cc-deck:render pipe.
//
// The controller builds a RenderPayload containing pre-computed display data
// for all sessions and broadcasts it to registered sidebar instances.
// Render broadcasting is coalesced: multiple state changes within a timer
// tick produce only one broadcast.

use super::state::ControllerState;
use crate::session::Activity;
use cc_deck::{RenderPayload, RenderSession};

/// Build a RenderPayload from the current controller state.
pub fn build_render_payload(state: &ControllerState) -> RenderPayload {
    let sessions: Vec<&crate::session::Session> = {
        // Sessions awaiting confirmation from the pane manifest are withheld.
        // A row that vanishes seconds later is more confusing than one that
        // arrives seconds late, and while shown it would inflate the header
        // counts below and be clickable through to a pane that does not exist.
        let mut s: Vec<_> = state
            .sessions
            .values()
            .filter(|sess| !state.is_hidden(sess.pane_id))
            .collect();
        if let Some(ref order) = state.sort_order {
            s.sort_by_key(|sess| {
                order
                    .iter()
                    .position(|&pid| pid == sess.pane_id)
                    .unwrap_or(usize::MAX)
            });
        } else {
            s.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        }
        s
    };

    let mut waiting = 0usize;
    let mut working = 0usize;
    let mut idle = 0usize;

    let done_timeout = state.config.done_timeout;
    let idle_fade_secs = state.config.idle_fade_secs;

    // Count distinct agent types to decide whether to show indicators
    let mut agent_types = std::collections::HashSet::new();
    for s in &sessions {
        if let Some(ref name) = s.agent_name {
            agent_types.insert(name.clone());
        }
    }
    let show_agent_indicators = agent_types.len() > 1;

    let mut render_sessions: Vec<RenderSession> = sessions
        .iter()
        .map(|s| {
            match &s.activity {
                Activity::Waiting(_) => waiting += 1,
                Activity::Working => working += 1,
                Activity::Idle | Activity::Init => idle += 1,
                Activity::Done | Activity::AgentDone => {
                    idle += 1;
                }
            }

            let agent_indicator = if show_agent_indicators {
                Some(
                    s.agent_indicator
                        .clone()
                        .unwrap_or_else(|| agent_name_to_indicator(s.agent_name.as_deref())),
                )
            } else {
                None
            };

            RenderSession {
                pane_id: s.pane_id,
                display_name: s.display_name.clone(),
                activity_label: activity_label(&s.activity),
                indicator: s.activity.indicator().to_string(),
                color: crate::session::faded_color(
                    &s.activity,
                    s.elapsed_secs(),
                    done_timeout,
                    idle_fade_secs,
                ),
                git_branch: s.git_branch.clone(),
                tab_index: s.tab_index.unwrap_or(0),
                paused: s.paused,
                done_attended: s.done_attended,
                badges: s.badges.clone(),
                agent_indicator,
                in_worktree: s.in_worktree,
            }
        })
        .collect();

    // Auto-sort: stable-partition active sessions above paused ones.
    // Recently-unpaused sessions (in auto_sort_tail) go to end of active zone.
    if state.config.auto_sort {
        let mut active = Vec::new();
        let mut paused = Vec::new();
        for s in std::mem::take(&mut render_sessions) {
            if s.paused {
                paused.push(s);
            } else {
                active.push(s);
            }
        }

        if !state.auto_sort_tail.is_empty() {
            let mut head = Vec::new();
            let mut tail = Vec::new();
            for s in active {
                if state.auto_sort_tail.contains(&s.pane_id) {
                    tail.push(s);
                } else {
                    head.push(s);
                }
            }
            tail.sort_by_key(|s| {
                state
                    .auto_sort_tail
                    .iter()
                    .position(|&p| p == s.pane_id)
                    .unwrap_or(usize::MAX)
            });
            active = head;
            active.extend(tail);
        }

        paused.sort_by_key(|s| s.tab_index);

        render_sessions = active;
        render_sessions.extend(paused);
    }

    let total = render_sessions.len();

    RenderPayload {
        sessions: render_sessions,
        notification: None,
        notification_expiry: None,
        total,
        waiting,
        working,
        idle,
        controller_plugin_id: state.plugin_id,
        voice_connected: state.voice_enabled,
        voice_muted: state.voice_muted,
        show_agent_indicators,
        sort_active: state.sort_order.is_some(),
        client_views: state
            .client_views
            .iter()
            .map(|(&client_id, view)| (client_id, view.snapshot()))
            .collect(),
        multiplayer_colors: state.multiplayer_colors.clone(),
    }
}

/// Map an agent name to a short indicator string for sidebar display.
fn agent_name_to_indicator(name: Option<&str>) -> String {
    match name {
        Some("claude") => "\u{2733}".to_string(),   // ✳
        Some("opencode") => "\u{276f}".to_string(), // ❯
        Some(other) => {
            let upper: String = other.chars().take(2).collect::<String>().to_uppercase();
            if upper.is_empty() {
                "?".to_string()
            } else {
                upper
            }
        }
        None => "?".to_string(),
    }
}

/// Broadcast the render payload to all registered sidebar instances.
pub fn broadcast_render(state: &ControllerState) {
    let start_us = crate::session::unix_now_ms().saturating_mul(1000);

    let payload = build_render_payload(state);
    let json = match serde_json::to_string(&payload) {
        Ok(j) => j,
        Err(e) => {
            crate::debug_log(&format!("CTRL RENDER failed to serialize payload: {}", e));
            return;
        }
    };

    let serialization_us = crate::session::unix_now_ms()
        .saturating_mul(1000)
        .saturating_sub(start_us);

    let mut send_count: u64 = 0;
    for &sidebar_plugin_id in state.sidebar_registry.keys() {
        send_render_to_plugin(sidebar_plugin_id, &json);
        send_count += 1;
    }

    // Untargeted broadcast as fallback for sidebars not yet in the registry.
    // Only fire when the registry is empty (no known sidebars). Once sidebars
    // register, targeted sends above handle delivery. This prevents the
    // untargeted broadcast from bypassing the client_id filter (FR-005).
    if state.sidebar_registry.is_empty() {
        broadcast_render_all(&json);
    }

    if state.perf.enabled {
        crate::debug_log(&format!(
            "CTRL RENDER broadcast: sidebars={} serialization_us={}",
            send_count, serialization_us
        ));
    }
}

/// Flush the render if dirty, then clear the flag.
pub fn flush_render(state: &mut ControllerState) {
    if state.render_dirty {
        // Log waiting sessions to diagnose missing attention icons
        let waiting_panes: Vec<u32> = state
            .sessions
            .values()
            .filter(|s| s.activity.is_waiting())
            .map(|s| s.pane_id)
            .collect();
        if !waiting_panes.is_empty() {
            crate::debug_log(&format!("CTRL FLUSH: waiting panes={waiting_panes:?}"));
        }
        state.perf.record_raw("render:broadcast", 1);
        let sidebar_count = state.sidebar_registry.len() as u64;
        state.perf.record_raw("render:pipe_send", sidebar_count);
        broadcast_render(state);
        state.render_dirty = false;
    } else {
        state.perf.record_raw("render:skipped", 1);
    }
}

/// Human-readable label for an activity state.
fn activity_label(activity: &Activity) -> String {
    match activity {
        Activity::Init => "Init".to_string(),
        Activity::Working => "Working".to_string(),
        Activity::Waiting(reason) => match reason {
            crate::session::WaitReason::Permission => "Permission".to_string(),
            crate::session::WaitReason::Notification => "Notification".to_string(),
        },
        Activity::Idle => "Idle".to_string(),
        Activity::Done => "Done".to_string(),
        Activity::AgentDone => "AgentDone".to_string(),
    }
}

// --- Wasm-gated host function wrappers ---

/// Broadcast an untargeted render payload to all sidebar instances.
/// Provides a fallback for sidebars not yet in the registry.
#[cfg(target_family = "wasm")]
fn broadcast_render_all(json: &str) {
    use zellij_tile::prelude::*;
    let mut msg = MessageToPlugin::new("cc-deck:render");
    msg.message_payload = Some(json.to_string());
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn broadcast_render_all(_json: &str) {}

/// Send a render payload to a specific sidebar plugin by ID.
#[cfg(target_family = "wasm")]
fn send_render_to_plugin(plugin_id: u32, json: &str) {
    use zellij_tile::prelude::*;
    let mut msg = MessageToPlugin::new("cc-deck:render");
    msg.message_payload = Some(json.to_string());
    msg.destination_plugin_id = Some(plugin_id);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn send_render_to_plugin(_plugin_id: u32, _json: &str) {}

/// Public wrapper for send_render_to_plugin (used by controller for targeted renders).
/// Build and send the current render payload to a single sidebar plugin.
pub fn targeted_render(state: &ControllerState, plugin_id: u32) {
    let payload = build_render_payload(state);
    match serde_json::to_string(&payload) {
        Ok(json) => send_render_to_plugin(plugin_id, &json),
        Err(e) => {
            crate::debug_log(&format!(
                "CTRL RENDER targeted_render failed to serialize for plugin_id={}: {}",
                plugin_id, e
            ));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::{Activity, Session, WaitReason};

    fn make_session(pane_id: u32, name: &str, activity: Activity) -> Session {
        let mut s = Session::new(pane_id, format!("session-{pane_id}"));
        s.display_name = name.to_string();
        s.activity = activity;
        s.tab_index = Some(pane_id as usize);
        s
    }

    #[test]
    fn test_render_payload_omits_hook_quarantined_sessions() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "real", Activity::Working));
        state
            .sessions
            .insert(2, make_session(2, "phantom", Activity::Working));
        state.quarantine(2, crate::controller::state::QuarantineKind::Hook);

        let payload = build_render_payload(&state);

        assert_eq!(payload.sessions.len(), 1, "the phantom row must not render");
        assert_eq!(payload.sessions[0].display_name, "real");
        assert_eq!(payload.total, 1, "header count must exclude it too");
        assert_eq!(payload.working, 1, "activity counters must exclude it too");
    }

    /// Sessions restored from disk stay visible through their grace window,
    /// otherwise the sidebar would blank on every reattach.
    #[test]
    fn test_render_payload_includes_restored_quarantined_sessions() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "restored", Activity::Idle));
        state.quarantine(1, crate::controller::state::QuarantineKind::Restored);

        let payload = build_render_payload(&state);

        assert_eq!(payload.sessions.len(), 1);
        assert_eq!(payload.total, 1);
    }

    #[test]
    fn test_build_render_payload_empty() {
        let state = ControllerState::default();
        let payload = build_render_payload(&state);
        assert!(payload.sessions.is_empty());
        assert_eq!(payload.total, 0);
        assert_eq!(payload.waiting, 0);
        assert_eq!(payload.working, 0);
        assert_eq!(payload.idle, 0);
    }

    #[test]
    fn test_build_render_payload_counts() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "working", Activity::Working));
        state.sessions.insert(
            2,
            make_session(2, "waiting", Activity::Waiting(WaitReason::Permission)),
        );
        state
            .sessions
            .insert(3, make_session(3, "idle", Activity::Idle));
        state
            .sessions
            .insert(4, make_session(4, "done", Activity::Done));

        let payload = build_render_payload(&state);
        assert_eq!(payload.total, 4);
        assert_eq!(payload.working, 1);
        assert_eq!(payload.waiting, 1);
        assert_eq!(payload.idle, 2); // Idle + Done both counted
    }

    #[test]
    fn test_build_render_payload_session_data() {
        let mut state = ControllerState::default();
        let mut s = make_session(42, "api-server", Activity::Working);
        s.git_branch = Some("main".to_string());
        s.paused = false;
        state.sessions.insert(42, s);
        state.set_client_focus_intent(state.client_id, 42, 0);
        state.plugin_id = 99;

        let payload = build_render_payload(&state);
        assert_eq!(payload.sessions.len(), 1);
        let rs = &payload.sessions[0];
        assert_eq!(rs.pane_id, 42);
        assert_eq!(rs.display_name, "api-server");
        assert_eq!(rs.activity_label, "Working");
        assert_eq!(rs.indicator, "\u{25cf}"); // ●
        assert_eq!(rs.color, (180, 140, 255));
        assert_eq!(rs.git_branch, Some("main".to_string()));
        assert!(!rs.paused);
        assert_eq!(
            payload.client_views[&state.client_id].focused_pane_id,
            Some(42)
        );
        assert_eq!(payload.controller_plugin_id, 99);
    }

    #[test]
    fn test_build_render_payload_sorted_by_tab() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "tab-2", Activity::Idle));
        // Override tab_index to 2
        state.sessions.get_mut(&1).unwrap().tab_index = Some(2);
        state
            .sessions
            .insert(2, make_session(2, "tab-0", Activity::Idle));
        state.sessions.get_mut(&2).unwrap().tab_index = Some(0);

        let payload = build_render_payload(&state);
        assert_eq!(payload.sessions[0].display_name, "tab-0");
        assert_eq!(payload.sessions[1].display_name, "tab-2");
    }

    #[test]
    fn test_activity_labels() {
        assert_eq!(activity_label(&Activity::Init), "Init");
        assert_eq!(activity_label(&Activity::Working), "Working");
        assert_eq!(
            activity_label(&Activity::Waiting(WaitReason::Permission)),
            "Permission"
        );
        assert_eq!(
            activity_label(&Activity::Waiting(WaitReason::Notification)),
            "Notification"
        );
        assert_eq!(activity_label(&Activity::Idle), "Idle");
        assert_eq!(activity_label(&Activity::Done), "Done");
        assert_eq!(activity_label(&Activity::AgentDone), "AgentDone");
    }

    #[test]
    fn test_flush_render_clears_dirty() {
        let mut state = ControllerState::default();
        state.render_dirty = true;
        flush_render(&mut state);
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_flush_render_noop_when_clean() {
        let mut state = ControllerState::default();
        assert!(!state.render_dirty);
        flush_render(&mut state);
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_broadcast_render_calls_both_targeted_and_untargeted() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "test", Activity::Working));
        state.sidebar_registry.insert(42, (0, 0));
        state.sidebar_registry.insert(43, (1, 0));

        // In non-WASM test mode, both send_render_to_plugin and
        // broadcast_render_all are no-ops. This test verifies broadcast_render
        // completes without panic and processes the registry.
        broadcast_render(&state);
    }

    #[test]
    fn test_targeted_render_builds_payload_without_panic() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "test-session", Activity::Working));
        state.voice_enabled = true;
        state.voice_muted = false;

        // In non-WASM test mode, send_render_to_plugin is a no-op.
        // This test verifies targeted_render builds a valid payload
        // and does not panic. Actual delivery is verified via WASM
        // integration tests.
        targeted_render(&state, 42);

        let payload = build_render_payload(&state);
        assert_eq!(payload.sessions.len(), 1);
        assert!(payload.voice_connected);
        assert!(!payload.voice_muted);
    }

    #[test]
    fn test_agent_name_to_indicator_known() {
        assert_eq!(agent_name_to_indicator(Some("claude")), "\u{2733}"); // ✳
        assert_eq!(agent_name_to_indicator(Some("opencode")), "\u{276f}"); // ❯
    }

    #[test]
    fn test_agent_name_to_indicator_unknown() {
        assert_eq!(agent_name_to_indicator(Some("foo")), "FO");
        assert_eq!(agent_name_to_indicator(None), "?");
        assert_eq!(agent_name_to_indicator(Some("")), "?");
    }

    #[test]
    fn test_build_render_payload_shows_indicators_when_mixed_agents() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1, "api", Activity::Working);
        s1.agent_name = Some("claude".to_string());
        let mut s2 = make_session(2, "web", Activity::Working);
        s2.agent_name = Some("opencode".to_string());
        state.sessions.insert(1, s1);
        state.sessions.insert(2, s2);

        let payload = build_render_payload(&state);
        assert!(payload.show_agent_indicators);
        assert!(payload.sessions.iter().all(|s| s.agent_indicator.is_some()));
    }

    #[test]
    fn test_build_render_payload_hides_indicators_when_same_agent() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1, "api", Activity::Working);
        s1.agent_name = Some("claude".to_string());
        let mut s2 = make_session(2, "web", Activity::Working);
        s2.agent_name = Some("claude".to_string());
        state.sessions.insert(1, s1);
        state.sessions.insert(2, s2);

        let payload = build_render_payload(&state);
        assert!(!payload.show_agent_indicators);
        assert!(payload.sessions.iter().all(|s| s.agent_indicator.is_none()));
    }

    #[test]
    fn test_build_render_payload_hides_indicators_when_no_agent() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(1, make_session(1, "api", Activity::Working));
        state
            .sessions
            .insert(2, make_session(2, "web", Activity::Working));

        let payload = build_render_payload(&state);
        assert!(!payload.show_agent_indicators);
    }

    // --- Virtual sort tests (T007, T008) ---

    #[test]
    fn test_build_render_payload_uses_frozen_sort_order() {
        let mut state = ControllerState::default();
        // Frozen order: work1, work2, idle, done, paused
        state.sort_order = Some(vec![20, 50, 10, 40, 30]);

        let mut s0 = make_session(10, "idle", Activity::Idle);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "work1", Activity::Working);
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "paused", Activity::Idle);
        s2.paused = true;
        s2.tab_index = Some(2);
        let mut s3 = make_session(40, "done", Activity::Done);
        s3.tab_index = Some(3);
        let mut s4 = make_session(50, "work2", Activity::Working);
        s4.tab_index = Some(4);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);
        state.sessions.insert(50, s4);

        let payload = build_render_payload(&state);
        assert!(payload.sort_active);

        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        assert_eq!(names, vec!["work1", "work2", "idle", "done", "paused"]);
    }

    #[test]
    fn test_build_render_payload_preserves_tab_order_without_sort() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "idle", Activity::Idle);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "work", Activity::Working);
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "paused", Activity::Idle);
        s2.paused = true;
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);

        let payload = build_render_payload(&state);
        assert!(!payload.sort_active);

        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        assert_eq!(names, vec!["idle", "work", "paused"]);
    }

    #[test]
    fn test_build_render_payload_frozen_order_stable_across_state_changes() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "idle1", Activity::Idle);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "work-a", Activity::Working);
        s1.tab_index = Some(1);
        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);

        // Frozen order: work-a first, then idle1
        state.sort_order = Some(vec![20, 10]);

        // Change work-a to Idle — frozen order should NOT change
        state.sessions.get_mut(&20).unwrap().activity = Activity::Idle;

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        assert_eq!(names, vec!["work-a", "idle1"]);
    }

    // --- Auto-sort partition tests ---

    #[test]
    fn test_auto_sort_partitions_active_above_paused() {
        let mut state = ControllerState::default();
        // auto_sort defaults to true

        let mut s0 = make_session(10, "active1", Activity::Working);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "paused1", Activity::Idle);
        s1.paused = true;
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "active2", Activity::Idle);
        s2.tab_index = Some(2);
        let mut s3 = make_session(40, "paused2", Activity::Idle);
        s3.paused = true;
        s3.tab_index = Some(3);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        assert_eq!(names, vec!["active1", "active2", "paused1", "paused2"]);
    }

    #[test]
    fn test_auto_sort_no_separator_all_active() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "a", Activity::Working);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "b", Activity::Idle);
        s1.tab_index = Some(1);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);

        let _payload = build_render_payload(&state);
    }

    #[test]
    fn test_auto_sort_no_separator_all_paused() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "a", Activity::Idle);
        s0.paused = true;
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "b", Activity::Idle);
        s1.paused = true;
        s1.tab_index = Some(1);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);

        let _payload = build_render_payload(&state);
    }

    #[test]
    fn test_auto_sort_single_session_no_separator() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "solo", Activity::Working);
        s0.tab_index = Some(0);
        state.sessions.insert(10, s0);

        let payload = build_render_payload(&state);
        assert_eq!(payload.sessions.len(), 1);
    }

    #[test]
    fn test_auto_sort_tail_moves_unpaused_to_end_of_active() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "orig1", Activity::Working);
        s0.tab_index = Some(1);
        let mut s1 = make_session(20, "orig2", Activity::Idle);
        s1.tab_index = Some(2);
        let mut s2 = make_session(30, "recently-unpaused", Activity::Idle);
        s2.tab_index = Some(0);
        let mut s3 = make_session(40, "paused", Activity::Idle);
        s3.paused = true;
        s3.tab_index = Some(3);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);
        state.auto_sort_tail = vec![30];

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        // Without auto_sort_tail, tab_index order would be: recently-unpaused(0), orig1(1), orig2(2)
        // With auto_sort_tail, recently-unpaused moves to end of active zone
        assert_eq!(names, vec!["orig1", "orig2", "recently-unpaused", "paused"]);
    }

    #[test]
    fn test_auto_sort_disabled_no_partition() {
        let mut state = ControllerState::default();
        state.config.auto_sort = false;

        let mut s0 = make_session(10, "active", Activity::Working);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "paused", Activity::Idle);
        s1.paused = true;
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "active2", Activity::Idle);
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        // No partition: tab order preserved
        assert_eq!(names, vec!["active", "paused", "active2"]);
    }

    #[test]
    fn test_auto_sort_preserves_relative_order_within_zones() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "a-active", Activity::Idle);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "b-paused", Activity::Idle);
        s1.paused = true;
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "c-active", Activity::Working);
        s2.tab_index = Some(2);
        let mut s3 = make_session(40, "d-paused", Activity::Idle);
        s3.paused = true;
        s3.tab_index = Some(3);
        let mut s4 = make_session(50, "e-active", Activity::Idle);
        s4.tab_index = Some(4);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);
        state.sessions.insert(50, s4);

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        // Both zones preserve their existing relative order. Activity does
        // not affect position inside the active zone.
        assert_eq!(
            names,
            vec!["a-active", "c-active", "e-active", "b-paused", "d-paused"]
        );
    }

    #[test]
    fn test_auto_sort_with_manual_sort_active_zone_only() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "idle1", Activity::Idle);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "work1", Activity::Working);
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "paused1", Activity::Idle);
        s2.paused = true;
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);

        // Manual sort order for active sessions only (work1 first, idle1 second)
        state.sort_order = Some(vec![20, 10]);

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        // Manual sort within active zone, paused stays below separator
        assert_eq!(names, vec!["work1", "idle1", "paused1"]);
    }

    #[test]
    fn test_auto_sort_empty_sessions() {
        let state = ControllerState::default();
        let payload = build_render_payload(&state);
        assert!(payload.sessions.is_empty());
    }

    #[test]
    fn test_auto_sort_tail_order_preserved() {
        let mut state = ControllerState::default();

        let mut s0 = make_session(10, "orig", Activity::Working);
        s0.tab_index = Some(0);
        let mut s1 = make_session(20, "unpaused-first", Activity::Idle);
        s1.tab_index = Some(1);
        let mut s2 = make_session(30, "unpaused-second", Activity::Idle);
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        // Unpaused in order: 20 first, then 30
        state.auto_sort_tail = vec![20, 30];

        let payload = build_render_payload(&state);
        let names: Vec<&str> = payload
            .sessions
            .iter()
            .map(|s| s.display_name.as_str())
            .collect();
        assert_eq!(names, vec!["orig", "unpaused-first", "unpaused-second"]);
    }

    // --- T012: Broadcast filtering by client_id ---

    #[test]
    fn test_broadcast_render_sends_to_all_clients() {
        // All registered sidebars receive renders, regardless of client_id.
        // Deduplication by (tab, client_id) in the registry caps the count.
        let mut state = ControllerState::default();
        state.client_id = 1;
        state
            .sessions
            .insert(1, make_session(1, "test", Activity::Working));

        // Two sidebars from client 1, one from client 2 (live second client)
        state.sidebar_registry.insert(42, (0, 1));
        state.sidebar_registry.insert(43, (1, 1));
        state.sidebar_registry.insert(44, (0, 2));

        // broadcast_render should send to all three.
        broadcast_render(&state);

        // Registry unchanged (broadcast is read-only)
        assert_eq!(state.sidebar_registry.len(), 3);
    }

    #[test]
    fn test_broadcast_render_all_skipped_when_registry_nonempty() {
        // When the sidebar registry has entries, broadcast_render_all
        // (untargeted fallback) should NOT be called. In non-WASM mode
        // both paths are no-ops, but we verify the function completes
        // and the guard condition is correct.
        let mut state = ControllerState::default();
        state.client_id = 1;
        state
            .sessions
            .insert(1, make_session(1, "test", Activity::Working));
        state.sidebar_registry.insert(42, (0, 1));

        // With a non-empty registry, broadcast_render_all is skipped.
        broadcast_render(&state);
    }

    // --- Multiplayer client views in payload tests ---

    #[test]
    fn test_build_render_payload_includes_client_views() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(10, make_session(10, "api", Activity::Working));
        state
            .sessions
            .insert(20, make_session(20, "web", Activity::Idle));
        state.set_client_focus_intent(1, 10, 0);
        state.set_client_focus_intent(2, 20, 1);

        let payload = build_render_payload(&state);
        assert_eq!(payload.client_views.len(), 2);
        assert_eq!(payload.client_views[&1].focused_pane_id, Some(10));
        assert_eq!(payload.client_views[&2].focused_pane_id, Some(20));
    }

    #[test]
    fn test_build_render_payload_includes_multiplayer_colors() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(10, make_session(10, "test", Activity::Working));
        let colors = vec![(255, 0, 0); 10];
        state.multiplayer_colors = Some(colors.clone());

        let payload = build_render_payload(&state);
        assert_eq!(payload.multiplayer_colors, Some(colors));
    }

    #[test]
    fn test_build_render_payload_has_independent_focus() {
        let mut state = ControllerState::default();
        state.client_id = 1;
        state
            .sessions
            .insert(10, make_session(10, "api", Activity::Working));
        state
            .sessions
            .insert(20, make_session(20, "web", Activity::Idle));
        state.set_client_focus_intent(1, 10, 0);
        state.set_client_focus_intent(2, 20, 1);

        let payload = build_render_payload(&state);
        assert_eq!(payload.client_views[&1].focused_pane_id, Some(10));
        assert_eq!(payload.client_views[&2].focused_pane_id, Some(20));
    }
}
