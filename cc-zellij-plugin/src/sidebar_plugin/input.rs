// Sidebar mouse and key event handlers.
//
// The sidebar handles all user interaction locally. Mouse clicks on session
// entries trigger Switch actions. Keyboard input drives the mode state machine
// (navigate, filter, rename, delete confirm, help). Confirmed actions are
// forwarded to the controller via cc-deck:action pipe messages.

use super::modes::{FilterState, NavigateContext, RenameState, SidebarMode};
use super::rename::{self, RenameAction};
use super::state::SidebarState;
use cc_deck::{ActionMessage, ActionType};
use crate::session::unix_now_ms;
use zellij_tile::prelude::*;

const DOUBLE_CLICK_MS: u64 = 300;

/// Handle a mouse event. Returns true if the sidebar should re-render.
pub fn handle_mouse(state: &mut SidebarState, mouse: Mouse) -> bool {
    match mouse {
        Mouse::LeftClick(row, _col) => handle_left_click(state, row as usize),
        Mouse::RightClick(row, _col) => handle_right_click(state, row as usize),
        _ => false,
    }
}

/// Handle a key event. Returns true if the sidebar should re-render.
pub fn handle_key(state: &mut SidebarState, key: KeyWithModifier) -> bool {
    crate::debug_log(&format!(
        "SIDEBAR KEY: {:?} mode={:?}",
        key.bare_key,
        std::mem::discriminant(&state.mode),
    ));


    // Help mode: any key dismisses
    if state.mode.is_help() {
        state.mode.dismiss_help();
        crate::wasm_compat::set_selectable_wasm(state.mode.is_selectable());
        return true;
    }

    match &state.mode {
        SidebarMode::Passive => false, // Keys handled by controller keybindings
        SidebarMode::Navigate(_) => handle_navigate_key(state, key),
        SidebarMode::NavigateFilter { .. } => handle_filter_key(state, key),
        SidebarMode::NavigateDeleteConfirm { .. } => handle_delete_confirm_key(state, key),
        SidebarMode::NavigateRename { .. } => handle_rename_key(state, key),
        SidebarMode::RenamePassive { .. } => handle_rename_key(state, key),
        SidebarMode::Help(_) => false, // Already handled above
    }
}

