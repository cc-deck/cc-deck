use std::collections::BTreeMap;

/// Plugin configuration parsed from Zellij KDL config.
///
/// All values come from the KDL plugin block as `BTreeMap<String, String>`.
pub struct PluginConfig {
    /// Seconds before a session is considered idle (default: 300)
    pub idle_timeout: u64,
    /// Keybinding for fuzzy picker (default: "Ctrl Shift t")
    pub picker_key: String,
    /// Keybinding for new session (default: "Ctrl Shift n")
    pub new_session_key: String,
    /// Keybinding for rename session (default: "Ctrl Shift r")
    pub rename_key: String,
    /// Keybinding for close session (default: "Ctrl Shift x")
    pub close_key: String,
    /// Maximum number of recent entries (default: 20)
    pub max_recent: usize,
}

impl Default for PluginConfig {
    fn default() -> Self {
        Self {
            idle_timeout: 300,
            picker_key: "Ctrl Shift t".to_string(),
            new_session_key: "Ctrl Shift n".to_string(),
            rename_key: "Ctrl Shift r".to_string(),
            close_key: "Ctrl Shift x".to_string(),
            max_recent: 20,
        }
    }
}

impl PluginConfig {
    /// Parse configuration from Zellij's KDL plugin config map.
    pub fn from_kdl(config: &BTreeMap<String, String>) -> Self {
        let mut result = Self::default();

        if let Some(v) = config.get("idle_timeout") {
            if let Ok(timeout) = v.parse::<u64>() {
                result.idle_timeout = timeout;
            }
        }
        if let Some(v) = config.get("picker_key") {
            result.picker_key = v.clone();
        }
        if let Some(v) = config.get("new_session_key") {
            result.new_session_key = v.clone();
        }
        if let Some(v) = config.get("rename_key") {
            result.rename_key = v.clone();
        }
        if let Some(v) = config.get("close_key") {
            result.close_key = v.clone();
        }
        if let Some(v) = config.get("max_recent") {
            if let Ok(max) = v.parse::<usize>() {
                result.max_recent = max;
            }
        }

        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config() {
        let config = PluginConfig::default();
        assert_eq!(config.idle_timeout, 300);
        assert_eq!(config.picker_key, "Ctrl Shift t");
        assert_eq!(config.new_session_key, "Ctrl Shift n");
        assert_eq!(config.rename_key, "Ctrl Shift r");
        assert_eq!(config.close_key, "Ctrl Shift x");
        assert_eq!(config.max_recent, 20);
    }

    #[test]
    fn test_parse_from_kdl() {
        let mut map = BTreeMap::new();
        map.insert("idle_timeout".to_string(), "600".to_string());
        map.insert("picker_key".to_string(), "Ctrl Shift p".to_string());
        map.insert("max_recent".to_string(), "50".to_string());

        let config = PluginConfig::from_kdl(&map);
        assert_eq!(config.idle_timeout, 600);
        assert_eq!(config.picker_key, "Ctrl Shift p");
        assert_eq!(config.max_recent, 50);
        // Unset values keep defaults
        assert_eq!(config.new_session_key, "Ctrl Shift n");
    }

    #[test]
    fn test_parse_invalid_values_use_defaults() {
        let mut map = BTreeMap::new();
        map.insert("idle_timeout".to_string(), "not_a_number".to_string());
        map.insert("max_recent".to_string(), "".to_string());

        let config = PluginConfig::from_kdl(&map);
        assert_eq!(config.idle_timeout, 300);
        assert_eq!(config.max_recent, 20);
    }
}
