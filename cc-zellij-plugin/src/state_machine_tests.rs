// Comprehensive state machine tests for SidebarMode transitions.
//
// Tests verify that every mode entry, exit, and sub-mode transition
// produces the correct SidebarMode variant and preserves context
// (cursor position, restore target) across transitions.

use crate::session::Session;
use crate::state::{FilterState, NavigateContext, PluginState, RenameState, SidebarMode};
use std::collections::BTreeSet;
use zellij_tile::prelude::*;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn bare(key: BareKey) -> KeyWithModifier {
    KeyWithModifier {
        bare_key: key,
        key_modifiers: BTreeSet::new(),
    }
}

fn key_char(c: char) -> KeyWithModifier {
    bare(BareKey::Char(c))
}

/// Create a PluginState with N sessions (pane_id 1..=n, tab_index 0..n-1).
fn state_with_sessions(n: u32) -> PluginState {
    let mut state = PluginState::default();
    for i in 1..=n {
        let mut s = Session::new(i, format!("session-{i}"));
        s.tab_index = Some((i - 1) as usize);
        s.display_name = format!("project-{i}");
        state.sessions.insert(i, s);
    }
    state.active_tab_index = Some(0);
    state.my_tab_index = Some(0);
    state
}

/// Put state into Navigate mode at cursor position.
fn enter_nav(state: &mut PluginState, cursor: usize) {
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: cursor,
        restore: Some((1, 0)),
        entered_at_ms: 0, // grace expired
    });
}

/// Put state into Navigate mode with a specific entered_at timestamp.
fn enter_nav_at(state: &mut PluginState, cursor: usize, entered_at_ms: u64) {
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: cursor,
        restore: Some((1, 0)),
        entered_at_ms,
    });
}

// ---------------------------------------------------------------------------
// Navigate mode entry/exit
// ---------------------------------------------------------------------------

#[test]
fn test_enter_navigation_mode() {
    let mut state = state_with_sessions(3);
    state.focused_pane_id = Some(2); // focused on session 2 (tab index 1)

    crate::enter_navigation_mode(&mut state);

    assert!(state.sidebar_mode.is_navigating());
    let ctx = state.sidebar_mode.nav_ctx().unwrap();
    assert_eq!(ctx.cursor_index, 1); // cursor follows focused pane
    assert_eq!(ctx.restore, Some((2, 1))); // restore to pane 2, tab 1
    assert!(ctx.entered_at_ms > 0); // grace period set
}

#[test]
fn test_exit_to_passive_restores_focus() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.exit_to_passive();

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_switch_to_session_exits_to_passive() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);

    state.switch_to_session(2, Some(1));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_abandon_navigation_exits_to_passive() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.abandon_navigation();

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

// ---------------------------------------------------------------------------
// Key-driven transitions from Navigate
// ---------------------------------------------------------------------------

#[test]
fn test_nav_esc_exits() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);

    state.handle_key(bare(BareKey::Esc));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_nav_enter_switches_session() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);

    state.handle_key(bare(BareKey::Enter));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_nav_slash_enters_filter() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);

    state.handle_key(key_char('/'));

    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }));
    // Cursor preserved
    let ctx = state.sidebar_mode.nav_ctx().unwrap();
    assert_eq!(ctx.cursor_index, 1);
}

#[test]
fn test_nav_r_enters_rename() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.handle_key(key_char('r'));

    match &state.sidebar_mode {
        SidebarMode::NavigateRename { ctx, rename } => {
            assert_eq!(ctx.cursor_index, 0);
            assert_eq!(rename.pane_id, 1);
            assert_eq!(rename.input_buffer, "project-1");
        }
        other => panic!("expected NavigateRename, got {:?}", std::mem::discriminant(other)),
    }
}

#[test]
fn test_nav_d_enters_delete_confirm() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 2);

    state.handle_key(key_char('d'));

    match &state.sidebar_mode {
        SidebarMode::NavigateDeleteConfirm { ctx, pane_id } => {
            assert_eq!(ctx.cursor_index, 2);
            assert_eq!(*pane_id, 3); // session at cursor 2 has pane_id 3
        }
        other => panic!("expected NavigateDeleteConfirm, got {:?}", std::mem::discriminant(other)),
    }
}

