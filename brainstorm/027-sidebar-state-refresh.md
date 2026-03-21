# Brainstorm: Sidebar State Refresh on Reattach

**Date:** 2026-03-21
**Status:** Brainstorm (ready for spec)
**Depends on:** 012-sidebar-plugin, 024-container-env
**Discovered during:** Manual testing of container environment attach/reattach

## Problem

When a user detaches from a Zellij session (Ctrl+o d) and reattaches later, the cc-deck sidebar shows "No Claude sessions" even though Claude Code panes are running and responsive.

This affects both local and container environments. In the container case, it is especially noticeable because the user explicitly runs `cc-deck env attach` expecting a fully functional session.

## Root Cause

The sidebar plugin tracks sessions via hook events from Claude Code's `settings.json` hooks (`cc-deck hook --pane-id ...`). These hooks fire on:
- Notification events
- Permission request events
- Stop events

The plugin maintains an in-memory session map. On reattach, the WASM plugin is reloaded with empty state. The pane map file (`/tmp/cc-deck-pane-map.json`) is ephemeral and may be stale or missing.

No mechanism exists to request a state refresh from all active Claude Code panes on plugin load.

## Desired Behavior

When the sidebar plugin loads (fresh start or reattach), it should:
1. Detect all running Claude Code panes in the current session
2. Request a status update from each one
3. Populate the session list within 1-2 seconds of reattach

## Possible Solutions

### Option A: Plugin requests pane list on load

On `load()` in the WASM plugin:
1. Call `get_pane_ids()` to enumerate all panes in the session
2. For each terminal pane, send a pipe message requesting status
3. Claude Code hooks respond with current state

**Pros**: No external dependency, works immediately on load.
**Cons**: Requires a "status request" pipe message type that Claude Code hooks can respond to. The hook currently only fires on specific events, not on demand.

### Option B: Persistent state in WASI cache

Store the session map in `/cache/sessions.json` (WASI filesystem). On load, read the cached state as initial data, then update as hook events arrive.

**Pros**: Instant UI on reattach, no round-trip needed.
**Cons**: State may be stale (sessions that ended while detached). Needs reconciliation logic.

### Option C: Periodic polling

The plugin periodically (every 5-10 seconds) sends a "heartbeat" pipe message to all known panes. Panes that don't respond are removed from the session list.

**Pros**: Self-healing, catches stale sessions.
**Cons**: Adds overhead. Still needs initial population (combine with Option A or B).

### Option D: Use Zellij tab events

Subscribe to `TabUpdate` and `PaneUpdate` events in the plugin. When a new pane appears or a tab changes, scan for Claude Code panes.

**Pros**: Event-driven, no polling.
**Cons**: Zellij events don't include pane content/process info, so the plugin cannot distinguish Claude Code panes from regular terminal panes without additional signals.

## Recommended Approach

**Option B (persistent cache) + Option A (request on load):**

1. On `load()`: Read `/cache/sessions.json` for immediate UI
2. On `load()`: Send a "status request" pipe message to all panes
3. Claude Code hooks respond with current state, updating the session map
4. Save updated map to `/cache/sessions.json` on every change

This gives instant (possibly stale) UI on reattach, followed by accurate data within 1-2 seconds.

## Scope

- Plugin Rust code (session state persistence, load-time refresh)
- Hook Go code (respond to "status request" pipe messages)
- Affects: local, container, compose, K8s environments (all use the same sidebar)

## Effort Estimate

Small to medium. The persistent cache is straightforward (JSON read/write to WASI cache). The status request pipe message needs a new message type in both the plugin and the hook.
