// Controller plugin: background, single instance, owns all authoritative state.
//
// The controller is a headless Zellij plugin (no rendering) that:
// - Subscribes to heavyweight events (PaneUpdate, TabUpdate, Timer, etc.)
//   but NOT Mouse or Key (no UI interaction)
// - Owns the single authoritative session state (BTreeMap<u32, Session>)
// - Processes hook events from the CLI (cc-deck:hook)
// - Broadcasts RenderPayload to sidebar instances (cc-deck:render)
// - Handles action messages from sidebars (cc-deck:action)
// - Manages sidebar discovery (cc-deck:sidebar-hello/init/reindex)
// - Registers keybindings via reconfigure()
// - Persists state to /cache/sessions.json (single writer)

pub mod actions;
pub mod events;
pub mod hooks;
pub mod render_broadcast;
pub mod sidebar_registry;
pub mod state;

use self::state::ControllerState;
use crate::config::PluginConfig;
use crate::pipe_handler::{parse_pipe_message, PipeAction};
use crate::session;
use cc_deck::{ActionMessage, SidebarHello};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

/// The controller plugin: headless, session-global, single instance.
#[derive(Default)]
pub struct ControllerPlugin {
    state: ControllerState,
}

impl ZellijPlugin for ControllerPlugin {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        crate::install_panic_hook();
        crate::debug_init();
        crate::debug_log("CTRL LOAD start");

        self.state.config = PluginConfig::from_configuration(&configuration);
        self.state.perf.enabled = self.state.config.perf_enabled;
        self.state.perf.dump_interval_secs = self.state.config.perf_interval;

        // Subscribe to heavyweight events only (no Mouse, Key since headless)
        crate::wasm_compat::subscribe_wasm(&[
            EventType::TabUpdate,
            EventType::PaneUpdate,
            EventType::Timer,
            EventType::PermissionRequestResult,
            EventType::RunCommandResult,
            EventType::CommandPaneOpened,
            EventType::PaneClosed,
        ]);

