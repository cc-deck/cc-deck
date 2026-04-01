// Inline rename editing for the sidebar renderer plugin.
//
// Adapted from crate::rename but operates on the local RenameState
// defined in modes.rs. Returns actions that the caller translates
// into pipe messages to the controller.

use super::modes::RenameState;
use zellij_tile::prelude::*;

/// Action returned by handle_key to drive the rename flow.
#[derive(Debug, PartialEq)]
pub enum RenameAction {
    /// Keep editing (re-render).
    Continue,
    /// User confirmed the rename with the given name.
    Confirm(String),
    /// User cancelled the rename.
    Cancel,
}

/// Process a key event during an active rename operation.
pub fn handle_key(rename: &mut RenameState, key: KeyWithModifier) -> RenameAction {
    let has_ctrl = key.key_modifiers.contains(&KeyModifier::Ctrl);

    // Ctrl- shortcuts (emacs-style)
    if has_ctrl {
        return match key.bare_key {
            BareKey::Char('a') => {
                rename.cursor_pos = 0;
                RenameAction::Continue
            }
            BareKey::Char('e') => {
                rename.cursor_pos = rename.input_buffer.len();
                RenameAction::Continue
            }
            BareKey::Char('k') => {
                rename.input_buffer.truncate(rename.cursor_pos);
                RenameAction::Continue
            }
            _ => RenameAction::Continue,
        };
    }

    match key.bare_key {
        BareKey::Enter => RenameAction::Confirm(rename.input_buffer.clone()),
        BareKey::Esc => RenameAction::Cancel,
        BareKey::Char(c) => {
            rename.input_buffer.insert(rename.cursor_pos, c);
            rename.cursor_pos += c.len_utf8();
            RenameAction::Continue
        }
        BareKey::Backspace => {
            if rename.cursor_pos > 0 {
                let prev = rename.input_buffer[..rename.cursor_pos]
                    .char_indices()
                    .last()
                    .map(|(i, _)| i)
                    .unwrap_or(0);
                rename.input_buffer.remove(prev);
                rename.cursor_pos = prev;
            }
            RenameAction::Continue
        }
        BareKey::Delete => {
            if rename.cursor_pos < rename.input_buffer.len()
                && rename.input_buffer.is_char_boundary(rename.cursor_pos)
            {
                rename.input_buffer.remove(rename.cursor_pos);
            }
            RenameAction::Continue
        }
        BareKey::Left => {
            if rename.cursor_pos > 0 {
                let prev = rename.input_buffer[..rename.cursor_pos]
                    .char_indices()
                    .last()
                    .map(|(i, _)| i)
                    .unwrap_or(0);
                rename.cursor_pos = prev;
            }
            RenameAction::Continue
        }
        BareKey::Right => {
            if rename.cursor_pos < rename.input_buffer.len() {
                let next = rename.input_buffer[rename.cursor_pos..]
                    .char_indices()
                    .nth(1)
                    .map(|(i, _)| rename.cursor_pos + i)
                    .unwrap_or(rename.input_buffer.len());
                rename.cursor_pos = next;
            }
            RenameAction::Continue
        }
        BareKey::Home => {
            rename.cursor_pos = 0;
            RenameAction::Continue
        }
        BareKey::End => {
            rename.cursor_pos = rename.input_buffer.len();
            RenameAction::Continue
        }
        _ => RenameAction::Continue,
    }
}

