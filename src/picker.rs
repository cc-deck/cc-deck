use zellij_tile::prelude::*;

use crate::group::ansi_fg;
use crate::session::Session;
use crate::state::PluginState;

/// A picker entry representing a session in the fuzzy picker list.
#[derive(Debug, Clone)]
pub struct PickerEntry {
    pub session_id: u32,
    pub pane_id: u32,
    pub display_name: String,
    pub status_indicator: String,
    pub group_color: String,
    pub last_activity_secs: u64,
    /// Fuzzy match score (higher is better match)
    pub score: i32,
}

impl PickerEntry {
    pub fn from_session(session: &Session, group_color: &str) -> Self {
        Self {
            session_id: session.id,
            pane_id: session.pane_id,
            display_name: session.display_name.clone(),
            status_indicator: session.status.indicator().to_string(),
            group_color: group_color.to_string(),
            last_activity_secs: session.last_activity_secs,
            score: 0,
        }
    }
}

/// Perform fuzzy matching of a query against a target string.
///
/// Returns Some(score) if the query matches, None if it doesn't.
/// Higher scores indicate better matches. The algorithm:
/// - Matches characters in order (not necessarily contiguous)
/// - Consecutive matches score higher
/// - Matches at word boundaries score higher
/// - Case-insensitive matching
pub fn fuzzy_match(query: &str, target: &str) -> Option<i32> {
    if query.is_empty() {
        return Some(0);
    }

    let query_lower: Vec<char> = query.to_lowercase().chars().collect();
    let target_lower: Vec<char> = target.to_lowercase().chars().collect();
    let target_chars: Vec<char> = target.chars().collect();

    if query_lower.len() > target_lower.len() {
        return None;
    }

    let mut score: i32 = 0;
    let mut query_idx = 0;
    let mut prev_match_idx: Option<usize> = None;

    for (target_idx, tc) in target_lower.iter().enumerate() {
        if query_idx >= query_lower.len() {
            break;
        }

        if *tc == query_lower[query_idx] {
            // Base score for a match
            score += 1;

            // Bonus for consecutive matches
            if let Some(prev) = prev_match_idx {
                if target_idx == prev + 1 {
                    score += 5;
                }
            }

            // Bonus for matching at word boundary (start, after -, _, /)
            if target_idx == 0 {
                score += 10;
            } else {
                let prev_char = target_chars[target_idx - 1];
                if prev_char == '-' || prev_char == '_' || prev_char == '/' || prev_char == '.' {
                    score += 8;
                }
            }

            // Bonus for exact case match
            if target_chars[target_idx] == query.chars().nth(query_idx).unwrap_or(' ') {
                score += 1;
            }

            prev_match_idx = Some(target_idx);
            query_idx += 1;
        }
    }

    if query_idx == query_lower.len() {
        Some(score)
    } else {
        None
    }
}

/// Get filtered and sorted picker entries from the current state.
pub fn get_filtered_entries(state: &PluginState) -> Vec<PickerEntry> {
    let mut entries: Vec<PickerEntry> = state
        .sessions
        .values()
        .filter_map(|session| {
            let group_color = state
                .groups
                .get(&session.group_id)
                .map(|g| g.color.as_str())
                .unwrap_or("white");

            let mut entry = PickerEntry::from_session(session, group_color);

            if state.picker_query.is_empty() {
                Some(entry)
            } else if let Some(score) = fuzzy_match(&state.picker_query, &session.display_name) {
                entry.score = score;
                Some(entry)
            } else {
                None
            }
        })
        .collect();

    if state.picker_query.is_empty() {
        // MRU ordering when no query: most recently used first
        entries.sort_by(|a, b| a.last_activity_secs.cmp(&b.last_activity_secs));
    } else {
        // Score ordering when filtering: best match first
        entries.sort_by(|a, b| b.score.cmp(&a.score));
    }

    entries
}

const RESET: &str = "\u{1b}[0m";
const BOLD: &str = "\u{1b}[1m";
const REVERSE: &str = "\u{1b}[7m";
const DIM: &str = "\u{1b}[2m";

