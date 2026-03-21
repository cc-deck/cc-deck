# Feature Specification: Sidebar State Refresh on Reattach

**Feature Branch**: `025-sidebar-state-refresh`
**Created**: 2026-03-21
**Status**: Draft
**Input**: User description: "Refresh the sidebar pane session list after reattaching to a Zellij session"

## Context: Existing Infrastructure

The plugin already persists sessions to `/cache/sessions.json` and restores them on load. It also broadcasts state via the `cc-deck:request` / `cc-deck:sync` protocol and removes dead sessions based on `PaneUpdate` manifests. Despite this, the sidebar shows "No Claude sessions" after reattach.

The root cause is a **startup race condition**: after the plugin restores cached sessions, a `PaneUpdate` event arrives with a manifest that may not yet contain all terminal panes. The `remove_dead_sessions` function removes any cached session whose pane ID is not in the manifest, wiping the restored state before panes finish initializing.

This feature addresses the race condition and adds reconciliation so that cached state accurately reflects live pane state after reattach.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reattach Preserves Session List (Priority: P1)

A user detaches from a Zellij session while multiple Claude Code panes are running. When the user reattaches later, the sidebar shows the previously known sessions. As the pane manifest stabilizes, any panes that no longer exist are removed, while active panes retain their cached state until new hook events update them.

**Why this priority**: This is the core problem. Without this, the sidebar is empty after every reattach, forcing users to guess which panes are active. It affects all environment types (local and container) equally, since the same plugin binary runs everywhere.

**Independent Test**: Can be fully tested by starting two Claude Code panes, detaching, reattaching, and verifying the sidebar populates with both sessions. Delivers the primary value of session continuity across reattach.

**Acceptance Scenarios**:

1. **Given** a session with three active Claude Code panes and the user detaches, **When** the user reattaches to the session, **Then** the sidebar shows cached session entries within one second of the plugin loading.
2. **Given** the sidebar has loaded cached entries and pane manifest events are still arriving, **When** the manifest stabilizes with all panes present, **Then** the sidebar retains the cached entries for all active panes.
3. **Given** a session where one Claude Code pane was closed while detached, **When** the pane manifest stabilizes after reattach, **Then** the stale entry is removed from the sidebar.

---

### User Story 2 - Fresh Start with No Cache (Priority: P2)

A user starts a brand new Zellij session with no prior cache. Claude Code panes are opened and begin working. The sidebar picks up sessions through hook events as it does today and begins persisting them for future reattach scenarios.

**Why this priority**: This is the existing behavior, and the feature must not break it. The cache is simply empty on first run.

**Independent Test**: Can be tested by clearing any cached state, starting a fresh session, opening a Claude Code pane, and verifying the sidebar picks it up via hook events.

**Acceptance Scenarios**:

1. **Given** a fresh session with no cached sidebar state, **When** the plugin loads, **Then** the sidebar shows "No Claude sessions" (existing behavior preserved).
2. **Given** a fresh session where a Claude Code pane fires a hook event, **When** the sidebar receives the event, **Then** the session is added to the sidebar and persisted to cache for future reattaches.

---

### User Story 3 - Stale Session Cleanup After Reattach (Priority: P2)

A Claude Code pane exits while the user is detached. On reattach, the sidebar initially shows the stale session from cache. Once the pane manifest has stabilized and the stale pane ID is confirmed absent, the entry is removed.

**Why this priority**: Important for accuracy. Stale entries are cosmetic and do not block workflows, but they should be cleaned up promptly.

**Independent Test**: Can be tested by starting a Claude Code pane, detaching, killing the pane externally, reattaching, and verifying the stale entry is removed once the manifest stabilizes.

**Acceptance Scenarios**:

1. **Given** a cached session for a pane that was closed while detached, **When** the pane manifest stabilizes and confirms the pane ID is absent, **Then** the stale entry is removed from the sidebar and the cache is updated.
2. **Given** multiple cached sessions where some are stale and some are active, **When** the manifest stabilizes, **Then** only sessions with matching pane IDs remain in the sidebar.

---

### User Story 4 - State Reconciliation After Reattach (Priority: P3)

After reattach, the sidebar shows cached activity states (e.g., "Working") that may no longer be accurate. As new hook events arrive from active panes, the sidebar updates to reflect current state.

**Why this priority**: Cached states are a reasonable approximation until live data arrives. The existing hook event flow handles this naturally, so no new mechanism is needed.

**Independent Test**: Can be tested by starting a Claude Code pane in "Working" state, detaching, waiting for it to finish, reattaching, and verifying the sidebar updates from "Working" to "Done" when the next hook event fires.

**Acceptance Scenarios**:

1. **Given** a cached session showing "Working" for a pane that has since finished, **When** the pane fires its next hook event after reattach, **Then** the sidebar updates the entry to reflect the current state.
2. **Given** a cached session showing "Idle" for a pane that is now waiting for permission, **When** a permission request hook event fires, **Then** the sidebar updates the entry to "Permission" and triggers attention.

---

### Edge Cases

- What happens when the cache file is corrupted or contains invalid data? The plugin discards it and starts with an empty session map (this is already implemented).
- What happens when the user reattaches very quickly (within milliseconds of detaching)? The cached state should be nearly identical to live state, so the transition is seamless.
- What happens when a pane ID in the cache corresponds to a pane that was replaced (closed and a new pane took its ID)? The dead session cleanup removes it once the manifest confirms it is not a terminal pane, and any new Claude Code pane registers through normal hook events with its own session ID.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The plugin MUST NOT remove cached sessions based on pane manifest data during a startup grace period after plugin load, to avoid the race condition where the manifest is still incomplete.
- **FR-002**: After the grace period, the plugin MUST reconcile cached sessions against the pane manifest, removing entries whose pane IDs are no longer present.
- **FR-003**: The existing session persistence (save on every state change, restore on load) MUST continue to function correctly.
- **FR-004**: The existing dead session cleanup via `PaneUpdate` manifests MUST continue to function correctly outside the grace period.
- **FR-005**: The existing multi-instance sync protocol (`cc-deck:request` / `cc-deck:sync`) MUST continue to function correctly.
- **FR-006**: The grace period MUST be short enough that stale entries do not persist visibly longer than a few seconds.

### Key Entities

- **Session Cache**: The existing persistent representation of the session map at `/cache/sessions.json`. Contains session entries keyed by pane ID with their last known state. Already implemented.
- **Startup Grace Period**: A brief window after plugin load during which dead session cleanup is deferred, allowing the pane manifest to stabilize before reconciliation occurs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After reattaching to a session with active Claude Code panes, the sidebar displays cached session entries within one second of the plugin loading.
- **SC-002**: Stale entries for panes that no longer exist are removed within a few seconds of reattach, once the pane manifest has stabilized.
- **SC-003**: Fresh sessions without any cache continue to work identically to existing behavior, with no regressions in session discovery via hook events.
- **SC-004**: The dead session cleanup continues to work correctly during normal operation (not just at startup).

## Assumptions

- The plugin's WASI cache directory is preserved across session detach/reattach cycles (standard Zellij behavior for plugin cache directories).
- Pane IDs are stable within a Zellij session across detach/reattach cycles (the same pane retains the same ID).
- The `PaneUpdate` manifest eventually converges to a complete picture of all panes after reattach, but the first few events may be incomplete.
- The existing hook event flow naturally reconciles cached activity states as Claude Code panes resume sending events after reattach.
