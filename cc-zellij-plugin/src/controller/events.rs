// Controller event handlers extracted from main.rs handle_event_inner().
//
// The controller subscribes to heavyweight events only (no Mouse, Key)
// since it has no UI. It processes tab/pane updates, timer ticks,
// run_command results, and pane lifecycle events.

use super::render_broadcast;
use super::state::ControllerState;
use crate::git::{self, GitResult};
use crate::session::{self, Activity, Session};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

/// Handle TabUpdate event: track tabs, detect active tab, register keybindings,
/// clean up dead sessions.
pub fn handle_tab_update(state: &mut ControllerState, tabs: Vec<TabInfo>) {
    let old_active = state.active_tab_index;
    let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);
    state.active_tab_index = new_active;

    // Detect tab count change for sidebar reindex
    let current_tab_count = tabs.len();
    let tab_count_changed = current_tab_count != state.last_tab_count;

    state.tabs = tabs;
    let pre_focus = state.focused_pane_id;
    state.rebuild_pane_map();
    if state.focused_pane_id != pre_focus {
        crate::debug_log(&format!(
            "CTRL[{}] TAB_UPDATE: rebuild changed focus {:?} -> {:?}",
            state.plugin_id, pre_focus, state.focused_pane_id
        ));
    }

    // Register keybindings on first TabUpdate or when tabs are closed,
    // but only if this instance is the active leader.
    if state.is_leader
        && (!state.keybindings_registered || current_tab_count < state.last_tab_count)
    {
        register_keybindings(state);
        state.keybindings_registered = true;
    }
    state.last_tab_count = current_tab_count;

    // Clean up dead sessions
    let dead_removed = state.remove_dead_sessions();
    let stale_transitioned = state.cleanup_stale_sessions(state.config.done_timeout);

    // If tab count changed, notify sidebars to reindex and clear virtual sort
    if tab_count_changed {
        state.sort_active = false;
        super::sidebar_registry::handle_tab_reindex(state);
    }

    // Only mark render dirty when something actually changed
    let active_tab_changed = new_active != old_active;
    if tab_count_changed || active_tab_changed || dead_removed || stale_transitioned {
        state.mark_render_dirty();
    }
}

/// Handle PaneUpdate event: update manifest, rebuild pane map, remove dead sessions.
pub fn handle_pane_update(state: &mut ControllerState, manifest: PaneManifest) {
    let old_focused = state.focused_pane_id;
    let old_session_count = state.sessions.len();

    state.pane_manifest = Some(manifest);
    state.rebuild_pane_map();

    // Confirm restored sessions whose panes still exist in the manifest.
    // On reattach, Claude Code processes don't re-fire hooks, so the pane
    // manifest is the only way to verify they're still alive.
    if !state.unconfirmed_pane_ids.is_empty() {
        if let Some(ref manifest) = state.pane_manifest {
            let mut confirmed = Vec::new();
            for panes in manifest.panes.values() {
                for pane in panes {
                    if !pane.is_plugin
                        && !pane.exited
                        && state.unconfirmed_pane_ids.contains(&pane.id)
                    {
                        confirmed.push(pane.id);
                    }
                }
            }
            for id in &confirmed {
                state.unconfirmed_pane_ids.remove(id);
            }
            if !confirmed.is_empty() {
                crate::debug_log(&format!(
                    "CTRL PANE_UPDATE confirmed {} restored sessions from manifest",
                    confirmed.len()
                ));
            }
        }
    }

    // Remove dead sessions (unless in startup grace)
    let mut removed = false;
    if !state.in_startup_grace() {
        removed = state.remove_dead_sessions();
    }

    // Auto-discover sidebar plugin panes from manifest for reliable targeting.
    // Send an immediate render to newly discovered sidebars so they don't
    // have to wait for the next timer tick.
    let new_sidebars = super::sidebar_registry::discover_sidebars_from_manifest(state);
    if !new_sidebars.is_empty() {
        let payload = render_broadcast::build_render_payload(state);
        if let Ok(json) = serde_json::to_string(&payload) {
            for &sidebar_id in &new_sidebars {
                render_broadcast::send_render_to_plugin_pub(sidebar_id, &json);
                crate::debug_log(&format!(
                    "CTRL[{}] PUSH-ON-DISCOVERY: sent render to new sidebar={}",
                    state.plugin_id, sidebar_id
                ));
            }
        }
    }

    // Auto-unpause: when the user focuses a paused session, unpause it.
    if let Some(focused_pid) = state.focused_pane_id {
        if state.focused_pane_id != old_focused {
            if let Some(s) = state.sessions.get_mut(&focused_pid) {
                if s.paused {
                    crate::debug_log(&format!(
                        "CTRL PANE_UPDATE: auto-unpausing pane={focused_pid}"
                    ));
                    s.paused = false;
                    s.last_event_ts = session::unix_now();
                }
            }
        }
    }

    // Broadcast immediately on focus change (for responsive click/switch feedback).
    // Other changes (count, removal) use coalesced rendering via timer.
    let focus_changed = state.focused_pane_id != old_focused;
    let count_changed = state.sessions.len() != old_session_count;
    if focus_changed {
        crate::debug_log(&format!(
            "CTRL PANE_UPDATE: focus changed {:?} -> {:?}, immediate broadcast",
            old_focused, state.focused_pane_id
        ));
        super::render_broadcast::broadcast_render(state);
        state.render_dirty = false;
    } else if count_changed || removed {
        state.mark_render_dirty();
    }
}

