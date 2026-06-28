// Controller hook processing: create/update sessions from CLI hook events.
//
// The controller is the single writer of session state. Hook events arrive
// via cc-deck:hook pipe messages from the CLI and are processed here to
// create new sessions, transition activity states, track CWD changes,
// and trigger git detection.

use super::state::{ControllerState, PendingOverride};
use crate::git;
use crate::pipe_handler::{hook_event_to_activity, is_session_end, HookPayload};
use crate::session::{self, Activity, Session};

/// Process a hook event from the CLI. Returns true if state changed visibly.
pub fn process_hook(state: &mut ControllerState, hook: HookPayload) -> bool {
    state.unconfirmed_pane_ids.remove(&hook.pane_id);

    // SessionEnd: remove the session only if the pane is actually gone.
    // Claude Code may fire SessionEnd transiently (e.g., during plugin
    // reinstall via `claude plugin install`) while the pane is still alive.
    // Check the manifest before removing to avoid false disappearances.
    if is_session_end(&hook.hook_event_name) {
        let pane_alive = state
            .pane_manifest
            .as_ref()
            .map(|m| {
                m.panes.values().flatten().any(|p| {
                    !p.is_plugin && p.id == hook.pane_id && !p.exited
                })
            })
            .unwrap_or(false);

        if pane_alive {
            // Pane still exists: transition to Idle instead of removing.
            if let Some(session) = state.sessions.get_mut(&hook.pane_id) {
                let changed = session.transition(Activity::Idle);
                if changed {
                    state.save_sessions();
                }
                state.mark_render_dirty();
                return changed;
            }
            return false;
        }

        let removed = state.sessions.remove(&hook.pane_id).is_some();
        if removed {
            if let Some(ref mut order) = state.sort_order {
                order.retain(|&p| p != hook.pane_id);
            }
            state.save_sessions();
            state.mark_render_dirty();
        }
        return removed;
    }

    // Map hook event to activity
    let activity = match hook_event_to_activity(&hook.hook_event_name, hook.tool_name.as_deref()) {
        Some(a) => a,
        None => {
            // Non-activity events (Notification, unknown): just refresh timestamp.
            // Notification fires after 6s of user inactivity (including while
            // a permission prompt is showing), so it does NOT indicate the
            // session has moved past the waiting state.
            if let Some(session) = state.sessions.get_mut(&hook.pane_id) {
                session.last_event_ts = session::unix_now();
            }
            return false;
        }
    };

    let is_new = !state.sessions.contains_key(&hook.pane_id);
    if is_new {
        state.sessions.insert(
            hook.pane_id,
            Session::new(
                hook.pane_id,
                hook.session_id.clone().unwrap_or_default(),
            ),
        );
    }

    // Detect session replacement: new Claude Code instance in same pane
    let session_replaced = !is_new
        && hook.session_id.as_ref().is_some_and(|new_sid| {
            state
                .sessions
                .get(&hook.pane_id)
                .map(|s| !s.session_id.is_empty() && s.session_id != *new_sid)
                .unwrap_or(false)
        });
    if session_replaced {
        if let Some(session) = state.sessions.get_mut(&hook.pane_id) {
            crate::debug_log(&format!(
                "CTRL SESSION replaced pane={}: {} -> {}",
                hook.pane_id,
                session.session_id,
                hook.session_id.as_deref().unwrap_or("?")
            ));
            session.manually_renamed = false;
            session.display_name = format!("session-{}", hook.pane_id);
            session.meta_ts = 0;
            session.done_attended = false;
            session.pending_permissions = 0;
            session.working_dir = None;
        }
    }

    // Skip updates for paused sessions
    if !is_new {
        if let Some(s) = state.sessions.get(&hook.pane_id) {
            if s.paused {
                return false;
            }
        }
    }

    // Permission counter management:
    // - PermissionRequest: increment pending_permissions, transition to Waiting
    // - PermissionReply: decrement pending_permissions, transition to Working only when 0
    // - Working (PreToolUse/PostToolUse): suppress while pending_permissions > 0
    // - Done/Stop: reset counter, allow transition
    if matches!(activity, Activity::Waiting(session::WaitReason::Permission)) {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.pending_permissions = s.pending_permissions.saturating_add(1);
            crate::debug_log(&format!(
                "CTRL HOOK: pane={} PermissionRequest, pending_permissions={}",
                hook.pane_id, s.pending_permissions,
            ));
        }
    }

    // PermissionReply: decrement counter. Only clear Waiting when all
    // permission prompts have been answered (counter reaches 0).
    if hook.hook_event_name == "PermissionReply" {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.pending_permissions = s.pending_permissions.saturating_sub(1);
            crate::debug_log(&format!(
                "CTRL HOOK: pane={} PermissionReply, pending_permissions={}",
                hook.pane_id, s.pending_permissions,
            ));
            if s.pending_permissions > 0 {
                // More prompts outstanding; stay in Waiting.
                s.last_event_ts = session::unix_now();
                return false;
            }
            // Counter reached 0: fall through to transition to Working.
        }
    }

    // Suppress Working transitions while permission prompts are outstanding.
    // For PostToolUse from main agent (no agent_id): treat as implicit
    // PermissionReply for backward compatibility with Claude Code, which
    // doesn't send explicit PermissionReply events.
    // For PreToolUse or subagent events: always suppress while waiting.
    if matches!(activity, Activity::Working) {
        if let Some(s) = state.sessions.get(&hook.pane_id) {
            if s.pending_permissions > 0 {
                let is_main_agent_post = hook.hook_event_name == "PostToolUse"
                    && hook.agent_id.is_none();
                if is_main_agent_post {
                    // Backward compat: PostToolUse from main agent acts as
                    // implicit PermissionReply (Claude Code flow).
                    if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
                        s.pending_permissions = s.pending_permissions.saturating_sub(1);
                        crate::debug_log(&format!(
                            "CTRL HOOK: pane={} PostToolUse as implicit PermissionReply, pending_permissions={}",
                            hook.pane_id, s.pending_permissions,
                        ));
                        if s.pending_permissions > 0 {
                            s.last_event_ts = session::unix_now();
                            return false;
                        }
                        // Counter reached 0: fall through to transition to Working.
                    }
                } else {
                    crate::debug_log(&format!(
                        "CTRL HOOK: pane={} suppressing Working while {} permissions pending (event={})",
                        hook.pane_id, s.pending_permissions, hook.hook_event_name,
                    ));
                    if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
                        s.last_event_ts = session::unix_now();
                    }
                    return false;
                }
            }
        }
    }

    // Reset permission counter on session-ending events.
    if matches!(activity, Activity::Done) {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.pending_permissions = 0;
        }
    }

    // Transition activity
    let prev_activity = state
        .sessions
        .get(&hook.pane_id)
        .map(|s| format!("{:?}", s.activity));
    let was_waiting = state
        .sessions
        .get(&hook.pane_id)
        .map(|s| s.activity.is_waiting())
        .unwrap_or(false);
    let changed = match state.sessions.get_mut(&hook.pane_id) {
        Some(s) => s.transition(activity),
        None => return false,
    };
    if changed {
        crate::debug_log(&format!(
            "CTRL HOOK: pane={} {} {:?}->{:?}",
            hook.pane_id, hook.hook_event_name,
            prev_activity.as_deref().unwrap_or("?"),
            state.sessions.get(&hook.pane_id).map(|s| format!("{:?}", s.activity)).unwrap_or_default()
        ));
    }
    if was_waiting && changed {
        crate::debug_log(&format!(
            "CTRL HOOK: pane={} left Waiting via {}",
            hook.pane_id, hook.hook_event_name,
        ));
    }
    if was_waiting && !changed {
        crate::debug_log(&format!(
            "CTRL HOOK: pane={} STUCK in Waiting, rejected {} transition",
            hook.pane_id, hook.hook_event_name,
        ));
    }

    // Update session_id
    if let Some(ref sid) = hook.session_id {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.session_id = sid.clone();
        }
    }

    // Store agent name and indicator from the first hook event
    if hook.agent.is_some() {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            if s.agent_name.is_none() {
                s.agent_name = hook.agent.clone();
                s.agent_indicator = hook.agent_indicator.clone();
            }
        }
    }

    // Process CWD changes
    if let Some(ref cwd) = hook.cwd {
        process_cwd_change(state, hook.pane_id, cwd);
    }

    // Store resolved badges from the hook payload
    if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
        s.badges = hook.badges.clone();
    }

    // Refresh tab info from pane map
    if let Some((idx, name)) = state.pane_to_tab.get(&hook.pane_id) {
        let (idx, name) = (*idx, name.clone());
        if let Some(session) = state.sessions.get_mut(&hook.pane_id) {
            session.tab_index = Some(idx);
            session.tab_name = Some(name);
        }
    }

    if changed {
        state.save_sessions();
    }
    state.mark_render_dirty();
    true
}

