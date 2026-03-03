mod config;
mod keybindings;
mod pipe_handler;
mod session;
mod state;

use std::collections::BTreeMap;

use zellij_tile::prelude::*;

use state::PluginState;

register_plugin!(PluginState);

impl ZellijPlugin for PluginState {
    fn load(&mut self, _configuration: BTreeMap<String, String>) {}

    fn update(&mut self, _event: Event) -> bool {
        false
    }

    fn render(&mut self, _rows: usize, _cols: usize) {}

    fn pipe(&mut self, _pipe_message: PipeMessage) -> bool {
        false
    }
}
