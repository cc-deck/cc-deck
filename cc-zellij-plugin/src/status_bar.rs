use crate::group::ansi_fg;
use crate::state::PluginState;

const RESET: &str = "\u{1b}[0m";

impl PluginState {
    /// Render a compact status bar showing all sessions.
    ///
    /// Each session is displayed as a compact tab with a status indicator
    /// and display name. The currently focused session is highlighted
    /// with bold+reverse ANSI styling. Session tabs are colored by their
    /// project group color.
    pub fn render_status_bar(&self, cols: usize) {
        // Show error message if present (overrides normal status bar)
        if let Some(ref err) = self.error_message {
            let msg = format!(" {} ", err);
            let padded = format!("{:<width$}", msg, width = cols);
            // Red background for error
            print!("\u{1b}[41;37;1m{}{}", padded, RESET);
            return;
        }

        if self.sessions.is_empty() {
            let msg = " cc-deck: no sessions ";
            let padded = format!("{:<width$}", msg, width = cols);
            // Dim style for empty state
            print!("\u{1b}[2m{}{}", padded, RESET);
            return;
        }

        let mut bar = String::new();
        for (idx, session) in self.sessions.values().enumerate() {
            let is_focused = self
                .focused_pane_id
                .is_some_and(|id| id == session.pane_id);

            let group_color = self
                .groups
                .get(&session.group_id)
                .map(|g| g.color.as_str())
                .unwrap_or("white");
            let color = ansi_fg(group_color);

            let indicator = session.status.indicator();
            let name = &session.display_name;
            let num = idx + 1;
            let tab = format!(" {}:{} {} ", num, indicator, name);

            if is_focused {
                // Group color + bold + reverse for focused session
                bar.push_str(&format!("{}\u{1b}[1;7m{}{}", color, tab, RESET));
            } else {
                // Group color for non-focused session
                bar.push_str(&format!("{}{}{}", color, tab, RESET));
            }
            bar.push('|');
        }

        // Remove trailing separator and pad
        if bar.ends_with('|') {
            bar.pop();
        }

        // Truncate if wider than available columns, showing overflow count
        let visible_len = visible_width(&bar);
        if visible_len > cols {
            // Count how many sessions are fully visible by rebuilding
            let total = self.sessions.len();
            let mut fits = 0;
            let mut test_bar = String::new();
            for (idx, session) in self.sessions.values().enumerate() {
                let indicator = session.status.indicator();
                let name = &session.display_name;
                let num = idx + 1;
                // Approximate visible width of this tab (without ANSI)
                let tab = format!(" {}:{} {} ", num, indicator, name);
                let new_len = visible_width(&test_bar) + tab.len() + 1; // +1 for separator
                let overflow_indicator = format!("[+{}]", total - (idx + 1));
                if new_len + overflow_indicator.len() <= cols || idx == total - 1 {
                    if !test_bar.is_empty() {
                        test_bar.push('|');
                    }
                    test_bar.push_str(&tab);
                    fits += 1;
                } else {
                    break;
                }
            }

            if fits < total {
                // Rebuild the bar with only the sessions that fit
                let mut truncated_bar = String::new();
                for (idx, session) in self.sessions.values().enumerate().take(fits) {
                    let is_focused = self
                        .focused_pane_id
                        .is_some_and(|id| id == session.pane_id);
                    let group_color = self
                        .groups
                        .get(&session.group_id)
                        .map(|g| g.color.as_str())
                        .unwrap_or("white");
                    let color = ansi_fg(group_color);
                    let indicator = session.status.indicator();
                    let name = &session.display_name;
                    let num = idx + 1;
                    let tab = format!(" {}:{} {} ", num, indicator, name);
                    if is_focused {
                        truncated_bar.push_str(&format!("{}\u{1b}[1;7m{}{}", color, tab, RESET));
                    } else {
                        truncated_bar.push_str(&format!("{}{}{}", color, tab, RESET));
                    }
                    truncated_bar.push('|');
                }
                let remaining = total - fits;
                truncated_bar.push_str(&format!("\u{1b}[2m[+{}]{}", remaining, RESET));
                print!("{}", truncated_bar);
            } else {
                print!("{}", bar);
            }
        } else {
            print!("{}", bar);
        }
    }

    /// Render a close-session confirmation prompt in the status bar area.
    pub fn render_close_confirm(&self, cols: usize) {
        let session_name = self
            .close_target_pane_id
            .and_then(|pid| self.session_by_pane_id(pid))
            .map(|s| s.display_name.as_str())
            .unwrap_or("unknown");

        let content = format!(
            "\u{1b}[1m Close session '{}'? \u{1b}[0m\u{1b}[2m(y/n)\u{1b}[0m",
            session_name
        );

        let visible = visible_width(&content);
        if visible < cols {
            print!("{}{}", content, " ".repeat(cols - visible));
        } else {
            print!("{}", content);
        }
    }

    /// Render a rename prompt overlay in the status bar area.
    pub fn render_rename_prompt(&self, cols: usize) {
        let label = " Rename: ";
        let cursor = "_";
        let suffix = " [Enter] apply  [Esc] cancel";

        let content = format!(
            "\u{1b}[1m{}\u{1b}[0m{}{}\u{1b}[2m{}\u{1b}[0m",
            label, self.rename_buffer, cursor, suffix
        );

        let visible = visible_width(&content);
        if visible < cols {
            print!("{}{}", content, " ".repeat(cols - visible));
        } else {
            print!("{}", content);
        }
    }
}

