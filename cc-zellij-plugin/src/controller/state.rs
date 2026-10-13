// Controller state: authoritative session store for the single-instance architecture.
//
// The controller is the sole writer of session state. Sidebars receive
// pre-computed RenderPayload via pipe and send ActionMessages back.
// This eliminates the N-instance sync protocol (cc-deck:sync, cc-deck:request)
// and the file-based metadata sync (session-meta.json).

use crate::config::PluginConfig;
use crate::perf::PerfTracker;
use crate::session::{Activity, Session};
use std::collections::{BTreeMap, HashMap, HashSet};
use zellij_tile::prelude::*;

/// PID-scoped sessions file path: `/cache/sessions-{pid}.json`.
/// Falls back to the legacy `/cache/sessions.json` when PID is 0 (native tests).
fn sessions_path(pid: u32) -> String {
    if pid == 0 {
        "/cache/sessions.json".to_string()
    } else {
        format!("/cache/sessions-{pid}.json")
    }
}

pub const FOCUS_CONFIRM_TIMEOUT_MS: u64 = 3000;

/// How long a hook-created session may wait for the pane manifest to confirm
/// that its pane really exists, before it is evicted.
///
/// Matches `FOCUS_CONFIRM_TIMEOUT_MS` and the startup grace: all three answer
/// the same question, "how long do we wait for Zellij to tell us the truth".
pub const HOOK_CONFIRM_TIMEOUT_MS: u64 = 3000;

/// How long a session restored from the on-disk cache may wait for
/// confirmation. Names the startup grace window that was an inline literal.
pub const RESTORE_GRACE_MS: u64 = 3000;

/// Why a pane id is awaiting confirmation from the pane manifest.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum QuarantineKind {
    /// Restored from the on-disk session cache. Carries no live evidence.
    Restored,
    /// Created by a hook event naming a pane the manifest has not confirmed.
    Hook,
}

/// A pane id awaiting confirmation, and the deadline by which it must arrive.
#[derive(Debug, Clone, Copy)]
pub struct Quarantine {
    pub kind: QuarantineKind,
    pub deadline_ms: u64,
}

/// Metadata override to apply when a restored session is discovered via CWD matching.
#[derive(Debug, Clone)]
pub struct PendingOverride {
    pub display_name: String,
    pub paused: bool,
    pub profile: Option<String>,
    pub profile_color: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct ClientViewState {
    pub active_tab_index: Option<usize>,
    pub focused_pane_id: Option<u32>,
    pub revision: u64,
    pub pending_focus: Option<PendingFocus>,
}

#[derive(Debug, Clone, Copy)]
pub struct PendingFocus {
    pub pane_id: u32,
    pub tab_index: usize,
    pub expires_at_ms: u64,
}

impl ClientViewState {
    pub fn snapshot(&self) -> cc_deck::ClientViewSnapshot {
        cc_deck::ClientViewSnapshot {
            active_tab_index: self.active_tab_index,
            focused_pane_id: self.focused_pane_id,
            revision: self.revision,
        }
    }
}

/// The authoritative state held by the controller plugin instance.
#[derive(Default)]
pub struct ControllerState {
    /// All known Claude sessions, keyed by pane_id. Single writer.
    pub sessions: BTreeMap<u32, Session>,
    /// Current tab list from TabUpdate events.
    pub tabs: Vec<TabInfo>,
    /// Current pane manifest from PaneUpdate events.
    pub pane_manifest: Option<PaneManifest>,
    /// Pane ID -> (tab_index, tab_name) mapping derived from manifest + tabs.
    pub pane_to_tab: HashMap<u32, (usize, String)>,
    /// Per-connected-client focus and ordering state.
    pub client_views: BTreeMap<u16, ClientViewState>,
    /// Registered sidebar instances: plugin_id -> (tab_index, client_id).
    /// The client_id component enables multiplayer filtering: the controller
    /// only broadcasts renders to sidebars from its own client connection.
    pub sidebar_registry: HashMap<u32, (usize, u16)>,
    /// This controller's plugin ID (set after permissions granted).
    pub plugin_id: u32,
    /// This controller's Zellij client ID (set alongside plugin_id during
    /// permission grant). Used to filter render broadcasts to only target
    /// sidebars from the same client connection.
    pub client_id: u16,
    /// Whether plugin permissions have been granted.
    pub permissions_granted: bool,

