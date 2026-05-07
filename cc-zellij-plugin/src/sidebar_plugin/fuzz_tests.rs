// Property-based fuzz tests for the sidebar mode state machine.
//
// Generates random sequences of user actions (keyboard, mouse, session
// mutations) and verifies that state invariants hold after every action.
// Uses proptest for structured generation and automatic shrinking.
//
// Historical regression seeds in `proptest-regressions/fuzz_tests.txt`
// reference the old top-level fuzz_tests module with a different FuzzAction
// shape. Those seeds are incompatible with this module's strategy tree and
// are kept as historical documentation only. New seeds will be written to
// `proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt`.

use proptest::prelude::*;
use super::input;
use super::modes::SidebarMode;
use super::state::SidebarState;
use super::test_helpers::{bare, make_payload, make_session};
use zellij_tile::prelude::*;

/// All possible user actions that can be applied to the sidebar state machine.
#[derive(Debug, Clone)]
enum FuzzAction {
    KeyJ,
    KeyK,
    KeyEnter,
    KeyEsc,
    KeyD,
    KeyR,
    KeyP,
    KeySlash,
    KeyQuestion,
    KeyM,
    KeyN,
    KeyBigR,
    KeyY,
    KeyBigY,
    KeyBackspace,
    ArbitraryChar(char),
    ToggleNavigate,
    ToggleNavigatePrev,
    LeftClick(usize),
    RightClick(usize),
    AddSession,
    RemoveSession,
}

/// Proptest strategy for generating a random `FuzzAction`.
fn arb_fuzz_action() -> impl Strategy<Value = FuzzAction> {
    prop_oneof![
        Just(FuzzAction::KeyJ),
        Just(FuzzAction::KeyK),
        Just(FuzzAction::KeyEnter),
        Just(FuzzAction::KeyEsc),
        Just(FuzzAction::KeyD),
        Just(FuzzAction::KeyR),
        Just(FuzzAction::KeyP),
        Just(FuzzAction::KeySlash),
        Just(FuzzAction::KeyQuestion),
        Just(FuzzAction::KeyM),
        Just(FuzzAction::KeyN),
        Just(FuzzAction::KeyBigR),
        Just(FuzzAction::KeyY),
        Just(FuzzAction::KeyBigY),
        Just(FuzzAction::KeyBackspace),
        any::<char>().prop_map(FuzzAction::ArbitraryChar),
        Just(FuzzAction::ToggleNavigate),
        Just(FuzzAction::ToggleNavigatePrev),
        (0..20usize).prop_map(FuzzAction::LeftClick),
        (0..20usize).prop_map(FuzzAction::RightClick),
        Just(FuzzAction::AddSession),
        Just(FuzzAction::RemoveSession),
    ]
}

/// Apply a single `FuzzAction` to the sidebar state, including any necessary
/// bookkeeping (click region rebuilds, cursor preservation after mutations).
fn apply_action(state: &mut SidebarState, action: &FuzzAction, next_pane_id: &mut u32) {
    match action {
        FuzzAction::KeyJ => { input::handle_key(state, bare(BareKey::Char('j'))); }
        FuzzAction::KeyK => { input::handle_key(state, bare(BareKey::Char('k'))); }
        FuzzAction::KeyEnter => { input::handle_key(state, bare(BareKey::Enter)); }
        FuzzAction::KeyEsc => { input::handle_key(state, bare(BareKey::Esc)); }
        FuzzAction::KeyD => { input::handle_key(state, bare(BareKey::Char('d'))); }
        FuzzAction::KeyR => { input::handle_key(state, bare(BareKey::Char('r'))); }
        FuzzAction::KeyP => { input::handle_key(state, bare(BareKey::Char('p'))); }
        FuzzAction::KeySlash => { input::handle_key(state, bare(BareKey::Char('/'))); }
        FuzzAction::KeyQuestion => { input::handle_key(state, bare(BareKey::Char('?'))); }
        FuzzAction::KeyM => { input::handle_key(state, bare(BareKey::Char('m'))); }
        FuzzAction::KeyN => { input::handle_key(state, bare(BareKey::Char('n'))); }
        FuzzAction::KeyBigR => { input::handle_key(state, bare(BareKey::Char('R'))); }
        FuzzAction::KeyY => { input::handle_key(state, bare(BareKey::Char('y'))); }
        FuzzAction::KeyBigY => { input::handle_key(state, bare(BareKey::Char('Y'))); }
        FuzzAction::KeyBackspace => { input::handle_key(state, bare(BareKey::Backspace)); }
        FuzzAction::ArbitraryChar(c) => { input::handle_key(state, bare(BareKey::Char(*c))); }
        FuzzAction::ToggleNavigate => { input::toggle_navigate(state); }
        FuzzAction::ToggleNavigatePrev => { input::toggle_navigate_prev(state); }
        FuzzAction::LeftClick(row) => {
            input::handle_mouse(state, Mouse::LeftClick(*row as isize, 0));
        }
        FuzzAction::RightClick(row) => {
            input::handle_mouse(state, Mouse::RightClick(*row as isize, 0));
        }
        FuzzAction::AddSession => {
            let id = *next_pane_id;
            *next_pane_id += 1;
            if let Some(ref mut payload) = state.cached_payload {
                let tab = payload.sessions.len();
                payload.sessions.push(make_session(id, &format!("session-{id}"), tab));
                payload.total = payload.sessions.len();
            }
            state.click_regions = build_click_regions(state);
            state.preserve_cursor();
        }
        FuzzAction::RemoveSession => {
            if let Some(ref mut payload) = state.cached_payload {
                if !payload.sessions.is_empty() {
                    payload.sessions.pop();
                    payload.total = payload.sessions.len();
                }
            }
            state.click_regions = build_click_regions(state);
            state.preserve_cursor();
        }
    }
}

