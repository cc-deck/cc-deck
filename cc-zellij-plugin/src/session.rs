// T002: Session struct with Activity enum and state transition logic

use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

pub fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

pub fn unix_now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as u64)
        .unwrap_or(0)
}

/// Why a session is in a waiting state.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum WaitReason {
    /// Session blocked on user permission decision (critical, blocking).
    Permission,
    /// Session paused with informational notification (soft, non-blocking).
    Notification,
}

/// Current activity state of a Claude Code session.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub enum Activity {
    #[default]
    Init,
    Working,
    Waiting(WaitReason),
    Idle,
    Done,
    AgentDone,
}


impl Activity {
    /// Activity indicator character for sidebar rendering.
    pub fn indicator(&self) -> &str {
        match self {
            Activity::Init | Activity::Idle => "○",
            Activity::Working => "●",
            Activity::Waiting(_) => "⚠",
            Activity::Done | Activity::AgentDone => "✓",
        }
    }

    /// RGB color for the activity indicator.
    pub fn color(&self) -> (u8, u8, u8) {
        match self {
            Activity::Init | Activity::Idle => (180, 175, 195),
            Activity::Working => (180, 140, 255),
            Activity::Waiting(WaitReason::Permission) => (255, 109, 109),
            Activity::Waiting(WaitReason::Notification) => (255, 194, 102),
            Activity::Done => (80, 200, 120),
            Activity::AgentDone => (80, 180, 100),
        }
    }

    /// Whether this state represents "needs human attention".
    pub fn is_waiting(&self) -> bool {
        matches!(self, Activity::Waiting(_))
    }
}

/// Compute a time-aware faded color for a session's activity indicator.
///
/// Active states (Working, Waiting, Init) return their static base color.
/// Done/AgentDone fades from green to light grey over `done_timeout` seconds.
/// Idle fades from light grey to dark grey over `idle_fade_secs` seconds.
///
/// Uses a square-root curve (via integer approximation) for perceptually
/// smooth decay: rapid initial darkening that tapers off.
pub fn faded_color(
    activity: &Activity,
    elapsed_secs: u64,
    done_timeout: u64,
    idle_fade_secs: u64,
) -> (u8, u8, u8) {
    match activity {
        Activity::Working | Activity::Waiting(_) | Activity::Init => activity.color(),
        Activity::Done | Activity::AgentDone => {
            let t = sqrt_ratio_1024(elapsed_secs, done_timeout.max(1));
            lerp_color_i(activity.color(), (180, 175, 195), t)
        }
        Activity::Idle => {
            let t = sqrt_ratio_1024(elapsed_secs, idle_fade_secs.max(1));
            lerp_color_i((180, 175, 195), (70, 65, 80), t)
        }
    }
}

/// Compute `sqrt(elapsed / duration)` as a fixed-point value 0..1024.
/// Uses integer square root to avoid WASM float-to-int saturating casts
/// that are incompatible with some wasm-opt versions.
fn sqrt_ratio_1024(elapsed: u64, duration: u64) -> u32 {
    if elapsed >= duration {
        return 1024;
    }
    // sqrt(elapsed/duration) = sqrt(elapsed * 1048576 / duration) / 1024
    // where 1048576 = 1024^2
    let scaled = elapsed.saturating_mul(1_048_576) / duration;
    isqrt(scaled).min(1024) as u32
}

/// Integer square root (Newton's method).
fn isqrt(n: u64) -> u64 {
    if n == 0 {
        return 0;
    }
    let mut x = n;
    let mut y = x.div_ceil(2);
    while y < x {
        x = y;
        y = (x + n / x) / 2;
    }
    x
}

/// Linearly interpolate between two RGB colors using fixed-point integer math.
/// `t` is 0..1024 representing 0.0..1.0.
fn lerp_color_i(from: (u8, u8, u8), to: (u8, u8, u8), t: u32) -> (u8, u8, u8) {
    let t = t.min(1024);
    let lerp = |a: u8, b: u8| -> u8 {
        let a = a as u32;
        let b = b as u32;
        ((a * (1024 - t) + b * t) / 1024) as u8
    };
    (lerp(from.0, to.0), lerp(from.1, to.1), lerp(from.2, to.2))
}

