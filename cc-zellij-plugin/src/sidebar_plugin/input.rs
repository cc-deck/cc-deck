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

const DOUBLE_CLICK_MS: u64 = 500;

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
    // Help mode: any key dismisses
    if state.mode.is_help() {
        state.mode.dismiss_help();
        set_selectable_wasm(state.mode.is_selectable());
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
    match &state.mode {
        SidebarMode::Passive => {
            // Enter navigate with cursor at the last session
            let count = state.filtered_sessions().len();
            let ctx = NavigateContext {
                cursor_index: if count > 0 { count - 1 } else { 0 },
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            set_selectable_wasm(true);
            focus_self_wasm();
            state.set_notification("Navigate mode", 2);
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
            }
        }
        _ => {
            exit_navigate(state);
        }
    }
}

/// Enter navigate mode, or move cursor down if already navigating.
/// Called when controller forwards navigate keybinding (Alt-s).
pub fn toggle_navigate(state: &mut SidebarState) {
    match &state.mode {
        SidebarMode::Passive => {
            let ctx = NavigateContext {
                cursor_index: 0,
                restore_pane_id: state.focused_pane_id(),
                restore_tab_index: state.my_tab_index,
                entered_at_ms: unix_now_ms(),
            };
            state.mode = SidebarMode::Navigate(ctx);
            set_selectable_wasm(true);
            focus_self_wasm();
            state.set_notification("Navigate mode", 2);
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
            }
        }
        _ => {
            // In a sub-mode (filter, rename, delete confirm): exit to passive
            exit_navigate(state);
        }
    }
}

// --- Left click ---

fn handle_left_click(state: &mut SidebarState, row: usize) -> bool {
    // Check for special click regions
    if let Some(&(_r, pane_id, _)) = state.click_regions.iter().find(|(r, _, _)| *r == row) {
        if pane_id == u32::MAX {
            send_action(state, ActionType::NewSession, None, None, None);
            return false;
        }
        if pane_id == u32::MAX - 1 {
            // Header clicked: enter/cycle navigate mode
            toggle_navigate(state);
            return true;
        }
    }

    // Find clicked session
    let hit = state
        .click_regions
        .iter()
        .find(|(r, pid, _)| *r <= row && row < r + 3 && *pid != u32::MAX && *pid != u32::MAX - 1)
        .copied();

    if let Some((_r, pane_id, tab_index)) = hit {
        // Double-click detection for rename
        let now = unix_now_ms();
        if let Some((last_ts, last_pid)) = state.last_click {
            if last_pid == pane_id && now.saturating_sub(last_ts) < DOUBLE_CLICK_MS {
                state.last_click = None;
                return enter_rename_passive(state, pane_id);
            }
        }
        state.last_click = Some((now, pane_id));

        // Single click: switch to session
        send_action(
            state,
            ActionType::Switch,
            Some(pane_id),
            Some(tab_index),
            None,
        );
        // Exit navigate mode on switch
        if state.mode.is_navigating() {
            state.mode = SidebarMode::Passive;
            set_selectable_wasm(false);
        }
        return true;
    }
    false
}

fn handle_right_click(state: &mut SidebarState, row: usize) -> bool {
    let hit = state
        .click_regions
        .iter()
        .find(|(r, pid, _)| *r <= row && row < r + 3 && *pid != u32::MAX && *pid != u32::MAX - 1)
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

    let rename = RenameState {
        pane_id,
        input_buffer: display_name.clone(),
        cursor_pos: display_name.len(),
    };
    state.mode = SidebarMode::RenamePassive {
        rename,
        entered_at_ms: unix_now_ms(),
    };
    set_selectable_wasm(true);
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
            // Switch to session at cursor, then exit navigate WITHOUT restoring
            let cursor = state.mode.cursor_index();
            if cursor < sessions.len() {
                let s = sessions[cursor];
                send_action(
                    state,
                    ActionType::Switch,
                    Some(s.pane_id),
                    Some(s.tab_index),
                    None,
                );
            }
            // Exit without restoring original pane (user chose a new target)
            state.mode = SidebarMode::Passive;
            state.filter_text.clear();
            set_selectable_wasm(false);
            true
        }
        BareKey::Esc => {
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
        BareKey::Char('?') | BareKey::F(1) => {
            state.mode.toggle_help();
            true
        }
        BareKey::Char('n') => {
            // New session
            send_action(state, ActionType::NewSession, None, None, None);
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
            true
        }
        BareKey::Char(c) => {
            if let SidebarMode::NavigateFilter { filter, .. } = &mut state.mode {
                filter.input_buffer.insert(filter.cursor_pos, c);
                filter.cursor_pos += c.len_utf8();
            }
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
                    set_selectable_wasm(false);
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
                    set_selectable_wasm(false);
                }
                other => state.mode = other,
            }
            true
        }
    }
}

// --- Helpers ---

fn exit_navigate(state: &mut SidebarState) {
    if let Some(ctx) = state.mode.nav_ctx() {
        // Restore focus to the original pane
        if let (Some(pid), Some(tab)) = (ctx.restore_pane_id, ctx.restore_tab_index) {
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
    set_selectable_wasm(false);
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
fn set_selectable_wasm(selectable: bool) {
    set_selectable(selectable);
}

#[cfg(not(target_family = "wasm"))]
fn set_selectable_wasm(_selectable: bool) {}

#[cfg(target_family = "wasm")]
fn focus_self_wasm() {
    focus_plugin_pane(get_plugin_ids().plugin_id, true, false);
}

#[cfg(not(target_family = "wasm"))]
fn focus_self_wasm() {}
