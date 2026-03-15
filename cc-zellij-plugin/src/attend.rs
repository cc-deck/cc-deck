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

/// Find the next session needing attention using exclusive tiers:
///
/// **Exclusive tier selection**: Only the highest non-empty tier is used.
/// When ⚠ sessions exist, Alt+a cycles ONLY among ⚠ sessions.
/// When no ⚠ but ✓ exists, Alt+a cycles among ✓ sessions first.
/// When only idle sessions exist, Alt+a cycles among those.
///
/// Tier priorities:
/// 1. Waiting (Permission first, then Notification) - sorted oldest first
/// 2. Done/AgentDone (most recent first, so newly finished jump to front)
/// 3. Idle/Init (tab order)
/// 4. Skip: Working, Paused
///
/// Round-robin within the selected tier only.
pub fn perform_attend(state: &mut PluginState) -> AttendResult {
    let sessions = state.sessions_by_tab_order();
    if sessions.is_empty() {
        return AttendResult::NoneWaiting;
    }

    // Tier 1: All waiting sessions (Permission + Notification, oldest first)
    let mut waiting: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Waiting(_)))
        .copied().collect();
    // Sort: Permission before Notification, then oldest first within each
    waiting.sort_by(|a, b| {
        let a_perm = matches!(a.activity, Activity::Waiting(WaitReason::Permission));
        let b_perm = matches!(b.activity, Activity::Waiting(WaitReason::Permission));
        b_perm.cmp(&a_perm).then(a.last_event_ts.cmp(&b.last_event_ts))
    });

    // Tier 2: Done/AgentDone (most recently finished first)
    let mut done: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Done | Activity::AgentDone))
        .copied().collect();
    done.sort_by(|a, b| b.last_event_ts.cmp(&a.last_event_ts));

    // Tier 3: Idle/Init (tab order)
    let mut idle: Vec<_> = sessions.iter()
        .filter(|s| !s.paused && matches!(s.activity, Activity::Idle | Activity::Init))
        .copied().collect();
    idle.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));

    // Pick the highest non-empty tier exclusively
    let candidates = if !waiting.is_empty() {
        waiting
    } else if !done.is_empty() {
        done
    } else if !idle.is_empty() {
        idle
    } else {
        return AttendResult::AllBusy;
    };

    // Round-robin within the selected tier.
    // Read last attended from shared file (survives instance switches).
    let last_attended = read_last_attended().or(state.last_attended_pane_id);
    let start_idx = if let Some(last_id) = last_attended {
        candidates.iter()
            .position(|s| s.pane_id == last_id)
            .map(|pos| (pos + 1) % candidates.len())
            .unwrap_or(0)
    } else {
        0
    };

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
    fn test_attend_waiting_excludes_done_and_idle() {
        // When ⚠ sessions exist, only cycle among ⚠ sessions
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting(WaitReason::Permission);
        s1.tab_index = Some(0);
        s1.display_name = "waiting".into();
        s1.last_event_ts = 100;

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Done;
        s2.tab_index = Some(1);
        s2.display_name = "done".into();

        let mut s3 = Session::new(3, "c".into());
        s3.activity = Activity::Idle;
        s3.tab_index = Some(2);
        s3.display_name = "idle".into();

        let mut state = make_state(vec![s1, s2, s3]);

        // First press: goes to waiting
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "waiting"),
            _ => panic!("expected waiting"),
        }

        // Second press: still waiting (only candidate in tier)
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "waiting"),
            _ => panic!("expected waiting again"),
        }
    }

    #[test]
    fn test_attend_done_most_recent_first() {
        // Done sessions sorted most recently finished first
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Done;
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "old-done".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Done;
        s2.tab_index = Some(1);
        s2.last_event_ts = 300;
        s2.display_name = "new-done".into();

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "new-done"),
            _ => panic!("expected newest done first"),
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

        let mut state = make_state(vec![s1, s2]);
        match perform_attend(&mut state) {
            AttendResult::Switched { display_name, .. } => assert_eq!(display_name, "notif"),
            _ => panic!("expected Switched to notification"),
        }
    }
}
