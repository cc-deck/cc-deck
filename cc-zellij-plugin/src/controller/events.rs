// Controller event handlers extracted from main.rs handle_event_inner().
//
// The controller subscribes to heavyweight events only (no Mouse, Key)
// since it has no UI. It processes tab/pane updates, timer ticks,
// run_command results, and pane lifecycle events.

use super::render_broadcast;
use super::state::{ControllerState, QuarantineKind};
use crate::git::{self, GitResult};
use crate::session::{self, Activity, Session};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

/// Handle TabUpdate event: track tabs, detect active tab, register keybindings,
/// clean up dead sessions.
pub fn handle_tab_update(state: &mut ControllerState, tabs: Vec<TabInfo>) {
    let old_active = state.own_active_tab();
    let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);

    // Detect tab count change for sidebar reindex
    let current_tab_count = tabs.len();
    let tab_count_changed = current_tab_count != state.last_tab_count;

    state.tabs = tabs;
    state.rebuild_pane_map();
    let client_views_changed = state.reconcile_client_views();

    // Register keybindings on first TabUpdate or when tabs are closed.
    if !state.keybindings_registered || current_tab_count < state.last_tab_count {
        register_keybindings(state);
        state.keybindings_registered = true;
    }
    state.last_tab_count = current_tab_count;

    let active_clients = state
        .client_views
        .keys()
        .copied()
        .collect::<std::collections::HashSet<_>>();
    state
        .sidebar_registry
        .retain(|_, (_, client_id)| active_clients.contains(client_id));

    // Clean up dead sessions
    let dead_removed = state.remove_dead_sessions();
    let stale_transitioned = state.cleanup_stale_sessions(state.config.done_timeout);

    // If tab count changed, notify sidebars to reindex and update virtual sort
    if tab_count_changed {
        if let Some(ref mut order) = state.sort_order {
            // Remove pane_ids for sessions that no longer exist
            order.retain(|pid| state.sessions.contains_key(pid));
            // Append any new session pane_ids in tab_index order
            let existing: std::collections::HashSet<u32> = order.iter().copied().collect();
            let mut new_sessions: Vec<(usize, u32)> = state
                .sessions
                .values()
                .filter_map(|s| s.tab_index.map(|idx| (idx, s.pane_id)))
                .filter(|(_, pid)| !existing.contains(pid))
                .collect();
            new_sessions.sort_by_key(|(idx, _)| *idx);
            order.extend(new_sessions.into_iter().map(|(_, pid)| pid));
        }
        super::sidebar_registry::handle_tab_reindex(state);
    }

    // Only mark render dirty when something actually changed
    let active_tab_changed = new_active != old_active;
    if tab_count_changed
        || active_tab_changed
        || client_views_changed
        || dead_removed
        || stale_transitioned
    {
        state.mark_render_dirty();
    }
}

/// Handle PaneUpdate event: update manifest, rebuild pane map, remove dead sessions.
pub fn handle_pane_update(state: &mut ControllerState, manifest: PaneManifest) {
    let old_session_count = state.sessions.len();

    state.pane_manifest = Some(manifest);
    state.rebuild_pane_map();
    let client_views_changed = state.reconcile_client_views();

    // Confirm quarantined sessions whose panes appear in the fresh manifest.
    // On reattach, Claude Code processes don't re-fire hooks, so the pane
    // manifest is the only way to verify they're still alive. It is also the
    // only thing that can vouch for a pane a hook event merely claimed.
    let mut graduated_hook = false;
    if !state.unconfirmed_panes.is_empty() {
        if let Some(ref manifest) = state.pane_manifest {
            let mut confirmed = Vec::new();
            for panes in manifest.panes.values() {
                for pane in panes {
                    if !pane.is_plugin
                        && !pane.exited
                        && state.unconfirmed_panes.contains_key(&pane.id)
                    {
                        confirmed.push(pane.id);
                    }
                }
            }
            for id in &confirmed {
                if let Some(q) = state.unconfirmed_panes.remove(id) {
                    // A hook session graduating goes from hidden and unpersisted
                    // to visible and persisted. The session count does not move,
                    // so the check below would otherwise miss it.
                    graduated_hook |= q.kind == QuarantineKind::Hook;
                }
            }
            if !confirmed.is_empty() {
                crate::debug_log(&format!(
                    "CTRL PANE_UPDATE confirmed {} unconfirmed sessions from manifest",
                    confirmed.len()
                ));
            }
        }
    }
    if graduated_hook {
        state.save_sessions();
    }

    // Remove dead sessions (unless in startup grace)
    let mut removed = false;
    if !state.in_startup_grace() {
        removed = state.remove_dead_sessions();
    }

    let count_changed = state.sessions.len() != old_session_count;
    if count_changed || removed || client_views_changed || graduated_hook {
        state.mark_render_dirty();
    }
}

