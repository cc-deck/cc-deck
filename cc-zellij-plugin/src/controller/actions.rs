// Controller action processing: handle ActionMessage from sidebars.
//
// Sidebars send user-initiated actions to the controller via cc-deck:action
// pipe messages. The controller processes them, updates authoritative state,
// and broadcasts an updated RenderPayload.

use super::state::ControllerState;
use crate::session::{self, deduplicate_name, Activity, Session, WaitReason};
use cc_deck::{ActionMessage, ActionType};

/// Process an action message from a sidebar plugin.
pub fn handle_action(state: &mut ControllerState, msg: ActionMessage) {
    match msg.action {
        ActionType::Switch => handle_switch(state, msg.pane_id, msg.tab_index),
        ActionType::Rename => handle_rename(state, msg.pane_id, msg.value),
        ActionType::Delete => handle_delete(state, msg.pane_id),
        ActionType::Pause => handle_pause(state, msg.pane_id),
        ActionType::Attend => handle_attend(state),
        ActionType::AttendPrev => handle_attend_prev(state),
        ActionType::Working => handle_working(state),
        ActionType::WorkingPrev => handle_working_prev(state),
        ActionType::Navigate => handle_navigate(state, msg.pane_id, msg.tab_index),
        ActionType::NewSession => handle_new_session(state),
        ActionType::Refresh => handle_refresh(state),
    }
}

/// Switch to a specific session (focus its pane and tab).
fn handle_switch(state: &mut ControllerState, pane_id: Option<u32>, tab_index: Option<usize>) {
    if let (Some(pid), Some(tab_idx)) = (pane_id, tab_index) {
        // Auto-unpause on switch
        if let Some(s) = state.sessions.get_mut(&pid) {
            if s.paused {
                s.paused = false;
                s.last_event_ts = crate::session::unix_now();
            }
        }
        state.focused_pane_id = Some(pid);
        state.active_tab_index = Some(tab_idx);
        state.last_attended_pane_id = Some(pid);
        state.in_flight_focus = Some((pid, crate::session::unix_now_ms()));
        write_last_attended(pid);
        crate::debug_log(&format!("CTRL SWITCH: pid={pid} tab={tab_idx} in_flight={pid}"));
        // Broadcast BEFORE the tab switch so pipe messages are queued in Zellij
        // ahead of the switch_tab_to command. This gives the target sidebar a
        // chance to cache the new focus before Zellij renders the new tab,
        // preventing the highlight flash on cross-tab switches.
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_idx);
        focus_terminal_pane_wasm(pid);
    }
}

/// Rename a session by pane_id.
fn handle_rename(state: &mut ControllerState, pane_id: Option<u32>, value: Option<String>) {
    let pid = match pane_id {
        Some(p) => p,
        None => return,
    };
    let new_name = match value {
        Some(ref v) => v.trim().to_string(),
        None => return,
    };
    if new_name.is_empty() {
        return;
    }

    let names = state.session_names_except(pid);
    let final_name = deduplicate_name(&new_name, &names);
    let now = session::unix_now();

    let tab_index = if let Some(session) = state.sessions.get_mut(&pid) {
        session.display_name = final_name.clone();
        session.manually_renamed = true;
        session.last_event_ts = now;
        session.meta_ts = now;
        session.tab_index
    } else {
        return;
    };

    // Rename the Zellij tab if this is the sole session on it
    if let Some(idx) = tab_index {
        let sessions_on_tab = state
            .sessions
            .values()
            .filter(|s| s.tab_index == Some(idx))
            .count();
        if sessions_on_tab <= 1 {
            rename_tab_wasm(idx, &final_name);
        }
    }

    state.save_sessions();
    state.mark_render_dirty();
}

