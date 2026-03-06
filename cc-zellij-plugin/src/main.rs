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
            // Keybindings are registered after permissions are granted
            // (in the PermissionRequestResult handler below)

            self.recent = RecentEntries::load();
            set_timeout(60.0);
        }

        fn update(&mut self, event: Event) -> bool {
            match event {
                Event::PaneUpdate(manifest) => {
                    // Rebuild tab_pane_mapping from the manifest
                    self.tab_pane_mapping.clear();
                    for (tab_index, panes) in &manifest.panes {
                        let pane_ids: Vec<u32> = panes.iter().map(|p| p.id).collect();
                        self.tab_pane_mapping.insert(*tab_index, pane_ids);
                    }

                    // T018: Auto-start Claude in the new tab's terminal pane
                    if let Some((session_id, cwd)) = self.pending_auto_start.take() {
                        // Find a terminal pane that is not already tracked as a session
                        let tracked_panes: Vec<u32> =
                            self.sessions.values().map(|s| s.pane_id).collect();
                        let new_pane = manifest
                            .panes
                            .values()
                            .flat_map(|panes| panes.iter())
                            .find(|p| !p.is_plugin && !tracked_panes.contains(&p.id));
                        if let Some(pane) = new_pane {
                            let pane_id = pane.id;
                            focus_terminal_pane(pane_id, true);
                            let mut context = BTreeMap::new();
                            context
                                .insert("session_id".to_string(), session_id.to_string());
                            open_command_pane_in_place(
                                CommandToRun {
                                    path: PathBuf::from("claude"),
                                    args: vec![],
                                    cwd: Some(cwd),
                                },
                                context,
                            );
                        }
                    }

                    // T014: Detect Claude sessions from pane titles
                    let new_ids = self.detect_claude_sessions(&manifest);

                    // T016: Update tab titles for newly detected sessions
                    for sid in new_ids {
                        if let Some(session) = self.sessions.get(&sid).cloned() {
                            self.update_tab_title(&session);
                        }
                    }

                    // Existing focus tracking
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
                Event::TabUpdate(tab_info) => {
                    // Update tab_index for each tracked session by cross-referencing
                    // which tab contains the session's pane_id
                    for session in self.sessions.values_mut() {
                        let tab_index = self.tab_pane_mapping.iter().find_map(
                            |(tab_idx, pane_ids)| {
                                if pane_ids.contains(&session.pane_id) {
                                    Some(*tab_idx)
                                } else {
                                    None
                                }
                            },
                        );
                        // Verify the tab still exists in tab_info before assigning
                        if let Some(idx) = tab_index {
                            if tab_info.iter().any(|t| t.position == idx) {
                                session.tab_index = Some(idx);
                            } else {
                                session.tab_index = None;
                            }
                        } else {
                            session.tab_index = None;
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
                        // T019: Show install guidance when Claude exits immediately
                        self.error_message = Some(
                            "Claude not found. Install: npm install -g @anthropic-ai/claude-code"
                                .to_string(),
                        );
                        self.error_clear_counter = 1;
                    }
                    true
                }
                Event::Timer(_elapsed) => {
                    let mut idle_sessions: Vec<u32> = Vec::new();
                    for session in self.sessions.values_mut() {
                        if matches!(session.status, SessionStatus::Done | SessionStatus::Unknown) {
                            session.idle_elapsed_secs += 60;
                            if session.idle_elapsed_secs >= self.idle_timeout_secs {
                                let duration = Duration::from_secs(session.idle_elapsed_secs);
                                session.transition_status(SessionStatus::Idle(duration));
                                idle_sessions.push(session.id);
                            }
                        }
                    }
                    for id in idle_sessions {
                        if let Some(session) = self.sessions.get(&id) {
                            self.update_tab_title(session);
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
                                        if let Some(session) = self.sessions.get(&session_id) {
                                            self.update_tab_title(session);
                                        }
                                    }
                                }
                            }
                        }
                    }
                    true
                }
                Event::PermissionRequestResult(status) => {
                    if status == PermissionStatus::Granted {
                        register_keybindings(self.plugin_id, &self.config);
                    }
                    true
                }
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
                                .unwrap_or_else(|| {
                                    std::env::var("HOME")
                                        .map(PathBuf::from)
                                        .unwrap_or_else(|_| PathBuf::from("/tmp"))
                                })
                        });
                    let session_id = self.prepare_session(cwd.clone());
                    let tab_name = format!("cc-{}", session_id);
                    let cwd_str = cwd.to_string_lossy().to_string();
                    // Create a new tab with name and cwd
                    new_tab(Some(&tab_name), Some(&cwd_str));
                    // T017: Set pending auto-start so PaneUpdate can launch Claude
                    self.pending_auto_start = Some((session_id, cwd));
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
                let found = if let Some(session) = self.session_by_pane_id_mut(event.pane_id) {
                    session.hooks_active = true;
                    let new_status = match event.event_type {
                        PipeEventType::Working => SessionStatus::Working,
                        PipeEventType::Waiting => SessionStatus::Waiting,
                        PipeEventType::Done => SessionStatus::Done,
                    };
                    session.transition_status(new_status);
                    true
                } else {
                    false
                };
                if found {
                    if let Some(session) = self.session_by_pane_id(event.pane_id) {
                        self.update_tab_title(session);
                    }
                    return true;
                }
            }

            false
        }
    }