/// Handle Timer event: flush render, clean up stale sessions, poll git branches.
pub fn handle_timer(state: &mut ControllerState, _elapsed: f64) {
    state.tick_count += 1;

    // After startup grace expires, run one deferred cleanup pass.
    // Quarantined sessions are no longer drained here: `sweep_quarantine`
    // below settles them on their own per-id deadlines, which for restored
    // sessions expire on this same tick.
    if state.startup_grace_until.is_some() && !state.in_startup_grace() {
        state.startup_grace_until = None;
        if state.remove_dead_sessions() {
            state.save_sessions();
            state.mark_render_dirty();
        }
    }

    // Settle quarantined panes whose deadline has passed. This must run before
    // the auto-restore below, so an eviction is not read straight back off disk
    // within the same tick.
    if state.sweep_quarantine() {
        state.save_sessions();
        state.mark_render_dirty();
    }

    // Auto-restore persisted sessions if sidebar is empty (reattach recovery).
    // Once only: without the guard this re-reads the cache on every tick that
    // finds no sessions, which resurrected the very sessions the sweep had just
    // evicted. Restored entries are quarantined so they must still prove
    // themselves against the manifest.
    if state.sessions.is_empty() && !state.restore_attempted {
        state.restore_attempted = true;
        if state.restore_and_quarantine() {
            state.mark_render_dirty();
        }
    }

    // Periodic stale session cleanup
    let stale = state.cleanup_stale_sessions(state.config.done_timeout);
    if stale {
        state.save_sessions();
        state.mark_render_dirty();
    }

    let now_ms = session::unix_now_ms();

    if state.client_views.values().any(|view| {
        view.pending_focus
            .is_some_and(|pending| now_ms >= pending.expires_at_ms)
    }) && state.reconcile_client_views()
    {
        state.mark_render_dirty();
    }

    // Voice heartbeat timeout: if voice is enabled but no ping for 15 seconds, clear voice state
    if state.voice_enabled
        && state.voice_last_ping_ms > 0
        && now_ms.saturating_sub(state.voice_last_ping_ms) > 15000
    {
        state.voice_enabled = false;
        state.voice_muted = false;
        state.voice_mute_requested = None;
        state.voice_mute_requested_ms = 0;
        state.mark_render_dirty();
        crate::debug_log("CTRL TIMER: voice heartbeat timeout, clearing voice state");
    }

    // Timeout for stale voice_mute_requested
    if state.voice_mute_requested.is_some()
        && state.voice_mute_requested_ms > 0
        && now_ms.saturating_sub(state.voice_mute_requested_ms) > 5000
    {
        crate::debug_log("CTRL TIMER: voice_mute_requested timeout, clearing");
        state.voice_mute_requested = None;
        state.voice_mute_requested_ms = 0;
        state.mark_render_dirty();
    }

    // Fading colors change over time for Done/Idle sessions.
    // Only re-render every 5 ticks (5s) to reduce broadcast frequency.
    // Skip sessions whose fade animation is already complete to reach
    // zero broadcasts at steady-state idle.
    let done_timeout = state.config.done_timeout;
    let idle_fade_secs = state.config.idle_fade_secs;
    if state.tick_count.is_multiple_of(5)
        && state.sessions.values().any(|s| {
            let elapsed = s.elapsed_secs();
            match s.activity {
                Activity::Done | Activity::AgentDone => elapsed < done_timeout,
                Activity::Idle => elapsed < idle_fade_secs,
                _ => false,
            }
        })
    {
        state.mark_render_dirty();
    }

    // Flush coalesced render if dirty
    render_broadcast::flush_render(state);

    // Flush buffered debug log lines
    crate::debug_flush();

    // Git branch polling and orphan cleanup: every 60s.
    let now_ms = session::unix_now_ms();
    if now_ms.saturating_sub(state.last_git_poll_ms) >= 60_000 {
        state.last_git_poll_ms = now_ms;
        // T019: Clean up orphaned state files from dead Zellij sessions
        super::state::cleanup_orphaned_state_files();
        for s in state.sessions.values() {
            if s.paused {
                continue;
            }
            if let Some(ref cwd) = s.working_dir {
                if !state.pending_git_branch.contains(&s.pane_id) {
                    state.pending_git_branch.insert(s.pane_id);
                    git::detect_git_branch(s.pane_id, cwd);
                }
            }
        }
    }

    // Perf stats
    if state.perf.enabled {
        state
            .perf
            .record_raw("gauge:sessions", state.sessions.len() as u64);
        state.perf.record_raw("gauge:tabs", state.tabs.len() as u64);
        state
            .perf
            .record_raw("gauge:sidebars", state.sidebar_registry.len() as u64);
    }
    state.perf.maybe_dump();

    // Reschedule timer
    set_timer(state.config.timer_interval);
}