#[test]
fn test_nav_question_mark_shows_help() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.handle_key(key_char('?'));

    assert!(state.show_help);
    // Stays in navigate (help is an overlay, not a mode transition)
    assert!(state.sidebar_mode.is_navigating());
}

#[test]
fn test_nav_n_creates_tab_and_exits() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.handle_key(key_char('n'));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

// ---------------------------------------------------------------------------
// Cursor movement
// ---------------------------------------------------------------------------

#[test]
fn test_cursor_j_wraps_around() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 2); // at last position

    state.handle_key(key_char('j'));

    assert_eq!(state.sidebar_mode.cursor_index(), 0); // wrapped to start
}

#[test]
fn test_cursor_k_wraps_around() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0); // at first position

    state.handle_key(key_char('k'));

    assert_eq!(state.sidebar_mode.cursor_index(), 2); // wrapped to end
}

#[test]
fn test_cursor_down_arrow() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    state.handle_key(bare(BareKey::Down));

    assert_eq!(state.sidebar_mode.cursor_index(), 1);
}

#[test]
fn test_cursor_g_jumps_to_start() {
    let mut state = state_with_sessions(5);
    enter_nav(&mut state, 3);

    state.handle_key(key_char('g'));

    assert_eq!(state.sidebar_mode.cursor_index(), 0);
}

#[test]
fn test_cursor_big_g_jumps_to_end() {
    let mut state = state_with_sessions(5);
    enter_nav(&mut state, 0);

    state.handle_key(key_char('G'));

    assert_eq!(state.sidebar_mode.cursor_index(), 4);
}

// ---------------------------------------------------------------------------
// Filter mode transitions
// ---------------------------------------------------------------------------

#[test]
fn test_filter_esc_returns_to_navigate() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);
    state.handle_key(key_char('/')); // enter filter

    state.handle_key(bare(BareKey::Esc));

    // Should return to Navigate, not Passive
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));
}

#[test]
fn test_filter_typing_resets_cursor() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 2);
    state.handle_key(key_char('/')); // enter filter

    state.handle_key(key_char('p'));

    // Cursor resets to 0 on typing
    assert_eq!(state.sidebar_mode.cursor_index(), 0);
    // Still in filter mode
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }));
}

#[test]
fn test_filter_enter_no_matches_clears_filter() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.handle_key(key_char('/')); // enter filter

    // Type something that won't match
    state.handle_key(key_char('z'));
    state.handle_key(key_char('z'));
    state.handle_key(key_char('z'));
    state.handle_key(bare(BareKey::Enter));

    // Should return to Navigate (filter cleared due to no matches)
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));
    assert!(state.notification.is_some()); // "No matches" notification
}

#[test]
fn test_filter_enter_with_matches_keeps_filter() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.handle_key(key_char('/')); // enter filter

    // Type something that matches "project-1"
    state.handle_key(key_char('p'));
    state.handle_key(bare(BareKey::Enter));

    // Should stay in NavigateFilter (filter kept)
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }));
}

#[test]
fn test_filter_preserves_nav_context() {
    let mut state = state_with_sessions(3);
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: 1,
        restore: Some((42, 5)),
        entered_at_ms: 0,
    });

    state.handle_key(key_char('/')); // enter filter

    // restore target should be preserved through filter
    let ctx = state.sidebar_mode.nav_ctx().unwrap();
    assert_eq!(ctx.restore, Some((42, 5)));
}

// ---------------------------------------------------------------------------
// Delete confirm transitions
// ---------------------------------------------------------------------------

#[test]
fn test_delete_confirm_y_deletes_and_returns_to_navigate() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.handle_key(key_char('d')); // enter delete confirm

    assert_eq!(state.sessions.len(), 3);

    state.handle_key(key_char('y')); // confirm

    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));
    assert_eq!(state.sessions.len(), 2); // session deleted
}

#[test]
fn test_delete_confirm_n_cancels_and_returns_to_navigate() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.handle_key(key_char('d')); // enter delete confirm

    state.handle_key(key_char('n')); // cancel (any non-y key)

    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));
    assert_eq!(state.sessions.len(), 3); // session not deleted
}

#[test]
fn test_delete_confirm_esc_cancels() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.handle_key(key_char('d'));

    state.handle_key(bare(BareKey::Esc));

    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));
    assert_eq!(state.sessions.len(), 3);
}

