// Lightweight performance instrumentation for post-mortem analysis.
//
// Records event counts and timing for hot paths. Stats are dumped
// to /cache/perf.csv on a configurable interval (default 30s).
// The CSV format makes it easy to analyze with standard tools:
//
//   sort -t, -k4 -rn /tmp/zellij-*/plugins/*/cache/perf.csv | head -20
//
// Enable via KDL config: `perf true`
// Dump interval: `perf_interval 30` (seconds)

use std::collections::BTreeMap;

const PERF_PATH: &str = "/cache/perf.csv";

/// Per-event-type accumulated stats.
#[derive(Default, Clone)]
struct EventStats {
    count: u64,
    total_us: u64,
    max_us: u64,
}

/// Performance tracker holding accumulated stats since last dump.
#[derive(Default)]
pub struct PerfTracker {
    /// Whether perf instrumentation is enabled.
    pub enabled: bool,
    /// Dump interval in seconds (default 30).
    pub dump_interval_secs: u64,
    /// Accumulated stats keyed by event label.
    stats: BTreeMap<String, EventStats>,
    /// Timestamp (ms) of last dump.
    last_dump_ms: u64,
    /// Whether the CSV header has been written this session.
    header_written: bool,
}

impl PerfTracker {
    /// Record an event occurrence with a duration in microseconds.
    /// Also useful for gauge values (session count, tab count).
    pub fn record_raw(&mut self, label: &str, duration_us: u64) {
        let entry = self.stats.entry(label.to_string()).or_default();
        entry.count += 1;
        entry.total_us += duration_us;
        if duration_us > entry.max_us {
            entry.max_us = duration_us;
        }
    }

    /// Record an event with timing. Call at the end of the measured block.
    pub fn record(&mut self, label: &str, start_ms: u64) {
        if !self.enabled {
            return;
        }
        let elapsed_us = crate::session::unix_now_ms()
            .saturating_sub(start_ms)
            .saturating_mul(1000);
        self.record_raw(label, elapsed_us);
    }

    /// Record a count-only event (no timing).
    pub fn count(&mut self, label: &str) {
        if !self.enabled {
            return;
        }
        let entry = self.stats.entry(label.to_string()).or_default();
        entry.count += 1;
    }

    /// Check if it's time to dump stats and do so if needed.
    /// Called from the timer handler.
    pub fn maybe_dump(&mut self) {
        if !self.enabled || self.stats.is_empty() {
            return;
        }
        let now_ms = crate::session::unix_now_ms();
        let interval_ms = self.dump_interval_secs.saturating_mul(1000);
        if now_ms.saturating_sub(self.last_dump_ms) < interval_ms {
            return;
        }
        self.dump(now_ms);
        self.last_dump_ms = now_ms;
    }

    /// Force dump current stats to the perf log.
    fn dump(&mut self, now_ms: u64) {
        // Write to file (best-effort; failure does not block stat reset)
        if let Ok(mut f) = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(PERF_PATH)
        {
            use std::io::Write;

            if !self.header_written {
                let _ = writeln!(f, "timestamp_ms,event,count,total_us,max_us,avg_us");
                self.header_written = true;
            }

            for (label, stats) in &self.stats {
                let avg = if stats.count > 0 {
                    stats.total_us / stats.count
                } else {
                    0
                };
                let _ = writeln!(
                    f,
                    "{now_ms},{label},{},{},{},{avg}",
                    stats.count, stats.total_us, stats.max_us
                );
            }
        }

        // Always reset accumulators, even if file write failed
        self.stats.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_record_accumulates() {
        let mut tracker = PerfTracker {
            enabled: true,
            dump_interval_secs: 30,
            ..Default::default()
        };

        tracker.record_raw("test_event", 100);
        tracker.record_raw("test_event", 200);
        tracker.record_raw("test_event", 50);

        let stats = &tracker.stats["test_event"];
        assert_eq!(stats.count, 3);
        assert_eq!(stats.total_us, 350);
        assert_eq!(stats.max_us, 200);
    }

    #[test]
    fn test_count_only() {
        let mut tracker = PerfTracker {
            enabled: true,
            dump_interval_secs: 30,
            ..Default::default()
        };

        tracker.count("pipe_hook");
        tracker.count("pipe_hook");
        tracker.count("pipe_sync");

        assert_eq!(tracker.stats["pipe_hook"].count, 2);
        assert_eq!(tracker.stats["pipe_hook"].total_us, 0);
        assert_eq!(tracker.stats["pipe_sync"].count, 1);
    }

    #[test]
    fn test_disabled_noop() {
        let mut tracker = PerfTracker::default(); // enabled=false
        tracker.record("test", 0);
        tracker.count("test");
        assert!(tracker.stats.is_empty());
    }

    #[test]
    fn test_dump_clears_stats() {
        let mut tracker = PerfTracker {
            enabled: true,
            dump_interval_secs: 30,
            header_written: true,
            ..Default::default()
        };

        tracker.record_raw("event_a", 100);
        tracker.record_raw("event_b", 200);
        assert_eq!(tracker.stats.len(), 2);

        // dump() clears stats (file write will fail in tests, but clear still happens)
        tracker.dump(1000);
        assert!(tracker.stats.is_empty());
    }
}
