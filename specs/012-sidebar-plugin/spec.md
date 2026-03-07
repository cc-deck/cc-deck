# Feature Specification: cc-deck Sidebar Plugin

**Feature Branch**: `012-sidebar-plugin`
**Created**: 2026-03-07
**Status**: Draft
**Input**: cc-deck v2 sidebar plugin for Claude Code session management in Zellij, based on brainstorm/08-cc-deck-v2-redesign.md

## Purpose

cc-deck provides a sidebar-based management layer for concurrent Claude Code sessions within the Zellij terminal multiplexer. A vertical session list appears on every tab, showing real-time activity status for each Claude instance. Users can see at a glance which Claude sessions are working, waiting for input, or idle, and switch between them with a click or keyboard shortcut. A companion "attend" action jumps directly to the next session needing human input, minimizing Claude wait time across multiple parallel sessions.

Target user: a developer running 2-10 Claude Code sessions simultaneously across one or more projects, using Zellij as their terminal multiplexer.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See Claude Activity at a Glance (Priority: P1)

A developer has four Claude Code sessions running in separate tabs. While working in one session, they glance at the sidebar and see that session "add-tests" shows a waiting indicator, meaning Claude needs permission approval. They click the entry in the sidebar to switch to that tab and approve the action.

**Why this priority**: Without visibility into what each Claude session is doing, the user has to manually check every tab. This is the core value proposition.

**Independent Test**: Can be fully tested by installing the plugin, opening multiple tabs with Claude Code running, triggering hook events, and verifying the sidebar renders activity indicators correctly and click-to-switch works.

**Acceptance Scenarios**:

1. **Given** the plugin is loaded in a Zellij layout, **When** a tab contains a running Claude Code instance, **Then** the sidebar shows an entry with the session name and current activity indicator
2. **Given** Claude Code sends a "PermissionRequest" hook event, **When** the sidebar renders, **Then** the corresponding session entry shows a waiting indicator (visually distinct, attention-grabbing)
3. **Given** Claude Code sends a "PreToolUse" hook event, **When** the sidebar renders, **Then** the corresponding session entry shows a working indicator
4. **Given** Claude Code sends a "Stop" hook event, **When** the sidebar renders, **Then** the corresponding session entry shows a done indicator
5. **Given** a session entry is clicked in the sidebar, **When** the click is processed, **Then** focus switches to the tab containing that Claude session
6. **Given** the currently active tab has a Claude session, **When** the sidebar renders, **Then** that session entry is visually highlighted (distinct from other entries)
7. **Given** the sidebar is displayed on tab A, **When** the user switches to tab B, **Then** the sidebar on tab B shows the same session list with the same activity states (state is synchronized across instances)

---

### User Story 2 - Install and Configure cc-deck (Priority: P1)

A developer installs cc-deck using a single CLI command. The command places the plugin binary, installs a Zellij layout, and registers Claude Code hooks in settings.json (with a backup of the original). After installation, starting Zellij with the cc-deck layout shows the sidebar.

**Why this priority**: Without installation, nothing else works. It must be simple and safe (especially the settings.json modification).

**Independent Test**: Can be tested by running `cc-deck install` on a clean system and verifying all files are placed correctly, settings.json is backed up, and `zellij --layout cc-deck` shows the sidebar.

**Acceptance Scenarios**:

1. **Given** cc-deck CLI is available, **When** the user runs `cc-deck install`, **Then** the WASM plugin binary is copied to the Zellij plugins directory
2. **Given** cc-deck CLI is available, **When** the user runs `cc-deck install`, **Then** a cc-deck layout file is placed in the Zellij layouts directory
3. **Given** a `~/.claude/settings.json` file exists, **When** the user runs `cc-deck install`, **Then** a timestamped backup is created before any modification (e.g., `settings.json.bak.20260307-143012`)
4. **Given** `cc-deck install` has completed, **When** the user runs `zellij --layout cc-deck`, **Then** every tab includes the cc-deck sidebar pane
5. **Given** `cc-deck install` has completed, **When** Claude Code starts in a Zellij pane, **Then** hook events are received by the sidebar plugin via the registered hook command
6. **Given** no `~/.claude/settings.json` exists, **When** the user runs `cc-deck install`, **Then** a new settings.json is created with only the hook registration
7. **Given** cc-deck is already installed, **When** the user runs `cc-deck install` again, **Then** the installation is idempotent (updates in place, creates a new backup)

---

### User Story 3 - Hook Integration (Priority: P1)