        crate::wasm_compat::request_permission_wasm(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
            PermissionType::WriteToStdin,
        ]);

        crate::wasm_compat::set_timeout_wasm(self.state.config.timer_interval);

        // Controller is headless: never selectable
        crate::wasm_compat::set_selectable_wasm(false);

        crate::debug_log("CTRL LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::PermissionRequestResult(status) => {
                crate::debug_log(&format!("CTRL PERMISSION result={status:?}"));
                if status == PermissionStatus::Granted {
                    self.state.permissions_granted = true;

                    // Capture plugin ID for keybinding registration and sidebar init
                    self.state.plugin_id = get_plugin_id_wasm();

                    crate::wasm_compat::set_selectable_wasm(false);

                    // Restore persisted sessions (reattach recovery)
                    let restored = ControllerState::restore_sessions();
                    if !restored.is_empty() {
                        self.state.merge_sessions(restored);
                    }
                    // Grace period lets the pane manifest stabilize before
                    // we start removing "dead" sessions that may just be
                    // slow to appear.
                    self.state.startup_grace_until =
                        Some(session::unix_now_ms() + 3000);

                    // Process any events queued before permissions
                    let pending = std::mem::take(&mut self.state.pending_events);
                    for e in pending {
                        self.handle_event_inner(e);
                    }
                    // Immediately broadcast initial render payload so
                    // sidebars stop showing "Waiting for controller..."
                    render_broadcast::broadcast_render(&self.state);
                    self.state.render_dirty = false;
                }
                false // Controller has no UI to render
            }
            _ => {
                if !self.state.permissions_granted {
                    self.state.pending_events.push(event);
                    return false;
                }
                self.handle_event_inner(event);
                // Automatic events (PaneUpdate, TabUpdate, Timer) use coalesced
                // rendering via render_dirty flag + timer flush. This prevents
                // message storms during rapid state changes (e.g., snapshot restore).
                false // Controller has no UI
            }
        }
    }

    fn pipe(&mut self, pipe_message: PipeMessage) -> bool {
        if !self.state.permissions_granted {
            return false;
        }

        // Trace log (skip high-volume internal messages)
        if pipe_message.name != "cc-deck:render"
            && pipe_message.name != "cc-deck:sidebar-init"
            && pipe_message.name != "cc-deck:sidebar-reindex"
        {
            crate::debug_log(&format!(
                "CTRL PIPE name={} payload={} sessions={}",
                pipe_message.name,
                pipe_message.payload.as_deref().unwrap_or("None"),
                self.state.sessions.len()
            ));
        }

        // Handle sidebar protocol messages BEFORE parse_pipe_message,
        // since parse_pipe_message returns Unknown for these names.
        match pipe_message.name.as_str() {
            "cc-deck:sidebar-hello" => {
                if let Some(payload) = pipe_message.payload.as_deref() {
                    if let Ok(hello) = serde_json::from_str::<SidebarHello>(payload) {
                        sidebar_registry::handle_sidebar_hello(&mut self.state, hello);
                    }
                }
                return false;
            }
            "cc-deck:action" => {
                if let Some(payload) = pipe_message.payload.as_deref() {
                    if let Ok(msg) = serde_json::from_str::<ActionMessage>(payload) {
                        actions::handle_action(&mut self.state, msg);
                        render_broadcast::flush_render(&mut self.state);
                    }
                }
                return false;
            }
            "cc-deck:render" | "cc-deck:sidebar-init" | "cc-deck:sidebar-reindex" => {
                // Controller ignores its own outbound broadcasts
                return false;
            }
            _ => {}
        }

        let action = parse_pipe_message(
            &pipe_message.name,
            pipe_message.payload.as_deref(),
        );

        // Unblock CLI pipe input so `zellij pipe` does not hang.
        // DumpState handles its own unblock after sending output.
        // VoiceControl holds the pipe open for PTT long-poll.
        #[cfg(target_family = "wasm")]
        if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
            if !matches!(action, PipeAction::DumpState | PipeAction::VoiceControl) {
                zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
            }
        }

        match action {
            PipeAction::HookEvent(hook) => {
                hooks::process_hook(&mut self.state, hook);
            }
            PipeAction::SyncState(_) | PipeAction::RequestState => {
                // Legacy sync messages: ignored in controller architecture.
                // The controller is the single writer; no peer sync needed.
            }
            PipeAction::Attend => {
                actions::handle_action(
                    &mut self.state,
                    ActionMessage {
                        action: cc_deck::ActionType::Attend,
                        pane_id: None,
                        tab_index: None,
                        value: None,
                        sidebar_plugin_id: 0,
                    },
                );
            }
            PipeAction::AttendPrev => {
                actions::handle_action(
                    &mut self.state,
                    ActionMessage {
                        action: cc_deck::ActionType::AttendPrev,
                        pane_id: None,
                        tab_index: None,
                        value: None,
                        sidebar_plugin_id: 0,
                    },
                );
            }
            PipeAction::Working => {
                actions::handle_action(
                    &mut self.state,
                    ActionMessage {
                        action: cc_deck::ActionType::Working,
                        pane_id: None,
                        tab_index: None,
                        value: None,
                        sidebar_plugin_id: 0,
                    },
                );
            }
            PipeAction::WorkingPrev => {
                actions::handle_action(
                    &mut self.state,
                    ActionMessage {
                        action: cc_deck::ActionType::WorkingPrev,
                        pane_id: None,
                        tab_index: None,
                        value: None,
                        sidebar_plugin_id: 0,
                    },
                );
            }
            PipeAction::Navigate | PipeAction::NavToggle => {
                let is_own_broadcast = matches!(
                    &pipe_message.source,
                    PipeSource::Plugin(id) if *id == self.state.plugin_id
                );
                if !is_own_broadcast {
                    broadcast_navigate(&self.state, "forward");
                }
            }
            PipeAction::NavigatePrev => {
                let is_own_broadcast = matches!(
                    &pipe_message.source,
                    PipeSource::Plugin(id) if *id == self.state.plugin_id
                );
                if !is_own_broadcast {
                    broadcast_navigate(&self.state, "backward");
                }
            }
            PipeAction::DumpState => {
                self.dump_state(&pipe_message);
            }
            PipeAction::RestoreMeta(payload) => {
                hooks::process_restore_meta(&mut self.state, &payload);
            }
            PipeAction::Refresh => {
                // Re-save current state (clears stale cache) and broadcast
                let _ = std::fs::remove_file("/cache/sessions.json");
                self.state.save_sessions();
                self.state.mark_render_dirty();
                crate::debug_log("CTRL REFRESH re-saved state and marked dirty");
            }
            PipeAction::NewSession => {
                actions::handle_action(
                    &mut self.state,
                    ActionMessage {
                        action: cc_deck::ActionType::NewSession,
                        pane_id: None,
                        tab_index: None,
                        value: None,
                        sidebar_plugin_id: 0,
                    },
                );
            }
            PipeAction::Pause => {
                // Pause from keybinding targets the focused pane
                if let Some(pid) = self.state.focused_pane_id {
                    actions::handle_action(
                        &mut self.state,
                        ActionMessage {
                            action: cc_deck::ActionType::Pause,
                            pane_id: Some(pid),
                            tab_index: None,
                            value: None,
                            sidebar_plugin_id: 0,
                        },
                    );
                }
            }
            PipeAction::Rename => {
                // Rename from keybinding targets the focused pane
                if let Some(pid) = self.state.focused_pane_id {
                    // The actual rename text comes from the sidebar UI.
                    // This keybinding just triggers navigation mode on the sidebar.
                    broadcast_navigate(&self.state, "forward");
                    let _ = pid; // Suppress unused warning
                }
            }
            PipeAction::Help => {
                // Help from keybinding: forward to sidebars
                broadcast_navigate(&self.state, "forward");
            }
            PipeAction::VoiceText(text) if !text.is_empty() => {
                let sessions = &self.state.sessions;
                let is_session = |id: &u32| sessions.contains_key(id);
                let target = self.state.last_attended_pane_id.filter(&is_session)
                    .or(self.state.focused_pane_id.filter(&is_session))
                    .or_else(|| sessions.keys().next().copied());
                if let Some(pane_id) = target {
                    write_chars_to_pane(pane_id, &text);
                    crate::debug_log(&format!(
                        "CTRL VOICE injected {} chars to pane={}",
                        text.len(), pane_id
                    ));
                } else {
                    crate::debug_log("CTRL VOICE discarded: no target pane");
                }
            }
            PipeAction::VoiceControl => {
                #[cfg(target_family = "wasm")]
                if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
                    self.state.voice_control_pipe = Some(pipe_id.clone());
                    self.state.voice_enabled = true;
                    crate::debug_log(&format!(
                        "CTRL VOICE-CONTROL pipe held: {}", pipe_id
                    ));
                }
                #[cfg(not(target_family = "wasm"))]
                {
                    self.state.voice_enabled = true;
                    crate::debug_log("CTRL VOICE-CONTROL enabled (non-wasm)");
                }
            }
            PipeAction::VoiceToggle => {
                if let Some(ref pipe_id) = self.state.voice_control_pipe.take() {
                    cli_pipe_output_wasm(pipe_id, "toggle");
                    unblock_cli_pipe_input_wasm(pipe_id);
                    crate::debug_log(&format!(
                        "CTRL VOICE-TOGGLE responded to pipe: {}", pipe_id
                    ));
                } else {
                    crate::debug_log("CTRL VOICE-TOGGLE ignored: no held pipe");
                }
            }
            PipeAction::TestInject => {
                // Diagnostic: inject hardcoded text into the focused pane.
                // Compares manifest-derived pane ID with tracked state to
                // isolate whether write_chars_to_pane_id fails due to a
                // wrong pane ID or the API itself.
                let manifest_focus = self.state.pane_manifest.as_ref().and_then(|m| {
                    self.state.tabs.iter().find(|t| t.active).and_then(|tab| {
                        m.panes.get(&tab.position).and_then(|panes| {
                            panes.iter()
                                .find(|p| !p.is_plugin && p.is_focused)
                                .map(|p| p.id)
                        })
                    })
                });
                let tracked_focus = self.state.focused_pane_id;
                let last_attended = self.state.last_attended_pane_id;
                let target = manifest_focus
                    .or(tracked_focus)
                    .or(last_attended);

                let debug_info = format!(
                    "manifest_focus={:?} tracked_focus={:?} last_attended={:?} target={:?}",
                    manifest_focus, tracked_focus, last_attended, target
                );
                crate::debug_log(&format!("CTRL TEST-INJECT {}", debug_info));

                if let Some(pane_id) = target {
                    write_chars_to_pane(pane_id, "VOICE_TEST ");
                    crate::debug_log(&format!(
                        "CTRL TEST-INJECT called write_chars_to_pane({})", pane_id
                    ));
                } else {
                    crate::debug_log("CTRL TEST-INJECT: no target pane found");
                }

                #[cfg(target_family = "wasm")]
                if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
                    cli_pipe_output_wasm(pipe_id, &debug_info);
                    unblock_cli_pipe_input_wasm(pipe_id);
                }
            }
            PipeAction::Unknown => {}
            _ => {
                // NavUp, NavDown, NavSelect, etc. are sidebar-local concerns.
                // The controller does not handle cursor movement; sidebars
                // manage their own navigation state.
            }
        }

        // Hook events and other pipe messages use coalesced rendering.
        // The 1s timer will flush. User-initiated actions (cc-deck:action)
        // flush immediately above via the early-return path.
        false // Controller has no UI
    }

    fn render(&mut self, _rows: usize, _cols: usize) {
        // Controller is headless: no rendering
    }
}

