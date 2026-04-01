// Sidebar discovery registry: hello/init handshake and tab reindexing.
//
// Sidebars register with the controller by sending cc-deck:sidebar-hello
// after receiving their first render payload. The controller cross-references
// the plugin_id with the PaneManifest to determine which tab the sidebar
// lives on, then responds with cc-deck:sidebar-init.

use super::state::ControllerState;
use cc_deck::{SidebarHello, SidebarInit};
use zellij_tile::prelude::*;

/// Handle a sidebar-hello registration message.
/// Cross-reference the plugin_id with the PaneManifest to find the tab.
pub fn handle_sidebar_hello(state: &mut ControllerState, hello: SidebarHello) {
    let tab_index = find_tab_for_plugin(state, hello.plugin_id);

    if let Some(idx) = tab_index {
        state.sidebar_registry.insert(hello.plugin_id, idx);
        crate::debug_log(&format!(
            "CTRL SIDEBAR registered plugin_id={} on tab={}",
            hello.plugin_id, idx
        ));

        let init = SidebarInit {
            tab_index: idx,
            controller_plugin_id: state.plugin_id,
        };
        send_sidebar_init(hello.plugin_id, &init);
    } else {
        crate::debug_log(&format!(
            "CTRL SIDEBAR plugin_id={} not found in manifest, skipping",
            hello.plugin_id
        ));
    }
}

/// Handle tab reindex: broadcast cc-deck:sidebar-reindex to all sidebars.
/// Called when TabUpdate shows a changed tab count.
pub fn handle_tab_reindex(state: &mut ControllerState) {
    crate::debug_log("CTRL REINDEX broadcasting sidebar-reindex");
    state.sidebar_registry.clear();
    broadcast_reindex();
}

/// Remove registry entries for plugin_ids that no longer appear in the PaneManifest.
pub fn cleanup_dead_sidebars(state: &mut ControllerState) {
    let manifest = match &state.pane_manifest {
        Some(m) => m,
        None => return,
    };

    let mut plugin_ids_in_manifest = std::collections::HashSet::new();
    for panes in manifest.panes.values() {
        for pane in panes {
            if pane.is_plugin {
                plugin_ids_in_manifest.insert(pane.id);
            }
        }
    }

    let before = state.sidebar_registry.len();
    state
        .sidebar_registry
        .retain(|plugin_id, _| plugin_ids_in_manifest.contains(plugin_id));

    if state.sidebar_registry.len() != before {
        crate::debug_log(&format!(
            "CTRL SIDEBAR cleaned {} dead sidebars, {} remaining",
            before - state.sidebar_registry.len(),
            state.sidebar_registry.len()
        ));
    }
}

/// Find which tab contains a plugin pane with the given plugin_id.
fn find_tab_for_plugin(state: &ControllerState, plugin_id: u32) -> Option<usize> {
    let manifest = state.pane_manifest.as_ref()?;
    for (&tab_pos, panes) in &manifest.panes {
        for pane in panes {
            if pane.is_plugin && pane.id == plugin_id {
                return Some(tab_pos);
            }
        }
    }
    None
}

// --- WASM-gated host function wrappers ---

#[cfg(target_family = "wasm")]
fn send_sidebar_init(plugin_id: u32, init: &SidebarInit) {
    let json = match serde_json::to_string(init) {
        Ok(j) => j,
        Err(e) => {
            crate::debug_log(&format!(
                "CTRL SIDEBAR failed to serialize init for plugin_id={}: {}",
                plugin_id, e
            ));
            return;
        }
    };
    let mut msg = MessageToPlugin::new("cc-deck:sidebar-init");
    msg.message_payload = Some(json);
    msg.destination_plugin_id = Some(plugin_id);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn send_sidebar_init(_plugin_id: u32, _init: &SidebarInit) {}

#[cfg(target_family = "wasm")]
fn broadcast_reindex() {
    let msg = MessageToPlugin::new("cc-deck:sidebar-reindex");
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn broadcast_reindex() {}

/// Auto-discover sidebar plugin panes from the PaneManifest.
/// Registers any plugin pane that is NOT the controller itself.
/// This avoids depending on the hello handshake for initial registration.
pub fn discover_sidebars_from_manifest(state: &mut ControllerState) {
    let manifest = match &state.pane_manifest {
        Some(m) => m,
        None => return,
    };

    for (&tab_pos, panes) in &manifest.panes {
        for pane in panes {
            // Register plugin panes that aren't the controller
            if pane.is_plugin && pane.id != state.plugin_id && !state.sidebar_registry.contains_key(&pane.id) {
                state.sidebar_registry.insert(pane.id, tab_pos);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cleanup_dead_sidebars_no_manifest() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(10, 0);
        state.sidebar_registry.insert(20, 1);
        // No manifest - cleanup is a no-op
        cleanup_dead_sidebars(&mut state);
        assert_eq!(state.sidebar_registry.len(), 2);
    }

    #[test]
    fn test_handle_tab_reindex_clears_registry() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(10, 0);
        state.sidebar_registry.insert(20, 1);
        handle_tab_reindex(&mut state);
        assert!(state.sidebar_registry.is_empty());
    }
}
