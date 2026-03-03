mod config;
#[cfg(target_arch = "wasm32")]
mod keybindings;
mod picker;
mod pipe_handler;
mod session;
mod state;
mod status_bar;

#[cfg(target_arch = "wasm32")]
mod plugin_impl {
    use std::collections::BTreeMap;
    use std::path::PathBuf;

    use zellij_tile::prelude::*;

    use crate::config::PluginConfig;
    use crate::keybindings::register_keybindings;
    use crate::pipe_handler::{parse_pipe_message, PipeEventType};
    use crate::session::SessionStatus;
    use crate::state::PluginState;

    register_plugin!(PluginState);

    impl ZellijPlugin for PluginState {
        fn load(&mut self, configuration: BTreeMap<String, String>) {
            // Parse plugin configuration from KDL
            self.config = PluginConfig::from_kdl(&configuration);
            self.idle_timeout_secs = self.config.idle_timeout;

            // Request required permissions
            request_permission(&[
                PermissionType::ReadApplicationState,
                PermissionType::ChangeApplicationState,
                PermissionType::OpenTerminalsOrPlugins,
                PermissionType::RunCommands,
                PermissionType::Reconfigure,
                PermissionType::ReadCliPipes,
            ]);

            // Subscribe to events we need
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

            // Store our plugin ID and register keybindings
            let ids = get_plugin_ids();
            self.plugin_id = ids.plugin_id;
            register_keybindings(self.plugin_id, &self.config);
        }

        fn update(&mut self, event: Event) -> bool {
            match event {
                Event::PaneUpdate(manifest) => {
                    // Track which pane is currently focused and update MRU
                    for panes in manifest.panes.values() {
                        for pane in panes {
                            if pane.is_focused && !pane.is_plugin {
                                let prev_focused = self.focused_pane_id;
                                self.focused_pane_id = Some(pane.id);

                                // Update MRU timestamp when focus changes to a tracked session
                                if prev_focused != Some(pane.id) {
                                    self.touch_session_mru(pane.id);
                                }
                            }
                        }
                    }
                    true // re-render to update focused highlight
                }
                Event::CommandPaneOpened(pane_id, context) => {
                    // Register the new session now that Zellij has assigned it a pane ID
                    self.register_session(pane_id, context);
                    true
                }
                Event::CommandPaneExited(pane_id, exit_code, _context) => {
                    if let Some(session) = self.session_by_pane_id_mut(pane_id) {
                        let code = exit_code.unwrap_or(-1);
                        session.transition_status(SessionStatus::Exited(code));
                    }
                    true
                }
                Event::Timer(_elapsed) => {
                    // Will be used for idle detection in Phase 5
                    false
                }
                Event::Key(key) => {
                    if self.picker_active {
                        match self.handle_picker_key(key) {
                            Ok(Some(pane_id)) => {
                                // Session selected: switch focus to that terminal pane
                                focus_terminal_pane(pane_id, true);
                                self.touch_session_mru(pane_id);
                                true
                            }
                            Ok(None) => true, // Key handled, re-render picker
                            Err(()) => {
                                // Picker dismissed: focus back to last terminal pane
                                if let Some(prev_pane) = self.focused_pane_id {
                                    focus_terminal_pane(prev_pane, true);
                                }
                                true
                            }
                        }
                    } else {
                        false
                    }
                }
                Event::PaneClosed(pane_id) => {
                    // Extract the terminal pane ID if applicable
                    let terminal_id = match pane_id {
                        PaneId::Terminal(id) => Some(id),
                        PaneId::Plugin(_) => None,
                    };
                    if let Some(tid) = terminal_id {
                        self.remove_session_by_pane(tid);
                    }
                    true
                }
                Event::RunCommandResult(_exit_code, _stdout, _stderr, _context) => {
                    // Will handle git detection results in Phase 4
                    false
                }
                Event::PermissionRequestResult(status) => {
                    if status == PermissionStatus::Granted {
                        // Permissions granted, plugin is ready
                    }
                    false
                }
                _ => false,
            }
        }

        fn render(&mut self, rows: usize, cols: usize) {
            if rows == 0 || cols == 0 {
                return;
            }

            if self.picker_active && rows > 1 {
                // Render the fuzzy picker overlay
                self.render_picker(rows, cols);
            } else {
                // Status bar rendering
                self.render_status_bar(cols);
            }
        }

        fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
            let message_name = if pipe_message.name.is_empty() {
                pipe_message.payload.as_deref().unwrap_or("")
            } else {
                &pipe_message.name
            };

            // Handle direct session switching via Ctrl+Shift+1-9
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

            // Handle keybinding messages from reconfigure
            match message_name {
                "open_picker" => {
                    self.picker_active = !self.picker_active;
                    if self.picker_active {
                        // Focus the plugin pane so it receives key events
                        focus_plugin_pane(self.plugin_id, true);
                    } else {
                        self.picker_query.clear();
                        self.picker_selected = 0;
                        // Return focus to the previously focused terminal pane
                        if let Some(prev_pane) = self.focused_pane_id {
                            focus_terminal_pane(prev_pane, true);
                        }
                    }
                    return true;
                }
                "new_session" => {
                    // Create a new Claude Code session.
                    // If a payload path is provided, use it; otherwise use "." (plugin CWD).
                    let cwd = pipe_message
                        .payload
                        .as_ref()
                        .filter(|p| !p.is_empty())
                        .map(PathBuf::from)
                        .unwrap_or_else(|| PathBuf::from("."));
                    let session_id = self.prepare_session(cwd.clone());
                    let cmd = CommandToRun {
                        path: PathBuf::from("claude"),
                        args: vec![],
                        cwd: Some(cwd),
                    };
                    let context = BTreeMap::from([
                        ("session_id".to_string(), session_id.to_string()),
                    ]);
                    open_command_pane(cmd, context);
                    return true;
                }
                "rename_session" => {
                    // Will trigger rename flow in Phase 4
                    return false;
                }
                "close_session" => {
                    // Will trigger session close in Phase 8
                    return false;
                }
                _ => {}
            }

            // Handle Claude Code hook messages (cc-deck::EVENT_TYPE::PANE_ID)
            if let Some(event) = parse_pipe_message(message_name) {
                if let Some(session) = self.session_by_pane_id_mut(event.pane_id) {
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
}