    /// How many times we have re-asked for a permission grant that never
    /// arrived. Bounded, because a request that is genuinely waiting on the
    /// user must not be re-raised on a loop.
    pub permission_retries: u8,
    /// Whether the render payload needs to be broadcast on the next timer tick.
    pub render_dirty: bool,
    /// Millisecond timestamp until which `remove_dead_sessions()` is skipped.
    pub startup_grace_until: Option<u64>,
    /// Pending metadata overrides from snapshot restore, keyed by working directory.
    pub pending_overrides: HashMap<String, Vec<PendingOverride>>,
    /// Configuration parsed from KDL layout.
    pub config: PluginConfig,
    /// Whether keybindings have been registered via reconfigure().
    pub keybindings_registered: bool,
    /// Tab count from last TabUpdate. Used to detect tab closures.
    pub last_tab_count: usize,
    /// Whether the on-disk session cache has already been read in this
    /// process. The timer restore runs once; `handle_refresh` ignores it on
    /// purpose, because refresh is the user's deliberate escape hatch.
    pub restore_attempted: bool,
    /// Pane IDs whose sessions the pane manifest has not confirmed, each with
    /// the deadline by which confirmation must arrive. Two sources feed it:
    /// sessions restored from the on-disk cache, and sessions created by a hook
    /// event naming a pane the manifest does not show. See `sweep_quarantine`.
    pub unconfirmed_panes: HashMap<u32, Quarantine>,
    /// Pane IDs with in-flight git branch detection commands.
    pub pending_git_branch: HashSet<u32>,
    /// Timestamp (ms) of the last timer-driven git branch poll.
    pub last_git_poll_ms: u64,
    /// Performance instrumentation tracker.
    pub perf: PerfTracker,
    /// Last pane_id that attend switched to, for round-robin cycling.
    pub last_attended_pane_id: Option<u32>,
    /// Timestamp (ms) of the last attend action, for rapid-cycle detection.
    pub last_attend_ms: u64,
    /// Pane IDs already visited during the current rapid-cycle sequence.
    pub attend_visited: HashSet<u32>,
    /// Whether voice relay is currently connected.
    pub voice_enabled: bool,
    /// Whether voice relay is currently muted.
    pub voice_muted: bool,
    /// Timestamp (ms) of last voice ping or voice:on message.
    pub voice_last_ping_ms: u64,
    /// Pending mute toggle from sidebar: Some(true) = mute, Some(false) = unmute.
    pub voice_mute_requested: Option<bool>,
    /// Timestamp (ms) when voice_mute_requested was set; used for timeout.
    pub voice_mute_requested_ms: u64,
    /// Events received before permissions were granted.
    pub pending_events: Vec<Event>,
    /// Monotonic tick counter for render coalescing.
    pub tick_count: u64,
    /// Deduplication guard for voice text injection. Zellij broadcast pipes
    /// can deliver the same message multiple times (once per plugin instance
    /// unblock). Tracks (text_hash, timestamp_ms) to suppress duplicates
    /// within a short window.
    pub voice_last_inject: Option<(u64, u64)>,
    /// Frozen display order from the last sort-by-activity (pane IDs).
    /// When Some, the render broadcast uses this order instead of tab_index.
    pub sort_order: Option<Vec<u32>>,
    /// Pane IDs of sessions that recently transitioned from paused to active.
    /// These appear at the end of the active zone (FR-002).
    pub auto_sort_tail: Vec<u32>,
    /// Multiplayer user colors extracted from Zellij's ModeUpdate palette.
    /// None until the first ModeUpdate event is received.
    pub multiplayer_colors: Option<Vec<(u8, u8, u8)>>,
}

impl ControllerState {
    /// Rebuild the pane-to-tab mapping from current tab and pane data.
    /// PaneInfo::is_focused is deliberately ignored: in multiplayer it means
    /// "focused by any client" and therefore has no stable client identity.
    pub fn rebuild_pane_map(&mut self) {
        self.pane_to_tab.clear();
        if self.tabs.is_empty() {
            return;
        }
        if let Some(ref manifest) = self.pane_manifest {
            for tab in &self.tabs {
                if let Some(panes) = manifest.panes.get(&tab.position) {
                    for pane in panes {
                        if !pane.is_plugin {
                            self.pane_to_tab
                                .insert(pane.id, (tab.position, tab.name.clone()));
                        }
                    }
                }
            }
        }

        // Refresh tab info on all sessions and process deferred tab renames.
        let mut pending_renames: Vec<(usize, String)> = Vec::new();
        for session in self.sessions.values_mut() {
            if let Some((idx, name)) = self.pane_to_tab.get(&session.pane_id) {
                session.tab_index = Some(*idx);
                session.tab_name = Some(name.clone());
                if session.pending_tab_rename {
                    session.pending_tab_rename = false;
                    pending_renames.push((*idx, session.display_name.clone()));
                }
            }
        }
        // Issue deferred tab renames for tabs with a single session.
        for (tab_idx, display_name) in &pending_renames {
            let sessions_on_tab = self
                .sessions
                .values()
                .filter(|s| s.tab_index == Some(*tab_idx))
                .count();
            if sessions_on_tab == 1 {
                crate::wasm_compat::rename_tab_wasm(*tab_idx, display_name);
            }
        }
    }

    /// Whether a pane is live according to the current manifest.
    ///
    /// This is the single place a missing manifest is interpreted:
    ///
    /// - `Some(true)`  the pane exists, is not a plugin, and has not exited
    /// - `Some(false)` the manifest is present and the pane is absent or exited
    /// - `None`        liveness is unknown
    ///
    /// `None` is never a verdict. Callers decide what unknown means for them
    /// and must not collapse it to `false`.
    ///
    /// A manifest carrying no terminal panes at all counts as unknown rather
    /// than as proof of absence, for the same reason `remove_dead_sessions`
    /// bails on an empty manifest: it is more likely mid-update than truthful.
    pub fn pane_is_live(&self, pane_id: u32) -> Option<bool> {
        let manifest = self.pane_manifest.as_ref()?;
        let mut saw_terminal_pane = false;
        let mut live = false;
        for pane in manifest.panes.values().flatten() {
            if pane.is_plugin {
                continue;
            }
            saw_terminal_pane = true;
            if pane.id == pane_id && !pane.exited {
                live = true;
            }
        }
        if !saw_terminal_pane {
            return None;
        }
        Some(live)
    }

