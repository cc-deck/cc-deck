// Sidebar mode state machine for the thin sidebar renderer.
//
// Adapted from crate::state::SidebarMode but simplified for the sidebar
// plugin context. The sidebar does not maintain session state; it only
// manages local UI modes (navigation, rename, filter, help).

/// Grace period (ms) for ignoring stale events after mode entry.
pub const ENTER_GRACE_MS: u64 = 1500;

/// Context shared across all navigation sub-modes.
#[derive(Debug, Clone)]
pub struct NavigateContext {
    /// Cursor position in the session list.
    pub cursor_index: usize,
    /// Pane + tab to restore on Esc.
    pub restore_pane_id: Option<u32>,
    pub restore_tab_index: Option<usize>,
    /// Timestamp (ms) when this mode was entered.
    pub entered_at_ms: u64,
}

/// Transient state for an active inline rename operation.
#[derive(Debug, Clone)]
pub struct RenameState {
    pub pane_id: u32,
    pub input_buffer: String,
    pub cursor_pos: usize,
}

/// Search/filter state during `/` sub-mode in navigation.
#[derive(Debug, Clone, Default)]
pub struct FilterState {
    pub input_buffer: String,
    pub cursor_pos: usize,
}

/// The sidebar interaction mode.
#[derive(Default, Debug, Clone)]
pub enum SidebarMode {
    /// Passive: sidebar displays sessions but captures no input.
    #[default]
    Passive,

    /// Cursor navigation active (amber highlight).
    Navigate(NavigateContext),

    /// Search/filter input active (sub-mode of navigate).
    NavigateFilter {
        ctx: NavigateContext,
        filter: FilterState,
    },

    /// Delete confirmation pending (sub-mode of navigate).
    NavigateDeleteConfirm {
        ctx: NavigateContext,
        pane_id: u32,
    },

    /// Inline rename within navigation (via 'r' key).
    NavigateRename {
        ctx: NavigateContext,
        rename: RenameState,
    },

    /// Rename initiated from passive mode (double-click, right-click).
    RenamePassive {
        rename: RenameState,
        entered_at_ms: u64,
    },

    /// Help overlay: any key dismisses and returns to the previous mode.
    Help(Box<SidebarMode>),
}

impl SidebarMode {
    /// Whether the sidebar should be selectable (captures mouse/keyboard).
    pub fn is_selectable(&self) -> bool {
        !matches!(self, SidebarMode::Passive)
    }

    /// Whether the help overlay is active.
    pub fn is_help(&self) -> bool {
        matches!(self, SidebarMode::Help(_))
    }

    /// Toggle help overlay: push Help on top of current mode, or pop it.
    pub fn toggle_help(&mut self) {
        match std::mem::take(self) {
            SidebarMode::Help(prev) => *self = *prev,
            other => *self = SidebarMode::Help(Box::new(other)),
        }
    }

    /// Dismiss help overlay, restoring the previous mode.
    pub fn dismiss_help(&mut self) {
        if let SidebarMode::Help(prev) = std::mem::take(self) {
            *self = *prev;
        }
    }

    /// Whether we're in any navigation sub-mode.
    pub fn is_navigating(&self) -> bool {
        matches!(
            self,
            SidebarMode::Navigate(_)
                | SidebarMode::NavigateFilter { .. }
                | SidebarMode::NavigateDeleteConfirm { .. }
                | SidebarMode::NavigateRename { .. }
        )
    }

    /// Whether this mode captures keyboard input.
    pub fn is_capturing_input(&self) -> bool {
        !matches!(self, SidebarMode::Passive)
    }

