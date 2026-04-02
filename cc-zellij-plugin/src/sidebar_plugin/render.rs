// Sidebar rendering from cached RenderPayload.
//
// Adapted from crate::sidebar but operates on RenderSession (pre-computed
// by the controller) instead of Session. The sidebar only iterates the
// session list and prints ANSI escape sequences; no heavy computation.

use cc_deck::RenderSession;
use super::modes::SidebarMode;
use super::rename;
use super::state::{ClickRegion, SidebarState};

// ANSI color constants (matching the existing sidebar)
const ACTIVE_BG: &str = "\x1b[48;2;25;45;55m";
const ACTIVE_FG: &str = "\x1b[38;2;120;200;220m";
const CURSOR_BG: &str = "\x1b[48;2;50;40;20m";
const CURSOR_FG: &str = "\x1b[38;2;230;200;140m";
const RENAME_FG: &str = "\x1b[38;2;140;220;255m";        // light cyan for rename in passive
const RENAME_NAV_FG: &str = "\x1b[38;2;255;220;150m";   // warm amber for rename in navigate
const RESET: &str = "\x1b[0m";

/// Render the sidebar into the plugin's stdout.
/// Returns click regions for mouse handling.
pub fn render_sidebar(state: &SidebarState, rows: usize, cols: usize) -> Vec<ClickRegion> {
    if state.mode.is_help() {
        return render_help_overlay(rows, cols);
    }

    if !state.initialized {
        return render_loading(rows, cols);
    }

    let payload = match &state.cached_payload {
        Some(p) => p,
        None => return render_loading(rows, cols),
    };

    let sessions = state.filtered_sessions();
    let mut click_regions = Vec::new();

    if sessions.is_empty() && state.mode.filter_state().is_none() {
        return render_empty_state(payload, rows, cols);
    }

    // Header with status counts (clickable to enter navigate mode)
    render_header(payload, cols);
    // Use pane_id u32::MAX - 1 as sentinel for "header clicked"
    click_regions.push((0, u32::MAX - 1, 0));
    click_regions.push((1, u32::MAX - 1, 0));

    // Available rows for sessions (header + separator at top)
    let content_start = 2;
    let content_end = rows;
    let available = content_end.saturating_sub(content_start);

    let lines_per_session = 3;
    let max_visible = if lines_per_session > 0 {
        available / lines_per_session
    } else {
        0
    };

    let total = sessions.len();
    let active_tab = Some(payload.active_tab_index);
    let (start_idx, end_idx, above_count, below_count) =
        visible_range(total, max_visible, active_tab, &sessions);

    // Defensive bounds clamping
    let start_idx = start_idx.min(total);
    let end_idx = end_idx.min(total).max(start_idx);

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

        // When navigating, the sidebar has focus so focused_pane_id points
        // to the sidebar plugin, not a terminal pane. Use the restore_pane_id
        // (the pane that was active before entering navigate) for highlighting.
        // In Passive mode, use effective_focused_pane_id() which prefers the
        // local override (set on Switch actions) for immediate highlight.
        let active_pane_id = if state.mode.is_navigating() {
            state.mode.nav_ctx().and_then(|ctx| ctx.restore_pane_id)
        } else {
            state.effective_focused_pane_id()
        };
        let is_active = active_pane_id
            .map(|pid| session.pane_id == pid)
            .unwrap_or(false);

        let has_cursor = state.mode.is_navigating() && abs_idx == state.mode.cursor_index();
        // When local_focus_override targets this session, force cyan highlight
        // immediately (even during the same frame as a mode transition).
        let force_active = state.local_focus_override == Some(session.pane_id);
        let is_delete_confirm = state.mode.delete_confirm_pane() == Some(session.pane_id);

        if is_delete_confirm {
            let prompt = format!(" Delete \"{}\"?", truncate(&session.display_name, cols.saturating_sub(14)));
            let confirm_hint = " [y/N]";
            print!("\x1b[{};1H{}", row + 1, pad(&format!("\x1b[38;2;255;60;60m{prompt}\x1b[0m"), cols));
            print!("\x1b[{};1H{}", row + 2, pad(&format!("\x1b[2m{confirm_hint}\x1b[0m"), cols));
            print!("\x1b[{};1H{}", row + 3, " ".repeat(cols));
            click_regions.push((row, session.pane_id, session.tab_index));
        } else {
            let rename_for_session = state.mode.rename_state()
                .filter(|rs| rs.pane_id == session.pane_id);

            if let Some(region) = render_session_entry(
                session, is_active, has_cursor, force_active, row, cols, rename_for_session,
            ) {
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
    if let Some(fs) = state.mode.filter_state() {
        if row < rows.saturating_sub(1) {
            let prefix = " / ";
            let max_input = cols.saturating_sub(prefix.len());
            let buf = &fs.input_buffer;
            let cursor_pos = fs.cursor_pos.min(buf.len());
            let (vis_start, vis_end) = if buf.len() <= max_input {
                (0, buf.len())
            } else if cursor_pos <= max_input {
                (0, max_input)
            } else {
                (cursor_pos - max_input + 1, cursor_pos + 1)
            };
            let vis_end = vis_end.min(buf.len());
            let vis_cursor = cursor_pos.saturating_sub(vis_start);
            let visible = buf.get(vis_start..vis_end).unwrap_or("");
            let input_display = if cursor_pos == buf.len() && visible.len() >= max_input {
                let last_start = char_floor(visible, visible.len().saturating_sub(1));
                let before_last = visible.get(..last_start).unwrap_or("");
                let last_ch = visible.get(last_start..).unwrap_or("");
                format!("{before_last}\x1b[7m{last_ch}\x1b[0m")
            } else {
                let vis_cursor = char_floor(visible, vis_cursor);
                let next = char_ceil(visible, vis_cursor);
                let before = visible.get(..vis_cursor).unwrap_or("");
                let cursor_char = visible.get(vis_cursor..next).unwrap_or(" ");
                let after = visible.get(next..).unwrap_or("");
                format!("{before}\x1b[7m{cursor_char}\x1b[0m{after}")
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

    // Render notification (if active)
    if let Some(ref notif) = state.notification {
        if crate::session::unix_now() < notif.expires_at && row < rows {
            let truncated = truncate_chars(&notif.message, cols);
            let char_len = truncated.chars().count();
            let padding = if char_len < cols { " ".repeat(cols - char_len) } else { String::new() };
            print!("\x1b[{};1H\x1b[2m{truncated}{padding}\x1b[0m", row + 1);
            row += 1;
        }
    }

    // Also show controller notification from payload
    if let Some(notif_msg) = state.cached_payload.as_ref().and_then(|p| p.notification.as_ref()) {
        if row < rows {
            let truncated = truncate_chars(notif_msg, cols);
            let char_len = truncated.chars().count();
            let padding = if char_len < cols { " ".repeat(cols - char_len) } else { String::new() };
            print!("\x1b[{};1H\x1b[2m{truncated}{padding}\x1b[0m", row + 1);
            row += 1;
        }
    }

    // Fill remaining rows
    while row < rows {
        print_line(row, cols, "", Style::Normal);
        row += 1;
    }

    // Header click region last so session regions take priority
    click_regions.push((0, u32::MAX - 1, usize::MAX));

    click_regions
}

/// Render the "Waiting for controller..." loading state.
pub fn render_loading(rows: usize, cols: usize) -> Vec<ClickRegion> {
    // Orange star header
    let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mClaude Code\x1b[0m".to_string();
    print!("\x1b[1;1H{}", pad(&header, cols));

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!("\x1b[2;1H\x1b[2m{}\x1b[0m{}", sep, " ".repeat(cols.saturating_sub(sep.len())));

    if rows > 3 {
        print_line(3, cols, "  Waiting for controller...", Style::Dim);
    }

    for row in 4..rows {
        print_line(row, cols, "", Style::Normal);
    }

    Vec::new()
}

/// Render the permission prompt (shown before permissions are granted).
pub fn render_permission_prompt(rows: usize, cols: usize) {
    let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mClaude Code\x1b[0m".to_string();
    print!("\x1b[1;1H{}", pad(&header, cols));

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!("\x1b[2;1H\x1b[2m{}\x1b[0m{}", sep, " ".repeat(cols.saturating_sub(sep.len())));

    if rows > 3 {
        print_line(3, cols, "", Style::Normal);
    }
    if rows > 4 {
        print_line(4, cols, "  Grant permissions", Style::Normal);
    }
    if rows > 5 {
        print_line(5, cols, "  to enable cc-deck", Style::Normal);
    }
    if rows > 6 {
        print_line(6, cols, "", Style::Normal);
    }
    if rows > 7 {
        print_line(7, cols, "  Press  y  to allow", Style::Dim);
    }

    for row in 8..rows {
        print_line(row, cols, "", Style::Normal);
    }
}

/// Render the "Controller unavailable" state.
pub fn render_unavailable(rows: usize, cols: usize) -> Vec<ClickRegion> {
    let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mClaude Code\x1b[0m".to_string();
    print!("\x1b[1;1H{}", pad(&header, cols));

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!("\x1b[2;1H\x1b[2m{}\x1b[0m{}", sep, " ".repeat(cols.saturating_sub(sep.len())));

    if rows > 3 {
        print_line(3, cols, "  Controller unavailable", Style::Dim);
    }
    if rows > 4 {
        print_line(4, cols, "  Check plugin status", Style::Dim);
    }

    for row in 5..rows {
        print_line(row, cols, "", Style::Normal);
    }

    Vec::new()
}

/// Render the status header with orange star and session counts.
fn render_header(payload: &cc_deck::RenderPayload, cols: usize) {
    if payload.total == 0 {
        let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mClaude Code\x1b[0m".to_string();
        print!("\x1b[1;1H{}", pad(&header, cols));
    } else {
        let mut status_parts = Vec::new();
        if payload.waiting > 0 {
            status_parts.push(format!("\x1b[38;2;255;60;60m\u{26a0} {}\x1b[0m", payload.waiting));
        }
        if payload.working > 0 {
            status_parts.push(format!("\x1b[38;2;180;140;255m\u{25cf} {}\x1b[0m", payload.working));
        }
        if payload.idle > 0 {
            status_parts.push(format!("\x1b[2m\u{25cb} {}\x1b[0m", payload.idle));
        }

        let status = if status_parts.is_empty() {
            format!("{}", payload.total)
        } else {
            format!("{} \x1b[2m\u{2502}\x1b[0m {}", payload.total, status_parts.join(" "))
        };
        let header = format!(" \x1b[38;2;255;170;50m\u{2731}\x1b[0m {status}");
        print!("\x1b[1;1H{}", pad(&header, cols));
    }

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!("\x1b[2;1H\x1b[2m{}\x1b[0m{}", sep, " ".repeat(cols.saturating_sub(sep.len())));
}

/// Render a single session entry (3 lines: indicator+name, branch, blank).
fn render_session_entry(
    session: &RenderSession,
    is_active: bool,
    has_cursor: bool,
    force_active: bool,
    start_row: usize,
    cols: usize,
    rename_state: Option<&super::modes::RenameState>,
) -> Option<ClickRegion> {
    let (r, g, b) = session.color;
    let indicator = if session.paused { "\u{23f8}" } else { &session.indicator };

    // force_active (from local_focus_override) takes priority over has_cursor
    // so that Enter immediately shows cyan instead of briefly showing amber.
    let (bg, fg, use_bg) = if force_active || is_active || rename_state.is_some() {
        (ACTIVE_BG, ACTIVE_FG, true)
    } else if has_cursor {
        (CURSOR_BG, CURSOR_FG, true)
    } else {
        ("", "", false)
    };

    // Line 1: indicator + name (or rename input buffer)
    let line1 = if let Some(rs) = rename_state {
        let max_input = cols.saturating_sub(3);
        let rename_fg = if has_cursor { RENAME_NAV_FG } else { RENAME_FG };
        let input_display = rename::render_input(rs, max_input, rename_fg, bg);
        format!("{bg} \x1b[38;2;{r};{g};{b}m{indicator}{bg}{rename_fg} {input_display}{bg}")
    } else {
        let name = &session.display_name;
        let prefix_len = 3;
        let max_name = cols.saturating_sub(prefix_len);
        let truncated_name = truncate(name, max_name);

        let name_part = if session.paused {
            format!("\x1b[2m{truncated_name}\x1b[0m")
        } else if is_active {
            format!("\x1b[1m{truncated_name}\x1b[0m")
        } else {
            truncated_name.to_string()
        };

        format!(" \x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {name_part}")
    };

    if rename_state.is_some() {
        // Rename mode: keep bg color, cursor uses reverse video within it
        print!("\x1b[{};1H{}", start_row + 1, pad_with_bg_color(&line1, cols, bg));
    } else if use_bg {
        let name = &session.display_name;
        let prefix_len = 3;
        let max_name = cols.saturating_sub(prefix_len);
        let truncated_name = truncate(name, max_name);

        let bold_or_dim = if session.paused { "\x1b[2m" } else { "\x1b[1m" };
        let styled_line1 = format!("{bg} \x1b[38;2;{r};{g};{b}m{indicator}{fg}{bold_or_dim} {truncated_name}{RESET}");
        print!("\x1b[{};1H{}", start_row + 1, pad_with_bg_color(&styled_line1, cols, bg));
    } else {
        print!("\x1b[{};1H{}", start_row + 1, pad(&line1, cols));
    }

    // Line 2: branch info
    let line2_content = if let Some(ref branch) = session.git_branch {
        format!("   \u{2387} {branch}")
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

    Some((start_row, session.pane_id, session.tab_index))
}

/// Render the empty state (no Claude sessions).
fn render_empty_state(payload: &cc_deck::RenderPayload, rows: usize, cols: usize) -> Vec<ClickRegion> {
    render_header(payload, cols);
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

    click_regions.push((0, u32::MAX - 1, usize::MAX));
    click_regions
}

/// Render a help overlay listing all keyboard shortcuts.
fn render_help_overlay(rows: usize, cols: usize) -> Vec<ClickRegion> {
    let help_lines = [
        "\x1b[1m Keyboard Shortcuts\x1b[0m",
        "\x1b[2m \u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\x1b[0m",
        " \x1b[1mAlt+s\x1b[0m  Session list \x1b[2m/ next\x1b[0m",
        " \x1b[1mAlt+S\x1b[0m  Session list \x1b[2m/ prev\x1b[0m",
        " \x1b[1mAlt+a\x1b[0m  Next session",
        " \x1b[1mAlt+A\x1b[0m  Prev session",
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
    for i in help_lines.len()..rows {
        print!("\x1b[{};1H{}", i + 1, " ".repeat(cols));
    }

    Vec::new()
}

/// Compute the visible range of sessions, keeping the active one visible.
fn visible_range(
    total: usize,
    max_visible: usize,
    active_tab_index: Option<usize>,
    sessions: &[&RenderSession],
) -> (usize, usize, usize, usize) {
    if total == 0 || max_visible == 0 {
        return (0, 0, 0, 0);
    }
    if total <= max_visible {
        return (0, total, 0, 0);
    }

    let active_pos = active_tab_index
        .and_then(|idx| sessions.iter().position(|s| s.tab_index == idx))
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

// --- Rendering helpers ---

enum Style {
    #[allow(dead_code)]
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

fn char_floor(s: &str, pos: usize) -> usize {
    if pos >= s.len() {
        return s.len();
    }
    let mut p = pos;
    while p > 0 && !s.is_char_boundary(p) {
        p -= 1;
    }
    p
}

fn char_ceil(s: &str, pos: usize) -> usize {
    if pos >= s.len() {
        return s.len();
    }
    let mut p = pos + 1;
    while p < s.len() && !s.is_char_boundary(p) {
        p += 1;
    }
    p
}

fn truncate(s: &str, max: usize) -> String {
    if max <= 3 {
        return ".".repeat(max);
    }
    let char_count = s.chars().count();
    if char_count <= max {
        return s.to_string();
    }
    let mut result = String::new();
    for (i, ch) in s.chars().enumerate() {
        if i + 4 > max {
            result.push_str("...");
            break;
        }
        result.push(ch);
    }
    result
}

/// Truncate a string to at most `max` characters (UTF-8 safe, no ellipsis).
fn truncate_chars(s: &str, max: usize) -> String {
    s.chars().take(max).collect()
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
        let s1 = cc_deck::RenderSession {
            pane_id: 1,
            display_name: "a".into(),
            activity_label: "Idle".into(),
            indicator: "\u{25cb}".into(),
            color: (180, 175, 195),
            git_branch: None,
            tab_index: 0,
            paused: false,
            done_attended: false,
        };
        let s2 = cc_deck::RenderSession {
            pane_id: 2,
            display_name: "b".into(),
            activity_label: "Idle".into(),
            indicator: "\u{25cb}".into(),
            color: (180, 175, 195),
            git_branch: None,
            tab_index: 1,
            paused: false,
            done_attended: false,
        };
        let sessions: Vec<&RenderSession> = vec![&s1, &s2];
        let (start, end, above, below) = visible_range(2, 5, None, &sessions);
        assert_eq!((start, end, above, below), (0, 2, 0, 0));
    }

    #[test]
    fn test_visible_range_overflow() {
        let sessions_owned: Vec<cc_deck::RenderSession> = (0..10).map(|i| {
            cc_deck::RenderSession {
                pane_id: i,
                display_name: format!("s{i}"),
                activity_label: "Idle".into(),
                indicator: "\u{25cb}".into(),
                color: (180, 175, 195),
                git_branch: None,
                tab_index: i as usize,
                paused: false,
                done_attended: false,
            }
        }).collect();
        let refs: Vec<&RenderSession> = sessions_owned.iter().collect();
        let (start, end, above, below) = visible_range(10, 3, Some(5), &refs);
        assert_eq!(end - start, 3);
        assert!(start <= 5 && end > 5);
        assert!(above > 0 || below > 0);
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