    /// Mark a pane id as awaiting confirmation from the pane manifest.
    pub fn quarantine(&mut self, pane_id: u32, kind: QuarantineKind) {
        let timeout = match kind {
            QuarantineKind::Hook => HOOK_CONFIRM_TIMEOUT_MS,
            QuarantineKind::Restored => RESTORE_GRACE_MS,
        };
        self.unconfirmed_panes.insert(
            pane_id,
            Quarantine {
                kind,
                deadline_ms: crate::session::unix_now_ms() + timeout,
            },
        );
    }

    /// Release a pane id from quarantine. Returns whether it was quarantined.
    pub fn confirm_pane(&mut self, pane_id: u32) -> bool {
        self.unconfirmed_panes.remove(&pane_id).is_some()
    }

    /// Whether a session must be withheld from the sidebar, the dump-state
    /// response, and the on-disk cache.
    ///
    /// Only hook-created sessions are withheld. Sessions restored from disk
    /// stay visible through their grace window, because hiding those would
    /// blank the sidebar on every reattach.
    pub fn is_hidden(&self, pane_id: u32) -> bool {
        matches!(
            self.unconfirmed_panes.get(&pane_id),
            Some(q) if q.kind == QuarantineKind::Hook
        )
    }

    /// Remove a session along with every index that refers to it.
    ///
    /// Consolidates removal side effects that were previously spelled out at
    /// each call site, where they had already drifted apart.
    pub fn evict_session(&mut self, pane_id: u32) -> bool {
        let removed = self.sessions.remove(&pane_id).is_some();
        self.unconfirmed_panes.remove(&pane_id);
        self.pending_git_branch.remove(&pane_id);
        if let Some(ref mut order) = self.sort_order {
            order.retain(|&p| p != pane_id);
        }
        self.auto_sort_tail.retain(|&p| p != pane_id);
        if removed {
            self.prune_client_views();
        }
        removed
    }

    /// Settle quarantined panes whose deadline has passed.
    ///
    /// Returns whether anything visible changed, so the caller can persist and
    /// re-render. Runs every tick; the map is normally empty.
    pub fn sweep_quarantine(&mut self) -> bool {
        if self.unconfirmed_panes.is_empty() {
            return false;
        }
        let now = crate::session::unix_now_ms();
        let expired: Vec<(u32, QuarantineKind)> = self
            .unconfirmed_panes
            .iter()
            .filter(|(_, q)| now >= q.deadline_ms)
            .map(|(pane_id, q)| (*pane_id, q.kind))
            .collect();

        let mut changed = false;
        let mut evicted = Vec::new();
        for (pane_id, kind) in expired {
            match (kind, self.pane_is_live(pane_id)) {
                // The manifest vouches for the pane. The race resolved in the
                // session's favour.
                (_, Some(true)) => {
                    self.unconfirmed_panes.remove(&pane_id);
                    // A hook session becoming confirmed also becomes visible.
                    changed |= kind == QuarantineKind::Hook;
                }
                // Positive proof of absence.
                (_, Some(false)) => evicted.push(pane_id),
                // No manifest after the full wait. For a hook-created session
                // this is the safety valve: without it, a controller that never
                // receives a PaneUpdate would hide every session forever,
                // trading a visible phantom for an invisible workspace. Give it
                // the benefit of the doubt, degrading to the older behaviour.
                (QuarantineKind::Hook, None) => {
                    self.unconfirmed_panes.remove(&pane_id);
                    changed = true;
                }
                // A restored session carries no live evidence at all: it came
                // off disk, possibly from a previous Zellij run. Absence of
                // proof is correctly fatal here, which preserves the previous
                // startup-grace semantics exactly.
                (QuarantineKind::Restored, None) => evicted.push(pane_id),
            }
        }

        if !evicted.is_empty() {
            for pane_id in &evicted {
                self.evict_session(*pane_id);
            }
            crate::debug_log(&format!(
                "CTRL QUARANTINE evicted {} unconfirmed sessions",
                evicted.len()
            ));
            changed = true;
        }
        changed
    }

    /// Sessions this controller is willing to show and to store.
    ///
    /// Hook-created sessions awaiting confirmation are excluded: reporting one
    /// puts a pane that may not exist in front of the user, and persisting one
    /// is precisely what lets a phantom survive into the next process and
    /// re-enter through the restore path. Restored sessions are kept, because
    /// dropping them here would erase legitimate sessions from the cache
    /// before they had any chance to be confirmed.
    pub fn visible_sessions(&self) -> BTreeMap<u32, &Session> {
        self.sessions
            .iter()
            .filter(|(pane_id, _)| !self.is_hidden(**pane_id))
            .map(|(pane_id, session)| (*pane_id, session))
            .collect()
    }

    /// Remove sessions whose panes no longer exist or have exited.
    /// Uses the raw pane manifest for stable pane IDs.
    pub fn remove_dead_sessions(&mut self) -> bool {
        let before = self.sessions.len();
        if before == 0 {
            return false;
        }
        let manifest = match self.pane_manifest {
            Some(ref m) => m,
            None => return false,
        };

        let mut all_pane_ids = HashSet::new();
        let mut exited_pane_ids = HashSet::new();
        for panes in manifest.panes.values() {
            for pane in panes {
                if !pane.is_plugin {
                    all_pane_ids.insert(pane.id);
                    if pane.exited {
                        exited_pane_ids.insert(pane.id);
                    }
                }
            }
        }

        if all_pane_ids.is_empty() {
            return false;
        }

        // Only remove sessions whose pane is confirmed exited.
        // Do NOT remove sessions whose pane_id is absent from the manifest,
        // as the manifest may be temporarily incomplete during rapid updates.
        let dead: Vec<u32> = self
            .sessions
            .keys()
            .copied()
            .filter(|id| exited_pane_ids.contains(id))
            .collect();
        for pane_id in dead {
            self.evict_session(pane_id);
        }
        if self.sessions.len() != before {
            crate::debug_log(&format!(
                "CTRL CLEANUP removed {} dead sessions, {} remaining",
                before - self.sessions.len(),
                self.sessions.len()
            ));
        }
        self.sessions.len() != before
    }

