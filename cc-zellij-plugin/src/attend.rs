// Smart attend: tiered priority session cycling

use crate::session::{Activity, WaitReason};
use crate::state::PluginState;

/// Result of an attend action.
pub enum AttendResult {
    /// Switched to a session.
    Switched {
        tab_index: usize,
        pane_id: u32,
        display_name: String,
    },
    /// No sessions need attention.
    NoneWaiting,
    /// All sessions are actively working.
    AllBusy,
}

/// Find the next session needing attention using tiered priority:
/// 1. Permission waiting (oldest first) - critical, blocks progress
/// 2. Notification waiting (oldest first) - soft, informational
/// 3. Idle/Done/AgentDone (newest first) - older idle may be intentionally parked
/// 4. Skip Working/ToolUse/Init - actively running
///
/// Skips the currently focused session unless it's the only one.
pub fn perform_attend(state: &PluginState) -> AttendResult {
    let sessions = state.sessions_by_tab_order();
    if sessions.is_empty() {
        return AttendResult::NoneWaiting;
    }

    let focused = state.focused_pane_id;
    let only_one = sessions.len() == 1;

    // Tier 1: Permission waiting (oldest first)
    let mut permission_waiting: Vec<_> = sessions.iter()
        .filter(|s| matches!(s.activity, Activity::Waiting(WaitReason::Permission)))
        .filter(|s| only_one || Some(s.pane_id) != focused)
        .collect();
    permission_waiting.sort_by_key(|s| s.last_event_ts);
    if let Some(session) = permission_waiting.first() {
        return switch_to(session);
    }

    // Tier 2: Notification waiting (oldest first)
    let mut notification_waiting: Vec<_> = sessions.iter()
        .filter(|s| matches!(s.activity, Activity::Waiting(WaitReason::Notification)))
        .filter(|s| only_one || Some(s.pane_id) != focused)
        .collect();
    notification_waiting.sort_by_key(|s| s.last_event_ts);
    if let Some(session) = notification_waiting.first() {
        return switch_to(session);
    }

    // Tier 3: Idle/Done/AgentDone (newest first)
    let mut idle: Vec<_> = sessions.iter()
        .filter(|s| matches!(s.activity, Activity::Idle | Activity::Done | Activity::AgentDone))
        .filter(|s| only_one || Some(s.pane_id) != focused)
        .collect();
    idle.sort_by_key(|s| std::cmp::Reverse(s.last_event_ts));
    if let Some(session) = idle.first() {
        return switch_to(session);
    }

    // All sessions are working/init
    let all_working = sessions.iter().all(|s| {
        matches!(s.activity, Activity::Working | Activity::ToolUse(_) | Activity::Init)
    });
    if all_working {
        AttendResult::AllBusy
    } else {
        AttendResult::NoneWaiting
    }
}

fn switch_to(session: &crate::session::Session) -> AttendResult {
    let tab_index = session.tab_index.unwrap_or(0);
    let pane_id = session.pane_id;
    let display_name = session.display_name.clone();

    switch_and_focus(tab_index, pane_id);

    AttendResult::Switched {
        tab_index,
        pane_id,
        display_name,
    }
}

#[cfg(target_family = "wasm")]
fn switch_and_focus(tab_idx: usize, pane_id: u32) {
    zellij_tile::prelude::switch_tab_to(tab_idx as u32 + 1);
    zellij_tile::prelude::focus_terminal_pane(pane_id, false);
}

#[cfg(not(target_family = "wasm"))]
fn switch_and_focus(_tab_idx: usize, _pane_id: u32) {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;

    fn make_state(sessions: Vec<Session>) -> PluginState {
        let mut state = PluginState::default();
        for s in sessions {
            state.sessions.insert(s.pane_id, s);
        }
        state
    }

    #[test]
    fn test_attend_empty() {
        let state = PluginState::default();
        assert!(matches!(perform_attend(&state), AttendResult::NoneWaiting));
    }

    #[test]
    fn test_attend_all_working() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Working;
        let state = make_state(vec![s1]);
        assert!(matches!(perform_attend(&state), AttendResult::AllBusy));
    }

    #[test]
    fn test_attend_permission_over_notification() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Notification);
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "notif".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting(WaitReason::Permission);
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "perm".into();

        let state = make_state(vec![s1, s2]);
        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "perm"),
            _ => panic!("expected Switched to permission session"),
        }
    }

    #[test]
    fn test_attend_oldest_permission_first() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "older".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting(WaitReason::Permission);
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "newer".into();

        let state = make_state(vec![s1, s2]);
        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "older"),
            _ => panic!("expected Switched"),
        }
    }

    #[test]
    fn test_attend_idle_newest_first() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "old-idle".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Done;
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "new-done".into();

        let state = make_state(vec![s1, s2]);
        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "new-done"),
            _ => panic!("expected Switched to newest idle"),
        }
    }

    #[test]
    fn test_attend_skips_focused() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.display_name = "focused".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting(WaitReason::Permission);
        s2.tab_index = Some(1);
        s2.display_name = "other".into();

        let mut state = make_state(vec![s1, s2]);
        state.focused_pane_id = Some(1); // s1 is focused

        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "other"),
            _ => panic!("expected Switched to non-focused"),
        }
    }

    #[test]
    fn test_attend_single_session_not_skipped() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.display_name = "only".into();

        let mut state = make_state(vec![s1]);
        state.focused_pane_id = Some(1); // focused on the only session

        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "only"),
            _ => panic!("expected Switched even though focused"),
        }
    }

    #[test]
    fn test_attend_notification_over_idle() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(0);
        s1.display_name = "idle".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting(WaitReason::Notification);
        s2.tab_index = Some(1);
        s2.display_name = "notif".into();

        let state = make_state(vec![s1, s2]);
        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "notif"),
            _ => panic!("expected Switched to notification"),
        }
    }
}
