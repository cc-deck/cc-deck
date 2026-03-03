// WASM entry point for the Zellij plugin.
// This binary target produces _start for WASI, which Zellij 0.43+ requires.
// The lib target (lib.rs) is used for native cargo test.

// On non-WASM targets (native), this binary is a no-op.
#[cfg(not(target_arch = "wasm32"))]
fn main() {}

#[cfg(target_arch = "wasm32")]
mod config;
#[cfg(target_arch = "wasm32")]
mod git;
#[cfg(target_arch = "wasm32")]
mod group;
#[cfg(target_arch = "wasm32")]
mod keybindings;
#[cfg(target_arch = "wasm32")]
mod picker;
#[cfg(target_arch = "wasm32")]
mod pipe_handler;
#[cfg(target_arch = "wasm32")]
mod recent;
#[cfg(target_arch = "wasm32")]
mod session;
#[cfg(target_arch = "wasm32")]
mod state;
#[cfg(target_arch = "wasm32")]
mod status_bar;

#[cfg(target_arch = "wasm32")]
use std::collections::BTreeMap;
#[cfg(target_arch = "wasm32")]
use std::path::PathBuf;
#[cfg(target_arch = "wasm32")]
use std::time::Duration;
#[cfg(target_arch = "wasm32")]
use zellij_tile::prelude::*;
#[cfg(target_arch = "wasm32")]
use config::PluginConfig;
#[cfg(target_arch = "wasm32")]
use keybindings::register_keybindings;
#[cfg(target_arch = "wasm32")]
use pipe_handler::{parse_pipe_message, PipeEventType};
#[cfg(target_arch = "wasm32")]
use recent::RecentEntries;
#[cfg(target_arch = "wasm32")]
use session::SessionStatus;
#[cfg(target_arch = "wasm32")]
use state::PluginState;

#[cfg(target_arch = "wasm32")]
register_plugin!(PluginState);

