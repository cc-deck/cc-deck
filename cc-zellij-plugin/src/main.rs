#![allow(dead_code, unused_imports)]
// cc-deck v2: Zellij plugin for Claude Code session management
//
// Two instance modes (differentiated by config):
//   - sidebar: vertical session list on every tab (via tab_template)
//   - picker:  floating fuzzy search (via LaunchOrFocusPlugin)
//
// See brainstorm/08-cc-deck-v2-redesign.md for architecture details.

mod attend;
mod config;
mod git;
mod notification;
mod pipe_handler;
mod rename;
mod session;
mod sidebar;
mod state;
#[cfg(test)]
mod state_machine_tests;
#[cfg(test)]
mod fuzz_tests;
mod sync;

#[cfg(target_family = "wasm")]
fn debug_log(msg: &str) {
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open("/cache/debug.log")
    {
        use std::io::Write;
        let _ = writeln!(f, "{}", msg);
    }
}

#[cfg(not(target_family = "wasm"))]
fn debug_log(_msg: &str) {}

#[cfg(target_family = "wasm")]
fn set_selectable_wasm(selectable: bool) {
    zellij_tile::prelude::set_selectable(selectable);
}

#[cfg(not(target_family = "wasm"))]
fn set_selectable_wasm(_selectable: bool) {}

/// Shared last_click across plugin instances via /cache/last_click file.
/// All instances in the same Zellij session share the same /cache/ directory.
const SHARED_LAST_CLICK_PATH: &str = "/cache/last_click";

fn write_shared_last_click(ts: u64, pane_id: u32) {
    let data = format!("{ts},{pane_id}");
    let _ = std::fs::write(SHARED_LAST_CLICK_PATH, data);
}

fn read_shared_last_click() -> Option<(u64, u32)> {
    let data = std::fs::read_to_string(SHARED_LAST_CLICK_PATH).ok()?;
    let mut parts = data.trim().split(',');
    let ts: u64 = parts.next()?.parse().ok()?;
    let pid: u32 = parts.next()?.parse().ok()?;
    Some((ts, pid))
}

fn clear_shared_last_click() {
    let _ = std::fs::remove_file(SHARED_LAST_CLICK_PATH);
}

/// Register global keybindings via reconfigure() with MessagePluginId.
/// Routes to this specific plugin instance. Each instance re-registers
/// on load, and the active-tab instance re-registers on tab changes
/// to recover from dead plugin IDs (when a tab with the last-registered
/// plugin is closed).
#[cfg(target_family = "wasm")]
fn register_keybindings(config: &config::PluginConfig) {
    let plugin_id = zellij_tile::prelude::get_plugin_ids().plugin_id;

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
    }}
}}"#,
        nav = config.navigate_key,
        att = config.attend_key,
        id = plugin_id,
    );
    debug_log(&format!("KEYBINDS registering: navigate={} attend={} plugin_id={}",
        config.navigate_key, config.attend_key, plugin_id));
    zellij_tile::prelude::reconfigure(kdl, false);
}

#[cfg(not(target_family = "wasm"))]
fn register_keybindings(_config: &config::PluginConfig) {}

