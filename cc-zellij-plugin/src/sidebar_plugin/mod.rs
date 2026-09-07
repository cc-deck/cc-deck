// Sidebar renderer plugin: thin, one per tab.
//
// Subscribes to Mouse and Key only. Receives RenderPayload from the
// controller via cc-deck:render pipe. Handles local interaction modes
// and forwards user actions to the controller via cc-deck:action pipe.

#[cfg(test)]
mod fuzz_tests;
pub mod input;
#[cfg(test)]
mod integration_tests;
pub mod modes;
pub mod rename;
pub mod render;
pub mod state;
#[cfg(test)]
pub(crate) mod test_helpers;

use self::state::SidebarState;
use crate::config::PluginConfig;
use cc_deck::{RenderPayload, SidebarHello, SidebarInit};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

/// How many times each plugin re-asks for a permission grant that never
/// arrived before giving up and leaving the request to the user.
pub(crate) const PERMISSION_RETRY_LIMIT: u8 = 5;

/// The sidebar renderer plugin: one per tab, thin display + local interaction.
#[derive(Default)]
pub struct SidebarRendererPlugin {
    state: SidebarState,
}

impl ZellijPlugin for SidebarRendererPlugin {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        crate::install_panic_hook();
        crate::debug_init();
        crate::debug_log_immediate("SIDEBAR LOAD start");

        self.state.config = PluginConfig::from_configuration(&configuration);

        crate::wasm_compat::subscribe_wasm(&[
            EventType::Mouse,
            EventType::Key,
            EventType::Timer,
            EventType::PermissionRequestResult,
        ]);

        // Request the full permission set, including what only the controller
        // uses. Both roles share one WASM URL, and Zellij caches grants by URL,
        // so this dialog is the one that grants the background controller.
        crate::wasm_compat::request_permission_wasm(&cc_deck::REQUIRED_PERMISSIONS);

        // Drive our own retry rather than waiting to be spoken to.
        //
        // Zellij addresses the permission result to a specific client. A pane
        // created during the initial layout load can be handed that result
        // before any client is ready to receive it, and the grant is then
        // simply lost. The pane sits on the permission prompt indefinitely.
        //
        // The recovery that already existed, re-requesting when a render pipe
        // arrives, cannot fire in that state: the controller only broadcasts to
        // sidebars that registered, registration needs the hello, and the hello
        // is only sent once the grant arrives. A closed loop with no way in.
        //
        // A timer sits outside that loop. On a healthy start the grant has
        // already arrived and the first tick returns early, so this costs one
        // tick and nothing more.
        crate::wasm_compat::set_timeout_wasm(1.0);

