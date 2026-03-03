mod config;
mod keybindings;
mod pipe_handler;
mod session;
mod state;

use std::collections::BTreeMap;

use zellij_tile::prelude::*;

use config::PluginConfig;
use keybindings::register_keybindings;
use pipe_handler::{parse_pipe_message, PipeEventType};
use session::SessionStatus;
use state::PluginState;

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
                // Track which pane is currently focused
                for (_tab_idx, panes) in &manifest.panes {
                    for pane in panes {
                        if pane.is_focused {
                            self.focused_pane_id = Some(pane.id);
                        }
                    }
                }
                true // re-render to update focused highlight
            }
            Event::CommandPaneOpened(_pane_id, _context) => {
                // Will be handled in Phase 3 (session creation)
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
            Event::Key(_key) => {
                // Key events handled when picker is active (Phase 3)
                false
            }
            Event::PaneClosed(_pane_id) => {
                // Will clean up sessions in Phase 3
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

        // Status bar rendering (single row mode)
        if rows == 1 {
            self.render_status_bar(cols);
        }
    }

    fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
        let message_name = if pipe_message.name.is_empty() {
            pipe_message.payload.as_deref().unwrap_or("")
        } else {
            &pipe_message.name
        };

        // Handle keybinding messages from reconfigure
        match message_name {
            "open_picker" => {
                self.picker_active = !self.picker_active;
                if !self.picker_active {
                    self.picker_query.clear();
                    self.picker_selected = 0;
                }
                return true;
            }
            "new_session" => {
                // Will trigger session creation in Phase 3
                return false;
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

impl PluginState {
    /// Render a compact status bar showing all sessions.
    fn render_status_bar(&self, cols: usize) {
        if self.sessions.is_empty() {
            let msg = " cc-deck: no sessions ";
            let padded = format!("{:<width$}", msg, width = cols);
            // Dim style for empty state
            print!("\u{1b}[2m{}\u{1b}[0m", padded);
            return;
        }

        let mut bar = String::new();
        for session in self.sessions.values() {
            let is_focused = self
                .focused_pane_id
                .is_some_and(|id| id == session.pane_id);

            let indicator = session.status.indicator();
            let name = &session.display_name;
            let tab = format!(" {} {} ", indicator, name);

            if is_focused {
                // Bold + reverse for focused session
                bar.push_str(&format!("\u{1b}[1;7m{}\u{1b}[0m", tab));
            } else {
                bar.push_str(&tab);
            }
            bar.push('|');
        }

        // Remove trailing separator and pad
        if bar.ends_with('|') {
            bar.pop();
        }

        // Truncate if wider than available columns
        let display_len: usize = bar
            .chars()
            .filter(|c| !matches!(c, '\u{1b}'))
            .count();
        if display_len > cols {
            // Simple truncation with overflow indicator
            bar.truncate(cols.saturating_sub(1));
            bar.push('…');
        }

        print!("{}", bar);
    }
}
