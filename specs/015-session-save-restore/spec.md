# Feature Specification: Session Save and Restore

**Feature Branch**: `015-session-save-restore`
**Created**: 2026-03-10
**Status**: Draft
**Input**: User description: "Session save and restore for cc-deck"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Save and Restore Before Plugin Upgrade (Priority: P1)

A user needs to restart Zellij to pick up a new cc-deck plugin build. They have 6 tabs open, each with a Claude Code session in a different project directory. They want to save the entire workspace, restart Zellij, and restore everything to the same state with minimal effort.

**Why this priority**: This is the core use case that motivated the feature. Without it, users must manually recreate every tab and restart every Claude session after any Zellij restart.

**Independent Test**: Can be fully tested by saving state with `cc-deck session save`, restarting Zellij, and running `cc-deck session restore`. Delivers immediate value by reducing multi-tab recovery from minutes of manual work to a single command.

**Acceptance Scenarios**:

1. **Given** a Zellij session with multiple tabs and Claude sessions, **When** the user runs `cc-deck session save`, **Then** a state file is written containing all tab names, working directories, session IDs, display names, and pause flags in tab order.
2. **Given** a saved state file and a freshly started Zellij session, **When** the user runs `cc-deck session restore`, **Then** new tabs are created for each saved session with the correct working directory, and Claude is started with `--resume` using the saved session ID.
3. **Given** a restore in progress, **When** a Claude session ID is no longer valid, **Then** Claude is started fresh (without `--resume`) in that tab and the user sees a note about it.
4. **Given** a restore in progress, **When** tabs are being created, **Then** progress output shows `Creating tab 1/N: <display_name>...` for each tab.

---

### User Story 2 - Auto-save as Safety Net (Priority: P2)

A user's Zellij crashes unexpectedly, or they restart without remembering to save first. The system should have an auto-saved snapshot they can restore from.

**Why this priority**: Explicit save covers the planned restart case, but crashes and forgotten saves are common. Auto-save provides a safety net without any user action.

**Independent Test**: Can be tested by starting Claude sessions, waiting for hook events to trigger auto-save, verifying auto-save files exist on disk, then restoring from them.

**Acceptance Scenarios**:

1. **Given** an active cc-deck session with hook events firing, **When** a hook event is processed, **Then** the current session state is auto-saved to disk as a rolling snapshot.
2. **Given** multiple auto-saves have accumulated, **When** the retention limit is reached (default: 5), **Then** the oldest auto-save is deleted to stay within the limit.
3. **Given** both auto-saves and named saves exist, **When** the user runs `cc-deck session restore` without arguments, **Then** the most recent snapshot (auto or named, whichever is newest) is used.

---

### User Story 3 - Named Saves for Multiple Configurations (Priority: P2)

A user works on different project sets (e.g., "ai-projects" vs "infra-work") and wants to save and switch between named workspace configurations.

**Why this priority**: Extends the save/restore concept to workspace management. Builds on the same mechanism as P1 but adds naming and listing for discoverability.

**Independent Test**: Can be tested by saving with `cc-deck session save ai-projects`, listing with `cc-deck session list`, and restoring with `cc-deck session restore ai-projects`.

**Acceptance Scenarios**:

1. **Given** an active session, **When** the user runs `cc-deck session save my-setup`, **Then** state is saved with the name "my-setup" and persists indefinitely (not subject to auto-save rotation).
2. **Given** saved snapshots exist, **When** the user runs `cc-deck session list`, **Then** all snapshots are listed with name, timestamp, and session count.
3. **Given** a named snapshot exists, **When** the user runs `cc-deck session restore my-setup`, **Then** that specific snapshot is restored.

---

### User Story 4 - Snapshot Cleanup (Priority: P3)

A user wants to remove old or unwanted saved snapshots to keep the list clean.

**Why this priority**: Housekeeping feature. Less critical than save/restore itself but needed to prevent unbounded growth of named saves.

**Independent Test**: Can be tested by creating named saves, removing individual ones with `cc-deck session remove <name>`, and clearing all with `cc-deck session remove --all`.

**Acceptance Scenarios**:

1. **Given** a named snapshot "old-setup" exists, **When** the user runs `cc-deck session remove old-setup`, **Then** that snapshot is deleted and no longer appears in `list`.
2. **Given** multiple snapshots exist, **When** the user runs `cc-deck session remove --all`, **Then** all snapshots (both auto and named) are deleted.
3. **Given** no snapshot with the requested name exists, **When** the user runs `cc-deck session remove nonexistent`, **Then** an error message is shown listing available snapshots.

