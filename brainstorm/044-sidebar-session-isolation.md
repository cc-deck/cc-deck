# Brainstorm: Sidebar Session Isolation

**Date:** 2026-04-25
**Status:** active

## Problem Framing

When multiple local cc-deck workspaces run simultaneously (each in its own Zellij session), the sidebar plugins cross-pollinate: session A shows session B's Claude sessions, and vice versa. This happens because:

1. `pipe_message_to_plugin` broadcasts to ALL plugin instances across ALL Zellij sessions in the same process.
2. `/cache/sessions.json` is a shared WASI filesystem path that all plugin instances read/write.
3. The existing PID guard only protects against stale data from a *different* Zellij process, not across sessions within the same process.

Each sidebar should only show sessions from its own Zellij session. This should be enforced by design, not by convention.

## Approaches Considered

### A: PID-scoped state files
- Scope all state files by Zellij PID: `/cache/sessions-{pid}.json`, `/cache/session-meta-{pid}.json`
- Filter `pipe_message_to_plugin` sync by including PID in message names
- Pros: Simple, uses existing infrastructure, PID already available via `get_plugin_ids().zellij_pid`
- Cons: PID changes on session restart (but this matches pane ID behavior, so not a regression)

### B: Session-name-scoped state files
- Use Zellij session name (e.g., `cc-deck-local-voice`) instead of PID
- Pros: Survives session restart
- Cons: Session name not directly available in plugin API, would need to be passed via config or detected

### C: Grouped visibility
- Keep shared state but visually separate sessions by source
- Pros: Cross-session awareness
- Cons: More complex UI, doesn't solve the fundamental isolation problem

## Decision

Approach A: PID-scoped state files. The Zellij PID is stable for detach/reattach cycles (the server process stays alive), and the behavior when PIDs change on full restart matches the existing behavior where pane IDs also change. The implementation is straightforward and requires no new APIs.

## Key Requirements

- State file paths include Zellij PID: `/cache/sessions-{pid}.json`, `/cache/session-meta-{pid}.json`
- Pipe sync messages include PID in the message name for filtering
- Startup cleanup removes orphaned state files from dead PIDs
- No user-visible configuration needed

## Open Threads

- How should orphaned state file cleanup handle PID reuse by the OS? (Low risk since Zellij PIDs are typically large and rarely reused quickly)
