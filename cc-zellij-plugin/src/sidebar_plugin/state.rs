// Sidebar renderer plugin state.
//
// Contains the cached render payload from the controller and all local
// UI state (mode, click regions, scroll, filter). Does NOT hold session
// data directly; sessions are received pre-computed in RenderPayload.

use super::modes::SidebarMode;
use crate::config::PluginConfig;
use cc_deck::{RenderPayload, RenderSession};

/// Click region: (row, pane_id, tab_index).
pub type ClickRegion = (usize, u32, usize);

/// A brief notification message displayed in the sidebar.
#[derive(Debug, Clone)]
pub struct Notification {
    pub message: String,
    pub expires_at: u64,
}

/// Aggregate state held by a sidebar renderer plugin instance.
pub struct SidebarState {
    /// Last received render data from the controller.
    pub cached_payload: Option<RenderPayload>,

    /// Current sidebar interaction mode.
    pub mode: SidebarMode,

    /// Click regions from the last render pass.
    pub click_regions: Vec<ClickRegion>,

    /// Tab index assigned by the controller during sidebar-init.
    pub my_tab_index: Option<usize>,

    /// This plugin instance's ID.
    pub my_plugin_id: u32,

    /// This plugin instance's client ID (from the Zellij client that spawned it).
    pub my_client_id: u16,

    /// The controller plugin's ID (learned from sidebar-init or render payload).
    pub controller_plugin_id: Option<u32>,

    /// Current filter text (applied locally to cached sessions).
    pub filter_text: String,

    /// Transient notification to display.
    pub notification: Option<Notification>,

    /// Plugin configuration.
    pub config: PluginConfig,

    /// Whether the first render payload has been received.
    pub initialized: bool,

    /// Whether plugin permissions have been granted.
    pub permissions_granted: bool,

    /// Last left-click timestamp (ms) and pane_id for double-click detection.
    pub last_click: Option<(u64, u32)>,

    /// Predictive focus override set when this sidebar focuses a pane locally.
    /// It provides immediate feedback until the controller confirms the view.
    pub local_focus_override: Option<u32>,

    /// Predictive mute override for immediate visual feedback on mute toggle.
    /// Set on click, cleared when the controller payload confirms the change.
    pub local_mute_override: Option<bool>,

    /// Pane ID to track the cursor to after the next render payload arrives.
    /// Set when a Sort action is dispatched; consumed on the next payload update.
    pub sort_cursor_pane_id: Option<u32>,

    /// Manual scroll offset: index of the first visible session.
    /// `None` means auto-center on the active session (default behavior).
    /// Set by mouse wheel, overflow indicator clicks, and navigate cursor tracking.
    pub scroll_offset: Option<usize>,

    /// Start index from the last render viewport (used as scroll base).
    pub last_viewport_start: usize,

    /// Max visible sessions from the last render pass (used for half-page scroll).
    pub last_max_visible: usize,
}

impl Default for SidebarState {
    fn default() -> Self {
        Self {
            cached_payload: None,
            mode: SidebarMode::Passive,
            click_regions: Vec::new(),
            my_tab_index: None,
            my_plugin_id: 0,
            my_client_id: 0,
            controller_plugin_id: None,
            filter_text: String::new(),
            notification: None,
            config: PluginConfig::default(),
            initialized: false,
            permissions_granted: false,
            last_click: None,
            local_focus_override: None,
            local_mute_override: None,
            sort_cursor_pane_id: None,
            scroll_offset: None,
            last_viewport_start: 0,
            last_max_visible: 0,
        }
    }
}

