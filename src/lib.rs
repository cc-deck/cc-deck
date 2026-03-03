mod config;
mod git;
mod group;
#[cfg(target_arch = "wasm32")]
mod keybindings;
mod picker;
mod pipe_handler;
mod recent;
mod session;
mod state;
mod status_bar;

#[cfg(target_arch = "wasm32")]
mod plugin_impl {
    use std::collections::BTreeMap;
    use std::path::PathBuf;

    use zellij_tile::prelude::*;

    use crate::config::PluginConfig;
    use crate::git;
    use crate::keybindings::register_keybindings;
    use crate::pipe_handler::{parse_pipe_message, PipeEventType};
    use crate::recent::RecentEntries;
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

            // Load recent session entries from cache
            self.recent = RecentEntries::load();

            // Start periodic timer for idle detection (check every 60 seconds)
            set_timeout(60.0);
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

                            // Fallback activity detection: when hooks are not active,
                            // use title changes as a basic signal that the session is active
                            if !pane.is_plugin {
                                if let Some(session) = self.session_by_pane_id_mut(pane.id) {
                                    if !session.hooks_active {
                                        let current_title = pane.title.clone();
                                        let title_changed = session
                                            .last_title
                                            .as_ref()
                                            .is_none_or(|prev| *prev != current_title);
                                        if title_changed {
                                            // Title changed: reset idle timer as a basic activity signal
                                            session.idle_elapsed_secs = 0;
                                            session.last_title = Some(current_title);
                                        }
                                    }
                                }
                            }
                        }
                    }
                    true // re-render to update focused highlight
                }
                Event::CommandPaneOpened(pane_id, context) => {
                    // Register the new session now that Zellij has assigned it a pane ID
                    let session_id = self.register_session(pane_id, context);
                    // Trigger git repo detection for auto-naming
                    if let Some(id) = session_id {
                        git::detect_git_repo(id);

                        // Add to recent entries and persist
                        if let Some(session) = self.sessions.get(&id) {
                            let max_recent = self.config.max_recent;
                            self.recent.add(
                                session.working_dir.clone(),
                                &session.display_name,
                                &Self::iso_timestamp(),
                                max_recent,
                            );
                            let _ = self.recent.save();
                        }
                    }
                    true
                }
                Event::CommandPaneExited(pane_id, exit_code, _context) => {
                    let code = exit_code.unwrap_or(-1);
                    if let Some(session) = self.session_by_pane_id_mut(pane_id) {
                        // If the pane exits while status is still Unknown and exit
                        // code is non-zero, the command likely was not found.
                        if matches!(session.status, SessionStatus::Unknown) && code != 0 {
                            let name = session.display_name.clone();
                            self.error_message = Some(
                                format!("Error: 'claude' not found for session '{}'", name),
                            );
                            // Clear after 5 timer ticks (roughly 5 minutes at 60s interval,
                            // but effectively ~5 renders since the bar re-renders on events)
                            self.error_clear_counter = 5;
                        }
                        if let Some(session) = self.session_by_pane_id_mut(pane_id) {
                            session.transition_status(SessionStatus::Exited(code));
                        }
                    }
                    true
                }
                Event::Timer(_elapsed) => {
                    let idle_timeout = self.idle_timeout_secs;
                    let mut needs_render = false;

                    // Increment idle elapsed time for sessions in Done or Unknown state
                    for session in self.sessions.values_mut() {
                        if matches!(session.status, SessionStatus::Exited(_)) {
                            continue;
                        }
                        match session.status {
                            SessionStatus::Done | SessionStatus::Unknown => {
                                session.idle_elapsed_secs += 60;
                                if session.idle_elapsed_secs >= idle_timeout {
                                    session.transition_status(SessionStatus::Idle(
                                        std::time::Duration::from_secs(session.idle_elapsed_secs),
                                    ));
                                    needs_render = true;
                                }
                            }
                            _ => {}
                        }
                    }

                    // Decrement picker toggle cooldown
                    if self.picker_toggle_cooldown > 0 {
                        self.picker_toggle_cooldown -= 1;
                    }

                    // Decrement error clear counter
                    if self.error_clear_counter > 0 {
                        self.error_clear_counter -= 1;
                        if self.error_clear_counter == 0 {
                            self.error_message = None;
                            needs_render = true;
                        }
                    }

                    // Re-arm the timer
                    set_timeout(60.0);
                    needs_render
                }
                Event::Key(key) => {
                    if self.close_confirm_active {
                        self.handle_close_confirm_key(key);
                        true
                    } else if self.rename_active {
                        self.handle_rename_key(key);
                        true
                    } else if self.picker_active {
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
                Event::RunCommandResult(exit_code, stdout, _stderr, context) => {
                    if context.get("type").map(|t| t.as_str()) == Some("git_detect") {
                        if exit_code == Some(0) {
                            if let Some(repo_name) = git::repo_name_from_stdout(&stdout) {
                                if let Some(session_id) = context
                                    .get("session_id")
                                    .and_then(|s| s.parse::<u32>().ok())
                                {
                                    let unique_name =
                                        self.unique_display_name(&repo_name, Some(session_id));
                                    if let Some(session) = self.sessions.get_mut(&session_id) {
                                        session.set_auto_name(unique_name);
                                    }
                                    // Update project group for the new name
                                    if let Some(session) = self.sessions.get(&session_id) {
                                        let group_id = session.group_id.clone();
                                        let display = session.display_name.clone();
                                        self.get_or_create_group(&group_id, &display);
                                    }
                                }
                            }
                        }
                        // Non-zero exit = not a git repo; directory basename is already set
                        true
                    } else {
                        false
                    }
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

            if self.close_confirm_active {
                self.render_close_confirm(cols);
            } else if self.rename_active {
                self.render_rename_prompt(cols);
            } else if self.picker_active && rows > 1 {
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
                    // Debounce: ignore rapid toggles
                    if self.picker_toggle_cooldown > 0 {
                        return false;
                    }

                    // Cancel other modal states before toggling picker
                    if self.close_confirm_active {
                        self.close_confirm_active = false;
                        self.close_target_pane_id = None;
                    }
                    if self.rename_active {
                        self.rename_active = false;
                        self.rename_buffer.clear();
                    }

                    self.picker_active = !self.picker_active;
                    self.picker_toggle_cooldown = 3;
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
                    // If a payload path is provided, use it. Otherwise, fall back
                    // to the most recently used directory (from recent entries),
                    // or "." (plugin CWD) if no recent entries exist.
                    let cwd = pipe_message
                        .payload
                        .as_ref()
                        .filter(|p| !p.is_empty())
                        .map(PathBuf::from)
                        .or_else(|| {
                            self.recent
                                .all()
                                .first()
                                .map(|entry| entry.directory.clone())
                        })
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
                    if self.focused_pane_id.is_some()
                        && self.session_by_pane_id(self.focused_pane_id.unwrap()).is_some()
                    {
                        self.rename_active = true;
                        self.rename_buffer.clear();
                        // Focus the plugin pane so it receives key events
                        focus_plugin_pane(self.plugin_id, true);
                    }
                    return true;
                }
                "close_session" => {
                    if let Some(pane_id) = self.focused_pane_id {
                        if self.session_by_pane_id(pane_id).is_some() {
                            self.close_confirm_active = true;
                            self.close_target_pane_id = Some(pane_id);
                            // Focus plugin to receive key events for confirmation
                            focus_plugin_pane(self.plugin_id, true);
                        }
                    }
                    return true;
                }
                _ => {}
            }

            // Handle Claude Code hook messages (cc-deck::EVENT_TYPE::PANE_ID)
            if let Some(event) = parse_pipe_message(message_name) {
                if let Some(session) = self.session_by_pane_id_mut(event.pane_id) {
                    // Mark that this session has hooks configured
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
}