/// A single Claude Code session running in a Zellij tab.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub pane_id: u32,
    pub session_id: String,
    pub display_name: String,
    pub activity: Activity,
    pub tab_index: Option<usize>,
    pub tab_name: Option<String>,
    pub working_dir: Option<String>,
    pub git_branch: Option<String>,
    pub last_event_ts: u64,
    pub manually_renamed: bool,
    #[serde(default)]
    pub paused: bool,
    /// Timestamp of last user metadata change (rename, pause toggle).
    /// Separate from last_event_ts which tracks hook events.
    #[serde(default)]
    pub meta_ts: u64,
    /// Whether this Done/AgentDone session has been attended already.
    /// Once attended, it drops to the Idle tier for subsequent attend presses.
    /// Reset when activity transitions away from Done/AgentDone.
    #[serde(default)]
    pub done_attended: bool,
    /// Whether the tab rename was deferred because tab_index was not yet
    /// available when the display name was first set. Cleared after the
    /// rename is issued in rebuild_pane_map.
    #[serde(default)]
    pub pending_tab_rename: bool,
}

impl Session {
    pub fn new(pane_id: u32, session_id: String) -> Self {
        Self {
            pane_id,
            session_id,
            display_name: format!("session-{pane_id}"),
            activity: Activity::Init,
            tab_index: None,
            tab_name: None,
            working_dir: None,
            git_branch: None,
            last_event_ts: unix_now(),
            manually_renamed: false,
            paused: false,
            meta_ts: 0,
            done_attended: false,
            pending_tab_rename: false,
        }
    }

    /// Transition to a new activity state based on a hook event.
    /// Returns true if the state actually changed.
    pub fn transition(&mut self, new_activity: Activity) -> bool {
        // Waiting can only transition to Working or Done, not back to Idle.
        // AgentDone (SubagentStop) must NOT clear Waiting: a subagent finishing
        // does not mean the permission prompt was answered.
        if matches!(self.activity, Activity::Waiting(_)) {
            match new_activity {
                Activity::Working | Activity::Done => {}
                // Allow upgrading Notification wait to Permission wait
                Activity::Waiting(_) => {}
                _ => return false,
            }
        }

        // Suppress AgentDone when the session is actively Working.
        // SubagentStop fires when a subagent finishes, but the parent
        // agent typically continues immediately with the next tool call.
        // Transitioning to AgentDone here causes a brief green checkmark
        // flicker before the next PreToolUse arrives. Only show AgentDone
        // when the session is idle/init/done (i.e., the subagent was the
        // final action).
        if matches!(new_activity, Activity::AgentDone) && matches!(self.activity, Activity::Working) {
            self.last_event_ts = unix_now();
            return false;
        }

        if self.activity != new_activity {
            // Reset done_attended when leaving Done/AgentDone
            if matches!(self.activity, Activity::Done | Activity::AgentDone)
                && !matches!(new_activity, Activity::Done | Activity::AgentDone)
            {
                self.done_attended = false;
            }
            self.activity = new_activity;
            self.last_event_ts = unix_now();
            true
        } else {
            self.last_event_ts = unix_now();
            false
        }
    }

    /// Elapsed seconds since last activity change.
    pub fn elapsed_secs(&self) -> u64 {
        unix_now().saturating_sub(self.last_event_ts)
    }
}