Claude Code fires hook events (SessionStart, PreToolUse, PermissionRequest, Stop, etc.) during operation. The `cc-deck hook` command receives these events, extracts relevant information, and forwards them to the Zellij plugin via pipe messages. If Zellij is not running, the hook command exits silently without error.

**Why this priority**: The hook is the bridge between Claude Code and the plugin. Without it, the sidebar has no activity data.

**Independent Test**: Can be tested by running `cc-deck hook` with sample hook JSON on stdin and verifying the correct `zellij pipe` command is invoked (or silently exits if Zellij is not available).

**Acceptance Scenarios**:

1. **Given** Claude Code fires a hook event, **When** `cc-deck hook` receives the JSON on stdin, **Then** it sends a pipe message to the cc-deck plugin with event type, pane ID, and session metadata
2. **Given** Zellij is not running, **When** `cc-deck hook` is invoked, **Then** it exits with code 0 and produces no output
3. **Given** malformed JSON is provided on stdin, **When** `cc-deck hook` processes it, **Then** it exits with code 0 and produces no output (never disrupts Claude Code)
4. **Given** a PermissionRequest hook event, **When** `cc-deck hook` forwards it, **Then** the pipe message includes the pane ID so the plugin can identify which session needs attention
5. **Given** a SessionEnd hook event, **When** `cc-deck hook` forwards it, **Then** the plugin removes the session from its state

---

### User Story 4 - Attend: Jump to Waiting Session (Priority: P2)

A developer presses a keyboard shortcut (the "attend" key). If any Claude session in the current Zellij session is waiting for input, focus jumps to that tab. If no local session is waiting, a brief notification appears indicating the state.

**Why this priority**: This is the key workflow optimization, but requires the sidebar (P1) to be functional first.

**Independent Test**: Can be tested by setting up two sessions (one working, one waiting), pressing the attend key, and verifying focus switches to the waiting session's tab.

**Acceptance Scenarios**:

1. **Given** one session is in "waiting" state, **When** the user presses the attend key, **Then** focus switches to the tab containing the waiting session
2. **Given** multiple sessions are in "waiting" state, **When** the user presses the attend key, **Then** focus switches to the session that has been waiting the longest
3. **Given** no sessions are in "waiting" state in the current deck, **When** the user presses the attend key, **Then** a notification appears briefly indicating no sessions need attention (or showing which other deck has waiting sessions)
4. **Given** the attend key is pressed while already on the waiting session's tab, **When** another session starts waiting, **Then** the next attend keypress jumps to the newly waiting session

---

### User Story 5 - Session Rename (Priority: P2)

A developer wants to give a Claude session a meaningful name. They trigger a rename action (via keybinding or sidebar interaction), type the new name, and the sidebar and tab title both update to reflect the new name.

**Why this priority**: Auto-naming from git repos covers most cases, but manual rename is needed for disambiguation and clarity.

**Independent Test**: Can be tested by triggering rename on a session, entering a new name, and verifying both the sidebar entry and the Zellij tab title are updated.

**Acceptance Scenarios**:

1. **Given** a session exists in the sidebar, **When** the user triggers the rename action, **Then** an inline text input appears on the session entry
2. **Given** the rename input is active, **When** the user types a name and presses Enter, **Then** the session display name and the Zellij tab title both update
3. **Given** the rename input is active, **When** the user presses Escape, **Then** the rename is cancelled and the original name is preserved
4. **Given** two sessions exist, **When** the user renames one to the same name as the other, **Then** the system appends a numeric suffix (e.g., "api-server-2")

---

### User Story 6 - Session Creation (Priority: P2)

A developer wants to start a new Claude Code session. They press a keybinding or click a "new session" button in the sidebar. A new tab opens with Claude Code running in the current deck's working directory. The session is auto-named based on the git repository or directory name.

**Why this priority**: Creating sessions within the cc-deck workflow is more convenient than manually creating tabs, but manual tab creation still works.

**Independent Test**: Can be tested by triggering the new session action and verifying a new tab appears with Claude Code running, and the session appears in the sidebar with an auto-detected name.

**Acceptance Scenarios**:

1. **Given** the user triggers the new session action, **When** the action completes, **Then** a new tab opens with Claude Code running
2. **Given** the new tab is in a git repository, **When** the session starts, **Then** the session name is set to the repository name
3. **Given** the new tab is not in a git repository, **When** the session starts, **Then** the session name is set to the directory basename
4. **Given** a session with the same auto-detected name already exists, **When** a new session is created, **Then** a numeric suffix is appended (e.g., "api-server-2")

