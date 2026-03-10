// T011-T013: Sidebar rendering with activity indicators, click-to-switch, empty state

use crate::session::{Activity, Session};
use crate::state::{PluginState, RenameState};

/// Click region: (row, pane_id, tab_index).
pub type ClickRegion = (usize, u32, usize);

// Active session highlight: dark teal background with brighter teal foreground
const ACTIVE_BG: &str = "\x1b[48;2;25;45;55m"; // dark teal background
const ACTIVE_FG: &str = "\x1b[38;2;120;200;220m"; // bright teal foreground
// Navigation cursor: warm amber tint background
const CURSOR_BG: &str = "\x1b[48;2;50;40;20m"; // muted amber background
const CURSOR_FG: &str = "\x1b[38;2;230;200;140m"; // warm amber foreground
const RESET: &str = "\x1b[0m";

/// Render the status header with orange star and session counts.
fn render_header(state: &PluginState, cols: usize) {
    let sessions: Vec<_> = state.sessions.values().collect();
    let total = sessions.len();

    if total == 0 {
        // Orange star + "Claude Code" (space-aligned with session indicators)
        let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mClaude Code\x1b[0m".to_string();
        print!("\x1b[1;1H{}", pad(&header, cols));
    } else {
        let waiting = sessions.iter().filter(|s| s.activity.is_waiting()).count();
        let working = sessions.iter().filter(|s| matches!(s.activity, Activity::Working | Activity::ToolUse(_))).count();
        let idle = sessions.iter().filter(|s| matches!(s.activity, Activity::Done | Activity::AgentDone | Activity::Idle | Activity::Init)).count();

        // Orange star + total | status counts
        let mut status_parts = Vec::new();
        if waiting > 0 {
            status_parts.push(format!("\x1b[38;2;255;60;60m\u{26a0} {waiting}\x1b[0m"));
        }
        if working > 0 {
            status_parts.push(format!("\x1b[38;2;180;140;255m\u{25cf} {working}\x1b[0m"));
        }
        if idle > 0 {
            status_parts.push(format!("\x1b[2m\u{25cb} {idle}\x1b[0m"));
        }

        let status = if status_parts.is_empty() {
            format!("{total}")
        } else {
            format!("{total} \x1b[2m\u{2502}\x1b[0m {}", status_parts.join(" "))
        };
        let header = format!(" \x1b[38;2;255;170;50m\u{2731}\x1b[0m {status}");
        print!("\x1b[1;1H{}", pad(&header, cols));
    }

    // Separator
    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!("\x1b[2;1H\x1b[2m{}\x1b[0m{}", sep, " ".repeat(cols.saturating_sub(sep.len())));
}

/// Render the sidebar into the plugin's stdout.
/// Returns click regions for mouse handling.
pub fn render_sidebar(state: &PluginState, rows: usize, cols: usize) -> Vec<ClickRegion> {
    if state.show_help {
        return render_help_overlay(rows, cols);
    }
    // Use filtered sessions when filter is active
    let sessions = if state.filter_state.is_some() {
        let filter = state.filter_state.as_ref().map(|f| &f.input_buffer).unwrap();
        if filter.is_empty() {
            state.sessions_by_tab_order()
        } else {
            let lower = filter.to_lowercase();
            let mut filtered: Vec<_> = state.sessions.values()
                .filter(|s| s.display_name.to_lowercase().contains(&lower))
                .collect();
            filtered.sort_by_key(|s| s.tab_index.unwrap_or(usize::MAX));
            filtered
        }
    } else {
        state.sessions_by_tab_order()
    };
    let mut click_regions = Vec::new();

    if sessions.is_empty() && state.filter_state.is_none() {
        return render_empty_state(state, rows, cols);
    }

    // Header with status
    render_header(state, cols);

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
    for (list_idx, session) in sessions[start_idx..end_idx].iter().enumerate() {
        if row >= content_end {
            break;
        }
        let abs_idx = start_idx + list_idx;

        // Highlight the focused pane's session
        let is_active = if let Some(focused_id) = state.focused_pane_id {
            session.pane_id == focused_id
        } else {
            state.active_tab_index
                .map(|idx| session.tab_index == Some(idx))
                .unwrap_or(false)
        };

        // Navigation mode: show cursor on the selected session
        let has_cursor = state.navigation_mode && abs_idx == state.cursor_index;

        // Check if this session has a pending delete confirmation
        let is_delete_confirm = state.delete_confirm == Some(session.pane_id);

        if is_delete_confirm {
            // Render delete confirmation instead of normal entry
            let prompt = format!(" Delete \"{}\"?", truncate(&session.display_name, cols.saturating_sub(14)));
            let confirm_hint = " [y/N]";
            print!("\x1b[{};1H{}", row + 1, pad(&format!("\x1b[38;2;255;60;60m{prompt}\x1b[0m"), cols));
            print!("\x1b[{};1H{}", row + 2, pad(&format!("\x1b[2m{confirm_hint}\x1b[0m"), cols));
            print!("\x1b[{};1H{}", row + 3, " ".repeat(cols));
            if let Some(tab_idx) = session.tab_index {
                click_regions.push((row, session.pane_id, tab_idx));
            }
        } else {
            // Check if this session is being renamed
            let rename_for_session = state.rename_state.as_ref().filter(|rs| rs.pane_id == session.pane_id);

            if let Some(region) = render_session_entry(session, is_active, has_cursor, row, cols, rename_for_session) {
                click_regions.push(region);
            }
        }
        row += lines_per_session;
    }

    // Overflow indicator (below)
    if below_count > 0 && row < content_end {
        let msg = format!("  \u{25bc} +{below_count}");
        print_line(row, cols, &msg, Style::Dim);
        row += 1;
    }

    // Bottom row: search input (when filtering) or [+] button
    if let Some(ref fs) = state.filter_state {
        // Render search input
        if row < rows.saturating_sub(1) {
            let prefix = " / ";
            let max_input = cols.saturating_sub(prefix.len());
            let buf = &fs.input_buffer;
            let cursor_pos = fs.cursor_pos.min(buf.len());
            let before = &buf[..cursor_pos];
            let cursor_char = buf.get(cursor_pos..cursor_pos + 1).unwrap_or(" ");
            let after = if cursor_pos < buf.len() { &buf[cursor_pos + 1..] } else { "" };
            let input_display = if buf.len() <= max_input {
                format!("{before}\x1b[7m{cursor_char}\x1b[0m{after}")
            } else {
                truncate(buf, max_input).to_string()
            };
            let search_line = format!("\x1b[2m{prefix}\x1b[0m{input_display}");
            print!("\x1b[{};1H{}", row + 1, pad(&search_line, cols));
            row += 1;
        }
    } else if row < rows.saturating_sub(1) {
        let btn = "  [+] New tab";
        print_line(row, cols, btn, Style::Dim);
        click_regions.push((row, u32::MAX, usize::MAX));
        row += 1;
    }

    // Render notification right below session list (if active)
    if let Some(ref notif) = state.notification {
        if !crate::notification::is_expired(notif) && row < rows {
            crate::notification::render_notification(notif, row, cols);
            row += 1;
        }
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
    has_cursor: bool,
    start_row: usize,
    cols: usize,
    rename_state: Option<&RenameState>,
) -> Option<ClickRegion> {
    // Paused sessions show ⏸ icon with grey color
    let (indicator, r, g, b) = if session.paused {
        ("⏸", 100u8, 100u8, 100u8)
    } else {
        let (r, g, b) = session.activity.color();
        (session.activity.indicator(), r, g, b)
    };

    // Line 1: indicator + name (or rename input buffer)
    let line1 = if let Some(rs) = rename_state {
        // Render rename input with amber bg when in navigation mode
        let rename_bg = if has_cursor { CURSOR_BG } else { "" };
        let rename_fg = if has_cursor { CURSOR_FG } else { "" };
        let prefix = format!("{rename_bg} \x1b[38;2;{r};{g};{b}m{indicator}{rename_fg} ");
        let max_input = cols.saturating_sub(3); // space + indicator + space
        let buf = &rs.input_buffer;
        let cursor_pos = rs.cursor_pos.min(buf.len());

        let before = &buf[..cursor_pos];
        let cursor_char = buf.get(cursor_pos..cursor_pos + 1).unwrap_or(" ");
        let after = if cursor_pos < buf.len() { &buf[cursor_pos + 1..] } else { "" };

        // Truncate if needed (simple approach)
        let input_display = if buf.len() <= max_input {
            format!("{before}\x1b[7m{cursor_char}\x1b[0m{rename_bg}{rename_fg}{after}")
        } else {
            let truncated = truncate(buf, max_input);
            truncated.to_string()
        };

        format!("{prefix}{input_display}{RESET}")
    } else {
        let elapsed = session.elapsed_display().unwrap_or_default();
        let name = &session.display_name;

        let prefix_len = 2; // indicator + space
        let elapsed_len = if elapsed.is_empty() { 0 } else { elapsed.len() + 1 };
        let max_name = cols.saturating_sub(prefix_len + elapsed_len);
        let truncated_name = truncate(name, max_name);

        let name_part = if session.paused {
            format!("\x1b[2m{truncated_name}\x1b[0m") // dimmed for paused
        } else if is_active {
            format!("\x1b[1m{truncated_name}\x1b[0m")
        } else {
            truncated_name.to_string()
        };

        if elapsed.is_empty() {
            format!(" \x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {name_part}")
        } else {
            format!(" \x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {name_part} \x1b[2m{elapsed}\x1b[0m")
        }
    };

    // Determine background style: cursor > active > normal
    let (bg, fg, use_bg) = if has_cursor {
        (CURSOR_BG, CURSOR_FG, true)
    } else if is_active {
        (ACTIVE_BG, ACTIVE_FG, true)
    } else {
        ("", "", false)
    };

    if use_bg {
        let elapsed = session.elapsed_display().unwrap_or_default();
        let name = &session.display_name;
        let prefix_len = 2; // indicator + space
        let elapsed_len = if elapsed.is_empty() { 0 } else { elapsed.len() + 1 };
        let max_name = cols.saturating_sub(prefix_len + elapsed_len);
        let truncated_name = truncate(name, max_name);

        let bold_or_dim = if session.paused { "\x1b[2m" } else { "\x1b[1m" };
        let styled_line1 = if rename_state.is_some() {
            line1
        } else if elapsed.is_empty() {
            format!("{bg} \x1b[38;2;{r};{g};{b}m{indicator}{fg}{bold_or_dim} {truncated_name}{RESET}")
        } else {
            format!("{bg} \x1b[38;2;{r};{g};{b}m{indicator}{fg}{bold_or_dim} {truncated_name} \x1b[2m{elapsed}{RESET}")
        };
        print!("\x1b[{};1H{}", start_row + 1, pad_with_bg_color(&styled_line1, cols, bg));
    } else {
        print!("\x1b[{};1H{}", start_row + 1, pad(&line1, cols));
    }

    // Line 2: branch or tool info
    let line2_content = if let Some(ref branch) = session.git_branch {
        format!("   \u{2387} {branch}")
    } else if let crate::session::Activity::ToolUse(ref tool) = session.activity {
        format!("   {tool}")
    } else {
        String::new()
    };

    if use_bg {
        let highlighted = format!("{bg}{fg}\x1b[2m{line2_content}{RESET}");
        print!("\x1b[{};1H{}", start_row + 2, pad_with_bg_color(&highlighted, cols, bg));
    } else {
        let dimmed = format!("\x1b[2m{line2_content}\x1b[0m");
        print!("\x1b[{};1H{}", start_row + 2, pad(&dimmed, cols));
    }

    // Line 3: blank separator
    print!("\x1b[{};1H{}", start_row + 3, " ".repeat(cols));

    // Click region covers lines 1-2
    session.tab_index.map(|tab_idx| (start_row, session.pane_id, tab_idx))
}

/// Render the empty state (no Claude sessions).
/// Returns click regions for the [+] button.
fn render_empty_state(state: &PluginState, rows: usize, cols: usize) -> Vec<ClickRegion> {
    render_header(state, cols);
    let mut click_regions = Vec::new();

    if rows > 2 {
        print_line(2, cols, "", Style::Normal);
    }
    if rows > 3 {
        print_line(3, cols, "  No Claude sessions", Style::Dim);
    }
    if rows > 4 {
        print_line(4, cols, "", Style::Normal);
    }
    if rows > 5 {
        let btn = "  [+] New tab";
        print_line(5, cols, btn, Style::Dim);
        click_regions.push((5, u32::MAX, usize::MAX));
    }

    for row in 6..rows {
        print_line(row, cols, "", Style::Normal);
    }

    click_regions
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

/// Render a help overlay listing all keyboard shortcuts.
fn render_help_overlay(rows: usize, cols: usize) -> Vec<ClickRegion> {
    let help_lines = [
        "\x1b[1m Keyboard Shortcuts\x1b[0m",
        "\x1b[2m \u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\x1b[0m",
        " \x1b[1mAlt+s\x1b[0m  Session list",
        " \x1b[1mAlt+a\x1b[0m  Next session",
        "",
        " \x1b[2mNavigation:\x1b[0m",
        " \x1b[1mj/\u{2193}\x1b[0m    Move down",
        " \x1b[1mk/\u{2191}\x1b[0m    Move up",
        " \x1b[1mEnter\x1b[0m  Go to session",
        " \x1b[1mEsc\x1b[0m    Cancel",
        "",
        " \x1b[2mActions:\x1b[0m",
        " \x1b[1mr\x1b[0m      Rename",
        " \x1b[1md\x1b[0m      Delete",
        " \x1b[1mp\x1b[0m      Pause/unpause",
        " \x1b[1mn\x1b[0m      New tab",
        " \x1b[1m/\x1b[0m      Search",
        " \x1b[1m?\x1b[0m      This help",
    ];

    for (i, line) in help_lines.iter().enumerate() {
        if i >= rows {
            break;
        }
        print!("\x1b[{};1H{}", i + 1, pad(line, cols));
    }
    // Fill remaining rows
    for i in help_lines.len()..rows {
        print!("\x1b[{};1H{}", i + 1, " ".repeat(cols));
    }

    Vec::new() // No click regions in help mode
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

/// Pad a string, continuing the given background color for padding spaces.
fn pad_with_bg_color(s: &str, width: usize, bg: &str) -> String {
    let display_len = display_width(s);
    if display_len >= width {
        format!("{s}{RESET}")
    } else {
        let padding = width - display_len;
        if !bg.is_empty() {
            format!("{s}{bg}{}{RESET}", " ".repeat(padding))
        } else {
            format!("{s}{}{RESET}", " ".repeat(padding))
        }
    }
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
