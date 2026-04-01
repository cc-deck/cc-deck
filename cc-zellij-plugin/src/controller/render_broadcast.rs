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
        let mut s: Vec<_> = state.sessions.values().collect();
        s.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        s
    };

    let mut waiting = 0usize;
    let mut working = 0usize;
    let mut idle = 0usize;

    let render_sessions: Vec<RenderSession> = sessions
        .iter()
        .map(|s| {
            match &s.activity {
                Activity::Waiting(_) => waiting += 1,
                Activity::Working => working += 1,
                Activity::Idle | Activity::Init => idle += 1,
                Activity::Done | Activity::AgentDone => {
                    // Done/AgentDone count as idle for summary purposes
                    idle += 1;
                }
            }

            RenderSession {
                pane_id: s.pane_id,
                display_name: s.display_name.clone(),
                activity_label: activity_label(&s.activity),
                indicator: s.activity.indicator().to_string(),
                color: s.activity.color(),
                git_branch: s.git_branch.clone(),
                tab_index: s.tab_index.unwrap_or(0),
                paused: s.paused,
                done_attended: s.done_attended,
            }
        })
        .collect();

    let total = render_sessions.len();

    RenderPayload {
        sessions: render_sessions,
        focused_pane_id: state.focused_pane_id,
        active_tab_index: state.active_tab_index.unwrap_or(0),
        notification: None,
        notification_expiry: None,
        total,
        waiting,
        working,
        idle,
        controller_plugin_id: state.plugin_id,
    }
}

/// Broadcast the render payload to all registered sidebar instances.
pub fn broadcast_render(state: &ControllerState) {
    let payload = build_render_payload(state);
    let json = match serde_json::to_string(&payload) {
        Ok(j) => j,
        Err(e) => {
            crate::debug_log(&format!("CTRL RENDER failed to serialize payload: {}", e));
            return;
        }
    };

    // Send to each registered sidebar
    for &sidebar_plugin_id in state.sidebar_registry.keys() {
        send_render_to_plugin(sidebar_plugin_id, &json);
    }

    // Also broadcast without target for any sidebars not yet registered
    broadcast_render_all(&json);
}

/// Mark render as dirty. The actual broadcast happens on the next timer flush.
pub fn mark_render_dirty(state: &mut ControllerState) {
    state.render_dirty = true;
}

/// Flush the render if dirty, then clear the flag.
pub fn flush_render(state: &mut ControllerState) {
    if state.render_dirty {
        broadcast_render(state);
        state.render_dirty = false;
    }
}

/// Human-readable label for an activity state.
fn activity_label(activity: &Activity) -> String {
    match activity {
        Activity::Init => "Init".to_string(),
        Activity::Working => "Working".to_string(),
        Activity::Waiting(reason) => {
            match reason {
                crate::session::WaitReason::Permission => "Permission".to_string(),
                crate::session::WaitReason::Notification => "Notification".to_string(),
            }
        }
        Activity::Idle => "Idle".to_string(),
        Activity::Done => "Done".to_string(),
        Activity::AgentDone => "AgentDone".to_string(),
    }
}

// --- Wasm-gated host function wrappers ---

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

/// Broadcast render payload to ALL plugins via pipe_message_to_plugin.
/// With no plugin_url and no destination_plugin_id, Zellij broadcasts
/// to every loaded plugin's pipe() handler (confirmed in Zellij source:
/// zellij-server/src/plugins/mod.rs pipe_to_all_plugins).
#[cfg(target_family = "wasm")]
fn broadcast_render_all(json: &str) {
    use zellij_tile::prelude::*;
    let mut msg = MessageToPlugin::new("cc-deck:render");
    msg.message_payload = Some(json.to_string());
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn broadcast_render_all(_json: &str) {}

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
        state.sessions.insert(
            1,
            make_session(1, "working", Activity::Working),
        );
        state.sessions.insert(
            2,
            make_session(2, "waiting", Activity::Waiting(WaitReason::Permission)),
        );
        state.sessions.insert(
            3,
            make_session(3, "idle", Activity::Idle),
        );
        state.sessions.insert(
            4,
            make_session(4, "done", Activity::Done),
        );

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
        state.focused_pane_id = Some(42);
        state.active_tab_index = Some(0);
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
        assert_eq!(payload.focused_pane_id, Some(42));
        assert_eq!(payload.controller_plugin_id, 99);
    }

    #[test]
    fn test_build_render_payload_sorted_by_tab() {
        let mut state = ControllerState::default();
        state.sessions.insert(
            1,
            make_session(1, "tab-2", Activity::Idle),
        );
        // Override tab_index to 2
        state.sessions.get_mut(&1).unwrap().tab_index = Some(2);
        state.sessions.insert(
            2,
            make_session(2, "tab-0", Activity::Idle),
        );
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
}
