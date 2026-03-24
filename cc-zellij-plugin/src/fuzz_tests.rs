// Property-based fuzz tests for the sidebar state machine.
//
// Generates random sequences of user actions and verifies that invariants
// hold after every action. This catches edge cases that deterministic tests
// miss, like the original guard flag race condition.

use crate::session::Session;
use crate::state::{NavigateContext, PluginState, SidebarMode, ENTER_GRACE_MS};
use proptest::prelude::*;
use std::collections::BTreeSet;
use zellij_tile::prelude::*;

// ---------------------------------------------------------------------------
// Action enum: all possible user interactions with the sidebar
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
enum Action {
    // Keys in navigation mode
    KeyJ,
    KeyK,
    KeyG,
    KeyBigG,
    KeyEnter,
    KeyEsc,
    KeySlash,
    KeyR,
    KeyD,
    KeyP,
    KeyN,
    KeyQuestion,
    KeyY,
    KeyChar(char),
    KeyBackspace,
    KeyDown,
    KeyUp,

    // Mode transitions
    EnterNavigation,
    Attend,

    // PaneUpdate with terminal focus (simulates clicking away)
    PaneUpdateFocused { delay_ms: u64 },
    // PaneUpdate without terminal focus (plugin has focus)
    PaneUpdateUnfocused,

    // Add or remove a session
    AddSession,
    RemoveSession,
}

// ---------------------------------------------------------------------------
// proptest strategy for generating random actions
// ---------------------------------------------------------------------------

fn action_strategy() -> impl Strategy<Value = Action> {
    prop_oneof![
        // Navigation keys (high weight, most common interaction)
        3 => Just(Action::KeyJ),
        3 => Just(Action::KeyK),
        1 => Just(Action::KeyG),
        1 => Just(Action::KeyBigG),
        3 => Just(Action::KeyEnter),
        3 => Just(Action::KeyEsc),
        2 => Just(Action::KeySlash),
        2 => Just(Action::KeyR),
        2 => Just(Action::KeyD),
        2 => Just(Action::KeyP),
        1 => Just(Action::KeyN),
        1 => Just(Action::KeyQuestion),
        2 => Just(Action::KeyY),
        2 => any::<char>().prop_filter("printable", |c| c.is_ascii_graphic())
            .prop_map(Action::KeyChar),
        2 => Just(Action::KeyBackspace),
        1 => Just(Action::KeyDown),
        1 => Just(Action::KeyUp),
        // Mode transitions
        3 => Just(Action::EnterNavigation),
        2 => Just(Action::Attend),
        // PaneUpdate (simulates focus changes)
        2 => (0u64..1000).prop_map(|delay| Action::PaneUpdateFocused { delay_ms: delay }),
        1 => Just(Action::PaneUpdateUnfocused),
        // Session mutations
        1 => Just(Action::AddSession),
        1 => Just(Action::RemoveSession),
    ]
}

// ---------------------------------------------------------------------------
// Applying actions to state
// ---------------------------------------------------------------------------

fn bare(key: BareKey) -> KeyWithModifier {
    KeyWithModifier {
        bare_key: key,
        key_modifiers: BTreeSet::new(),
    }
}