        crate::debug_log_immediate("SIDEBAR LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::PermissionRequestResult(status) => {
                crate::debug_log(&format!("SIDEBAR PERMISSION result={:?}", status));
                if status == PermissionStatus::Granted {
                    self.state.permissions_granted = true;
                    crate::wasm_compat::set_selectable_wasm(false);

                    #[cfg(target_family = "wasm")]
                    {
                        let ids = get_plugin_ids();
                        self.state.my_plugin_id = ids.plugin_id;
                        self.state.my_client_id = ids.client_id;
                    }

                    self.send_hello();
                    // Retry registration until sidebar-init arrives. This is
                    // resilient to controller and manifest startup races.
                    crate::wasm_compat::set_timeout_wasm(1.0);

                    // Repaint now: render() draws the permission prompt while
                    // permissions_granted is false, and set_selectable(false)
                    // above has already made this pane impossible to focus.
                    return true;
                }
                // Denied: stop asking. The prompt stays on screen so the user
                // can see why the sidebar is empty, but it is never re-raised.
                self.state.permission_retries = PERMISSION_RETRY_LIMIT;
                crate::debug_log("SIDEBAR PERMISSION denied; not retrying");
                false
            }
            Event::Timer(_) => {
                // Still unpermissioned means the grant was lost in transit, not
                // that the user has yet to answer: a cached grant is delivered
                // without any prompt. Ask again from here, where nothing depends
                // on the controller having heard of us.
                if !self.state.permissions_granted {
                    // Bounded: a grant that was merely lost arrives on the next
                    // ask, so a handful of attempts is plenty. A request that is
                    // genuinely waiting on the user must not be re-raised
                    // forever, or we would redraw their prompt every second.
                    if self.state.permission_retries >= PERMISSION_RETRY_LIMIT {
                        return false;
                    }
                    self.state.permission_retries += 1;
                    crate::debug_log("SIDEBAR TIMER re-requesting lost permission grant");
                    crate::wasm_compat::request_permission_wasm(&cc_deck::REQUIRED_PERMISSIONS);
                    crate::wasm_compat::set_timeout_wasm(1.0);
                    return false;
                }

                if self.state.my_tab_index.is_some() && self.state.initialized {
                    return false;
                }
                self.send_hello();

                // Reschedule timer if still waiting
                if self.state.my_tab_index.is_none() || !self.state.initialized {
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
                // A lost grant is recovered by the bounded timer retry above;
                // this path deliberately does not ask again, so a user who is
                // still reading the prompt is never nagged by a render.
                if let Some(json) = payload {
                    if let Ok(render_payload) = serde_json::from_str::<RenderPayload>(json) {
                        // Update controller_plugin_id from payload
                        self.state.controller_plugin_id = Some(render_payload.controller_plugin_id);

                        // Clear predictive focus once the authoritative client
                        // view acknowledges the same pane.
                        if let Some(override_pid) = self.state.local_focus_override {
                            let confirmed = render_payload
                                .client_views
                                .get(&self.state.my_client_id)
                                .is_some_and(|view| view.focused_pane_id == Some(override_pid));
                            if confirmed {
                                self.state.local_focus_override = None;
                                crate::debug_log(&format!(
                                    "SIDEBAR PAYLOAD: cleared override={override_pid}, client view confirmed",
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
                            let old_active = self
                                .state
                                .cached_payload
                                .as_ref()
                                .and_then(|p| p.client_views.get(&self.state.my_client_id))
                                .and_then(|view| view.active_tab_index);
                            let new_active = render_payload
                                .client_views
                                .get(&self.state.my_client_id)
                                .and_then(|view| view.active_tab_index);
                            if !in_grace && old_active.is_some() && old_active != new_active {
                                crate::debug_log(&format!(
                                    "SIDEBAR RENDER: exiting navigate due to tab change old={:?} new={:?}",
                                    old_active, new_active,
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
                        if let modes::SidebarMode::RenamePassive {
                            ref rename,
                            entered_at_ms,
                        } = self.state.mode
                        {
                            let now_ms = crate::session::unix_now_ms();
                            let in_grace =
                                now_ms.saturating_sub(entered_at_ms) < modes::ENTER_GRACE_MS;
                            if in_grace {
                                // Double-click enters RenamePassive right after
                                // the first click sends a Switch action. The
                                // controller processes Switch asynchronously and
                                // calls focus_terminal_pane, stealing focus from
                                // the sidebar. Re-assert focus so keystrokes
                                // reach the rename input.
                                crate::debug_log(
                                    "SIDEBAR RENDER: re-asserting focus during RenamePassive grace",
                                );
                                input::focus_self_wasm();
                            } else if render_payload
                                .client_views
                                .get(&self.state.my_client_id)
                                .and_then(|view| view.focused_pane_id)
                                != Some(rename.pane_id)
                            {
                                crate::debug_log(&format!(
                                    "SIDEBAR RENDER: exiting RenamePassive, focus moved from {} to {:?}",
                                    rename.pane_id, render_payload.client_views.get(&self.state.my_client_id)
                                        .and_then(|view| view.focused_pane_id),
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

                        let old_focus = self.state.effective_focused_pane_id();

                        self.state.cached_payload = Some(render_payload);
                        self.state.initialized = true;

                        // Reset manual scroll when focus changes outside navigate mode
                        if !self.state.mode.is_navigating() && self.state.scroll_offset.is_some() {
                            let new_focus = self.state.effective_focused_pane_id();
                            if old_focus != new_focus {
                                self.state.scroll_offset = None;
                            }
                        }

                        // After a sort, relocate the cursor to track the
                        // same session by pane_id instead of by index.
                        if let Some(pane_id) = self.state.sort_cursor_pane_id.take() {
                            self.state.track_cursor_by_pane_id(pane_id);
                        }

                        // Preserve cursor position after payload update
                        self.state.preserve_cursor();

                        return true; // Trigger re-render
                    }
                }
                false
            }
            "cc-deck:sidebar-init" => {
                if let Some(json) = payload {
                    if let Ok(init) = serde_json::from_str::<SidebarInit>(json) {
                        self.state.my_tab_index = Some(init.tab_index);
                        self.state.controller_plugin_id = Some(init.controller_plugin_id);
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
                self.send_hello();
                crate::wasm_compat::set_timeout_wasm(1.0);
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
                            let backward =
                                nav.get("direction").and_then(|v| v.as_str()) == Some("backward");
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
        let result = render::render_sidebar(&self.state, rows, cols);
        self.state.click_regions = result.click_regions;
        self.state.last_viewport_start = result.viewport_start;
        self.state.last_max_visible = result.max_visible;
    }
}

impl SidebarRendererPlugin {
    fn send_hello(&self) {
        let hello = SidebarHello {
            plugin_id: self.state.my_plugin_id,
            client_id: self.state.my_client_id,
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

#[cfg(test)]
impl SidebarRendererPlugin {
    pub(crate) fn test_state(&self) -> &SidebarState {
        &self.state
    }
    pub(crate) fn test_state_mut(&mut self) -> &mut SidebarState {
        &mut self.state
    }
}