/// Handle RunCommandResult: git repo/branch detection results.
pub fn handle_run_command_result(
    state: &mut ControllerState,
    exit_code: Option<i32>,
    stdout: Vec<u8>,
    _stderr: Vec<u8>,
    context: BTreeMap<String, String>,
) {
    // Track git_branch command context before parsing consumes it
    let is_branch_cmd = context.get("type").map(|t| t.as_str()) == Some("git_branch");
    let branch_pane_id = if is_branch_cmd {
        context.get("pane_id").and_then(|s| s.parse::<u32>().ok())
    } else {
        None
    };

    // Clear in-flight tracking for git branch commands
    if let Some(pane_id) = branch_pane_id {
        state.pending_git_branch.remove(&pane_id);
    }

    match git::parse_git_result(exit_code, stdout, context) {
        GitResult::RepoDetected { pane_id, repo_path } => {
            let should_rename = state
                .sessions
                .get(&pane_id)
                .map(|s| !s.manually_renamed && !s.in_worktree)
                .unwrap_or(false);

            if should_rename {
                let repo_name = git::repo_name_from_path(&repo_path).to_string();
                let names = state.session_names_except(pane_id);
                let new_name = session::deduplicate_name(&repo_name, &names);

                if let Some(s) = state.sessions.get_mut(&pane_id) {
                    s.display_name = new_name.clone();
                    s.last_event_ts = session::unix_now().max(s.last_event_ts + 1);
                }

                if let Some(tab_idx) = state.sessions.get(&pane_id).and_then(|s| s.tab_index) {
                    let sessions_on_tab = state
                        .sessions
                        .values()
                        .filter(|s| s.tab_index == Some(tab_idx))
                        .count();
                    if sessions_on_tab == 1 {
                        crate::wasm_compat::rename_tab_wasm(tab_idx, &new_name);
                    }
                }

                state.save_sessions();
                state.mark_render_dirty();
            }
        }
        GitResult::BranchDetected { pane_id, branch } => {
            if let Some(s) = state.sessions.get_mut(&pane_id) {
                let changed = s.git_branch.as_deref() != Some(&branch);
                s.git_branch = Some(branch);
                if changed {
                    state.mark_render_dirty();
                }
            }
        }
        GitResult::NotGit => {
            // If a git_branch command failed, clear the stale branch
            if let Some(pane_id) = branch_pane_id {
                if let Some(s) = state.sessions.get_mut(&pane_id) {
                    if s.git_branch.is_some() {
                        s.git_branch = None;
                        state.mark_render_dirty();
                    }
                }
            }
        }
    }
}

/// Handle CommandPaneOpened: detect new session panes created by the controller.
pub fn handle_command_pane_opened(
    state: &mut ControllerState,
    terminal_pane_id: u32,
    context: BTreeMap<String, String>,
) {
    if context.get("cc-deck").map(|v| v.as_str()) == Some("new-session") {
        let session = Session::new(terminal_pane_id, String::new());
        state.sessions.insert(terminal_pane_id, session);
        // Git detection will happen when the first hook event arrives with a CWD.
        // We cannot use std::env::current_dir() here because the controller plugin
        // process CWD is not the new pane's CWD.
        state.save_sessions();
        state.mark_render_dirty();
    }
}