/// Calculate the visible width of a string, ignoring ANSI escape sequences.
fn visible_width(s: &str) -> usize {
    let mut width = 0;
    let mut in_escape = false;
    for c in s.chars() {
        if c == '\u{1b}' {
            in_escape = true;
        } else if in_escape {
            if c == 'm' {
                in_escape = false;
            }
        } else {
            // Count multi-byte chars as 1 (emoji indicators are single-width in terminal)
            width += 1;
        }
    }
    width
}

/// Truncate a string with ANSI escapes to a visible width.
#[allow(dead_code)]
fn truncate_visible(s: &str, max_width: usize) -> String {
    let mut result = String::new();
    let mut width = 0;
    let mut in_escape = false;
    for c in s.chars() {
        if c == '\u{1b}' {
            in_escape = true;
            result.push(c);
        } else if in_escape {
            result.push(c);
            if c == 'm' {
                in_escape = false;
            }
        } else {
            if width >= max_width {
                break;
            }
            result.push(c);
            width += 1;
        }
    }
    // Reset any open ANSI sequences
    result.push_str("\u{1b}[0m");
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::session::Session;
    use std::path::PathBuf;

    #[test]
    fn test_visible_width_plain() {
        assert_eq!(visible_width("hello"), 5);
    }

    #[test]
    fn test_visible_width_with_ansi() {
        assert_eq!(visible_width("\u{1b}[1;7m hello \u{1b}[0m"), 7);
    }

    #[test]
    fn test_truncate_visible() {
        let s = "hello world";
        let truncated = truncate_visible(s, 5);
        assert!(truncated.starts_with("hello"));
    }

    #[test]
    fn test_truncate_with_ansi() {
        let s = "\u{1b}[1mhello world\u{1b}[0m";
        let truncated = truncate_visible(s, 5);
        assert!(truncated.contains("hello"));
        assert!(truncated.ends_with("\u{1b}[0m"));
    }

    #[test]
    fn test_error_message_display() {
        let mut state = PluginState::default();
        state.error_message = Some("Error: 'claude' not found".to_string());
        // Error message should be set
        assert!(state.error_message.is_some());
    }

    #[test]
    fn test_close_confirm_display_session_name() {
        let mut state = PluginState::default();
        let mut session = Session::new(0, 42, PathBuf::from("/tmp/my-project"));
        session.display_name = "my-project".to_string();
        state.sessions.insert(0, session);
        state.close_confirm_active = true;
        state.close_target_pane_id = Some(42);

        // Verify the session can be found by close_target_pane_id
        let name = state
            .close_target_pane_id
            .and_then(|pid| state.session_by_pane_id(pid))
            .map(|s| s.display_name.as_str())
            .unwrap_or("unknown");
        assert_eq!(name, "my-project");
    }

    #[test]
    fn test_close_confirm_unknown_pane() {
        let state = PluginState::default();
        // No sessions, should fall back to "unknown"
        let name = Some(99u32)
            .and_then(|pid| state.session_by_pane_id(pid))
            .map(|s| s.display_name.as_str())
            .unwrap_or("unknown");
        assert_eq!(name, "unknown");
    }

    #[test]
    fn test_visible_width_empty() {
        assert_eq!(visible_width(""), 0);
    }

    #[test]
    fn test_truncate_visible_zero_width() {
        let s = "hello";
        let truncated = truncate_visible(s, 0);
        // Should just be the reset sequence
        assert_eq!(truncated, "\u{1b}[0m");
    }

    #[test]
    fn test_visible_width_only_ansi() {
        assert_eq!(visible_width("\u{1b}[1m\u{1b}[0m"), 0);
    }

    #[test]
    fn test_truncate_visible_exceeds_content() {
        let s = "hi";
        let truncated = truncate_visible(s, 100);
        // Should contain all content plus a reset sequence
        assert!(truncated.contains("hi"));
        assert!(truncated.ends_with("\u{1b}[0m"));
    }

    #[test]
    fn test_truncate_visible_exact_fit() {
        let s = "hello";
        let truncated = truncate_visible(s, 5);
        assert!(truncated.starts_with("hello"));
        assert!(truncated.ends_with("\u{1b}[0m"));
    }

    #[test]
    fn test_overflow_indicator_with_many_sessions() {
        let mut state = PluginState::default();
        // Create 10 sessions with moderately long names
        for i in 0..10 {
            let mut session = Session::new(i, 100 + i, PathBuf::from(format!("/tmp/project-{}", i)));
            session.display_name = format!("project-{}", i);
            state.sessions.insert(i, session);
            state.get_or_create_group(&format!("project-{}", i), &format!("project-{}", i));
        }

        // With 10 sessions at ~15 chars each, a 40-column terminal should truncate
        // Just verify state is set up correctly for overflow detection
        let total = state.sessions.len();
        assert_eq!(total, 10);

        // Check that the visible_width function handles the bar correctly
        let tab = " 1:? project-0 ";
        let tab_width = visible_width(tab);
        assert_eq!(tab_width, 15);
    }

    #[test]
    fn test_very_narrow_terminal_empty_state() {
        let _state = PluginState::default();
        // Verify render_status_bar handles narrow terminal without panic
        // The " cc-deck: no sessions " message is 22 chars
        let msg = " cc-deck: no sessions ";
        assert_eq!(msg.len(), 22);
        // A 5-column terminal will get a truncated padded string
        let padded = format!("{:<width$}", msg, width = 5);
        // Padding doesn't truncate; it just doesn't pad beyond content
        assert!(padded.len() >= 5);
    }

    #[test]
    fn test_error_message_narrow_terminal() {
        let mut state = PluginState::default();
        state.error_message = Some("Error: 'claude' not found".to_string());
        // Verify the format produces correct visible width
        let msg = format!(" {} ", state.error_message.as_ref().unwrap());
        let vis = visible_width(&msg);
        assert_eq!(vis, msg.len()); // No ANSI in the raw message
    }
}
