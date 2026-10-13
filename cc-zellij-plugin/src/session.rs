// T002: Session struct with Activity enum and state transition logic

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
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
        Activity::Working | Activity::Waiting(_) => activity.color(),
        Activity::Done | Activity::AgentDone => {
            let t = sqrt_ratio_1024(elapsed_secs, done_timeout.max(1));
            lerp_color_i(activity.color(), (180, 175, 195), t)
        }
        Activity::Idle | Activity::Init => {
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

/// A single AI agent session running in a Zellij tab.
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
    #[serde(default)]
    pub badges: Vec<String>,
    /// Agent type for this session (e.g., "claude", "opencode").
    /// Set from the first hook event's agent field.
    #[serde(default)]
    pub agent_name: Option<String>,
    /// Agent indicator (e.g., "CC", "OC") from the Go CLI.
    /// Single source of truth for sidebar display.
    #[serde(default)]
    pub agent_indicator: Option<String>,
    /// Outstanding permission prompts, counted per agent that raised them.
    ///
    /// The key is the hook's `agent_id`; the main agent uses the empty string.
    /// A session is in Waiting as long as any agent has a non-zero count, so a
    /// prompt raised by a subagent keeps the whole row in attention (the
    /// attend key then lands here). Each agent's count drains when that same
    /// agent resumes work (its next `PostToolUse`), replies to the prompt
    /// (`PermissionReply`), or finishes (`SubagentStop`); `Stop` and
    /// `UserPromptSubmit` clear every agent at once. This per-agent accounting
    /// is what keeps a main agent parked inside a long `Task` from staying
    /// stuck in Waiting while its subagents churn.
    ///
    /// Renamed from the old scalar `pending_permissions`; an old cache's
    /// numeric field is simply ignored and the map rebuilds from live events.
    #[serde(default)]
    pub pending_prompts: BTreeMap<String, u32>,
    /// Whether this session is currently operating inside a `.claude/worktrees/`
    /// directory. Set from CWD changes; persisted for reattach.
    #[serde(default)]
    pub in_worktree: bool,
    /// Profile name from CC_DECK_PROFILE (e.g., "work", "private").
    /// Set from the first hook event carrying a profile field.
    #[serde(default)]
    pub profile: Option<String>,
    /// Profile color as #RRGGBB string. Set alongside the profile name,
    /// parsed into an RGB tuple for rendering by the controller.
    #[serde(default)]
    pub profile_color: Option<(u8, u8, u8)>,
    /// Declared profile icon, set alongside the profile name. Drives the
    /// help overlay legend; `None` when the profile declares no icon.
    #[serde(default)]
    pub profile_icon: Option<String>,
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
            badges: Vec::new(),
            agent_name: None,
            agent_indicator: None,
            pending_prompts: BTreeMap::new(),
            in_worktree: false,
            profile: None,
            profile_color: None,
            profile_icon: None,
        }
    }

    /// Transition to a new activity state based on a hook event.
    /// Returns true if the state actually changed.
    pub fn transition(&mut self, new_activity: Activity) -> bool {
        // The Waiting lifecycle is owned entirely by `apply_hook` via the
        // per-agent `pending_prompts` map: it forces Waiting while any prompt
        // is outstanding and only calls `transition` to another state once the
        // map is empty. `transition` is therefore a plain state setter here
        // (plus the two adjustments below), and leaving Waiting is safe.

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

    /// Total outstanding prompts across all agents under this session.
    pub fn pending_prompt_count(&self) -> u32 {
        self.pending_prompts.values().copied().sum()
    }

    /// Apply a hook event that maps to `activity` to this session.
    ///
    /// Permission bookkeeping is tracked per agent (keyed by `agent_id`, main
    /// agent = `""`). The session is Waiting while any agent has an outstanding
    /// prompt, so a subagent's prompt keeps the whole row in attention; it
    /// drains when that same agent resumes, replies, or finishes:
    ///
    /// - `PermissionRequest` records a prompt for the emitting agent → Waiting.
    /// - `PermissionReply` answers one of that agent's prompts (OpenCode sends
    ///   this explicitly).
    /// - A `PostToolUse` from an agent is an implicit reply for that agent,
    ///   because Claude Code sends no `PermissionReply`: the agent could only
    ///   emit it after its prompt was answered. (`PreToolUse` is not a reply,
    ///   so OpenCode's parallel tool calls do not clear a sibling's prompt.)
    /// - `SubagentStop` clears the finishing subagent's prompts entirely.
    /// - `Stop` (Done), `UserPromptSubmit`, and `SessionStart` clear every
    ///   agent: the turn or session has moved on.
    ///
    /// While any prompt remains the event is absorbed and the row stays
    /// Waiting; every absorbed event still refreshes `last_event_ts`.
    pub fn apply_hook(
        &mut self,
        event: &str,
        activity: Activity,
        agent_id: Option<&str>,
    ) -> HookOutcome {
        let key = agent_id.unwrap_or("");

        // Record a new prompt for the emitting agent.
        if matches!(activity, Activity::Waiting(WaitReason::Permission)) {
            *self.pending_prompts.entry(key.to_string()).or_insert(0) += 1;
        }

        // Drain prompts. A blocked agent cannot emit further work until its
        // prompt is answered, so any of these events proves resolution.
        let drain_one = |prompts: &mut BTreeMap<String, u32>, k: &str| {
            if let Some(count) = prompts.get_mut(k) {
                *count = count.saturating_sub(1);
                if *count == 0 {
                    prompts.remove(k);
                }
            }
        };
        match (event, &activity) {
            ("Stop", _) | ("UserPromptSubmit", _) | ("SessionStart", _) => {
                self.pending_prompts.clear();
            }
            ("PermissionReply", _) => drain_one(&mut self.pending_prompts, key),
            ("SubagentStop", _) => {
                self.pending_prompts.remove(key);
            }
            ("PostToolUse", Activity::Working) | ("PostToolUseFailure", Activity::Working) => {
                drain_one(&mut self.pending_prompts, key);
            }
            _ => {}
        }

        // As long as any agent is still blocked, the row stays in attention,
        // regardless of this event's nominal activity. Entering Waiting for the
        // first time is a real change (the UI must redraw); staying in Waiting
        // absorbs the event.
        if self.pending_prompt_count() > 0 {
            if !matches!(self.activity, Activity::Waiting(WaitReason::Permission)) {
                self.transition(Activity::Waiting(WaitReason::Permission));
                return HookOutcome::Changed;
            }
            self.last_event_ts = unix_now();
            return HookOutcome::Absorbed;
        }

        // Nobody blocked: apply the event's own activity.
        if self.transition(activity) {
            HookOutcome::Changed
        } else {
            HookOutcome::Unchanged
        }
    }
}

