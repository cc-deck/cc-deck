// Sidebar discovery registry: hello/init handshake and tab reindexing.
//
// Sidebars proactively register with the controller by sending
// cc-deck:sidebar-hello after permission grant. The controller cross-references
// the plugin_id with the PaneManifest to determine which tab the sidebar
// lives on, then responds with cc-deck:sidebar-init.

use super::state::ControllerState;
use cc_deck::{SidebarHello, SidebarInit};
#[allow(unused_imports)]
use zellij_tile::prelude::*;

/// Handle a sidebar-hello registration message.
/// Cross-reference the plugin_id with the PaneManifest to find the tab.
pub fn handle_sidebar_hello(state: &mut ControllerState, hello: SidebarHello) {
    if !state.client_views.is_empty() && !state.client_views.contains_key(&hello.client_id) {
        crate::debug_log(&format!(
            "CTRL SIDEBAR ignoring hello from disconnected/unobserved client_id={}",
            hello.client_id
        ));
        return;
    }
    let tab_index = find_tab_for_plugin(state, hello.plugin_id);

    if let Some(idx) = tab_index {
        // Dedup: remove any existing sidebar with the same (tab_index, client_id).
        // This prevents unbounded registry growth when a client reconnects and
        // gets new plugin instances on the same tabs.
        state.sidebar_registry.retain(|&pid, &mut (tab, cid)| {
            pid == hello.plugin_id || tab != idx || cid != hello.client_id
        });

        state
            .sidebar_registry
            .insert(hello.plugin_id, (idx, hello.client_id));
        crate::debug_log(&format!(
            "CTRL SIDEBAR registered plugin_id={} on tab={} client_id={}",
            hello.plugin_id, idx, hello.client_id
        ));

        let init = SidebarInit {
            tab_index: idx,
            controller_plugin_id: state.plugin_id,
        };
        send_sidebar_init(hello.plugin_id, &init);
        super::render_broadcast::targeted_render(state, hello.plugin_id);
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

        // Client liveness is reconciled from TabUpdate, never from the pane
        // manifest (which can retain zombie plugin panes after disconnect).
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

#[cfg(test)]
mod tests {
    use super::*;

    fn make_plugin_pane(id: u32) -> PaneInfo {
        PaneInfo {
            id,
            is_plugin: true,
            is_focused: false,
            is_fullscreen: false,
            is_floating: false,
            is_suppressed: false,
            title: String::new(),
            exited: false,
            exit_status: None,
            is_held: false,
            pane_x: 0,
            pane_content_x: 0,
            pane_y: 0,
            pane_content_y: 0,
            pane_rows: 0,
            pane_content_rows: 0,
            pane_columns: 0,
            pane_content_columns: 0,
            cursor_coordinates_in_pane: None,
            terminal_command: None,
            plugin_url: None,
            is_selectable: true,
            index_in_pane_group: std::collections::BTreeMap::new(),
            default_bg: None,
            default_fg: None,
        }
    }

    #[test]
    fn test_cleanup_dead_sidebars_no_manifest() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(10, (0, 0));
        state.sidebar_registry.insert(20, (1, 0));
        // No manifest - cleanup is a no-op
        cleanup_dead_sidebars(&mut state);
        assert_eq!(state.sidebar_registry.len(), 2);
    }

    #[test]
    fn test_handle_sidebar_hello_registers_and_renders() {
        let mut state = ControllerState::default();
        state.plugin_id = 1;

        let mut panes = std::collections::HashMap::new();
        panes.insert(0, vec![make_plugin_pane(1), make_plugin_pane(42)]);
        state.pane_manifest = Some(PaneManifest { panes });

        state.voice_enabled = true;
        state.voice_muted = false;

        let hello = SidebarHello {
            plugin_id: 42,
            client_id: 0,
        };
        // In non-WASM test mode, send_sidebar_init and targeted_render
        // are no-ops. This test verifies registration succeeds and the
        // code path through targeted_render does not panic.
        handle_sidebar_hello(&mut state, hello);

        assert!(state.sidebar_registry.contains_key(&42));
        assert_eq!(*state.sidebar_registry.get(&42).unwrap(), (0, 0));
    }

    #[test]
    fn test_handle_tab_reindex_clears_registry() {
        let mut state = ControllerState::default();
        state.sidebar_registry.insert(10, (0, 0));
        state.sidebar_registry.insert(20, (1, 0));
        handle_tab_reindex(&mut state);
        assert!(state.sidebar_registry.is_empty());
    }

    // --- T009: Dedup tests for (tab_index, client_id) ---

    #[test]
    fn test_dedup_replaces_old_entry_same_tab_same_client() {
        // When a new sidebar hello arrives for the same (tab, client_id),
        // the old entry is removed and replaced by the new plugin_id.
        let mut state = ControllerState::default();
        state.plugin_id = 1;

        let mut panes = std::collections::HashMap::new();
        panes.insert(
            0,
            vec![
                make_plugin_pane(1),
                make_plugin_pane(50),
                make_plugin_pane(60),
            ],
        );
        state.pane_manifest = Some(PaneManifest { panes });

        // First hello: plugin 50, tab 0, client 2
        let hello1 = SidebarHello {
            plugin_id: 50,
            client_id: 2,
        };
        handle_sidebar_hello(&mut state, hello1);
        assert!(state.sidebar_registry.contains_key(&50));
        assert_eq!(state.sidebar_registry.len(), 1);

        // Second hello: plugin 60, same tab 0, same client 2
        // Should replace plugin 50's entry.
        let hello2 = SidebarHello {
            plugin_id: 60,
            client_id: 2,
        };
        handle_sidebar_hello(&mut state, hello2);
        assert!(!state.sidebar_registry.contains_key(&50));
        assert!(state.sidebar_registry.contains_key(&60));
        assert_eq!(state.sidebar_registry.len(), 1);
    }

    #[test]
    fn test_different_client_ids_same_tab_coexist() {
        // Two sidebars from different clients on the same tab should
        // both remain in the registry (no dedup across clients).
        let mut state = ControllerState::default();
        state.plugin_id = 1;

        let mut panes = std::collections::HashMap::new();
        panes.insert(
            0,
            vec![
                make_plugin_pane(1),
                make_plugin_pane(50),
                make_plugin_pane(60),
            ],
        );
        state.pane_manifest = Some(PaneManifest { panes });

        let hello1 = SidebarHello {
            plugin_id: 50,
            client_id: 1,
        };
        handle_sidebar_hello(&mut state, hello1);

        let hello2 = SidebarHello {
            plugin_id: 60,
            client_id: 2,
        };
        handle_sidebar_hello(&mut state, hello2);

        // Both entries should coexist
        assert!(state.sidebar_registry.contains_key(&50));
        assert!(state.sidebar_registry.contains_key(&60));
        assert_eq!(state.sidebar_registry.len(), 2);
    }

    #[test]
    fn test_backward_compat_hello_default_client_id() {
        // A SidebarHello without client_id (deserialized with default=0)
        // should register normally.
        let mut state = ControllerState::default();
        state.plugin_id = 1;

        let mut panes = std::collections::HashMap::new();
        panes.insert(0, vec![make_plugin_pane(1), make_plugin_pane(50)]);
        state.pane_manifest = Some(PaneManifest { panes });

        let hello = SidebarHello {
            plugin_id: 50,
            client_id: 0, // default value from #[serde(default)]
        };
        handle_sidebar_hello(&mut state, hello);

        assert!(state.sidebar_registry.contains_key(&50));
        assert_eq!(state.sidebar_registry[&50], (0, 0));
    }

    #[test]
    fn test_cleanup_dead_sidebars_only_removes_registry_entries() {
        let mut state = ControllerState::default();
        state.plugin_id = 1;

        // Two sidebars: plugin 10 (client 1), plugin 20 (client 2)
        state.sidebar_registry.insert(10, (0, 1));
        state.sidebar_registry.insert(20, (0, 2));

        // Manifest only contains plugin 10 (client 2's sidebar is gone)
        let mut panes = std::collections::HashMap::new();
        panes.insert(0, vec![make_plugin_pane(1), make_plugin_pane(10)]);
        state.pane_manifest = Some(PaneManifest { panes });

        cleanup_dead_sidebars(&mut state);

        assert!(state.sidebar_registry.contains_key(&10));
        assert!(!state.sidebar_registry.contains_key(&20));
    }
}
