// Sidebar rendering from cached RenderPayload.
//
// Adapted from crate::sidebar but operates on RenderSession (pre-computed
// by the controller) instead of Session. The sidebar only iterates the
// session list and prints ANSI escape sequences; no heavy computation.

use super::rename;
use super::state::{ClickRegion, SidebarState};
use cc_deck::RenderSession;
use unicode_width::UnicodeWidthChar;

// ANSI color constants (matching the existing sidebar)
const ACTIVE_BG: &str = "\x1b[48;2;25;45;55m";
const ACTIVE_FG: &str = "\x1b[38;2;120;200;220m";
const CURSOR_BG: &str = "\x1b[48;2;50;40;20m";
const CURSOR_FG: &str = "\x1b[38;2;230;200;140m";
const RENAME_FG: &str = "\x1b[38;2;140;220;255m"; // light cyan for rename in passive
const RENAME_NAV_FG: &str = "\x1b[38;2;255;220;150m"; // warm amber for rename in navigate
const RESET: &str = "\x1b[0m";

/// Result from a render pass, including click regions and viewport metadata.
pub struct RenderResult {
    pub click_regions: Vec<ClickRegion>,
    pub viewport_start: usize,
    pub max_visible: usize,
}

/// Render the sidebar into the plugin's stdout.
/// Returns click regions and viewport metadata for mouse/scroll handling.
pub fn render_sidebar(state: &SidebarState, rows: usize, cols: usize) -> RenderResult {
    if state.mode.is_help() {
        let legend = state
            .cached_payload
            .as_ref()
            .map(|p| p.profile_legend.as_slice())
            .unwrap_or(&[]);
        return RenderResult {
            click_regions: render_help_overlay(rows, cols, legend),
            viewport_start: 0,
            max_visible: 0,
        };
    }

    if !state.initialized {
        return RenderResult {
            click_regions: render_loading(rows, cols),
            viewport_start: 0,
            max_visible: 0,
        };
    }

    let payload = match &state.cached_payload {
        Some(p) => p,
        None => {
            return RenderResult {
                click_regions: render_loading(rows, cols),
                viewport_start: 0,
                max_visible: 0,
            }
        }
    };

    let sessions = state.filtered_sessions();
    let separator_after_index = sessions
        .iter()
        .rposition(|s| !s.paused)
        .filter(|&idx| idx < sessions.len().saturating_sub(1));
    let mut click_regions = Vec::new();

    if sessions.is_empty() && state.mode.filter_state().is_none() {
        return RenderResult {
            click_regions: render_empty_state(state, payload, rows, cols),
            viewport_start: 0,
            max_visible: 0,
        };
    }

    // Header with status counts (clickable to enter navigate mode)
    render_header(state, payload, cols);

    // Voice indicator click region MUST come before header regions so
    // handle_left_click's .find() matches the voice sentinel first on row 0.
    if payload.voice_connected {
        click_regions.push((0, VOICE_CLICK_SENTINEL, 0, 1));
    }

    // Use pane_id u32::MAX - 1 as sentinel for "header clicked"
    click_regions.push((0, u32::MAX - 1, 0, 1));
    // Row 1 (separator): use u32::MAX - 3 sentinel to activate first session
    click_regions.push((1, SEPARATOR_CLICK_SENTINEL, 0, 1));

    // Available rows for sessions (header + separator at top)
    let content_start = 2;
    let content_end = rows;
    let available = content_end.saturating_sub(content_start);

    // Per-entry heights (two rows each, no blank separator row between
    // entries).
    let heights: Vec<usize> = sessions.iter().map(|s| entry_height(s)).collect();
    // Reserve one row for the active/paused group divider when present, matching
    // where it is drawn in the render loop below.
    let effective_available = if separator_after_index.is_some() {
        available.saturating_sub(1)
    } else {
        available
    };

    let total = sessions.len();
    // Use the same pane-id source for scrolling as for highlighting:
    // in navigate mode, focused_pane_id points to the sidebar plugin,
    // so use restore_pane_id (the terminal pane that was active before).
    let scroll_anchor_pane_id = if state.mode.is_navigating() {
        state.mode.nav_ctx().and_then(|ctx| ctx.restore_pane_id)
    } else {
        state.effective_focused_pane_id()
    };
    let active_tab_index = payload
        .client_views
        .get(&state.my_client_id)
        .and_then(|view| view.active_tab_index)
        .or(state.my_tab_index)
        .unwrap_or(0);
    // Resolve which session the viewport should keep in view (the scroll
    // anchor), then pack entries around it.
    let active_pos = resolve_active_pos(
        &sessions,
        active_tab_index,
        scroll_anchor_pane_id,
        payload.sort_active,
    );
    let (start_idx, end_idx, above_count, below_count) =
        visible_range(&heights, effective_available, active_pos, state.scroll_offset);

    // Defensive bounds clamping
    let start_idx = start_idx.min(total);
    let end_idx = end_idx.min(total).max(start_idx);

    let mut row = content_start;

    // Overflow indicator (above) - clickable to scroll up
    if above_count > 0 {
        let msg = format!("  \u{25b2} +{above_count}");
        print_line(row, cols, &msg, Style::Dim);
        click_regions.push((row, OVERFLOW_UP_CLICK_SENTINEL, 0, 1));
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

        let has_cursor = state.mode.is_navigating() && abs_idx == state.cursor_index();
        // When local_focus_override targets this session, force cyan highlight
        // immediately (even during the same frame as a mode transition).
        let force_active = state.local_focus_override == Some(session.pane_id);
        // Debug: detect when no row gets any highlight (potential flash)
        if !is_active
            && !has_cursor
            && !force_active
            && !state.mode.is_selectable()
            && active_pane_id.is_none()
        {
            crate::debug_log(&format!(
                "SIDEBAR RENDER: no highlight for pane={} override={:?} active_pid={:?} mode=Passive",
                session.pane_id, state.local_focus_override, active_pane_id
            ));
        }
        let is_delete_confirm = state.mode.delete_confirm_pane() == Some(session.pane_id);
        // Delete confirmation uses the entry's two rows (prompt + hint).
        let entry_h = if is_delete_confirm {
            2
        } else {
            entry_height(session)
        };

        if is_delete_confirm {
            let prompt = format!(
                " Delete \"{}\"?",
                truncate(&session.display_name, cols.saturating_sub(14))
            );
            let confirm_hint = " [y/N]";
            print!(
                "\x1b[{};1H{}",
                row + 1,
                pad(&format!("\x1b[38;2;255;109;109m{prompt}\x1b[0m"), cols)
            );
            print!(
                "\x1b[{};1H{}",
                row + 2,
                pad(&format!("\x1b[2m{confirm_hint}\x1b[0m"), cols)
            );
            click_regions.push((row, session.pane_id, session.tab_index, entry_h));
        } else {
            let rename_for_session = state
                .mode
                .rename_state()
                .filter(|rs| rs.pane_id == session.pane_id);

            let presence = build_local_presence(state);
            let presence_ref = presence.as_slice();
            if let Some(region) = render_session_entry(
                session,
                is_active,
                has_cursor,
                force_active,
                row,
                cols,
                rename_for_session,
                presence_ref,
            ) {
                click_regions.push(region);
            }
        }
        row += entry_h;

        // Draw separator after the last active session (before paused zone)
        if let Some(sep_idx) = separator_after_index {
            if abs_idx == sep_idx && row < content_end {
                let sep_width = cols.min(40);
                let line = "\u{2500}".repeat(sep_width);
                print!(
                    "\x1b[{};1H\x1b[2m{line}\x1b[0m{}",
                    row + 1,
                    " ".repeat(cols.saturating_sub(sep_width))
                );
                row += 1;
            }
        }
    }

    // Overflow indicator (below) - clickable to scroll down
    if below_count > 0 && row < content_end {
        let msg = format!("  \u{25bc} +{below_count}");
        print_line(row, cols, &msg, Style::Dim);
        click_regions.push((row, OVERFLOW_DOWN_CLICK_SENTINEL, 0, 1));
        row += 1;
    }

    // Bottom row: search input (when filtering)
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
    }

    // Render notification (if active)
    if let Some(ref notif) = state.notification {
        if crate::session::unix_now() < notif.expires_at && row < rows {
            let truncated = truncate_chars(&notif.message, cols);
            let char_len = truncated.chars().count();
            let padding = if char_len < cols {
                " ".repeat(cols - char_len)
            } else {
                String::new()
            };
            print!("\x1b[{};1H\x1b[2m{truncated}{padding}\x1b[0m", row + 1);
            row += 1;
        }
    }

    // Also show controller notification from payload
    if let Some(notif_msg) = state
        .cached_payload
        .as_ref()
        .and_then(|p| p.notification.as_ref())
    {
        if row < rows {
            let truncated = truncate_chars(notif_msg, cols);
            let char_len = truncated.chars().count();
            let padding = if char_len < cols {
                " ".repeat(cols - char_len)
            } else {
                String::new()
            };
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
    click_regions.push((0, u32::MAX - 1, usize::MAX, 1));

    RenderResult {
        click_regions,
        viewport_start: start_idx,
        // Report the number of entries actually shown as the page size, so
        // keyboard paging and overflow scrolling have a sensible step.
        max_visible: end_idx.saturating_sub(start_idx),
    }
}

/// Render the "Connecting..." loading state.
pub fn render_loading(rows: usize, cols: usize) -> Vec<ClickRegion> {
    // Orange star header
    let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mcc-deck\x1b[0m".to_string();
    print!("\x1b[1;1H{}", pad(&header, cols));

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!(
        "\x1b[2;1H\x1b[2m{}\x1b[0m{}",
        sep,
        " ".repeat(cols.saturating_sub(sep.len()))
    );

    if rows > 3 {
        print_line(3, cols, "  Connecting...", Style::Dim);
    }

    for row in 4..rows {
        print_line(row, cols, "", Style::Normal);
    }

    Vec::new()
}

/// Render the permission prompt (shown before permissions are granted).
pub fn render_permission_prompt(rows: usize, cols: usize) {
    let header = " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mcc-deck\x1b[0m".to_string();
    print!("\x1b[1;1H{}", pad(&header, cols));

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!(
        "\x1b[2;1H\x1b[2m{}\x1b[0m{}",
        sep,
        " ".repeat(cols.saturating_sub(sep.len()))
    );

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

/// Click sentinel for the voice indicator.
pub const VOICE_CLICK_SENTINEL: u32 = u32::MAX - 2;
/// Click sentinel for the separator row (activates first session).
pub const SEPARATOR_CLICK_SENTINEL: u32 = u32::MAX - 3;
/// Click sentinel for the "above" overflow indicator (scroll up).
pub const OVERFLOW_UP_CLICK_SENTINEL: u32 = u32::MAX - 4;
/// Click sentinel for the "below" overflow indicator (scroll down).
pub const OVERFLOW_DOWN_CLICK_SENTINEL: u32 = u32::MAX - 5;

/// Render the status header with orange star and session counts.
fn render_header(
    state: &super::state::SidebarState,
    payload: &cc_deck::RenderPayload,
    cols: usize,
) {
    let effective_muted = state.local_mute_override.unwrap_or(payload.voice_muted);
    let voice_indicator = if payload.voice_connected {
        if effective_muted {
            "\x1b[2m\u{266b}\x1b[0m" // dim ♫
        } else {
            "\x1b[38;2;80;220;120m\u{266b}\x1b[0m" // bright green ♫
        }
    } else {
        ""
    };

    let header = if payload.total == 0 {
        " \x1b[38;2;255;170;50m\u{2731}\x1b[0m \x1b[1mcc-deck\x1b[0m".to_string()
    } else {
        let mut status_parts = Vec::new();
        if payload.waiting > 0 {
            status_parts.push(format!(
                "\x1b[38;2;255;109;109m\u{26a0} {}\x1b[0m",
                payload.waiting
            ));
        }
        if payload.working > 0 {
            status_parts.push(format!(
                "\x1b[38;2;180;140;255m\u{25cf} {}\x1b[0m",
                payload.working
            ));
        }
        if payload.idle > 0 {
            status_parts.push(format!("\x1b[2m\u{25cb} {}\x1b[0m", payload.idle));
        }

        let status = if status_parts.is_empty() {
            format!("{}", payload.total)
        } else {
            format!(
                "{} \x1b[2m\u{2502}\x1b[0m {}",
                payload.total,
                status_parts.join(" ")
            )
        };
        format!(" \x1b[38;2;255;170;50m\u{2731}\x1b[0m {status}")
    };

    // Build right-side indicators: sort symbol (↕) and voice symbol (♫)
    if voice_indicator.is_empty() {
        print!("\x1b[1;1H{}", pad(&header, cols));
    } else {
        let right_start = cols;
        print!(
            "\x1b[1;1H{}\x1b[1;{}H{}",
            pad(&header, cols),
            right_start,
            voice_indicator
        );
    }

    let sep: String = "\u{2500}".repeat(cols.min(40));
    print!(
        "\x1b[2;1H\x1b[2m{}\x1b[0m{}",
        sep,
        " ".repeat(cols.saturating_sub(sep.len()))
    );
}

/// Map agent indicator icon to its brand color.
fn agent_indicator_color(indicator: &str) -> (u8, u8, u8) {
    match indicator {
        "\u{2733}" => (255, 170, 50), // ✳ Claude Code: orange
        "\u{276f}" => (60, 190, 190), // ❯ OpenCode: dark cyan
        "\u{25c6}" => (16, 163, 127), // ◆ Codex: OpenAI green
        _ => (180, 175, 195),         // fallback: light grey
    }
}

/// Pick the row highlight as (background, foreground, highlighted).
///
/// has_cursor (amber) wins over is_active (cyan) during navigation so the
/// cursor remains visible on the active row.  force_active (from
/// local_focus_override after Enter/click) still wins over everything so
/// the target row flips to cyan immediately.
fn row_highlight(
    is_active: bool,
    has_cursor: bool,
    force_active: bool,
    renaming: bool,
) -> (&'static str, &'static str, bool) {
    if force_active || renaming {
        (ACTIVE_BG, ACTIVE_FG, true)
    } else if has_cursor {
        (CURSOR_BG, CURSOR_FG, true)
    } else if is_active {
        (ACTIVE_BG, ACTIVE_FG, true)
    } else {
        ("", "", false)
    }
}

/// Edge bar glyph drawn in the left margin column of profiled entries.
const EDGE_BAR: &str = "\u{258c}"; // ▌

/// The edge bar in a profile color on the default background.
fn edge_bar((r, g, b): (u8, u8, u8)) -> String {
    format!("\x1b[38;2;{r};{g};{b}m{EDGE_BAR}{RESET}")
}

/// Height of a session entry in rows. Every entry has two rows, even when it
/// has no branch or badges, so entries stay uniform and a profiled entry's
/// edge bar always spans both lines.
fn entry_height(_session: &RenderSession) -> usize {
    2
}

/// Render a single session entry (name line plus branch/badge line, which is
/// blank when there is neither). Entries are drawn back to back with no blank
/// separator.
#[allow(clippy::too_many_arguments)]
fn render_session_entry(
    session: &RenderSession,
    is_active: bool,
    has_cursor: bool,
    force_active: bool,
    start_row: usize,
    cols: usize,
    rename_state: Option<&super::modes::RenameState>,
    presence: &[LocalPresence],
) -> Option<ClickRegion> {
    let (bg, fg, use_bg) =
        row_highlight(is_active, has_cursor, force_active, rename_state.is_some());
    let (line1, line2) =
        session_entry_lines(session, is_active, has_cursor, force_active, cols, rename_state);

    print!("\x1b[{};1H{}", start_row + 1, line1);

    // Presence indicators: overwrite right side of line 1
    if let Some((indicator_str, indicator_width)) =
        presence_indicator_string(session.pane_id, presence)
    {
        let col_pos = cols.saturating_sub(indicator_width);
        if col_pos > 3 {
            let reset = if use_bg {
                format!("{bg}{fg}")
            } else {
                RESET.to_string()
            };
            print!(
                "\x1b[{};{}H{}{reset}",
                start_row + 1,
                col_pos + 1,
                indicator_str
            );
        }
    }

    // Line 2 (branch/badges, or blank). For profiled entries it still carries
    // the edge bar.
    let height = entry_height(session);
    print!("\x1b[{};1H{}", start_row + 2, line2);

    Some((start_row, session.pane_id, session.tab_index, height))
}

/// Build the two text lines of a session entry, each padded to `cols`.
pub(super) fn session_entry_lines(
    session: &RenderSession,
    is_active: bool,
    has_cursor: bool,
    force_active: bool,
    cols: usize,
    rename_state: Option<&super::modes::RenameState>,
) -> (String, String) {
    // The activity indicator always carries the status color (working, idle,
    // fading).
    let (r, g, b) = session.color;
    let indicator = if session.paused {
        "\u{23f8}"
    } else {
        &session.indicator
    };

    let (bg, fg, use_bg) =
        row_highlight(is_active, has_cursor, force_active, rename_state.is_some());

    // Column 1 of both text lines is the edge bar for profiled sessions and
    // the plain margin space otherwise (R12). The bar stays on the default
    // background, so highlights start at column 2 on profiled rows (FR-016a).
    let bar = session.profile_color.map(edge_bar);
    let lead = |bg: &str| match bar {
        Some(ref bar) => format!("{bar}{bg}"),
        None => format!("{bg} "),
    };

    // Line 1: indicator + name (or rename input buffer)
    // When in a worktree, show only the project part (before @) on line 1.
    // The worktree branch is already visible on line 2 via git_branch.
    let render_name: &str = if session.in_worktree {
        session
            .display_name
            .split('@')
            .next()
            .unwrap_or(&session.display_name)
    } else {
        &session.display_name
    };

    let line1 = if let Some(rs) = rename_state {
        let max_input = cols.saturating_sub(3);
        let rename_fg = if has_cursor { RENAME_NAV_FG } else { RENAME_FG };
        let input_display = rename::render_input(rs, max_input, rename_fg, bg);
        let lead = lead(bg);
        let line1 =
            format!("{lead}\x1b[38;2;{r};{g};{b}m{indicator}{bg}{rename_fg} {input_display}{bg}");
        pad_with_bg_color(&line1, cols, bg)
    } else {
        let agent_prefix = session.agent_indicator.clone().unwrap_or_default();
        let agent_prefix_len = if agent_prefix.is_empty() {
            0
        } else {
            display_width(&agent_prefix) + 1
        };
        // The background marker is drawn flush-right (appended below), so its two
        // columns are reserved from the right of the name budget rather than the
        // left prefix. This keeps the name's start column identical on background
        // and non-background rows, so names stay aligned.
        let right_reserve = if session.background { 2 } else { 0 };
        let prefix_len = 1 + display_width(indicator) + 1 + agent_prefix_len;
        let max_name = cols.saturating_sub(prefix_len + right_reserve);
        let truncated_name = truncate(render_name, max_name);

        if use_bg {
            let agent_part = if agent_prefix.is_empty() {
                String::new()
            } else {
                let (ar, ag, ab) = agent_indicator_color(&agent_prefix);
                format!("\x1b[38;2;{ar};{ag};{ab}m{agent_prefix}{bg}{fg} ")
            };
            let bold_or_dim = if session.paused { "\x1b[2m" } else { "\x1b[1m" };
            let lead = lead(bg);
            let styled_line1 = format!("{lead}\x1b[38;2;{r};{g};{b}m{indicator}{fg} {agent_part}{bold_or_dim}{truncated_name}{RESET}");
            let padded = pad_with_bg_color(&styled_line1, cols.saturating_sub(right_reserve), bg);
            if session.background {
                format!("{padded}{bg}\x1b[2m ⇣{RESET}")
            } else {
                padded
            }
        } else {
            let agent_part = if agent_prefix.is_empty() {
                String::new()
            } else {
                let (ar, ag, ab) = agent_indicator_color(&agent_prefix);
                format!("\x1b[38;2;{ar};{ag};{ab}m{agent_prefix}\x1b[0m ")
            };

            let name_part = if session.paused {
                format!("\x1b[2m{truncated_name}\x1b[0m")
            } else if is_active {
                format!("\x1b[1m{truncated_name}\x1b[0m")
            } else {
                truncated_name.to_string()
            };

            let lead = lead("");
            let line1 =
                format!("{lead}\x1b[38;2;{r};{g};{b}m{indicator}\x1b[0m {agent_part}{name_part}");
            let padded = pad(&line1, cols.saturating_sub(right_reserve));
            if session.background {
                format!("{padded}\x1b[2m ⇣\x1b[0m")
            } else {
                padded
            }
        }
    };

    let line2_content = format_line2(
        &session.badges,
        session.git_branch.as_deref(),
        session.color,
        session.in_worktree,
    );

    // format_line2 starts every non-empty variant with the margin space,
    // which the bar replaces on profiled rows.
    let (bar2, line2_text) = match bar {
        Some(ref bar) => (
            bar.as_str(),
            line2_content.strip_prefix(' ').unwrap_or(&line2_content),
        ),
        None => ("", line2_content.as_str()),
    };
    let line2 = if use_bg {
        let highlighted = format!("{bar2}{bg}{fg}\x1b[2m{line2_text}{RESET}");
        pad_with_bg_color(&highlighted, cols, bg)
    } else {
        let dimmed = format!("{bar2}\x1b[2m{line2_text}\x1b[0m");
        pad(&dimmed, cols)
    };

    (line1, line2)
}

/// Build presence indicator data from the shared client-view projection.
/// This sidebar's own client is excluded.
#[derive(Debug)]
struct LocalPresence {
    pane_id: u32,
    clients: Vec<(u16, (u8, u8, u8))>,
}

type PresenceByPane = std::collections::HashMap<u32, Vec<(u16, (u8, u8, u8))>>;

fn build_local_presence(state: &super::state::SidebarState) -> Vec<LocalPresence> {
    let payload = match &state.cached_payload {
        Some(p) => p,
        None => return vec![],
    };
    if payload.client_views.len() <= 1 {
        return vec![];
    }
    let colors = payload
        .multiplayer_colors
        .as_deref()
        .unwrap_or(&cc_deck::FALLBACK_MULTIPLAYER_COLORS);
    let my_cid = state.my_client_id;

    let mut by_pane = PresenceByPane::new();
    for (&cid, view) in &payload.client_views {
        if cid == my_cid {
            continue;
        }
        let Some(pane_id) = view.focused_pane_id else {
            continue;
        };
        let color_idx = (cid.saturating_sub(1) as usize) % colors.len();
        let color = colors[color_idx];
        by_pane.entry(pane_id).or_default().push((cid, color));
    }

    by_pane
        .into_iter()
        .map(|(pane_id, mut clients)| {
            clients.sort_by_key(|c| c.0);
            clients.truncate(5);
            LocalPresence { pane_id, clients }
        })
        .collect()
}

/// Build a right-aligned presence indicator string for a session line.
/// Returns (ansi_string, display_width) or None if no indicators for this session.
fn presence_indicator_string(pane_id: u32, presence: &[LocalPresence]) -> Option<(String, usize)> {
    let entry = presence.iter().find(|p| p.pane_id == pane_id)?;
    if entry.clients.is_empty() {
        return None;
    }
    let mut parts = Vec::new();
    let mut width = 0;
    for (i, (_, color)) in entry.clients.iter().take(5).enumerate() {
        let (r, g, b) = *color;
        if i > 0 {
            parts.push(" ".to_string());
            width += 1;
        }
        parts.push(format!("\x1b[48;2;{r};{g};{b}m \x1b[0m"));
        width += 1;
    }
    Some((parts.join(""), width))
}

/// Render the empty state (no active sessions).
fn render_empty_state(
    state: &super::state::SidebarState,
    payload: &cc_deck::RenderPayload,
    rows: usize,
    cols: usize,
) -> Vec<ClickRegion> {
    render_header(state, payload, cols);
    let mut click_regions = Vec::new();

    if rows > 2 {
        print_line(2, cols, "", Style::Normal);
    }
    if rows > 3 {
        print_line(3, cols, "  No active sessions", Style::Dim);
    }
    if rows > 4 {
        print_line(4, cols, "", Style::Normal);
    }
    for row in 5..rows {
        print_line(row, cols, "", Style::Normal);
    }

    click_regions.push((0, u32::MAX - 1, usize::MAX, 1));
    click_regions
}

const HELP_LINES: &[&str] = &[
    "\x1b[1m Keyboard Shortcuts\x1b[0m",
    "\x1b[2m \u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\u{2500}\x1b[0m",
    " \x1b[2mLegend:\x1b[0m",
    " \x1b[2m⇣\x1b[0m background job",
    "",
    " \x1b[1mAlt+s\x1b[0m  Session list \x1b[2m/ next\x1b[0m",
    " \x1b[1mAlt+S\x1b[0m  Session list \x1b[2m/ prev\x1b[0m",
    " \x1b[1mAlt+a\x1b[0m  Next session",
    " \x1b[1mAlt+A\x1b[0m  Prev session",
    " \x1b[1mAlt+w\x1b[0m  Next working",
    " \x1b[1mAlt+W\x1b[0m  Prev working",
    " \x1b[1mAlt+m\x1b[0m  Voice mute toggle",
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
    " \x1b[1mm\x1b[0m      Voice mute",
    " \x1b[1mn\x1b[0m      New tab",
    " \x1b[1mR\x1b[0m      Refresh sidebar",
    " \x1b[1mS\x1b[0m      Sort by activity",
    " \x1b[1mJ/K\x1b[0m    Move session down/up",
    " \x1b[1m/\x1b[0m      Search",
    " \x1b[1m?\x1b[0m      This help",
];

/// Render a help overlay listing all keyboard shortcuts and an optional
/// profile legend section when profiles are active.
fn render_help_overlay(rows: usize, cols: usize, legend: &[cc_deck::LegendEntry]) -> Vec<ClickRegion> {
    let help_lines = HELP_LINES;
    let mut row = 0;
    for line in help_lines.iter() {
        if row >= rows {
            break;
        }
        print!("\x1b[{};1H{}", row + 1, pad(line, cols));
        row += 1;
    }

    // Append a dynamic Profiles section when legend entries exist.
    if !legend.is_empty() && row < rows {
        // Blank separator
        print!("\x1b[{};1H{}", row + 1, " ".repeat(cols));
        row += 1;
        if row < rows {
            print!(
                "\x1b[{};1H{}",
                row + 1,
                pad(" \x1b[2mProfiles:\x1b[0m", cols)
            );
            row += 1;
        }
        for entry in legend {
            if row >= rows {
                break;
            }
            print!("\x1b[{};1H{}", row + 1, pad(&legend_line(entry), cols));
            row += 1;
        }
    }

    for i in row..rows {
        print!("\x1b[{};1H{}", i + 1, " ".repeat(cols));
    }

    Vec::new()
}

/// Format one profile legend entry: " ▌ <name>", followed by the profile
/// icon when one is declared.
pub(super) fn legend_line(entry: &cc_deck::LegendEntry) -> String {
    let bar = edge_bar(entry.color);
    if entry.icon.is_empty() {
        format!(" {bar} {}", entry.name)
    } else {
        format!(" {bar} {} {}", entry.name, entry.icon)
    }
}

/// Resolve the session index the viewport should anchor on. When sorted,
/// prefer the focused pane so the viewport stays stable when tabs are
/// physically reordered via MoveTab (Alt-I/O), falling back to the active tab.
fn resolve_active_pos(
    sessions: &[&RenderSession],
    active_tab_index: usize,
    scroll_anchor_pane_id: Option<u32>,
    sort_active: bool,
) -> usize {
    if sort_active {
        scroll_anchor_pane_id
            .and_then(|pid| sessions.iter().position(|s| s.pane_id == pid))
            .or_else(|| sessions.iter().position(|s| s.tab_index == active_tab_index))
            .unwrap_or(0)
    } else {
        sessions
            .iter()
            .position(|s| s.tab_index == active_tab_index)
            .unwrap_or(0)
    }
}

/// Compute the visible range of sessions, packing variable-height entries into
/// `available` rows and keeping the entry at `active_pos` in view.
///
/// `heights[i]` is the row count of session `i`. Returns `(start, end, above,
/// below)`: the half-open session window `[start, end)` plus how many entries
/// fall above and below it.
fn visible_range(
    heights: &[usize],
    available: usize,
    active_pos: usize,
    scroll_offset: Option<usize>,
) -> (usize, usize, usize, usize) {
    let total = heights.len();
    if total == 0 || available == 0 {
        return (0, 0, 0, 0);
    }
    // Everything fits: no scrolling needed.
    if heights.iter().sum::<usize>() <= available {
        return (0, total, 0, 0);
    }

    // Greedily pack forward from `start`, returning the exclusive end index of
    // the window that fits in `available` rows. Always shows at least the
    // starting entry even if it alone overflows (the render loop truncates).
    let pack_from = |start: usize| -> usize {
        let mut used = 0;
        let mut end = start;
        while end < total && used + heights[end] <= available {
            used += heights[end];
            end += 1;
        }
        end.max((start + 1).min(total))
    };

    // Smallest start whose tail `[start..total]` still fits, so the last entry
    // stays reachable when scrolled to the bottom.
    let max_start = {
        let mut used = 0;
        let mut s = total;
        while s > 0 && used + heights[s - 1] <= available {
            used += heights[s - 1];
            s -= 1;
        }
        s
    };

    // Manual scroll offset overrides auto-centering.
    if let Some(offset) = scroll_offset {
        let start = offset.min(max_start);
        let end = pack_from(start);
        return (start, end, start, total.saturating_sub(end));
    }

    // Auto-center: give the rows above the active entry up to half the budget,
    // then pack forward. Clamp to `max_start` so the tail stays visible.
    let half = available / 2;
    let mut start = active_pos.min(total - 1);
    let mut used_above = 0;
    while start > 0 && used_above + heights[start - 1] <= half {
        used_above += heights[start - 1];
        start -= 1;
    }
    let start = start.min(max_start);
    let end = pack_from(start);
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

/// Parse a badge string that may contain a color prefix: `#RRGGBB:icon` or plain `icon`.
/// Returns (icon, optional RGB tuple).
fn parse_badge_color(badge: &str) -> (&str, Option<(u8, u8, u8)>) {
    if badge.len() >= 8 && badge.starts_with('#') {
        if let Some(colon) = badge.find(':') {
            let hex = &badge[1..colon];
            if hex.len() == 6 {
                if let (Ok(r), Ok(g), Ok(b)) = (
                    u8::from_str_radix(&hex[0..2], 16),
                    u8::from_str_radix(&hex[2..4], 16),
                    u8::from_str_radix(&hex[4..6], 16),
                ) {
                    return (&badge[colon + 1..], Some((r, g, b)));
                }
            }
        }
    }
    (badge, None)
}

/// Format line 2 content (badges + branch) with alignment.
/// When badges are narrow enough, pads so the branch icon aligns at column 4 (under the session name).
/// Badges with a `#RRGGBB:` prefix use that color; others use the session's faded color.
/// Uses \x1b[39m (reset foreground only) to preserve background and dim on highlighted rows.
/// When `in_worktree` is true, the branch icon changes from `⎇` (U+2387) to `⌥` (U+2325).
fn format_line2(
    badges: &[String],
    branch: Option<&str>,
    color: (u8, u8, u8),
    in_worktree: bool,
) -> String {
    const BRANCH_COL: usize = 4;
    let branch_icon = if in_worktree { "\u{2325}" } else { "\u{2387}" };
    match (branch, badges.is_empty()) {
        (Some(branch), true) => format!("   {branch_icon} {branch}"),
        (Some(branch), false) => {
            let mut colored_parts = Vec::new();
            let mut total_icon_width: usize = 0;
            for badge in badges {
                let (icon, custom_color) = parse_badge_color(badge);
                let (r, g, b) = custom_color.unwrap_or(color);
                colored_parts.push(format!("\x1b[38;2;{r};{g};{b}m{icon}\x1b[39m"));
                total_icon_width += display_width(icon);
            }
            let badges_str = colored_parts.join(" ");
            let total_width = 1 + total_icon_width + badges.len().saturating_sub(1);
            if total_width < BRANCH_COL {
                let padding = BRANCH_COL - total_width;
                format!(" {badges_str}{}{branch_icon} {branch}", " ".repeat(padding))
            } else {
                format!(" {badges_str} {branch_icon} {branch}")
            }
        }
        (None, false) => {
            let mut colored_parts = Vec::new();
            for badge in badges {
                let (icon, custom_color) = parse_badge_color(badge);
                let (r, g, b) = custom_color.unwrap_or(color);
                colored_parts.push(format!("\x1b[38;2;{r};{g};{b}m{icon}\x1b[39m"));
            }
            format!(" {}", colored_parts.join(" "))
        }
        (None, true) => String::new(),
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
            width += ch.width().unwrap_or(0);
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
    if display_width(s) <= max {
        return s.to_string();
    }
    let mut result = String::new();
    let mut width = 0;
    for ch in s.chars() {
        let cw = ch.width().unwrap_or(0);
        if width + cw > max {
            break;
        }
        result.push(ch);
        width += cw;
    }
    result
}

/// Truncate a string to at most `max` characters (UTF-8 safe, no ellipsis).
fn truncate_chars(s: &str, max: usize) -> String {
    s.chars().take(max).collect()
}

#[cfg(test)]
fn handle_click(click_row: usize, click_regions: &[ClickRegion]) -> Option<(usize, u32)> {
    for &(row, pane_id, tab_index, height) in click_regions {
        if click_row >= row && click_row < row + height {
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
        assert_eq!(display_width("🚢"), 2);
        assert_eq!(display_width("🚢 ✅"), 5);
        assert_eq!(display_width("\u{2387}"), 1);
        // East Asian Ambiguous characters (activity indicators) must be width 1
        assert_eq!(display_width("○"), 1); // U+25CB WHITE CIRCLE (Idle)
        assert_eq!(display_width("●"), 1); // U+25CF BLACK CIRCLE (Working)
    }

    const TEST_COLOR: (u8, u8, u8) = (180, 140, 255);

    #[test]
    fn test_format_line2_no_badges_with_branch() {
        let result = format_line2(&[], Some("main"), TEST_COLOR, false);
        assert_eq!(result, "   \u{2387} main");
    }

    #[test]
    fn test_format_line2_one_badge_aligns() {
        let badges = vec!["❯".to_string()];
        let result = format_line2(&badges, Some("main"), TEST_COLOR, false);
        assert!(result.starts_with(' '));
        assert!(result.contains("\x1b[38;2;180;140;255m❯\x1b[39m"));
        assert!(result.contains("\u{2387} main"));
    }

    #[test]
    fn test_format_line2_two_badges() {
        let badges = vec!["❯".to_string(), "✎".to_string()];
        let result = format_line2(&badges, Some("main"), TEST_COLOR, false);
        assert!(result.starts_with(' '));
        assert!(result.contains("❯"));
        assert!(result.contains("✎"));
        assert!(result.contains("\u{2387}"));
    }

    #[test]
    fn test_format_line2_badges_only() {
        let badges = vec!["❯".to_string()];
        let result = format_line2(&badges, None, TEST_COLOR, false);
        assert!(result.starts_with(' '));
        assert!(result.contains("\x1b[38;2;180;140;255m❯\x1b[39m"));
    }

    #[test]
    fn test_format_line2_empty() {
        let result = format_line2(&[], None, TEST_COLOR, false);
        assert!(result.is_empty());
    }

    #[test]
    fn test_format_line2_worktree_icon() {
        let result = format_line2(&[], Some("076-fix"), TEST_COLOR, true);
        assert_eq!(result, "   \u{2325} 076-fix");
        // Verify the worktree icon is used instead of branch icon
        assert!(!result.contains("\u{2387}"));
    }

    #[test]
    fn test_format_line2_worktree_icon_with_badges() {
        let badges = vec!["❯".to_string()];
        let result = format_line2(&badges, Some("feature"), TEST_COLOR, true);
        assert!(result.contains("\u{2325} feature"));
        assert!(!result.contains("\u{2387}"));
    }

    #[test]
    fn test_display_width_wide_chars() {
        // U+2630 TRIGRAM FOR HEAVEN: unicode_width 0.1.x returns 1 but many
        // terminals render it as 2 columns. Avoid using it in badge icons.
        // Badge icons with known safe widths (Narrow, always 1 column)
        assert_eq!(display_width("⋮"), 1); // U+22EE, Narrow
        assert_eq!(display_width("✎"), 1); // U+270E, Narrow
        assert_eq!(display_width("⚙"), 1); // U+2699, Narrow
        assert_eq!(display_width("❯"), 1); // U+276F, Narrow
        assert_eq!(display_width("◆"), 1); // U+25C6, Ambiguous (width 1 in unicode_width)
    }

    #[test]
    fn test_truncate() {
        assert_eq!(truncate("short", 10), "short");
        assert_eq!(truncate("a-very-long-name", 10), "a-very-lon");
        assert_eq!(truncate("ab", 2), "ab");
        assert_eq!(truncate("abcde", 3), "abc");
    }

    #[test]
    fn test_truncate_wide_chars() {
        // Wide char (width 2) should be counted as 2 columns
        let s = "a☰bcdefgh"; // 'a'=1, '☰'=2, rest=1 each -> total width 10
        let result = truncate(s, 8);
        assert_eq!(display_width(&result), 8); // "a☰bc..."
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
            badges: vec![],
            agent_indicator: None,
            in_worktree: false,
            profile_color: None,
            background: false,
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
            badges: vec![],
            agent_indicator: None,
            in_worktree: false,
            profile_color: None,
            background: false,
        };
        let sessions: Vec<&RenderSession> = vec![&s1, &s2];
        let heights: Vec<usize> = sessions.iter().map(|s| entry_height(s)).collect();
        // Both are branchless but still 2 rows each, so 4 rows fit in 5.
        let (start, end, above, below) = visible_range(&heights, 5, 0, None);
        assert_eq!((start, end, above, below), (0, 2, 0, 0));
    }

    #[test]
    fn test_visible_range_overflow() {
        let sessions_owned: Vec<cc_deck::RenderSession> = (0..10)
            .map(|i| cc_deck::RenderSession {
                pane_id: i,
                display_name: format!("s{i}"),
                activity_label: "Idle".into(),
                indicator: "\u{25cb}".into(),
                color: (180, 175, 195),
                git_branch: None,
                tab_index: i as usize,
                paused: false,
                done_attended: false,
                badges: vec![],
                agent_indicator: None,
                in_worktree: false,
                profile_color: None,
                background: false,
            })
            .collect();
        let refs: Vec<&RenderSession> = sessions_owned.iter().collect();
        let heights: Vec<usize> = refs.iter().map(|s| entry_height(s)).collect();
        // 10 branchless entries (2 rows each), budget 6 rows, anchored on index 5.
        let (start, end, above, below) = visible_range(&heights, 6, 5, None);
        assert_eq!(end - start, 3);
        assert!(start <= 5 && end > 5);
        assert!(above > 0 || below > 0);
    }

    #[test]
    fn test_resolve_active_pos_sorted_uses_pane_id_anchor() {
        let sessions_owned: Vec<cc_deck::RenderSession> = (0..10)
            .map(|i| cc_deck::RenderSession {
                pane_id: i + 100,
                display_name: format!("s{i}"),
                activity_label: "Idle".into(),
                indicator: "\u{25cb}".into(),
                color: (180, 175, 195),
                git_branch: None,
                tab_index: i as usize,
                paused: false,
                done_attended: false,
                badges: vec![],
                agent_indicator: None,
                in_worktree: false,
                profile_color: None,
                background: false,
            })
            .collect();
        let refs: Vec<&RenderSession> = sessions_owned.iter().collect();
        // Sorted mode with focused_pane_id=107 resolves to display position 7.
        let pos = resolve_active_pos(&refs, 0, Some(107), true);
        assert_eq!(pos, 7, "anchor should resolve to pane_id 107");
    }

    #[test]
    fn test_resolve_active_pos_sorted_fallback_to_tab_index() {
        let sessions_owned: Vec<cc_deck::RenderSession> = (0..10)
            .map(|i| cc_deck::RenderSession {
                pane_id: i + 100,
                display_name: format!("s{i}"),
                activity_label: "Idle".into(),
                indicator: "\u{25cb}".into(),
                color: (180, 175, 195),
                git_branch: None,
                tab_index: i as usize,
                paused: false,
                done_attended: false,
                badges: vec![],
                agent_indicator: None,
                in_worktree: false,
                profile_color: None,
                background: false,
            })
            .collect();
        let refs: Vec<&RenderSession> = sessions_owned.iter().collect();
        // Sorted mode but no focused_pane_id falls back to active_tab_index 5.
        let pos = resolve_active_pos(&refs, 5, None, true);
        assert_eq!(pos, 5, "anchor should fall back to tab_index 5");
    }

    #[test]
    fn test_entry_height() {
        let mut s = cc_deck::RenderSession {
            pane_id: 1,
            display_name: "a".into(),
            activity_label: "Idle".into(),
            indicator: "\u{25cb}".into(),
            color: (180, 175, 195),
            git_branch: None,
            tab_index: 0,
            paused: false,
            done_attended: false,
            badges: vec![],
            agent_indicator: None,
            in_worktree: false,
            profile_color: None,
            background: false,
        };
        assert_eq!(entry_height(&s), 2, "branchless, no badges => still 2 rows");
        s.git_branch = Some("main".into());
        assert_eq!(entry_height(&s), 2, "with branch => 2 rows");
        s.git_branch = None;
        s.badges = vec!["x".into()];
        assert_eq!(entry_height(&s), 2, "with badge => 2 rows");
    }

    #[test]
    fn test_visible_range_mixed_heights_pack() {
        // Five 2-row entries, 5-row budget: exactly two entries fit.
        let heights = vec![2, 2, 2, 2, 2];
        let (start, end, above, below) = visible_range(&heights, 5, 0, None);
        assert_eq!((start, end), (0, 2));
        assert_eq!((above, below), (0, 3));
    }

    #[test]
    fn test_visible_range_scroll_offset_clamps_to_tail() {
        // Six 2-row entries, 5-row budget: the tail window starts at index 4
        // no matter how large the requested offset.
        let heights = vec![2; 6];
        let (start, end, _, below) = visible_range(&heights, 5, 0, Some(100));
        assert_eq!((start, end), (4, 6));
        assert_eq!(below, 0);
    }

    #[test]
    fn test_visible_range_center_keeps_active_visible() {
        // Mixed heights; the active entry must always land inside [start, end).
        let heights = vec![2, 1, 2, 1, 2, 1, 2, 1];
        for active in 0..heights.len() {
            let (start, end, _, _) = visible_range(&heights, 4, active, None);
            assert!(
                start <= active && active < end,
                "active {active} not in window [{start}, {end})"
            );
        }
    }

    #[test]
    fn test_handle_click_hit() {
        let regions: Vec<ClickRegion> = vec![(2, 1, 0, 2), (5, 2, 1, 2)];
        assert_eq!(handle_click(2, &regions), Some((0, 1)));
        assert_eq!(handle_click(3, &regions), Some((0, 1)));
        assert_eq!(handle_click(5, &regions), Some((1, 2)));
        assert_eq!(handle_click(10, &regions), None);
    }

    #[test]
    fn test_handle_click_mixed_heights() {
        // Row 2: 1-row entry (pane 1). Rows 3-4: 2-row entry (pane 2).
        let regions: Vec<ClickRegion> = vec![(2, 1, 0, 1), (3, 2, 1, 2)];
        assert_eq!(handle_click(2, &regions), Some((0, 1)));
        // The row below the 1-row entry belongs to the next entry, not it.
        assert_eq!(handle_click(3, &regions), Some((1, 2)));
        assert_eq!(handle_click(4, &regions), Some((1, 2)));
        assert_eq!(handle_click(5, &regions), None);
    }

    #[test]
    fn test_presence_indicator_string_no_match() {
        let presence = vec![LocalPresence {
            pane_id: 7,
            clients: vec![(2, (255, 0, 0))],
        }];
        assert!(presence_indicator_string(8, &presence).is_none());
    }

    #[test]
    fn test_presence_indicator_string_single() {
        let presence = vec![LocalPresence {
            pane_id: 7,
            clients: vec![(2, (255, 0, 0))],
        }];
        let (s, w) = presence_indicator_string(7, &presence).unwrap();
        assert_eq!(w, 1);
        assert!(s.contains("\x1b[48;2;255;0;0m"));
    }

    #[test]
    fn test_presence_indicator_string_multiple() {
        let presence = vec![LocalPresence {
            pane_id: 7,
            clients: vec![(2, (255, 0, 0)), (3, (0, 255, 0))],
        }];
        let (s, w) = presence_indicator_string(7, &presence).unwrap();
        assert_eq!(w, 3); // 1 + space + 1
        assert!(s.contains("\x1b[48;2;255;0;0m"));
        assert!(s.contains("\x1b[48;2;0;255;0m"));
    }

    #[test]
    fn test_presence_indicator_string_empty_clients() {
        let presence = vec![LocalPresence {
            pane_id: 7,
            clients: vec![],
        }];
        assert!(presence_indicator_string(7, &presence).is_none());
    }

    // --- Profile edge bar (R12, FR-016, FR-016a, SC-008) ---

    use crate::sidebar_plugin::modes::RenameState;
    use crate::sidebar_plugin::test_helpers::make_session;

    const PROFILE_COLORS: [(u8, u8, u8); 3] = [(226, 124, 90), (90, 192, 226), (140, 226, 90)];

    /// Visible text of an ANSI string.
    fn visible(s: &str) -> String {
        let mut out = String::new();
        let mut in_escape = false;
        for ch in s.chars() {
            if ch == '\x1b' {
                in_escape = true;
            } else if in_escape {
                if ch == 'm' {
                    in_escape = false;
                }
            } else {
                out.push(ch);
            }
        }
        out
    }

    /// Three profiled sessions of different profiles plus one unprofiled.
    fn bar_sessions() -> Vec<RenderSession> {
        let mut sessions: Vec<RenderSession> = PROFILE_COLORS
            .iter()
            .enumerate()
            .map(|(i, &color)| {
                let mut s = make_session(i as u32 + 1, "a-rather-long-session-name", i);
                s.git_branch = Some("feature-branch".into());
                s.agent_indicator = Some("\u{2733}".into());
                s.profile_color = Some(color);
                s
            })
            .collect();
        let mut plain = make_session(4, "a-rather-long-session-name", 3);
        plain.git_branch = Some("feature-branch".into());
        plain.agent_indicator = Some("\u{2733}".into());
        sessions.push(plain);
        sessions
    }

    /// Entry states from SC-008 as (name, session tweak, is_active, has_cursor, renaming).
    #[allow(clippy::type_complexity)]
    fn bar_states() -> Vec<(&'static str, fn(&mut RenderSession), bool, bool, bool)> {
        vec![
            ("plain", |_| {}, false, false, false),
            ("active", |_| {}, true, false, false),
            ("cursor", |_| {}, false, true, false),
            ("rename", |_| {}, false, false, true),
            ("faded", |s| s.color = (70, 65, 80), false, false, false),
            ("paused", |s| s.paused = true, false, false, false),
            ("paused-active", |s| s.paused = true, true, false, false),
        ]
    }

    #[test]
    fn test_edge_bar_on_both_lines_in_every_state() {
        let cols = 24;
        let rename = RenameState {
            pane_id: 0,
            input_buffer: "renamed".into(),
            cursor_pos: 7,
        };
        for (state, tweak, is_active, has_cursor, renaming) in bar_states() {
            for base in bar_sessions() {
                let mut session = base.clone();
                tweak(&mut session);
                let rs = renaming.then_some(&rename);
                let (line1, line2) =
                    session_entry_lines(&session, is_active, has_cursor, false, cols, rs);

                // Pre-revision rendering: the same entry without a profile.
                let mut unprofiled = session.clone();
                unprofiled.profile_color = None;
                let (old1, old2) =
                    session_entry_lines(&unprofiled, is_active, has_cursor, false, cols, rs);

                for (line, old) in [(&line1, &old1), (&line2, &old2)] {
                    assert_eq!(display_width(line), cols, "{state}: width of {line:?}");
                    assert_eq!(display_width(line), display_width(old), "{state}");
                    match session.profile_color {
                        Some(color) => {
                            // The bar leads the line on the default background.
                            assert!(
                                line.starts_with(&edge_bar(color)),
                                "{state}: profiled line must start with the bar: {line:?}"
                            );
                            // Everything after column 1 is unchanged.
                            let (new_vis, old_vis) = (visible(line), visible(old));
                            assert!(new_vis.starts_with(EDGE_BAR));
                            assert!(old_vis.starts_with(' '));
                            assert_eq!(
                                new_vis.chars().skip(1).collect::<String>(),
                                old_vis.chars().skip(1).collect::<String>(),
                                "{state}"
                            );
                        }
                        None => {
                            assert_eq!(line, old);
                            assert!(visible(line).starts_with(' '), "{state}: {line:?}");
                            assert!(!line.contains(EDGE_BAR));
                        }
                    }
                }
            }
        }
    }

    #[test]
    fn test_edge_bar_highlight_starts_at_column_two() {
        let session = bar_sessions().remove(0);
        let bar = edge_bar(PROFILE_COLORS[0]);
        let rename = RenameState {
            pane_id: 1,
            input_buffer: "x".into(),
            cursor_pos: 1,
        };
        for (is_active, has_cursor, rs, bg) in [
            (true, false, None, ACTIVE_BG),
            (false, true, None, CURSOR_BG),
            (false, false, Some(&rename), ACTIVE_BG),
        ] {
            let (line1, line2) = session_entry_lines(&session, is_active, has_cursor, false, 24, rs);
            assert!(line1.starts_with(&format!("{bar}{bg}")), "{line1:?}");
            assert!(line2.starts_with(&format!("{bar}{bg}")), "{line2:?}");
        }

        // Unprofiled rows keep the highlight on the margin column.
        let plain = bar_sessions().remove(3);
        let (line1, line2) = session_entry_lines(&plain, true, false, false, 24, None);
        assert!(line1.starts_with(&format!("{ACTIVE_BG} ")));
        assert!(line2.starts_with(&format!("{ACTIVE_BG}{ACTIVE_FG}\x1b[2m ")));
    }

    #[test]
    fn test_edge_bar_on_empty_second_line() {
        let mut session = make_session(1, "s", 0);
        session.profile_color = Some(PROFILE_COLORS[1]);
        let (_, line2) = session_entry_lines(&session, false, false, false, 10, None);
        assert!(line2.starts_with(&edge_bar(PROFILE_COLORS[1])));
        assert_eq!(display_width(&line2), 10);
    }

    #[test]
    fn test_glyph_uses_brand_color_on_profiled_rows() {
        let brand = "\x1b[38;2;255;170;50m\u{2733}";
        for (_, tweak, is_active, has_cursor, _) in bar_states() {
            for base in bar_sessions() {
                let mut session = base.clone();
                tweak(&mut session);
                let (line1, _) = session_entry_lines(&session, is_active, has_cursor, false, 40, None);
                assert!(line1.contains(brand), "{line1:?}");
                if let Some((r, g, b)) = session.profile_color {
                    assert!(!line1.contains(&format!("\x1b[38;2;{r};{g};{b}m\u{2733}")));
                }
            }
        }
    }

    #[test]
    fn test_unprofiled_entry_layout_unchanged() {
        let mut session = make_session(1, "api", 0);
        session.git_branch = Some("main".into());
        let (line1, line2) = session_entry_lines(&session, false, false, false, 12, None);
        assert_eq!(line1, " \x1b[38;2;180;175;195m\u{25cb}\x1b[0m api      ");
        assert_eq!(line2, "\x1b[2m   \u{2387} main\x1b[0m   ");
    }

    #[test]
    fn test_legend_line_shows_bar_and_name() {
        let entry = cc_deck::LegendEntry {
            name: "private".into(),
            icon: String::new(),
            color: (226, 124, 90),
        };
        let line = legend_line(&entry);
        assert_eq!(line, format!(" {} private", edge_bar((226, 124, 90))));
        assert_eq!(visible(&line), " \u{258c} private");
    }

    #[test]
    fn test_legend_line_appends_declared_icon() {
        let entry = cc_deck::LegendEntry {
            name: "work".into(),
            icon: "W".into(),
            color: (90, 192, 226),
        };
        let line = legend_line(&entry);
        assert!(line.starts_with(&format!(" {}", edge_bar((90, 192, 226)))));
        assert_eq!(visible(&line), " \u{258c} work W");
    }

    #[test]
    fn test_edge_bar_is_one_column() {
        assert_eq!(display_width(EDGE_BAR), 1);
        assert_eq!(display_width(&edge_bar((1, 2, 3))), 1);
    }

    #[test]
    fn test_background_marker_keeps_status_columns_and_second_line() {
        let mut s = make_session(7, "abcdefghijklmnopqrstuvwxyz", 0);
        s.agent_indicator = Some("✳".into());
        s.profile_color = Some((90, 192, 226));
        for (active, cursor) in [(false, false), (true, false), (false, true)] {
            s.background = false;
            let (fg1, fg2) = session_entry_lines(&s, active, cursor, false, 24, None);
            s.background = true;
            let (bg1, bg2) = session_entry_lines(&s, active, cursor, false, 24, None);
            let fg = visible(&fg1);
            let bg = visible(&bg1);
            // The marker is dim-only on the foreground line and only on the
            // background line, and it now sits at the right edge, not before name.
            assert!(!fg.contains('⇣'));
            assert!(bg.trim_end().ends_with('⇣'), "marker at right edge: {bg:?}");
            // Names stay aligned: the agent prefix is followed directly by the
            // name on both rows, so the first 22 columns (prefix + shared name
            // prefix) are identical; only the last two columns differ (name tail
            // vs. the reserved marker).
            assert_eq!(
                fg.chars().take(22).collect::<String>(),
                bg.chars().take(22).collect::<String>()
            );
            assert_eq!(bg2, fg2);
            assert_eq!(display_width(&bg1), 24);
            assert!(bg.contains(&truncate(&s.display_name, 17)));
            assert_ne!(bg, fg);
        }
    }

    #[test]
    fn test_harness_glyphs_have_brand_colors() {
        // FR-017a: every known harness glyph uses its brand color, not the
        // grey fallback.
        use crate::controller::render_broadcast::agent_name_to_indicator;
        let fallback = agent_indicator_color("?");
        for harness in ["claude", "codex", "opencode"] {
            let glyph = agent_name_to_indicator(Some(harness));
            assert_ne!(agent_indicator_color(&glyph), fallback, "{harness} glyph {glyph}");
        }
        assert_eq!(agent_indicator_color("\u{25c6}"), (16, 163, 127));
    }
}
