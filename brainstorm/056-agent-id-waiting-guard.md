# 056: Use `agent_id` to Fix Waiting State Detection

## Problem

The cc-deck sidebar plugin tracks Claude Code session activity via hook events.
When a session needs permission (`PermissionRequest`), it shows a red warning indicator.
The current logic uses an `active_subagents` counter to suppress `Working` transitions from subagent tool events that would incorrectly clear the `Waiting(Permission)` state.

This counter-based heuristic has two race conditions:

1. **Stuck in Working**: Subagent tool events (`PreToolUse`/`PostToolUse`) can arrive before the `PermissionRequest` hook fires, keeping the session in `Working` and preventing it from ever entering `Waiting`.
2. **Stuck in Waiting**: After the user answers a permission prompt, the main agent's `PostToolUse` is suppressed because `active_subagents > 0`. The session stays in `Waiting` even though work is actively proceeding.

## Root Cause

The `active_subagents` counter is an indirect heuristic.
It cannot distinguish whether a given `PostToolUse` came from the main agent (permission answered) or a parallel subagent (unrelated tool call).
Claude Code hook payloads include an `agent_id` field that is present only for subagent events and absent for main-agent events.
This field is not currently parsed or forwarded by cc-deck.

## Solution

Parse and forward the `agent_id` field through the hook pipeline.
Replace the counter-based guard with a direct `agent_id` check:

- **`agent_id` absent** (main agent): allow `Working` transition, clearing `Waiting`
- **`agent_id` present** (subagent): suppress `Working` transition, preserving `Waiting`

This is deterministic regardless of event ordering.

## Changes

### Go CLI (`cc-deck/internal/cmd/hook.go`)

Add `AgentID string` field (json tag `agent_id,omitempty`) to both `hookPayload` and `pipePayload`.
Forward the field in `runHook` when building the pipe payload.

### Rust pipe handler (`cc-zellij-plugin/src/pipe_handler.rs`)

Add `pub agent_id: Option<String>` to `HookPayload`.
Update test payloads.

### Rust hook processor (`cc-zellij-plugin/src/controller/hooks.rs`)

Replace the guard logic (lines 147-167) that checks `s.active_subagents > 0` with a check on `hook.agent_id.is_some()`.
Remove SubagentStart/SubagentStop counter tracking (lines 127-145).
Remove all `active_subagents`-related tests.
Update `make_hook` test helper to include `agent_id: None`.
Add new tests that verify `agent_id`-based suppression.

### Rust session struct (`cc-zellij-plugin/src/session.rs`)

Remove `active_subagents: u32` from `Session` struct, `Session::new`, and serde defaults.

### Session replacement (`hooks.rs`)

Remove `active_subagents = 0` reset from session replacement block.

## What Stays the Same

- `Session::transition()` still blocks `AgentDone` from clearing `Waiting` (separate safety check)
- `SubagentStart`/`SubagentStop` events still map to `Working`/`AgentDone` via `hook_event_to_activity`
- The overall hook pipeline architecture (Go CLI reads stdin, pipes to Zellij plugin) is unchanged

## Verification

1. `cargo test` in `cc-zellij-plugin/` passes
2. `cargo clippy` has no warnings
3. `go build ./...` in `cc-deck/` compiles
4. Manual test: start a Claude Code session with subagents, trigger a permission prompt, verify the sidebar shows the red warning indicator and clears it after the prompt is answered
