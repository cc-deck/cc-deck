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
#[cfg(test)]
pub(crate) mod test_helpers;
#[cfg(test)]
mod fuzz_tests;
#[cfg(test)]
mod integration_tests;

use self::state::SidebarState;
use crate::config::PluginConfig;
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
        crate::install_panic_hook();
        crate::debug_init();
        crate::debug_log("SIDEBAR LOAD start");

        self.state.config = PluginConfig::from_configuration(&configuration);

        crate::wasm_compat::subscribe_wasm(&[
            EventType::Mouse,
            EventType::Key,
            EventType::Timer,
            EventType::PermissionRequestResult,
        ]);

        // Request the full permission set (including RunCommands and Reconfigure
        // needed by the controller). Since both controller and sidebar share the
        // same WASM URL, the sidebar's permission dialog must cover all permissions
        // the controller needs. Background plugins (controller) cannot show dialogs.
        crate::wasm_compat::request_permission_wasm(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
            PermissionType::WriteToStdin,
        ]);

        crate::debug_log("SIDEBAR LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::PermissionRequestResult(status) => {
                if status == PermissionStatus::Granted {
                    self.state.permissions_granted = true;
                    crate::wasm_compat::set_selectable_wasm(false);

                    #[cfg(target_family = "wasm")]
                    {
                        self.state.my_plugin_id = get_plugin_ids().plugin_id;
                    }

                    // Start the timer for render request fallback
                    crate::wasm_compat::set_timeout_wasm(1.0);
                }
                false
            }
            Event::Timer(_) => {
                if self.state.initialized {
                    // No longer need timer events after first render received
                    return false;
                }
                self.state.ticks_since_init = self.state.ticks_since_init.saturating_add(1);

                // One-shot render request: if no payload received after 3 ticks
                if self.state.ticks_since_init >= 3
                    && !self.state.initialized
                    && !self.state.render_request_sent
                {
                    if let Some(controller_id) = self.state.controller_plugin_id {
                        send_render_request(self.state.my_plugin_id, controller_id);
                        self.state.render_request_sent = true;
                        crate::debug_log(&format!(
                            "SIDEBAR[{}] sent render-request to controller={}",
                            self.state.my_plugin_id, controller_id
                        ));
                    }
                    // If controller_plugin_id is None, wait for next tick
                }

                // Reschedule timer if still waiting
                if !self.state.initialized {
                    crate::wasm_compat::set_timeout_wasm(1.0);
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
                    crate::wasm_compat::request_permission_wasm(&[
                        PermissionType::ReadApplicationState,
                        PermissionType::ChangeApplicationState,
                        PermissionType::RunCommands,
                        PermissionType::ReadCliPipes,
                        PermissionType::MessageAndLaunchOtherPlugins,
                        PermissionType::Reconfigure,
                        PermissionType::WriteToStdin,
                    ]);
                }

                if let Some(json) = payload {
                    if let Ok(render_payload) = serde_json::from_str::<RenderPayload>(json) {
                        // Update controller_plugin_id from payload
                        self.state.controller_plugin_id =
                            Some(render_payload.controller_plugin_id);

                        // Clear predictive focus override once the controller
                        // confirms the focus change in its payload.
                        if let Some(override_pid) = self.state.local_focus_override {
                            if render_payload.focused_pane_id == Some(override_pid) {
                                self.state.local_focus_override = None;
                                crate::debug_log(&format!(
                                    "SIDEBAR PAYLOAD: cleared override={override_pid}, payload_focus={:?}",
                                    render_payload.focused_pane_id
                                ));
                            } else {
                                crate::debug_log(&format!(
                                    "SIDEBAR PAYLOAD: kept override={override_pid}, payload_focus={:?} (mismatch)",
                                    render_payload.focused_pane_id
                                ));
                            }
                        } else {
                            let prev_focus = self.state.cached_payload.as_ref().and_then(|p| p.focused_pane_id);
                            if render_payload.focused_pane_id != prev_focus {
                                crate::debug_log(&format!(
                                    "SIDEBAR PAYLOAD: no override, focus changed {:?} -> {:?}",
                                    prev_focus, render_payload.focused_pane_id
                                ));
                            }
                        }

                        // Exit navigate mode if active tab changed (user switched tabs),
                        // but NOT during the grace period after entering navigate.
                        // Alt+s/Alt+a cause tab switches that arrive after the navigate
                        // entry; exiting immediately would fight the navigation action.
                        if self.state.mode.is_navigating() {
                            let now_ms = crate::session::unix_now_ms();
                            let in_grace = self.state.mode.in_grace_period(now_ms);
                            let old_active = self.state.cached_payload
                                .as_ref()
                                .map(|p| p.active_tab_index);
                            if !in_grace && old_active.is_some() && old_active != Some(render_payload.active_tab_index) {
                                crate::debug_log(&format!(
                                    "SIDEBAR RENDER: exiting navigate due to tab change old={:?} new={}",
                                    old_active, render_payload.active_tab_index,
                                ));
                                self.state.mode = modes::SidebarMode::Passive;
                                self.state.filter_text.clear();
                                crate::wasm_compat::set_selectable_wasm(false);
                            }
                        }

                        // Navigation mode persists until explicit exit (Escape,
                        // Enter/select, or PaneUpdate detecting terminal focus).
                        // No inactivity timeout: the user may be reading the
                        // sidebar without pressing keys.

                        // Exit RenamePassive when focus moves to a different pane.
                        // Without this, the rename cursor stays visible on a row
                        // that is no longer active, creating a ghost rename state.
                        if let modes::SidebarMode::RenamePassive { ref rename, entered_at_ms } = self.state.mode {
                            let now_ms = crate::session::unix_now_ms();
                            let in_grace = now_ms.saturating_sub(entered_at_ms) < modes::ENTER_GRACE_MS;
                            if !in_grace && render_payload.focused_pane_id != Some(rename.pane_id) {
                                crate::debug_log(&format!(
                                    "SIDEBAR RENDER: exiting RenamePassive, focus moved from {} to {:?}",
                                    rename.pane_id, render_payload.focused_pane_id,
                                ));
                                self.state.mode = modes::SidebarMode::Passive;
                                crate::wasm_compat::set_selectable_wasm(false);
                            }
                        }

                        // Clear mute override on disconnect (prevents stale
                        // overrides surviving across sleep/wake cycles) or
                        // when the controller confirms the expected state.
                        if !render_payload.voice_connected {
                            self.state.local_mute_override = None;
                        } else if let Some(expected) = self.state.local_mute_override {
                            if render_payload.voice_muted == expected {
                                self.state.local_mute_override = None;
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
    fn send_hello(&self) {
        let hello = SidebarHello {
            plugin_id: self.state.my_plugin_id,
        };
        send_hello_wasm(&hello);
    }
}

// --- WASM-gated helpers ---

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

/// Send a render-request to the controller.
#[cfg(target_family = "wasm")]
fn send_render_request(sidebar_plugin_id: u32, controller_plugin_id: u32) {
    let mut msg = MessageToPlugin::new("cc-deck:render-request");
    msg.message_payload = Some(sidebar_plugin_id.to_string());
    msg.destination_plugin_id = Some(controller_plugin_id);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn send_render_request(_sidebar_plugin_id: u32, _controller_plugin_id: u32) {}

#[cfg(test)]
impl SidebarRendererPlugin {
    pub(crate) fn test_state(&self) -> &SidebarState {
        &self.state
    }
    pub(crate) fn test_state_mut(&mut self) -> &mut SidebarState {
        &mut self.state
    }
}