    /// Get sessions sorted by tab index for display.
    pub fn sessions_by_tab_order(&self) -> Vec<&Session> {
        // Unconfirmed hook sessions are withheld here too, so a pane the
        // manifest has never shown cannot become an attend or navigation
        // target and yank the user to a tab that does not exist.
        let mut sessions: Vec<&Session> = self
            .sessions
            .values()
            .filter(|s| !self.is_hidden(s.pane_id))
            .collect();
        sessions.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
        sessions
    }

    /// Get all session display names except a given pane_id (for deduplication).
    pub fn session_names_except(&self, exclude_pane_id: u32) -> Vec<&str> {
        self.sessions
            .iter()
            .filter(|(&id, _)| id != exclude_pane_id)
            .map(|(_, s)| s.display_name.as_str())
            .collect()
    }

    /// Transition stale sessions to Idle after timeout.
    ///
    /// Done/AgentDone: transition after `timeout_secs` (show green checkmark).
    /// Working: transition after `timeout_secs` as a fallback. Claude Code's
    ///   `Stop` hook does not fire reliably on natural response completion,
    ///   so sessions can get stuck in Working after Claude finishes generating.
    ///   If no hook events arrive within the timeout, the session has finished.
    /// Waiting: NOT cleaned up. The user may take arbitrarily long to respond
    ///   to a permission prompt. Only cleared by actual hook events.
    pub fn cleanup_stale_sessions(&mut self, timeout_secs: u64) -> bool {
        let now = crate::session::unix_now();
        let auto_pause = self.config.auto_pause_secs;
        let mut changed = false;
        let mut auto_paused_pids = Vec::new();
        for session in self.sessions.values_mut() {
            match session.activity {
                Activity::Done | Activity::AgentDone => {
                    if now.saturating_sub(session.last_event_ts) >= timeout_secs {
                        session.activity = Activity::Idle;
                        changed = true;
                    }
                }
                Activity::Working => {
                    if now.saturating_sub(session.last_event_ts) >= timeout_secs {
                        session.activity = Activity::Done;
                        changed = true;
                    }
                }
                Activity::Idle | Activity::Init
                    if !session.paused
                        && auto_pause > 0
                        && now.saturating_sub(session.last_event_ts) >= auto_pause =>
                {
                    session.paused = true;
                    auto_paused_pids.push(session.pane_id);
                    changed = true;
                }
                _ => {}
            }
        }
        for pid in &auto_paused_pids {
            self.auto_sort_tail.retain(|&p| p != *pid);
        }
        changed
    }

    /// Whether the startup grace period is currently active.
    pub fn in_startup_grace(&self) -> bool {
        self.startup_grace_until
            .map(|deadline| crate::session::unix_now_ms() < deadline)
            .unwrap_or(false)
    }

    /// Mark render payload as needing broadcast on the next timer flush.
    pub fn mark_render_dirty(&mut self) {
        self.render_dirty = true;
    }

    /// Whether the session on `pane_id` is currently waiting on the user.
    pub fn session_is_waiting(&self, pane_id: u32) -> bool {
        self.sessions
            .get(&pane_id)
            .is_some_and(|s| s.activity.is_waiting())
    }

    pub fn own_focus(&self) -> Option<u32> {
        self.client_views
            .get(&self.client_id)
            .and_then(|view| view.focused_pane_id)
    }

    pub fn own_active_tab(&self) -> Option<usize> {
        self.client_views
            .get(&self.client_id)
            .and_then(|view| view.active_tab_index)
    }

    pub fn set_client_focus_intent(&mut self, client_id: u16, pane_id: u32, tab_index: usize) {
        let view = self.client_views.entry(client_id).or_default();
        view.focused_pane_id = Some(pane_id);
        view.pending_focus = Some(PendingFocus {
            pane_id,
            tab_index,
            expires_at_ms: crate::session::unix_now_ms() + FOCUS_CONFIRM_TIMEOUT_MS,
        });
        view.revision = view.revision.wrapping_add(1);
    }

    pub fn prune_client_views(&mut self) {
        for view in self.client_views.values_mut() {
            if view
                .focused_pane_id
                .is_some_and(|pane_id| !self.sessions.contains_key(&pane_id))
            {
                view.focused_pane_id = None;
                view.pending_focus = None;
                view.revision = view.revision.wrapping_add(1);
            }
        }
    }