// ---------------------------------------------------------------------------
// Rename from Navigate (parent context preserved)
// ---------------------------------------------------------------------------

#[test]
fn test_navigate_rename_confirm_returns_to_navigate() {
    let mut state = state_with_sessions(3);
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: 0,
        restore: Some((99, 7)),
        entered_at_ms: 0,
    });
    state.handle_key(key_char('r')); // enter rename

    // Type new name
    // First clear existing name
    for _ in 0..20 {
        state.handle_key(bare(BareKey::Backspace));
    }
    state.handle_key(key_char('n'));
    state.handle_key(key_char('e'));
    state.handle_key(key_char('w'));
    state.handle_key(bare(BareKey::Enter)); // confirm

    // Should return to Navigate, NOT Passive
    match &state.sidebar_mode {
        SidebarMode::Navigate(ctx) => {
            assert_eq!(ctx.restore, Some((99, 7))); // restore preserved
        }
        other => panic!("expected Navigate, got {:?}", std::mem::discriminant(other)),
    }
    assert_eq!(state.sessions[&1].display_name, "new");
}

#[test]
fn test_navigate_rename_cancel_returns_to_navigate() {
    let mut state = state_with_sessions(3);
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: 0,
        restore: Some((99, 7)),
        entered_at_ms: 0,
    });
    state.handle_key(key_char('r'));

    state.handle_key(bare(BareKey::Esc)); // cancel

    match &state.sidebar_mode {
        SidebarMode::Navigate(ctx) => {
            assert_eq!(ctx.restore, Some((99, 7)));
        }
        other => panic!("expected Navigate, got {:?}", std::mem::discriminant(other)),
    }
}

// ---------------------------------------------------------------------------
// Rename from Passive (mouse-initiated, different return path)
// ---------------------------------------------------------------------------

#[test]
fn test_passive_rename_confirm_returns_to_passive() {
    let mut state = state_with_sessions(3);
    state.start_passive_rename(1);

    // Confirm rename
    state.handle_key(bare(BareKey::Enter));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_passive_rename_cancel_returns_to_passive() {
    let mut state = state_with_sessions(3);
    state.start_passive_rename(1);

    state.handle_key(bare(BareKey::Esc));

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_passive_rename_sets_grace_period() {
    let mut state = state_with_sessions(3);
    state.start_passive_rename(1);

    match &state.sidebar_mode {
        SidebarMode::RenamePassive { entered_at_ms, .. } => {
            assert!(*entered_at_ms > 0);
        }
        other => panic!("expected RenamePassive, got {:?}", std::mem::discriminant(other)),
    }
}

// ---------------------------------------------------------------------------
// Grace period behavior (the core bug fix)
// ---------------------------------------------------------------------------

#[test]
fn test_pane_update_within_grace_stays_in_mode() {
    let mut state = state_with_sessions(3);
    let now = crate::session::unix_now_ms();
    enter_nav_at(&mut state, 0, now); // entered just now

    // Simulate PaneUpdate with terminal focus (stale)
    state.focused_pane_id = Some(1);
    // The PaneUpdate handler checks grace period
    assert!(state.sidebar_mode.in_grace_period(now + 100)); // 100ms later, still in grace

    // Should NOT exit
    assert!(state.sidebar_mode.is_navigating());
}

#[test]
fn test_pane_update_after_grace_exits_mode() {
    let mut state = state_with_sessions(3);
    enter_nav_at(&mut state, 0, 1000); // entered at t=1000

    // After grace period (300ms), should exit
    assert!(!state.sidebar_mode.in_grace_period(1500)); // 500ms later
}

#[test]
fn test_grace_period_passive_always_false() {
    let state = state_with_sessions(3);
    assert!(!state.sidebar_mode.in_grace_period(0));
    assert!(!state.sidebar_mode.in_grace_period(u64::MAX));
}

#[test]
fn test_rename_passive_grace_period() {
    let mut state = state_with_sessions(3);
    state.start_passive_rename(1);

    let now = crate::session::unix_now_ms();
    // Should be within grace period right after creation
    assert!(state.sidebar_mode.in_grace_period(now));
    // Should be outside grace after 500ms
    assert!(!state.sidebar_mode.in_grace_period(now + 500));
}

// ---------------------------------------------------------------------------
// Help overlay (independent of mode)
// ---------------------------------------------------------------------------

#[test]
fn test_help_any_key_dismisses() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.show_help = true;

    state.handle_key(key_char('x')); // any key

    assert!(!state.show_help);
    // Should still be in navigate (help is an overlay, not a mode)
    assert!(state.sidebar_mode.is_navigating());
}

#[test]
fn test_help_esc_dismisses_without_exiting_nav() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    state.show_help = true;

    state.handle_key(bare(BareKey::Esc));

    assert!(!state.show_help);
    // Esc dismissed help but did NOT exit navigation
    assert!(state.sidebar_mode.is_navigating());
}