#[cfg(target_arch = "wasm32")]

    impl ZellijPlugin for PluginState {
        fn load(&mut self, configuration: BTreeMap<String, String>) {
            self.config = PluginConfig::from_kdl(&configuration);
            self.idle_timeout_secs = self.config.idle_timeout;

            request_permission(&[
                PermissionType::ReadApplicationState,
                PermissionType::ChangeApplicationState,
                PermissionType::OpenTerminalsOrPlugins,
                PermissionType::RunCommands,
                PermissionType::Reconfigure,
                PermissionType::ReadCliPipes,
            ]);

            subscribe(&[
                EventType::TabUpdate,
                EventType::PaneUpdate,
                EventType::Timer,
                EventType::Key,
                EventType::Mouse,
                EventType::ModeUpdate,
                EventType::CommandPaneOpened,
                EventType::CommandPaneExited,
                EventType::RunCommandResult,
                EventType::PaneClosed,
                EventType::PermissionRequestResult,
            ]);

            let ids = get_plugin_ids();
            self.plugin_id = ids.plugin_id;
            register_keybindings(self.plugin_id, &self.config);

            self.recent = RecentEntries::load();
            set_timeout(60.0);
        }

        fn update(&mut self, event: Event) -> bool {
            match event {
                Event::PaneUpdate(manifest) => {
                    for panes in manifest.panes.values() {
                        for pane in panes {
                            if pane.is_focused && !pane.is_plugin {
                                let prev_focused = self.focused_pane_id;
                                self.focused_pane_id = Some(pane.id);
                                if prev_focused != Some(pane.id) {
                                    self.touch_session_mru(pane.id);
                                }
                            }
                        }
                    }
                    true
                }
                Event::CommandPaneOpened(pane_id, context) => {
                    let session_id = self.register_session(pane_id, context);
                    if let Some(sid) = session_id {
                        git::detect_git_repo(sid);
                    }
                    true
                }
                Event::CommandPaneExited(pane_id, exit_code, _context) => {
                    let is_early_exit = self
                        .session_by_pane_id(pane_id)
                        .map(|s| matches!(s.status, SessionStatus::Unknown))
                        .unwrap_or(false);
                    if let Some(session) = self.session_by_pane_id_mut(pane_id) {
                        let code = exit_code.unwrap_or(-1);
                        session.transition_status(SessionStatus::Exited(code));
                    }
                    if is_early_exit {
                        self.error_message =
                            Some("Failed to start claude (command not found?)".to_string());
                        self.error_clear_counter = 5;
                    }
                    true
                }
                Event::Timer(_elapsed) => {
                    for session in self.sessions.values_mut() {
                        if matches!(session.status, SessionStatus::Done | SessionStatus::Unknown) {
                            session.idle_elapsed_secs += 60;
                            if session.idle_elapsed_secs >= self.idle_timeout_secs {
                                let duration = Duration::from_secs(session.idle_elapsed_secs);
                                session.transition_status(SessionStatus::Idle(duration));
                            }
                        }
                    }
                    if self.error_clear_counter > 0 {
                        self.error_clear_counter -= 1;
                        if self.error_clear_counter == 0 {
                            self.error_message = None;
                        }
                    }
                    if self.picker_toggle_cooldown > 0 {
                        self.picker_toggle_cooldown -= 1;
                    }
                    set_timeout(60.0);
                    true
                }
                Event::Key(key) => {
                    if self.picker_active {
                        match self.handle_picker_key(key) {
                            Ok(Some(pane_id)) => {
                                focus_terminal_pane(pane_id, true);
                                self.touch_session_mru(pane_id);
                                true
                            }
                            Ok(None) => true,
                            Err(()) => {
                                if let Some(prev_pane) = self.focused_pane_id {
                                    focus_terminal_pane(prev_pane, true);
                                }
                                true
                            }
                        }
                    } else if self.rename_active {
                        self.handle_rename_key(key);
                        true
                    } else if self.close_confirm_active {
                        self.handle_close_confirm_key(key);
                        true
                    } else {
                        false
                    }
                }
                Event::PaneClosed(pane_id) => {
                    if let PaneId::Terminal(tid) = pane_id {
                        self.remove_session_by_pane(tid);
                    }
                    true
                }
                Event::RunCommandResult(exit_code, stdout, _stderr, context) => {
                    if context.get("type").map(|t| t.as_str()) == Some("git_detect") {
                        if let Some(session_id_str) = context.get("session_id") {
                            if let Ok(session_id) = session_id_str.parse::<u32>() {
                                if exit_code == Some(0) {
                                    if let Some(repo_name) = git::repo_name_from_stdout(&stdout) {
                                        let unique_name =
                                            self.unique_display_name(&repo_name, Some(session_id));
                                        if let Some(session) = self.sessions.get_mut(&session_id) {
                                            session.set_auto_name(unique_name);
                                        }
                                    }
                                }
                            }
                        }
                    }
                    true
                }
                Event::PermissionRequestResult(_status) => false,
                _ => false,
            }
        }

        fn render(&mut self, rows: usize, cols: usize) {
            if rows == 0 || cols == 0 {
                return;
            }
            if self.picker_active && rows > 1 {
                self.render_picker(rows, cols);
            } else if self.rename_active && rows > 1 {
                self.render_rename_prompt(cols);
            } else if self.close_confirm_active && rows > 1 {
                self.render_close_confirm(cols);
            } else {
                self.render_status_bar(cols);
            }
        }

        fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
            let message_name = if pipe_message.name.is_empty() {
                pipe_message.payload.as_deref().unwrap_or("")
            } else {
                &pipe_message.name
            };

            if let Some(idx_str) = message_name.strip_prefix("switch_session_") {
                if let Ok(idx) = idx_str.parse::<usize>() {
                    if let Some(pane_id) = self.session_pane_by_index(idx) {
                        focus_terminal_pane(pane_id, true);
                        self.touch_session_mru(pane_id);
                        return true;
                    }
                }
                return false;
            }

            match message_name {
                "open_picker" => {
                    if self.picker_toggle_cooldown > 0 {
                        return false;
                    }
                    self.picker_toggle_cooldown = 3;
                    self.close_confirm_active = false;
                    self.rename_active = false;
                    self.picker_active = !self.picker_active;
                    if self.picker_active {
                        focus_plugin_pane(self.plugin_id, true);
                    } else {
                        self.picker_query.clear();
                        self.picker_selected = 0;
                        if let Some(prev_pane) = self.focused_pane_id {
                            focus_terminal_pane(prev_pane, true);
                        }
                    }
                    return true;
                }
                "new_session" => {
                    let cwd = pipe_message
                        .payload
                        .as_ref()
                        .filter(|p| !p.is_empty())
                        .map(PathBuf::from)
                        .unwrap_or_else(|| {
                            self.recent
                                .all()
                                .first()
                                .map(|e| e.directory.clone())
                                .unwrap_or_else(|| PathBuf::from("."))
                        });
                    let session_id = self.prepare_session(cwd.clone());
                    let cmd = CommandToRun {
                        path: PathBuf::from("claude"),
                        args: vec![],
                        cwd: Some(cwd),
                    };
                    let context =
                        BTreeMap::from([("session_id".to_string(), session_id.to_string())]);
                    open_command_pane(cmd, context);
                    return true;
                }
                "rename_session" => {
                    if let Some(pane_id) = self.focused_pane_id {
                        if self.session_by_pane_id(pane_id).is_some() {
                            self.rename_active = true;
                            self.rename_buffer.clear();
                            focus_plugin_pane(self.plugin_id, true);
                        }
                    }
                    return true;
                }
                "close_session" => {
                    if let Some(pane_id) = self.focused_pane_id {
                        if self.session_by_pane_id(pane_id).is_some() {
                            self.close_confirm_active = true;
                            self.close_target_pane_id = Some(pane_id);
                            focus_plugin_pane(self.plugin_id, true);
                        }
                    }
                    return true;
                }
                _ => {}
            }

            if let Some(event) = parse_pipe_message(message_name) {
                if let Some(session) = self.session_by_pane_id_mut(event.pane_id) {
                    session.hooks_active = true;
                    let new_status = match event.event_type {
                        PipeEventType::Working => SessionStatus::Working,
                        PipeEventType::Waiting => SessionStatus::Waiting,
                        PipeEventType::Done => SessionStatus::Done,
                    };
                    session.transition_status(new_status);
                    return true;
                }
            }

            false
        }
    }