/// Handle PaneClosed: remove session for the closed pane.
pub fn handle_pane_closed(state: &mut ControllerState, pane_id: PaneId) {
    let id = match pane_id {
        PaneId::Terminal(id) => id,
        PaneId::Plugin(id) => {
            // If a plugin pane closed, it might be a sidebar. Clean up the registry.
            state.sidebar_registry.remove(&id);
            return;
        }
    };
    let removed = state.sessions.remove(&id).is_some();
    if removed {
        state.pending_git_branch.remove(&id);
        if let Some(ref mut order) = state.sort_order {
            order.retain(|&p| p != id);
        }
        state.save_sessions();
        state.mark_render_dirty();
    }
}

// --- Wasm-gated host function wrappers ---

/// Register global keybindings via reconfigure() pointing to this controller plugin.
#[cfg(target_family = "wasm")]
fn register_keybindings(state: &ControllerState) {
    let nav_prev = shift_variant(&state.config.navigate_key);
    let att_prev = shift_variant(&state.config.attend_key);
    let wrk_prev = shift_variant(&state.config.working_key);

    // Target this controller by plugin id so a keypress is delivered to one
    // plugin instance rather than broadcast to every sidebar.
    let id = state.plugin_id;
    let kdl = format!(
        r#"keybinds {{
    shared_except "locked" {{
        bind "{nav}" {{
            MessagePluginId {id} {{
                name "cc-deck:navigate"
            }}
        }}
        bind "{att}" {{
            MessagePluginId {id} {{
                name "cc-deck:attend"
            }}
        }}
        bind "{wrk}" {{
            MessagePluginId {id} {{
                name "cc-deck:working"
            }}
        }}
        bind "{nav_prev}" {{
            MessagePluginId {id} {{
                name "cc-deck:navigate-prev"
            }}
        }}
        bind "{att_prev}" {{
            MessagePluginId {id} {{
                name "cc-deck:attend-prev"
            }}
        }}
        bind "{wrk_prev}" {{
            MessagePluginId {id} {{
                name "cc-deck:working-prev"
            }}
        }}
        bind "{voice}" {{
            MessagePluginId {id} {{
                name "cc-deck:voice-mute-toggle"
            }}
        }}
    }}
}}"#,
        id = id,
        nav = state.config.navigate_key,
        att = state.config.attend_key,
        wrk = state.config.working_key,
        nav_prev = nav_prev,
        att_prev = att_prev,
        wrk_prev = wrk_prev,
        voice = state.config.voice_key,
    );
    crate::debug_log(&format!(
        "CTRL KEYBINDS registering: navigate={} attend={} working={} (plugin_id={})",
        state.config.navigate_key, state.config.attend_key, state.config.working_key, id
    ));
    zellij_tile::prelude::reconfigure(kdl, false);
}

#[cfg(not(target_family = "wasm"))]
fn register_keybindings(_state: &ControllerState) {}

/// Set a timer for the next tick.
#[cfg(target_family = "wasm")]
fn set_timer(interval: f64) {
    zellij_tile::prelude::set_timeout(interval);
}

#[cfg(not(target_family = "wasm"))]
fn set_timer(_interval: f64) {}

/// Standard ANSI 16-color palette (indices 0-15).
const ANSI_16: [(u8, u8, u8); 16] = [
    (0, 0, 0),
    (128, 0, 0),
    (0, 128, 0),
    (128, 128, 0),
    (0, 0, 128),
    (128, 0, 128),
    (0, 128, 128),
    (192, 192, 192),
    (128, 128, 128),
    (255, 0, 0),
    (0, 255, 0),
    (255, 255, 0),
    (0, 0, 255),
    (255, 0, 255),
    (0, 255, 255),
    (255, 255, 255),
];

fn eightbit_to_rgb(n: u8) -> (u8, u8, u8) {
    match n {
        0..=15 => ANSI_16[n as usize],
        16..=231 => {
            let idx = n - 16;
            let r = (idx / 36) * 51;
            let g = ((idx % 36) / 6) * 51;
            let b = (idx % 6) * 51;
            (r, g, b)
        }
        232..=255 => {
            let v = 8 + (n - 232) * 10;
            (v, v, v)
        }
    }
}