/// Enter navigate mode, or move cursor up if already navigating.
/// Called when controller forwards navigate-prev keybinding (Shift-Alt-s).
pub fn toggle_navigate_prev(state: &mut SidebarState) {
    crate::debug_log(&format!(
        "SIDEBAR NAV-PREV: mode={:?} tab={:?}",
        std::mem::discriminant(&state.mode),
        state.my_tab_index,
    ));
    match &state.mode {
        SidebarMode::Passive => {
            let start = active_session_index(state);
            let ctx = NavigateContext {
                cursor_index: start,
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            crate::wasm_compat::set_selectable_wasm(true);
            focus_self_wasm();
            crate::debug_log(&format!("SIDEBAR NAV-PREV: entered Navigate cursor={start}"));
        }
        SidebarMode::Navigate(_) => {
            // Already navigating: move cursor up
            let count = state.filtered_sessions().len();
            if let Some(ctx) = state.mode.nav_ctx_mut() {
                ctx.cursor_index = if count == 0 {
                    0
                } else if ctx.cursor_index == 0 {
                    count - 1
                } else {
                    ctx.cursor_index - 1
                };
                crate::debug_log(&format!("SIDEBAR NAV-PREV: cursor up -> {}", ctx.cursor_index));
            }
            // Re-assert focus (see toggle_navigate for rationale)
            focus_self_wasm();
        }
        _ => {
            // Cancel current mode and enter Navigate directly
            crate::debug_log(&format!(
                "SIDEBAR NAV-PREV: mode {:?}, cancelling and entering navigate",
                std::mem::discriminant(&state.mode),
            ));
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            let start = active_session_index(state);
            let ctx = NavigateContext {
                cursor_index: start,
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            crate::wasm_compat::set_selectable_wasm(true);
            focus_self_wasm();
            crate::debug_log(&format!("SIDEBAR NAV-PREV: entered Navigate cursor={start}"));
        }
    }
}

/// Find the index of the currently active (focused) session in the filtered list.
fn active_session_index(state: &SidebarState) -> usize {
    let focused = state.focused_pane_id();
    if let Some(pid) = focused {
        state
            .filtered_sessions()
            .iter()
            .position(|s| s.pane_id == pid)
            .unwrap_or(0)
    } else {
        0
    }
}

/// Enter navigate mode, or move cursor down if already navigating.
/// Called when controller forwards navigate keybinding (Alt-s).
pub fn toggle_navigate(state: &mut SidebarState) {
    crate::debug_log(&format!(
        "SIDEBAR NAV: mode={:?} tab={:?}",
        std::mem::discriminant(&state.mode),
        state.my_tab_index,
    ));
    match &state.mode {
        SidebarMode::Passive => {
            let start = active_session_index(state);
            let ctx = NavigateContext {
                cursor_index: start,
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            crate::wasm_compat::set_selectable_wasm(true);
            focus_self_wasm();
            crate::debug_log(&format!("SIDEBAR NAV: entered Navigate cursor={start}"));
        }
        SidebarMode::Navigate(_) => {
            // Already navigating: move cursor down
            let count = state.filtered_sessions().len();
            if let Some(ctx) = state.mode.nav_ctx_mut() {
                ctx.cursor_index = if count == 0 {
                    0
                } else {
                    (ctx.cursor_index + 1) % count
                };
                crate::debug_log(&format!("SIDEBAR NAV: cursor down -> {}", ctx.cursor_index));
            }
            // Re-assert focus: the keybinding routes through the controller
            // plugin, which can cause Zellij to shift focus away from the
            // sidebar. Without this, key events (Enter, Esc, j/k) are
            // delivered to the terminal instead.
            focus_self_wasm();
        }
        _ => {
            // In another mode (filter, rename, delete confirm, RenamePassive):
            // cancel it and enter Navigate directly so Alt+s always works.
            crate::debug_log(&format!(
                "SIDEBAR NAV: mode {:?}, cancelling and entering navigate",
                std::mem::discriminant(&state.mode),
            ));
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            // Re-enter navigate from the now-Passive state
            let start = active_session_index(state);
            let ctx = NavigateContext {
                cursor_index: start,
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            crate::wasm_compat::set_selectable_wasm(true);
            focus_self_wasm();
            crate::debug_log(&format!("SIDEBAR NAV: entered Navigate cursor={start}"));
        }
    }
}

// --- Left click ---

fn handle_left_click(state: &mut SidebarState, row: usize) -> bool {
    // Check for special click regions
    if let Some(&(_r, pane_id, _)) = state.click_regions.iter().find(|(r, _, _)| *r == row) {
        if pane_id == super::render::VOICE_CLICK_SENTINEL {
            // Immediate visual feedback: toggle mute locally before
            // the controller round-trip. Cleared when the payload confirms.
            let current = state.local_mute_override.unwrap_or_else(|| {
                state.cached_payload.as_ref().is_some_and(|p| p.voice_muted)
            });
            state.local_mute_override = Some(!current);
            send_action(state, ActionType::VoiceMute, None, None, None);
            return true;
        }
        if pane_id == u32::MAX - 1 {
            // Header clicked: enter/cycle navigate mode
            toggle_navigate(state);
            return true;
        }
        if pane_id == super::render::SEPARATOR_CLICK_SENTINEL {
            // Separator clicked: switch to first session
            let sessions = state.filtered_sessions();
            if let Some(first) = sessions.first() {
                let first_pane = first.pane_id;
                let first_tab = first.tab_index;
                drop(sessions);
                state.local_focus_override = Some(first_pane);
                send_action(state, ActionType::Switch, Some(first_pane), Some(first_tab), None);
            }
            return true;
        }
    }

    // Find clicked session
    let hit = state
        .click_regions
        .iter()
        .find(|(r, pid, _)| *r <= row && row < r + 3 && *pid != u32::MAX && *pid != u32::MAX - 1 && *pid != super::render::SEPARATOR_CLICK_SENTINEL && *pid != super::render::VOICE_CLICK_SENTINEL)
        .copied();

    if let Some((_r, pane_id, tab_index)) = hit {
        // Double-click detection for rename
        let now = unix_now_ms();
        if let Some((last_ts, last_pid)) = state.last_click {
            let delta = now.saturating_sub(last_ts);
            crate::debug_log(&format!(
                "SIDEBAR CLICK: row={row} pane_id={pane_id} last_pid={last_pid} delta={delta}ms dbl={}",
                last_pid == pane_id && delta < DOUBLE_CLICK_MS,
            ));
            if last_pid == pane_id && delta < DOUBLE_CLICK_MS {
                state.last_click = None;
                return enter_rename_passive(state, pane_id);
            }
        } else {
            crate::debug_log(&format!(
                "SIDEBAR CLICK: row={row} pane_id={pane_id} first_click mode={:?}",
                std::mem::discriminant(&state.mode),
            ));
        }
        state.last_click = Some((now, pane_id));

        // Single click: switch to session with immediate highlight
        state.local_focus_override = Some(pane_id);
        send_action(
            state,
            ActionType::Switch,
            Some(pane_id),
            Some(tab_index),
            None,
        );
        // Exit navigate or RenamePassive mode on switch
        if state.mode.is_navigating() || matches!(state.mode, SidebarMode::RenamePassive { .. }) {
            crate::debug_log(&format!(
                "SIDEBAR CLICK: exiting {:?}, switching to pane={pane_id}",
                std::mem::discriminant(&state.mode),
            ));
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            crate::wasm_compat::set_selectable_wasm(false);
        }
        return true;
    }
    false
}

fn handle_right_click(state: &mut SidebarState, row: usize) -> bool {
    let hit = state
        .click_regions
        .iter()
        .find(|(r, pid, _)| *r <= row && row < r + 3 && *pid != u32::MAX && *pid != u32::MAX - 1 && *pid != super::render::SEPARATOR_CLICK_SENTINEL && *pid != super::render::VOICE_CLICK_SENTINEL)
        .copied();

    if let Some((_r, pane_id, _tab_index)) = hit {
        enter_rename_passive(state, pane_id)
    } else {
        false
    }
}

fn enter_rename_passive(state: &mut SidebarState, pane_id: u32) -> bool {
    let display_name = state
        .cached_payload
        .as_ref()
        .and_then(|p| p.sessions.iter().find(|s| s.pane_id == pane_id))
        .map(|s| s.display_name.clone())
        .unwrap_or_default();

    // Clear any leftover filter text when entering RenamePassive
    // (may be set if we came here via right-click while navigating with an active filter)
    state.filter_text.clear();

    let rename = RenameState {
        pane_id,
        input_buffer: display_name.clone(),
        cursor_pos: display_name.len(),
    };
    let entered_at_ms = unix_now_ms();
    crate::debug_log(&format!(
        "SIDEBAR RENAME: entering RenamePassive pane_id={pane_id} name={display_name:?}",
    ));
    state.mode = SidebarMode::RenamePassive {
        rename,
        entered_at_ms,
    };
    crate::wasm_compat::set_selectable_wasm(true);
    focus_self_wasm();
    true
}

// --- Navigate mode keys ---

fn handle_navigate_key(state: &mut SidebarState, key: KeyWithModifier) -> bool {
    let sessions = state.filtered_sessions();
    let count = sessions.len();

    match key.bare_key {
        BareKey::Char('j') | BareKey::Down => {
            if let Some(ctx) = state.mode.nav_ctx_mut() {
                ctx.cursor_index = if count == 0 {
                    0
                } else {
                    (ctx.cursor_index + 1) % count
                };
            }
            true
        }
        BareKey::Char('k') | BareKey::Up => {
            if let Some(ctx) = state.mode.nav_ctx_mut() {
                ctx.cursor_index = if count == 0 {
                    0
                } else if ctx.cursor_index == 0 {
                    count - 1
                } else {
                    ctx.cursor_index - 1
                };
            }
            true
        }
        BareKey::Enter => {
            // Switch to session at cursor. Set local_focus_override for
            // immediate highlight and go directly to Passive.
            let cursor = state.mode.cursor_index();
            let target = if cursor < sessions.len() {
                Some((sessions[cursor].pane_id, sessions[cursor].tab_index))
            } else {
                None
            };
            // Drop the immutable borrow on sessions before mutating state
            drop(sessions);

            if let Some((pane_id, tab_index)) = target {
                crate::debug_log(&format!(
                    "SIDEBAR ENTER: switching to pane={pane_id} tab={tab_index} cursor={cursor}"
                ));
                send_action(
                    state,
                    ActionType::Switch,
                    Some(pane_id),
                    Some(tab_index),
                    None,
                );
                state.local_focus_override = Some(pane_id);
            }
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            crate::wasm_compat::set_selectable_wasm(false);
            true
        }
        BareKey::Esc => {
            crate::debug_log("SIDEBAR ESC: exiting navigate");
            exit_navigate(state);
            true
        }
        BareKey::Char('d') => {
            // Enter delete confirmation
            let cursor = state.mode.cursor_index();
            if cursor < sessions.len() {
                let pane_id = sessions[cursor].pane_id;
                if let SidebarMode::Navigate(ctx) = std::mem::take(&mut state.mode) {
                    state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id };
                }
            }
            true
        }
        BareKey::Char('r') => {
            // Enter rename mode
            let cursor = state.mode.cursor_index();
            if cursor < sessions.len() {
                let s = sessions[cursor];
                let rename = RenameState {
                    pane_id: s.pane_id,
                    input_buffer: s.display_name.clone(),
                    cursor_pos: s.display_name.len(),
                };
                if let SidebarMode::Navigate(ctx) = std::mem::take(&mut state.mode) {
                    state.mode = SidebarMode::NavigateRename { ctx, rename };
                }
            }
            true
        }
        BareKey::Char('p') => {
            // Toggle pause
            let cursor = state.mode.cursor_index();
            if cursor < sessions.len() {
                let pane_id = sessions[cursor].pane_id;
                send_action(state, ActionType::Pause, Some(pane_id), None, None);
            }
            true
        }
        BareKey::Char('/') => {
            // Enter filter mode
            if let SidebarMode::Navigate(ctx) = std::mem::take(&mut state.mode) {
                state.mode = SidebarMode::NavigateFilter {
                    ctx,
                    filter: FilterState::default(),
                };
            }
            true
        }
        BareKey::Char('R') => {
            send_action(state, ActionType::Refresh, None, None, None);
            true
        }
        BareKey::Char('?') | BareKey::F(1) => {
            state.mode.toggle_help();
            true
        }
        BareKey::Char('m') => {
            let current = state.local_mute_override.unwrap_or_else(|| {
                state.cached_payload.as_ref().is_some_and(|p| p.voice_muted)
            });
            state.local_mute_override = Some(!current);
            send_action(state, ActionType::VoiceMute, None, None, None);
            true
        }
        BareKey::Char('S') => {
            // Sort sessions by activity tier.
            // Store the pane_id at the cursor so the sidebar can relocate the
            // cursor after the sort completes and a new render payload arrives.
            let cursor = state.mode.cursor_index();
            let cursor_pane_id = if cursor < sessions.len() {
                Some(sessions[cursor].pane_id)
            } else {
                None
            };
            // Drop sessions borrow before mutating state
            drop(sessions);
            state.sort_cursor_pane_id = cursor_pane_id;
            send_action(state, ActionType::Sort, cursor_pane_id, None, None);
            true
        }
        BareKey::Char('n') => {
            // New session: create tab and exit navigate without restoring focus
            send_action(state, ActionType::NewSession, None, None, None);
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            crate::wasm_compat::set_selectable_wasm(false);
            true
        }
        BareKey::Char('J') => {
            // Move session down in sort order
            let cursor = state.mode.cursor_index();
            let cursor_pane_id = if cursor < sessions.len() {
                Some(sessions[cursor].pane_id)
            } else {
                None
            };
            drop(sessions);
            state.sort_cursor_pane_id = cursor_pane_id;
            send_action(state, ActionType::MoveDown, cursor_pane_id, None, None);
            true
        }
        BareKey::Char('K') => {
            // Move session up in sort order
            let cursor = state.mode.cursor_index();
            let cursor_pane_id = if cursor < sessions.len() {
                Some(sessions[cursor].pane_id)
            } else {
                None
            };
            drop(sessions);
            state.sort_cursor_pane_id = cursor_pane_id;
            send_action(state, ActionType::MoveUp, cursor_pane_id, None, None);
            true
        }
        _ => false,
    }
}

// --- Filter mode keys ---

fn handle_filter_key(state: &mut SidebarState, key: KeyWithModifier) -> bool {
    match key.bare_key {
        BareKey::Esc => {
            // Exit filter, return to navigate
            if let SidebarMode::NavigateFilter { ctx, .. } = std::mem::take(&mut state.mode) {
                state.mode = SidebarMode::Navigate(ctx);
            }
            true
        }
        BareKey::Enter => {
            // Confirm filter, stay in navigate with filtered results
            if let SidebarMode::NavigateFilter { mut ctx, filter } =
                std::mem::take(&mut state.mode)
            {
                state.filter_text = filter.input_buffer;
                ctx.cursor_index = 0;
                state.mode = SidebarMode::Navigate(ctx);
            }
            true
        }
        BareKey::Backspace => {
            if let SidebarMode::NavigateFilter { filter, .. } = &mut state.mode {
                if filter.cursor_pos > 0 {
                    let prev = filter.input_buffer[..filter.cursor_pos]
                        .char_indices()
                        .last()
                        .map(|(i, _)| i)
                        .unwrap_or(0);
                    filter.input_buffer.remove(prev);
                    filter.cursor_pos = prev;
                }
            }
            // Clamp cursor after filter change narrows/widens results
            state.preserve_cursor();
            true
        }
        BareKey::Char(c) => {
            if let SidebarMode::NavigateFilter { filter, .. } = &mut state.mode {
                filter.input_buffer.insert(filter.cursor_pos, c);
                filter.cursor_pos += c.len_utf8();
            }
            // Clamp cursor after filter change narrows/widens results
            state.preserve_cursor();
            true
        }
        _ => false,
    }
}

// --- Delete confirm keys ---

fn handle_delete_confirm_key(state: &mut SidebarState, key: KeyWithModifier) -> bool {
    match key.bare_key {
        BareKey::Char('y') | BareKey::Char('Y') => {
            if let SidebarMode::NavigateDeleteConfirm { ctx, pane_id } =
                std::mem::take(&mut state.mode)
            {
                send_action(state, ActionType::Delete, Some(pane_id), None, None);
                state.mode = SidebarMode::Navigate(ctx);
            }
            true
        }
        BareKey::Char('n') | BareKey::Char('N') | BareKey::Esc => {
            if let SidebarMode::NavigateDeleteConfirm { ctx, .. } =
                std::mem::take(&mut state.mode)
            {
                state.mode = SidebarMode::Navigate(ctx);
            }
            true
        }
        _ => false,
    }
}

// --- Rename keys ---

fn handle_rename_key(state: &mut SidebarState, key: KeyWithModifier) -> bool {
    let (pane_id, action) = match &mut state.mode {
        SidebarMode::NavigateRename { rename, .. } => {
            let pid = rename.pane_id;
            (pid, rename::handle_key(rename, key))
        }
        SidebarMode::RenamePassive { rename, .. } => {
            let pid = rename.pane_id;
            (pid, rename::handle_key(rename, key))
        }
        _ => return false,
    };

    match action {
        RenameAction::Continue => true,
        RenameAction::Confirm(name) => {
            send_action(
                state,
                ActionType::Rename,
                Some(pane_id),
                None,
                Some(name),
            );
            match std::mem::take(&mut state.mode) {
                SidebarMode::NavigateRename { ctx, .. } => {
                    state.mode = SidebarMode::Navigate(ctx);
                }
                SidebarMode::RenamePassive { .. } => {
                    state.mode = SidebarMode::Passive;
                    state.filter_text.clear();
                    crate::wasm_compat::set_selectable_wasm(false);
                }
                other => state.mode = other,
            }
            true
        }
        RenameAction::Cancel => {
            match std::mem::take(&mut state.mode) {
                SidebarMode::NavigateRename { ctx, .. } => {
                    state.mode = SidebarMode::Navigate(ctx);
                }
                SidebarMode::RenamePassive { .. } => {
                    state.mode = SidebarMode::Passive;
                    state.filter_text.clear();
                    crate::wasm_compat::set_selectable_wasm(false);
                }
                other => state.mode = other,
            }
            true
        }
    }
}

// --- Helpers ---

fn exit_navigate(state: &mut SidebarState) {
    crate::debug_log(&format!(
        "SIDEBAR EXIT-NAV: mode={:?}",
        std::mem::discriminant(&state.mode),
    ));
    if let Some(ctx) = state.mode.nav_ctx() {
        // Restore focus to the original pane
        if let (Some(pid), Some(tab)) = (ctx.restore_pane_id, ctx.restore_tab_index) {
            crate::debug_log(&format!("SIDEBAR EXIT-NAV: restoring pane={pid} tab={tab}"));
            send_action(
                state,
                ActionType::Switch,
                Some(pid),
                Some(tab),
                None,
            );
        }
    }
    state.mode = SidebarMode::Passive;
    state.filter_text.clear();
    crate::wasm_compat::set_selectable_wasm(false);
}

fn send_action(
    state: &SidebarState,
    action: ActionType,
    pane_id: Option<u32>,
    tab_index: Option<usize>,
    value: Option<String>,
) {
    let msg = ActionMessage {
        action,
        pane_id,
        tab_index,
        value,
        sidebar_plugin_id: state.my_plugin_id,
    };
    send_action_wasm(&msg);
}

// --- WASM-gated host function wrappers ---

#[cfg(target_family = "wasm")]
fn send_action_wasm(msg: &ActionMessage) {
    let json = match serde_json::to_string(msg) {
        Ok(j) => j,
        Err(_) => return,
    };
    let mut pipe_msg = MessageToPlugin::new("cc-deck:action");
    pipe_msg.message_payload = Some(json);
    pipe_message_to_plugin(pipe_msg);
}

#[cfg(not(target_family = "wasm"))]
fn send_action_wasm(_msg: &ActionMessage) {}

#[cfg(target_family = "wasm")]
pub fn focus_self_wasm() {
    focus_plugin_pane(get_plugin_ids().plugin_id, true, false);
}

#[cfg(not(target_family = "wasm"))]
pub fn focus_self_wasm() {}

#[cfg(test)]
mod tests {
    use super::*;
    use super::super::modes::{FilterState, NavigateContext, RenameState, SidebarMode};
    use super::super::render::VOICE_CLICK_SENTINEL;
    use super::super::test_helpers::*;

    // --- handle_key dispatcher ---

    #[test]
    fn test_passive_ignores_keys() {
        let mut state = make_state_with_sessions(&[(1, "api", 0)]);
        assert!(!handle_key(&mut state, bare(BareKey::Char('j'))));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_help_any_key_dismisses() {
        let mut state = make_nav_state(&[(1, "api", 0)], 0);
        state.mode.toggle_help();
        assert!(state.mode.is_help());
        assert!(handle_key(&mut state, bare(BareKey::Char('x'))));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_help_esc_dismisses() {
        let mut state = make_state_with_sessions(&[(1, "api", 0)]);
        state.mode = SidebarMode::Help(Box::new(SidebarMode::Passive));
        assert!(handle_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    // --- toggle_navigate ---

    #[test]
    fn test_toggle_nav_from_passive() {
        let mut state = make_state_with_sessions(&[(1, "api", 0), (2, "web", 1), (3, "db", 2)]);
        toggle_navigate(&mut state);
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_toggle_nav_from_passive_no_sessions() {
        let mut state = SidebarState::default();
        toggle_navigate(&mut state);
        assert!(state.mode.is_navigating());
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_toggle_nav_already_navigating_moves_down() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1), (3, "c", 2)], 0);
        toggle_navigate(&mut state);
        assert_eq!(state.mode.cursor_index(), 1);
    }

    #[test]
    fn test_toggle_nav_wraps_around() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1), (3, "c", 2)], 2);
        toggle_navigate(&mut state);
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_toggle_nav_from_filter_cancels() {
        let mut state = make_state_with_sessions(&[(1, "api", 0)]);
        state.mode = SidebarMode::NavigateFilter {
            ctx: NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 },
            filter: FilterState::default(),
        };
        state.filter_text = "old".to_string();
        toggle_navigate(&mut state);
        assert!(state.mode.is_navigating());
        assert!(state.filter_text.is_empty());
    }

    #[test]
    fn test_toggle_nav_from_rename_passive_cancels() {
        let mut state = make_state_with_sessions(&[(1, "api", 0)]);
        state.mode = SidebarMode::RenamePassive {
            rename: RenameState { pane_id: 1, input_buffer: "api".into(), cursor_pos: 3 },
            entered_at_ms: 0,
        };
        toggle_navigate(&mut state);
        assert!(state.mode.is_navigating());
    }

    // --- toggle_navigate_prev ---

    #[test]
    fn test_toggle_nav_prev_from_passive() {
        let mut state = make_state_with_sessions(&[(1, "a", 0), (2, "b", 1), (3, "c", 2)]);
        toggle_navigate_prev(&mut state);
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_toggle_nav_prev_already_navigating_moves_up() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1), (3, "c", 2)], 1);
        toggle_navigate_prev(&mut state);
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_toggle_nav_prev_wraps_to_end() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1), (3, "c", 2)], 0);
        toggle_navigate_prev(&mut state);
        assert_eq!(state.mode.cursor_index(), 2);
    }

    // --- handle_navigate_key ---

    #[test]
    fn test_nav_j_moves_down() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('j'))));
        assert_eq!(state.mode.cursor_index(), 1);
    }

    #[test]
    fn test_nav_k_moves_up() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 1);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('k'))));
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_nav_down_arrow() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Down)));
        assert_eq!(state.mode.cursor_index(), 1);
    }

    #[test]
    fn test_nav_up_arrow() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 1);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Up)));
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_nav_j_wraps() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 1);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('j'))));
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_nav_k_wraps() {
        let mut state = make_nav_state(&[(1, "a", 0), (2, "b", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('k'))));
        assert_eq!(state.mode.cursor_index(), 1);
    }

    #[test]
    fn test_nav_enter_switches_and_exits() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Enter)));
        assert!(matches!(state.mode, SidebarMode::Passive));
        assert_eq!(state.local_focus_override, Some(10));
        assert!(state.filter_text.is_empty());
    }

    #[test]
    fn test_nav_enter_empty_sessions() {
        let mut state = make_nav_state(&[], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Enter)));
        assert!(matches!(state.mode, SidebarMode::Passive));
        assert_eq!(state.local_focus_override, None);
    }

    #[test]
    fn test_nav_esc_exits() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        state.filter_text = "some filter".into();
        assert!(handle_navigate_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Passive));
        assert!(state.filter_text.is_empty());
    }

    #[test]
    fn test_nav_d_enters_delete_confirm() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('d'))));
        match &state.mode {
            SidebarMode::NavigateDeleteConfirm { pane_id, .. } => assert_eq!(*pane_id, 10),
            other => panic!("expected NavigateDeleteConfirm, got {:?}", std::mem::discriminant(other)),
        }
    }

    #[test]
    fn test_nav_d_empty_sessions() {
        let mut state = make_nav_state(&[], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('d'))));
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_nav_r_enters_rename() {
        let mut state = make_nav_state(&[(10, "api", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('r'))));
        match &state.mode {
            SidebarMode::NavigateRename { rename, .. } => {
                assert_eq!(rename.pane_id, 10);
                assert_eq!(rename.input_buffer, "api");
            }
            other => panic!("expected NavigateRename, got {:?}", std::mem::discriminant(other)),
        }
    }

    #[test]
    fn test_nav_r_empty_sessions() {
        let mut state = make_nav_state(&[], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('r'))));
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_nav_slash_enters_filter() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('/'))));
        assert!(matches!(state.mode, SidebarMode::NavigateFilter { .. }));
    }

    #[test]
    fn test_nav_question_enters_help() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('?'))));
        assert!(state.mode.is_help());
    }

    #[test]
    fn test_nav_f1_enters_help() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::F(1))));
        assert!(state.mode.is_help());
    }

    #[test]
    fn test_nav_p_stays_navigate() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('p'))));
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_nav_m_toggles_mute() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert_eq!(state.local_mute_override, None);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('m'))));
        assert_eq!(state.local_mute_override, Some(true));
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('m'))));
        assert_eq!(state.local_mute_override, Some(false));
    }

    #[test]
    fn test_nav_n_exits_to_passive() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('n'))));
        assert!(matches!(state.mode, SidebarMode::Passive));
        assert!(state.filter_text.is_empty());
    }

    #[test]
    fn test_nav_s_sort_dispatches_sort_action() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('S'))));
        assert!(state.mode.is_navigating());
        assert_eq!(state.sort_cursor_pane_id, Some(10));
    }

    #[test]
    fn test_nav_s_sort_passes_cursor_pane_id() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 1);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('S'))));
        assert!(state.mode.is_navigating());
        assert_eq!(state.sort_cursor_pane_id, Some(20));
    }

    #[test]
    fn test_nav_s_sort_empty_sessions() {
        let mut state = make_nav_state(&[], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('S'))));
        assert!(state.mode.is_navigating());
        assert_eq!(state.sort_cursor_pane_id, None);
    }

    #[test]
    fn test_nav_j_move_down_dispatches_action() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('J'))));
        assert!(state.mode.is_navigating());
        assert_eq!(state.sort_cursor_pane_id, Some(10));
    }

    #[test]
    fn test_nav_k_move_up_dispatches_action() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 1);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('K'))));
        assert!(state.mode.is_navigating());
        assert_eq!(state.sort_cursor_pane_id, Some(20));
    }

    #[test]
    fn test_nav_j_move_empty_sessions() {
        let mut state = make_nav_state(&[], 0);
        assert!(handle_navigate_key(&mut state, bare(BareKey::Char('J'))));
        assert_eq!(state.sort_cursor_pane_id, None);
    }

    #[test]
    fn test_passive_s_sort_ignored() {
        let mut state = make_state_with_sessions(&[(1, "api", 0)]);
        assert!(!handle_key(&mut state, bare(BareKey::Char('S'))));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_nav_unknown_key_returns_false() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        assert!(!handle_navigate_key(&mut state, bare(BareKey::Char('z'))));
        assert!(state.mode.is_navigating());
    }

    // --- handle_filter_key ---

    #[test]
    fn test_filter_esc_returns_to_navigate() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateFilter { ctx, filter: FilterState::default() };
        assert!(handle_filter_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_filter_enter_applies_filter() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        let mut filter = FilterState::default();
        filter.input_buffer = "api".into();
        filter.cursor_pos = 3;
        state.mode = SidebarMode::NavigateFilter { ctx, filter };
        assert!(handle_filter_key(&mut state, bare(BareKey::Enter)));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
        assert_eq!(state.filter_text, "api");
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_filter_char_appends() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateFilter { ctx, filter: FilterState::default() };
        assert!(handle_filter_key(&mut state, bare(BareKey::Char('a'))));
        if let SidebarMode::NavigateFilter { filter, .. } = &state.mode {
            assert_eq!(filter.input_buffer, "a");
            assert_eq!(filter.cursor_pos, 1);
        } else {
            panic!("expected NavigateFilter");
        }
    }

    #[test]
    fn test_filter_backspace_removes() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        let mut filter = FilterState::default();
        filter.input_buffer = "ab".into();
        filter.cursor_pos = 2;
        state.mode = SidebarMode::NavigateFilter { ctx, filter };
        assert!(handle_filter_key(&mut state, bare(BareKey::Backspace)));
        if let SidebarMode::NavigateFilter { filter, .. } = &state.mode {
            assert_eq!(filter.input_buffer, "a");
            assert_eq!(filter.cursor_pos, 1);
        } else {
            panic!("expected NavigateFilter");
        }
    }

    #[test]
    fn test_filter_backspace_at_start() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateFilter { ctx, filter: FilterState::default() };
        assert!(handle_filter_key(&mut state, bare(BareKey::Backspace)));
        if let SidebarMode::NavigateFilter { filter, .. } = &state.mode {
            assert!(filter.input_buffer.is_empty());
        }
    }

    #[test]
    fn test_filter_unknown_key_returns_false() {
        let mut state = make_state_with_sessions(&[(1, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateFilter { ctx, filter: FilterState::default() };
        assert!(!handle_filter_key(&mut state, bare(BareKey::Tab)));
    }

    // --- handle_delete_confirm_key ---

    #[test]
    fn test_delete_y_confirms() {
        let mut state = make_state_with_sessions(&[(10, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id: 10 };
        assert!(handle_delete_confirm_key(&mut state, bare(BareKey::Char('y'))));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_delete_uppercase_y_confirms() {
        let mut state = make_state_with_sessions(&[(10, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id: 10 };
        assert!(handle_delete_confirm_key(&mut state, bare(BareKey::Char('Y'))));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_delete_n_cancels() {
        let mut state = make_state_with_sessions(&[(10, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id: 10 };
        assert!(handle_delete_confirm_key(&mut state, bare(BareKey::Char('n'))));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_delete_esc_cancels() {
        let mut state = make_state_with_sessions(&[(10, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id: 10 };
        assert!(handle_delete_confirm_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_delete_unknown_key_returns_false() {
        let mut state = make_state_with_sessions(&[(10, "a", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        state.mode = SidebarMode::NavigateDeleteConfirm { ctx, pane_id: 10 };
        assert!(!handle_delete_confirm_key(&mut state, bare(BareKey::Char('x'))));
    }

    // --- handle_rename_key ---

    #[test]
    fn test_rename_navigate_enter_confirms() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        let rename = RenameState { pane_id: 10, input_buffer: "new-name".into(), cursor_pos: 8 };
        state.mode = SidebarMode::NavigateRename { ctx, rename };
        assert!(handle_rename_key(&mut state, bare(BareKey::Enter)));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_rename_navigate_esc_cancels() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        let rename = RenameState { pane_id: 10, input_buffer: "api".into(), cursor_pos: 3 };
        state.mode = SidebarMode::NavigateRename { ctx, rename };
        assert!(handle_rename_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Navigate(_)));
    }

    #[test]
    fn test_rename_passive_enter_confirms() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        let rename = RenameState { pane_id: 10, input_buffer: "new-name".into(), cursor_pos: 8 };
        state.mode = SidebarMode::RenamePassive { rename, entered_at_ms: 0 };
        assert!(handle_rename_key(&mut state, bare(BareKey::Enter)));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_rename_passive_esc_cancels() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        let rename = RenameState { pane_id: 10, input_buffer: "api".into(), cursor_pos: 3 };
        state.mode = SidebarMode::RenamePassive { rename, entered_at_ms: 0 };
        assert!(handle_rename_key(&mut state, bare(BareKey::Esc)));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_rename_typing_continues() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        let ctx = NavigateContext { cursor_index: 0, restore_pane_id: None, restore_tab_index: None, entered_at_ms: 0 };
        let rename = RenameState { pane_id: 10, input_buffer: "api".into(), cursor_pos: 3 };
        state.mode = SidebarMode::NavigateRename { ctx, rename };
        assert!(handle_rename_key(&mut state, bare(BareKey::Char('x'))));
        assert!(matches!(state.mode, SidebarMode::NavigateRename { .. }));
    }

    // --- Mouse handlers ---

    #[test]
    fn test_left_click_session_switches() {
        let mut state = make_state_with_sessions(&[(10, "api", 0), (20, "web", 1)]);
        state.click_regions = vec![(2, 10, 0), (5, 20, 1)];
        assert!(handle_left_click(&mut state, 2));
        assert_eq!(state.local_focus_override, Some(10));
    }

    #[test]
    fn test_left_click_exits_navigate() {
        let mut state = make_nav_state(&[(10, "api", 0), (20, "web", 1)], 0);
        state.click_regions = vec![(2, 10, 0), (5, 20, 1)];
        assert!(handle_left_click(&mut state, 2));
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_left_click_header_toggles_nav() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        state.click_regions = vec![(0, u32::MAX - 1, 0)];
        assert!(handle_left_click(&mut state, 0));
        assert!(state.mode.is_navigating());
    }

    #[test]
    fn test_left_click_voice_toggles_mute() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        state.click_regions = vec![(0, VOICE_CLICK_SENTINEL, 0)];
        assert!(handle_left_click(&mut state, 0));
        assert_eq!(state.local_mute_override, Some(true));
    }

    #[test]
    fn test_left_click_no_hit() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        state.click_regions = vec![(2, 10, 0)];
        assert!(!handle_left_click(&mut state, 10));
    }

    #[test]
    fn test_right_click_enters_rename() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        state.click_regions = vec![(2, 10, 0)];
        assert!(handle_right_click(&mut state, 2));
        assert!(matches!(state.mode, SidebarMode::RenamePassive { .. }));
    }

    #[test]
    fn test_right_click_no_hit() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        state.click_regions = vec![(2, 10, 0)];
        assert!(!handle_right_click(&mut state, 10));
    }

    // --- active_session_index ---

    #[test]
    fn test_active_session_index_found() {
        let mut state = make_state_with_sessions(&[(10, "a", 0), (20, "b", 1)]);
        if let Some(ref mut p) = state.cached_payload {
            p.focused_pane_id = Some(20);
        }
        assert_eq!(active_session_index(&state), 1);
    }

    #[test]
    fn test_active_session_index_not_found() {
        let state = make_state_with_sessions(&[(10, "a", 0), (20, "b", 1)]);
        assert_eq!(active_session_index(&state), 0);
    }

    #[test]
    fn test_active_session_index_no_payload() {
        let state = SidebarState::default();
        assert_eq!(active_session_index(&state), 0);
    }

    // --- exit_navigate ---

    #[test]
    fn test_exit_navigate_clears_mode_and_filter() {
        let mut state = make_nav_state(&[(1, "a", 0)], 0);
        state.filter_text = "test".into();
        exit_navigate(&mut state);
        assert!(matches!(state.mode, SidebarMode::Passive));
        assert!(state.filter_text.is_empty());
    }

    // --- enter_rename_passive ---

    #[test]
    fn test_enter_rename_passive() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        assert!(enter_rename_passive(&mut state, 10));
        match &state.mode {
            SidebarMode::RenamePassive { rename, .. } => {
                assert_eq!(rename.pane_id, 10);
                assert_eq!(rename.input_buffer, "api");
                assert_eq!(rename.cursor_pos, 3);
            }
            other => panic!("expected RenamePassive, got {:?}", std::mem::discriminant(other)),
        }
    }

    #[test]
    fn test_enter_rename_passive_unknown_pane() {
        let mut state = make_state_with_sessions(&[(10, "api", 0)]);
        assert!(enter_rename_passive(&mut state, 99));
        match &state.mode {
            SidebarMode::RenamePassive { rename, .. } => {
                assert_eq!(rename.pane_id, 99);
                assert!(rename.input_buffer.is_empty());
            }
            other => panic!("expected RenamePassive, got {:?}", std::mem::discriminant(other)),
        }
    }
}
