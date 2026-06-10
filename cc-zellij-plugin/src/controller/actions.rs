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
        ActionType::VoiceMute => handle_voice_mute(state),
        ActionType::Sort => handle_sort(state),
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
            crate::wasm_compat::rename_tab_wasm(idx, &final_name);
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

/// Toggle voice mute state (from sidebar action).
fn handle_voice_mute(state: &mut ControllerState) {
    if state.voice_enabled {
        state.voice_mute_requested = Some(!state.voice_muted);
        state.voice_mute_requested_ms = crate::session::unix_now_ms();
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
    }
}

/// Create a new session tab.
fn handle_new_session(state: &mut ControllerState) {
    match state.config.new_session_mode {
        crate::config::NewSessionMode::Tab => new_tab_wasm(),
        crate::config::NewSessionMode::Pane => new_session_pane_wasm(),
    }
}

// --- Sort by activity ---

/// Classify a session into a sort tier.
/// Tier 0 (Active): Working or Waiting, not paused.
/// Tier 1 (Inactive): Idle, Done, AgentDone, Init, not paused.
/// Tier 2 (Paused): paused == true, regardless of activity.
fn sort_tier(session: &Session) -> u8 {
    if session.paused {
        return 2;
    }
    match session.activity {
        Activity::Working | Activity::Waiting(_) => 0,
        Activity::Idle | Activity::Done | Activity::AgentDone | Activity::Init => 1,
    }
}

/// Sort sessions by activity tiers and physically reorder Zellij tabs.
///
/// The sort is stable within each tier (preserves relative tab order).
/// After computing the target order, executes a swap sequence using
/// switch_tab_to + Action::MoveTab to bubble each tab into place.
/// Restores focus to the original active tab after completion.
///
fn handle_sort(state: &mut ControllerState) {
    let mut sessions: Vec<(u32, usize, u8)> = state
        .sessions
        .values()
        .filter_map(|s| {
            s.tab_index.map(|idx| (s.pane_id, idx, sort_tier(s)))
        })
        .collect();

    if sessions.len() <= 1 {
        return;
    }

    // Sort by current tab index first (to establish initial relative order)
    sessions.sort_by_key(|&(_, idx, _)| idx);

    // Stable sort by tier (preserves relative tab order within each tier)
    sessions.sort_by_key(|&(_, _, tier)| tier);

    // Build target mapping: target_position -> current_tab_index
    let target_order: Vec<(u32, usize)> = sessions.iter().map(|&(pid, idx, _)| (pid, idx)).collect();

    // Check if already sorted (no-op optimization): the sessions should end up
    // in the same order as their
    // current tab indices when the sort produces no changes
    let current_indices: Vec<usize> = target_order.iter().map(|&(_, idx)| idx).collect();
    let mut sorted_indices = current_indices.clone();
    sorted_indices.sort();
    let is_noop = current_indices == sorted_indices
        && target_order
            .windows(2)
            .all(|w| sort_tier_by_pane(state, w[0].0) <= sort_tier_by_pane(state, w[1].0));

    if is_noop {
        return;
    }

    // Save the focused pane_id so we can restore focus to its new position
    // after tabs have moved.
    let focused_pane = state.focused_pane_id;

    // Execute the swap sequence using bubble approach.
    // We need to move tabs so that the tab at current_indices[i] ends up
    // at the position occupied by sorted_indices[i].
    //
    // Build a working copy of current positions that we update as we swap.
    let mut current_positions: Vec<usize> = target_order.iter().map(|&(_, idx)| idx).collect();

    for target_pos in 0..current_positions.len() {
        // Find which session should be at target_pos
        // target_order[target_pos] tells us which pane_id should be here
        // current_positions[target_pos] tells us where that pane currently is

        let target_tab_idx = if target_pos == 0 {
            // The first session should be at the smallest tab index
            *current_positions.iter().min().unwrap_or(&0)
        } else {
            // Each subsequent session should be one position after the previous
            current_positions[target_pos - 1] + 1
        };

        let current_tab_idx = current_positions[target_pos];

        if current_tab_idx == target_tab_idx {
            continue;
        }

        // Focus the tab we want to move
        switch_tab_to_wasm(current_tab_idx);

        // Move it left or right to the target position
        if current_tab_idx > target_tab_idx {
            for _ in 0..(current_tab_idx - target_tab_idx) {
                move_tab_wasm(zellij_tile::prelude::Direction::Left);
            }
        } else {
            for _ in 0..(target_tab_idx - current_tab_idx) {
                move_tab_wasm(zellij_tile::prelude::Direction::Right);
            }
        }

        // Update positions: the moved tab is now at target_tab_idx.
        // All tabs between the old and new positions shifted by one.
        let old_pos = current_tab_idx;
        let new_pos = target_tab_idx;
        for pos in current_positions.iter_mut() {
            if *pos == old_pos {
                *pos = new_pos;
            } else if old_pos > new_pos && *pos >= new_pos && *pos < old_pos {
                *pos += 1;
            } else if old_pos < new_pos && *pos > old_pos && *pos <= new_pos {
                *pos -= 1;
            }
        }
    }

    // Update session tab indices in state to reflect new positions.
    // The controller will get fresh TabUpdate/PaneUpdate events from Zellij,
    // but we update proactively for immediate render accuracy.
    let base_idx = sessions.iter().map(|&(_, idx, _)| idx).min().unwrap_or(0);
    for (i, &(pane_id, _, _)) in sessions.iter().enumerate() {
        if let Some(session) = state.sessions.get_mut(&pane_id) {
            session.tab_index = Some(base_idx + i);
        }
    }

    // Restore focus to the originally focused pane's NEW tab index.
    // This prevents the sidebar from seeing a tab change and exiting
    // navigate mode (T010c).
    if let Some(fpid) = focused_pane {
        if let Some(new_tab) = state.sessions.get(&fpid).and_then(|s| s.tab_index) {
            state.active_tab_index = Some(new_tab);
            switch_tab_to_wasm(new_tab);
        }
    }

    state.mark_render_dirty();
}