/// Build click regions matching the sidebar render layout.
/// Each session occupies 3 rows starting at row 2 (after the header).
/// Row 0 gets the header sentinel (u32::MAX - 1).
fn build_click_regions(state: &SidebarState) -> Vec<(usize, u32, usize)> {
    let mut regions = vec![(0, u32::MAX - 1, 0)];
    if let Some(ref payload) = state.cached_payload {
        for (i, session) in payload.sessions.iter().enumerate() {
            regions.push((2 + i * 3, session.pane_id, session.tab_index));
        }
    }
    regions
}

/// Verify all state invariants after an action. Panics with a descriptive
/// message if any invariant is violated.
fn check_invariants(state: &SidebarState, action: &FuzzAction, step: usize) {
    let context = || format!("step={step} action={action:?} mode={:?}", state.mode);

    // INV-1: Cursor in bounds when in a navigation sub-mode.
    if let Some(ctx) = state.mode.nav_ctx() {
        let filtered_count = state.filtered_sessions().len();
        let upper = std::cmp::max(1, filtered_count);
        assert!(
            ctx.cursor_index < upper,
            "INV-1 CURSOR OUT OF BOUNDS: cursor_index={} but max valid={} (filtered_count={}) [{}]",
            ctx.cursor_index,
            upper - 1,
            filtered_count,
            context(),
        );
    }
    // Also check cursor inside Help wrapping a nav mode.
    if let SidebarMode::Help(inner) = &state.mode {
        if let Some(ctx) = inner.nav_ctx() {
            let filtered_count = state.filtered_sessions().len();
            let upper = std::cmp::max(1, filtered_count);
            assert!(
                ctx.cursor_index < upper,
                "INV-1 CURSOR OUT OF BOUNDS (Help inner): cursor_index={} max valid={} [{}]",
                ctx.cursor_index,
                upper - 1,
                context(),
            );
        }
    }

    // INV-2: Filter state consistency. NavigateFilter must have filter_state().
    if matches!(state.mode, SidebarMode::NavigateFilter { .. }) {
        assert!(
            state.mode.filter_state().is_some(),
            "INV-2 FILTER STATE MISSING: mode is NavigateFilter but filter_state() is None [{}]",
            context(),
        );
    }

    // INV-3: Passive filter clean. filter_text must be empty in Passive mode.
    if matches!(state.mode, SidebarMode::Passive) {
        assert!(
            state.filter_text.is_empty(),
            "INV-3 PASSIVE FILTER DIRTY: filter_text={:?} [{}]",
            state.filter_text,
            context(),
        );
    }

    // INV-4: Selectable matches mode. Passive is the only non-selectable mode.
    let expected_selectable = !matches!(state.mode, SidebarMode::Passive);
    assert_eq!(
        state.mode.is_selectable(),
        expected_selectable,
        "INV-4 SELECTABLE MISMATCH: is_selectable()={} expected={} [{}]",
        state.mode.is_selectable(),
        expected_selectable,
        context(),
    );

    // INV-5: Help consistency. If in Help mode, the inner mode should be valid
    // (not another Help, and toggle_help round-trip preserves state).
    if let SidebarMode::Help(inner) = &state.mode {
        assert!(
            !inner.is_help(),
            "INV-5 NESTED HELP: Help wraps another Help [{}]",
            context(),
        );
    }
}

proptest! {
    #![proptest_config(ProptestConfig {
        cases: 2000,
        .. ProptestConfig::default()
    })]

    #[test]
    fn test_sidebar_invariants(
        initial_session_count in 0..=5usize,
        actions in prop::collection::vec(arb_fuzz_action(), 1..=50),
    ) {
        // Build initial state with 0-5 sessions.
        let sessions: Vec<_> = (0..initial_session_count)
            .map(|i| {
                let id = (i + 1) as u32;
                make_session(id, &format!("session-{id}"), i)
            })
            .collect();
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(sessions));
        state.click_regions = build_click_regions(&state);

        // Next pane_id for AddSession actions (start after initial sessions).
        let mut next_pane_id = (initial_session_count + 1) as u32 + 100;

        // Apply each action and verify invariants.
        for (step, action) in actions.iter().enumerate() {
            apply_action(&mut state, action, &mut next_pane_id);
            check_invariants(&state, action, step);
        }
    }
}
