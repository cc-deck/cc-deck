# Feature Specification: Sidebar Session Isolation

**Feature Branch**: `044-sidebar-session-isolation`
**Created**: 2026-04-25
**Status**: Draft
**Input**: Scope all plugin state by Zellij PID so each sidebar instance only sees sessions from its own Zellij session.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Two local workspaces show independent sidebars (Priority: P1)

A user runs two local cc-deck workspaces simultaneously, each in its own Zellij session. Each sidebar shows only the Claude Code sessions that belong to its own Zellij session. No cross-pollination between sessions.

**Why this priority**: This is the core problem. Without isolation, sidebars display incorrect data, making the session list unreliable when running multiple workspaces.

**Independent Test**: Open two local workspaces side by side. Start a Claude Code session in each. Verify that each sidebar shows exactly one session (its own), not two.

**Acceptance Scenarios**:

1. **Given** two local workspaces running in separate Zellij sessions, **When** a Claude Code session starts in workspace A, **Then** workspace A's sidebar shows the new session and workspace B's sidebar does not.
2. **Given** two workspaces each with active Claude sessions, **When** a session finishes in workspace A, **Then** workspace A's sidebar reflects the change and workspace B's sidebar is unaffected.
3. **Given** a workspace with three Claude sessions across multiple tabs, **When** the user checks the sidebar, **Then** all three sessions appear (they share the same Zellij PID and belong to the same session).

---

### User Story 2 - Detach and reattach preserves sidebar state (Priority: P1)

A user detaches from a Zellij session (Ctrl+O D) and later reattaches. The sidebar restores the same sessions it had before detachment.

**Why this priority**: Equally critical to isolation. If reattach loses sidebar state, the feature is broken.

**Independent Test**: Attach to a workspace with active Claude sessions, detach, verify the session file on disk, reattach, verify the sidebar shows the same sessions.

**Acceptance Scenarios**:

1. **Given** a workspace with active Claude sessions in the sidebar, **When** the user detaches and reattaches, **Then** the sidebar shows the same sessions as before detachment.
2. **Given** a workspace with active Claude sessions, **When** another workspace is running simultaneously and the user detaches and reattaches the first workspace, **Then** the first workspace's sidebar shows only its own sessions (not sessions from the other workspace).

---

### User Story 3 - Orphaned state files are cleaned up (Priority: P2)

When a Zellij session is fully killed (not just detached), its state files should be cleaned up so they do not accumulate over time.

**Why this priority**: Important for system hygiene but does not affect core functionality. Users will not notice stale files unless disk space becomes a concern.

**Independent Test**: Create a workspace, note the PID-scoped state file, kill the Zellij session, start a new workspace, verify the old state file is removed.

**Acceptance Scenarios**:

1. **Given** orphaned state files from a killed Zellij session, **When** a new plugin instance starts, **Then** state files belonging to dead PIDs are removed.
2. **Given** state files from a still-running Zellij session, **When** cleanup runs, **Then** those state files are preserved.

---

### Edge Cases

- What happens when two Zellij sessions have the same PID (OS PID reuse)? The probability is extremely low for short-lived processes, and Zellij server processes are long-lived. The risk is acceptable.
- What happens when a session is EXITED and then resurrected? Zellij assigns a new PID, so the old state file becomes orphaned and is cleaned up. The sidebar starts fresh, which matches the behavior where pane IDs also change on resurrection.
- What happens when multiple plugin instances exist within the same Zellij session (multiple tabs)? They share the same PID, so they share state correctly. This is the desired behavior.
- What happens when pipe sync messages arrive from a foreign PID? They are silently ignored by the receiver.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: State file paths MUST include the Zellij PID as a suffix: `/cache/sessions-{pid}.json` and `/cache/session-meta-{pid}.json`.
- **FR-002**: The `broadcast_state` and `broadcast_and_save` functions MUST include the sender's PID in the pipe message name (e.g., `cc-deck:sync:{pid}`).
- **FR-003**: The `handle_sync` function MUST extract the PID from incoming sync message names and ignore messages where the PID does not match the current instance's PID.
- **FR-004**: The `restore_sessions` function MUST read only from the PID-scoped state file path.
- **FR-005**: The `save_sessions` function MUST write only to the PID-scoped state file path.
- **FR-006**: The `write_session_meta` and `apply_session_meta` functions MUST use the PID-scoped meta file path.
- **FR-007**: On plugin startup and periodically (via the existing timer), the plugin MUST scan `/cache/` for state files matching the `sessions-*.json` and `session-meta-*.json` patterns and remove files whose PID corresponds to a process that is no longer running. If process liveness cannot be determined (WASI limitation), files older than 7 days MUST be removed as a fallback.
- **FR-008**: The `request_state` function MUST include the requesting instance's PID in the request message name so that only same-PID instances respond.
- **FR-009**: The `prune_session_meta` function MUST operate on the PID-scoped meta file path.
- **FR-010**: No user-facing configuration MUST be required. Isolation is automatic and transparent.
- **FR-011**: On first startup after upgrade, if the legacy `/cache/sessions.json` (without PID suffix) exists, the plugin MUST migrate it to the PID-scoped path and remove the legacy file. The same applies to `/cache/session-meta.json` and `/cache/zellij_pid`.

### Key Entities

- **Zellij PID**: The process ID of the Zellij server, obtained via `get_plugin_ids().zellij_pid`. Stable across detach/reattach cycles but changes when the session is killed and recreated.
- **Session State File**: `/cache/sessions-{pid}.json` containing the serialized BTreeMap of Claude sessions for a specific Zellij session.
- **Session Meta File**: `/cache/session-meta-{pid}.json` containing display name overrides, pause states, and other user-modified metadata for a specific Zellij session.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Two simultaneously running local workspaces each show only their own Claude sessions in the sidebar, with zero cross-session entries visible.
- **SC-002**: Detaching from and reattaching to a Zellij session restores the sidebar to its pre-detach state within one timer cycle (under 2 seconds).
- **SC-003**: Orphaned state files from killed sessions are removed on the next plugin startup, leaving no stale files after cleanup.
- **SC-004**: Existing single-session workflows (one workspace at a time) continue to work identically with no user-visible changes.

## Assumptions

- The Zellij PID is available via `get_plugin_ids().zellij_pid` in the WASM plugin API and is non-zero for all real Zellij sessions.
- The Zellij PID remains stable for the lifetime of a Zellij server process (survives detach/reattach).
- Process liveness checking from WASI may not be available (`/proc/` may not be mounted). If unavailable, file age (mtime older than 7 days) is used as the fallback cleanup criterion.
- The WASI `/cache/` directory supports listing files (needed for orphan cleanup).
- The `pipe_message_to_plugin` API allows variable message names (not just fixed strings).
