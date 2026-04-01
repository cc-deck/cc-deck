// Sidebar renderer plugin: thin, one per tab.
//
// Subscribes to Mouse and Key only. Receives RenderPayload from the
// controller via cc-deck:render pipe. Handles local interaction modes
// and forwards user actions to the controller via cc-deck:action pipe.

pub mod state;
pub mod render;
pub mod input;
pub mod modes;
pub mod rename;

use self::state::SidebarState;
use crate::config::PluginConfig;
use crate::session;
use cc_deck::{RenderPayload, SidebarHello, SidebarInit};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

/// The sidebar renderer plugin: one per tab, thin display + local interaction.
#[derive(Default)]
pub struct SidebarRendererPlugin {
    state: SidebarState,
}

impl ZellijPlugin for SidebarRendererPlugin {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        crate::debug_init();
        crate::debug_log("SIDEBAR LOAD start");

        self.state.config = PluginConfig::from_configuration(&configuration);

        subscribe(&[
            EventType::Mouse,
            EventType::Key,
            EventType::PermissionRequestResult,
        ]);

        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
        ]);

        crate::debug_log("SIDEBAR LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::PermissionRequestResult(status) => {
                if status == PermissionStatus::Granted {
                    self.state.permissions_granted = true;
                    set_selectable_wasm(false);

                    #[cfg(target_family = "wasm")]
                    {
                        self.state.my_plugin_id = get_plugin_ids().plugin_id;
                    }
                }
                false
            }
            Event::Mouse(mouse_event) => {
                if !self.state.permissions_granted {
                    return false;
                }
                input::handle_mouse(&mut self.state, mouse_event)
            }
            Event::Key(key) => {
                if !self.state.permissions_granted {
                    return false;
                }
                input::handle_key(&mut self.state, key)
            }
            _ => false,
        }
    }

    fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
        // Unblock CLI pipes immediately so broadcast dump-state doesn't hang
        #[cfg(target_family = "wasm")]
        if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
            zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
        }

        let name = &pipe_message.name;
        let payload = pipe_message.payload.as_deref();

        match name.as_str() {
            "cc-deck:render" => {
                // Re-request permissions if not yet granted (initial layout load
                // can suppress the dialog before Zellij's UI is ready)
                if !self.state.permissions_granted {
                    request_permission(&[
                        PermissionType::ReadApplicationState,
                        PermissionType::ChangeApplicationState,
                        PermissionType::ReadCliPipes,
                        PermissionType::MessageAndLaunchOtherPlugins,
                    ]);
                }

                if let Some(json) = payload {
                    if let Ok(render_payload) = serde_json::from_str::<RenderPayload>(json) {
                        // Update controller_plugin_id from payload
                        self.state.controller_plugin_id =
                            Some(render_payload.controller_plugin_id);

                        // Exit navigate mode if pending (Enter was pressed, switch completed)
                        if self.state.pending_navigate_exit && self.state.mode.is_navigating() {
                            self.state.mode = modes::SidebarMode::Passive;
                            self.state.filter_text.clear();
                            self.state.pending_navigate_exit = false;
                            // Don't set_selectable(false) here to avoid focus race
                        }

                        // Exit navigate mode if active tab changed (user switched tabs)
                        if self.state.mode.is_navigating() {
                            let old_active = self.state.cached_payload
                                .as_ref()
                                .map(|p| p.active_tab_index);
                            if old_active.is_some() && old_active != Some(render_payload.active_tab_index) {
                                self.state.mode = modes::SidebarMode::Passive;
                                self.state.filter_text.clear();
                                set_selectable_wasm(false);
                            }
                        }

                        self.state.cached_payload = Some(render_payload);
                        self.state.initialized = true;

                        // Preserve cursor position after payload update
                        self.state.preserve_cursor();

                        // Send hello on first payload if not yet sent
                        if !self.state.hello_sent {
                            self.send_hello();
                            self.state.hello_sent = true;
                        }

                        return true; // Trigger re-render
                    }
                }
                false
            }
            "cc-deck:sidebar-init" => {
                if let Some(json) = payload {
                    if let Ok(init) = serde_json::from_str::<SidebarInit>(json) {
                        self.state.my_tab_index = Some(init.tab_index);
                        self.state.controller_plugin_id =
                            Some(init.controller_plugin_id);
                        crate::debug_log(&format!(
                            "SIDEBAR INIT tab_index={} controller={}",
                            init.tab_index, init.controller_plugin_id
                        ));
                    }
                }
                false
            }
            "cc-deck:sidebar-reindex" => {
                crate::debug_log("SIDEBAR REINDEX: clearing tab_index, re-sending hello");
                self.state.my_tab_index = None;
                self.state.hello_sent = false;
                // Will re-send hello on next render payload
                false
            }
            "cc-deck:navigate" => {
                // Controller forwarded a navigate keybinding press.
                // Only the active-tab sidebar should respond.
                if let Some(json) = payload {
                    if let Ok(nav) = serde_json::from_str::<serde_json::Value>(json) {
                        let active = nav
                            .get("active_tab_index")
                            .and_then(|v| v.as_u64())
                            .map(|v| v as usize);
                        if active == self.state.my_tab_index {
                            let backward = nav
                                .get("direction")
                                .and_then(|v| v.as_str())
                                == Some("backward");
                            if backward {
                                input::toggle_navigate_prev(&mut self.state);
                            } else {
                                input::toggle_navigate(&mut self.state);
                            }
                            return true;
                        }
                    }
                }
                false
            }
            _ => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        if !self.state.permissions_granted {
            render::render_permission_prompt(rows, cols);
            return;
        }
        self.state.clear_expired_notifications();
        let regions = render::render_sidebar(&self.state, rows, cols);
        self.state.click_regions = regions;
    }
}