fn apply_action(state: &mut PluginState, action: &Action) {
    match action {
        Action::KeyJ => { state.handle_key(bare(BareKey::Char('j'))); }
        Action::KeyK => { state.handle_key(bare(BareKey::Char('k'))); }
        Action::KeyG => { state.handle_key(bare(BareKey::Char('g'))); }
        Action::KeyBigG => { state.handle_key(bare(BareKey::Char('G'))); }
        Action::KeyEnter => { state.handle_key(bare(BareKey::Enter)); }
        Action::KeyEsc => { state.handle_key(bare(BareKey::Esc)); }
        Action::KeySlash => { state.handle_key(bare(BareKey::Char('/'))); }
        Action::KeyR => { state.handle_key(bare(BareKey::Char('r'))); }
        Action::KeyD => { state.handle_key(bare(BareKey::Char('d'))); }
        Action::KeyP => { state.handle_key(bare(BareKey::Char('p'))); }
        Action::KeyN => { state.handle_key(bare(BareKey::Char('n'))); }
        Action::KeyQuestion => { state.handle_key(bare(BareKey::Char('?'))); }
        Action::KeyY => { state.handle_key(bare(BareKey::Char('y'))); }
        Action::KeyChar(c) => { state.handle_key(bare(BareKey::Char(*c))); }
        Action::KeyBackspace => { state.handle_key(bare(BareKey::Backspace)); }
        Action::KeyDown => { state.handle_key(bare(BareKey::Down)); }
        Action::KeyUp => { state.handle_key(bare(BareKey::Up)); }

        Action::EnterNavigation => {
            if !state.sidebar_mode.is_navigating() {
                crate::enter_navigation_mode(state);
            }
        }

        Action::Attend => {
            if state.sidebar_mode.is_selectable() {
                state.abandon_navigation();
            }
            crate::attend::perform_attend(state);
        }

        Action::PaneUpdateFocused { delay_ms } => {
            // Simulate a PaneUpdate arriving `delay_ms` after the mode was entered,
            // with a terminal pane focused (user clicked away from sidebar).
            let now_ms = crate::session::unix_now_ms();
            state.focused_pane_id = state.sessions.keys().next().copied();
            if state.focused_pane_id.is_some() && !matches!(state.sidebar_mode, SidebarMode::Passive) {
                // Simulate the grace period check from the PaneUpdate handler
                let effective_now = now_ms + delay_ms;
                if state.sidebar_mode.in_grace_period(effective_now) {
                    // Within grace: stay in mode (stale PaneUpdate)
                } else {
                    // After grace: exit
                    state.abandon_navigation();
                }
            }
        }

        Action::PaneUpdateUnfocused => {
            state.focused_pane_id = None;
            // No terminal pane focused, no auto-exit
        }

        Action::AddSession => {
            let next_id = state.sessions.keys().last().map(|k| k + 1).unwrap_or(1);
            if next_id <= 20 { // cap at 20 sessions
                let mut s = Session::new(next_id, format!("session-{next_id}"));
                s.tab_index = Some((next_id - 1) as usize);
                s.display_name = format!("project-{next_id}");
                state.sessions.insert(next_id, s);
                state.preserve_cursor();
            }
        }

        Action::RemoveSession => {
            if let Some(&id) = state.sessions.keys().last() {
                state.sessions.remove(&id);
                state.preserve_cursor();
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Invariant checks
// ---------------------------------------------------------------------------

fn check_invariants(state: &PluginState, action_idx: usize, action: &Action) {
    let session_count = state.sessions.len();

    // INV 1: cursor_index is within bounds
    if let Some(ctx) = state.sidebar_mode.nav_ctx() {
        let filtered_count = state.filtered_sessions_by_tab_order().len();
        assert!(
            ctx.cursor_index <= filtered_count.max(1) - 1.min(filtered_count.max(1)),
            "INV1 violated after action #{action_idx} ({action:?}): cursor={} but filtered_count={filtered_count}",
            ctx.cursor_index,
        );
    }

    // INV 2: is_navigating implies is_selectable
    if state.sidebar_mode.is_navigating() {
        assert!(
            state.sidebar_mode.is_selectable(),
            "INV2 violated after action #{action_idx} ({action:?}): is_navigating but not is_selectable"
        );
    }

    // INV 3: Passive is not selectable
    if matches!(state.sidebar_mode, SidebarMode::Passive) {
        assert!(
            !state.sidebar_mode.is_selectable(),
            "INV3 violated after action #{action_idx} ({action:?}): Passive but is_selectable"
        );
    }

    // INV 4: NavigateFilter has a filter_state
    if matches!(state.sidebar_mode, SidebarMode::NavigateFilter { .. }) {
        assert!(
            state.sidebar_mode.filter_state().is_some(),
            "INV4 violated after action #{action_idx} ({action:?}): NavigateFilter but filter_state() is None"
        );
    }

    // INV 5: NavigateDeleteConfirm has a delete_confirm_pane
    if matches!(state.sidebar_mode, SidebarMode::NavigateDeleteConfirm { .. }) {
        assert!(
            state.sidebar_mode.delete_confirm_pane().is_some(),
            "INV5 violated after action #{action_idx} ({action:?}): NavigateDeleteConfirm but delete_confirm_pane() is None"
        );
    }

    // INV 6: NavigateRename and RenamePassive have rename_state
    if matches!(state.sidebar_mode, SidebarMode::NavigateRename { .. } | SidebarMode::RenamePassive { .. }) {
        assert!(
            state.sidebar_mode.rename_state().is_some(),
            "INV6 violated after action #{action_idx} ({action:?}): Rename mode but rename_state() is None"
        );
    }

    // INV 7: nav_ctx is available for all Navigate* variants
    if state.sidebar_mode.is_navigating() {
        assert!(
            state.sidebar_mode.nav_ctx().is_some(),
            "INV7 violated after action #{action_idx} ({action:?}): is_navigating but nav_ctx() is None"
        );
    }

    // INV 8: Passive has no nav_ctx
    if matches!(state.sidebar_mode, SidebarMode::Passive) {
        assert!(
            state.sidebar_mode.nav_ctx().is_none(),
            "INV8 violated after action #{action_idx} ({action:?}): Passive but nav_ctx() is Some"
        );
    }
}

// ---------------------------------------------------------------------------
// proptest: random action sequences
// ---------------------------------------------------------------------------

fn make_initial_state(session_count: u32) -> PluginState {
    let mut state = PluginState::default();
    for i in 1..=session_count {
        let mut s = Session::new(i, format!("session-{i}"));
        s.tab_index = Some((i - 1) as usize);
        s.display_name = format!("project-{i}");
        s.activity = crate::session::Activity::Idle;
        state.sessions.insert(i, s);
    }
    state.active_tab_index = Some(0);
    state.my_tab_index = Some(0);
    state
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(500))]

    /// Fuzz the state machine with random action sequences starting from
    /// various initial states. Verifies invariants hold after every action.
    #[test]
    fn fuzz_state_machine(
        initial_sessions in 0u32..8,
        actions in prop::collection::vec(action_strategy(), 1..50),
    ) {
        let mut state = make_initial_state(initial_sessions);

        // Verify invariants on initial state
        check_invariants(&state, 0, &Action::PaneUpdateUnfocused);

        for (i, action) in actions.iter().enumerate() {
            apply_action(&mut state, action);
            check_invariants(&state, i + 1, action);
        }
    }

    /// Fuzz specifically the navigate → sub-mode → navigate round-trip.
    /// Always starts in navigation mode to exercise sub-mode transitions.
    #[test]
    fn fuzz_navigation_submodes(
        initial_sessions in 1u32..6,
        actions in prop::collection::vec(action_strategy(), 1..30),
    ) {
        let mut state = make_initial_state(initial_sessions);
        crate::enter_navigation_mode(&mut state);
        check_invariants(&state, 0, &Action::EnterNavigation);

        for (i, action) in actions.iter().enumerate() {
            apply_action(&mut state, action);
            check_invariants(&state, i + 1, action);
        }
    }

    /// Fuzz grace period behavior: enter a mode, then send PaneUpdates
    /// at various delays to test the timestamp-based guard.
    #[test]
    fn fuzz_grace_period(
        initial_sessions in 1u32..4,
        delays in prop::collection::vec(0u64..1000, 1..20),
    ) {
        let mut state = make_initial_state(initial_sessions);
        crate::enter_navigation_mode(&mut state);

        for (i, &delay) in delays.iter().enumerate() {
            let action = Action::PaneUpdateFocused { delay_ms: delay };
            apply_action(&mut state, &action);
            check_invariants(&state, i + 1, &action);

            // If we exited to passive, re-enter nav for next iteration
            if matches!(state.sidebar_mode, SidebarMode::Passive) {
                crate::enter_navigation_mode(&mut state);
            }
        }
    }
}
