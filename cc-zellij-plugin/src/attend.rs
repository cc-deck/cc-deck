// T025: Attend action - jump to the oldest waiting session

use crate::state::PluginState;

/// Result of an attend action.
pub enum AttendResult {
    /// Switched to a waiting session.
    Switched {
        tab_index: usize,
        pane_id: u32,
        display_name: String,
    },
    /// No sessions are waiting for input.
    NoneWaiting,
}

/// Find the oldest waiting session and switch to it.
pub fn perform_attend(state: &PluginState) -> AttendResult {
    match state.oldest_waiting_session() {
        Some(session) => {
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
        None => AttendResult::NoneWaiting,
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
    use crate::session::{Activity, Session};

    fn make_state_with_sessions(sessions: Vec<Session>) -> PluginState {
        let mut state = PluginState::default();
        for s in sessions {
            state.sessions.insert(s.pane_id, s);
        }
        state
    }

    #[test]
    fn test_attend_none_waiting() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Working;
        let state = make_state_with_sessions(vec![s1]);

        match perform_attend(&state) {
            AttendResult::NoneWaiting => {}
            _ => panic!("expected NoneWaiting"),
        }
    }

    #[test]
    fn test_attend_empty_sessions() {
        let state = PluginState::default();
        match perform_attend(&state) {
            AttendResult::NoneWaiting => {}
            _ => panic!("expected NoneWaiting"),
        }
    }

    #[test]
    fn test_attend_finds_waiting() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Working;
        s1.tab_index = Some(0);

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting;
        s2.tab_index = Some(1);
        s2.display_name = "my-session".into();

        let state = make_state_with_sessions(vec![s1, s2]);

        match perform_attend(&state) {
            AttendResult::Switched {
                tab_index,
                pane_id,
                display_name,
            } => {
                assert_eq!(tab_index, 1);
                assert_eq!(pane_id, 2);
                assert_eq!(display_name, "my-session");
            }
            _ => panic!("expected Switched"),
        }
    }

    #[test]
    fn test_attend_oldest_waiting() {
        let mut s1 = Session::new(1, "a".into());
        s1.activity = Activity::Waiting;
        s1.tab_index = Some(0);
        s1.last_event_ts = 100;
        s1.display_name = "older".into();

        let mut s2 = Session::new(2, "b".into());
        s2.activity = Activity::Waiting;
        s2.tab_index = Some(1);
        s2.last_event_ts = 200;
        s2.display_name = "newer".into();

        let state = make_state_with_sessions(vec![s1, s2]);

        match perform_attend(&state) {
            AttendResult::Switched { display_name, .. } => {
                assert_eq!(display_name, "older");
            }
            _ => panic!("expected Switched"),
        }
    }
}