/// Helper to look up the sort tier for a pane_id from state.
fn sort_tier_by_pane(state: &ControllerState, pane_id: u32) -> u8 {
    state
        .sessions
        .get(&pane_id)
        .map(sort_tier)
        .unwrap_or(1)
}

/// WASM wrapper for MoveTab action (physically reorders a tab).
#[cfg(target_family = "wasm")]
fn move_tab_wasm(direction: zellij_tile::prelude::Direction) {
    zellij_tile::prelude::run_action(
        zellij_utils::input::actions::Action::MoveTab { direction },
        std::collections::BTreeMap::new(),
    );
}

#[cfg(not(target_family = "wasm"))]
fn move_tab_wasm(_direction: zellij_tile::prelude::Direction) {}

// --- Tiered attend logic ---

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

    // --- sort_tier tests ---

    #[test]
    fn test_sort_tier_working_is_active() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Working;
        assert_eq!(sort_tier(&s), 0);
    }

    #[test]
    fn test_sort_tier_waiting_permission_is_active() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Waiting(WaitReason::Permission);
        assert_eq!(sort_tier(&s), 0);
    }

    #[test]
    fn test_sort_tier_waiting_notification_is_active() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Waiting(WaitReason::Notification);
        assert_eq!(sort_tier(&s), 0);
    }

    #[test]
    fn test_sort_tier_idle_is_inactive() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Idle;
        assert_eq!(sort_tier(&s), 1);
    }

    #[test]
    fn test_sort_tier_done_is_inactive() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Done;
        assert_eq!(sort_tier(&s), 1);
    }

    #[test]
    fn test_sort_tier_agent_done_is_inactive() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::AgentDone;
        assert_eq!(sort_tier(&s), 1);
    }

    #[test]
    fn test_sort_tier_init_is_inactive() {
        let s = Session::new(1, "test".into());
        // Default activity is Init
        assert_eq!(sort_tier(&s), 1);
    }

    #[test]
    fn test_sort_tier_paused_working_is_paused() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Working;
        s.paused = true;
        assert_eq!(sort_tier(&s), 2);
    }

    #[test]
    fn test_sort_tier_paused_idle_is_paused() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Idle;
        s.paused = true;
        assert_eq!(sort_tier(&s), 2);
    }

    #[test]
    fn test_sort_tier_paused_waiting_is_paused() {
        let mut s = Session::new(1, "test".into());
        s.activity = Activity::Waiting(WaitReason::Permission);
        s.paused = true;
        assert_eq!(sort_tier(&s), 2);
    }

    // --- handle_sort tests ---

    #[test]
    fn test_handle_sort_reorders_by_tier() {
        let mut state = ControllerState::default();
        // Tab order: [Idle, Working, Paused, Done, Working]
        let mut s0 = Session::new(10, "idle".into());
        s0.activity = Activity::Idle;
        s0.tab_index = Some(0);
        let mut s1 = Session::new(20, "work1".into());
        s1.activity = Activity::Working;
        s1.tab_index = Some(1);
        let mut s2 = Session::new(30, "paused".into());
        s2.activity = Activity::Idle;
        s2.paused = true;
        s2.tab_index = Some(2);
        let mut s3 = Session::new(40, "done".into());
        s3.activity = Activity::Done;
        s3.tab_index = Some(3);
        let mut s4 = Session::new(50, "work2".into());
        s4.activity = Activity::Working;
        s4.tab_index = Some(4);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);
        state.sessions.insert(50, s4);

        handle_sort(&mut state);

        // Expected order: Working(20), Working(50), Idle(10), Done(40), Paused(30)
        let ordered: Vec<(u32, usize)> = {
            let mut v: Vec<_> = state.sessions.values()
                .map(|s| (s.pane_id, s.tab_index.unwrap_or(usize::MAX)))
                .collect();
            v.sort_by_key(|&(_, idx)| idx);
            v
        };
        assert_eq!(ordered[0].0, 20, "First should be work1");
        assert_eq!(ordered[1].0, 50, "Second should be work2");
        assert_eq!(ordered[2].0, 10, "Third should be idle");
        assert_eq!(ordered[3].0, 40, "Fourth should be done");
        assert_eq!(ordered[4].0, 30, "Fifth should be paused");
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_sort_already_sorted_is_noop() {
        let mut state = ControllerState::default();
        // Already in correct order: Working, Idle, Paused
        let mut s0 = Session::new(10, "work".into());
        s0.activity = Activity::Working;
        s0.tab_index = Some(0);
        let mut s1 = Session::new(20, "idle".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(1);
        let mut s2 = Session::new(30, "paused".into());
        s2.activity = Activity::Idle;
        s2.paused = true;
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);

        handle_sort(&mut state);

        // render_dirty should NOT be set since it was a no-op
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_sort_single_session_is_noop() {
        let mut state = ControllerState::default();
        let mut s = Session::new(10, "only".into());
        s.tab_index = Some(0);
        state.sessions.insert(10, s);

        handle_sort(&mut state);
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_sort_empty_sessions_is_noop() {
        let mut state = ControllerState::default();
        handle_sort(&mut state);
        assert!(!state.render_dirty);
    }

    #[test]
    fn test_handle_sort_preserves_relative_order_within_tier() {
        let mut state = ControllerState::default();
        // Three Working sessions at tab positions 1, 4, 6
        let mut s0 = Session::new(10, "idle1".into());
        s0.activity = Activity::Idle;
        s0.tab_index = Some(0);
        let mut s1 = Session::new(20, "work-a".into());
        s1.activity = Activity::Working;
        s1.tab_index = Some(1);
        let mut s2 = Session::new(30, "idle2".into());
        s2.activity = Activity::Idle;
        s2.tab_index = Some(2);
        let mut s3 = Session::new(40, "idle3".into());
        s3.activity = Activity::Idle;
        s3.tab_index = Some(3);
        let mut s4 = Session::new(50, "work-b".into());
        s4.activity = Activity::Working;
        s4.tab_index = Some(4);
        let mut s5 = Session::new(60, "idle4".into());
        s5.activity = Activity::Idle;
        s5.tab_index = Some(5);
        let mut s6 = Session::new(70, "work-c".into());
        s6.activity = Activity::Working;
        s6.tab_index = Some(6);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);
        state.sessions.insert(40, s3);
        state.sessions.insert(50, s4);
        state.sessions.insert(60, s5);
        state.sessions.insert(70, s6);

        handle_sort(&mut state);

        let ordered: Vec<u32> = {
            let mut v: Vec<_> = state.sessions.values()
                .map(|s| (s.pane_id, s.tab_index.unwrap_or(usize::MAX)))
                .collect();
            v.sort_by_key(|&(_, idx)| idx);
            v.into_iter().map(|(pid, _)| pid).collect()
        };

        // Working sessions should appear first, in their original relative order
        assert_eq!(ordered[0], 20, "work-a should be first");
        assert_eq!(ordered[1], 50, "work-b should be second");
        assert_eq!(ordered[2], 70, "work-c should be third");
        // Idle sessions follow, in their original relative order
        assert_eq!(ordered[3], 10, "idle1 should be fourth");
        assert_eq!(ordered[4], 30, "idle2 should be fifth");
        assert_eq!(ordered[5], 40, "idle3 should be sixth");
        assert_eq!(ordered[6], 60, "idle4 should be seventh");
    }

    #[test]
    fn test_handle_sort_all_same_tier_is_noop() {
        let mut state = ControllerState::default();
        // All Working sessions
        let mut s0 = Session::new(10, "w1".into());
        s0.activity = Activity::Working;
        s0.tab_index = Some(0);
        let mut s1 = Session::new(20, "w2".into());
        s1.activity = Activity::Working;
        s1.tab_index = Some(1);
        let mut s2 = Session::new(30, "w3".into());
        s2.activity = Activity::Working;
        s2.tab_index = Some(2);

        state.sessions.insert(10, s0);
        state.sessions.insert(20, s1);
        state.sessions.insert(30, s2);

        handle_sort(&mut state);
        assert!(!state.render_dirty);
    }
}