/// Delete a session: remove from state and close its pane.
fn handle_delete(state: &mut ControllerState, pane_id: Option<u32>) {
    let pid = match pane_id {
        Some(p) => p,
        None => return,
    };

    let session_info = state.sessions.get(&pid).map(|s| {
        let tab_idx = s.tab_index;
        let is_only = tab_idx
            .map(|idx| {
                state
                    .sessions
                    .values()
                    .filter(|s2| s2.tab_index == Some(idx))
                    .count()
                    <= 1
            })
            .unwrap_or(false);
        (tab_idx, is_only)
    });

    state.sessions.remove(&pid);
    state.pending_git_branch.remove(&pid);

    if let Some((tab_idx, is_only)) = session_info {
        close_session_pane_wasm(pid, tab_idx, is_only);
    }

    state.save_sessions();
    state.mark_render_dirty();
}

/// Toggle pause on a session.
fn handle_pause(state: &mut ControllerState, pane_id: Option<u32>) {
    let pid = match pane_id {
        Some(p) => p,
        None => return,
    };
    if let Some(s) = state.sessions.get_mut(&pid) {
        s.paused = !s.paused;
        let now = session::unix_now();
        s.last_event_ts = now;
        s.meta_ts = now;
    }
    state.save_sessions();
    state.mark_render_dirty();
}

/// Smart attend: find the next session needing attention using tiered priority.
fn handle_attend(state: &mut ControllerState) {
    let result = perform_attend_directed(state, AttendDirection::Forward);
    if let Some((pane_id, tab_index)) = result {
        state.focused_pane_id = Some(pane_id);
        state.active_tab_index = Some(tab_index);
        state.in_flight_focus = Some((pane_id, crate::session::unix_now_ms()));
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_index);
        focus_terminal_pane_wasm(pane_id);
    }
}

/// Smart attend in reverse direction.
fn handle_attend_prev(state: &mut ControllerState) {
    let result = perform_attend_directed(state, AttendDirection::Backward);
    if let Some((pane_id, tab_index)) = result {
        state.focused_pane_id = Some(pane_id);
        state.active_tab_index = Some(tab_index);
        state.in_flight_focus = Some((pane_id, crate::session::unix_now_ms()));
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_index);
        focus_terminal_pane_wasm(pane_id);
    }
}

/// Cycle through working sessions (Alt+w).
fn handle_working(state: &mut ControllerState) {
    let result = perform_working_directed(state, AttendDirection::Forward);
    if let Some((pane_id, tab_index)) = result {
        state.focused_pane_id = Some(pane_id);
        state.active_tab_index = Some(tab_index);
        state.last_attended_pane_id = Some(pane_id);
        state.in_flight_focus = Some((pane_id, crate::session::unix_now_ms()));
        write_last_attended(pane_id);
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_index);
        focus_terminal_pane_wasm(pane_id);
    }
}

/// Cycle through working sessions in reverse (Shift+Alt+w).
fn handle_working_prev(state: &mut ControllerState) {
    let result = perform_working_directed(state, AttendDirection::Backward);
    if let Some((pane_id, tab_index)) = result {
        state.focused_pane_id = Some(pane_id);
        state.active_tab_index = Some(tab_index);
        state.last_attended_pane_id = Some(pane_id);
        state.in_flight_focus = Some((pane_id, crate::session::unix_now_ms()));
        write_last_attended(pane_id);
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_index);
        focus_terminal_pane_wasm(pane_id);
    }
}

/// Navigate action: switch to the specified pane (forwarded from sidebar).
fn handle_navigate(
    state: &mut ControllerState,
    pane_id: Option<u32>,
    tab_index: Option<usize>,
) {
    // Navigate is equivalent to switch for the controller.
    // The sidebar handles cursor state locally and sends a Switch
    // when the user selects. This handler covers keybinding-triggered navigate.
    if let (Some(pid), Some(tab_idx)) = (pane_id, tab_index) {
        state.focused_pane_id = Some(pid);
        state.active_tab_index = Some(tab_idx);
        state.last_attended_pane_id = Some(pid);
        state.in_flight_focus = Some((pid, crate::session::unix_now_ms()));
        write_last_attended(pid);
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
        switch_tab_to_wasm(tab_idx);
        focus_terminal_pane_wasm(pid);
    }
}

