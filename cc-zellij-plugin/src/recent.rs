use std::path::PathBuf;

use serde::{Deserialize, Serialize};

/// Path to the recent entries JSON file (WASI virtual path).
const RECENT_FILE_PATH: &str = "/cache/recent.json";

/// Default maximum number of recent entries.
const DEFAULT_MAX_RECENT: usize = 20;

/// A single recent session entry.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RecentEntry {
    /// Working directory of the session
    pub directory: PathBuf,
    /// Display name of the session
    pub name: String,
    /// ISO 8601 timestamp string of last use
    pub last_used: String,
}

/// Collection of recent session entries with LRU eviction.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RecentEntries {
    /// Schema version for forward compatibility
    pub version: u32,
    /// Recent entries, ordered by most recent first
    pub entries: Vec<RecentEntry>,
}

impl Default for RecentEntries {
    fn default() -> Self {
        Self {
            version: 1,
            entries: Vec::new(),
        }
    }
}

#[allow(dead_code)]
impl RecentEntries {
    /// Create a new empty RecentEntries.
    pub fn new() -> Self {
        Self::default()
    }

    /// Add or update a recent entry.
    ///
    /// If a directory already exists, it is moved to the front (most recent)
    /// and its name/timestamp are updated. If over max_recent entries,
    /// the oldest entry is evicted.
    pub fn add(&mut self, directory: PathBuf, name: &str, timestamp: &str, max_recent: usize) {
        let max = if max_recent == 0 {
            DEFAULT_MAX_RECENT
        } else {
            max_recent
        };

        // Remove existing entry for this directory (if any)
        self.entries.retain(|e| e.directory != directory);

        // Insert at the front (most recent)
        self.entries.insert(
            0,
            RecentEntry {
                directory,
                name: name.to_string(),
                last_used: timestamp.to_string(),
            },
        );

        // Evict oldest entries if over max
        self.entries.truncate(max);
    }

    /// Look up a recent entry by directory.
    pub fn lookup(&self, directory: &PathBuf) -> Option<&RecentEntry> {
        self.entries.iter().find(|e| &e.directory == directory)
    }

    /// Get all entries, ordered by most recent first.
    pub fn all(&self) -> &[RecentEntry] {
        &self.entries
    }

    /// Get the number of entries.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// Check if there are no entries.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Load recent entries from the cache file.
    ///
    /// Returns a default (empty) RecentEntries if the file is missing,
    /// unreadable, or contains invalid JSON. This ensures the plugin
    /// never crashes due to a corrupted recent file.
    pub fn load() -> Self {
        std::fs::read_to_string(RECENT_FILE_PATH)
            .ok()
            .and_then(|data| serde_json::from_str(&data).ok())
            .unwrap_or_default()
    }

