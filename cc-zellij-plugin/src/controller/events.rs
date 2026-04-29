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
            "CTRL TAB_UPDATE: rebuild changed focus {:?} -> {:?}",
            pre_focus, state.focused_pane_id
        ));
    }

    // Register keybindings on first TabUpdate or when tabs are closed
    // (the registered plugin_id may have been on a closed tab).
    if !state.keybindings_registered || current_tab_count < state.last_tab_count {
        register_keybindings(state);
        state.keybindings_registered = true;
    }
    state.last_tab_count = current_tab_count;

    // Clean up dead sessions
    state.remove_dead_sessions();
    state.cleanup_stale_sessions(state.config.done_timeout);

    // If tab count changed, notify sidebars to reindex
    if tab_count_changed {
        super::sidebar_registry::handle_tab_reindex(state);
    }

    state.mark_render_dirty();
}

/// Handle PaneUpdate event: update manifest, rebuild pane map, remove dead sessions.
pub fn handle_pane_update(state: &mut ControllerState, manifest: PaneManifest) {
    let old_focused = state.focused_pane_id;
    let old_session_count = state.sessions.len();

    state.pane_manifest = Some(manifest);
    state.rebuild_pane_map();

    // Remove dead sessions (unless in startup grace)
    let mut removed = false;
    if !state.in_startup_grace() {
        removed = state.remove_dead_sessions();
    }

    // Auto-discover sidebar plugin panes from manifest for reliable targeting
    super::sidebar_registry::discover_sidebars_from_manifest(state);

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

    // After startup grace expires, run one deferred cleanup pass.
    if state.startup_grace_until.is_some() && !state.in_startup_grace() {
        state.startup_grace_until = None;
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

    // Voice heartbeat timeout: if voice is enabled but no ping for 15 seconds, clear voice state
    if state.voice_enabled {
        let now_ms = session::unix_now_ms();
        if state.voice_last_ping_ms > 0
            && now_ms.saturating_sub(state.voice_last_ping_ms) > 15000
        {
            state.voice_enabled = false;
            state.voice_muted = false;
            state.voice_mute_requested = None;
            state.mark_render_dirty();
            crate::debug_log("CTRL TIMER: voice heartbeat timeout, clearing voice state");
        }
    }

    // Fading colors change every tick for Done/Idle sessions
    if state.sessions.values().any(|s| {
        matches!(
            s.activity,
            Activity::Done | Activity::AgentDone | Activity::Idle
        )
    }) {
        state.mark_render_dirty();
    }

    // Flush coalesced render if dirty
    render_broadcast::flush_render(state);

    // Git branch polling and orphan cleanup: every 60s.
    let now_ms = session::unix_now_ms();
    if now_ms.saturating_sub(state.last_git_poll_ms) >= 60_000 {
        state.last_git_poll_ms = now_ms;
        // T019: Clean up orphaned state files from dead Zellij sessions
        crate::sync::cleanup_orphaned_state_files();
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
                        rename_tab_wasm(tab_idx, &new_name);
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
    let plugin_id = state.plugin_id;
    let nav_prev = shift_variant(&state.config.navigate_key);
    let att_prev = shift_variant(&state.config.attend_key);
    let wrk_prev = shift_variant(&state.config.working_key);

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
        nav = state.config.navigate_key,
        att = state.config.attend_key,
        wrk = state.config.working_key,
        nav_prev = nav_prev,
        att_prev = att_prev,
        wrk_prev = wrk_prev,
        voice = state.config.voice_key,
        id = plugin_id,
    );
    crate::debug_log(&format!(
        "CTRL KEYBINDS registering: navigate={} attend={} working={} plugin_id={}",
        state.config.navigate_key, state.config.attend_key, state.config.working_key, plugin_id
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

/// Rename a Zellij tab.
#[cfg(target_family = "wasm")]
fn rename_tab_wasm(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn rename_tab_wasm(_tab_idx: usize, _name: &str) {}

/// Derive the Shift variant of a keybinding string by uppercasing the last character.
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
}