/// Process a CWD change for a session: apply pending overrides, auto-rename
/// from directory name, and trigger git detection.
fn process_cwd_change(state: &mut ControllerState, pane_id: u32, cwd: &str) {
    let is_worktree_cwd = cwd.contains("/.claude/");
    let cwd_changed = state
        .sessions
        .get(&pane_id)
        .map(|s| s.working_dir.as_deref() != Some(cwd))
        .unwrap_or(false);

    if !is_worktree_cwd && cwd_changed {
        if let Some(s) = state.sessions.get_mut(&pane_id) {
            s.working_dir = Some(cwd.to_string());
        }

        // Check for pending override from snapshot restore (FIFO per directory)
        let ovr = state
            .pending_overrides
            .get_mut(cwd)
            .and_then(|v| if v.is_empty() { None } else { Some(v.remove(0)) });

        // Clean up empty override entries
        if let Some(empty_key) = ovr.as_ref().and_then(|_| {
            state
                .pending_overrides
                .get(cwd)
                .filter(|v| v.is_empty())
                .map(|_| cwd.to_string())
        }) {
            state.pending_overrides.remove(&empty_key);
        }

        if let Some(ovr) = ovr {
            apply_override(state, pane_id, cwd, &ovr);
        } else if let Some(session) = state.sessions.get(&pane_id) {
            let needs_dir_name =
                !session.manually_renamed && session.display_name.starts_with("session-");
            let not_renamed = !session.manually_renamed;

            if needs_dir_name {
                let dir_name = std::path::Path::new(cwd)
                    .file_name()
                    .and_then(|n| n.to_str())
                    .unwrap_or("session")
                    .to_string();
                let names: Vec<String> = state
                    .sessions
                    .iter()
                    .filter(|(&id, _)| id != pane_id)
                    .map(|(_, s)| s.display_name.clone())
                    .collect();
                let name_refs: Vec<&str> = names.iter().map(|s| s.as_str()).collect();
                if let Some(s) = state.sessions.get_mut(&pane_id) {
                    s.display_name = session::deduplicate_name(&dir_name, &name_refs);
                }
                // Rename the Zellij tab if this is the sole session on it
                maybe_rename_tab(state, pane_id);
            }
            if not_renamed {
                git::detect_git_repo(pane_id, cwd);
            }
        }
    }

    if !is_worktree_cwd {
        git::detect_git_branch(pane_id, cwd);
    }
}