    /// Replace connected-client/tab knowledge from a client-scoped TabUpdate
    /// and reconcile exact focus without consulting PaneInfo::is_focused.
    pub fn reconcile_client_views(&mut self) -> bool {
        let mut client_tabs = BTreeMap::new();
        if let Some(tab) = self.tabs.iter().find(|tab| tab.active) {
            client_tabs.insert(self.client_id, tab.position);
        }
        for tab in &self.tabs {
            for &client_id in &tab.other_focused_clients {
                client_tabs.insert(client_id, tab.position);
            }
        }

        let old = self
            .client_views
            .iter()
            .map(|(&id, view)| (id, view.snapshot()))
            .collect::<BTreeMap<_, _>>();
        self.client_views
            .retain(|client_id, _| client_tabs.contains_key(client_id));

        let now = crate::session::unix_now_ms();
        for (&client_id, &tab_index) in &client_tabs {
            let candidates = self
                .sessions
                .values()
                .filter(|session| session.tab_index == Some(tab_index))
                .map(|session| session.pane_id)
                .collect::<Vec<_>>();
            let view = self.client_views.entry(client_id).or_default();
            view.active_tab_index = Some(tab_index);

            if let Some(pending) = view.pending_focus {
                if pending.tab_index == tab_index {
                    view.focused_pane_id = Some(pending.pane_id);
                    view.pending_focus = None;
                } else if now < pending.expires_at_ms {
                    continue;
                } else {
                    view.pending_focus = None;
                }
            }

            let current_is_on_tab = view.focused_pane_id.is_some_and(|pane_id| {
                self.sessions
                    .get(&pane_id)
                    .is_some_and(|session| session.tab_index == Some(tab_index))
            });
            if !current_is_on_tab {
                view.focused_pane_id = if candidates.len() == 1 {
                    candidates.first().copied()
                } else {
                    None
                };
                view.revision = view.revision.wrapping_add(1);
            }
        }

        let new = self
            .client_views
            .iter()
            .map(|(&id, view)| (id, view.snapshot()))
            .collect::<BTreeMap<_, _>>();
        old.iter().any(|(id, old_view)| {
            new.get(id).is_none_or(|new_view| {
                old_view.active_tab_index != new_view.active_tab_index
                    || old_view.focused_pane_id != new_view.focused_pane_id
            })
        }) || new.keys().any(|id| !old.contains_key(id))
    }

    /// Merge incoming sessions (used for restore from cache).
    pub fn merge_sessions(&mut self, incoming: BTreeMap<u32, Session>) -> bool {
        let mut changed = false;
        for (pane_id, mut session) in incoming {
            if let Some(existing) = self.sessions.get(&pane_id) {
                if session.last_event_ts > existing.last_event_ts {
                    // Preserve manual rename from the existing session:
                    // the user's rename is about the pane's purpose, not
                    // about which snapshot is newer.
                    if existing.manually_renamed && !session.manually_renamed {
                        session.display_name = existing.display_name.clone();
                        session.manually_renamed = true;
                        session.meta_ts = existing.meta_ts;
                    }
                    if let Some((idx, name)) = self.pane_to_tab.get(&pane_id) {
                        session.tab_index = Some(*idx);
                        session.tab_name = Some(name.clone());
                    }
                    self.sessions.insert(pane_id, session);
                    changed = true;
                }
            } else {
                if let Some((idx, name)) = self.pane_to_tab.get(&pane_id) {
                    session.tab_index = Some(*idx);
                    session.tab_name = Some(name.clone());
                }
                self.sessions.insert(pane_id, session);
                changed = true;
            }
        }
        changed
    }

    // --- Persistence (single-writer pattern) ---

    /// Persist full session state to disk for reattach recovery.
    /// Only the controller calls this; sidebars never write.
    /// Writes to the PID-scoped path (no separate PID file needed).
    pub fn save_sessions(&self) {
        let pid = current_zellij_pid();
        if let Ok(json) = serde_json::to_string(&self.visible_sessions()) {
            let _ = std::fs::write(sessions_path(pid), json);
        }
    }

    /// Merge the on-disk session cache into state, quarantining every session
    /// it newly introduces and re-arming the grace window.
    ///
    /// Restored sessions carry no live evidence, so they must prove themselves
    /// against the pane manifest before they count as real. Returns whether
    /// anything was merged.
    pub fn restore_and_quarantine(&mut self) -> bool {
        let restored = Self::restore_sessions();
        if restored.is_empty() {
            return false;
        }
        let count = restored.len();
        // Only genuinely new ids are quarantined: merge_sessions also replaces
        // entries that are already present and already confirmed.
        let new_ids: Vec<u32> = restored
            .keys()
            .copied()
            .filter(|id| !self.sessions.contains_key(id))
            .collect();
        let changed = self.merge_sessions(restored);
        for pane_id in new_ids {
            self.quarantine(pane_id, QuarantineKind::Restored);
        }
        self.startup_grace_until = Some(crate::session::unix_now_ms() + RESTORE_GRACE_MS);
        crate::debug_log(&format!("CTRL RESTORE merged {count} sessions from disk"));
        changed
    }