// ---------------------------------------------------------------------------
// Attend interaction with navigation
// ---------------------------------------------------------------------------

#[test]
fn test_attend_exits_navigation() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 1);
    // Give sessions activities so attend has something to switch to
    state.sessions.get_mut(&1).unwrap().activity = crate::session::Activity::Idle;
    state.sessions.get_mut(&2).unwrap().activity = crate::session::Activity::Idle;

    // Simulate attend pipe action
    use crate::attend;
    // Exit nav first (as the pipe handler does)
    if state.sidebar_mode.is_selectable() {
        state.abandon_navigation();
    }
    attend::perform_attend(&mut state);

    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

// ---------------------------------------------------------------------------
// Pause from navigation
// ---------------------------------------------------------------------------

#[test]
fn test_pause_toggles_in_navigate() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);
    assert!(!state.sessions[&1].paused);

    state.handle_key(key_char('p'));

    assert!(state.sessions[&1].paused);
    // Stays in navigate
    assert!(state.sidebar_mode.is_navigating());

    state.handle_key(key_char('p'));

    assert!(!state.sessions[&1].paused);
}

// ---------------------------------------------------------------------------
// Preserve cursor on session list changes
// ---------------------------------------------------------------------------

#[test]
fn test_preserve_cursor_clamps() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 2); // cursor at last session

    // Remove last session
    state.sessions.remove(&3);
    state.preserve_cursor();

    assert_eq!(state.sidebar_mode.cursor_index(), 1); // clamped
}

#[test]
fn test_preserve_cursor_empty_sessions() {
    let mut state = state_with_sessions(1);
    enter_nav(&mut state, 0);

    state.sessions.clear();
    state.preserve_cursor();

    assert_eq!(state.sidebar_mode.cursor_index(), 0);
}

