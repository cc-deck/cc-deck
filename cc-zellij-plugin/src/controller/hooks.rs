// Controller hook processing: create/update sessions from CLI hook events.
//
// The controller is the single writer of session state. Hook events arrive
// via cc-deck:hook pipe messages from the CLI and are processed here to
// create new sessions, transition activity states, track CWD changes,
// and trigger git detection.

use super::state::{ControllerState, PendingOverride, QuarantineKind};
use crate::git;
use crate::pipe_handler::{hook_event_to_activity, is_session_end, HookPayload};
use crate::session::{self, Activity, HookOutcome, Session};

/// Process a hook event from the CLI. Returns true if state changed visibly.
pub fn process_hook(state: &mut ControllerState, hook: HookPayload) -> bool {
    // A hook is evidence of a claim, never proof of it. Only the pane manifest
    // confirms that a pane exists, so a hook may release a pane from quarantine
    // solely when the manifest agrees. Clearing unconditionally let a phantom
    // session promote itself simply by continuing to fire events.
    if state.pane_is_live(hook.pane_id) == Some(true) {
        state.confirm_pane(hook.pane_id);
    }

    // SessionEnd: remove the session only if the pane is actually gone.
    // Claude Code may fire SessionEnd transiently (e.g., during plugin
    // reinstall via `claude plugin install`) while the pane is still alive.
    // Check the manifest before removing to avoid false disappearances.
    if is_session_end(&hook.hook_event_name) {
        // An unknown manifest still counts as "not alive" here, preserving the
        // established bias toward removal on SessionEnd.
        let pane_alive = state.pane_is_live(hook.pane_id).unwrap_or(false);

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

        let removed = state.evict_session(hook.pane_id);
        if removed {
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
        // Hold the new session back until the manifest vouches for its pane.
        // Both "the manifest says absent" and "there is no manifest" quarantine,
        // because a pane racing its own PaneUpdate is indistinguishable from a
        // pane that never existed at this instant. `sweep_quarantine` settles it.
        //
        // The session is still created and still runs the full state machine.
        // Quarantine gates visibility and persistence, not participation.
        if state.pane_is_live(hook.pane_id) != Some(true) {
            state.quarantine(hook.pane_id, QuarantineKind::Hook);
        }
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
            let same_agent = hook
                .agent
                .as_deref()
                .map(|a| session.agent_name.as_deref() == Some(a))
                .unwrap_or(true);

            if same_agent {
                crate::debug_log(&format!(
                    "CTRL SESSION replaced pane={}: {} -> {} (manually_renamed={})",
                    hook.pane_id,
                    session.session_id,
                    hook.session_id.as_deref().unwrap_or("?"),
                    session.manually_renamed,
                ));
                if !session.manually_renamed {
                    session.display_name = format!("session-{}", hook.pane_id);
                }
                session.meta_ts = 0;
                session.done_attended = false;
                session.pending_permissions = 0;
                session.working_dir = None;
                session.in_worktree = false;
                session.agent_name = None;
                session.agent_indicator = None;
                session.profile = None;
                session.profile_color = None;
            } else {
                crate::debug_log(&format!(
                    "CTRL SESSION cross-agent pane={}: {} -> {} (no state reset)",
                    hook.pane_id,
                    session.agent_name.as_deref().unwrap_or("?"),
                    hook.agent.as_deref().unwrap_or("?"),
                ));
                // Update agent identity without resetting session state.
                session.agent_name = hook.agent.clone();
                session.agent_indicator = hook.agent_indicator.clone();
                session.profile = hook.profile.clone();
                if let Some(ref color_str) = hook.profile_color {
                    session.profile_color = parse_hex_color(color_str);
                }
            }
            // Always update session_id to prevent repeated replacement
            // detection on subsequent hooks from the same new session.
            if let Some(ref sid) = hook.session_id {
                session.session_id = sid.clone();
            }
        }
    }

    // Skip updates for paused sessions, but always allow session_id to be
    // updated (handled above) to prevent replacement detection spirals.
    if !is_new {
        if let Some(s) = state.sessions.get(&hook.pane_id) {
            if s.paused {
                // Still update session_id even for paused sessions to prevent
                // the replacement detection from firing on every subsequent hook.
                if let Some(ref sid) = hook.session_id {
                    if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
                        s.session_id = sid.clone();
                    }
                }
                return false;
            }
        }
    }

    // Apply the event to the session's activity. The permission counter,
    // the implicit PermissionReply carried by a main-agent PostToolUse, and
    // the Waiting guard all live in Session::apply_hook so they can be
    // tested and fuzzed without a controller.
    let from_subagent = hook.agent_id.is_some();
    let (prev_activity, was_waiting) = match state.sessions.get(&hook.pane_id) {
        Some(s) => (format!("{:?}", s.activity), s.activity.is_waiting()),
        None => return false,
    };
    let outcome = match state.sessions.get_mut(&hook.pane_id) {
        Some(s) => s.apply_hook(&hook.hook_event_name, activity, from_subagent),
        None => return false,
    };
    let changed = match outcome {
        HookOutcome::Changed => true,
        HookOutcome::Unchanged => false,
        HookOutcome::Absorbed => {
            crate::debug_log(&format!(
                "CTRL HOOK: pane={} {} absorbed while {} permissions pending",
                hook.pane_id,
                hook.hook_event_name,
                state
                    .sessions
                    .get(&hook.pane_id)
                    .map(|s| s.pending_permissions)
                    .unwrap_or(0),
            ));
            return false;
        }
    };
    if changed {
        crate::debug_log(&format!(
            "CTRL HOOK: pane={} {} {}->{:?}",
            hook.pane_id,
            hook.hook_event_name,
            prev_activity,
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

    // Store profile name and color from the first hook carrying them.
    // Update profile_color on subsequent hooks if the color changed.
    if hook.profile.is_some() {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            if s.profile.is_none() {
                s.profile = hook.profile.clone();
            }
            // Always update color in case the config changed between hooks.
            if let Some(ref color_str) = hook.profile_color {
                s.profile_color = parse_hex_color(color_str);
            }
            // The indicator carries the profile icon override, so it follows
            // config changes the same way the color does.
            if hook.agent_indicator.is_some() {
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
    let is_worktree_path = cwd.contains("/.claude/worktrees/");
    let is_suppressed_claude_dir = cwd.contains("/.claude/") && !is_worktree_path;
    let cwd_changed = state
        .sessions
        .get(&pane_id)
        .map(|s| s.working_dir.as_deref() != Some(cwd))
        .unwrap_or(false);

    if !is_suppressed_claude_dir && cwd_changed {
        if let Some(s) = state.sessions.get_mut(&pane_id) {
            s.working_dir = Some(cwd.to_string());
            s.in_worktree = is_worktree_path;

            if is_worktree_path {
                // Use the project name for worktree sessions.
                // The branch is already shown on line 2 via git_branch.
                if s.display_name.starts_with("session-") {
                    if let Some(project_name) = std::path::Path::new(cwd)
                        .ancestors()
                        .nth(3)
                        .and_then(|p| p.file_name())
                        .and_then(|n| n.to_str())
                    {
                        s.display_name = project_name.to_string();
                    }
                }
            }
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

            if needs_dir_name && !is_worktree_path {
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

    if !is_suppressed_claude_dir {
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
        if let Some(ref p) = ovr.profile {
            s.profile = Some(p.clone());
        }
        if let Some(ref pc) = ovr.profile_color {
            s.profile_color = parse_hex_color(pc);
        }
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
                let profile = val
                    .get("profile")
                    .and_then(|v| v.as_str())
                    .filter(|s| !s.is_empty())
                    .map(|s| s.to_string());
                let profile_color = val
                    .get("profile_color")
                    .and_then(|v| v.as_str())
                    .filter(|s| !s.is_empty())
                    .map(|s| s.to_string());
                if !name.is_empty() {
                    state
                        .pending_overrides
                        .entry(dir.clone())
                        .or_default()
                        .push(PendingOverride {
                            display_name: name,
                            paused,
                            profile,
                            profile_color,
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

/// Parse a #RRGGBB color string into an (r, g, b) tuple.
/// Returns None for malformed input.
fn parse_hex_color(s: &str) -> Option<(u8, u8, u8)> {
    let s = s.strip_prefix('#').unwrap_or(s);
    if s.len() != 6 {
        return None;
    }
    let r = u8::from_str_radix(&s[0..2], 16).ok()?;
    let g = u8::from_str_radix(&s[2..4], 16).ok()?;
    let b = u8::from_str_radix(&s[4..6], 16).ok()?;
    Some((r, g, b))
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
            profile: None,
            profile_color: None,
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
            profile: None,
            profile_color: None,
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

    // ---------------------------------------------------------------------
    // Quarantine: a hook names a pane, only the manifest confirms it exists.
    // ---------------------------------------------------------------------

    /// The live bug. An agent running outside Zellij reached the sidebar with a
    /// pane id from an earlier run. The manifest is populated and does not
    /// contain that pane, so the session must not survive.
    #[test]
    fn test_hook_for_pane_absent_from_manifest_is_evicted() {
        let mut state = ControllerState::default();
        state.pane_manifest = Some(make_manifest(vec![make_pane_info(10, false, false)]));

        process_hook(&mut state, make_hook(99, "SessionStart"));

        assert!(state.sessions.contains_key(&99), "session is created first");
        assert_eq!(
            state.unconfirmed_panes.get(&99).map(|q| q.kind),
            Some(QuarantineKind::Hook),
            "a pane the manifest does not show must be quarantined"
        );
        assert!(state.is_hidden(99), "quarantined sessions are withheld");

        // Let the deadline pass.
        state.unconfirmed_panes.get_mut(&99).unwrap().deadline_ms = 0;
        assert!(state.sweep_quarantine());

        assert!(!state.sessions.contains_key(&99), "the phantom must be gone");
        assert!(state.unconfirmed_panes.is_empty());
    }

    #[test]
    fn test_hook_for_live_pane_is_not_quarantined() {
        let mut state = ControllerState::default();
        state.pane_manifest = Some(make_manifest(vec![make_pane_info(42, false, false)]));

        process_hook(&mut state, make_hook(42, "SessionStart"));

        assert!(state.sessions.contains_key(&42));
        assert!(
            state.unconfirmed_panes.is_empty(),
            "the happy path must not be delayed"
        );
        assert!(!state.is_hidden(42));
    }

    #[test]
    fn test_hook_for_exited_pane_is_quarantined() {
        let mut state = ControllerState::default();
        state.pane_manifest = Some(make_manifest(vec![make_pane_info(42, false, true)]));

        process_hook(&mut state, make_hook(42, "SessionStart"));

        assert_eq!(
            state.unconfirmed_panes.get(&42).map(|q| q.kind),
            Some(QuarantineKind::Hook)
        );
    }

    /// No manifest means unknown, never rejected. The session is still created
    /// and still runs its state machine; only its visibility is deferred.
    #[test]
    fn test_hook_without_manifest_is_quarantined_not_rejected() {
        let mut state = ControllerState::default();
        assert!(state.pane_manifest.is_none());

        process_hook(&mut state, make_hook(42, "SessionStart"));

        assert!(state.sessions.contains_key(&42), "unknown must not reject");
        assert!(state.unconfirmed_panes.contains_key(&42));
    }

    /// A hook is evidence of a claim, not proof of it. Without a manifest to
    /// agree, a phantom must not be able to promote itself by firing events.
    #[test]
    fn test_hook_does_not_self_confirm_without_manifest() {
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "restored".into()));
        state.quarantine(42, QuarantineKind::Restored);

        process_hook(&mut state, make_hook(42, "PreToolUse"));

        assert!(
            state.unconfirmed_panes.contains_key(&42),
            "only the manifest may confirm a pane"
        );
    }

    #[test]
    fn test_hook_confirms_when_manifest_agrees() {
        let mut state = ControllerState::default();
        state.sessions.insert(42, Session::new(42, "restored".into()));
        state.quarantine(42, QuarantineKind::Restored);
        state.pane_manifest = Some(make_manifest(vec![make_pane_info(42, false, false)]));

        process_hook(&mut state, make_hook(42, "PreToolUse"));

        assert!(state.unconfirmed_panes.is_empty());
    }

    #[test]
    fn test_session_end_clears_quarantine_entry() {
        let mut state = ControllerState::default();
        state.pane_manifest = Some(make_manifest(vec![make_pane_info(10, false, false)]));
        process_hook(&mut state, make_hook(99, "SessionStart"));
        assert!(state.unconfirmed_panes.contains_key(&99));

        process_hook(&mut state, make_hook(99, "SessionEnd"));

        assert!(!state.sessions.contains_key(&99));
        assert!(
            state.unconfirmed_panes.is_empty(),
            "a removed session must not leave an orphan quarantine entry"
        );
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
            profile: None,
            profile_color: None,
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
    fn test_process_hook_claude_internal_cwd_suppressed() {
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
            profile: None,
            profile_color: None,
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
            profile: None,
            profile_color: None,
        };
        process_hook(&mut state, hook);

        // Manual rename is preserved across session replacement
        assert!(state.sessions[&42].manually_renamed);
        assert_eq!(state.sessions[&42].display_name, "my-project");
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
                profile: None,
                profile_color: None,
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
            profile: None,
            profile_color: None,
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
            profile: None,
            profile_color: None,
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

    #[test]
    fn test_session_replacement_resets_agent_name() {
        let mut state = ControllerState::default();

        let mut hook1 = make_hook(42, "SessionStart");
        hook1.agent = Some("claude".to_string());
        hook1.agent_indicator = Some("\u{2733}".to_string());
        hook1.session_id = Some("session-a".to_string());
        process_hook(&mut state, hook1);

        assert_eq!(state.sessions[&42].agent_name, Some("claude".to_string()));

        let mut hook2 = make_hook(42, "SessionStart");
        hook2.agent = Some("codex".to_string());
        hook2.agent_indicator = Some("\u{25c6}".to_string());
        hook2.session_id = Some("session-b".to_string());
        process_hook(&mut state, hook2);

        assert_eq!(state.sessions[&42].agent_name, Some("codex".to_string()));
        assert_eq!(
            state.sessions[&42].agent_indicator,
            Some("\u{25c6}".to_string())
        );
    }

    #[test]
    fn test_session_replacement_preserves_manual_rename() {
        let mut state = ControllerState::default();

        let mut hook1 = make_hook(42, "SessionStart");
        hook1.session_id = Some("session-a".to_string());
        process_hook(&mut state, hook1);

        state.sessions.get_mut(&42).unwrap().display_name = "callum-gordon".to_string();
        state.sessions.get_mut(&42).unwrap().manually_renamed = true;

        let mut hook2 = make_hook(42, "SessionStart");
        hook2.session_id = Some("session-b".to_string());
        process_hook(&mut state, hook2);

        assert_eq!(state.sessions[&42].display_name, "callum-gordon");
        assert!(state.sessions[&42].manually_renamed);
    }

    #[test]
    fn test_cross_agent_no_replacement_reset() {
        let mut state = ControllerState::default();

        let mut hook1 = make_hook(42, "SessionStart");
        hook1.agent = Some("claude".to_string());
        hook1.session_id = Some("claude-session".to_string());
        process_hook(&mut state, hook1);

        state.sessions.get_mut(&42).unwrap().display_name = "my-project".to_string();
        state.sessions.get_mut(&42).unwrap().manually_renamed = true;

        // Codex hook arrives for the same pane (child process)
        let mut hook2 = make_hook(42, "PreToolUse");
        hook2.agent = Some("codex".to_string());
        hook2.agent_indicator = Some("\u{25c6}".to_string());
        hook2.session_id = Some("codex-session".to_string());
        process_hook(&mut state, hook2);

        // Name preserved, agent updated, no replacement state reset
        assert_eq!(state.sessions[&42].display_name, "my-project");
        assert!(state.sessions[&42].manually_renamed);
        assert_eq!(state.sessions[&42].agent_name, Some("codex".to_string()));
        assert_eq!(state.sessions[&42].session_id, "codex-session");
    }

    #[test]
    fn test_paused_session_updates_session_id() {
        let mut state = ControllerState::default();

        let mut hook1 = make_hook(42, "SessionStart");
        hook1.session_id = Some("old-session".to_string());
        hook1.agent = Some("codex".to_string());
        process_hook(&mut state, hook1);

        state.sessions.get_mut(&42).unwrap().paused = true;

        // New session starts in same pane while paused
        let mut hook2 = make_hook(42, "SessionStart");
        hook2.session_id = Some("new-session".to_string());
        hook2.agent = Some("codex".to_string());
        process_hook(&mut state, hook2);

        // Session ID must be updated even though the session is paused,
        // otherwise every subsequent hook triggers another replacement.
        assert_eq!(state.sessions[&42].session_id, "new-session");
    }

    #[test]
    fn test_process_hook_worktree_cwd_sets_in_worktree() {
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
            cwd: Some("/home/user/project/.claude/worktrees/076-fix/".to_string()),
            agent_id: None,
            badges: vec![],
            profile: None,
            profile_color: None,
        };
        process_hook(&mut state, hook);

        // CWD should be updated to the worktree path
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/project/.claude/worktrees/076-fix/")
        );
        // in_worktree should be set to true
        assert!(state.sessions[&42].in_worktree);
    }

    #[test]
    fn test_process_hook_claude_settings_cwd_suppressed() {
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
            cwd: Some("/home/user/project/.claude/settings.json".to_string()),
            agent_id: None,
            badges: vec![],
            profile: None,
            profile_color: None,
        };
        process_hook(&mut state, hook);

        // CWD should NOT change to the .claude/ path
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/project")
        );
        // in_worktree should remain false
        assert!(!state.sessions[&42].in_worktree);
    }

    #[test]
    fn test_process_hook_worktree_to_normal_clears_in_worktree() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.working_dir = Some("/home/user/project/.claude/worktrees/076-fix/".to_string());
        s.in_worktree = true;
        state.sessions.insert(42, s);

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test".to_string()),
            pane_id: 42,
            hook_event_name: "PreToolUse".to_string(),
            tool_name: None,
            cwd: Some("/home/user/project".to_string()),
            agent_id: None,
            badges: vec![],
            profile: None,
            profile_color: None,
        };
        process_hook(&mut state, hook);

        // CWD should change to the normal path
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/project")
        );
        // in_worktree should be cleared
        assert!(!state.sessions[&42].in_worktree);
    }

    #[test]
    fn test_process_hook_worktree_to_worktree_keeps_in_worktree() {
        let mut state = ControllerState::default();
        let mut s = Session::new(42, "test".into());
        s.working_dir = Some("/home/user/project/.claude/worktrees/076-fix/".to_string());
        s.in_worktree = true;
        state.sessions.insert(42, s);

        let hook = HookPayload {
            agent: None,
            agent_indicator: None,
            session_id: Some("test".to_string()),
            pane_id: 42,
            hook_event_name: "PreToolUse".to_string(),
            tool_name: None,
            cwd: Some("/home/user/project/.claude/worktrees/077-other/".to_string()),
            agent_id: None,
            badges: vec![],
            profile: None,
            profile_color: None,
        };
        process_hook(&mut state, hook);

        // CWD should update to the new worktree
        assert_eq!(
            state.sessions[&42].working_dir.as_deref(),
            Some("/home/user/project/.claude/worktrees/077-other/")
        );
        // in_worktree should remain true
        assert!(state.sessions[&42].in_worktree);
    }
}