impl SidebarRendererPlugin {
    /// Handle a CustomMessage event (from controller's post_message_to broadcast).
    fn handle_custom_message(&mut self, name: &str, payload: &str) -> bool {
        if name == "cc-deck:render" {
            if let Ok(render_payload) = serde_json::from_str::<RenderPayload>(payload) {
                self.state.controller_plugin_id =
                    Some(render_payload.controller_plugin_id);
                self.state.cached_payload = Some(render_payload);
                self.state.initialized = true;
                self.state.preserve_cursor();

                if !self.state.hello_sent {
                    self.send_hello();
                    self.state.hello_sent = true;
                }
                return true;
            }
        } else if name == "cc-deck:sidebar-init" {
            if let Ok(init) = serde_json::from_str::<SidebarInit>(payload) {
                self.state.my_tab_index = Some(init.tab_index);
                self.state.controller_plugin_id = Some(init.controller_plugin_id);
            }
        } else if name == "cc-deck:sidebar-reindex" {
            self.state.my_tab_index = None;
            self.state.hello_sent = false;
        } else if name == "cc-deck:navigate" {
            if let Ok(nav) = serde_json::from_str::<serde_json::Value>(payload) {
                let active = nav.get("active_tab_index")
                    .and_then(|v| v.as_u64())
                    .map(|v| v as usize);
                if active == self.state.my_tab_index {
                    input::toggle_navigate(&mut self.state);
                    return true;
                }
            }
        }
        false
    }

    fn send_hello(&self) {
        let hello = SidebarHello {
            plugin_id: self.state.my_plugin_id,
        };
        send_hello_wasm(&hello);
    }
}

// --- WASM-gated helpers ---

#[cfg(target_family = "wasm")]
fn set_selectable_wasm(selectable: bool) {
    set_selectable(selectable);
}

#[cfg(not(target_family = "wasm"))]
fn set_selectable_wasm(_selectable: bool) {}

#[cfg(target_family = "wasm")]
fn send_hello_wasm(hello: &SidebarHello) {
    let json = match serde_json::to_string(hello) {
        Ok(j) => j,
        Err(_) => return,
    };
    let mut msg = MessageToPlugin::new("cc-deck:sidebar-hello");
    msg.message_payload = Some(json);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn send_hello_wasm(_hello: &SidebarHello) {}