    /// Save recent entries to the cache file.
    ///
    /// Returns Ok(()) on success, or an error message on failure.
    /// Callers should log but not crash on save errors.
    pub fn save(&self) -> Result<(), String> {
        let json = serde_json::to_string_pretty(self)
            .map_err(|e| format!("Failed to serialize recent entries: {}", e))?;
        std::fs::write(RECENT_FILE_PATH, json)
            .map_err(|e| format!("Failed to write {}: {}", RECENT_FILE_PATH, e))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_entry(dir: &str, name: &str, ts: &str) -> RecentEntry {
        RecentEntry {
            directory: PathBuf::from(dir),
            name: name.to_string(),
            last_used: ts.to_string(),
        }
    }

    #[test]
    fn test_default_is_empty() {
        let recent = RecentEntries::default();
        assert!(recent.is_empty());
        assert_eq!(recent.len(), 0);
        assert_eq!(recent.version, 1);
    }

    #[test]
    fn test_add_single_entry() {
        let mut recent = RecentEntries::new();
        recent.add(
            PathBuf::from("/home/user/project"),
            "project",
            "2025-01-01T00:00:00Z",
            20,
        );

        assert_eq!(recent.len(), 1);
        assert_eq!(recent.entries[0].name, "project");
        assert_eq!(
            recent.entries[0].directory,
            PathBuf::from("/home/user/project")
        );
    }

    #[test]
    fn test_add_moves_existing_to_front() {
        let mut recent = RecentEntries::new();
        recent.add(
            PathBuf::from("/a"),
            "a",
            "2025-01-01T00:00:00Z",
            20,
        );
        recent.add(
            PathBuf::from("/b"),
            "b",
            "2025-01-02T00:00:00Z",
            20,
        );
        recent.add(
            PathBuf::from("/c"),
            "c",
            "2025-01-03T00:00:00Z",
            20,
        );

        // Re-add /a, should move to front with updated timestamp
        recent.add(
            PathBuf::from("/a"),
            "a-updated",
            "2025-01-04T00:00:00Z",
            20,
        );

        assert_eq!(recent.len(), 3);
        assert_eq!(recent.entries[0].directory, PathBuf::from("/a"));
        assert_eq!(recent.entries[0].name, "a-updated");
        assert_eq!(recent.entries[0].last_used, "2025-01-04T00:00:00Z");
        assert_eq!(recent.entries[1].directory, PathBuf::from("/c"));
        assert_eq!(recent.entries[2].directory, PathBuf::from("/b"));
    }

    #[test]
    fn test_add_evicts_oldest_when_over_max() {
        let mut recent = RecentEntries::new();
        let max = 3;

        recent.add(PathBuf::from("/a"), "a", "2025-01-01T00:00:00Z", max);
        recent.add(PathBuf::from("/b"), "b", "2025-01-02T00:00:00Z", max);
        recent.add(PathBuf::from("/c"), "c", "2025-01-03T00:00:00Z", max);
        recent.add(PathBuf::from("/d"), "d", "2025-01-04T00:00:00Z", max);

        assert_eq!(recent.len(), 3);
        // /a should be evicted (oldest)
        assert_eq!(recent.entries[0].directory, PathBuf::from("/d"));
        assert_eq!(recent.entries[1].directory, PathBuf::from("/c"));
        assert_eq!(recent.entries[2].directory, PathBuf::from("/b"));
    }

    #[test]
    fn test_add_existing_does_not_increase_count() {
        let mut recent = RecentEntries::new();
        let max = 3;

        recent.add(PathBuf::from("/a"), "a", "2025-01-01T00:00:00Z", max);
        recent.add(PathBuf::from("/b"), "b", "2025-01-02T00:00:00Z", max);
        recent.add(PathBuf::from("/c"), "c", "2025-01-03T00:00:00Z", max);

        // Re-add /a should not push anything out
        recent.add(PathBuf::from("/a"), "a", "2025-01-04T00:00:00Z", max);

        assert_eq!(recent.len(), 3);
        assert!(recent.lookup(&PathBuf::from("/a")).is_some());
        assert!(recent.lookup(&PathBuf::from("/b")).is_some());
        assert!(recent.lookup(&PathBuf::from("/c")).is_some());
    }

    #[test]
    fn test_lookup_found() {
        let mut recent = RecentEntries::new();
        recent.add(
            PathBuf::from("/home/user/project"),
            "project",
            "2025-01-01T00:00:00Z",
            20,
        );

        let found = recent.lookup(&PathBuf::from("/home/user/project"));
        assert!(found.is_some());
        assert_eq!(found.unwrap().name, "project");
    }

    #[test]
    fn test_lookup_not_found() {
        let recent = RecentEntries::new();
        assert!(recent.lookup(&PathBuf::from("/nonexistent")).is_none());
    }

    #[test]
    fn test_all_returns_mru_order() {
        let mut recent = RecentEntries::new();
        recent.add(PathBuf::from("/a"), "a", "2025-01-01T00:00:00Z", 20);
        recent.add(PathBuf::from("/b"), "b", "2025-01-02T00:00:00Z", 20);
        recent.add(PathBuf::from("/c"), "c", "2025-01-03T00:00:00Z", 20);

        let all = recent.all();
        assert_eq!(all.len(), 3);
        // Most recent first
        assert_eq!(all[0].directory, PathBuf::from("/c"));
        assert_eq!(all[1].directory, PathBuf::from("/b"));
        assert_eq!(all[2].directory, PathBuf::from("/a"));
    }

    #[test]
    fn test_max_recent_zero_uses_default() {
        let mut recent = RecentEntries::new();
        // With max_recent=0, should use DEFAULT_MAX_RECENT (20)
        for i in 0..25 {
            recent.add(
                PathBuf::from(format!("/dir/{}", i)),
                &format!("dir-{}", i),
                &format!("2025-01-{:02}T00:00:00Z", (i % 28) + 1),
                0,
            );
        }
        assert_eq!(recent.len(), DEFAULT_MAX_RECENT);
    }

    #[test]
    fn test_serde_roundtrip() {
        let mut recent = RecentEntries::new();
        recent.add(
            PathBuf::from("/home/user/project"),
            "project",
            "2025-01-01T12:00:00Z",
            20,
        );
        recent.add(
            PathBuf::from("/home/user/other"),
            "other",
            "2025-01-02T12:00:00Z",
            20,
        );

        let json = serde_json::to_string_pretty(&recent).unwrap();
        let deserialized: RecentEntries = serde_json::from_str(&json).unwrap();

        assert_eq!(recent, deserialized);
    }

    #[test]
    fn test_deserialize_empty_json_object() {
        // Missing fields should fail gracefully
        let json = "{}";
        let result: Result<RecentEntries, _> = serde_json::from_str(json);
        // serde will fail because version and entries are required
        assert!(result.is_err());
    }

    #[test]
    fn test_deserialize_invalid_json() {
        let json = "not valid json at all";
        let result: Result<RecentEntries, _> = serde_json::from_str(json);
        assert!(result.is_err());
    }

    #[test]
    fn test_deserialize_valid_empty_entries() {
        let json = r#"{"version": 1, "entries": []}"#;
        let result: RecentEntries = serde_json::from_str(json).unwrap();
        assert_eq!(result.version, 1);
        assert!(result.entries.is_empty());
    }

    #[test]
    fn test_deserialize_with_entries() {
        let json = r#"{
            "version": 1,
            "entries": [
                {
                    "directory": "/home/user/project",
                    "name": "project",
                    "last_used": "2025-01-01T00:00:00Z"
                }
            ]
        }"#;
        let result: RecentEntries = serde_json::from_str(json).unwrap();
        assert_eq!(result.entries.len(), 1);
        assert_eq!(result.entries[0].name, "project");
    }

    #[test]
    fn test_load_returns_default_on_missing_file() {
        // load() should return default when file doesn't exist
        // (In tests, /cache/recent.json won't exist)
        let recent = RecentEntries::load();
        assert!(recent.is_empty());
        assert_eq!(recent.version, 1);
    }
}
