// T029: Core rename logic for inline session renaming

use crate::session::deduplicate_name;
use crate::state::{PluginState, RenameState};
use zellij_tile::prelude::*;

/// Action returned by handle_key to drive the rename flow.
#[derive(Debug, PartialEq)]
pub enum RenameAction {
    /// Keep editing (re-render).
    Continue,
    /// User confirmed the rename with the given name.
    Confirm(String),
    /// User cancelled the rename.
    Cancel,
}

/// Start an inline rename for the session on the active tab.
/// Returns None if there is no session on the active tab.
pub fn start_rename(state: &PluginState) -> Option<RenameState> {
    let active_tab = state.active_tab_index?;
    let session = state
        .sessions
        .values()
        .find(|s| s.tab_index == Some(active_tab))?;
    let name = session.display_name.clone();
    let len = name.len();
    Some(RenameState {
        pane_id: session.pane_id,
        input_buffer: name,
        cursor_pos: len,
    })
}

/// Process a key event during an active rename operation.
pub fn handle_key(rename: &mut RenameState, key: KeyWithModifier) -> RenameAction {
    match key.bare_key {
        BareKey::Enter => RenameAction::Confirm(rename.input_buffer.clone()),
        BareKey::Esc => RenameAction::Cancel,
        BareKey::Char(c) => {
            rename.input_buffer.insert(rename.cursor_pos, c);
            rename.cursor_pos += 1;
            RenameAction::Continue
        }
        BareKey::Backspace => {
            if rename.cursor_pos > 0 {
                rename.cursor_pos -= 1;
                rename.input_buffer.remove(rename.cursor_pos);
            }
            RenameAction::Continue
        }
        BareKey::Delete => {
            if rename.cursor_pos < rename.input_buffer.len() {
                rename.input_buffer.remove(rename.cursor_pos);
            }
            RenameAction::Continue
        }
        BareKey::Left => {
            if rename.cursor_pos > 0 {
                rename.cursor_pos -= 1;
            }
            RenameAction::Continue
        }
        BareKey::Right => {
            if rename.cursor_pos < rename.input_buffer.len() {
                rename.cursor_pos += 1;
            }
            RenameAction::Continue
        }
        BareKey::Home => {
            rename.cursor_pos = 0;
            RenameAction::Continue
        }
        BareKey::End => {
            rename.cursor_pos = rename.input_buffer.len();
            RenameAction::Continue
        }
        _ => RenameAction::Continue,
    }
}

/// Complete a rename: update session display_name, set manually_renamed,
/// rename the Zellij tab, and flag updating_tabs to prevent re-entrancy.
pub fn complete_rename(state: &mut PluginState, pane_id: u32, new_name: String) {
    let new_name = new_name.trim().to_string();
    if new_name.is_empty() {
        return;
    }

    // Deduplicate against other sessions
    let names = state.session_names_except(pane_id);
    let final_name = deduplicate_name(&new_name, &names);

    let now = crate::session::unix_now();
    let tab_index = if let Some(session) = state.sessions.get_mut(&pane_id) {
        session.display_name = final_name.clone();
        session.manually_renamed = true;
        session.last_event_ts = now;
        session.meta_ts = now;
        session.tab_index
    } else {
        return;
    };

    // Only rename the Zellij tab if this is the sole session on the tab
    if let Some(idx) = tab_index {
        let sessions_on_tab = state.sessions.values()
            .filter(|s| s.tab_index == Some(idx))
            .count();
        if sessions_on_tab <= 1 {
            rename_tab(idx, &final_name);
            state.updating_tabs = true;
        }
    }

    crate::sync::sync_now(state);
    crate::sync::write_session_meta(&state.sessions);
}

