mod config;
mod debug;
mod git;
mod pipe_handler;
mod session;
mod perf;

mod controller;
mod sidebar_plugin;
mod wasm_compat;

pub use debug::{debug_init, debug_log, install_panic_hook};

/// Strip ANSI escape sequences and control characters from voice text.
fn sanitize_voice_text(text: &str) -> String {
    let mut result = String::with_capacity(text.len());
    let mut in_escape = false;
    for ch in text.chars() {
        if ch == '\x1b' {
            in_escape = true;
        } else if in_escape {
            if ch.is_ascii_alphabetic() || ch == '\\' {
                in_escape = false;
            }
        } else if ch == '\x07' {
            // BEL terminates OSC sequences; strip it
        } else if ch == '\t' || ch == ' ' || (ch >= '\x20' && ch != '\x7f') {
            result.push(ch);
        }
    }
    result.retain(|c| c != '\x1b');
    result
}

use std::collections::BTreeMap;
use zellij_tile::prelude::*;

#[derive(Default)]
#[allow(clippy::large_enum_variant)]
enum UnifiedPlugin {
    Controller(controller::ControllerPlugin),
    Sidebar(sidebar_plugin::SidebarRendererPlugin),
    #[default]
    Uninitialized,
}

impl ZellijPlugin for UnifiedPlugin {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        let mode = configuration.get("mode").map(|s| s.as_str());
        match mode {
            Some("controller") => {
                let mut plugin = controller::ControllerPlugin::default();
                plugin.load(configuration);
                *self = UnifiedPlugin::Controller(plugin);
            }
            _ => {
                let mut plugin = sidebar_plugin::SidebarRendererPlugin::default();
                plugin.load(configuration);
                *self = UnifiedPlugin::Sidebar(plugin);
            }
        }
    }

    fn update(&mut self, event: Event) -> bool {
        #[cfg(target_family = "wasm")]
        {
            let variant = match self {
                UnifiedPlugin::Controller(_) => "controller",
                UnifiedPlugin::Sidebar(_) => "sidebar",
                UnifiedPlugin::Uninitialized => "uninitialized",
            };
            let flag = format!("/cache/unified_update_{}", variant);
            if std::fs::metadata(&flag).is_err() {
                let _ = std::fs::write(&flag, "first update event received\n");
            }
        }
        match self {
            UnifiedPlugin::Controller(p) => p.update(event),
            UnifiedPlugin::Sidebar(p) => p.update(event),
            UnifiedPlugin::Uninitialized => false,
        }
    }

    fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
        match self {
            UnifiedPlugin::Controller(p) => p.pipe(pipe_message),
            UnifiedPlugin::Sidebar(p) => p.pipe(pipe_message),
            UnifiedPlugin::Uninitialized => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self {
            UnifiedPlugin::Controller(p) => p.render(rows, cols),
            UnifiedPlugin::Sidebar(p) => p.render(rows, cols),
            UnifiedPlugin::Uninitialized => {}
        }
    }
}

register_plugin!(UnifiedPlugin);

#[cfg(test)]
mod unified_plugin_tests {
    use super::*;

    #[test]
    fn test_unified_defaults_to_uninitialized() {
        let p = UnifiedPlugin::default();
        assert!(matches!(p, UnifiedPlugin::Uninitialized));
    }

    #[test]
    fn test_unified_loads_as_sidebar_without_mode() {
        let mut p = UnifiedPlugin::default();
        p.load(BTreeMap::new());
        assert!(matches!(p, UnifiedPlugin::Sidebar(_)));
    }

    #[test]
    fn test_unified_loads_as_controller_with_mode() {
        let mut p = UnifiedPlugin::default();
        let mut config = BTreeMap::new();
        config.insert("mode".to_string(), "controller".to_string());
        p.load(config);
        assert!(matches!(p, UnifiedPlugin::Controller(_)));
    }

    #[test]
    fn test_unified_loads_as_sidebar_with_explicit_mode() {
        let mut p = UnifiedPlugin::default();
        let mut config = BTreeMap::new();
        config.insert("mode".to_string(), "sidebar".to_string());
        p.load(config);
        assert!(matches!(p, UnifiedPlugin::Sidebar(_)));
    }

    #[test]
    fn test_unified_defaults_to_sidebar_for_unknown_mode() {
        let mut p = UnifiedPlugin::default();
        let mut config = BTreeMap::new();
        config.insert("mode".to_string(), "unknown".to_string());
        p.load(config);
        assert!(matches!(p, UnifiedPlugin::Sidebar(_)));
    }
}
