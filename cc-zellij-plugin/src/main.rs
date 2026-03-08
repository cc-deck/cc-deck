#![allow(dead_code, unused_imports)]
// cc-deck v2: Zellij plugin for Claude Code session management
//
// Two instance modes (differentiated by config):
//   - sidebar: vertical session list on every tab (via tab_template)
//   - picker:  floating fuzzy search (via LaunchOrFocusPlugin)
//
// See brainstorm/08-cc-deck-v2-redesign.md for architecture details.

mod attend;
mod config;
mod git;
mod notification;
mod pipe_handler;
mod rename;
mod session;
mod sidebar;
mod state;
mod sync;

#[cfg(target_family = "wasm")]
fn debug_log(msg: &str) {
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open("/cache/debug.log")
    {
        use std::io::Write;
        let _ = writeln!(f, "{}", msg);
    }
}

#[cfg(not(target_family = "wasm"))]
fn debug_log(_msg: &str) {}

#[cfg(target_family = "wasm")]
fn set_selectable_wasm(selectable: bool) {
    zellij_tile::prelude::set_selectable(selectable);
}

#[cfg(not(target_family = "wasm"))]
fn set_selectable_wasm(_selectable: bool) {}

#[cfg(target_family = "wasm")]
fn create_new_session_tab() {
    // Create a new tab using the session's new_tab_template (respects user's layout)
    zellij_tile::prelude::new_tab(None::<&str>, None::<&str>);
    // Claude is started on the next TabUpdate via pending_claude_start flag
}

#[cfg(target_family = "wasm")]
fn start_claude_in_focused_pane() {
    zellij_tile::prelude::write_chars("claude\n");
}

#[cfg(not(target_family = "wasm"))]
fn start_claude_in_focused_pane() {}

#[cfg(not(target_family = "wasm"))]
fn create_new_session_tab() {}

#[cfg(target_family = "wasm")]
fn create_new_session_pane() {
    use std::collections::BTreeMap;
    let mut context = BTreeMap::new();
    context.insert("cc-deck".to_string(), "new-session".to_string());
    zellij_tile::prelude::open_command_pane(
        zellij_tile::prelude::CommandToRun {
            path: std::path::PathBuf::from("claude"),
            args: vec![],
            cwd: None,
        },
        context,
    );
}

#[cfg(not(target_family = "wasm"))]
fn create_new_session_pane() {}

#[cfg(target_family = "wasm")]
fn close_session_pane(pane_id: u32, tab_index: Option<usize>, is_only_pane: bool) {
    zellij_tile::prelude::close_terminal_pane(pane_id);
    if is_only_pane {
        if let Some(idx) = tab_index {
            zellij_tile::prelude::close_tab_with_index(idx);
        }
    }
}

#[cfg(not(target_family = "wasm"))]
fn close_session_pane(_pane_id: u32, _tab_index: Option<usize>, _is_only_pane: bool) {}

