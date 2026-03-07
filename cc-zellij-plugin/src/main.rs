#![allow(dead_code, unused_imports)]
// cc-deck v2: Zellij plugin for Claude Code session management
//
// Two instance modes (differentiated by config):
//   - sidebar: vertical session list on every tab (via tab_template)
//   - picker:  floating fuzzy search (via LaunchOrFocusPlugin)
//
// See brainstorm/08-cc-deck-v2-redesign.md for architecture details.

mod config;
mod git;
mod pipe_handler;
mod session;
mod sidebar;
mod state;
mod sync;

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
        self.mode = match configuration.get("mode").map(|s| s.as_str()) {
            Some("picker") => PluginMode::Picker,
            _ => PluginMode::Sidebar,
        };

        self.config = PluginConfig::from_configuration(&configuration);

        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
            PermissionType::RunCommands,
            PermissionType::ReadCliPipes,
            PermissionType::MessageAndLaunchOtherPlugins,
            PermissionType::Reconfigure,
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
            EventType::CommandPaneOpened,
            EventType::PaneClosed,
        ]);

        set_timeout(self.config.timer_interval);
    }

    fn update(&mut self, event: Event) -> bool {
        if let Event::PermissionRequestResult(status) = event {
            if status == PermissionStatus::Granted {
                self.permissions_granted = true;
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
        let action = parse_pipe_message(
            &pipe_message.name,
            pipe_message.payload.as_deref(),
        );

        match action {
            PipeAction::HookEvent(hook) => {
                if is_session_end(&hook.hook_event) {
                    let removed = self.sessions.remove(&hook.pane_id).is_some();
                    if removed {
                        sync::broadcast_state(self);
                    }
                    return removed;
                }

                let activity = match hook_event_to_activity(
                    &hook.hook_event,
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
                // TODO (Phase 6/US4): implement attend action
                false
            }

            PipeAction::Rename => {
                // TODO (Phase 7/US5): implement rename action
                false
            }

            PipeAction::NewSession => {
                // TODO (Phase 8/US6): implement new session creation
                false
            }

            PipeAction::Unknown => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        match self.mode {
            PluginMode::Sidebar => {
                self.click_regions = sidebar::render_sidebar(self, rows, cols);
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
                    return false;
                }
                let new_active = tabs.iter().find(|t| t.active).map(|t| t.position);
                self.active_tab_index = new_active;
                self.tabs = tabs;
                self.rebuild_pane_map();
                self.remove_dead_sessions();
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
            Event::Mouse(Mouse::LeftClick(row, _col)) => {
                if let Some(tab_idx) = sidebar::handle_click(row as usize, &self.click_regions) {
                    // switch_tab_to is 1-indexed
                    switch_tab_to(tab_idx as u32 + 1);
                }
                false
            }
            Event::Mouse(_) => false,
            Event::Key(_key) => {
                // TODO (Phase 7/US5): handle rename key input
                false
            }
            Event::RunCommandResult(exit_code, stdout, _stderr, context) => {
                self.handle_git_result(exit_code, stdout, context)
            }
            Event::CommandPaneOpened(_pane_id, _context) => {
                // TODO (Phase 8/US6): handle new session creation
                false
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
                        session.display_name = new_name;
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