/// Handle Timer event: flush render, clean up stale sessions, poll git branches.
pub fn handle_timer(state: &mut ControllerState, _elapsed: f64) {
    state.tick_count += 1;

    // --- Leader election protocol ---
    use super::state::{ELECTION_TIMEOUT_TICKS, LEADER_HEARTBEAT_TICKS, LEADER_FAILURE_TIMEOUT_MS};

    if !state.is_leader {
        state.election_ticks += 1;

        // Check for leader failure: if we know a leader but haven't heard
        // from it in LEADER_FAILURE_TIMEOUT_MS, clear it and start a new election.
        if let Some(_leader_id) = state.leader_plugin_id {
            let now_ms = session::unix_now_ms();
            if state.last_leader_ping_ms > 0
                && now_ms.saturating_sub(state.last_leader_ping_ms) > LEADER_FAILURE_TIMEOUT_MS
            {
                crate::debug_log("CTRL ELECTION: leader timeout, starting new election");
                state.leader_plugin_id = None;
                state.election_ticks = 0;
            }
        }

        // Election timeout: if no lower-ID controller has pinged us,
        // activate as leader.
        if state.election_ticks >= ELECTION_TIMEOUT_TICKS && state.leader_plugin_id.is_none() {
            state.is_leader = true;
            crate::debug_log(&format!(
                "CTRL ELECTION: won (activating as leader) plugin_id={}",
                state.plugin_id
            ));
            // Broadcast ping so other dormant instances discover the new
            // leader and don't also self-activate after their own timeout.
            broadcast_controller_ping(state.plugin_id);
            register_keybindings(state);
            state.keybindings_registered = true;
            render_broadcast::broadcast_render(state);
            state.render_dirty = false;
        }

        // Reschedule timer even when dormant
        set_timer(state.config.timer_interval);
        return;
    }

    // Leader heartbeat: broadcast ping periodically so dormant instances
    // know the leader is still alive.
    if state.tick_count.is_multiple_of(LEADER_HEARTBEAT_TICKS) {
        broadcast_controller_ping(state.plugin_id);
        crate::debug_log("CTRL ELECTION: leader heartbeat");
    }

    // After startup grace expires, run one deferred cleanup pass.
    if state.startup_grace_until.is_some() && !state.in_startup_grace() {
        state.startup_grace_until = None;
        // Remove restored sessions that were never confirmed by a hook event.
        if !state.unconfirmed_pane_ids.is_empty() {
            let count = state.unconfirmed_pane_ids.len();
            for pane_id in state.unconfirmed_pane_ids.drain() {
                state.sessions.remove(&pane_id);
            }
            crate::debug_log(&format!("CTRL CLEANUP removed {count} unconfirmed restored sessions"));
            state.save_sessions();
            state.mark_render_dirty();
        }
        if state.remove_dead_sessions() {
            state.save_sessions();
            state.mark_render_dirty();
        }
    }

    // Auto-restore persisted sessions if sidebar is empty (reattach recovery).
    if state.sessions.is_empty() {
        let restored = ControllerState::restore_sessions();
        if !restored.is_empty() {
            crate::debug_log(&format!(
                "CTRL TIMER: auto-restored {} sessions from disk",
                restored.len()
            ));
            state.merge_sessions(restored);
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
        state
            .perf
            .record_raw("gauge:tabs", state.tabs.len() as u64);
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
                .map(|s| !s.manually_renamed)
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

    // Use broadcast MessagePlugin (no ID) instead of MessagePluginId.
    // With duplicate controller instances (Zellij bug), MessagePluginId
    // routes to the plugin_map entry which may be the dormant instance.
    // Broadcast reaches both; the dormant guard drops it, the leader processes.
    let kdl = format!(
        r#"keybinds {{
    shared_except "locked" {{
        bind "{nav}" {{
            MessagePlugin {{
                name "cc-deck:navigate"
            }}
        }}
        bind "{att}" {{
            MessagePlugin {{
                name "cc-deck:attend"
            }}
        }}
        bind "{wrk}" {{
            MessagePlugin {{
                name "cc-deck:working"
            }}
        }}
        bind "{nav_prev}" {{
            MessagePlugin {{
                name "cc-deck:navigate-prev"
            }}
        }}
        bind "{att_prev}" {{
            MessagePlugin {{
                name "cc-deck:attend-prev"
            }}
        }}
        bind "{wrk_prev}" {{
            MessagePlugin {{
                name "cc-deck:working-prev"
            }}
        }}
        bind "{voice}" {{
            MessagePlugin {{
                name "cc-deck:voice-mute-toggle"
            }}
        }}
    }}
}}"#,
        nav = state.config.navigate_key,
        att = state.config.attend_key,
        wrk = state.config.working_key,
        nav_prev = nav_prev,
        att_prev = att_prev,
        wrk_prev = wrk_prev,
        voice = state.config.voice_key,
    );
    crate::debug_log(&format!(
        "CTRL KEYBINDS registering: navigate={} attend={} working={} (broadcast)",
        state.config.navigate_key, state.config.attend_key, state.config.working_key
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

/// Broadcast a controller ping for leader election protocol.
#[cfg(target_family = "wasm")]
pub fn broadcast_controller_ping(plugin_id: u32) {
    use zellij_tile::prelude::*;
    let mut msg = MessageToPlugin::new("cc-deck:controller-ping");
    msg.message_payload = Some(plugin_id.to_string());
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
pub fn broadcast_controller_ping(_plugin_id: u32) {}

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

    #[test]
    fn test_pane_update_confirms_restored_sessions() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.unconfirmed_pane_ids.insert(10);
        state.unconfirmed_pane_ids.insert(20);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        handle_pane_update(&mut state, make_manifest(&[10, 20]));

        assert!(state.unconfirmed_pane_ids.is_empty());
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_pane_update_does_not_confirm_absent_panes() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.unconfirmed_pane_ids.insert(10);
        state.unconfirmed_pane_ids.insert(20);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        // Only pane 10 in manifest, pane 20 is missing
        handle_pane_update(&mut state, make_manifest(&[10]));

        assert!(!state.unconfirmed_pane_ids.contains(&10));
        assert!(state.unconfirmed_pane_ids.contains(&20));
    }

    #[test]
    fn test_pane_update_does_not_confirm_exited_panes() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, Session::new(10, "s1".into()));
        state.sessions.insert(20, Session::new(20, "s2".into()));
        state.unconfirmed_pane_ids.insert(10);
        state.unconfirmed_pane_ids.insert(20);
        state.startup_grace_until = Some(session::unix_now_ms() + 3000);

        // Both panes in manifest but pane 20 has exited
        handle_pane_update(&mut state, make_manifest_with_exited(&[10, 20], &[20]));

        assert!(!state.unconfirmed_pane_ids.contains(&10));
        assert!(state.unconfirmed_pane_ids.contains(&20));
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
        state.active_tab_index = Some(0);
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
        state.active_tab_index = Some(0);
        state.last_tab_count = 2;

        let tabs = vec![make_tab_info(0, false), make_tab_info(1, true)];

        handle_tab_update(&mut state, tabs);

        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_pane_closed_terminal() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));

        handle_pane_closed(&mut state, PaneId::Terminal(42));
        assert!(!state.sessions.contains_key(&42));
        assert!(state.render_dirty);
    }

    #[test]
    fn test_handle_pane_closed_plugin_cleans_registry() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(99, 0);

        handle_pane_closed(&mut state, PaneId::Plugin(99));
        assert!(!state.sidebar_registry.contains_key(&99));
    }

    #[test]
    fn test_handle_pane_closed_unknown_noop() {
        let mut state = ControllerState::default();
        handle_pane_closed(&mut state, PaneId::Terminal(999));
        assert!(!state.render_dirty);
    }

    // --- Virtual sort auto-clear tests (T015, T016) ---

    #[test]
    fn test_tab_update_clears_sort_active_on_tab_count_change() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.is_leader = true;
        state.sort_active = true;

        // Initial state: 2 tabs
        state.last_tab_count = 2;

        // Update with 3 tabs (tab count changed)
        let tabs = vec![
            make_tab_info(0, true),
            make_tab_info(1, false),
            make_tab_info(2, false),
        ];
        handle_tab_update(&mut state, tabs);

        assert!(!state.sort_active, "sort_active should be cleared when tab count changes");
    }

    #[test]
    fn test_tab_update_preserves_sort_active_when_tab_count_unchanged() {
        let mut state = ControllerState::default();
        state.permissions_granted = true;
        state.keybindings_registered = true;
        state.is_leader = true;
        state.sort_active = true;

        // Initial state: 2 tabs
        state.last_tab_count = 2;
        state.active_tab_index = Some(0);

        // Update with same 2 tabs (tab count unchanged)
        let tabs = vec![
            make_tab_info(0, true),
            make_tab_info(1, false),
        ];
        handle_tab_update(&mut state, tabs);

        assert!(state.sort_active, "sort_active should be preserved when tab count is unchanged");
    }
}