---

### User Story 7 - Uninstall cc-deck (Priority: P2)

A developer wants to remove cc-deck. They run `cc-deck uninstall`, which safely removes the hook entries from settings.json (after creating a backup), deletes the WASM plugin and layout files, and leaves all other configuration intact.

**Why this priority**: Safe uninstall is important for user trust, but only relevant after the install flow works.

**Independent Test**: Can be tested by running `cc-deck uninstall` after a successful install and verifying settings.json contains no cc-deck hooks, the WASM file is removed, and all other settings.json content is preserved.

**Acceptance Scenarios**:

1. **Given** cc-deck is installed, **When** the user runs `cc-deck uninstall`, **Then** a timestamped backup of settings.json is created before modification
2. **Given** cc-deck is installed, **When** the user runs `cc-deck uninstall`, **Then** only cc-deck hook entries are removed from settings.json (other hooks and settings preserved)
3. **Given** cc-deck is installed, **When** the user runs `cc-deck uninstall`, **Then** the WASM plugin binary and layout files are deleted
4. **Given** cc-deck is not installed, **When** the user runs `cc-deck uninstall`, **Then** a message indicates nothing to remove and no files are modified
5. **Given** the user runs `cc-deck uninstall --skip-backup`, **When** settings.json is modified, **Then** no backup is created

---

### Edge Cases

- What happens when the user closes a tab that has a Claude session? The session disappears from the sidebar.
- What happens when Claude Code crashes mid-session? The hook sends no "Stop" event. The sidebar detects the closed pane via PaneUpdate/PaneClosed events and removes the session.
- What happens when the sidebar pane is too narrow to display session names? Names truncate with an ellipsis.
- What happens when there are more sessions than sidebar rows? The list scrolls, with the active session always visible, and overflow indicators show count of hidden sessions above/below.
- What happens when permissions are requested on plugin load? The permission dialog is displayed within the sidebar pane (which has enough space, unlike a 1-row bar).
- What happens when the user has no Claude sessions? The sidebar shows an empty state with instructions on how to create a session.
- What happens when the hook command is called outside of Zellij? The hook exits silently (code 0).
- What happens when `cc-deck uninstall` is run while settings.json contains hooks from other tools? Only cc-deck entries are removed; other tools' hooks are preserved.
- What happens when the sidebar has no Claude sessions but the user has regular terminal tabs? The sidebar shows only Claude sessions, so it displays the empty state. Regular tabs are visible in the Zellij tab bar, not in the sidebar.

## Requirements *(mandatory)*

### Functional Requirements

**Sidebar Plugin:**
- **FR-001**: The sidebar MUST render a vertical list of only Claude Code sessions (not regular terminal tabs) in the current Zellij session, one entry per session
- **FR-002**: Each session entry MUST display an activity indicator reflecting the current state (working, waiting, idle, done, or tool-specific activity)
- **FR-003**: The sidebar MUST highlight the session entry corresponding to the currently active tab
- **FR-004**: The sidebar MUST be visible on every tab within the Zellij session (via layout tab template)
- **FR-005**: Multiple sidebar instances (one per tab) MUST synchronize their state so all show identical session information
- **FR-006**: Clicking a session entry MUST switch focus to the tab containing that session
- **FR-007**: The sidebar MUST auto-detect Claude Code sessions by tracking panes that receive hook events
- **FR-008**: The sidebar MUST auto-name sessions based on git repository detection, falling back to directory basename
- **FR-009**: The sidebar MUST handle sessions disappearing (tab/pane closed) by removing them from the list
- **FR-010**: The sidebar MUST display elapsed time since the last activity change for sessions in non-idle states
- **FR-011**: The sidebar MUST have a configurable width with a sensible default, adjustable via plugin configuration parameters

**Hook Integration:**
- **FR-012**: The `cc-deck hook` command MUST read Claude Code hook JSON from stdin and forward it as a pipe message to the cc-deck plugin
- **FR-013**: The hook command MUST exit silently (code 0, no output) if Zellij is not running or if input is malformed
- **FR-014**: The hook command MUST handle all Claude Code hook event types: SessionStart, SessionEnd, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStop
- **FR-015**: The hook command MUST include the Zellij pane ID in forwarded messages so the plugin can associate events with sessions

