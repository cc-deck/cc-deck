use crate::state::PluginState;

impl PluginState {
    /// Render a compact status bar showing all sessions.
    ///
    /// Each session is displayed as a compact tab with a status indicator
    /// and display name. The currently focused session is highlighted
    /// with bold+reverse ANSI styling.
    pub fn render_status_bar(&self, cols: usize) {
        if self.sessions.is_empty() {
            let msg = " cc-deck: no sessions ";
            let padded = format!("{:<width$}", msg, width = cols);
            // Dim style for empty state
            print!("\u{1b}[2m{}\u{1b}[0m", padded);
            return;
        }

        let mut bar = String::new();
        for (idx, session) in self.sessions.values().enumerate() {
            let is_focused = self
                .focused_pane_id
                .is_some_and(|id| id == session.pane_id);

            let indicator = session.status.indicator();
            let name = &session.display_name;
            let num = idx + 1;
            let tab = format!(" {}:{} {} ", num, indicator, name);

            if is_focused {
                // Bold + reverse for focused session
                bar.push_str(&format!("\u{1b}[1;7m{}\u{1b}[0m", tab));
            } else {
                bar.push_str(&tab);
            }
            bar.push('|');
        }

        // Remove trailing separator and pad
        if bar.ends_with('|') {
            bar.pop();
        }

        // Truncate if wider than available columns
        let visible_len = visible_width(&bar);
        if visible_len > cols {
            // Simple truncation: re-build with shorter names
            let truncated = truncate_visible(&bar, cols.saturating_sub(1));
            print!("{}…", truncated);
        } else {
            print!("{}", bar);
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
}
