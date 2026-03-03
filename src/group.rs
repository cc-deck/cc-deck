/// Color palette for project groups.
pub const GROUP_COLORS: &[&str] = &[
    "blue", "green", "yellow", "magenta", "cyan", "red", "white",
];

/// A logical grouping of sessions sharing the same project.
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct ProjectGroup {
    /// Normalized project name (lowercase)
    pub id: String,
    /// Original-case project name
    pub display_name: String,
    /// Assigned color from palette
    pub color: String,
    /// Number of active sessions in this group
    pub session_count: usize,
}

/// ANSI foreground color code for a group color name.
pub fn ansi_fg(color: &str) -> &str {
    match color {
        "blue" => "\x1b[34m",
        "green" => "\x1b[32m",
        "yellow" => "\x1b[33m",
        "magenta" => "\x1b[35m",
        "cyan" => "\x1b[36m",
        "red" => "\x1b[31m",
        "white" => "\x1b[37m",
        _ => "\x1b[37m",
    }
}

/// ANSI background color code for a group color name.
#[allow(dead_code)]
pub fn ansi_bg(color: &str) -> &str {
    match color {
        "blue" => "\x1b[44m",
        "green" => "\x1b[42m",
        "yellow" => "\x1b[43m",
        "magenta" => "\x1b[45m",
        "cyan" => "\x1b[46m",
        "red" => "\x1b[41m",
        "white" => "\x1b[47m",
        _ => "\x1b[47m",
    }
}

/// ANSI reset code.
#[allow(dead_code)]
pub const RESET: &str = "\x1b[0m";

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_group_colors_palette_length() {
        assert_eq!(GROUP_COLORS.len(), 7);
    }

    #[test]
    fn test_ansi_fg_known_colors() {
        assert_eq!(ansi_fg("blue"), "\x1b[34m");
        assert_eq!(ansi_fg("green"), "\x1b[32m");
        assert_eq!(ansi_fg("yellow"), "\x1b[33m");
        assert_eq!(ansi_fg("magenta"), "\x1b[35m");
        assert_eq!(ansi_fg("cyan"), "\x1b[36m");
        assert_eq!(ansi_fg("red"), "\x1b[31m");
        assert_eq!(ansi_fg("white"), "\x1b[37m");
    }

    #[test]
    fn test_ansi_fg_unknown_defaults_to_white() {
        assert_eq!(ansi_fg("orange"), "\x1b[37m");
        assert_eq!(ansi_fg(""), "\x1b[37m");
    }

    #[test]
    fn test_ansi_bg_known_colors() {
        assert_eq!(ansi_bg("blue"), "\x1b[44m");
        assert_eq!(ansi_bg("green"), "\x1b[42m");
        assert_eq!(ansi_bg("red"), "\x1b[41m");
    }

    #[test]
    fn test_ansi_bg_unknown_defaults_to_white() {
        assert_eq!(ansi_bg("orange"), "\x1b[47m");
    }

    #[test]
    fn test_project_group_creation() {
        let group = ProjectGroup {
            id: "my-project".to_string(),
            display_name: "My-Project".to_string(),
            color: "blue".to_string(),
            session_count: 1,
        };
        assert_eq!(group.id, "my-project");
        assert_eq!(group.display_name, "My-Project");
        assert_eq!(group.color, "blue");
        assert_eq!(group.session_count, 1);
    }

    #[test]
    fn test_color_cycling() {
        for (i, expected) in GROUP_COLORS.iter().enumerate() {
            let color_idx = i % GROUP_COLORS.len();
            assert_eq!(GROUP_COLORS[color_idx], *expected);
        }
        // After wrapping around
        assert_eq!(GROUP_COLORS[7 % GROUP_COLORS.len()], "blue");
        assert_eq!(GROUP_COLORS[8 % GROUP_COLORS.len()], "green");
    }
}
