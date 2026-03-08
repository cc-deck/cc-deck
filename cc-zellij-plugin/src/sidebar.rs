// T011-T013: Sidebar rendering with activity indicators, click-to-switch, empty state

use crate::session::Session;
use crate::state::{PluginState, RenameState};

/// Click region: (row, pane_id, tab_index).
pub type ClickRegion = (usize, u32, usize);

/// Render the sidebar into the plugin's stdout.
/// Returns click regions for mouse handling.
pub fn render_sidebar(state: &PluginState, rows: usize, cols: usize) -> Vec<ClickRegion> {
    let sessions = state.sessions_by_tab_order();
    let mut click_regions = Vec::new();

    if sessions.is_empty() {
        render_empty_state(rows, cols);
        return click_regions;
    }

    // Header
    print_line(0, cols, "CC-DECK", Style::Header);

    // Separator
    let sep: String = "\u{2500}".repeat(cols.min(40));
    print_line(1, cols, &sep, Style::Dim);

    // Available rows for sessions (header + sep at top)
    let content_start = 2;
    let content_end = rows;
    let available = content_end.saturating_sub(content_start);

    // Each session takes 3 lines (indicator+name, branch, blank)
    let lines_per_session = 3;
    let max_visible = if lines_per_session > 0 {
        available / lines_per_session
    } else {
        0
    };

    let total = sessions.len();
    let (start_idx, end_idx, above_count, below_count) =
        visible_range(total, max_visible, state.active_tab_index, &sessions);

    let mut row = content_start;

    // Overflow indicator (above)
    if above_count > 0 {
        let msg = format!("  \u{25b2} +{above_count}");
        print_line(row, cols, &msg, Style::Dim);
        row += 1;
    }

    // Render visible sessions
    for session in &sessions[start_idx..end_idx] {
        if row >= content_end {
            break;
        }
        let is_active = state
            .active_tab_index
            .map(|idx| session.tab_index == Some(idx))
            .unwrap_or(false);

        // Check if this session is being renamed
        let rename_for_session = state.rename_state.as_ref().filter(|rs| rs.pane_id == session.pane_id);

        if let Some(region) = render_session_entry(session, is_active, row, cols, rename_for_session) {
            click_regions.push(region);
        }
        row += lines_per_session;
    }

    // Overflow indicator (below)
    if below_count > 0 && row < content_end {
        let msg = format!("  \u{25bc} +{below_count}");
        print_line(row, cols, &msg, Style::Dim);
        row += 1;
    }

    // Fill remaining rows
    while row < rows {
        print_line(row, cols, "", Style::Normal);
        row += 1;
    }

    click_regions
}

/// Compute the visible range of sessions, keeping the active one visible.
fn visible_range(
    total: usize,
    max_visible: usize,
    active_tab_index: Option<usize>,
    sessions: &[&Session],
) -> (usize, usize, usize, usize) {
    if total == 0 || max_visible == 0 {
        return (0, 0, 0, 0);
    }
    if total <= max_visible {
        return (0, total, 0, 0);
    }

    let active_pos = active_tab_index
        .and_then(|idx| sessions.iter().position(|s| s.tab_index == Some(idx)))
        .unwrap_or(0);

    let half = max_visible / 2;
    let start = if active_pos <= half {
        0
    } else if active_pos + half >= total {
        total.saturating_sub(max_visible)
    } else {
        active_pos - half
    };
    let end = (start + max_visible).min(total);

    (start, end, start, total.saturating_sub(end))
}

/// Render a single session entry (3 lines: indicator+name, branch, blank).
fn render_session_entry(
    session: &Session,
    is_active: bool,
    start_row: usize,
    cols: usize,
    rename_state: Option<&RenameState>,
) -> Option<ClickRegion> {
    let indicator = session.activity.indicator();
    let (r, g, b) = session.activity.color();

    // Line 1: indicator + name (or rename input buffer)
    let line1 = if let Some(rs) = rename_state {
        // Render rename input with cursor
        let prefix = format!("\x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m ");
        let max_input = cols.saturating_sub(2); // indicator + space
        let buf = &rs.input_buffer;
        let cursor_pos = rs.cursor_pos.min(buf.len());

        let before = &buf[..cursor_pos];
        let cursor_char = buf.get(cursor_pos..cursor_pos + 1).unwrap_or(" ");
        let after = if cursor_pos < buf.len() { &buf[cursor_pos + 1..] } else { "" };

        // Truncate if needed (simple approach)
        let input_display = if buf.len() <= max_input {
            format!("{before}\x1b[7m{cursor_char}\x1b[0m{after}")
        } else {
            let truncated = truncate(buf, max_input);
            truncated.to_string()
        };

        format!("{prefix}{input_display}")
    } else {
        let elapsed = session.elapsed_display().unwrap_or_default();
        let name = &session.display_name;

        let prefix_len = 2; // indicator + space
        let elapsed_len = if elapsed.is_empty() { 0 } else { elapsed.len() + 1 };
        let max_name = cols.saturating_sub(prefix_len + elapsed_len);
        let truncated_name = truncate(name, max_name);

        let name_part = if is_active {
            format!("\x1b[1m{truncated_name}\x1b[0m")
        } else {
            truncated_name.to_string()
        };

        if elapsed.is_empty() {
            format!("\x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {name_part}")
        } else {
            format!("\x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {name_part} \x1b[2m{elapsed}\x1b[0m")
        }
    };

    if is_active {
        print!("\x1b[{};1H\x1b[48;5;236m{}\x1b[0m", start_row + 1, pad(&line1, cols));
    } else {
        print!("\x1b[{};1H{}", start_row + 1, pad(&line1, cols));
    }

    // Line 2: branch or tool info (dimmed)
    let line2 = if let Some(ref branch) = session.git_branch {
        format!("  \x1b[2m{branch}\x1b[0m")
    } else if let crate::session::Activity::ToolUse(ref tool) = session.activity {
        format!("  \x1b[2m{tool}\x1b[0m")
    } else {
        String::new()
    };

    if is_active {
        print!("\x1b[{};1H\x1b[48;5;236m{}\x1b[0m", start_row + 2, pad(&line2, cols));
    } else {
        print!("\x1b[{};1H{}", start_row + 2, pad(&line2, cols));
    }

    // Line 3: blank separator
    print!("\x1b[{};1H{}", start_row + 3, " ".repeat(cols));

    // Click region covers lines 1-2
    session.tab_index.map(|tab_idx| (start_row, session.pane_id, tab_idx))
}

