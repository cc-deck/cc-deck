// T004: Plugin configuration parsing from KDL

use std::collections::BTreeMap;

/// How new sessions are created.
#[derive(Debug, Clone, PartialEq, Default)]
pub enum NewSessionMode {
    /// Create in a new tab (default).
    #[default]
    Tab,
    /// Create as a tiled pane in the current tab.
    Pane,
}

/// Plugin configuration parsed from KDL layout parameters.
pub struct PluginConfig {
    /// Sidebar width in characters (default 22).
    pub sidebar_width: usize,
    /// Done-to-Idle timeout in seconds (default 120).
    pub done_timeout: u64,
    /// Duration over which idle session indicators fade to dark grey (default 3600).
    pub idle_fade_secs: u64,
    /// Auto-pause idle sessions after this many seconds (default 3600 = 1h).
    pub auto_pause_secs: u64,
    /// Rapid-cycle window for attend in milliseconds (default 2000).
    /// Within this window, repeated Alt+a presses skip already-visited sessions.
    /// Set to 0 to disable rapid-cycle (always jump to most recent).
    pub attend_cycle_ms: u64,
    /// Timer interval in seconds for stale session cleanup (default 10.0).
    pub timer_interval: f64,
    /// How new sessions are created (default: Tab).
    pub new_session_mode: NewSessionMode,
    /// Global shortcut to toggle sidebar navigation mode (default: "Alt s").
    pub navigate_key: String,
    /// Global shortcut for smart attend action (default: "Alt a").
    pub attend_key: String,
    /// Global shortcut to cycle through working sessions (default: "Alt w").
    pub working_key: String,
    /// Enable performance instrumentation (default: false).
    pub perf_enabled: bool,
    /// Perf stats dump interval in seconds (default: 30).
    pub perf_interval: u64,
}

impl Default for PluginConfig {
    fn default() -> Self {
        Self {
            sidebar_width: 22,
            done_timeout: 300,
            idle_fade_secs: 3600,
            auto_pause_secs: 3600,
            attend_cycle_ms: 2000,
            timer_interval: 1.0,
            new_session_mode: NewSessionMode::Tab,
            navigate_key: "Alt s".to_string(),
            attend_key: "Alt a".to_string(),
            working_key: "Alt w".to_string(),
            perf_enabled: false,
            perf_interval: 30,
        }
    }
}

impl PluginConfig {
    /// Parse configuration from KDL key-value pairs provided by Zellij.
    pub fn from_configuration(config: &BTreeMap<String, String>) -> Self {
        let mut result = Self::default();

        if let Some(v) = config.get("sidebar_width") {
            if let Ok(w) = v.parse::<usize>() {
                if (10..=60).contains(&w) {
                    result.sidebar_width = w;
                }
            }
        }

        if let Some(v) = config.get("done_timeout") {
            if let Ok(t) = v.parse::<u64>() {
                result.done_timeout = t;
            }
        }

        if let Some(v) = config.get("idle_fade_secs") {
            if let Ok(t) = v.parse::<u64>() {
                if t >= 60 {
                    result.idle_fade_secs = t;
                }
            }
        }

        if let Some(v) = config.get("auto_pause_secs") {
            if let Ok(t) = v.parse::<u64>() {
                if t >= 60 {
                    result.auto_pause_secs = t;
                }
            }
        }

        if let Some(v) = config.get("attend_cycle_ms") {
            if let Ok(t) = v.parse::<u64>() {
                result.attend_cycle_ms = t;
            }
        }

        if let Some(v) = config.get("new_session") {
            match v.as_str() {
                "pane" => result.new_session_mode = NewSessionMode::Pane,
                _ => result.new_session_mode = NewSessionMode::Tab,
            }
        }

        if let Some(v) = config.get("navigate_key") {
            if !v.is_empty() {
                result.navigate_key = v.clone();
            }
        }

        if let Some(v) = config.get("attend_key") {
            if !v.is_empty() {
                result.attend_key = v.clone();
            }
        }

        if let Some(v) = config.get("working_key") {
            if !v.is_empty() {
                result.working_key = v.clone();
            }
        }


        if let Some(v) = config.get("perf") {
            result.perf_enabled = v == "true" || v == "1";
        }

        if let Some(v) = config.get("perf_interval") {
            if let Ok(i) = v.parse::<u64>() {
                if i >= 5 {
                    result.perf_interval = i;
                }
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
        assert_eq!(config.sidebar_width, 22);
        assert_eq!(config.done_timeout, 300);
        assert_eq!(config.idle_fade_secs, 3600);
        assert!((config.timer_interval - 1.0).abs() < f64::EPSILON);
    }

    #[test]
    fn test_parse_config() {
        let mut map = BTreeMap::new();
        map.insert("sidebar_width".into(), "30".into());
        map.insert("done_timeout".into(), "60".into());
        let config = PluginConfig::from_configuration(&map);
        assert_eq!(config.sidebar_width, 30);
        assert_eq!(config.done_timeout, 60);
    }

    #[test]
    fn test_sidebar_width_bounds() {
        let mut map = BTreeMap::new();
        map.insert("sidebar_width".into(), "5".into()); // too small
        let config = PluginConfig::from_configuration(&map);
        assert_eq!(config.sidebar_width, 22); // stays default

        map.insert("sidebar_width".into(), "100".into()); // too large
        let config = PluginConfig::from_configuration(&map);
        assert_eq!(config.sidebar_width, 22); // stays default
    }
}