**Installation:**
- **FR-016**: `cc-deck install` MUST copy the WASM plugin binary to the Zellij plugins directory (`~/.config/zellij/plugins/`)
- **FR-017**: `cc-deck install` MUST install a layout file that includes the sidebar in every tab's template
- **FR-018**: `cc-deck install` MUST create a timestamped backup of `~/.claude/settings.json` before modifying it, unless the user provides a `--skip-backup` option
- **FR-019**: `cc-deck install` MUST register hook entries in `~/.claude/settings.json` pointing to the `cc-deck hook` command
- **FR-020**: `cc-deck install` MUST be idempotent (safe to run multiple times)
- **FR-021**: `cc-deck install` MUST NOT modify the user's `config.kdl`

**Uninstall:**
- **FR-022**: `cc-deck uninstall` MUST create a timestamped backup of `~/.claude/settings.json` before modifying it, unless the user provides a `--skip-backup` option
- **FR-023**: `cc-deck uninstall` MUST remove only cc-deck hook entries from `~/.claude/settings.json`, preserving all other content
- **FR-024**: `cc-deck uninstall` MUST remove the WASM plugin binary and layout files installed by cc-deck
- **FR-025**: `cc-deck uninstall` MUST never truncate, zero out, or destroy any user configuration file

**Attend:**
- **FR-026**: The attend action MUST be triggered by a configurable keyboard shortcut with a sensible default
- **FR-027**: The attend action MUST find the session that has been waiting longest and switch focus to its tab
- **FR-028**: If no session is waiting in the current deck, the attend action MUST display a brief inline notification in the sidebar (e.g., a flash message at the top of the session list)

**Session Management:**
- **FR-029**: The rename action MUST update both the cc-deck display name and the Zellij tab title
- **FR-030**: The new session action MUST open a new tab running Claude Code in the current working directory
- **FR-031**: Duplicate session names MUST be resolved by appending numeric suffixes

### Key Entities

- **Session**: Represents a single Claude Code instance. Key attributes: display name, activity state, pane ID, tab index, working directory, git branch, last event timestamp.
- **Activity**: The current state of a session. States: Init, Working (thinking/tool use), Waiting (permission request), Idle, Done, Exited. Tool-specific sub-states during Working (e.g., Bash, Edit, Read).
- **Deck**: A logical grouping of sessions. Maps to a Zellij session. Contains zero or more sessions.
- **HookEvent**: A notification from Claude Code about session activity. Contains: session ID, pane ID, event type, tool name (optional), working directory.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After installation, users can see Claude session activity in the sidebar within 1 second of a hook event firing
- **SC-002**: Users can switch to a waiting Claude session within 2 seconds of noticing it (one click or one keystroke)
- **SC-003**: Installation completes in under 10 seconds and requires no manual file editing
- **SC-004**: The hook command adds no perceptible delay to Claude Code operation (exits in under 100ms)
- **SC-005**: All sidebar instances across tabs show consistent state within 1 second of a state change
- **SC-006**: Zero data loss risk from installation or uninstall: settings.json backup is always created before modification (unless explicitly skipped by the user)
- **SC-007**: The plugin correctly tracks session lifecycle from creation through completion with no phantom entries remaining after sessions close

## Assumptions

- Users have Zellij 0.42.0+ installed and use it as their terminal multiplexer
- Users have the `cc-deck` Go CLI binary on their PATH
- Claude Code supports hook registration via `~/.claude/settings.json`
- The Zellij plugin API (zellij-tile 0.43+) supports all required features: pipe messages, tab/pane events, `run_command()`, mouse events, and `set_selectable()`
- The user's terminal supports true-color (24-bit) ANSI escape sequences for the sidebar rendering
- Only one cc-deck plugin version is installed at a time (no multi-version support needed)
- The plugin binary is distributed as part of the CLI binary (single artifact to install)

## Scope Boundaries

**In scope:**
- Sidebar plugin (vertical session list with activity indicators)
- Hook integration via Go binary
- Installation and uninstall commands with safe settings.json management (backup before modify)
- Attend key (jump to waiting session within current deck)
- Session rename and creation
- Multi-instance state synchronization via pipe messages

**Out of scope (deferred to future specs):**
- Floating fuzzy picker (will be a separate feature building on the sidebar)
- Cross-deck attend (switching between Zellij sessions)
- Sidebar collapse to icon-only mode
- Alternative 1-row horizontal bar mode
- Opinionated multi-session layouts
- Desktop notifications
- Settings menu in sidebar
- Remote/container-based session management
- Deck lifecycle management from CLI (`cc-deck new`, `cc-deck switch`)
- Session persistence across Zellij restarts
