// Sidebar mode state machine for the thin sidebar renderer.
//
// The sidebar does not maintain session state; it only manages local
// UI modes (navigation, rename, filter, help).

/// Grace period (ms) for ignoring stale events after mode entry.
pub const ENTER_GRACE_MS: u64 = 1500;

/// Context shared across all navigation sub-modes.
#[derive(Debug, Clone)]
pub struct NavigateContext {
    /// Stable cursor identity. Its display index is derived after every
    /// filter/order update, so payload reordering cannot move the selection.
    pub cursor_pane_id: Option<u32>,
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

#[derive(Debug, Clone, Default)]
pub enum NavigationOverlay {
    #[default]
    None,
    Filter(FilterState),
    DeleteConfirm(u32),
    Rename(RenameState),
    Help,
}

/// The sidebar interaction mode.
#[derive(Default, Debug, Clone)]
pub enum SidebarMode {
    /// Passive: sidebar displays sessions but captures no input.
    #[default]
    Passive,

    /// Cursor navigation active (amber highlight).
    Navigate {
        ctx: NavigateContext,
        overlay: NavigationOverlay,
    },

    /// Rename initiated from passive mode (double-click, right-click).
    RenamePassive {
        rename: RenameState,
        entered_at_ms: u64,
    },
}

impl SidebarMode {
    /// Whether the sidebar should be selectable (captures mouse/keyboard).
    pub fn is_selectable(&self) -> bool {
        !matches!(self, SidebarMode::Passive)
    }

    /// Whether the help overlay is active.
    pub fn is_help(&self) -> bool {
        matches!(
            self,
            SidebarMode::Navigate {
                overlay: NavigationOverlay::Help,
                ..
            }
        )
    }

    /// Toggle help overlay: push Help on top of current mode, or pop it.
    pub fn toggle_help(&mut self) {
        if let SidebarMode::Navigate { overlay, .. } = self {
            *overlay = if matches!(overlay, NavigationOverlay::Help) {
                NavigationOverlay::None
            } else {
                NavigationOverlay::Help
            };
        }
    }

    /// Dismiss help overlay, restoring the previous mode.
    pub fn dismiss_help(&mut self) {
        if let SidebarMode::Navigate { overlay, .. } = self {
            if matches!(overlay, NavigationOverlay::Help) {
                *overlay = NavigationOverlay::None;
            }
        }
    }

    /// Whether we're in any navigation sub-mode.
    pub fn is_navigating(&self) -> bool {
        matches!(self, SidebarMode::Navigate { .. })
    }

    /// Get navigate context reference (if in any navigate sub-mode).
    pub fn nav_ctx(&self) -> Option<&NavigateContext> {
        match self {
            SidebarMode::Navigate { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    /// Get mutable navigate context reference.
    pub fn nav_ctx_mut(&mut self) -> Option<&mut NavigateContext> {
        match self {
            SidebarMode::Navigate { ctx, .. } => Some(ctx),
            _ => None,
        }
    }

    pub fn cursor_pane_id(&self) -> Option<u32> {
        self.nav_ctx().and_then(|ctx| ctx.cursor_pane_id)
    }

    /// Whether within the entry grace period (stale event suppression).
    pub fn in_grace_period(&self, now_ms: u64) -> bool {
        let entered = match self {
            SidebarMode::Navigate { ctx, .. } => ctx.entered_at_ms,
            SidebarMode::RenamePassive { entered_at_ms, .. } => *entered_at_ms,
            SidebarMode::Passive => return false,
        };
        now_ms.saturating_sub(entered) < ENTER_GRACE_MS
    }

    /// Get the rename state if currently renaming.
    pub fn rename_state(&self) -> Option<&RenameState> {
        match self {
            SidebarMode::Navigate {
                overlay: NavigationOverlay::Rename(rename),
                ..
            }
            | SidebarMode::RenamePassive { rename, .. } => Some(rename),
            _ => None,
        }
    }

    /// Get the filter state if currently filtering.
    pub fn filter_state(&self) -> Option<&FilterState> {
        match self {
            SidebarMode::Navigate {
                overlay: NavigationOverlay::Filter(filter),
                ..
            } => Some(filter),
            _ => None,
        }
    }

    /// Get the delete confirm pane_id if pending.
    pub fn delete_confirm_pane(&self) -> Option<u32> {
        match self {
            SidebarMode::Navigate {
                overlay: NavigationOverlay::DeleteConfirm(pane_id),
                ..
            } => Some(*pane_id),
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
    }

    #[test]
    fn test_navigate_selectable() {
        let mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };
        assert!(mode.is_selectable());
        assert!(mode.is_navigating());
        assert_eq!(mode.cursor_pane_id(), None);
    }

    #[test]
    fn test_grace_period() {
        let mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 1000,
            },
            overlay: NavigationOverlay::None,
        };
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
        let mut mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };
        mode.toggle_help();
        assert!(mode.is_help());

        mode.toggle_help();
        assert!(!mode.is_help());
        assert!(mode.is_navigating());
        assert_eq!(mode.cursor_pane_id(), None);
    }

    #[test]
    fn test_dismiss_help() {
        let mut mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::Help,
        };
        mode.dismiss_help();
        assert!(mode.is_navigating());
    }

    #[test]
    fn test_filter_state_access() {
        let mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::Filter(FilterState {
                input_buffer: "test".into(),
                cursor_pos: 4,
            }),
        };
        assert_eq!(mode.filter_state().unwrap().input_buffer, "test");
    }

    #[test]
    fn test_delete_confirm_pane() {
        let mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::DeleteConfirm(42),
        };
        assert_eq!(mode.delete_confirm_pane(), Some(42));
    }

    #[test]
    fn test_nav_ctx_mut() {
        let mut mode = SidebarMode::Navigate {
            ctx: NavigateContext {
                cursor_pane_id: None,
                restore_pane_id: None,
                restore_tab_index: None,
                entered_at_ms: 0,
            },
            overlay: NavigationOverlay::None,
        };
        if let Some(ctx) = mode.nav_ctx_mut() {
            ctx.cursor_pane_id = Some(5);
        }
        assert_eq!(mode.cursor_pane_id(), Some(5));
    }
}