/// Render the empty state (no Claude sessions).
fn render_empty_state(rows: usize, cols: usize) {
    print_line(0, cols, "CC-DECK", Style::Header);

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print_line(1, cols, &sep, Style::Dim);

    let messages = [
        "",
        "  No Claude sessions",
        "",
        "  Start Claude Code in",
        "  a terminal tab to",
        "  see sessions here.",
        "",
        "  Hooks must be",
        "  installed via:",
        "  cc-deck install",
    ];

    for (i, msg) in messages.iter().enumerate() {
        let row = 2 + i;
        if row >= rows {
            break;
        }
        print_line(row, cols, msg, Style::Dim);
    }

    for row in (2 + messages.len())..rows {
        print_line(row, cols, "", Style::Normal);
    }
}

/// Handle a mouse click on the sidebar.
/// Returns (tab_index, pane_id) if a session was clicked.
pub fn handle_click(click_row: usize, click_regions: &[ClickRegion]) -> Option<(usize, u32)> {
    for &(row, pane_id, tab_index) in click_regions {
        if click_row >= row && click_row < row + 3 {
            return Some((tab_index, pane_id));
        }
    }
    None
}

// --- Rendering helpers ---

enum Style {
    Header,
    Dim,
    Normal,
}

fn print_line(row: usize, cols: usize, text: &str, style: Style) {
    let styled = match style {
        Style::Header => format!("\x1b[1m{text}\x1b[0m"),
        Style::Dim => format!("\x1b[2m{text}\x1b[0m"),
        Style::Normal => text.to_string(),
    };
    print!("\x1b[{};1H{}", row + 1, pad(&styled, cols));
}

fn pad(s: &str, width: usize) -> String {
    let display_len = display_width(s);
    if display_len >= width {
        s.to_string()
    } else {
        format!("{}{}", s, " ".repeat(width - display_len))
    }
}

fn display_width(s: &str) -> usize {
    let mut width = 0;
    let mut in_escape = false;
    for ch in s.chars() {
        if ch == '\x1b' {
            in_escape = true;
        } else if in_escape {
            if ch == 'm' {
                in_escape = false;
            }
        } else {
            width += 1;
        }
    }
    width
}

fn truncate(s: &str, max: usize) -> String {
    if max <= 3 {
        return ".".repeat(max);
    }
    if s.len() <= max {
        return s.to_string();
    }
    let mut result = String::new();
    for (width, ch) in s.chars().enumerate() {
        if width + 4 > max {
            result.push_str("...");
            break;
        }
        result.push(ch);
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_display_width() {
        assert_eq!(display_width("hello"), 5);
        assert_eq!(display_width("\x1b[1mhello\x1b[0m"), 5);
        assert_eq!(display_width("\x1b[38;2;255;0;0mX\x1b[0m text"), 6);
    }

    #[test]
    fn test_truncate() {
        assert_eq!(truncate("short", 10), "short");
        assert_eq!(truncate("a-very-long-name", 10), "a-very-...");
        assert_eq!(truncate("ab", 2), "..");
    }

    #[test]
    fn test_pad() {
        assert_eq!(pad("hi", 5), "hi   ");
        assert_eq!(pad("\x1b[1mhi\x1b[0m", 5), "\x1b[1mhi\x1b[0m   ");
    }

    #[test]
    fn test_visible_range_all_fit() {
        let s1 = Session::new(1, "a".into());
        let s2 = Session::new(2, "b".into());
        let sessions: Vec<&Session> = vec![&s1, &s2];
        let (start, end, above, below) = visible_range(2, 5, None, &sessions);
        assert_eq!((start, end, above, below), (0, 2, 0, 0));
    }

    #[test]
    fn test_visible_range_overflow() {
        let s: Vec<Session> = (0..10).map(|i| {
            let mut s = Session::new(i, format!("s{i}"));
            s.tab_index = Some(i as usize);
            s
        }).collect();
        let refs: Vec<&Session> = s.iter().collect();
        let (start, end, above, below) = visible_range(10, 3, Some(5), &refs);
        assert_eq!(end - start, 3);
        assert!(start <= 5 && end > 5); // active session visible
        assert!(above > 0 || below > 0); // some overflow
    }

    #[test]
    fn test_handle_click_hit() {
        let regions: Vec<ClickRegion> = vec![
            (2, 1, 0),
            (5, 2, 1),
        ];
        assert_eq!(handle_click(2, &regions), Some((0, 1)));
        assert_eq!(handle_click(3, &regions), Some((0, 1)));
        assert_eq!(handle_click(5, &regions), Some((1, 2)));
        assert_eq!(handle_click(10, &regions), None);
    }
}