/// Render the rename input buffer with cursor indicator.
/// `fg` and `bg` are ANSI escape sequences to restore after the cursor block.
pub fn render_input(rename: &RenameState, max_width: usize, fg: &str, bg: &str) -> String {
    let buf = &rename.input_buffer;
    let cursor_pos = rename.cursor_pos.min(buf.len());

    let (vis_start, vis_end) = if buf.len() <= max_width {
        (0, buf.len())
    } else if cursor_pos <= max_width {
        (0, max_width)
    } else {
        (cursor_pos - max_width + 1, cursor_pos + 1)
    };
    let vis_end = vis_end.min(buf.len());
    let vis_cursor = cursor_pos.saturating_sub(vis_start);
    let visible = buf.get(vis_start..vis_end).unwrap_or("");

    if cursor_pos == buf.len() {
        // Cursor at end of text: show a block cursor (reverse-video space) after the text
        format!("{visible}\x1b[7m \x1b[0m{bg}{fg}")
    } else if vis_cursor >= visible.len() {
        // Edge case: cursor beyond visible range
        format!("{visible}\x1b[7m \x1b[0m{bg}{fg}")
    } else {
        let vis_cursor = char_floor(visible, vis_cursor);
        let next = char_ceil(visible, vis_cursor);
        let before = visible.get(..vis_cursor).unwrap_or("");
        let cursor_char = visible.get(vis_cursor..next).unwrap_or(" ");
        let after = visible.get(next..).unwrap_or("");
        format!("{before}\x1b[7m{cursor_char}\x1b[0m{bg}{fg}{after}")
    }
}

/// Round a byte position down to the nearest char boundary.
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

/// Round a byte position up to the next char boundary.
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeSet;

    fn bare(key: BareKey) -> KeyWithModifier {
        KeyWithModifier {
            bare_key: key,
            key_modifiers: BTreeSet::new(),
        }
    }

    #[test]
    fn test_char_insert() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "ab".into(),
            cursor_pos: 1,
        };
        assert_eq!(handle_key(&mut rs, bare(BareKey::Char('X'))), RenameAction::Continue);
        assert_eq!(rs.input_buffer, "aXb");
        assert_eq!(rs.cursor_pos, 2);
    }

    #[test]
    fn test_backspace() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "abc".into(),
            cursor_pos: 2,
        };
        assert_eq!(handle_key(&mut rs, bare(BareKey::Backspace)), RenameAction::Continue);
        assert_eq!(rs.input_buffer, "ac");
        assert_eq!(rs.cursor_pos, 1);
    }

    #[test]
    fn test_backspace_at_start() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "abc".into(),
            cursor_pos: 0,
        };
        handle_key(&mut rs, bare(BareKey::Backspace));
        assert_eq!(rs.input_buffer, "abc");
        assert_eq!(rs.cursor_pos, 0);
    }

    #[test]
    fn test_enter_confirms() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "new-name".into(),
            cursor_pos: 8,
        };
        assert_eq!(handle_key(&mut rs, bare(BareKey::Enter)), RenameAction::Confirm("new-name".into()));
    }

    #[test]
    fn test_esc_cancels() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "test".into(),
            cursor_pos: 4,
        };
        assert_eq!(handle_key(&mut rs, bare(BareKey::Esc)), RenameAction::Cancel);
    }

    #[test]
    fn test_left_right() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "abc".into(),
            cursor_pos: 2,
        };
        handle_key(&mut rs, bare(BareKey::Left));
        assert_eq!(rs.cursor_pos, 1);
        handle_key(&mut rs, bare(BareKey::Right));
        assert_eq!(rs.cursor_pos, 2);
    }

    #[test]
    fn test_home_end() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "hello".into(),
            cursor_pos: 3,
        };
        handle_key(&mut rs, bare(BareKey::Home));
        assert_eq!(rs.cursor_pos, 0);
        handle_key(&mut rs, bare(BareKey::End));
        assert_eq!(rs.cursor_pos, 5);
    }

    #[test]
    fn test_delete() {
        let mut rs = RenameState {
            pane_id: 1,
            input_buffer: "abc".into(),
            cursor_pos: 1,
        };
        handle_key(&mut rs, bare(BareKey::Delete));
        assert_eq!(rs.input_buffer, "ac");
        assert_eq!(rs.cursor_pos, 1);
    }

    #[test]
    fn test_render_input_basic() {
        let rs = RenameState {
            pane_id: 1,
            input_buffer: "test".into(),
            cursor_pos: 2,
        };
        let output = render_input(&rs, 20, "", "");
        // Should contain reverse-video cursor marker
        assert!(output.contains("\x1b[7m"));
    }

    #[test]
    fn test_render_input_empty() {
        let rs = RenameState {
            pane_id: 1,
            input_buffer: String::new(),
            cursor_pos: 0,
        };
        let output = render_input(&rs, 20, "", "");
        assert!(output.contains("\x1b[7m"));
    }
}