/// Handle ModeUpdate event: extract multiplayer user colors from the Zellij palette.
pub fn handle_mode_update(state: &mut ControllerState, mode_info: ModeInfo) {
    let colors = &mode_info.style.colors;
    let mp = &colors.multiplayer_user_colors;
    let to_rgb = |c: &PaletteColor| -> (u8, u8, u8) {
        match c {
            PaletteColor::Rgb((r, g, b)) => (*r, *g, *b),
            PaletteColor::EightBit(n) => eightbit_to_rgb(*n),
        }
    };
    let palette: Vec<(u8, u8, u8)> = vec![
        to_rgb(&mp.player_1),
        to_rgb(&mp.player_2),
        to_rgb(&mp.player_3),
        to_rgb(&mp.player_4),
        to_rgb(&mp.player_5),
        to_rgb(&mp.player_6),
        to_rgb(&mp.player_7),
        to_rgb(&mp.player_8),
        to_rgb(&mp.player_9),
        to_rgb(&mp.player_10),
    ];
    if state.multiplayer_colors.as_ref() != Some(&palette) {
        state.multiplayer_colors = Some(palette);
        state.mark_render_dirty();
        crate::debug_log("CTRL MODE_UPDATE: multiplayer palette changed; render scheduled");
    }
}

/// Derive the Shift variant of a keybinding string by uppercasing the last character.
#[allow(dead_code)]
fn shift_variant(key: &str) -> String {
    let trimmed = key.trim_end();
    if let Some((prefix, last_char)) = trimmed.rsplit_once(' ') {
        let shifted: String = last_char
            .chars()
            .map(|c| c.to_uppercase().next().unwrap_or(c))
            .collect();
        format!("{} {}", prefix, shifted)
    } else {
        trimmed.to_uppercase()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn mode_update_marks_render_dirty_only_when_palette_changes() {
        let mut state = ControllerState::default();
        let mode_info = ModeInfo::default();

        handle_mode_update(&mut state, mode_info.clone());
        assert!(state.render_dirty);

        state.render_dirty = false;
        handle_mode_update(&mut state, mode_info.clone());
        assert!(!state.render_dirty);

        let mut changed = mode_info;
        changed.style.colors.multiplayer_user_colors.player_1 =
            PaletteColor::Rgb((12, 34, 56));
        handle_mode_update(&mut state, changed);
        assert!(state.render_dirty);
        assert_eq!(state.multiplayer_colors.as_ref().unwrap()[0], (12, 34, 56));
    }

    #[test]
    fn test_shift_variant() {
        assert_eq!(shift_variant("Alt s"), "Alt S");
        assert_eq!(shift_variant("Alt a"), "Alt A");
        assert_eq!(shift_variant("Ctrl x"), "Ctrl X");
        assert_eq!(shift_variant("Alt S"), "Alt S");
    }

    fn make_pane_info(id: u32, is_plugin: bool) -> PaneInfo {
        PaneInfo {
            id,
            is_plugin,
            is_focused: false,
            is_fullscreen: false,
            is_floating: false,
            is_suppressed: false,
            title: String::new(),
            exited: false,
            exit_status: None,
            is_held: false,
            pane_x: 0,
            pane_content_x: 0,
            pane_y: 0,
            pane_content_y: 0,
            pane_rows: 10,
            pane_content_rows: 10,
            pane_columns: 80,
            pane_content_columns: 80,
            cursor_coordinates_in_pane: None,
            terminal_command: None,
            plugin_url: None,
            is_selectable: true,
            index_in_pane_group: std::collections::BTreeMap::new(),
            default_bg: None,
            default_fg: None,
        }
    }

    fn make_manifest(terminal_pane_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_pane_ids
            .iter()
            .map(|&id| make_pane_info(id, false))
            .collect();
        let mut map = std::collections::HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    fn make_manifest_with_exited(terminal_ids: &[u32], exited_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_ids
            .iter()
            .map(|&id| {
                let mut p = make_pane_info(id, false);
                if exited_ids.contains(&id) {
                    p.exited = true;
                }
                p
            })
            .collect();
        let mut map = std::collections::HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    /// A hook session graduating goes from hidden to visible without the
    /// session count moving, so the render must be marked dirty explicitly.
    #[test]
    fn test_pane_update_graduating_hook_marks_render_dirty() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.quarantine(10, QuarantineKind::Hook);
        state.render_dirty = false;

        handle_pane_update(&mut state, make_manifest(&[10]));

        assert!(state.unconfirmed_panes.is_empty(), "pane 10 is confirmed");
        assert!(
            state.render_dirty,
            "becoming visible must trigger a re-render"
        );
    }

    #[test]
    fn test_timer_auto_restore_runs_only_once() {
        let mut state = ControllerState::default();
        assert!(!state.restore_attempted);

        handle_timer(&mut state, 1.0);

        assert!(
            state.restore_attempted,
            "the disk cache must not be re-read on every idle tick"
        );
    }

    #[test]
    fn test_pane_update_confirms_restored_sessions() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.quarantine(10, QuarantineKind::Restored);
        state.quarantine(20, QuarantineKind::Restored);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        handle_pane_update(&mut state, make_manifest(&[10, 20]));

        assert!(state.unconfirmed_panes.is_empty());
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_pane_update_does_not_confirm_absent_panes() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.quarantine(10, QuarantineKind::Restored);
        state.quarantine(20, QuarantineKind::Restored);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        // Only pane 10 in manifest, pane 20 is missing
        handle_pane_update(&mut state, make_manifest(&[10]));

        assert!(!state.unconfirmed_panes.contains_key(&10));
        assert!(state.unconfirmed_panes.contains_key(&20));
    }

    #[test]
    fn test_pane_update_does_not_confirm_exited_panes() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.quarantine(10, QuarantineKind::Restored);
        state.quarantine(20, QuarantineKind::Restored);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        // Both panes in manifest but pane 20 has exited
        handle_pane_update(&mut state, make_manifest_with_exited(&[10, 20], &[20]));

        assert!(!state.unconfirmed_panes.contains_key(&10));
        assert!(state.unconfirmed_panes.contains_key(&20));
    }

    // -----------------------------------------------------------------------
    // Conditional handle_tab_update tests (T014-T016)
    // -----------------------------------------------------------------------

    fn make_tab_info(position: usize, active: bool) -> TabInfo {
        TabInfo {
            position,
            name: format!("Tab {position}"),
            active,
            panes_to_hide: 0,
            is_fullscreen_active: false,
            is_sync_panes_active: false,
            are_floating_panes_visible: false,
            other_focused_clients: Vec::new(),
            active_swap_layout_name: None,
            is_swap_layout_dirty: false,
            viewport_rows: 24,
            viewport_columns: 80,
            display_area_rows: 24,
            display_area_columns: 80,
            selectable_tiled_panes_count: 1,
            selectable_floating_panes_count: 0,
            tab_id: position,
            has_bell_notification: false,
            is_flashing_bell: false,
        }
    }

    #[test]
    fn test_tab_update_no_changes_does_not_mark_dirty() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;

        let tabs = vec![make_tab_info(0, true)];
        state
            .client_views
            .entry(state.client_id)
            .or_default()
            .active_tab_index = Some(0);
        state.last_tab_count = 1;

        handle_tab_update(&mut state, tabs);

        assert!(!state.render_dirty);
    }

    #[test]
    fn test_tab_update_tab_count_change_marks_dirty() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.last_tab_count = 1;

        let tabs = vec![make_tab_info(0, true), make_tab_info(1, false)];

        handle_tab_update(&mut state, tabs);

        assert!(state.render_dirty);
    }

    #[test]
    fn test_tab_update_active_tab_change_marks_dirty() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state
            .client_views
            .entry(state.client_id)
            .or_default()
            .active_tab_index = Some(0);
        state.last_tab_count = 2;

        let tabs = vec![make_tab_info(0, false), make_tab_info(1, true)];

        handle_tab_update(&mut state, tabs);

        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_pane_closed_terminal() {
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "test".into()));

        handle_pane_closed(&mut state, PaneId::Terminal(42));
        assert!(!state.sessions.contains_key(&42));
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_pane_closed_plugin_cleans_registry() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(99, (0, 0));

        handle_pane_closed(&mut state, PaneId::Plugin(99));
        assert!(!state.sidebar_registry.contains_key(&99));
    }

    #[test]
    fn test_handle_pane_closed_unknown_noop() {
        let mut state = ControllerState::default();
        handle_pane_closed(&mut state, PaneId::Terminal(999));
        assert!(!state.render_dirty);
    }

    // --- Virtual sort persistence tests ---

    #[test]
    fn test_tab_update_preserves_sort_order_on_tab_added() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        let mut s1 = Session::new(1, "s1".into());
        s1.tab_index = Some(0);
        let mut s2 = Session::new(2, "s2".into());
        s2.tab_index = Some(1);
        let mut s3 = Session::new(3, "s3".into());
        s3.tab_index = Some(2);
        state.sessions.insert(1, s1);
        state.sessions.insert(2, s2);
        state.sessions.insert(3, s3);
        state.sort_order = Some(vec![2, 1]);

        state.last_tab_count = 2;

        let tabs = vec![
            make_tab_info(0, true),
            make_tab_info(1, false),
            make_tab_info(2, false),
        ];
        handle_tab_update(&mut state, tabs);

        let order = state
            .sort_order
            .expect("sort_order should be preserved when tab is added");
        assert_eq!(order[0], 2, "existing order preserved");
        assert_eq!(order[1], 1, "existing order preserved");
        assert_eq!(order[2], 3, "new session appended at end");
    }

    #[test]
    fn test_tab_update_preserves_sort_order_on_tab_closed() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        // Session 2 will be "dead" (not in sessions map after cleanup)
        state.sessions.insert(1, Session::new(1, "s1".into()));
        state.sessions.insert(3, Session::new(3, "s3".into()));
        state.sort_order = Some(vec![3, 2, 1]);

        state.last_tab_count = 3;

        let tabs = vec![make_tab_info(0, true), make_tab_info(1, false)];
        handle_tab_update(&mut state, tabs);

        let order = state
            .sort_order
            .expect("sort_order should be preserved when tab is closed");
        assert_eq!(order, vec![3, 1], "dead pane_id removed from sort order");
    }

    #[test]
    fn test_tab_update_preserves_sort_order_when_tab_count_unchanged() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.sort_order = Some(vec![1, 2]);

        state.last_tab_count = 2;
        state
            .client_views
            .entry(state.client_id)
            .or_default()
            .active_tab_index = Some(0);

        let tabs = vec![make_tab_info(0, true), make_tab_info(1, false)];
        handle_tab_update(&mut state, tabs);

        assert!(
            state.sort_order.is_some(),
            "sort_order should be preserved when tab count is unchanged"
        );
    }

    // --- Multiplayer client_focus cleanup tests ---

    #[test]
    fn test_tab_update_cleans_disconnected_client_focus() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.client_id = 1;
        state.last_tab_count = 1;

        // Two clients with focus entries
        state
            .client_views
            .insert(1, super::super::state::ClientViewState::default());
        state
            .client_views
            .insert(2, super::super::state::ClientViewState::default());

        // TabUpdate shows only client 1 (client 2 disconnected)
        let tabs = vec![make_tab_info(0, true)];
        handle_tab_update(&mut state, tabs);

        assert!(
            state.client_views.contains_key(&1),
            "controller's own client preserved"
        );
        assert!(
            !state.client_views.contains_key(&2),
            "disconnected client removed"
        );
        assert!(state.render_dirty, "render dirty after cleanup");
    }

    #[test]
    fn test_tab_update_preserves_active_client_focus() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.client_id = 1;
        state.last_tab_count = 1;

        state
            .client_views
            .insert(1, super::super::state::ClientViewState::default());
        state
            .client_views
            .insert(2, super::super::state::ClientViewState::default());

        // TabUpdate shows client 2 still connected
        let mut tab = make_tab_info(0, true);
        tab.other_focused_clients = vec![2];
        let tabs = vec![tab];
        handle_tab_update(&mut state, tabs);

        assert!(state.client_views.contains_key(&1));
        assert!(
            state.client_views.contains_key(&2),
            "connected client preserved"
        );
    }

    #[test]
    fn test_tab_update_no_cleanup_when_no_focus_entries() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.last_tab_count = 1;
        state
            .client_views
            .entry(state.client_id)
            .or_default()
            .active_tab_index = Some(0);

        let tabs = vec![make_tab_info(0, true)];
        handle_tab_update(&mut state, tabs);

        assert!(!state.render_dirty, "no render dirty when nothing to clean");
    }
}