/// Force refresh: restore persisted sessions and broadcast.
fn handle_refresh(state: &mut ControllerState) {
    let restored = ControllerState::restore_sessions();
    if !restored.is_empty() {
        state.merge_sessions(restored);
    }
    state.mark_render_dirty();
}

/// Create a new session tab.
fn handle_new_session(state: &mut ControllerState) {
    match state.config.new_session_mode {
        crate::config::NewSessionMode::Tab => new_tab_wasm(),
        crate::config::NewSessionMode::Pane => new_session_pane_wasm(),
    }
}

// --- Tiered attend logic (adapted from crate::attend) ---

enum AttendDirection {
    Forward,
    Backward,
}

const ATTEND_STATE_PATH: &str = "/cache/attend-state.json";

/// Lightweight candidate data extracted from Session to avoid borrow conflicts.
#[derive(Clone)]
struct AttendCandidate {
    pane_id: u32,
    tab_index: Option<usize>,
    is_done: bool,
}

/// Perform tiered attend. Returns (pane_id, tab_index) if a session was found.
fn perform_attend_directed(
    state: &mut ControllerState,
    direction: AttendDirection,
) -> Option<(u32, usize)> {
    // Build candidate tiers from session data, then drop the borrow.
    let tiers: Vec<Vec<AttendCandidate>> = {
        let sessions = state.sessions_by_tab_order();
        if sessions.is_empty() {
            return None;
        }

        // Tier 1: Waiting (Permission first, then Notification, oldest first)
        let mut waiting: Vec<_> = sessions
            .iter()
            .filter(|s| !s.paused && matches!(s.activity, Activity::Waiting(_)))
            .copied()
            .collect();
        waiting.sort_by(|a, b| {
            let a_perm = matches!(a.activity, Activity::Waiting(WaitReason::Permission));
            let b_perm = matches!(b.activity, Activity::Waiting(WaitReason::Permission));
            b_perm
                .cmp(&a_perm)
                .then(a.last_event_ts.cmp(&b.last_event_ts))
        });

        // Tier 2: Done/AgentDone not yet attended (most recent first)
        let mut done: Vec<_> = sessions
            .iter()
            .filter(|s| {
                !s.paused
                    && !s.done_attended
                    && matches!(s.activity, Activity::Done | Activity::AgentDone)
            })
            .copied()
            .collect();
        done.sort_by(|a, b| b.last_event_ts.cmp(&a.last_event_ts));

        // Tier 3: Idle/Init + already-attended Done (most recent first)
        let mut idle: Vec<_> = sessions
            .iter()
            .filter(|s| {
                !s.paused
                    && (matches!(s.activity, Activity::Idle | Activity::Init)
                        || (s.done_attended
                            && matches!(s.activity, Activity::Done | Activity::AgentDone)))
            })
            .copied()
            .collect();
        idle.sort_by(|a, b| b.last_event_ts.cmp(&a.last_event_ts));

        // Convert to lightweight candidates and drop the session borrows.
        let to_candidates = |tier: Vec<&Session>| -> Vec<AttendCandidate> {
            tier.iter()
                .map(|s| AttendCandidate {
                    pane_id: s.pane_id,
                    tab_index: s.tab_index,
                    is_done: matches!(s.activity, Activity::Done | Activity::AgentDone),
                })
                .collect()
        };

        [to_candidates(waiting), to_candidates(done), to_candidates(idle)]
            .into_iter()
            .filter(|t| !t.is_empty())
            .collect()
    };

    if tiers.is_empty() {
        return None;
    }

    let result = cycle_through_tiers(state, &tiers, direction);

    if let Some((pane_id, _)) = result {
        write_last_attended(pane_id);
        let is_done = tiers.iter().flatten().any(|c| c.pane_id == pane_id && c.is_done);
        if is_done {
            if let Some(session) = state.sessions.get_mut(&pane_id) {
                session.done_attended = true;
            }
        }
    }

    result
}

