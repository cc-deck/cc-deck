// cc-deck v2: Zellij plugin for Claude Code session management
//
// Two instance modes (differentiated by config):
//   - sidebar: vertical session list on every tab (via tab_template)
//   - picker:  floating fuzzy search (via LaunchOrFocusPlugin)
//
// See brainstorm/08-cc-deck-v2-redesign.md for architecture details.

mod config;
mod session;
mod state;

use config::PluginConfig;
use state::{PluginMode, PluginState};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

register_plugin!(PluginState);

const TIMER_INTERVAL: f64 = 1.0;

impl ZellijPlugin for PluginState {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        self.mode = match configuration.get("mode").map(|s| s.as_str()) {
            Some("picker") => PluginMode::Picker,
            _ => PluginMode::Sidebar,
        };

        self.config = PluginConfig::from_configuration(&configuration);

        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
        ]);

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

        set_timeout(TIMER_INTERVAL);
    }

    fn update(&mut self, event: Event) -> bool {
        if let Event::PermissionRequestResult(status) = event {
            if status == PermissionStatus::Granted {
                self.permissions_granted = true;
                set_selectable(false);
                // Process any events that arrived before permissions
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
        // TODO (Phase 2): implement pipe message handling
        let _ = pipe_message;
        false
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self.mode {
            PluginMode::Sidebar => {
                // TODO (Phase 3/US1): render vertical session list
                let session_count = self.sessions.len();
                if session_count == 0 {
                    print!("\x1b[2mcc-deck: no sessions\x1b[0m");
                } else {
                    let _ = (rows, cols);
                    print!("cc-deck: {session_count} sessions");
                }
            }
            PluginMode::Picker => {
                // Out of scope for 012-sidebar-plugin
                print!("cc-deck picker ({rows}x{cols})");
            }
        }
    }
}

impl PluginState {
    fn handle_event(&mut self, event: Event) -> bool {
        match event {
            Event::TabUpdate(tabs) => {
                if self.updating_tabs {
                    return false;
                }
                let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);
                self.active_tab_index = new_active;
                self.tabs = tabs;
                self.rebuild_pane_map();
                self.remove_dead_sessions();
                true
            }
            Event::PaneUpdate(manifest) => {
                self.pane_manifest = Some(manifest);
                self.rebuild_pane_map();
                self.remove_dead_sessions();
                true
            }
            Event::ModeUpdate(mode_info) => {
                self.input_mode = mode_info.mode;
                true
            }
            Event::Timer(_) => {
                let stale = self.cleanup_stale_sessions(self.config.done_timeout);
                set_timeout(self.config.timer_interval);
                stale
            }
            Event::Mouse(_mouse) => {
                // TODO (Phase 3/US1): handle click-to-switch
                false
            }
            Event::Key(_key) => {
                // TODO (Phase 7/US5): handle rename key input
                false
            }
            Event::RunCommandResult(_exit_code, _stdout, _stderr, _context) => {
                // TODO (Phase 2): handle git detection results
                false
            }
            Event::CommandPaneOpened(_pane_id, _context) => {
                // TODO (Phase 8/US6): handle new session creation
                false
            }
            Event::PaneClosed(pane_id) => {
                let id = match pane_id {
                    PaneId::Terminal(id) => id,
                    PaneId::Plugin(_) => return false,
                };
                self.sessions.remove(&id).is_some()
            }
            _ => false,
        }
    }
}