/// Generate a unique display name by appending a numeric suffix if needed.
pub fn deduplicate_name(base: &str, existing_names: &[&str]) -> String {
    if !existing_names.contains(&base) {
        return base.to_string();
    }
    for i in 2..10_000 {
        let candidate = format!("{base}-{i}");
        if !existing_names.iter().any(|n| *n == candidate) {
            return candidate;
        }
    }
    // Fallback: should never happen with fewer than 10k sessions
    format!("{base}-dup")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_activity_indicators() {
        assert_eq!(Activity::Working.indicator(), "●");
        assert_eq!(Activity::Waiting(WaitReason::Permission).indicator(), "⚠");
        assert_eq!(Activity::Idle.indicator(), "○");
        assert_eq!(Activity::Done.indicator(), "✓");
    }

    #[test]
    fn test_waiting_transition_restrictions() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Waiting(WaitReason::Permission);
        session.last_event_ts = unix_now();

        // Waiting should not transition to Idle
        assert!(!session.transition(Activity::Idle));
        assert_eq!(session.activity, Activity::Waiting(WaitReason::Permission));

        // Waiting should transition to Working
        assert!(session.transition(Activity::Working));
        assert_eq!(session.activity, Activity::Working);
    }

    #[test]
    fn test_waiting_blocks_agent_done() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Waiting(WaitReason::Permission);
        session.last_event_ts = unix_now();

        // AgentDone (SubagentStop) must not clear Waiting
        assert!(!session.transition(Activity::AgentDone));
        assert_eq!(session.activity, Activity::Waiting(WaitReason::Permission));
    }

    #[test]
    fn test_waiting_transition_to_done() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Waiting(WaitReason::Permission);
        session.last_event_ts = unix_now();

        assert!(session.transition(Activity::Done));
        assert_eq!(session.activity, Activity::Done);
    }

    #[test]
    fn test_agent_done_suppressed_when_working() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Working;

        // AgentDone should be suppressed when Working
        assert!(!session.transition(Activity::AgentDone));
        assert_eq!(session.activity, Activity::Working);
    }

    #[test]
    fn test_agent_done_allowed_when_idle() {
        let mut session = Session::new(1, "test".into());
        session.activity = Activity::Idle;

        // AgentDone should be allowed when Idle
        assert!(session.transition(Activity::AgentDone));
        assert_eq!(session.activity, Activity::AgentDone);
    }

    #[test]
    fn test_deduplicate_name() {
        assert_eq!(deduplicate_name("api", &[]), "api");
        assert_eq!(deduplicate_name("api", &["api"]), "api-2");
        assert_eq!(deduplicate_name("api", &["api", "api-2"]), "api-3");
    }

    #[test]
    fn test_deduplicate_name_no_conflict() {
        assert_eq!(deduplicate_name("frontend", &["api", "backend"]), "frontend");
    }

    #[test]
    fn test_lerp_color_endpoints() {
        assert_eq!(lerp_color_i((0, 0, 0), (255, 255, 255), 0), (0, 0, 0));
        assert_eq!(lerp_color_i((0, 0, 0), (255, 255, 255), 1024), (255, 255, 255));
    }

    #[test]
    fn test_lerp_color_midpoint() {
        let (r, g, b) = lerp_color_i((0, 0, 0), (200, 100, 50), 512);
        assert_eq!(r, 100);
        assert_eq!(g, 50);
        assert_eq!(b, 25);
    }

    #[test]
    fn test_lerp_color_clamps() {
        assert_eq!(lerp_color_i((10, 20, 30), (50, 60, 70), 2000), (50, 60, 70));
        assert_eq!(lerp_color_i((10, 20, 30), (50, 60, 70), 0), (10, 20, 30));
    }

    #[test]
    fn test_isqrt() {
        assert_eq!(isqrt(0), 0);
        assert_eq!(isqrt(1), 1);
        assert_eq!(isqrt(4), 2);
        assert_eq!(isqrt(100), 10);
        assert_eq!(isqrt(1_048_576), 1024);
    }

    #[test]
    fn test_sqrt_ratio_1024() {
        assert_eq!(sqrt_ratio_1024(0, 100), 0);
        assert_eq!(sqrt_ratio_1024(100, 100), 1024);
        assert_eq!(sqrt_ratio_1024(200, 100), 1024); // clamped
        // sqrt(0.25) = 0.5 -> 512
        assert_eq!(sqrt_ratio_1024(25, 100), 512);
    }

    #[test]
    fn test_faded_color_active_states_unchanged() {
        // Active states return their base color regardless of elapsed time
        assert_eq!(faded_color(&Activity::Working, 9999, 120, 3600), Activity::Working.color());
        assert_eq!(
            faded_color(&Activity::Waiting(WaitReason::Permission), 9999, 120, 3600),
            Activity::Waiting(WaitReason::Permission).color()
        );
        assert_eq!(faded_color(&Activity::Init, 9999, 120, 3600), Activity::Init.color());
    }

    #[test]
    fn test_faded_color_done_at_zero() {
        assert_eq!(faded_color(&Activity::Done, 0, 120, 3600), Activity::Done.color());
    }

    #[test]
    fn test_faded_color_done_at_timeout() {
        // At done_timeout, should be fully faded to light grey
        let color = faded_color(&Activity::Done, 120, 120, 3600);
        assert_eq!(color, (180, 175, 195));
    }

    #[test]
    fn test_faded_color_done_beyond_timeout() {
        // Beyond timeout, clamped to light grey
        let color = faded_color(&Activity::Done, 999, 120, 3600);
        assert_eq!(color, (180, 175, 195));
    }

    #[test]
    fn test_faded_color_idle_at_zero() {
        assert_eq!(faded_color(&Activity::Idle, 0, 120, 3600), (180, 175, 195));
    }

    #[test]
    fn test_faded_color_idle_at_fade_end() {
        let color = faded_color(&Activity::Idle, 3600, 120, 3600);
        assert_eq!(color, (70, 65, 80));
    }

    #[test]
    fn test_faded_color_idle_midpoint_darker_than_start() {
        let start = faded_color(&Activity::Idle, 0, 120, 3600);
        let mid = faded_color(&Activity::Idle, 900, 120, 3600); // 15 min
        let end = faded_color(&Activity::Idle, 3600, 120, 3600);
        // Each channel should decrease monotonically
        assert!(mid.0 < start.0 && mid.0 > end.0);
        assert!(mid.1 < start.1 && mid.1 > end.1);
        assert!(mid.2 < start.2 && mid.2 > end.2);
    }
}