impl SidebarState {
    /// Get sessions from the cached payload, filtered by the current filter text.
    /// Returns an empty slice if no payload is cached.
    pub fn filtered_sessions(&self) -> Vec<&RenderSession> {
        let payload = match &self.cached_payload {
            Some(p) => p,
            None => return Vec::new(),
        };

        // The controller owns the stable auto-sort projection. Focus and
        // activity changes must never reorder either zone in the sidebar.
        let sessions = payload.sessions.iter().collect::<Vec<_>>();

        if self.filter_text.is_empty() {
            // Also check mode-level filter state
            if let Some(fs) = self.mode.filter_state() {
                if fs.input_buffer.is_empty() {
                    return sessions;
                }
                let lower = fs.input_buffer.to_lowercase();
                return sessions
                    .into_iter()
                    .filter(|s| s.display_name.to_lowercase().contains(&lower))
                    .collect();
            }
            return sessions;
        }

        let lower = self.filter_text.to_lowercase();
        sessions
            .into_iter()
            .filter(|s| s.display_name.to_lowercase().contains(&lower))
            .collect()
    }

    /// Get the focused pane ID from the cached payload.
    pub fn focused_pane_id(&self) -> Option<u32> {
        self.effective_focused_pane_id()
    }

    /// Get the effective focused pane ID, preferring the local override
    /// (set when the sidebar focuses a pane) over the payload value.
    /// This provides immediate highlight without waiting for controller confirmation.
    pub fn effective_focused_pane_id(&self) -> Option<u32> {
        if let Some(pid) = self.local_focus_override {
            return Some(pid);
        }
        if let Some(ref payload) = self.cached_payload {
            if let Some(view) = payload.client_views.get(&self.my_client_id) {
                return view.focused_pane_id;
            }
        }
        None
    }

    pub fn cursor_index(&self) -> usize {
        let cursor = self.mode.cursor_pane_id();
        cursor
            .and_then(|pane_id| {
                self.filtered_sessions()
                    .iter()
                    .position(|session| session.pane_id == pane_id)
            })
            .unwrap_or(0)
    }

    /// Preserve cursor position by clamping after session list changes.
    pub fn preserve_cursor(&mut self) {
        let sessions = self.filtered_sessions();
        let current = self.mode.cursor_pane_id();
        let preserved = current
            .filter(|pane_id| sessions.iter().any(|s| s.pane_id == *pane_id))
            .or_else(|| sessions.first().map(|session| session.pane_id));
        if let Some(ctx) = self.mode.nav_ctx_mut() {
            ctx.cursor_pane_id = preserved;
        }
    }

    /// Track cursor by pane_id after a sort operation.
    ///
    /// When sessions are reordered, the cursor index may no longer point to
    /// the same session. This method finds the new position of the session
    /// that the cursor was on (by pane_id) and preserves its identity.
    pub fn track_cursor_by_pane_id(&mut self, pane_id: u32) {
        if self
            .filtered_sessions()
            .iter()
            .any(|session| session.pane_id == pane_id)
        {
            if let Some(ctx) = self.mode.nav_ctx_mut() {
                ctx.cursor_pane_id = Some(pane_id);
            }
        }
        // If pane_id not found, preserve_cursor() will clamp
    }

    /// Clear expired notifications.
    pub fn clear_expired_notifications(&mut self) {
        if let Some(ref notif) = self.notification {
            if crate::session::unix_now() >= notif.expires_at {
                self.notification = None;
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::super::modes::NavigateContext;
    use super::super::modes::NavigationOverlay;
    use super::super::test_helpers::{make_payload, make_session};
    use super::*;

    #[test]
    fn test_filtered_sessions_no_payload() {
        let state = SidebarState::default();
        assert!(state.filtered_sessions().is_empty());
    }

    #[test]
    fn test_filtered_sessions_no_filter() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![
            make_session(1, "api-server", 0),
            make_session(2, "frontend", 1),
        ]));
        assert_eq!(state.filtered_sessions().len(), 2);
    }