/// Apply a pending override from snapshot restore.
fn apply_override(state: &mut ControllerState, pane_id: u32, cwd: &str, ovr: &PendingOverride) {
    crate::debug_log(&format!(
        "CTRL RESTORE applying override for {cwd}: name={}",
        ovr.display_name
    ));
    let names: Vec<String> = state
        .sessions
        .iter()
        .filter(|(&id, _)| id != pane_id)
        .map(|(_, s)| s.display_name.clone())
        .collect();
    let name_refs: Vec<&str> = names.iter().map(|s| s.as_str()).collect();
    if let Some(s) = state.sessions.get_mut(&pane_id) {
        s.display_name = session::deduplicate_name(&ovr.display_name, &name_refs);
        s.manually_renamed = true;
        s.paused = ovr.paused;
        let now = session::unix_now();
        s.last_event_ts = now;
        s.meta_ts = now;
    }
    maybe_rename_tab(state, pane_id);
}

/// Process restore-meta payload: queue pending overrides keyed by working directory.
/// Called when `cc-deck:restore-meta` arrives from the CLI during snapshot restore.
pub fn process_restore_meta(state: &mut ControllerState, payload: &str) {
    if let Ok(map) =
        serde_json::from_str::<std::collections::HashMap<String, Vec<serde_json::Value>>>(payload)
    {
        for (dir, entries) in map {
            for val in entries {
                let name = val
                    .get("display_name")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_string();
                let paused = val
                    .get("paused")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                if !name.is_empty() {
                    state
                        .pending_overrides
                        .entry(dir.clone())
                        .or_default()
                        .push(PendingOverride {
                            display_name: name,
                            paused,
                        });
                }
            }
        }
        let total: usize = state.pending_overrides.values().map(|v| v.len()).sum();
        crate::debug_log(&format!(
            "CTRL RESTORE-META loaded {total} pending overrides"
        ));
    }
}