/// What applying a hook event did to a session's activity.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HookOutcome {
    /// The activity changed.
    Changed,
    /// The event was applied but the activity is the same (or the transition
    /// was rejected by the Waiting guard).
    Unchanged,
    /// The event was swallowed by the permission bookkeeping.
    Absorbed,
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
    fn test_apply_hook_keeps_waiting_until_requesting_agent_resumes() {
        // The Waiting lifecycle lives in apply_hook (transition is now a plain
        // setter). A main-agent prompt stays in attention through unrelated
        // subagent activity and clears only when the main agent itself resumes.
        let mut session = Session::new(1, "test".into());
        session.apply_hook(
            "PermissionRequest",
            Activity::Waiting(WaitReason::Permission),
            None,
        );
        assert!(session.activity.is_waiting());

        // A subagent's PostToolUse keys a different agent: the main prompt holds.
        session.apply_hook("PostToolUse", Activity::Working, Some("sub-1"));
        assert_eq!(session.activity, Activity::Waiting(WaitReason::Permission));

        // The main agent's own PostToolUse drains the prompt.
        session.apply_hook("PostToolUse", Activity::Working, None);
        assert_eq!(session.activity, Activity::Working);
    }

    #[test]
    fn test_apply_hook_subagent_stop_does_not_clear_main_prompt() {
        // A subagent finishing (SubagentStop) must not answer the main agent's
        // prompt: it only clears that subagent's own pending entry.
        let mut session = Session::new(1, "test".into());
        session.apply_hook(
            "PermissionRequest",
            Activity::Waiting(WaitReason::Permission),
            None,
        );
        session.apply_hook("SubagentStop", Activity::AgentDone, Some("sub-1"));
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
    fn test_session_deserialize_ignores_unknown_fields() {
        let json = r#"{
            "pane_id": 42,
            "session_id": "test-session",
            "display_name": "my-project",
            "activity": "Working",
            "tab_index": null,
            "tab_name": null,
            "working_dir": "/home/user/project",
            "git_branch": null,
            "last_event_ts": 1234567890,
            "manually_renamed": false,
            "paused": false,
            "meta_ts": 0,
            "done_attended": false,
            "pending_tab_rename": false,
            "active_subagents": 3
        }"#;
        let session: Session = serde_json::from_str(json).unwrap();
        assert_eq!(session.pane_id, 42);
        assert_eq!(session.session_id, "test-session");
        assert_eq!(session.activity, Activity::Working);
        // Pre-profile JSON loads with no profile fields.
        assert_eq!(session.profile, None);
        assert_eq!(session.profile_color, None);
        assert_eq!(session.profile_icon, None);
    }

    #[test]
    fn test_session_profile_fields_round_trip() {
        let mut s = Session::new(7, "sid".into());
        s.profile = Some("work".into());
        s.profile_color = Some((90, 192, 226));
        s.profile_icon = Some("W".into());
        let json = serde_json::to_string(&s).unwrap();
        let restored: Session = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.profile.as_deref(), Some("work"));
        assert_eq!(restored.profile_color, Some((90, 192, 226)));
        assert_eq!(restored.profile_icon.as_deref(), Some("W"));
    }

    #[test]
    fn test_faded_color_active_states_unchanged() {
        assert_eq!(faded_color(&Activity::Working, 9999, 120, 3600), Activity::Working.color());
        assert_eq!(
            faded_color(&Activity::Waiting(WaitReason::Permission), 9999, 120, 3600),
            Activity::Waiting(WaitReason::Permission).color()
        );
    }

    #[test]
    fn test_faded_color_init_fades_like_idle() {
        assert_eq!(faded_color(&Activity::Init, 0, 120, 3600), Activity::Init.color());
        // After full fade period, Init should reach the same dark grey as Idle
        assert_eq!(faded_color(&Activity::Init, 3600, 120, 3600), (70, 65, 80));
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

#[cfg(test)]
mod hook_fuzz {
    //! Random hook sequences against a single session. The controller
    //! forwards every hook through `apply_hook`, so these invariants are the
    //! ones the sidebar's attention indicator depends on.
    use super::*;
    use crate::pipe_handler::hook_event_to_activity;
    use proptest::prelude::*;

    const EVENTS: [&str; 11] = [
        "SessionStart",
        "PreToolUse",
        "PostToolUse",
        "PostToolUseFailure",
        "UserPromptSubmit",
        "PermissionRequest",
        "PermissionReply",
        "Stop",
        "SubagentStart",
        "SubagentStop",
        "Notification",
    ];

    /// `main` picks the agent: main agent (no agent_id) when true, else a
    /// single stand-in subagent key.
    fn apply(session: &mut Session, event: &str, main: bool) {
        let agent_id = if main { None } else { Some("sub") };
        if let Some(activity) = hook_event_to_activity(event, None) {
            session.apply_hook(event, activity, agent_id);
        }
    }

    proptest! {
        #![proptest_config(ProptestConfig::with_cases(2000))]

        #[test]
        fn pending_prompts_and_waiting_agree(
            steps in prop::collection::vec((0usize..EVENTS.len(), any::<bool>()), 1..40)
        ) {
            // The row is in Waiting(Permission) exactly when some agent still
            // has an outstanding prompt. This is the invariant the attention
            // indicator and the attend key depend on.
            let mut session = Session::new(1, "s".into());
            for (idx, main) in steps {
                apply(&mut session, EVENTS[idx], main);
                let waiting_on_permission =
                    session.activity == Activity::Waiting(WaitReason::Permission);
                prop_assert_eq!(
                    session.pending_prompt_count() > 0,
                    waiting_on_permission,
                    "pending={} activity={:?} after {}",
                    session.pending_prompt_count(),
                    session.activity,
                    EVENTS[idx]
                );
            }
        }

        #[test]
        fn stop_always_ends_waiting(
            steps in prop::collection::vec((0usize..EVENTS.len(), any::<bool>()), 0..40)
        ) {
            let mut session = Session::new(1, "s".into());
            for (idx, main) in steps {
                apply(&mut session, EVENTS[idx], main);
            }
            apply(&mut session, "Stop", true);
            prop_assert_eq!(session.pending_prompt_count(), 0);
            prop_assert_eq!(session.activity, Activity::Done);
        }

        #[test]
        fn subagent_activity_never_answers_a_main_prompt(
            n in 1u32..4,
            steps in prop::collection::vec(0usize..EVENTS.len(), 0..20)
        ) {
            // A main-agent prompt can only be answered by the main agent. No
            // amount of subagent tool activity clears it, so the row keeps
            // attention until the main agent itself resumes (or the turn ends).
            let mut session = Session::new(1, "s".into());
            for _ in 0..n {
                apply(&mut session, "PermissionRequest", true);
            }
            for idx in steps {
                let event = EVENTS[idx];
                // Skip turn/session-level resets and explicit main replies;
                // those legitimately clear everything and are main-agent events.
                if matches!(event, "Stop" | "UserPromptSubmit" | "SessionStart") {
                    continue;
                }
                apply(&mut session, event, false); // all from a subagent
            }
            prop_assert_eq!(session.pending_prompts.get("").copied().unwrap_or(0), n);
            prop_assert_eq!(session.activity, Activity::Waiting(WaitReason::Permission));
        }

        #[test]
        fn subagent_prompt_clears_when_that_subagent_resumes(
            drain_event in prop::sample::select(vec!["PostToolUse", "PostToolUseFailure", "SubagentStop"])
        ) {
            // The reported bug: a subagent prompt pinned the whole row in
            // attention forever. It must clear as soon as that subagent
            // produces its next tool event (or finishes).
            let mut session = Session::new(1, "s".into());
            apply(&mut session, "PermissionRequest", false); // subagent "sub"
            prop_assert!(session.activity.is_waiting());
            apply(&mut session, drain_event, false); // same subagent resumes/finishes
            prop_assert_eq!(session.pending_prompt_count(), 0);
            prop_assert!(!session.activity.is_waiting());
        }
    }
}