---

### Edge Cases

- What happens when the saved working directory no longer exists on disk? The tab is created but the shell shows a directory error; Claude starts in the default directory.
- What happens when restore is run while existing tabs have active sessions? Fresh tabs are always created alongside existing ones; no existing tabs are modified or closed.
- What happens when the state file is corrupt or has an unsupported version? An error message is shown and no tabs are created.
- What happens during save when no sessions are tracked (empty sidebar)? An empty state file is saved (zero sessions), and restore creates no tabs.
- What happens when two auto-saves fire nearly simultaneously? File writes use atomic rename to prevent corruption.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `cc-deck session save [name]` command that captures current session state to a persistent file on the host filesystem. When no name is provided, a timestamp-based name MUST be generated (e.g., `save-2026-03-10T14-30-00`). If a snapshot with the same name already exists, it MUST be silently overwritten.
- **FR-002**: System MUST provide a `cc-deck session restore [name]` command that recreates tabs and starts Claude sessions from a saved state file, running inside an active Zellij session.
- **FR-003**: System MUST provide a `cc-deck session list` command that displays all saved snapshots with name, timestamp, and session count.
- **FR-004**: System MUST provide a `cc-deck session remove <name>` command that deletes a specific snapshot, and `cc-deck session remove --all` to delete all snapshots.
- **FR-005**: The save command MUST query the cc-deck plugin for current state via `zellij pipe` and write the response to a state file.
- **FR-006**: The restore command MUST create new tabs in tab order, set the working directory, and start Claude with `--resume <session_id>`, falling back to a fresh Claude start if resume fails.
- **FR-007**: The restore command MUST show progress output for each tab being created (e.g., `Creating tab 1/5: cc-deck...`).
- **FR-008**: The hook command (`cc-deck hook`) MUST auto-save session state as a side-effect, maintaining a rolling set of the N most recent auto-saves (default: 5). Auto-save MUST skip if the last auto-save was less than 5 minutes ago (cooldown to avoid excessive disk writes).
- **FR-009**: State files MUST be stored in the XDG-conformant config directory (`$XDG_CONFIG_HOME/cc-deck/sessions/` or `~/.config/cc-deck/sessions/`).
- **FR-010**: `cc-deck session restore` without arguments MUST select the most recent snapshot (auto or named, whichever is newest by timestamp).
- **FR-011**: Named saves MUST persist indefinitely and not be subject to auto-save rotation.
- **FR-012**: State file writes MUST use atomic operations (write to temp file, then rename) to prevent corruption from concurrent access or crashes.

### Key Entities

- **Snapshot**: A point-in-time capture of all tracked sessions, including tab name, working directory, Claude session ID, display name, pause state, and git branch. Identified by a name (user-provided or auto-generated).
- **Session Entry**: One Claude Code session within a snapshot, representing a single tab with its working directory and session metadata.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can save and restore a 6-tab workspace in under 30 seconds total (save + restart + restore), compared to several minutes of manual recreation.
- **SC-002**: Auto-save captures state at most every 5 minutes, ensuring no more than 5 minutes of state loss in an unexpected crash.
- **SC-003**: Restore successfully resumes at least 90% of Claude sessions that were active within the last hour (measured by `--resume` success rate).
- **SC-004**: Users can manage multiple named workspace configurations and switch between them with a single command.

## Clarifications

### Session 2026-03-10

- Q: Should auto-save have a cooldown to avoid excessive disk writes from frequent hook events? → A: Yes, skip auto-save if the last one was less than 5 minutes ago.
- Q: What happens when saving to a name that already exists? → A: Silently overwrite the existing snapshot.

## Assumptions

- The `cc-deck hook` command already runs on every Claude event and can be extended with auto-save logic without significant performance impact.
- Claude Code's `--resume <session_id>` works reliably for recent sessions (within the last hour). Older sessions may fail to resume.
- Zellij's `new-tab` action creates tabs using the configured `new_tab_template`, which includes the cc-deck sidebar plugin.
- The XDG config directory is writable and available on all target platforms (macOS, Linux).
- Tab creation and Claude startup can be sequenced with brief pauses to allow each tab's plugin instance to initialize.