    /// Get navigate context reference (if in any navigate sub-mode).
    pub fn nav_ctx(&self) -> Option<&NavigateContext> {
        match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    /// Get mutable navigate context reference.
    pub fn nav_ctx_mut(&mut self) -> Option<&mut NavigateContext> {
        match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    /// Get cursor index (if in a navigation sub-mode).
    pub fn cursor_index(&self) -> usize {
        self.nav_ctx().map(|ctx| ctx.cursor_index).unwrap_or(0)
    }

    /// Whether within the entry grace period (stale event suppression).
    pub fn in_grace_period(&self, now_ms: u64) -> bool {
        let entered = match self {
            SidebarMode::Navigate(ctx)
            | SidebarMode::NavigateFilter { ctx, .. }
            | SidebarMode::NavigateDeleteConfirm { ctx, .. }
            | SidebarMode::NavigateRename { ctx, .. } => ctx.entered_at_ms,
            SidebarMode::RenamePassive { entered_at_ms, .. } => *entered_at_ms,
            SidebarMode::Help(prev) => return prev.in_grace_period(now_ms),
            SidebarMode::Passive => return false,
        };
        now_ms.saturating_sub(entered) < ENTER_GRACE_MS
    }

    /// Get the rename state if currently renaming.
    pub fn rename_state(&self) -> Option<&RenameState> {
        match self {
            SidebarMode::NavigateRename { rename, .. }
            | SidebarMode::RenamePassive { rename, .. } => Some(rename),
            _ => None,
        }
    }

    /// Get mutable rename state if currently renaming.
    pub fn rename_state_mut(&mut self) -> Option<&mut RenameState> {
        match self {
            SidebarMode::NavigateRename { rename, .. }
            | SidebarMode::RenamePassive { rename, .. } => Some(rename),
            _ => None,
        }
    }

    /// Get the filter state if currently filtering.
    pub fn filter_state(&self) -> Option<&FilterState> {
        match self {
            SidebarMode::NavigateFilter { filter, .. } => Some(filter),
            _ => None,
        }
    }

    /// Get the delete confirm pane_id if pending.
    pub fn delete_confirm_pane(&self) -> Option<u32> {
        match self {
            SidebarMode::NavigateDeleteConfirm { pane_id, .. } => Some(*pane_id),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_passive_not_selectable() {
        assert!(!SidebarMode::Passive.is_selectable());
        assert!(!SidebarMode::Passive.is_navigating());
        assert!(!SidebarMode::Passive.is_capturing_input());
    }

    #[test]
    fn test_navigate_selectable() {
        let mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 0,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 0,
        });
        assert!(mode.is_selectable());
        assert!(mode.is_navigating());
        assert!(mode.is_capturing_input());
        assert_eq!(mode.cursor_index(), 0);
    }

    #[test]
    fn test_grace_period() {
        let mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 0,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 1000,
        });
        assert!(mode.in_grace_period(1100));
        assert!(mode.in_grace_period(2000));
        assert!(!mode.in_grace_period(2600));
    }

    #[test]
    fn test_passive_no_grace() {
        assert!(!SidebarMode::Passive.in_grace_period(0));
        assert!(!SidebarMode::Passive.in_grace_period(u64::MAX));
    }

    #[test]
    fn test_rename_passive_grace() {
        let mode = SidebarMode::RenamePassive {
            rename: RenameState {
                pane_id: 42,
                input_buffer: "test".into(),
                cursor_pos: 4,
            },
            entered_at_ms: 1000,
        };
        assert!(mode.is_selectable());
        assert!(!mode.is_navigating());
        assert!(mode.in_grace_period(1100));
        assert!(!mode.in_grace_period(2600));
    }

    #[test]
    fn test_toggle_help() {
        let mut mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 2,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 0,
        });
        mode.toggle_help();
        assert!(mode.is_help());

        mode.toggle_help();
        assert!(!mode.is_help());
        assert!(mode.is_navigating());
        assert_eq!(mode.cursor_index(), 2);
    }

    #[test]
    fn test_dismiss_help() {
        let mut mode = SidebarMode::Help(Box::new(SidebarMode::Passive));
        mode.dismiss_help();
        assert!(matches!(mode, SidebarMode::Passive));
    }

    #[test]
    fn test_filter_state_access() {
        let mode = SidebarMode::NavigateFilter {
            ctx: NavigateContext {
                cursor_index: 0,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            filter: FilterState {
                input_buffer: "test".into(),
                cursor_pos: 4,
            },
        };
        assert_eq!(mode.filter_state().unwrap().input_buffer, "test");
    }

    #[test]
    fn test_delete_confirm_pane() {
        let mode = SidebarMode::NavigateDeleteConfirm {
            ctx: NavigateContext {
                cursor_index: 0,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            pane_id: 42,
        };
        assert_eq!(mode.delete_confirm_pane(), Some(42));
    }

    #[test]
    fn test_nav_ctx_mut() {
        let mut mode = SidebarMode::Navigate(NavigateContext {
            cursor_index: 0,
            restore_pane_id: None,
            restore_tab_index: None,
            entered_at_ms: 0,
        });
        if let Some(ctx) = mode.nav_ctx_mut() {
            ctx.cursor_index = 5;
        }
        assert_eq!(mode.cursor_index(), 5);
    }
}