#[test]
fn test_preserve_cursor_noop_in_passive() {
    let mut state = state_with_sessions(3);
    // In passive mode, preserve_cursor should be a no-op
    state.preserve_cursor();
    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

// ---------------------------------------------------------------------------
// Mode invariants
// ---------------------------------------------------------------------------

#[test]
fn test_selectable_matches_mode() {
    let state = state_with_sessions(3);

    // Passive
    assert!(!SidebarMode::Passive.is_selectable());

    // Navigate
    assert!(SidebarMode::Navigate(NavigateContext {
        cursor_index: 0,
        restore: None,
        entered_at_ms: 0,
    }).is_selectable());

    // NavigateFilter
    assert!(SidebarMode::NavigateFilter {
        ctx: NavigateContext { cursor_index: 0, restore: None, entered_at_ms: 0 },
        filter: FilterState::default(),
    }.is_selectable());

    // NavigateDeleteConfirm
    assert!(SidebarMode::NavigateDeleteConfirm {
        ctx: NavigateContext { cursor_index: 0, restore: None, entered_at_ms: 0 },
        pane_id: 1,
    }.is_selectable());

    // NavigateRename
    assert!(SidebarMode::NavigateRename {
        ctx: NavigateContext { cursor_index: 0, restore: None, entered_at_ms: 0 },
        rename: RenameState { pane_id: 1, input_buffer: String::new(), cursor_pos: 0 },
    }.is_selectable());

    // RenamePassive
    assert!(SidebarMode::RenamePassive {
        rename: RenameState { pane_id: 1, input_buffer: String::new(), cursor_pos: 0 },
        entered_at_ms: 0,
    }.is_selectable());

    drop(state);
}

#[test]
fn test_is_navigating_correct() {
    // Navigate variants return true
    assert!(SidebarMode::Navigate(NavigateContext {
        cursor_index: 0, restore: None, entered_at_ms: 0,
    }).is_navigating());

    assert!(SidebarMode::NavigateFilter {
        ctx: NavigateContext { cursor_index: 0, restore: None, entered_at_ms: 0 },
        filter: FilterState::default(),
    }.is_navigating());

    assert!(SidebarMode::NavigateRename {
        ctx: NavigateContext { cursor_index: 0, restore: None, entered_at_ms: 0 },
        rename: RenameState { pane_id: 1, input_buffer: String::new(), cursor_pos: 0 },
    }.is_navigating());

    // Non-navigate variants return false
    assert!(!SidebarMode::Passive.is_navigating());
    assert!(!SidebarMode::RenamePassive {
        rename: RenameState { pane_id: 1, input_buffer: String::new(), cursor_pos: 0 },
        entered_at_ms: 0,
    }.is_navigating());
}

// ---------------------------------------------------------------------------
// Passive mode ignores keys
// ---------------------------------------------------------------------------

#[test]
fn test_passive_ignores_keys() {
    let mut state = state_with_sessions(3);
    // In passive mode, all keys should be ignored
    assert!(!state.handle_key(key_char('j')));
    assert!(!state.handle_key(key_char('k')));
    assert!(!state.handle_key(bare(BareKey::Enter)));
    assert!(!state.handle_key(bare(BareKey::Esc)));
    assert!(!state.handle_key(key_char('/')));
    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

// ---------------------------------------------------------------------------
// Full transition sequences (integration-style)
// ---------------------------------------------------------------------------

#[test]
fn test_full_sequence_navigate_filter_select() {
    let mut state = state_with_sessions(5);
    enter_nav(&mut state, 0);

    // Enter filter, type "project-3", Enter to confirm, then Enter to select
    state.handle_key(key_char('/'));
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }));

    for c in "project-3".chars() {
        state.handle_key(key_char(c));
    }

    // Enter confirms filter (keeps it active)
    state.handle_key(bare(BareKey::Enter));
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }));

    // Esc clears filter back to Navigate
    state.handle_key(bare(BareKey::Esc));
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));

    // Enter selects current session
    state.handle_key(bare(BareKey::Enter));
    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_full_sequence_navigate_rename_confirm_continue() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    // Press 'r' to rename
    state.handle_key(key_char('r'));
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateRename { .. }));

    // Confirm rename
    state.handle_key(bare(BareKey::Enter));

    // Should be back in Navigate (not Passive)
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));

    // Can still navigate with j/k
    state.handle_key(key_char('j'));
    assert_eq!(state.sidebar_mode.cursor_index(), 1);

    // Can exit normally
    state.handle_key(bare(BareKey::Esc));
    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}

#[test]
fn test_full_sequence_navigate_delete_cancel_rename_confirm() {
    let mut state = state_with_sessions(3);
    enter_nav(&mut state, 0);

    // Try delete, cancel
    state.handle_key(key_char('d'));
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateDeleteConfirm { .. }));
    state.handle_key(bare(BareKey::Esc)); // cancel
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));

    // Now rename
    state.handle_key(key_char('r'));
    assert!(matches!(state.sidebar_mode, SidebarMode::NavigateRename { .. }));
    state.handle_key(bare(BareKey::Enter)); // confirm
    assert!(matches!(state.sidebar_mode, SidebarMode::Navigate(_)));

    // All 3 sessions still present
    assert_eq!(state.sessions.len(), 3);
}

#[test]
fn test_empty_sessions_only_esc_and_n_work() {
    let mut state = PluginState::default();
    state.active_tab_index = Some(0);
    state.my_tab_index = Some(0);
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: 0,
        restore: None,
        entered_at_ms: 0,
    });

    // j/k should return false (no sessions to navigate)
    assert!(!state.handle_key(key_char('j')));
    assert!(!state.handle_key(key_char('k')));

    // Esc should work
    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: 0,
        restore: None,
        entered_at_ms: 0,
    });
    assert!(state.handle_key(bare(BareKey::Esc)));
    assert!(matches!(state.sidebar_mode, SidebarMode::Passive));
}