/// Cycle through working sessions. Tier 1: Working.
/// Uses the same rapid-cycle visited-set logic as attend.
fn perform_working_directed(
    state: &mut ControllerState,
    direction: AttendDirection,
) -> Option<(u32, usize)> {
    let tiers: Vec<Vec<AttendCandidate>> = {
        let sessions = state.sessions_by_tab_order();
        if sessions.is_empty() {
            return None;
        }

        let mut working: Vec<_> = sessions
            .iter()
            .filter(|s| !s.paused && matches!(s.activity, Activity::Working))
            .copied()
            .collect();
        working.sort_by(|a, b| b.last_event_ts.cmp(&a.last_event_ts));

        let candidates: Vec<AttendCandidate> = working
            .iter()
            .map(|s| AttendCandidate {
                pane_id: s.pane_id,
                tab_index: s.tab_index,
                is_done: false,
            })
            .collect();

        if candidates.is_empty() {
            vec![]
        } else {
            vec![candidates]
        }
    };

    if tiers.is_empty() {
        return None;
    }

    cycle_through_tiers(state, &tiers, direction)
}

/// Shared rapid-cycle iteration logic for directed navigation.
/// Manages visited-set, direction-based candidate selection, and wrap-around retry.
fn cycle_through_tiers(
    state: &mut ControllerState,
    tiers: &[Vec<AttendCandidate>],
    direction: AttendDirection,
) -> Option<(u32, usize)> {
    let now_ms = crate::session::unix_now_ms();
    let in_rapid_cycle = state.config.attend_cycle_ms > 0
        && now_ms.saturating_sub(state.last_attend_ms) < state.config.attend_cycle_ms;

    if !in_rapid_cycle {
        state.attend_visited.clear();
    }

    if let Some(fpid) = state.focused_pane_id {
        state.attend_visited.insert(fpid);
    }

    for attempt in 0..2 {
        for candidates in tiers {
            let pick = match direction {
                AttendDirection::Forward => {
                    candidates.iter().find(|c| !state.attend_visited.contains(&c.pane_id))
                }
                AttendDirection::Backward => {
                    candidates.iter().rev().find(|c| !state.attend_visited.contains(&c.pane_id))
                }
            };

            if let Some(candidate) = pick {
                let pane_id = candidate.pane_id;
                let tab_index = match candidate.tab_index {
                    Some(t) => t,
                    None => continue,
                };

                state.attend_visited.insert(pane_id);
                state.last_attend_ms = now_ms;
                state.last_attended_pane_id = Some(pane_id);

                return Some((pane_id, tab_index));
            }
        }

        if attempt == 0 && in_rapid_cycle {
            state.attend_visited.clear();
            if let Some(fpid) = state.focused_pane_id {
                state.attend_visited.insert(fpid);
            }
        } else {
            break;
        }
    }

    None
}

fn write_last_attended(pane_id: u32) {
    let _ = std::fs::write(ATTEND_STATE_PATH, pane_id.to_string());
}

// --- Wasm-gated host function wrappers ---

#[cfg(target_family = "wasm")]
fn switch_tab_to_wasm(tab_idx: usize) {
    zellij_tile::prelude::switch_tab_to(tab_idx as u32 + 1);
}

#[cfg(not(target_family = "wasm"))]
fn switch_tab_to_wasm(_tab_idx: usize) {}

#[cfg(target_family = "wasm")]
fn focus_terminal_pane_wasm(pane_id: u32) {
    zellij_tile::prelude::focus_terminal_pane(pane_id, false, false);
}

#[cfg(not(target_family = "wasm"))]
fn focus_terminal_pane_wasm(_pane_id: u32) {}