/// Broadcast an action to all plugins via pipe_message_to_plugin.
/// Only cc_deck instances handle cc-deck:* messages, so no URL filtering needed.
/// NOTE: Do NOT use plugin_url here. Zellij's pipe_to_specific_plugins matches
/// by both URL and configuration. Since we can't pass the running instances'
/// config (mode, navigate_key, etc.), the match fails and Zellij creates a
/// spurious floating pane instead of routing to existing instances.
#[cfg(target_family = "wasm")]
fn broadcast_action(name: &str) {
    let msg = MessageToPlugin::new(name);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn broadcast_action(_name: &str) {}

#[cfg(target_family = "wasm")]
fn focus_plugin() {
    let plugin_id = zellij_tile::prelude::get_plugin_ids().plugin_id;
    zellij_tile::prelude::focus_plugin_pane(plugin_id, false);
}

#[cfg(not(target_family = "wasm"))]
fn focus_plugin() {}

#[cfg(target_family = "wasm")]
fn focus_terminal(pane_id: u32) {
    zellij_tile::prelude::focus_terminal_pane(pane_id, false);
}

#[cfg(not(target_family = "wasm"))]
fn focus_terminal(_pane_id: u32) {}

#[cfg(target_family = "wasm")]
fn switch_to_tab(tab_idx: usize) {
    zellij_tile::prelude::switch_tab_to(tab_idx as u32 + 1);
}

#[cfg(not(target_family = "wasm"))]
fn switch_to_tab(_tab_idx: usize) {}

#[cfg(target_family = "wasm")]
fn create_new_session_tab() {
    zellij_tile::prelude::new_tab(None::<&str>, None::<&str>);
}

#[cfg(not(target_family = "wasm"))]
fn create_new_session_tab() {}

#[cfg(target_family = "wasm")]
fn create_new_session_pane() {
    use std::collections::BTreeMap;
    let mut context = BTreeMap::new();
    context.insert("cc-deck".to_string(), "new-session".to_string());
    zellij_tile::prelude::open_command_pane(
        zellij_tile::prelude::CommandToRun {
            path: std::path::PathBuf::from("claude"),
            args: vec![],
            cwd: None,
        },
        context,
    );
}

#[cfg(not(target_family = "wasm"))]
fn create_new_session_pane() {}

#[cfg(target_family = "wasm")]
fn close_session_pane(pane_id: u32, tab_index: Option<usize>, is_only_pane: bool) {
    zellij_tile::prelude::close_terminal_pane(pane_id);
    if is_only_pane {
        if let Some(idx) = tab_index {
            zellij_tile::prelude::close_tab_with_index(idx);
        }
    }
}

#[cfg(not(target_family = "wasm"))]
fn close_session_pane(_pane_id: u32, _tab_index: Option<usize>, _is_only_pane: bool) {}

#[cfg(target_family = "wasm")]
fn auto_rename_tab(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn auto_rename_tab(_tab_idx: usize, _name: &str) {}

use config::PluginConfig;
use git::GitResult;
use pipe_handler::{hook_event_to_activity, is_session_end, parse_pipe_message, PipeAction};
use session::Session;
use state::{NavigateContext, PluginMode, PluginState, SidebarMode};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

register_plugin!(PluginState);

// ---------------------------------------------------------------------------
// Centralized mode transition functions
// ---------------------------------------------------------------------------

/// Enter navigation mode: make sidebar selectable and focusable, initialize cursor.
fn enter_navigation_mode(state: &mut PluginState) {
    let sessions = state.sessions_by_tab_order();
    let restore = state.focused_pane_id.and_then(|pid| {
        sessions.iter()
            .find(|s| s.pane_id == pid)
            .and_then(|s| s.tab_index.map(|idx| (pid, idx)))
    });
    let cursor = state.focused_pane_id
        .and_then(|pid| sessions.iter().position(|s| s.pane_id == pid))
        .unwrap_or(0);
    let now_ms = session::unix_now_ms();

    state.sidebar_mode = SidebarMode::Navigate(NavigateContext {
        cursor_index: cursor,
        restore,
        entered_at_ms: now_ms,
    });
    set_selectable_wasm(true);
    focus_plugin();
    debug_log(&format!("NAV entered, cursor_index={cursor} restore={restore:?}"));
}

impl PluginState {
    /// Exit to passive mode, restoring the pre-navigation focus (Esc path).
    fn exit_to_passive(&mut self) {
        let old = std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive);
        set_selectable_wasm(false);

        // Restore the pane that was focused before navigation mode
        if let Some(ctx) = old.nav_ctx() {
            if let Some((pane_id, tab_idx)) = ctx.restore {
                #[cfg(target_family = "wasm")]
                {
                    zellij_tile::prelude::switch_tab_to(tab_idx as u32 + 1);
                    zellij_tile::prelude::focus_terminal_pane(pane_id, false);
                }
            }
        }
        debug_log("NAV exited (restored original pane)");
    }

    /// Exit navigation and switch to a specific session (Enter, click, NavSelect).
    fn switch_to_session(&mut self, pane_id: u32, tab_idx: Option<usize>) {
        self.sidebar_mode = SidebarMode::Passive;
        set_selectable_wasm(false);
        #[cfg(target_family = "wasm")]
        if let Some(idx) = tab_idx {
            zellij_tile::prelude::switch_tab_to(idx as u32 + 1);
            zellij_tile::prelude::focus_terminal_pane(pane_id, false);
        }
        debug_log(&format!("NAV switch to pane={pane_id} tab={tab_idx:?}"));
    }

    /// Abandon navigation without restoring focus (Attend, tab switch).
    fn abandon_navigation(&mut self) {
        self.sidebar_mode = SidebarMode::Passive;
        set_selectable_wasm(false);
        debug_log("NAV abandoned (no focus restore)");
    }

    /// Start a rename from passive mode (double-click, right-click).
    fn start_passive_rename(&mut self, pane_id: u32) -> bool {
        if let Some(session) = self.sessions.get(&pane_id) {
            let name = session.display_name.clone();
            let len = name.len();
            self.sidebar_mode = SidebarMode::RenamePassive {
                rename: state::RenameState {
                    pane_id,
                    input_buffer: name,
                    cursor_pos: len,
                },
                entered_at_ms: session::unix_now_ms(),
            };
            set_selectable_wasm(true);
            focus_plugin();
            true
        } else {
            false
        }
    }
}

// ---------------------------------------------------------------------------
// ZellijPlugin implementation
// ---------------------------------------------------------------------------

impl ZellijPlugin for PluginState {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        debug_log("LOAD start");

        self.mode = match configuration.get("mode").map(|s| s.as_str()) {
            Some("picker") => PluginMode::Picker,
            _ => PluginMode::Sidebar,
        };

        self.config = PluginConfig::from_configuration(&configuration);

        debug_log("LOAD subscribing");
        subscribe(&[
            EventType::TabUpdate,
            EventType::PaneUpdate,
            EventType::ModeUpdate,
            EventType::Timer,
            EventType::Mouse,
            EventType::Key,
            EventType::PermissionRequestResult,
            EventType::RunCommandResult,
            EventType::CommandPaneOpened,
            EventType::PaneClosed,
        ]);

        debug_log("LOAD requesting permissions");
        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
        ]);

        debug_log("LOAD setting timeout");
        set_timeout(self.config.timer_interval);

        debug_log("LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        if let Event::PermissionRequestResult(status) = event {
            debug_log(&format!("PERMISSION result={status:?}"));
            if status == PermissionStatus::Granted {
                self.permissions_granted = true;
                debug_log("PERMISSION granted, calling set_selectable(false)");
                set_selectable(false);
                register_keybindings(&self.config);
                // Restore persisted sessions (reattach recovery)
                let restored = sync::restore_sessions();
                if !restored.is_empty() {
                    self.merge_sessions(restored);
                    // Defer dead session cleanup for 3 seconds to let the
                    // pane manifest stabilize before reconciliation.
                    self.startup_grace_until =
                        Some(session::unix_now_ms() + 3000);
                }
                sync::request_state();
                sync::apply_session_meta(&mut self.sessions);
                let pending = std::mem::take(&mut self.pending_events);
                let mut render = true;
                for e in pending {
                    render |= self.handle_event(e);
                }
                return render;
            }
            return false;
        }

        if !self.permissions_granted {
            self.pending_events.push(event);
            return false;
        }

        self.handle_event(event)
    }

    fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
        // Keybinds and CLI pipes both represent direct user actions.
        // Plugin-to-plugin broadcasts (PipeSource::Plugin) are internal forwarding.
        let is_user_action = !matches!(pipe_message.source, PipeSource::Plugin(_));
        // Trace log
        debug_log(&format!("PIPE name={} payload={} source={} sessions={} pane_keys={:?}",
            pipe_message.name,
            pipe_message.payload.as_deref().unwrap_or("None"),
            if is_user_action { "keybind" } else { "plugin/cli" },
            self.sessions.len(),
            self.pane_to_tab.keys().collect::<Vec<_>>()));

        let action = parse_pipe_message(
            &pipe_message.name,
            pipe_message.payload.as_deref(),
        );

        match action {
            PipeAction::HookEvent(hook) => {
                if is_session_end(&hook.hook_event_name) {
                    let removed = self.sessions.remove(&hook.pane_id).is_some();
                    if removed {
                        sync::broadcast_state(self);
                        sync::save_sessions(&self.sessions);
                    }
                    return removed;
                }

                let activity = match hook_event_to_activity(
                    &hook.hook_event_name,
                    hook.tool_name.as_deref(),
                ) {
                    Some(a) => a,
                    None => {
                        if let Some(session) = self.sessions.get_mut(&hook.pane_id) {
                            session.last_event_ts = session::unix_now();
                        }
                        return false;
                    }
                };

                let is_new = !self.sessions.contains_key(&hook.pane_id);
                if is_new {
                    self.sessions.insert(
                        hook.pane_id,
                        Session::new(
                            hook.pane_id,
                            hook.session_id.clone().unwrap_or_default(),
                        ),
                    );
                }

                let changed = self.sessions.get_mut(&hook.pane_id).unwrap().transition(activity);

                if let Some(ref sid) = hook.session_id {
                    self.sessions.get_mut(&hook.pane_id).unwrap().session_id = sid.clone();
                }
                if let Some(ref cwd) = hook.cwd {
                    let is_worktree_cwd = cwd.contains("/.claude/");
                    let cwd_changed = self.sessions.get(&hook.pane_id)
                        .map(|s| s.working_dir.as_deref() != Some(cwd))
                        .unwrap_or(false);
                    let pane = hook.pane_id;
                    if !is_worktree_cwd && cwd_changed {
                        self.sessions.get_mut(&hook.pane_id).unwrap().working_dir = Some(cwd.clone());
                        let cwd_clone = cwd.clone();

                        let pending = self.pending_overrides.remove(cwd);
                        if let Some(ovr) = pending {
                            debug_log(&format!("RESTORE applying override for {cwd}: name={}", ovr.display_name));
                            let names: Vec<String> = self.sessions.iter()
                                .filter(|(&id, _)| id != pane)
                                .map(|(_, s)| s.display_name.clone())
                                .collect();
                            let name_refs: Vec<&str> = names.iter().map(|s| s.as_str()).collect();
                            if let Some(s) = self.sessions.get_mut(&pane) {
                                s.display_name = session::deduplicate_name(&ovr.display_name, &name_refs);
                                s.manually_renamed = true;
                                s.paused = ovr.paused;
                                let now = session::unix_now();
                                s.last_event_ts = now;
                                s.meta_ts = now;
                            }
                            if let Some(tab_idx) = self.sessions.get(&pane).and_then(|s| s.tab_index) {
                                let sessions_on_tab = self.sessions.values()
                                    .filter(|s| s.tab_index == Some(tab_idx))
                                    .count();
                                if sessions_on_tab == 1 {
                                    if let Some(s) = self.sessions.get(&pane) {
                                        auto_rename_tab(tab_idx, &s.display_name);
                                        self.updating_tabs = true;
                                    }
                                }
                            } else {
                                if let Some(s) = self.sessions.get_mut(&pane) {
                                    s.pending_tab_rename = true;
                                }
                            }
                            sync::write_session_meta(&self.sessions);
                        } else {
                            let session = self.sessions.get(&pane).unwrap();
                            let needs_dir_name = !session.manually_renamed && session.display_name.starts_with("session-");
                            let not_renamed = !session.manually_renamed;

                            if needs_dir_name {
                                let dir_name = std::path::Path::new(&cwd_clone)
                                    .file_name()
                                    .and_then(|n| n.to_str())
                                    .unwrap_or("session")
                                    .to_string();
                                let names: Vec<String> = self.sessions.iter()
                                    .filter(|(&id, _)| id != pane)
                                    .map(|(_, s)| s.display_name.clone())
                                    .collect();
                                let name_refs: Vec<&str> = names.iter().map(|s| s.as_str()).collect();
                                if let Some(s) = self.sessions.get_mut(&pane) {
                                    s.display_name = session::deduplicate_name(&dir_name, &name_refs);
                                }
                                if let Some(tab_idx) = self.sessions.get(&pane).and_then(|s| s.tab_index) {
                                    let sessions_on_tab = self.sessions.values()
                                        .filter(|s| s.tab_index == Some(tab_idx))
                                        .count();
                                    if sessions_on_tab == 1 {
                                        if let Some(s) = self.sessions.get(&pane) {
                                            auto_rename_tab(tab_idx, &s.display_name);
                                            self.updating_tabs = true;
                                        }
                                    }
                                } else {
                                    if let Some(s) = self.sessions.get_mut(&pane) {
                                        s.pending_tab_rename = true;
                                    }
                                }
                            }
                            if not_renamed {
                                git::detect_git_repo(pane, &cwd_clone);
                            }
                        }
                    }
                    if !is_worktree_cwd {
                        git::detect_git_branch(pane, cwd);
                    }
                }

                if let Some((idx, name)) = self.pane_to_tab.get(&hook.pane_id) {
                    let session = self.sessions.get_mut(&hook.pane_id).unwrap();
                    session.tab_index = Some(*idx);
                    session.tab_name = Some(name.clone());
                }

                if changed {
                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                }
                true
            }

            PipeAction::SyncState(payload) => sync::handle_sync(self, &payload),

            PipeAction::RequestState => {
                sync::broadcast_state(self);
                false
            }

            PipeAction::Attend => {
                // Exit navigation/rename if active before attending
                if self.sidebar_mode.is_selectable() {
                    self.abandon_navigation();
                }
                match attend::perform_attend(self) {
                    attend::AttendResult::Switched { .. } => {}
                    attend::AttendResult::NoneWaiting => {
                        self.notification = Some(notification::create_notification(
                            "No sessions waiting",
                            3,
                        ));
                    }
                    attend::AttendResult::AllBusy => {
                        self.notification = Some(notification::create_notification(
                            "All sessions busy",
                            3,
                        ));
                    }
                }
                true
            }

            PipeAction::Rename => {
                if let Some(rs) = rename::start_rename(self) {
                    self.sidebar_mode = SidebarMode::RenamePassive {
                        rename: rs,
                        entered_at_ms: session::unix_now_ms(),
                    };
                    set_selectable_wasm(true);
                    focus_plugin();
                    true
                } else {
                    self.notification = Some(notification::create_notification(
                        "No session to rename",
                        3,
                    ));
                    true
                }
            }

            PipeAction::NewSession => {
                match self.config.new_session_mode {
                    config::NewSessionMode::Tab => {
                        create_new_session_tab();
                    }
                    config::NewSessionMode::Pane => create_new_session_pane(),
                }
                self.notification = Some(notification::create_notification(
                    "Creating tab...",
                    2,
                ));
                true
            }

            PipeAction::Navigate => {
                if !self.is_on_active_tab() {
                    if is_user_action {
                        debug_log("NAVIGATE forwarding to active tab via broadcast");
                        broadcast_action("cc-deck:navigate");
                    }
                    return false;
                }
                if self.sidebar_mode.is_navigating() {
                    // Move cursor down with wrapping (same as j/down)
                    let count = self.filtered_sessions_by_tab_order().len();
                    if count > 0 {
                        if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                            ctx.cursor_index = (ctx.cursor_index + 1) % count;
                        }
                    }
                } else {
                    enter_navigation_mode(self);
                }
                true
            }

            PipeAction::NavToggle => {
                if !self.is_on_active_tab() {
                    if is_user_action {
                        broadcast_action("cc-deck:nav-toggle");
                    }
                    return false;
                }
                if self.sidebar_mode.is_navigating() {
                    self.exit_to_passive();
                } else {
                    enter_navigation_mode(self);
                }
                true
            }

            PipeAction::NavUp => {
                if !self.is_on_active_tab() || !self.sidebar_mode.is_navigating() {
                    return false;
                }
                let count = self.filtered_sessions_by_tab_order().len();
                if count > 0 {
                    if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                        ctx.cursor_index = if ctx.cursor_index == 0 {
                            count - 1
                        } else {
                            ctx.cursor_index - 1
                        };
                    }
                }
                true
            }

            PipeAction::NavDown => {
                if !self.is_on_active_tab() || !self.sidebar_mode.is_navigating() {
                    return false;
                }
                let count = self.filtered_sessions_by_tab_order().len();
                if count > 0 {
                    if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                        ctx.cursor_index = (ctx.cursor_index + 1) % count;
                    }
                }
                true
            }

            PipeAction::NavSelect => {
                if !self.is_on_active_tab() || !self.sidebar_mode.is_navigating() {
                    return false;
                }
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let pane_id = session.pane_id;
                    let tab_idx = session.tab_index;
                    self.switch_to_session(pane_id, tab_idx);
                    debug_log(&format!("NAV-SELECT: switched to pane={pane_id} tab={tab_idx:?}"));
                }
                true
            }

            PipeAction::Pause => {
                if !self.is_on_active_tab() || !self.sidebar_mode.is_navigating() {
                    return false;
                }
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let pane_id = session.pane_id;
                    if let Some(s) = self.sessions.get_mut(&pane_id) {
                        s.paused = !s.paused;
                        let now = session::unix_now();
                        s.last_event_ts = now;
                        s.meta_ts = now;
                    }
                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                    sync::write_session_meta(&self.sessions);
                }
                true
            }

            PipeAction::Help => {
                if !self.is_on_active_tab() {
                    return false;
                }
                self.show_help = !self.show_help;
                true
            }

            PipeAction::DumpState => {
                if !self.is_on_active_tab() {
                    return false;
                }
                let state_json = serde_json::to_string(&self.sessions)
                    .unwrap_or_else(|_| "{}".to_string());
                #[cfg(target_family = "wasm")]
                {
                    if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
                        zellij_tile::prelude::cli_pipe_output(pipe_id, &state_json);
                        zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
                    }
                }
                debug_log(&format!("DUMP-STATE responded with {} sessions", self.sessions.len()));
                false
            }

            PipeAction::RestoreMeta(payload) => {
                if let Ok(map) = serde_json::from_str::<std::collections::HashMap<String, serde_json::Value>>(&payload) {
                    for (dir, val) in map {
                        let name = val.get("display_name")
                            .and_then(|v| v.as_str())
                            .unwrap_or("")
                            .to_string();
                        let paused = val.get("paused")
                            .and_then(|v| v.as_bool())
                            .unwrap_or(false);
                        if !name.is_empty() {
                            self.pending_overrides.insert(dir, state::PendingOverride {
                                display_name: name,
                                paused,
                            });
                        }
                    }
                    debug_log(&format!("RESTORE-META loaded {} pending overrides", self.pending_overrides.len()));
                }
                false
            }

            PipeAction::Unknown => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self.mode {
            PluginMode::Sidebar => {
                self.click_regions = sidebar::render_sidebar(self, rows, cols);

                // Clear expired notifications
                if let Some(ref notif) = self.notification {
                    if notification::is_expired(notif) {
                        self.notification = None;
                    }
                }
            }
            PluginMode::Picker => {
                print!("cc-deck picker ({rows}x{cols})");
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Event handling
// ---------------------------------------------------------------------------

impl PluginState {
    fn handle_event(&mut self, event: Event) -> bool {
        match event {
            Event::TabUpdate(tabs) => {
                if self.updating_tabs {
                    self.updating_tabs = false;
                    return false;
                }

                let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);
                self.active_tab_index = new_active;
                self.tabs = tabs;
                self.rebuild_pane_map();
                self.remove_dead_sessions();
                self.preserve_cursor();

                // Exit navigation mode if user switched away from this tab
                if self.sidebar_mode.is_navigating() && !self.is_on_active_tab() {
                    self.abandon_navigation();
                    debug_log("NAV auto-exited: tab switched away");
                }

                // Re-register keybindings from the active-tab instance.
                if self.is_on_active_tab() {
                    register_keybindings(&self.config);
                }

                true
            }
            Event::PaneUpdate(manifest) => {
                self.pane_manifest = Some(manifest);
                self.rebuild_pane_map();
                if !self.in_startup_grace() {
                    self.remove_dead_sessions();
                }
                self.preserve_cursor();

                // Exit interactive modes if a terminal pane gained focus
                // (user clicked away from the sidebar).
                // Uses timestamp-based grace period instead of boolean guard:
                // if the stale PaneUpdate never arrives, the grace expires naturally.
                if self.focused_pane_id.is_some() && !matches!(self.sidebar_mode, SidebarMode::Passive) {
                    let now_ms = session::unix_now_ms();
                    if self.sidebar_mode.in_grace_period(now_ms) {
                        debug_log("PANE_UPDATE within grace period, ignoring stale focus");
                    } else {
                        debug_log(&format!("PANE_UPDATE auto-exit: focused_pane_id={:?} mode={:?}",
                            self.focused_pane_id, std::mem::discriminant(&self.sidebar_mode)));
                        self.abandon_navigation();
                    }
                }

                true
            }
            Event::ModeUpdate(mode_info) => {
                self.input_mode = mode_info.mode;
                true
            }
            Event::Timer(_) => {
                let stale = self.cleanup_stale_sessions(self.config.done_timeout);
                if stale {
                    sync::save_sessions(&self.sessions);
                }
                let meta_changed = sync::apply_session_meta(&mut self.sessions);
                // Git branch polling: only every 60s as a fallback for in-place
                // `git checkout`. The primary path is event-driven (cwd changes).
                let now_ms = session::unix_now_ms();
                if self.is_on_active_tab() && now_ms.saturating_sub(self.last_git_poll_ms) >= 60_000 {
                    self.last_git_poll_ms = now_ms;
                    for session in self.sessions.values() {
                        if session.paused {
                            continue;
                        }
                        if let Some(ref cwd) = session.working_dir {
                            if !self.pending_git_branch.contains(&session.pane_id) {
                                self.pending_git_branch.insert(session.pane_id);
                                git::detect_git_branch(session.pane_id, cwd);
                            }
                        }
                    }
                }
                set_timeout(self.config.timer_interval);
                stale || meta_changed
            }
            Event::Mouse(Mouse::LeftClick(row, col)) => {
                debug_log(&format!("CLICK row={row} col={col} regions={:?}", self.click_regions));
                if let Some((tab_idx, pane_id)) = sidebar::handle_click(row as usize, &self.click_regions) {
                    debug_log(&format!("CLICK tab_idx={tab_idx} pane_id={pane_id}"));
                    if pane_id == u32::MAX - 1 {
                        // Header clicked: toggle navigation mode
                        if self.sidebar_mode.is_navigating() {
                            self.exit_to_passive();
                        } else {
                            enter_navigation_mode(self);
                        }
                        return true;
                    } else if pane_id == u32::MAX {
                        // [+] New session button clicked
                        match self.config.new_session_mode {
                            config::NewSessionMode::Tab => {
                                debug_log(&format!("AUTO-START [+] clicked, tabs.len()={}", self.tabs.len()));
                                create_new_session_tab();
                            }
                            config::NewSessionMode::Pane => create_new_session_pane(),
                        }
                        self.notification = Some(notification::create_notification(
                            "Creating tab...",
                            2,
                        ));
                    } else {
                        // Double-click detection
                        let shared = read_shared_last_click();
                        let effective_last = shared.or(self.last_click);
                        let now_ms = session::unix_now_ms();
                        let is_double_click = effective_last
                            .map(|(ts, pid)| pid == pane_id && now_ms.saturating_sub(ts) < 800)
                            .unwrap_or(false);
                        debug_log(&format!("DBLCLICK check: now={now_ms} local={:?} shared={shared:?} pane={pane_id} is_dbl={is_double_click} mode={:?}",
                            self.last_click, std::mem::discriminant(&self.sidebar_mode)));
                        self.last_click = Some((now_ms, pane_id));
                        write_shared_last_click(now_ms, pane_id);

                        if is_double_click {
                            debug_log(&format!("DBLCLICK detected! Starting rename for pane={pane_id}"));
                            self.last_click = None;
                            clear_shared_last_click();
                            if self.start_passive_rename(pane_id) {
                                return true;
                            }
                        } else {
                            debug_log(&format!("SINGLE click: switching to pane={pane_id}"));
                            // Single click: switch tab and focus pane.
                            // Only focus if clicking a session on a different tab,
                            // otherwise skip to keep sidebar focused for double-click.
                            if self.active_tab_index != Some(tab_idx) {
                                switch_to_tab(tab_idx);
                                focus_terminal(pane_id);
                            }
                        }
                    }
                }
                false
            }
            Event::Mouse(Mouse::RightClick(row, _col)) => {
                // Right-click on a session starts rename
                if let Some((_tab_idx, pane_id)) = sidebar::handle_click(row as usize, &self.click_regions) {
                    if pane_id != u32::MAX && pane_id != u32::MAX - 1 && self.start_passive_rename(pane_id) {
                        return true;
                    }
                }
                false
            }
            Event::Mouse(mouse) => {
                debug_log(&format!("MOUSE event={mouse:?}"));
                false
            }
            Event::Key(key) => self.handle_key(key),
            Event::RunCommandResult(exit_code, stdout, _stderr, context) => {
                // Clear in-flight tracking for git branch commands
                if context.get("type").map(|t| t.as_str()) == Some("git_branch") {
                    if let Some(pane_id) = context.get("pane_id").and_then(|s| s.parse::<u32>().ok()) {
                        self.pending_git_branch.remove(&pane_id);
                    }
                }
                self.handle_git_result(exit_code, stdout, context)
            }
            Event::CommandPaneOpened(terminal_pane_id, context) => {
                if context.get("cc-deck").map(|v| v.as_str()) == Some("new-session") {
                    let session = Session::new(terminal_pane_id, String::new());
                    self.sessions.insert(terminal_pane_id, session);
                    if let Some(cwd) = std::env::current_dir().ok().and_then(|p| p.to_str().map(String::from)) {
                        git::detect_git_repo(terminal_pane_id, &cwd);
                        git::detect_git_branch(terminal_pane_id, &cwd);
                    }
                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                    true
                } else {
                    false
                }
            }
            Event::PaneClosed(pane_id) => {
                let id = match pane_id {
                    PaneId::Terminal(id) => id,
                    PaneId::Plugin(_) => return false,
                };
                let removed = self.sessions.remove(&id).is_some();
                if removed {
                    self.pending_git_branch.remove(&id);
                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                }
                removed
            }
            _ => false,
        }
    }

    /// Unified key handler dispatching based on sidebar_mode.
    fn handle_key(&mut self, key: KeyWithModifier) -> bool {
        // Help overlay takes priority over everything (any key dismisses)
        if self.show_help {
            self.show_help = false;
            return true;
        }

        match self.sidebar_mode {
            // Rename (from any parent mode)
            SidebarMode::NavigateRename { .. } | SidebarMode::RenamePassive { .. } => {
                self.handle_rename_key(key)
            }
            // Delete confirmation (sub-mode of navigate)
            SidebarMode::NavigateDeleteConfirm { .. } => {
                self.handle_delete_confirm_key(key)
            }
            // Filter/search (sub-mode of navigate)
            SidebarMode::NavigateFilter { .. } => {
                self.handle_filter_key(key)
            }
            // Normal navigation
            SidebarMode::Navigate(_) => {
                self.handle_navigation_key(key)
            }
            // Passive: no keyboard input
            SidebarMode::Passive => false,
        }
    }

    /// Handle a key during rename (works for both NavigateRename and RenamePassive).
    fn handle_rename_key(&mut self, key: KeyWithModifier) -> bool {
        // Get mutable rename state
        let rs = match self.sidebar_mode.rename_state_mut() {
            Some(rs) => rs,
            None => return false,
        };

        match rename::handle_key(rs, key) {
            rename::RenameAction::Continue => true,
            rename::RenameAction::Confirm(new_name) => {
                let pane_id = self.sidebar_mode.rename_state().unwrap().pane_id;
                // Determine return mode based on current variant
                match std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
                    SidebarMode::NavigateRename { ctx, .. } => {
                        // Return to navigation mode
                        self.sidebar_mode = SidebarMode::Navigate(ctx);
                    }
                    SidebarMode::RenamePassive { .. } => {
                        // Return to passive
                        set_selectable_wasm(false);
                        focus_terminal(pane_id);
                    }
                    _ => {}
                }
                rename::complete_rename(self, pane_id, new_name);
                true
            }
            rename::RenameAction::Cancel => {
                let pane_id = self.sidebar_mode.rename_state().unwrap().pane_id;
                match std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
                    SidebarMode::NavigateRename { ctx, .. } => {
                        self.sidebar_mode = SidebarMode::Navigate(ctx);
                    }
                    SidebarMode::RenamePassive { .. } => {
                        set_selectable_wasm(false);
                        focus_terminal(pane_id);
                    }
                    _ => {}
                }
                true
            }
        }
    }

    /// Handle a key during delete confirmation: y confirms, anything else cancels.
    fn handle_delete_confirm_key(&mut self, key: KeyWithModifier) -> bool {
        let (ctx, pane_id) = match std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
            SidebarMode::NavigateDeleteConfirm { ctx, pane_id } => (ctx, pane_id),
            other => {
                self.sidebar_mode = other;
                return false;
            }
        };

        // Return to navigate regardless of confirm/cancel
        self.sidebar_mode = SidebarMode::Navigate(ctx);

        if key.bare_key == BareKey::Char('y') {
            let session_info = self.sessions.get(&pane_id).map(|s| {
                let tab_idx = s.tab_index;
                let is_only = tab_idx.map(|idx| {
                    self.sessions.values()
                        .filter(|s2| s2.tab_index == Some(idx))
                        .count() <= 1
                }).unwrap_or(false);
                (tab_idx, is_only)
            });

            self.sessions.remove(&pane_id);
            if let Some((tab_idx, is_only)) = session_info {
                close_session_pane(pane_id, tab_idx, is_only);
            }
            sync::broadcast_state(self);
            sync::save_sessions(&self.sessions);
            self.preserve_cursor();
        }
        true
    }

    /// Handle a key in filter/search mode.
    fn handle_filter_key(&mut self, key: KeyWithModifier) -> bool {
        // Extract filter + ctx, work on them, then put back
        let (mut ctx, mut filter) = match std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
            SidebarMode::NavigateFilter { ctx, filter } => (ctx, filter),
            other => {
                self.sidebar_mode = other;
                return false;
            }
        };

        match key.bare_key {
            BareKey::Char(c) => {
                filter.input_buffer.insert(filter.cursor_pos, c);
                filter.cursor_pos += 1;
                ctx.cursor_index = 0;
                self.sidebar_mode = SidebarMode::NavigateFilter { ctx, filter };
                true
            }
            BareKey::Backspace => {
                if filter.cursor_pos > 0 {
                    filter.cursor_pos -= 1;
                    filter.input_buffer.remove(filter.cursor_pos);
                    ctx.cursor_index = 0;
                }
                self.sidebar_mode = SidebarMode::NavigateFilter { ctx, filter };
                true
            }
            BareKey::Enter => {
                let matches = self.filtered_session_count(&filter.input_buffer);
                if matches == 0 && !filter.input_buffer.is_empty() {
                    self.notification = Some(notification::create_notification("No matches", 2));
                    ctx.cursor_index = 0;
                    self.sidebar_mode = SidebarMode::Navigate(ctx);
                } else {
                    ctx.cursor_index = 0;
                    self.sidebar_mode = SidebarMode::NavigateFilter { ctx, filter };
                }
                true
            }
            BareKey::Esc => {
                ctx.cursor_index = 0;
                self.sidebar_mode = SidebarMode::Navigate(ctx);
                true
            }
            _ => {
                self.sidebar_mode = SidebarMode::NavigateFilter { ctx, filter };
                false
            }
        }
    }

    /// Handle a key event in navigation mode.
    fn handle_navigation_key(&mut self, key: KeyWithModifier) -> bool {
        let session_count = self.filtered_sessions_by_tab_order().len();
        if session_count == 0 {
            match key.bare_key {
                BareKey::Esc => {
                    self.exit_to_passive();
                    return true;
                }
                BareKey::Char('n') => {
                    create_new_session_tab();
                    self.exit_to_passive();
                    return true;
                }
                _ => return false,
            }
        }

        match key.bare_key {
            // Cursor movement
            BareKey::Char('j') | BareKey::Down => {
                if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                    ctx.cursor_index = (ctx.cursor_index + 1) % session_count;
                }
                true
            }
            BareKey::Char('k') | BareKey::Up => {
                if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                    ctx.cursor_index = if ctx.cursor_index == 0 {
                        session_count - 1
                    } else {
                        ctx.cursor_index - 1
                    };
                }
                true
            }
            BareKey::Char('g') | BareKey::Home => {
                if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                    ctx.cursor_index = 0;
                }
                true
            }
            BareKey::Char('G') | BareKey::End => {
                if let Some(ctx) = self.sidebar_mode.nav_ctx_mut() {
                    ctx.cursor_index = session_count.saturating_sub(1);
                }
                true
            }

            // Switch to cursor session
            BareKey::Enter => {
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let pane_id = session.pane_id;
                    let tab_idx = session.tab_index;
                    self.switch_to_session(pane_id, tab_idx);
                }
                true
            }

            // Exit navigation mode
            BareKey::Esc => {
                self.exit_to_passive();
                true
            }

            // Search/filter mode: transition Navigate -> NavigateFilter
            BareKey::Char('/') => {
                if let SidebarMode::Navigate(ctx) = std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
                    self.sidebar_mode = SidebarMode::NavigateFilter {
                        ctx,
                        filter: state::FilterState::default(),
                    };
                }
                true
            }

            // Rename cursor session: transition Navigate -> NavigateRename
            BareKey::Char('r') => {
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let pane_id = session.pane_id;
                    let name = session.display_name.clone();
                    let len = name.len();
                    if let SidebarMode::Navigate(ctx) = std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
                        self.sidebar_mode = SidebarMode::NavigateRename {
                            ctx,
                            rename: state::RenameState {
                                pane_id,
                                input_buffer: name,
                                cursor_pos: len,
                            },
                        };
                    }
                }
                true
            }

            // Delete cursor session: transition Navigate -> NavigateDeleteConfirm
            BareKey::Char('d') => {
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let target_pane = session.pane_id;
                    if let SidebarMode::Navigate(ctx) = std::mem::replace(&mut self.sidebar_mode, SidebarMode::Passive) {
                        self.sidebar_mode = SidebarMode::NavigateDeleteConfirm {
                            ctx,
                            pane_id: target_pane,
                        };
                    }
                }
                true
            }

            // Show help overlay
            BareKey::Char('?') => {
                self.show_help = true;
                true
            }

            // Toggle pause on cursor session
            BareKey::Char('p') => {
                let cursor = self.sidebar_mode.cursor_index();
                let sessions = self.filtered_sessions_by_tab_order();
                if let Some(session) = sessions.get(cursor) {
                    let pane_id = session.pane_id;
                    if let Some(s) = self.sessions.get_mut(&pane_id) {
                        s.paused = !s.paused;
                        let now = session::unix_now();
                        s.last_event_ts = now;
                        s.meta_ts = now;
                    }
                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                    sync::write_session_meta(&self.sessions);
                }
                true
            }

            // New tab
            BareKey::Char('n') => {
                create_new_session_tab();
                self.exit_to_passive();
                true
            }

            _ => false,
        }
    }

    /// Check if this plugin instance is on the currently active tab.
    fn is_on_active_tab(&self) -> bool {
        match (self.my_tab_index, self.active_tab_index) {
            (Some(my), Some(active)) => my == active,
            _ => true, // if unknown, assume active (safe default)
        }
    }

    fn handle_git_result(
        &mut self,
        exit_code: Option<i32>,
        stdout: Vec<u8>,
        context: BTreeMap<String, String>,
    ) -> bool {
        match git::parse_git_result(exit_code, stdout, context) {
            GitResult::RepoDetected { pane_id, repo_path } => {
                let should_rename = self
                    .sessions
                    .get(&pane_id)
                    .map(|s| !s.manually_renamed)
                    .unwrap_or(false);

                if should_rename {
                    let repo_name = git::repo_name_from_path(&repo_path).to_string();
                    let names = self.session_names_except(pane_id);
                    let new_name = session::deduplicate_name(&repo_name, &names);

                    if let Some(session) = self.sessions.get_mut(&pane_id) {
                        session.display_name = new_name.clone();
                    }

                    if let Some(tab_idx) = self.sessions.get(&pane_id).and_then(|s| s.tab_index) {
                        let sessions_on_tab = self.sessions.values()
                            .filter(|s| s.tab_index == Some(tab_idx))
                            .count();
                        if sessions_on_tab == 1 {
                            auto_rename_tab(tab_idx, &new_name);
                            self.updating_tabs = true;
                        }
                    }

                    sync::broadcast_state(self);
                    sync::save_sessions(&self.sessions);
                    true
                } else {
                    false
                }
            }
            GitResult::BranchDetected { pane_id, branch } => {
                if let Some(session) = self.sessions.get_mut(&pane_id) {
                    session.git_branch = Some(branch);
                    true
                } else {
                    false
                }
            }
            GitResult::NotGit => false,
        }
    }

    fn session_names_except(&self, exclude_pane_id: u32) -> Vec<&str> {
        self.sessions
            .iter()
            .filter(|(&id, _)| id != exclude_pane_id)
            .map(|(_, s)| s.display_name.as_str())
            .collect()
    }
}