/// If the pane is the sole session on its tab, rename the Zellij tab to match.
fn maybe_rename_tab(state: &mut ControllerState, pane_id: u32) {
    if let Some(tab_idx) = state.sessions.get(&pane_id).and_then(|s| s.tab_index) {
        let sessions_on_tab = state
            .sessions
            .values()
            .filter(|s| s.tab_index == Some(tab_idx))
            .count();
        if sessions_on_tab == 1 {
            if let Some(s) = state.sessions.get(&pane_id) {
                crate::wasm_compat::rename_tab_wasm(tab_idx, &s.display_name);
            }
        }
    } else if let Some(s) = state.sessions.get_mut(&pane_id) {
        s.pending_tab_rename = true;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Activity;
    use zellij_tile::prelude::{PaneInfo, PaneManifest};

    fn make_hook(pane_id: u32, event: &str) -> HookPayload {
        HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test-session".to_string()),
            pane_id,
            hook_event_name: event.to_string(),
            tool_name: None,
            cwd: None,
            agent_id: None,
            badges: vec![],
        }
    }

    fn make_subagent_hook(pane_id: u32, event: &str) -> HookPayload {
        HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test-session".to_string()),
            pane_id,
            hook_event_name: event.to_string(),
            tool_name: None,
            cwd: None,
            agent_id: Some("sub-1".to_string()),
            badges: vec![],
        }
    }

    #[test]
    fn test_process_hook_creates_session() {
        let mut state = ControllerState::default();
        let hook = make_hook(42, "SessionStart");

        let changed = process_hook(&mut state, hook);
        assert!(changed);
        assert!(state.sessions.contains_key(&42));
        assert_eq!(state.sessions[&42].activity, Activity::Init);
    }

    #[test]
    fn test_process_hook_transitions_activity() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));

        let hook = make_hook(42, "PreToolUse");
        let changed = process_hook(&mut state, hook);
        assert!(changed);
        assert_eq!(state.sessions[&42].activity, Activity::Working);
    }

    fn make_pane_info(id: u32, is_plugin: bool, exited: bool) -> PaneInfo {
        PaneInfo {
            id,
            is_plugin,
            is_focused: false,
            is_fullscreen: false,
            is_floating: false,
            is_suppressed: false,
            title: String::new(),
            exited,
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

    fn make_manifest(panes_list: Vec<PaneInfo>) -> PaneManifest {
        let mut map = std::collections::HashMap::new();
        map.insert(0, panes_list);
        PaneManifest { panes: map }
    }

    #[test]
    fn test_process_hook_session_end_removes_when_pane_gone() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));
        // No manifest = pane not confirmed alive -> remove
        let hook = make_hook(42, "SessionEnd");
        let changed = process_hook(&mut state, hook);
        assert!(changed);
        assert!(!state.sessions.contains_key(&42));
    }

    #[test]
    fn test_process_hook_session_end_removes_when_pane_exited() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));
        state.pane_manifest = Some(make_manifest(vec![
            make_pane_info(42, false, true),
        ]));

        let hook = make_hook(42, "SessionEnd");
        let changed = process_hook(&mut state, hook);
        assert!(changed);
        assert!(!state.sessions.contains_key(&42));
    }

    #[test]
    fn test_process_hook_session_end_keeps_when_pane_alive() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.activity = Activity::Working;
        state.sessions.insert(42, s);
        state.pane_manifest = Some(make_manifest(vec![
            make_pane_info(42, false, false),
        ]));

        let hook = make_hook(42, "SessionEnd");
        let changed = process_hook(&mut state, hook);
        assert!(changed);
        // Session should still exist, transitioned to Idle
        assert!(state.sessions.contains_key(&42));
        assert_eq!(state.sessions[&42].activity, Activity::Idle);
    }

    #[test]
    fn test_process_hook_session_end_ignores_plugin_panes() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));
        // Pane 42 exists but as a plugin pane, not a terminal pane
        state.pane_manifest = Some(make_manifest(vec![
            make_pane_info(42, true, false),
        ]));

        let hook = make_hook(42, "SessionEnd");
        let changed = process_hook(&mut state, hook);
        assert!(changed);
        assert!(!state.sessions.contains_key(&42));
    }

    #[test]
    fn test_process_hook_paused_session_skipped() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.paused = true;
        s.activity = Activity::Idle;
        state.sessions.insert(42, s);

        let hook = make_hook(42, "PreToolUse");
        let changed = process_hook(&mut state, hook);
        assert!(!changed);
        assert_eq!(state.sessions[&42].activity, Activity::Idle);
    }

    #[test]
    fn test_process_hook_with_cwd() {
        let mut state = ControllerState::default();
        state
            .sessions
            .insert(42, Session::new(42, "test".into()));

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test".to_string()),
            pane_id: 42,
            hook_event_name: "PreToolUse".to_string(),
            tool_name: None,
            cwd: Some("/home/user/my-project".to_string()),
            agent_id: None,
            badges: vec![],
        };
        process_hook(&mut state, hook);
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/my-project")
        );
        // Auto-rename should have applied since display_name starts with "session-"
        assert_eq!(state.sessions[&42].display_name, "my-project");
    }

    #[test]
    fn test_process_hook_worktree_cwd_ignored() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.working_dir = Some("/home/user/project".to_string());
        state.sessions.insert(42, s);

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test".to_string()),
            pane_id: 42,
            hook_event_name: "PreToolUse".to_string(),
            tool_name: None,
            cwd: Some("/home/user/project/.claude/worktree".to_string()),
            agent_id: None,
            badges: vec![],
        };
        process_hook(&mut state, hook);
        // CWD should NOT change to the worktree path
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/project")
        );
    }

    #[test]
    fn test_process_hook_session_replacement() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "old-session".into());
        s.display_name = "my-project".to_string();
        s.manually_renamed = true;
        s.session_id = "old-session".to_string();
        state.sessions.insert(42, s);

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("new-session".to_string()),
            pane_id: 42,
            hook_event_name: "SessionStart".to_string(),
            tool_name: None,
            cwd: None,
            agent_id: None,
            badges: vec![],
        };
        process_hook(&mut state, hook);

        // Session should be reset (new Claude Code instance)
        assert!(!state.sessions[&42].manually_renamed);
        assert!(state.sessions[&42].display_name.starts_with("session-"));
    }

    #[test]
    fn test_pending_override_applied() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.display_name = "session-42".to_string();
        state.sessions.insert(42, s);

        // Set up a pending override
        state.pending_overrides.insert(
            "/home/user/api".to_string(),
            vec![PendingOverride {
                display_name: "api-server".to_string(),
                paused: true,
            }],
        );

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test".to_string()),
            pane_id: 42,
            hook_event_name: "PreToolUse".to_string(),
            tool_name: None,
            cwd: Some("/home/user/api".to_string()),
            agent_id: None,
            badges: vec![],
        };
        process_hook(&mut state, hook);

        assert_eq!(state.sessions[&42].display_name, "api-server");
        assert!(state.sessions[&42].manually_renamed);
        assert!(state.sessions[&42].paused);
        // Override should be consumed
        assert!(!state.pending_overrides.contains_key("/home/user/api"));
    }

    #[test]
    fn test_process_restore_meta() {
        let mut state = ControllerState::default();
        let payload = r#"{"/home/user/api":[{"display_name":"api-server","paused":false}],"/home/user/web":[{"display_name":"frontend","paused":true}]}"#;

        process_restore_meta(&mut state, payload);

        assert_eq!(state.pending_overrides.len(), 2);
        let api = &state.pending_overrides["/home/user/api"];
        assert_eq!(api.len(), 1);
        assert_eq!(api[0].display_name, "api-server");
        assert!(!api[0].paused);

        let web = &state.pending_overrides["/home/user/web"];
        assert_eq!(web.len(), 1);
        assert_eq!(web[0].display_name, "frontend");
        assert!(web[0].paused);
    }

    #[test]
    fn test_process_restore_meta_invalid_json() {
        let mut state = ControllerState::default();
        process_restore_meta(&mut state, "not valid json");
        assert!(state.pending_overrides.is_empty());
    }

    #[test]
    fn test_process_restore_meta_empty_names_skipped() {
        let mut state = ControllerState::default();
        let payload = r#"{"/tmp":[{"display_name":"","paused":false}]}"#;
        process_restore_meta(&mut state, payload);
        assert!(state.pending_overrides.is_empty());
    }

    #[test]
    fn test_process_restore_meta_multiple_per_dir() {
        let mut state = ControllerState::default();
        let payload = r#"{"/home/user/mono":[{"display_name":"api","paused":false},{"display_name":"worker","paused":true}]}"#;
        process_restore_meta(&mut state, payload);

        let overrides = &state.pending_overrides["/home/user/mono"];
        assert_eq!(overrides.len(), 2);
        assert_eq!(overrides[0].display_name, "api");
        assert_eq!(overrides[1].display_name, "worker");
        assert!(overrides[1].paused);
    }

    #[test]
    fn test_waiting_preserved_when_subagent_event() {
        let mut state = ControllerState::default();
        // Use PermissionRequest hook to enter Waiting and set counter.
        state.sessions.insert(42, Session::new(42, "test-session".into()));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        assert_eq!(state.sessions[&42].pending_permissions, 1);

        // PreToolUse from subagent should NOT clear Waiting
        let changed = process_hook(&mut state, make_subagent_hook(42, "PreToolUse"));
        assert!(!changed);
        assert_eq!(
            state.sessions[&42].activity,
            Activity::Waiting(crate::session::WaitReason::Permission)
        );

        // PostToolUse from subagent should NOT clear Waiting either
        let changed = process_hook(&mut state, make_subagent_hook(42, "PostToolUse"));
        assert!(!changed);
        assert_eq!(
            state.sessions[&42].activity,
            Activity::Waiting(crate::session::WaitReason::Permission)
        );
    }

    #[test]
    fn test_waiting_clears_on_main_agent_post_tool_use() {
        let mut state = ControllerState::default();
        // Enter Waiting via PermissionRequest (sets counter=1).
        state.sessions.insert(42, Session::new(42, "test-session".into()));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        assert_eq!(state.sessions[&42].pending_permissions, 1);

        // PostToolUse from main agent (no agent_id) acts as implicit
        // PermissionReply: decrements counter and clears Waiting.
        let changed = process_hook(&mut state, make_hook(42, "PostToolUse"));
        assert!(changed);
        assert_eq!(state.sessions[&42].activity, Activity::Working);
        assert_eq!(state.sessions[&42].pending_permissions, 0);
    }

    #[test]
    fn test_waiting_clears_on_main_agent_even_with_subagents_running() {
        let mut state = ControllerState::default();
        // Enter Waiting via PermissionRequest.
        state.sessions.insert(42, Session::new(42, "test-session".into()));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));

        // Subagent events arrive (don't clear Waiting)
        process_hook(&mut state, make_subagent_hook(42, "PreToolUse"));
        assert!(state.sessions[&42].activity.is_waiting());

        process_hook(&mut state, make_subagent_hook(42, "PostToolUse"));
        assert!(state.sessions[&42].activity.is_waiting());

        // Main agent PostToolUse arrives (clears Waiting via implicit PermissionReply)
        let changed = process_hook(&mut state, make_hook(42, "PostToolUse"));
        assert!(changed);
        assert_eq!(state.sessions[&42].activity, Activity::Working);
    }

    #[test]
    fn test_waiting_preserved_with_empty_agent_id() {
        let mut state = ControllerState::default();
        // Enter Waiting via PermissionRequest.
        state.sessions.insert(42, Session::new(42, "test-session".into()));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));

        // PostToolUse with empty agent_id (treated as subagent) should
        // NOT act as implicit PermissionReply.
        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test-session".to_string()),
            pane_id: 42,
            hook_event_name: "PostToolUse".to_string(),
            tool_name: None,
            cwd: None,
            agent_id: Some("".to_string()),
            badges: vec![],
        };
        let changed = process_hook(&mut state, hook);
        assert!(!changed);
        assert_eq!(
            state.sessions[&42].activity,
            Activity::Waiting(crate::session::WaitReason::Permission)
        );
    }

    #[test]
    fn test_parallel_permissions_not_cleared_by_pre_tool_use() {
        // OpenCode parallel tool calls: 2nd PreToolUse must NOT clear Waiting
        // set by 1st PermissionRequest.
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "test-session".into()));

        // Tool A fires PreToolUse
        process_hook(&mut state, make_hook(42, "PreToolUse"));
        assert_eq!(state.sessions[&42].activity, Activity::Working);

        // Tool A hits permission
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        assert!(state.sessions[&42].activity.is_waiting());
        assert_eq!(state.sessions[&42].pending_permissions, 1);

        // Tool B fires PreToolUse (parallel, no agent_id) - must NOT clear Waiting
        let changed = process_hook(&mut state, make_hook(42, "PreToolUse"));
        assert!(!changed);
        assert!(state.sessions[&42].activity.is_waiting());
    }

    #[test]
    fn test_parallel_permissions_both_must_be_replied() {
        // Two parallel PermissionRequests: both need PermissionReply to clear.
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "test-session".into()));

        // Two PermissionRequests
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        assert_eq!(state.sessions[&42].pending_permissions, 2);
        assert!(state.sessions[&42].activity.is_waiting());

        // First PermissionReply: counter decrements but stays in Waiting
        let changed = process_hook(&mut state, make_hook(42, "PermissionReply"));
        assert!(!changed);
        assert!(state.sessions[&42].activity.is_waiting());
        assert_eq!(state.sessions[&42].pending_permissions, 1);

        // Second PermissionReply: counter reaches 0, transitions to Working
        let changed = process_hook(&mut state, make_hook(42, "PermissionReply"));
        assert!(changed);
        assert_eq!(state.sessions[&42].activity, Activity::Working);
        assert_eq!(state.sessions[&42].pending_permissions, 0);
    }

    #[test]
    fn test_permission_counter_reset_on_done() {
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "test-session".into()));

        // Enter Waiting with 2 pending permissions
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        process_hook(&mut state, make_hook(42, "PermissionRequest"));
        assert_eq!(state.sessions[&42].pending_permissions, 2);

        // Stop event resets counter and transitions to Done
        process_hook(&mut state, make_hook(42, "Stop"));
        assert_eq!(state.sessions[&42].activity, Activity::Done);
        assert_eq!(state.sessions[&42].pending_permissions, 0);
    }

    #[test]
    fn test_post_tool_use_not_suppressed_when_no_pending_permissions() {
        // When pending_permissions is 0 (e.g. manual state or already cleared),
        // PostToolUse should transition normally.
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test-session".into());
        s.activity = Activity::Waiting(crate::session::WaitReason::Permission);
        s.pending_permissions = 0; // counter already at 0
        state.sessions.insert(42, s);

        let changed = process_hook(&mut state, make_hook(42, "PostToolUse"));
        assert!(changed);
        assert_eq!(state.sessions[&42].activity, Activity::Working);
    }

    #[test]
    fn test_process_hook_stores_agent_name() {
        let mut state = ControllerState::default();
        let mut hook = make_hook(42, "SessionStart");
        hook.agent = Some("claude".to_string());

        process_hook(&mut state, hook);
        assert_eq!(
            state.sessions[&42].agent_name,
            Some("claude".to_string())
        );
    }

    #[test]
    fn test_process_hook_agent_name_set_once() {
        let mut state = ControllerState::default();
        let mut hook1 = make_hook(42, "SessionStart");
        hook1.agent = Some("claude".to_string());
        process_hook(&mut state, hook1);

        let mut hook2 = make_hook(42, "PreToolUse");
        hook2.agent = Some("opencode".to_string());
        process_hook(&mut state, hook2);

        assert_eq!(
            state.sessions[&42].agent_name,
            Some("claude".to_string())
        );
    }
}
