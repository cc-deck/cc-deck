# Review Finding: Pipe Death Spiral from Multi-Agent Panes

**Date**: 2026-07-27
**Severity**: High
**Affected**: Session rename stability, agent indicator accuracy, activity state

## Root Cause

When a user runs Codex from inside a Claude Code session (or vice versa), the child agent inherits `$ZELLIJ_PANE_ID`. Both agents send hooks with the same pane ID but different session IDs. The plugin interprets each alternating session ID as a "session replacement," triggering rapid-fire state resets.

## Evidence (from debug.log)

Pane 27 had 16 session replacements. Two agents interleaved:
- Claude Code: `757facc1-7d58-4b41-ae0f-09584d4c365e` (agent "claude")
- Codex CLI: `019fb30e-2404-7ff3-9b98-67e376e48b75` (agent "codex")

Each hook from one agent was seen as replacing the other's session, causing a ping-pong of resets. This manifested as:
1. Manual renames being lost (fixed separately by preserving `manually_renamed`)
2. Agent indicator flickering between Claude and Codex icons
3. Activity state bouncing between the two agents' states

## Impact

- Session names revert to auto-generated names after manual rename
- Agent type indicator is wrong half the time
- Activity state is unreliable when two agents coexist in a pane

## Affected Panes (from current debug.log)

| Pane | Replacements | Pattern |
|------|-------------|---------|
| 23 | 49 | Most severe |
| 25 | 39 | |
| 27 | 16 | The openshell-Gordon case |
| 5 | 17 | |
| 10 | 13 | |
| 7 | 10 | |

## Proposed Fix Approaches

### A: Agent-aware session tracking (Recommended)

Track sessions by `(pane_id, agent_name)` tuple instead of just `pane_id`. Each agent in a pane gets its own session entry. The sidebar shows the "primary" agent (the one most recently interacted with by the user).

### B: Suppress rapid session replacement

Add a debounce: if a "replacement" happens within N seconds of the previous one, and the session IDs are just alternating (A->B->A->B), suppress it and keep the most recent user-facing state.

### C: Ignore session_id from child agents

When a hook arrives with a different `agent` name than the stored one, don't treat the session_id change as a replacement. Only detect replacement when the same agent type has a different session_id.

## Recommendation

Approach C is the simplest and most targeted fix. The session replacement detection should compare `(session_id, agent_name)` pairs, not just `session_id`. A new agent type in the same pane is not a replacement; it's a coexisting process.