#[cfg(target_family = "wasm")]
fn rename_tab_wasm(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn rename_tab_wasm(_tab_idx: usize, _name: &str) {}

#[cfg(target_family = "wasm")]
fn close_session_pane_wasm(pane_id: u32, tab_index: Option<usize>, is_only_pane: bool) {
    zellij_tile::prelude::close_terminal_pane(pane_id);
    if is_only_pane {
        if let Some(idx) = tab_index {
            zellij_tile::prelude::close_tab_with_index(idx);
        }
    }
}

#[cfg(not(target_family = "wasm"))]
fn close_session_pane_wasm(_pane_id: u32, _tab_index: Option<usize>, _is_only_pane: bool) {}

#[cfg(target_family = "wasm")]
fn new_tab_wasm() {
    zellij_tile::prelude::new_tab(None::<&str>, None::<&str>);
}

#[cfg(not(target_family = "wasm"))]
fn new_tab_wasm() {}

#[cfg(target_family = "wasm")]
fn new_session_pane_wasm() {
    use std::collections::BTreeMap;
    let mut context = BTreeMap::new();
    context.insert("cc-deck".to_string(), "new-session".to_string());
    zellij_tile::prelude::open_command_pane(
        zellij_tile::prelude::CommandToRun {
            path: std::path::PathBuf::from("claude"),
            args: vec![],
            cwd: None,
        },
        context,
    );
}

#[cfg(not(target_family = "wasm"))]
fn new_session_pane_wasm() {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;

    #[test]
    fn test_handle_switch() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.tab_index = Some(1);
        state.sessions.insert(42, s);

        handle_switch(&mut state, Some(42), Some(1));
        assert_eq!(state.focused_pane_id, Some(42));
        assert_eq!(state.active_tab_index, Some(1));
        // render_dirty is false because handle_switch broadcasts immediately
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_rename() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.tab_index = Some(0);
        state.sessions.insert(42, s);

        handle_rename(&mut state, Some(42), Some("my-api".to_string()));
        assert_eq!(state.sessions[&42].display_name, "my-api");
        assert!(state.sessions[&42].manually_renamed);
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_rename_empty_ignored() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.display_name = "original".to_string();
        state.sessions.insert(42, s);

        handle_rename(&mut state, Some(42), Some("  ".to_string()));
        assert_eq!(state.sessions[&42].display_name, "original");
    }

    #[test]
    fn test_handle_rename_deduplicates() {
        let mut state = ControllerState::default();
        let mut s1 = Session::new(1, "s1".into());
        s1.display_name = "taken".to_string();
        state.sessions.insert(1, s1);
        let mut s2 = Session::new(2, "s2".into());
        s2.display_name = "other".to_string();
        state.sessions.insert(2, s2);

        handle_rename(&mut state, Some(2), Some("taken".to_string()));
        assert_eq!(state.sessions[&2].display_name, "taken-2");
    }

    #[test]
    fn test_handle_delete() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.tab_index = Some(0);
        state.sessions.insert(42, s);

        handle_delete(&mut state, Some(42));
        assert!(!state.sessions.contains_key(&42));
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_pause_toggle() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));

        handle_pause(&mut state, Some(42));
        assert!(state.sessions[&42].paused);

        state.render_dirty = false;
        handle_pause(&mut state, Some(42));
        assert!(!state.sessions[&42].paused);
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_attend_empty() {
        let mut state = ControllerState::default();
        // No sessions, attend should be a no-op
        handle_attend(&mut state);
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_attend_finds_waiting() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.activity = Activity::Waiting(WaitReason::Permission);
        s.tab_index = Some(0);
        state.sessions.insert(42, s);

        handle_attend(&mut state);
        assert_eq!(state.last_attended_pane_id, Some(42));
        // render_dirty is false because handle_attend broadcasts immediately
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_new_session() {
        let mut state = ControllerState::default();
        // Just verify it does not panic
        handle_new_session(&mut state);
    }
}
