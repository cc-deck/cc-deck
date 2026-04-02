// Sidebar renderer plugin state.
//
// Contains the cached render payload from the controller and all local
// UI state (mode, click regions, scroll, filter). Does NOT hold session
// data directly; sessions are received pre-computed in RenderPayload.

use cc_deck::{RenderPayload, RenderSession};
use crate::config::PluginConfig;
use super::modes::{SidebarMode, NavigateContext};

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

    /// The controller plugin's ID (learned from sidebar-init or render payload).
    pub controller_plugin_id: Option<u32>,

    /// Scroll offset for overflow rendering.
    pub scroll_offset: usize,

    /// Current filter text (applied locally to cached sessions).
    pub filter_text: String,

    /// Transient notification to display.
    pub notification: Option<Notification>,

    /// Plugin configuration.
    pub config: PluginConfig,

    /// Whether the first render payload has been received.
    pub initialized: bool,

    /// Whether the sidebar-hello handshake has been sent.
    pub hello_sent: bool,

    /// Whether plugin permissions have been granted.
    pub permissions_granted: bool,

    /// Last left-click timestamp (ms) and pane_id for double-click detection.
    pub last_click: Option<(u64, u32)>,

    /// Predictive focus override set locally when the sidebar sends a Switch
    /// action. Provides immediate highlight without waiting for the controller
    /// to confirm the focus change in the next RenderPayload.
    pub local_focus_override: Option<u32>,
}

impl Default for SidebarState {
    fn default() -> Self {
        Self {
            cached_payload: None,
            mode: SidebarMode::Passive,
            click_regions: Vec::new(),
            my_tab_index: None,
            my_plugin_id: 0,
            controller_plugin_id: None,
            scroll_offset: 0,
            filter_text: String::new(),
            notification: None,
            config: PluginConfig::default(),
            initialized: false,
            hello_sent: false,
            permissions_granted: false,
            last_click: None,
            local_focus_override: None,
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

        if self.filter_text.is_empty() {
            // Also check mode-level filter state
            if let Some(fs) = self.mode.filter_state() {
                if fs.input_buffer.is_empty() {
                    return payload.sessions.iter().collect();
                }
                let lower = fs.input_buffer.to_lowercase();
                return payload.sessions.iter()
                    .filter(|s| s.display_name.to_lowercase().contains(&lower))
                    .collect();
            }
            return payload.sessions.iter().collect();
        }

        let lower = self.filter_text.to_lowercase();
        payload.sessions.iter()
            .filter(|s| s.display_name.to_lowercase().contains(&lower))
            .collect()
    }

    /// Count sessions matching a filter string.
    pub fn filtered_session_count(&self, filter: &str) -> usize {
        let payload = match &self.cached_payload {
            Some(p) => p,
            None => return 0,
        };
        if filter.is_empty() {
            return payload.sessions.len();
        }
        let lower = filter.to_lowercase();
        payload.sessions.iter()
            .filter(|s| s.display_name.to_lowercase().contains(&lower))
            .count()
    }

    /// Get the active tab index from the cached payload.
    pub fn active_tab_index(&self) -> Option<usize> {
        self.cached_payload.as_ref().map(|p| p.active_tab_index)
    }

    /// Get the focused pane ID from the cached payload.
    pub fn focused_pane_id(&self) -> Option<u32> {
        self.cached_payload.as_ref().and_then(|p| p.focused_pane_id)
    }

    /// Get the effective focused pane ID, preferring the local override
    /// (set when the sidebar sends a Switch action) over the payload value.
    /// This provides immediate highlight without waiting for controller confirmation.
    pub fn effective_focused_pane_id(&self) -> Option<u32> {
        self.local_focus_override.or_else(|| {
            self.cached_payload.as_ref().and_then(|p| p.focused_pane_id)
        })
    }

    /// Preserve cursor position by clamping after session list changes.
    pub fn preserve_cursor(&mut self) {
        let count = self.filtered_sessions().len();
        let clamp = |ctx: &mut NavigateContext| {
            if count == 0 {
                ctx.cursor_index = 0;
            } else if ctx.cursor_index >= count {
                ctx.cursor_index = count - 1;
            }
        };
        match &mut self.mode {
            SidebarMode::Help(inner) => {
                if let Some(ctx) = inner.nav_ctx_mut() {
                    clamp(ctx);
                }
            }
            _ => {
                if let Some(ctx) = self.mode.nav_ctx_mut() {
                    clamp(ctx);
                }
            }
        }
    }

    /// Create a notification that expires after the given duration.
    pub fn set_notification(&mut self, message: &str, duration_secs: u64) {
        let now = crate::session::unix_now();
        self.notification = Some(Notification {
            message: message.to_string(),
            expires_at: now + duration_secs,
        });
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
    use super::*;
    use cc_deck::RenderPayload;

    fn make_payload(sessions: Vec<RenderSession>) -> RenderPayload {
        let total = sessions.len();
        RenderPayload {
            sessions,
            focused_pane_id: None,
            active_tab_index: 0,
            notification: None,
            notification_expiry: None,
            total,
            waiting: 0,
            working: 0,
            idle: 0,
            controller_plugin_id: 1,
        }
    }

    fn make_session(pane_id: u32, name: &str, tab_index: usize) -> RenderSession {
        RenderSession {
            pane_id,
            display_name: name.to_string(),
            activity_label: "Idle".to_string(),
            indicator: "\u{25cb}".to_string(),
            color: (180, 175, 195),
            git_branch: None,
            tab_index,
            paused: false,
            done_attended: false,
        }
    }

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
    fn test_filtered_session_count() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![
            make_session(1, "api", 0),
            make_session(2, "web", 1),
        ]));
        assert_eq!(state.filtered_session_count(""), 2);
        assert_eq!(state.filtered_session_count("api"), 1);
        assert_eq!(state.filtered_session_count("xyz"), 0);
    }

    #[test]
    fn test_preserve_cursor_clamps() {
        let mut state = SidebarState::default();
        state.cached_payload = Some(make_payload(vec![
            make_session(1, "a", 0),
            make_session(2, "b", 1),
        ]));
        state.mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 5,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 0,
        });
        state.preserve_cursor();
        assert_eq!(state.mode.cursor_index(), 1);
    }

    #[test]
    fn test_preserve_cursor_empty() {
        let mut state = SidebarState::default();
        // No payload, so no sessions
        state.mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 3,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 0,
        });
        state.preserve_cursor();
        assert_eq!(state.mode.cursor_index(), 0);
    }

    #[test]
    fn test_active_tab_index() {
        let mut state = SidebarState::default();
        assert!(state.active_tab_index().is_none());

        state.cached_payload = Some(make_payload(vec![]));
        assert_eq!(state.active_tab_index(), Some(0));
    }

    #[test]
    fn test_set_notification() {
        let mut state = SidebarState::default();
        state.set_notification("test msg", 5);
        assert!(state.notification.is_some());
        assert_eq!(state.notification.as_ref().unwrap().message, "test msg");
    }
}