impl ControllerPlugin {
    /// Dispatch an event to the appropriate handler.
    fn handle_event_inner(&mut self, event: Event) {
        match event {
            Event::TabUpdate(tabs) => events::handle_tab_update(&mut self.state, tabs),
            Event::PaneUpdate(manifest) => {
                events::handle_pane_update(&mut self.state, manifest);
                // Clean up dead sidebars on pane manifest changes
                sidebar_registry::cleanup_dead_sidebars(&mut self.state);
            }
            Event::Timer(elapsed) => events::handle_timer(&mut self.state, elapsed),
            Event::RunCommandResult(exit_code, stdout, stderr, context) => {
                events::handle_run_command_result(
                    &mut self.state,
                    exit_code,
                    stdout,
                    stderr,
                    context,
                );
            }
            Event::CommandPaneOpened(terminal_pane_id, context) => {
                events::handle_command_pane_opened(
                    &mut self.state,
                    terminal_pane_id,
                    context,
                );
            }
            Event::PaneClosed(pane_id) => {
                events::handle_pane_closed(&mut self.state, pane_id);
            }
            _ => {}
        }
    }

    /// Serialize session state and send it via CLI pipe output.
    fn dump_state(&self, pipe_message: &PipeMessage) {
        #[derive(serde::Serialize)]
        struct DumpStateResponse<'a> {
            sessions: &'a std::collections::BTreeMap<u32, crate::session::Session>,
            attended_pane_id: Option<u32>,
        }
        let resp = DumpStateResponse {
            sessions: &self.state.sessions,
            attended_pane_id: self.state.last_attended_pane_id,
        };
        let _state_json = serde_json::to_string(&resp)
            .unwrap_or_else(|_| "{}".to_string());
        #[cfg(target_family = "wasm")]
        {
            if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
                zellij_tile::prelude::cli_pipe_output(pipe_id, &_state_json);
                zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
            }
        }
        // Suppress unused variable warning in non-wasm builds
        let _ = pipe_message;
        crate::debug_log(&format!(
            "CTRL DUMP-STATE responded with {} sessions",
            self.state.sessions.len()
        ));
    }
}