#[cfg(target_family = "wasm")]
fn auto_rename_tab(tab_idx: usize, name: &str) {
    zellij_tile::prelude::rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
fn auto_rename_tab(_tab_idx: usize, _name: &str) {}

use config::PluginConfig;
use git::GitResult;
use pipe_handler::{hook_event_to_activity, is_session_end, parse_pipe_message, PipeAction};
use session::Session;
use state::{PluginMode, PluginState};
use std::collections::BTreeMap;
use zellij_tile::prelude::*;

register_plugin!(PluginState);

impl ZellijPlugin for PluginState {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        debug_log("LOAD start");

        self.mode = match configuration.get("mode").map(|s| s.as_str()) {
            Some("picker") => PluginMode::Picker,
            _ => PluginMode::Sidebar,
        };

        self.config = PluginConfig::from_configuration(&configuration);

        debug_log("LOAD subscribing");
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

        debug_log("LOAD requesting permissions");
        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
        ]);

        debug_log("LOAD setting timeout");
        set_timeout(self.config.timer_interval);

        debug_log("LOAD complete");
    }

    fn update(&mut self, event: Event) -> bool {
        if let Event::PermissionRequestResult(status) = event {
            debug_log(&format!("PERMISSION result={status:?}"));
            if status == PermissionStatus::Granted {
                self.permissions_granted = true;
                debug_log("PERMISSION granted, calling set_selectable(false)");
                set_selectable(false);
                sync::request_state();
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
        // Trace log
        debug_log(&format!("PIPE name={} payload={} sessions={} pane_keys={:?}",
            pipe_message.name,
            pipe_message.payload.as_deref().unwrap_or("None"),
            self.sessions.len(),
            self.pane_to_tab.keys().collect::<Vec<_>>()));

        let action = parse_pipe_message(
            &pipe_message.name,
            pipe_message.payload.as_deref(),
        );

        match action {
            PipeAction::HookEvent(hook) => {
                if is_session_end(&hook.hook_event_name) {
                    // Get tab info before removing the session
                    let session_info = self.sessions.get(&hook.pane_id).map(|s| {
                        let tab_idx = s.tab_index;
                        let is_only = tab_idx.map(|idx| {
                            self.sessions.values()
                                .filter(|s2| s2.tab_index == Some(idx))
                                .count() <= 1
                        }).unwrap_or(false);
                        (tab_idx, is_only)
                    });

                    let removed = self.sessions.remove(&hook.pane_id).is_some();
                    if removed {
                        // Close the command pane (and tab if it was the only session)
                        if let Some((tab_idx, is_only)) = session_info {
                            close_session_pane(hook.pane_id, tab_idx, is_only);
                        }
                        sync::broadcast_state(self);
                    }
                    return removed;
                }

                let activity = match hook_event_to_activity(
                    &hook.hook_event_name,
                    hook.tool_name.as_deref(),
                ) {
                    Some(a) => a,
                    None => {
                        if let Some(session) = self.sessions.get_mut(&hook.pane_id) {
                            session.last_event_ts = session::unix_now();
                        }
                        return false;
                    }
                };

                let session = self.sessions.entry(hook.pane_id).or_insert_with(|| {
                    Session::new(
                        hook.pane_id,
                        hook.session_id.clone().unwrap_or_default(),
                    )
                });

                let changed = session.transition(activity);

                if let Some(ref sid) = hook.session_id {
                    session.session_id = sid.clone();
                }
                if let Some(ref cwd) = hook.cwd {
                    if session.working_dir.as_deref() != Some(cwd) {
                        session.working_dir = Some(cwd.clone());
                        if !session.manually_renamed {
                            git::detect_git_repo(hook.pane_id, cwd);
                            git::detect_git_branch(hook.pane_id, cwd);
                        }
                    }
                }

                if let Some((idx, name)) = self.pane_to_tab.get(&hook.pane_id) {
                    let session = self.sessions.get_mut(&hook.pane_id).unwrap();
                    session.tab_index = Some(*idx);
                    session.tab_name = Some(name.clone());
                }

                if changed {
                    sync::broadcast_state(self);
                }
                true
            }

            PipeAction::SyncState(payload) => sync::handle_sync(self, &payload),

            PipeAction::RequestState => {
                sync::broadcast_state(self);
                false
            }

            PipeAction::Attend => {
                match attend::perform_attend(self) {
                    attend::AttendResult::Switched { display_name, .. } => {
                        self.notification = Some(notification::create_notification(
                            &format!(">> {display_name}"),
                            3,
                        ));
                    }
                    attend::AttendResult::NoneWaiting => {
                        self.notification = Some(notification::create_notification(
                            "No sessions waiting",
                            3,
                        ));
                    }
                }
                true
            }

            PipeAction::Rename => {
                if let Some(rs) = rename::start_rename(self) {
                    self.rename_state = Some(rs);
                    set_selectable_wasm(true);
                    true
                } else {
                    self.notification = Some(notification::create_notification(
                        "No session to rename",
                        3,
                    ));
                    true
                }
            }

            PipeAction::NewSession => {
                match self.config.new_session_mode {
                    config::NewSessionMode::Tab => {
                        create_new_session_tab();
                        self.pending_claude_start = true;
                    }
                    config::NewSessionMode::Pane => create_new_session_pane(),
                }
                self.notification = Some(notification::create_notification(
                    "Creating session...",
                    2,
                ));
                true
            }

            PipeAction::Unknown => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self.mode {
            PluginMode::Sidebar => {
                self.click_regions = sidebar::render_sidebar(self, rows, cols);

                // Render notification on the last row if active
                if let Some(ref notif) = self.notification {
                    if notification::is_expired(notif) {
                        self.notification = None;
                    } else if rows > 0 {
                        notification::render_notification(notif, rows - 1, cols);
                    }
                }
            }
            PluginMode::Picker => {
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
                    self.updating_tabs = false;
                    return false;
                }
                let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);
                self.active_tab_index = new_active;
                self.tabs = tabs;
                self.rebuild_pane_map();
                self.remove_dead_sessions();

                // Start claude in the new tab after it's created
                if self.pending_claude_start {
                    self.pending_claude_start = false;
                    start_claude_in_focused_pane();
                }

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
                // Re-render to update elapsed times
                stale || !self.sessions.is_empty()
            }
            Event::Mouse(Mouse::LeftClick(row, col)) => {
                debug_log(&format!("CLICK row={row} col={col} regions={:?}", self.click_regions));
                if let Some((tab_idx, pane_id)) = sidebar::handle_click(row as usize, &self.click_regions) {
                    debug_log(&format!("CLICK tab_idx={tab_idx} pane_id={pane_id}"));
                    if pane_id == u32::MAX {
                        // [+] New session button clicked
                        match self.config.new_session_mode {
                            config::NewSessionMode::Tab => {
                                create_new_session_tab();
                                self.pending_claude_start = true;
                            }
                            config::NewSessionMode::Pane => create_new_session_pane(),
                        }
                        self.notification = Some(notification::create_notification(
                            "Creating session...",
                            2,
                        ));
                    } else {
                        // Switch tab if on a different tab, then focus the pane
                        if self.active_tab_index != Some(tab_idx) {
                            switch_tab_to(tab_idx as u32 + 1);
                        }
                        focus_terminal_pane(pane_id, false);
                    }
                }
                false
            }
            Event::Mouse(mouse) => {
                debug_log(&format!("MOUSE event={mouse:?}"));
                false
            }
            Event::Key(key) => {
                if let Some(ref mut rs) = self.rename_state {
                    match rename::handle_key(rs, key) {
                        rename::RenameAction::Continue => true,
                        rename::RenameAction::Confirm(new_name) => {
                            let pane_id = rs.pane_id;
                            self.rename_state = None;
                            set_selectable_wasm(false);
                            rename::complete_rename(self, pane_id, new_name);
                            true
                        }
                        rename::RenameAction::Cancel => {
                            self.rename_state = None;
                            set_selectable_wasm(false);
                            true
                        }
                    }
                } else {
                    false
                }
            }
            Event::RunCommandResult(exit_code, stdout, _stderr, context) => {
                self.handle_git_result(exit_code, stdout, context)
            }
            Event::CommandPaneOpened(terminal_pane_id, context) => {
                // Check if this was created by cc-deck
                if context.get("cc-deck").map(|v| v.as_str()) == Some("new-session") {
                    let session = Session::new(terminal_pane_id, String::new());
                    self.sessions.insert(terminal_pane_id, session);
                    // Trigger git detection for auto-naming
                    if let Some(cwd) = std::env::current_dir().ok().and_then(|p| p.to_str().map(String::from)) {
                        git::detect_git_repo(terminal_pane_id, &cwd);
                        git::detect_git_branch(terminal_pane_id, &cwd);
                    }
                    sync::broadcast_state(self);
                    true
                } else {
                    false
                }
            }
            Event::PaneClosed(pane_id) => {
                let id = match pane_id {
                    PaneId::Terminal(id) => id,
                    PaneId::Plugin(_) => return false,
                };
                let removed = self.sessions.remove(&id).is_some();
                if removed {
                    sync::broadcast_state(self);
                }
                removed
            }
            _ => false,
        }
    }

    fn handle_git_result(
        &mut self,
        exit_code: Option<i32>,
        stdout: Vec<u8>,
        context: BTreeMap<String, String>,
    ) -> bool {
        match git::parse_git_result(exit_code, stdout, context) {
            GitResult::RepoDetected { pane_id, repo_path } => {
                let should_rename = self
                    .sessions
                    .get(&pane_id)
                    .map(|s| !s.manually_renamed)
                    .unwrap_or(false);

                if should_rename {
                    let repo_name = git::repo_name_from_path(&repo_path).to_string();
                    let names = self.session_names_except(pane_id);
                    let new_name = session::deduplicate_name(&repo_name, &names);

                    if let Some(session) = self.sessions.get_mut(&pane_id) {
                        session.display_name = new_name.clone();
                    }

                    // Auto-rename tab if this is the only session on it
                    if let Some(tab_idx) = self.sessions.get(&pane_id).and_then(|s| s.tab_index) {
                        let sessions_on_tab = self.sessions.values()
                            .filter(|s| s.tab_index == Some(tab_idx))
                            .count();
                        if sessions_on_tab == 1 {
                            auto_rename_tab(tab_idx, &new_name);
                            self.updating_tabs = true;
                        }
                    }

                    sync::broadcast_state(self);
                    true
                } else {
                    false
                }
            }
            GitResult::BranchDetected { pane_id, branch } => {
                if let Some(session) = self.sessions.get_mut(&pane_id) {
                    session.git_branch = Some(branch);
                    true
                } else {
                    false
                }
            }
            GitResult::NotGit => false,
        }
    }

    fn session_names_except(&self, exclude_pane_id: u32) -> Vec<&str> {
        self.sessions
            .iter()
            .filter(|(&id, _)| id != exclude_pane_id)
            .map(|(_, s)| s.display_name.as_str())
            .collect()
    }
}