impl PluginState {
    /// Render the fuzzy picker overlay.
    ///
    /// The picker shows a search input at the top and a filtered list of sessions.
    /// The selected entry is highlighted. It renders within the plugin's own
    /// render area (not a floating pane).
    pub fn render_picker(&self, rows: usize, cols: usize) {
        let entries = get_filtered_entries(self);
        let max_list_rows = rows.saturating_sub(3); // header + search + footer

        // Top border
        let title = " cc-deck: session picker ";
        let border_width = cols.saturating_sub(title.len());
        let left_pad = border_width / 2;
        let right_pad = border_width - left_pad;
        println!(
            "{}{}{}{}{}",
            DIM,
            "─".repeat(left_pad),
            title,
            "─".repeat(right_pad),
            RESET
        );

        // Search input
        let query_display = if self.picker_query.is_empty() {
            format!("{}type to filter...{}", DIM, RESET)
        } else {
            self.picker_query.clone()
        };
        let search_line = format!(" > {}", query_display);
        println!(
            "{}{}",
            search_line,
            " ".repeat(cols.saturating_sub(visible_len(&search_line)))
        );

        // Session list
        if entries.is_empty() {
            println!(
                "{}  no matching sessions{}",
                DIM, RESET
            );
        } else {
            for (idx, entry) in entries.iter().take(max_list_rows).enumerate() {
                let is_selected = idx == self.picker_selected;
                let color = ansi_fg(&entry.group_color);
                let num = idx + 1;

                if is_selected {
                    print!(
                        "{}{}{} {}:{} {} {}",
                        color, BOLD, REVERSE, num, entry.status_indicator, entry.display_name, RESET
                    );
                } else {
                    print!(
                        "  {}:{}{}  {} {}",
                        num, color, entry.status_indicator, entry.display_name, RESET
                    );
                }

                // Pad remaining width
                println!();
            }
        }

        // Footer
        let footer = format!(
            " {}[Enter]{} select  {}[Esc]{} close  {}[↑↓]{} navigate",
            DIM, RESET, DIM, RESET, DIM, RESET
        );
        // Fill remaining rows
        let used_rows = 2 + entries.len().min(max_list_rows).max(1);
        for _ in used_rows..rows.saturating_sub(1) {
            println!();
        }
        print!("{}", footer);
    }

    /// Handle a key event while the picker is active.
    ///
    /// Returns Some(pane_id) if a session was selected, None otherwise.
    /// Returns Err(()) if the picker should be dismissed without selection.
    pub fn handle_picker_key(&mut self, key: KeyWithModifier) -> Result<Option<u32>, ()> {
        let entries = get_filtered_entries(self);

        match key.bare_key {
            BareKey::Esc => {
                self.close_picker();
                return Err(());
            }
            BareKey::Enter => {
                if let Some(entry) = entries.get(self.picker_selected) {
                    let pane_id = entry.pane_id;
                    self.close_picker();
                    return Ok(Some(pane_id));
                }
                return Ok(None);
            }
            BareKey::Up => {
                if self.picker_selected > 0 {
                    self.picker_selected -= 1;
                }
            }
            BareKey::Down => {
                if !entries.is_empty() && self.picker_selected < entries.len() - 1 {
                    self.picker_selected += 1;
                }
            }
            BareKey::Backspace => {
                self.picker_query.pop();
                self.picker_selected = 0;
            }
            BareKey::Char(c) => {
                self.picker_query.push(c);
                self.picker_selected = 0;
            }
            _ => {}
        }

        Ok(None)
    }

    /// Close the picker and reset its state.
    fn close_picker(&mut self) {
        self.picker_active = false;
        self.picker_query.clear();
        self.picker_selected = 0;
    }
}

/// Calculate visible string length ignoring ANSI escapes.
fn visible_len(s: &str) -> usize {
    let mut len = 0;
    let mut in_escape = false;
    for c in s.chars() {
        if c == '\u{1b}' {
            in_escape = true;
        } else if in_escape {
            if c == 'm' {
                in_escape = false;
            }
        } else {
            len += 1;
        }
    }
    len
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_fuzzy_match_empty_query() {
        assert_eq!(fuzzy_match("", "anything"), Some(0));
    }

    #[test]
    fn test_fuzzy_match_exact() {
        let score = fuzzy_match("api", "api").unwrap();
        assert!(score > 0);
    }

    #[test]
    fn test_fuzzy_match_substring() {
        let score = fuzzy_match("api", "api-server").unwrap();
        assert!(score > 0);
    }

    #[test]
    fn test_fuzzy_match_scattered() {
        let score = fuzzy_match("as", "api-server").unwrap();
        assert!(score > 0);
    }

    #[test]
    fn test_fuzzy_match_case_insensitive() {
        let score = fuzzy_match("API", "api-server").unwrap();
        assert!(score > 0);
    }

    #[test]
    fn test_fuzzy_match_no_match() {
        assert_eq!(fuzzy_match("xyz", "api-server"), None);
    }

    #[test]
    fn test_fuzzy_match_longer_query() {
        assert_eq!(fuzzy_match("api-server-long", "api"), None);
    }

    #[test]
    fn test_fuzzy_match_word_boundary_bonus() {
        // "as" matching "api-server" should score higher than "as" in "abcdefgs"
        // because 's' is at a word boundary in "api-server"
        let boundary_score = fuzzy_match("as", "api-server").unwrap();
        let no_boundary_score = fuzzy_match("as", "abcdefgs").unwrap();
        assert!(boundary_score > no_boundary_score);
    }

    #[test]
    fn test_fuzzy_match_consecutive_bonus() {
        // "ap" in "api" (consecutive) should score higher than "ap" in "axyzp" (scattered, no boundary)
        let consecutive_score = fuzzy_match("ap", "api").unwrap();
        let scattered_score = fuzzy_match("ap", "axyzp").unwrap();
        assert!(consecutive_score > scattered_score);
    }

    #[test]
    fn test_visible_len() {
        assert_eq!(visible_len("hello"), 5);
        assert_eq!(visible_len("\u{1b}[1mhello\u{1b}[0m"), 5);
    }
}