// --- WASM-gated helpers ---

/// Get this plugin's ID.
#[cfg(target_family = "wasm")]
fn get_plugin_id_wasm() -> u32 {
    zellij_tile::prelude::get_plugin_ids().plugin_id
}

#[cfg(not(target_family = "wasm"))]
fn get_plugin_id_wasm() -> u32 {
    0
}

/// Forward a navigate keybinding to sidebars via broadcast.
#[cfg(target_family = "wasm")]
fn broadcast_navigate(state: &ControllerState, direction: &str) {
    let payload = format!(
        r#"{{"active_tab_index":{},"direction":"{}"}}"#,
        state.active_tab_index.unwrap_or(0),
        direction
    );
    let mut msg = MessageToPlugin::new("cc-deck:navigate");
    msg.message_payload = Some(payload);
    pipe_message_to_plugin(msg);
}

#[cfg(not(target_family = "wasm"))]
fn broadcast_navigate(_state: &ControllerState, _direction: &str) {}

#[cfg(target_family = "wasm")]
fn write_chars_to_pane(pane_id: u32, chars: &str) {
    zellij_tile::prelude::write_chars_to_pane_id(chars, PaneId::Terminal(pane_id));
}

#[cfg(not(target_family = "wasm"))]
fn write_chars_to_pane(_pane_id: u32, _chars: &str) {}

#[cfg(target_family = "wasm")]
fn cli_pipe_output_wasm(pipe_id: &str, output: &str) {
    zellij_tile::prelude::cli_pipe_output(pipe_id, output);
}

#[cfg(not(target_family = "wasm"))]
fn cli_pipe_output_wasm(_pipe_id: &str, _output: &str) {}

#[cfg(target_family = "wasm")]
fn unblock_cli_pipe_input_wasm(pipe_id: &str) {
    zellij_tile::prelude::unblock_cli_pipe_input(pipe_id);
}

#[cfg(not(target_family = "wasm"))]
fn unblock_cli_pipe_input_wasm(_pipe_id: &str) {}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::{Activity, Session};

    #[test]
    fn test_controller_plugin_default() {
        let plugin = ControllerPlugin::default();
        assert!(plugin.state.sessions.is_empty());
        assert!(!plugin.state.permissions_granted);
        assert_eq!(plugin.state.plugin_id, 0);
    }
}