    /// Restore sessions from disk (called on load/reattach).
    /// Reads from the PID-scoped file. No cross-session PID check needed
    /// because each PID has its own file.
    pub fn restore_sessions() -> BTreeMap<u32, Session> {
        let pid = current_zellij_pid();
        match std::fs::read_to_string(sessions_path(pid)) {
            Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
            Err(_) => BTreeMap::new(),
        }
    }

}

/// Get the Zellij server PID.
#[cfg(target_family = "wasm")]
fn current_zellij_pid() -> u32 {
    zellij_tile::prelude::get_plugin_ids().zellij_pid
}

#[cfg(not(target_family = "wasm"))]
fn current_zellij_pid() -> u32 {
    0
}

/// Files earlier releases wrote to the shared cache and nothing reads any
/// more. Removed once so a long-lived install does not carry them forever.
const RETIRED_CACHE_FILES: [&str; 6] = [
    "/cache/sessions.json",
    "/cache/session-meta.json",
    "/cache/zellij_pid",
    "/cache/attend-state.json",
    "/cache/unified_update_controller",
    "/cache/unified_update_sidebar",
];

/// Clean up orphaned state files from killed Zellij sessions.
/// Scans `/cache/` for `sessions-*.json` and `session-meta-*.json` files.
/// Attempts to check process liveness via `/proc/{pid}/`. If `/proc/` is
/// not available (WASI limitation), falls back to removing files older
/// than 7 days based on modification time.
pub fn cleanup_orphaned_state_files() {
    let current_pid = current_zellij_pid();
    if current_pid == 0 {
        return;
    }

    for path in RETIRED_CACHE_FILES {
        let _ = std::fs::remove_file(path);
    }

    let entries = match std::fs::read_dir("/cache/") {
        Ok(e) => e,
        Err(_) => return,
    };

    let seven_days_secs: u64 = 7 * 24 * 60 * 60;
    let now_secs = crate::session::unix_now();

    for entry in entries.flatten() {
        let name = match entry.file_name().into_string() {
            Ok(n) => n,
            Err(_) => continue,
        };

        let pid = extract_pid_from_filename(&name);
        let pid = match pid {
            Some(p) => p,
            None => continue,
        };

        if pid == current_pid {
            continue;
        }

        let proc_path = format!("/proc/{pid}");
        let is_alive = std::fs::metadata(&proc_path).is_ok();

        if is_alive {
            continue;
        }

        let should_remove = match entry.metadata().and_then(|m| m.modified()) {
            Ok(mtime) => match mtime.duration_since(std::time::UNIX_EPOCH) {
                Ok(d) => now_secs.saturating_sub(d.as_secs()) > seven_days_secs,
                Err(_) => false,
            },
            Err(_) => false,
        };

        if should_remove {
            let path = entry.path();
            let _ = std::fs::remove_file(&path);
        }
    }
}

fn extract_pid_from_filename(name: &str) -> Option<u32> {
    if let Some(rest) = name.strip_prefix("sessions-") {
        rest.strip_suffix(".json")?.parse().ok()
    } else if let Some(rest) = name.strip_prefix("session-meta-") {
        rest.strip_suffix(".json")?.parse().ok()
    } else {
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::{Session, WaitReason};

    fn make_session(pane_id: u32) -> Session {
        Session::new(pane_id, format!("session-{pane_id}"))
    }

    fn make_pane_info(id: u32, is_plugin: bool) -> PaneInfo {
        PaneInfo {
            id,
            is_plugin,
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

    fn make_manifest(terminal_pane_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_pane_ids
            .iter()
            .map(|&id| make_pane_info(id, false))
            .collect();
        let mut map = HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    fn make_manifest_with_exited(terminal_ids: &[u32], exited_ids: &[u32]) -> PaneManifest {
        let panes: Vec<PaneInfo> = terminal_ids
            .iter()
            .map(|&id| {
                let mut p = make_pane_info(id, false);
                if exited_ids.contains(&id) {
                    p.exited = true;
                }
                p
            })
            .collect();
        let mut map = HashMap::new();
        map.insert(0, panes);
        PaneManifest { panes: map }
    }

    #[test]
    fn test_remove_dead_sessions_basic() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        // Pane 20 must be present AND exited for removal (absent panes are not removed)
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
    }

    #[test]
    fn test_remove_dead_sessions_exited() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));

        let changed = state.remove_dead_sessions();
        assert!(changed);
        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
    }

    #[test]
    fn test_startup_grace_skips_removal() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.pane_manifest = Some(make_manifest(&[10]));
        state.startup_grace_until = Some(crate::session::unix_now_ms() + 3000);

        assert!(state.in_startup_grace());
        // Caller should check in_startup_grace() before calling remove_dead_sessions
        assert_eq!(state.sessions.len(), 2);
    }