    #[test]
    fn test_filtered_sessions_with_filter() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![
            make_session(1, "api-server", 0),
            make_session(2, "frontend", 1),
            make_session(3, "api-gateway", 2),
        ]));
        state.filter_text = "api".to_string();
        let filtered = state.filtered_sessions();
        assert_eq!(filtered.len(), 2);
        assert_eq!(filtered[0].display_name, "api-server");
        assert_eq!(filtered[1].display_name, "api-gateway");
    }

    #[test]
    fn test_client_focus_never_reorders_payload_sessions() {
        let mut state = SidebarState::default();
        state.my_client_id = 2;
        let mut payload = make_payload(vec![
            make_session(10, "first", 0),
            make_session(20, "second", 1),
            make_session(30, "third", 2),
        ]);
        payload.client_views.insert(
            2,
            cc_deck::ClientViewSnapshot {
                active_tab_index: Some(2),
                focused_pane_id: Some(30),
                revision: 3,
            },
        );
        state.cached_payload = Some(payload);
        state.local_focus_override = Some(30);

        let names = state
            .filtered_sessions()
            .into_iter()
            .map(|session| session.display_name.as_str())
            .collect::<Vec<_>>();
        assert_eq!(names, vec!["first", "second", "third"]);
    }

    #[test]
    fn test_preserve_cursor_falls_back_to_first_session() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![
            make_session(1, "a", 0),
            make_session(2, "b", 1),
        ]));
        state.mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: Some(99),
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };
        state.preserve_cursor();
        assert_eq!(state.mode.cursor_pane_id(), Some(1));
        assert_eq!(state.cursor_index(), 0);
    }

    #[test]
    fn test_preserve_cursor_empty() {
        let mut state = SidebarState::default();
        // No payload, so no sessions
        state.mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };
        state.preserve_cursor();
        assert_eq!(state.cursor_index(), 0);
    }

    #[test]
    fn test_track_cursor_by_pane_id_finds_new_position() {
        let mut state = SidebarState::default();
        // Simulate post-sort session order: pane 20 moved from index 1 to index 0
        state.cached_payload = Some(make_payload(vec![
            make_session(20, "web", 0), // was at index 1, now at 0
            make_session(10, "api", 1), // was at index 0, now at 1
        ]));
        state.mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None, // cursor was at old position of "web"
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };

        state.track_cursor_by_pane_id(20); // Track "web" by pane_id
        assert_eq!(
            state.cursor_index(),
            0,
            "cursor should follow web to position 0"
        );
    }

    #[test]
    fn test_track_cursor_by_pane_id_session_not_found() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![make_session(10, "api", 0)]));
        state.mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };

        // Track a pane_id that doesn't exist in the session list
        state.track_cursor_by_pane_id(99);
        // cursor_index should remain unchanged (preserve_cursor will clamp later)
        assert_eq!(state.cursor_index(), 0);
    }

    #[test]
    fn test_track_cursor_by_pane_id_not_navigating() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![make_session(10, "api", 0)]));
        // In passive mode, track_cursor_by_pane_id is a no-op
        state.track_cursor_by_pane_id(10);
        assert!(matches!(state.mode, SidebarMode::Passive));
    }

    #[test]
    fn test_effective_focus_prefers_override() {
        let mut state = SidebarState::default();
        state.my_client_id = 2;
        let mut payload = make_payload(vec![make_session(10, "api", 0)]);
        payload.client_views.insert(
            2,
            cc_deck::ClientViewSnapshot {
                active_tab_index: Some(1),
                focused_pane_id: Some(20),
                revision: 1,
            },
        );
        state.cached_payload = Some(payload);
        state.local_focus_override = Some(30);
        assert_eq!(state.effective_focused_pane_id(), Some(30));
    }

    #[test]
    fn test_effective_focus_uses_client_view() {
        let mut state = SidebarState::default();
        state.my_client_id = 2;
        let mut payload = make_payload(vec![make_session(10, "api", 0)]);
        payload.client_views.insert(
            2,
            cc_deck::ClientViewSnapshot {
                active_tab_index: Some(1),
                focused_pane_id: Some(20),
                revision: 1,
            },
        );
        state.cached_payload = Some(payload);
        assert_eq!(state.effective_focused_pane_id(), Some(20));
    }

    #[test]
    fn test_effective_focus_unknown_without_client_view() {
        let mut state = SidebarState::default();
        state.my_client_id = 2;
        let payload = make_payload(vec![make_session(10, "api", 0)]);
        state.cached_payload = Some(payload);
        assert_eq!(state.effective_focused_pane_id(), None);
    }
}
