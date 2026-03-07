// cc-deck v2: Zellij plugin for Claude Code session management
//
// Two instance modes (differentiated by config):
//   - sidebar: vertical session list on every tab (via tab_template)
//   - picker:  floating fuzzy search (via LaunchOrFocusPlugin)
//
// See brainstorm/08-cc-deck-v2-redesign.md for architecture details.

use std::collections::BTreeMap;
use zellij_tile::prelude::*;

#[derive(Default)]
struct State {
    mode: PluginMode,
}

#[derive(Default, PartialEq)]
enum PluginMode {
    #[default]
    Sidebar,
    Picker,
}

register_plugin!(State);

impl ZellijPlugin for State {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        self.mode = match configuration.get("mode").map(|s| s.as_str()) {
            Some("picker") => PluginMode::Picker,
            _ => PluginMode::Sidebar,
        };

        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
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
        ]);
    }

    fn update(&mut self, _event: Event) -> bool {
        // TODO: implement event handling
        false
    }

    fn pipe(&mut self, _pipe_message: PipeMessage) -> bool {
        // TODO: implement pipe message handling (cc-deck:hook, cc-deck:sync, etc.)
        false
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self.mode {
            PluginMode::Sidebar => {
                // TODO: render vertical session list
                print!("cc-deck sidebar ({rows}x{cols})");
            }
            PluginMode::Picker => {
                // TODO: render floating picker with fuzzy search
                print!("cc-deck picker ({rows}x{cols})");
            }
        }
    }
}
