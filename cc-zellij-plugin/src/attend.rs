// Smart attend: tiered priority session cycling

use crate::session::{Activity, WaitReason};
use crate::state::PluginState;

const ATTEND_STATE_PATH: &str = "/cache/attend-state.json";

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
/// 3. Done/AgentDone (tab order) - just finished, need review
/// 4. Idle/Init (tab order) - already seen, nothing new
/// 5. Skip Working - actively running
///
/// Round-robin: subsequent presses cycle through the priority-ordered list,
/// starting after the last attended session.
pub fn perform_attend(state: &mut PluginState) -> AttendResult {
    let sessions = state.sessions_by_tab_order();
    if sessions.is_empty() {
        return AttendResult::NoneWaiting;
    }

    // Build priority-ordered candidate list
    let mut candidates: Vec<&crate::session::Session> = Vec::new();

    // Tier 1: Permission waiting (oldest first), skip paused
    let mut t1: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Waiting(WaitReason::Permission)))
        .copied().collect();
    t1.sort_by_key(|s| s.last_event_ts);
    candidates.extend(t1);

    // Tier 2: Notification waiting (oldest first), skip paused
    let mut t2: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Waiting(WaitReason::Notification)))
        .copied().collect();
    t2.sort_by_key(|s| s.last_event_ts);
    candidates.extend(t2);

    // Tier 3: Done/AgentDone (tab order) - just finished, likely need review
    let mut t3: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Done | Activity::AgentDone))
        .copied().collect();
    t3.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
    candidates.extend(t3);

    // Tier 4: Idle/Init (tab order) - already seen, nothing new
    let mut t4: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Idle | Activity::Init))
        .copied().collect();
    t4.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
    candidates.extend(t4);

    if candidates.is_empty() {
        return AttendResult::AllBusy;
    }

    // Round-robin: read last attended from shared file (survives instance switches).
    // When the active-tab instance changes (after attend switches tabs and keybinds
    // re-register), the new instance picks up where the previous one left off.
    let last_attended = read_last_attended().or(state.last_attended_pane_id);
    let start_idx = if let Some(last_id) = last_attended {
        candidates.iter()
            .position(|s| s.pane_id == last_id)
            .map(|pos| (pos + 1) % candidates.len())
            .unwrap_or(0)
    } else {
        0
    };

    // Extract data before dropping the borrow on state
    let pane_id = candidates[start_idx].pane_id;
    let tab_index = candidates[start_idx].tab_index.unwrap_or(0);
    let display_name = candidates[start_idx].display_name.clone();

    state.last_attended_pane_id = Some(pane_id);
    write_last_attended(pane_id);
    switch_and_focus(tab_index, pane_id);

    AttendResult::Switched {
        tab_index,
        pane_id,
        display_name,
    }
}

/// Read last attended pane_id from shared WASI cache.
fn read_last_attended() -> Option<u32> {
    std::fs::read_to_string(ATTEND_STATE_PATH)
        .ok()
        .and_then(|s| s.trim().parse::<u32>().ok())
}

/// Write last attended pane_id to shared WASI cache.
fn write_last_attended(pane_id: u32) {
    let _ = std::fs::write(ATTEND_STATE_PATH, pane_id.to_string());
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
        let mut state = PluginState::default();
        assert!(matches!(perform_attend(&mut state), AttendResult::NoneWaiting));
    }

    #[test]
    fn test_attend_all_working() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Working;
        let mut state = make_state(vec![s1]);
        assert!(matches!(perform_attend(&mut state), AttendResult::AllBusy));
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

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
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

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "older"),
            _ => panic!("expected Switched"),
        }
    }

    #[test]
    fn test_attend_done_before_idle() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "idle-first-tab".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Done;
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "done-second-tab".into();

        let mut state = make_state(vec![s1, s2]);
        // Done sessions should be attended before Idle
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "done-second-tab"),
            _ => panic!("expected Switched to Done session first"),
        }
    }

    #[test]
    fn test_attend_idle_tab_order() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(1);
        s1.last_event_ts = 100;
        s1.display_name = "second-tab".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Init;
        s2.tab_index = Some(0);
        s2.last_event_ts = 200;
        s2.display_name = "first-tab".into();

        let mut state = make_state(vec![s1, s2]);
        // Init and Idle are same tier, sorted by tab order
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "first-tab"),
            _ => panic!("expected Switched to first tab"),
        }
    }

    #[test]
    fn test_attend_round_robin() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "first".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting(WaitReason::Permission);
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "second".into();

        let mut state = make_state(vec![s1, s2]);

        // First attend: picks "first" (oldest permission)
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "first"),
            _ => panic!("expected first"),
        }

        // Second attend: round-robin to "second"
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "second"),
            _ => panic!("expected second"),
        }

        // Third attend: wraps back to "first"
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "first"),
            _ => panic!("expected first again"),
        }
    }

    #[test]
    fn test_attend_includes_init() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Init;
        s1.tab_index = Some(0);
        s1.display_name = "init-session".into();

        let mut state = make_state(vec![s1]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "init-session"),
            _ => panic!("expected Init session to be attended"),
        }
    }

    #[test]
    fn test_attend_skips_paused() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.display_name = "paused-one".into();
        s1.paused = true;

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Idle;
        s2.tab_index = Some(1);
        s2.display_name = "active-idle".into();

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "active-idle"),
            _ => panic!("expected Switched to non-paused session"),
        }
    }

    #[test]
    fn test_attend_all_paused() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Idle;
        s1.tab_index = Some(0);
        s1.paused = true;

        let mut state = make_state(vec![s1]);
        assert!(matches!(perform_attend(&mut state), AttendResult::AllBusy));
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

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "notif"),
            _ => panic!("expected Switched to notification"),
        }
    }
}