    #[test]
    fn test_pane_is_live_is_tri_state() {
        let mut state = ControllerState::default();

        // No manifest: unknown, never a verdict.
        assert_eq!(state.pane_is_live(10), None);

        state.pane_manifest = Some(make_manifest(&[10]));
        assert_eq!(state.pane_is_live(10), Some(true));
        assert_eq!(state.pane_is_live(99), Some(false));

        // An exited pane is not live.
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));
        assert_eq!(state.pane_is_live(20), Some(false));
    }

    /// A manifest carrying no terminal panes is more likely mid-update than
    /// truthful, so it must not be read as proof that everything is gone.
    #[test]
    fn test_pane_is_live_treats_empty_manifest_as_unknown() {
        let mut state = ControllerState::default();
        state.pane_manifest = Some(PaneManifest {
            panes: std::collections::HashMap::new(),
        });
        assert_eq!(state.pane_is_live(10), None);
    }

    #[test]
    fn test_visible_sessions_excludes_hook_quarantine_only() {
        let mut state = ControllerState::default();
        state.sessions.insert(1, make_session(1));
        state.sessions.insert(2, make_session(2));
        state.sessions.insert(3, make_session(3));
        state.quarantine(2, QuarantineKind::Hook);
        state.quarantine(3, QuarantineKind::Restored);

        let visible = state.visible_sessions();

        assert!(visible.contains_key(&1), "confirmed sessions persist");
        assert!(
            !visible.contains_key(&2),
            "an unverified hook session must not reach disk"
        );
        assert!(
            visible.contains_key(&3),
            "restored sessions must not be erased from the cache"
        );
    }

    #[test]
    fn test_evict_session_clears_every_index() {
        let mut state = ControllerState::default();
        state.sessions.insert(7, make_session(7));
        state.quarantine(7, QuarantineKind::Hook);
        state.pending_git_branch.insert(7);
        state.sort_order = Some(vec![7, 8]);
        state.auto_sort_tail = vec![7, 8];

        assert!(state.evict_session(7));

        assert!(!state.sessions.contains_key(&7));
        assert!(!state.unconfirmed_panes.contains_key(&7));
        assert!(!state.pending_git_branch.contains(&7));
        assert_eq!(state.sort_order, Some(vec![8]));
        assert_eq!(state.auto_sort_tail, vec![8]);
    }

    /// The asymmetry is the subtlest part of the design, so the two arms are
    /// asserted side by side. A hook session with no manifest gets the benefit
    /// of the doubt; a restored session, which carries no live evidence at
    /// all, does not.
    #[test]
    fn test_sweep_without_manifest_confirms_hook_but_evicts_restored() {
        let mut state = ControllerState::default();
        state.sessions.insert(1, make_session(1));
        state.sessions.insert(2, make_session(2));
        state.quarantine(1, QuarantineKind::Hook);
        state.quarantine(2, QuarantineKind::Restored);
        assert!(state.pane_manifest.is_none());
        for q in state.unconfirmed_panes.values_mut() {
            q.deadline_ms = 0;
        }

        state.sweep_quarantine();

        assert!(state.sessions.contains_key(&1), "hook session survives");
        assert!(!state.is_hidden(1), "and becomes visible");
        assert!(!state.sessions.contains_key(&2), "restored session is evicted");
    }

    #[test]
    fn test_sweep_before_deadline_does_nothing() {
        let mut state = ControllerState::default();
        state.sessions.insert(1, make_session(1));
        state.quarantine(1, QuarantineKind::Hook);
        state.pane_manifest = Some(make_manifest(&[99]));

        assert!(!state.sweep_quarantine());

        assert!(state.sessions.contains_key(&1));
        assert!(state.unconfirmed_panes.contains_key(&1));
    }

    #[test]
    fn test_sweep_confirms_when_manifest_shows_pane_live() {
        let mut state = ControllerState::default();
        state.sessions.insert(1, make_session(1));
        state.quarantine(1, QuarantineKind::Hook);
        state.pane_manifest = Some(make_manifest(&[1]));
        state.unconfirmed_panes.get_mut(&1).unwrap().deadline_ms = 0;

        assert!(state.sweep_quarantine());

        assert!(state.sessions.contains_key(&1));
        assert!(state.unconfirmed_panes.is_empty());
    }

    #[test]
    fn test_sessions_by_tab_order_withholds_hook_quarantine() {
        let mut state = ControllerState::default();
        state.sessions.insert(1, make_session(1));
        state.sessions.insert(2, make_session(2));
        state.quarantine(2, QuarantineKind::Hook);

        let ordered = state.sessions_by_tab_order();

        assert_eq!(ordered.len(), 1);
        assert_eq!(ordered[0].pane_id, 1);
    }

    #[test]
    fn test_unconfirmed_restored_sessions_removed() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.quarantine(10, QuarantineKind::Restored);
        state.quarantine(20, QuarantineKind::Restored);
        // Pane 10 is real. Pane 20 is not in the manifest at all.
        state.pane_manifest = Some(make_manifest(&[10]));

        // Expire both deadlines so the sweep settles them on this call.
        for q in state.unconfirmed_panes.values_mut() {
            q.deadline_ms = 0;
        }
        assert!(state.sweep_quarantine());

        assert_eq!(state.sessions.len(), 1);
        assert!(state.sessions.contains_key(&10));
        assert!(!state.sessions.contains_key(&20));
        assert!(state.unconfirmed_panes.is_empty());
    }

    #[test]
    fn test_cleanup_stale_sessions() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Done;
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        let changed = state.cleanup_stale_sessions(30);
        assert!(changed);
        assert_eq!(state.sessions[&10].activity, Activity::Idle);
    }

    #[test]
    fn test_cleanup_stale_working_becomes_done() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Working;
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        let changed = state.cleanup_stale_sessions(30);
        assert!(changed);
        assert_eq!(state.sessions[&10].activity, Activity::Done);
    }

    #[test]
    fn test_cleanup_never_touches_waiting_sessions() {
        let mut state = ControllerState::default();
        let mut s = make_session(10);
        s.activity = Activity::Waiting(WaitReason::Permission);
        s.last_event_ts = 0; // Very old
        state.sessions.insert(10, s);

        // Waiting is never cleaned up by the timer, regardless of age
        let changed = state.cleanup_stale_sessions(30);
        assert!(!changed);
        assert_eq!(
            state.sessions[&10].activity,
            Activity::Waiting(WaitReason::Permission)
        );
    }

    #[test]
    fn test_merge_sessions() {
        let mut state = ControllerState::default();
        let mut incoming = BTreeMap::new();
        incoming.insert(1, {
            let mut s = make_session(1);
            s.display_name = "api".to_string();
            s.last_event_ts = 100;
            s
        });

        assert!(state.merge_sessions(incoming));
        assert_eq!(state.sessions.len(), 1);
        assert_eq!(state.sessions[&1].display_name, "api");
    }

    #[test]
    fn test_merge_sessions_preserves_manual_rename() {
        let mut state = ControllerState::default();
        let mut existing = make_session(1);
        existing.display_name = "my-custom-name".to_string();
        existing.manually_renamed = true;
        existing.last_event_ts = 50;
        existing.meta_ts = 42;
        state.sessions.insert(1, existing);

        let mut incoming = BTreeMap::new();
        let mut newer = make_session(1);
        newer.display_name = "cc-deck".to_string();
        newer.manually_renamed = false;
        newer.last_event_ts = 100;
        incoming.insert(1, newer);

        assert!(state.merge_sessions(incoming));
        assert_eq!(state.sessions[&1].display_name, "my-custom-name");
        assert!(state.sessions[&1].manually_renamed);
        assert_eq!(state.sessions[&1].meta_ts, 42);
    }

    #[test]
    fn test_session_names_except() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1);
        s1.display_name = "api".to_string();
        state.sessions.insert(1, s1);
        let mut s2 = make_session(2);
        s2.display_name = "web".to_string();
        state.sessions.insert(2, s2);

        let names = state.session_names_except(1);
        assert_eq!(names, vec!["web"]);
    }

    #[test]
    fn test_sessions_by_tab_order() {
        let mut state = ControllerState::default();
        let mut s1 = make_session(1);
        s1.tab_index = Some(2);
        state.sessions.insert(1, s1);
        let mut s2 = make_session(2);
        s2.tab_index = Some(0);
        state.sessions.insert(2, s2);

        let ordered = state.sessions_by_tab_order();
        assert_eq!(ordered[0].pane_id, 2);
        assert_eq!(ordered[1].pane_id, 1);
    }

    #[test]
    fn test_mark_render_dirty() {
        let mut state = ControllerState::default();
        assert!(!state.render_dirty);
        state.mark_render_dirty();
        assert!(state.render_dirty);
    }

    #[test]
    fn test_extract_pid_from_filename() {
        assert_eq!(
            super::extract_pid_from_filename("sessions-12345.json"),
            Some(12345)
        );
        assert_eq!(
            super::extract_pid_from_filename("session-meta-12345.json"),
            Some(12345)
        );
        assert_eq!(super::extract_pid_from_filename("sessions.json"), None);
        assert_eq!(super::extract_pid_from_filename("session-meta.json"), None);
        assert_eq!(super::extract_pid_from_filename("debug.log"), None);
        assert_eq!(super::extract_pid_from_filename("sessions-abc.json"), None);
    }

    #[test]
    fn test_sessions_path() {
        assert_eq!(super::sessions_path(12345), "/cache/sessions-12345.json");
        assert_eq!(super::sessions_path(0), "/cache/sessions.json");
    }

    #[test]
    fn test_remove_dead_sessions_cleans_auto_sort_tail() {
        let mut state = ControllerState::default();
        state.sessions.insert(10, make_session(10));
        state.sessions.insert(20, make_session(20));
        state.auto_sort_tail = vec![10, 20];
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[20]));

        state.remove_dead_sessions();
        assert!(state.auto_sort_tail.contains(&10));
        assert!(!state.auto_sort_tail.contains(&20));
    }

    #[test]
    fn test_cleanup_stale_sessions_cleans_auto_sort_tail() {
        let mut state = ControllerState::default();
        state.config.auto_pause_secs = 10;
        let mut s = make_session(10);
        s.activity = Activity::Idle;
        s.last_event_ts = 0;
        state.sessions.insert(10, s);
        state.auto_sort_tail = vec![10];

        state.cleanup_stale_sessions(300);
        assert!(state.sessions[&10].paused);
        assert!(!state.auto_sort_tail.contains(&10));
    }

    #[test]
    fn pane_manifest_focus_never_changes_client_focus() {
        let mut state = ControllerState::default();
        state.client_id = 1;
        state.set_client_focus_intent(1, 10, 0);
        state.tabs = vec![TabInfo {
            position: 0,
            active: true,
            ..Default::default()
        }];
        state.pane_manifest = Some(make_manifest_with_exited(&[10, 20], &[]));

        state.rebuild_pane_map();

        assert_eq!(state.own_focus(), Some(10));
    }

    #[test]
    fn reconcile_keeps_clients_independent_and_cleans_disconnects() {
        let mut state = ControllerState::default();
        state.client_id = 1;
        let mut first = make_session(10);
        first.tab_index = Some(0);
        let mut second = make_session(20);
        second.tab_index = Some(1);
        state.sessions.insert(10, first);
        state.sessions.insert(20, second);
        state.set_client_focus_intent(1, 10, 0);
        state.set_client_focus_intent(2, 20, 1);
        state.tabs = vec![
            TabInfo {
                position: 0,
                active: true,
                ..Default::default()
            },
            TabInfo {
                position: 1,
                other_focused_clients: vec![2],
                ..Default::default()
            },
        ];

        state.reconcile_client_views();
        assert_eq!(state.client_views[&1].focused_pane_id, Some(10));
        assert_eq!(state.client_views[&2].focused_pane_id, Some(20));

        state.tabs[1].other_focused_clients.clear();
        state.reconcile_client_views();
        assert!(!state.client_views.contains_key(&2));
    }

    #[test]
    fn reconcile_clears_ambiguous_tab_without_exact_focus() {
        let mut state = ControllerState::default();
        state.client_id = 1;
        for pane_id in [10, 20] {
            let mut session = make_session(pane_id);
            session.tab_index = Some(0);
            state.sessions.insert(pane_id, session);
        }
        state.tabs = vec![TabInfo {
            position: 0,
            active: true,
            ..Default::default()
        }];

        state.reconcile_client_views();

        assert_eq!(state.client_views[&1].focused_pane_id, None);
    }
}
