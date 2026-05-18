# Agent-ID Waiting Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `active_subagents` counter heuristic with Claude Code's `agent_id` hook field to fix race conditions in permission state tracking.

**Architecture:** Parse `agent_id` from hook payloads through the Go CLI and Rust plugin pipeline. Use its presence/absence to determine whether a tool event is from a subagent (suppress Working transition during Waiting) or the main agent (allow it). Remove the `active_subagents` counter entirely.

**Tech Stack:** Go 1.25 (CLI), Rust stable wasm32-wasip1 (plugin), serde_json

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cc-deck/internal/cmd/hook.go` | Modify | Add `AgentID` field to Go structs, forward through pipeline |
| `cc-zellij-plugin/src/pipe_handler.rs` | Modify | Add `agent_id` to `HookPayload`, update tests |
| `cc-zellij-plugin/src/controller/hooks.rs` | Modify | Replace guard logic, remove counter tracking, update tests |
| `cc-zellij-plugin/src/session.rs` | Modify | Remove `active_subagents` field |
| `cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs` | Modify | Add `make_hook_pipe_with_agent_id` helper |

---

### Task 1: Add `agent_id` to Go hook structs

**Files:**
- Modify: `cc-deck/internal/cmd/hook.go:23-37`

- [ ] **Step 1: Add `AgentID` field to `hookPayload`**

In `cc-deck/internal/cmd/hook.go`, add `AgentID` to the input struct:

```go
type hookPayload struct {
	SessionID string `json:"session_id,omitempty"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}
```

- [ ] **Step 2: Add `AgentID` field to `pipePayload`**

```go
type pipePayload struct {
	SessionID string `json:"session_id,omitempty"`
	PaneID    uint32 `json:"pane_id"`
	HookEvent string `json:"hook_event_name"`
	ToolName  string `json:"tool_name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}
```

- [ ] **Step 3: Forward `AgentID` in `runHook`**

In the `runHook` function, add `AgentID` to the payload construction (around line 140):

```go
payload := pipePayload{
	SessionID: hook.SessionID,
	PaneID:    paneID,
	HookEvent: hook.HookEvent,
	ToolName:  hook.ToolName,
	CWD:       hook.CWD,
	AgentID:   hook.AgentID,
}
```

- [ ] **Step 4: Verify Go compiles**

Run: `cd cc-deck && go build ./...`
Expected: Clean compilation, no errors.

- [ ] **Step 5: Commit**

```bash
git add cc-deck/internal/cmd/hook.go
git commit -m "feat(hook): forward agent_id from Claude Code hook payloads"
```

---

### Task 2: Add `agent_id` to Rust `HookPayload`

**Files:**
- Modify: `cc-zellij-plugin/src/pipe_handler.rs:7-14`

- [ ] **Step 1: Write test for agent_id parsing**

Add to the existing `tests` module in `pipe_handler.rs`:

```rust
#[test]
fn test_parse_hook_payload_with_agent_id() {
    let json = r#"{"session_id":"abc","pane_id":42,"hook_event_name":"PostToolUse","tool_name":"Bash","agent_id":"sub-123"}"#;
    let payload: HookPayload = serde_json::from_str(json).unwrap();
    assert_eq!(payload.agent_id.as_deref(), Some("sub-123"));
}

#[test]
fn test_parse_hook_payload_without_agent_id() {
    let json = r#"{"pane_id":1,"hook_event_name":"PostToolUse"}"#;
    let payload: HookPayload = serde_json::from_str(json).unwrap();
    assert!(payload.agent_id.is_none());
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cc-zellij-plugin && cargo test test_parse_hook_payload_with_agent_id -- --nocapture`
Expected: FAIL with compilation error (no `agent_id` field on `HookPayload`).

- [ ] **Step 3: Add `agent_id` field to `HookPayload`**

```rust
#[derive(Debug, Deserialize)]
pub struct HookPayload {
    pub session_id: Option<String>,
    pub pane_id: u32,
    pub hook_event_name: String,
    pub tool_name: Option<String>,
    pub cwd: Option<String>,
    pub agent_id: Option<String>,
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd cc-zellij-plugin && cargo test test_parse_hook_payload -- --nocapture`
Expected: All `test_parse_hook_payload*` tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cc-zellij-plugin/src/pipe_handler.rs
git commit -m "feat(pipe): add agent_id field to HookPayload"
```

---

### Task 3: Add test helper for agent_id hook pipes

**Files:**
- Modify: `cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs`

- [ ] **Step 1: Add `make_hook_pipe_with_agent_id` helper**

Add after the existing `make_hook_pipe_with_cwd` function (around line 117):

```rust
/// Construct a hook event PipeMessage with agent_id (simulating a subagent event).
pub fn make_hook_pipe_with_agent_id(hook_event: &str, pane_id: u32, agent_id: &str) -> PipeMessage {
    let payload = serde_json::json!({
        "session_id": "test-session",
        "pane_id": pane_id,
        "hook_event_name": hook_event,
        "agent_id": agent_id,
    });
    PipeMessage {
        source: PipeSource::Cli("test-pipe".to_string()),
        name: "cc-deck:hook".to_string(),
        payload: Some(payload.to_string()),
        args: std::collections::BTreeMap::new(),
        is_private: false,
    }
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd cc-zellij-plugin && cargo test --no-run`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
git commit -m "test: add make_hook_pipe_with_agent_id test helper"
```

---

### Task 4: Remove `active_subagents` from Session

**Files:**
- Modify: `cc-zellij-plugin/src/session.rs:166-171,191`

- [ ] **Step 1: Remove `active_subagents` field from Session struct**

Remove these lines from the `Session` struct (lines 166-171):

```rust
    /// Number of currently active subagents (between SubagentStart/SubagentStop).
    /// Used to suppress Working transitions that would clear Waiting(Permission)
    /// when the tool events are coming from parallel subagents, not from the
    /// main agent answering the permission prompt.
    #[serde(default)]
    pub active_subagents: u32,
```

- [ ] **Step 2: Remove `active_subagents` from `Session::new`**

Remove this line from `Session::new` (line 191):

```rust
            active_subagents: 0,
```

- [ ] **Step 3: Fix compilation errors in hooks.rs**

In `cc-zellij-plugin/src/controller/hooks.rs`, remove the `active_subagents = 0` reset in the session replacement block (line 114):

```rust
            session.active_subagents = 0;
```

- [ ] **Step 4: Verify compilation**

Run: `cd cc-zellij-plugin && cargo build 2>&1 | head -30`
Expected: Compilation errors in hooks.rs tests referencing `active_subagents`. These are fixed in the next task. Do not commit yet; this task continues into Task 5.

---

### Task 5: Replace guard logic and update tests in hooks.rs

**Files:**
- Modify: `cc-zellij-plugin/src/controller/hooks.rs:53-167,680-799`

- [ ] **Step 1: Write failing test for agent_id-based suppression**

Replace the subagent-related tests at the bottom of `hooks.rs` (the `test_subagent_counter_*` and `test_waiting_*` tests, lines 680-799) with these new tests:

First, update the `make_hook` helper to include `agent_id`:

```rust
fn make_hook(pane_id: u32, event: &str) -> HookPayload {
    HookPayload {
        session_id: Some("test-session".to_string()),
        pane_id,
        hook_event_name: event.to_string(),
        tool_name: None,
        cwd: None,
        agent_id: None,
    }
}

fn make_subagent_hook(pane_id: u32, event: &str) -> HookPayload {
    HookPayload {
        session_id: Some("test-session".to_string()),
        pane_id,
        hook_event_name: event.to_string(),
        tool_name: None,
        cwd: None,
        agent_id: Some("sub-1".to_string()),
    }
}
```

Then replace the old tests with:

```rust
#[test]
fn test_waiting_preserved_when_subagent_event() {
    let mut state = ControllerState::default();
    let mut s = Session::new(42, "test-session".into());
    s.activity = Activity::Waiting(crate::session::WaitReason::Permission);
    state.sessions.insert(42, s);

    // PreToolUse from subagent should NOT clear Waiting
    let changed = process_hook(&mut state, make_subagent_hook(42, "PreToolUse"));
    assert!(!changed);
    assert_eq!(
        state.sessions[&42].activity,
        Activity::Waiting(crate::session::WaitReason::Permission)
    );

    // PostToolUse from subagent should NOT clear Waiting either
    let changed = process_hook(&mut state, make_subagent_hook(42, "PostToolUse"));
    assert!(!changed);
    assert_eq!(
        state.sessions[&42].activity,
        Activity::Waiting(crate::session::WaitReason::Permission)
    );
}

#[test]
fn test_waiting_clears_on_main_agent_event() {
    let mut state = ControllerState::default();
    let mut s = Session::new(42, "test-session".into());
    s.activity = Activity::Waiting(crate::session::WaitReason::Permission);
    state.sessions.insert(42, s);

    // PostToolUse from main agent (no agent_id) should clear Waiting
    let changed = process_hook(&mut state, make_hook(42, "PostToolUse"));
    assert!(changed);
    assert_eq!(state.sessions[&42].activity, Activity::Working);
}

#[test]
fn test_waiting_clears_on_main_agent_even_with_subagents_running() {
    let mut state = ControllerState::default();
    let mut s = Session::new(42, "test-session".into());
    s.activity = Activity::Waiting(crate::session::WaitReason::Permission);
    state.sessions.insert(42, s);

    // Subagent events arrive (don't clear Waiting)
    process_hook(&mut state, make_subagent_hook(42, "PreToolUse"));
    assert!(state.sessions[&42].activity.is_waiting());

    process_hook(&mut state, make_subagent_hook(42, "PostToolUse"));
    assert!(state.sessions[&42].activity.is_waiting());

    // Main agent PostToolUse arrives (clears Waiting)
    let changed = process_hook(&mut state, make_hook(42, "PostToolUse"));
    assert!(changed);
    assert_eq!(state.sessions[&42].activity, Activity::Working);
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cc-zellij-plugin && cargo test test_waiting -- --nocapture 2>&1 | head -20`
Expected: FAIL because the old guard logic using `active_subagents` no longer compiles.

- [ ] **Step 3: Remove SubagentStart/SubagentStop counter tracking**

In `hooks.rs`, remove the counter tracking block (lines 127-145):

```rust
    // Track active subagent count so we can distinguish subagent tool events
    // from main-agent tool events when deciding whether to clear Waiting state.
    if hook.hook_event_name == "SubagentStart" {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.active_subagents = s.active_subagents.saturating_add(1);
            crate::debug_log(&format!(
                "CTRL HOOK: pane={} SubagentStart, active_subagents={}",
                hook.pane_id, s.active_subagents,
            ));
        }
    } else if hook.hook_event_name == "SubagentStop" {
        if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
            s.active_subagents = s.active_subagents.saturating_sub(1);
            crate::debug_log(&format!(
                "CTRL HOOK: pane={} SubagentStop, active_subagents={}",
                hook.pane_id, s.active_subagents,
            ));
        }
    }
```

- [ ] **Step 4: Replace the guard logic**

Replace the old guard block (lines 147-167) and its preceding comment (lines 53-64) with:

Old code to remove (lines 53-64):
```rust
    // PostToolUse/PostToolUseFailure is allowed to clear Waiting state,
    // but only when no subagents are active (see suppression logic below).
    //
    // Without subagent tracking, these events would clear Waiting(Permission)
    // even when they originated from parallel subagent tool calls, not from
    // the user answering the permission prompt. The active_subagents counter
    // (incremented on SubagentStart, decremented on SubagentStop) lets us
    // suppress Working transitions while subagents are running.
    //
    // When no subagents are active, PostToolUse freely clears Waiting to
    // handle the auto-approve case: PermissionRequest fires but the tool
    // runs immediately, and PostToolUse follows within milliseconds.
```

Old guard logic to replace (lines 147-167):
```rust
    if matches!(activity, Activity::Working) {
        if let Some(s) = state.sessions.get(&hook.pane_id) {
            if s.activity.is_waiting() && s.active_subagents > 0 {
                crate::debug_log(&format!(
                    "CTRL HOOK: pane={} suppressing Working during Waiting (active_subagents={})",
                    hook.pane_id, s.active_subagents,
                ));
                // Still refresh the timestamp so the session doesn't appear stale
                if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
                    s.last_event_ts = session::unix_now();
                }
                return false;
            }
        }
    }
```

New guard logic:
```rust
    // Suppress Working transitions from subagent tool events when the session
    // is waiting for permission. Subagent events carry agent_id; main-agent
    // events do not. Only main-agent PostToolUse should clear Waiting.
    if matches!(activity, Activity::Working) {
        if let Some(s) = state.sessions.get(&hook.pane_id) {
            if s.activity.is_waiting() && hook.agent_id.is_some() {
                crate::debug_log(&format!(
                    "CTRL HOOK: pane={} suppressing Working during Waiting (subagent event)",
                    hook.pane_id,
                ));
                if let Some(s) = state.sessions.get_mut(&hook.pane_id) {
                    s.last_event_ts = session::unix_now();
                }
                return false;
            }
        }
    }
```

- [ ] **Step 5: Run all tests**

Run: `cd cc-zellij-plugin && cargo test -- --nocapture 2>&1 | tail -20`
Expected: All tests PASS.

- [ ] **Step 6: Run clippy**

Run: `cd cc-zellij-plugin && cargo clippy 2>&1`
Expected: No warnings.

- [ ] **Step 7: Commit**

```bash
git add cc-zellij-plugin/src/session.rs cc-zellij-plugin/src/controller/hooks.rs
git commit -m "fix: use agent_id to guard Waiting state instead of active_subagents counter

Replace the active_subagents counter heuristic with agent_id field check.
Fixes two race conditions:
- Sessions stuck in Working when PermissionRequest fires after subagent tool events
- Sessions stuck in Waiting after permission answered while subagents are active"
```

---

### Task 6: Update integration tests

**Files:**
- Modify: `cc-zellij-plugin/src/controller/integration_tests.rs`

- [ ] **Step 1: Add integration test for subagent vs main-agent Waiting behavior**

Add to `integration_tests.rs`:

```rust
#[test]
fn test_controller_waiting_not_cleared_by_subagent_events() {
    let mut plugin = setup_controller();

    // Start session and enter Waiting
    plugin.pipe(make_hook_pipe("SessionStart", 10));
    plugin.pipe(make_hook_pipe("PermissionRequest", 10));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    // Subagent tool events should NOT clear Waiting
    plugin.pipe(make_hook_pipe_with_agent_id("PreToolUse", 10, "sub-1"));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    plugin.pipe(make_hook_pipe_with_agent_id("PostToolUse", 10, "sub-1"));
    assert!(plugin.test_state().sessions[&10].activity.is_waiting());

    // Main agent PostToolUse SHOULD clear Waiting
    plugin.pipe(make_hook_pipe("PostToolUse", 10));
    assert_eq!(plugin.test_state().sessions[&10].activity, Activity::Working);
}
```

- [ ] **Step 2: Run integration tests**

Run: `cd cc-zellij-plugin && cargo test test_controller_waiting -- --nocapture`
Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run: `cd cc-zellij-plugin && cargo test 2>&1 | tail -5`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cc-zellij-plugin/src/controller/integration_tests.rs
git commit -m "test: add integration test for agent_id-based Waiting guard"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full Rust test suite**

Run: `cd cc-zellij-plugin && cargo test`
Expected: All tests pass.

- [ ] **Step 2: Run clippy**

Run: `cd cc-zellij-plugin && cargo clippy`
Expected: No warnings.

- [ ] **Step 3: Verify Go compilation**

Run: `cd cc-deck && go build ./...`
Expected: Clean build.

- [ ] **Step 4: Verify no remaining references to `active_subagents`**

Run: `rg "active_subagents" cc-zellij-plugin/src/`
Expected: No matches.
