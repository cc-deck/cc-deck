use super::modes::{NavigateContext, SidebarMode};
use super::state::SidebarState;
use cc_deck::{RenderPayload, RenderSession};
use std::collections::BTreeSet;
use zellij_tile::prelude::*;

pub fn make_payload(sessions: Vec<RenderSession>) -> RenderPayload {
    let total = sessions.len();
    RenderPayload {
        sessions,
        focused_pane_id: None,
        active_tab_index: 0,
        notification: None,
        notification_expiry: None,
        total,
        waiting: 0,
        working: 0,
        idle: 0,
        controller_plugin_id: 1,
        voice_connected: false,
        voice_muted: false,
    }
}

pub fn make_session(pane_id: u32, name: &str, tab_index: usize) -> RenderSession {
    RenderSession {
        pane_id,
        display_name: name.to_string(),
        activity_label: "Idle".to_string(),
        indicator: "\u{25cb}".to_string(),
        color: (180, 175, 195),
        git_branch: None,
        tab_index,
        paused: false,
        done_attended: false,
    }
}

pub fn bare(key: BareKey) -> KeyWithModifier {
    KeyWithModifier {
        bare_key: key,
        key_modifiers: BTreeSet::new(),
    }
}

pub fn make_state_with_sessions(sessions: &[(u32, &str, usize)]) -> SidebarState {
    let mut state = SidebarState::default();
    state.cached_payload = Some(make_payload(
        sessions
            .iter()
            .map(|(id, name, tab)| make_session(*id, name, *tab))
            .collect(),
    ));
    state
}

pub fn make_nav_state(sessions: &[(u32, &str, usize)], cursor: usize) -> SidebarState {
    let mut state = make_state_with_sessions(sessions);
    state.mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: cursor,
        restore_pane_id: None,
        restore_tab_index: None,
        entered_at_ms: 0,
    });
    state
}