#[cfg(target_family = "wasm")]
fn rename_tab(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn rename_tab(_tab_idx: usize, _name: &str) {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;
    use crate::state::PluginState;
    use std::collections::BTreeSet;

    fn bare(key: BareKey) -> KeyWithModifier {
        KeyWithModifier {
            bare_key: key,
            key_modifiers: BTreeSet::new(),
        }
    }

    fn make_state_with_session(pane_id: u32, name: &str, tab_index: usize) -> PluginState {
        let mut state = PluginState::default();
        let mut session = Session::new(pane_id, format!("session-{pane_id}"));
        session.display_name = name.to_string();
        session.tab_index = Some(tab_index);
        state.sessions.insert(pane_id, session);
        state.active_tab_index = Some(tab_index);
        state
    }

    #[test]
    fn test_start_rename_creates_state() {
        let state = make_state_with_session(42, "my-project", 0);
        let rename = start_rename(&state).unwrap();
        assert_eq!(rename.pane_id, 42);
        assert_eq!(rename.input_buffer, "my-project");
        assert_eq!(rename.cursor_pos, 10);
    }

    #[test]
    fn test_start_rename_no_active_tab() {
        let mut state = make_state_with_session(42, "test", 0);
        state.active_tab_index = None;
        assert!(start_rename(&state).is_none());
    }

    #[test]
    fn test_start_rename_no_session_on_tab() {
        let mut state = make_state_with_session(42, "test", 0);
        state.active_tab_index = Some(5); // no session on tab 5
        assert!(start_rename(&state).is_none());
    }

    #[test]
    fn test_handle_key_char_insert() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "ab".to_string(),
            cursor_pos: 1,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Char('X'))), RenameAction::Continue);
        assert_eq!(rename.input_buffer, "aXb");
        assert_eq!(rename.cursor_pos, 2);
    }

    #[test]
    fn test_handle_key_backspace() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 2,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Backspace)), RenameAction::Continue);
        assert_eq!(rename.input_buffer, "ac");
        assert_eq!(rename.cursor_pos, 1);
    }

    #[test]
    fn test_handle_key_backspace_at_start() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 0,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Backspace)), RenameAction::Continue);
        assert_eq!(rename.input_buffer, "abc");
        assert_eq!(rename.cursor_pos, 0);
    }

    #[test]
    fn test_handle_key_enter_confirms() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "new-name".to_string(),
            cursor_pos: 8,
        };
        assert_eq!(
            handle_key(&mut rename, bare(BareKey::Enter)),
            RenameAction::Confirm("new-name".to_string())
        );
    }

    #[test]
    fn test_handle_key_esc_cancels() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "test".to_string(),
            cursor_pos: 4,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Esc)), RenameAction::Cancel);
    }

    #[test]
    fn test_handle_key_left_right() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 2,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Left)), RenameAction::Continue);
        assert_eq!(rename.cursor_pos, 1);
        assert_eq!(handle_key(&mut rename, bare(BareKey::Right)), RenameAction::Continue);
        assert_eq!(rename.cursor_pos, 2);
    }

    #[test]
    fn test_handle_key_left_at_start() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 0,
        };
        handle_key(&mut rename, bare(BareKey::Left));
        assert_eq!(rename.cursor_pos, 0);
    }

    #[test]
    fn test_handle_key_right_at_end() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 3,
        };
        handle_key(&mut rename, bare(BareKey::Right));
        assert_eq!(rename.cursor_pos, 3);
    }

    #[test]
    fn test_complete_rename() {
        let mut state = make_state_with_session(42, "old-name", 0);
        complete_rename(&mut state, 42, "new-name".to_string());
        assert_eq!(state.sessions[&42].display_name, "new-name");
        assert!(state.sessions[&42].manually_renamed);
        assert!(state.updating_tabs);
    }

    #[test]
    fn test_complete_rename_empty_name_ignored() {
        let mut state = make_state_with_session(42, "old-name", 0);
        complete_rename(&mut state, 42, "  ".to_string());
        assert_eq!(state.sessions[&42].display_name, "old-name");
        assert!(!state.sessions[&42].manually_renamed);
    }

    #[test]
    fn test_complete_rename_deduplicates() {
        let mut state = make_state_with_session(42, "old", 0);
        let mut s2 = Session::new(99, "s2".into());
        s2.display_name = "taken".to_string();
        s2.tab_index = Some(1);
        state.sessions.insert(99, s2);

        complete_rename(&mut state, 42, "taken".to_string());
        assert_eq!(state.sessions[&42].display_name, "taken-2");
    }

    #[test]
    fn test_handle_key_delete() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "abc".to_string(),
            cursor_pos: 1,
        };
        assert_eq!(handle_key(&mut rename, bare(BareKey::Delete)), RenameAction::Continue);
        assert_eq!(rename.input_buffer, "ac");
        assert_eq!(rename.cursor_pos, 1);
    }

    #[test]
    fn test_handle_key_home_end() {
        let mut rename = RenameState {
            pane_id: 1,
            input_buffer: "hello".to_string(),
            cursor_pos: 3,
        };
        handle_key(&mut rename, bare(BareKey::Home));
        assert_eq!(rename.cursor_pos, 0);
        handle_key(&mut rename, bare(BareKey::End));
        assert_eq!(rename.cursor_pos, 5);
    }
}
