# Feature Specification: cc-deck

**Feature Branch**: `001-cc-deck`
**Created**: 2026-03-02
**Status**: Draft
**Input**: Zellij plugin for managing multiple Claude Code sessions with auto-naming, fuzzy picker, activity status detection, and session grouping

## Purpose

cc-deck is a Zellij plugin that provides a dedicated management layer for multiple concurrent Claude Code sessions. It solves the problem of identifying, tracking, and switching between Claude Code sessions by providing auto-naming, activity status detection, project-based grouping, and a fuzzy picker for instant session switching.

Target user: a developer running 3-5 Claude Code sessions simultaneously across mixed projects, using a modern terminal emulator with Zellij as the multiplexer.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch and Switch Between Sessions (Priority: P1)

A developer starts cc-deck in Zellij and creates three Claude Code sessions in different project directories. They work in one session, then press Ctrl-T to open the fuzzy picker, type a few characters of another project name, and instantly switch to that session.

**Why this priority**: This is the core value proposition. Without fast session creation and switching, cc-deck has no reason to exist.

**Independent Test**: Can be fully tested by launching cc-deck, creating 2+ sessions, and verifying the fuzzy picker filters and switches correctly.

**Acceptance Scenarios**:

1. **Given** cc-deck is running with no sessions, **When** the user presses the "new session" keybinding, **Then** a directory picker appears and a new Claude Code session starts in the selected directory
2. **Given** 3 sessions exist, **When** the user presses Ctrl-T and types "api", **Then** only sessions whose names contain "api" appear in the filtered list
3. **Given** the fuzzy picker is open with a session highlighted, **When** the user presses Enter, **Then** focus switches to that session and the picker closes
4. **Given** the fuzzy picker is open, **When** the user presses Escape, **Then** the picker closes without switching sessions

---

### User Story 2 - Auto-Named Sessions (Priority: P1)

A developer creates a session in a git repository directory. cc-deck automatically detects the repository name and uses it as the session name. The developer can also manually rename any session.

**Why this priority**: Auto-naming removes the mental overhead of identifying sessions and is fundamental to the switching experience.

**Independent Test**: Can be tested by creating a session in a git repo and verifying the name matches the repo name.

**Acceptance Scenarios**:

1. **Given** a session is created in a directory containing a git repository, **When** the session starts, **Then** the session name is set to the git repository name
2. **Given** a session is created in a directory without a git repository, **When** the session starts, **Then** the session name is set to the directory basename
3. **Given** a session exists, **When** the user presses the rename keybinding, **Then** a text input appears and the session name updates to the entered value
4. **Given** two sessions are in the same git repository, **When** both are active, **Then** the second session gets a numeric suffix (e.g., "api-server-2")

---

### User Story 3 - Activity Status Awareness (Priority: P2)

While working in one session, a developer glances at the status bar and sees that another session is marked "waiting," meaning Claude needs user input there. They switch to that session to unblock Claude.

**Why this priority**: Status awareness prevents sessions from stalling unnoticed, directly improving productivity across concurrent sessions.

**Independent Test**: Can be tested by setting up Claude Code hooks, starting a session, and verifying status transitions appear correctly in the status bar.

**Acceptance Scenarios**:

1. **Given** a session where Claude is generating output, **When** the status bar renders, **Then** the session shows a "working" indicator
2. **Given** a session where Claude is waiting for user input, **When** the status bar renders, **Then** the session shows a "waiting" indicator
3. **Given** a session with no activity for 5 minutes, **When** the status bar renders, **Then** the session shows an "idle" indicator with elapsed time
4. **Given** Claude Code hooks are not configured, **When** a session is running, **Then** status detection falls back to basic idle/active detection using pane title changes and timers

---

### User Story 4 - Session Grouping by Project (Priority: P2)

A developer has 5 sessions open: 2 for "api-server" and 3 for "frontend." The status bar groups them by project with distinct colors, making it easy to visually distinguish which sessions belong to which project.

**Why this priority**: Visual grouping reduces cognitive load when managing many sessions, but is not strictly required for basic session management.

**Independent Test**: Can be tested by creating sessions in 2+ different repos and verifying distinct color coding in the status bar and picker.

**Acceptance Scenarios**:

1. **Given** sessions exist in 3 different git repositories, **When** the status bar renders, **Then** each group of sessions has a distinct color
2. **Given** the fuzzy picker is open, **When** sessions are displayed, **Then** sessions are visually grouped by project color
3. **Given** a new session is created in an existing project, **When** it appears, **Then** it inherits the color of that project's group

---

### User Story 5 - Recent Sessions (Priority: P3)

A developer closes cc-deck and reopens it the next day. When creating a new session, the directory picker shows recently used directories at the top, making it quick to re-launch familiar sessions.

**Why this priority**: Convenience feature that improves the day-to-day experience but is not critical for core functionality.

**Independent Test**: Can be tested by creating sessions, restarting cc-deck, and verifying recent directories appear in the new-session picker.

**Acceptance Scenarios**:

1. **Given** the user previously created sessions in directories A, B, and C, **When** they open the new-session picker after a restart, **Then** directories A, B, C appear as suggestions (most recently used first)
2. **Given** 25 directories have been used, **When** the recent list is shown, **Then** only the 20 most recent are displayed (LRU eviction)

---

### Edge Cases

- **Rapid picker toggling**: Opening and closing the fuzzy picker rapidly (multiple Ctrl-T presses in quick succession) does not corrupt state or leave orphan floating panes
- **Terminal resize**: Status bar and fuzzy picker adapt correctly when the terminal window is resized
- **Many sessions**: The fuzzy picker remains scrollable and responsive with 10+ sessions; the status bar truncates gracefully with an overflow indicator
- **Claude process crash**: If a Claude session process exits with a non-zero code, the pane remains visible with the exit code and the status bar marks it as "exited"
- **Missing claude binary**: If the `claude` command is not found when creating a session, an error message is displayed and no pane is created
- **Corrupted recent sessions file**: If the persistence file is unreadable, cc-deck starts with an empty recent list (no crash)
- **No git repo**: Sessions in non-git directories use the directory basename; functionality is not degraded

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: cc-deck MUST run as a Zellij WASM plugin, loaded via Zellij's plugin configuration
- **FR-002**: cc-deck MUST spawn Claude Code sessions as Zellij command panes, each running `claude` in a user-specified working directory
- **FR-003**: cc-deck MUST provide a fuzzy picker overlay activated by a configurable keybinding (default: Ctrl-T) rendered as a floating pane
- **FR-004**: The fuzzy picker MUST support incremental text search, filtering the session list on every keystroke
- **FR-005**: The fuzzy picker MUST display session name, activity status indicator, and project group color for each session
- **FR-006**: cc-deck MUST auto-detect session names from the git repository name in the session's working directory
- **FR-007**: If no git repository is detected, cc-deck MUST fall back to using the directory basename as the session name
- **FR-008**: Users MUST be able to manually rename any session via a keybinding (default: prefix + r)
- **FR-009**: cc-deck MUST detect three activity states per session: working (output being generated), waiting (user input needed), and idle (no activity for a configurable timeout, default 5 minutes)
- **FR-010**: Activity detection MUST use Claude Code hooks (pipe messages) as the primary mechanism, falling back to pane title monitoring and timer-based idle detection when hooks are not configured
- **FR-011**: cc-deck MUST group sessions by project (git repo name or directory) and assign each group a distinct color from a configurable palette
- **FR-012**: cc-deck MUST display a persistent status bar showing all sessions as compact tabs with group color, name, and status indicator
- **FR-013**: cc-deck MUST highlight the currently focused session in the status bar
- **FR-014**: All keybindings MUST be configurable via plugin configuration, with sensible defaults using Ctrl+Shift modifiers (Ctrl+Shift+T for picker, Ctrl+Shift+N for new session, Ctrl+Shift+R for rename, Ctrl+Shift+X for close). No prefix key model; all actions are direct single-keystroke bindings registered via Zellij's `reconfigure` API.
- **FR-015**: cc-deck MUST support direct session switching via Ctrl+Shift+1-9 number keys
- **FR-016**: cc-deck MUST persist recently used session directories and names across restarts, stored as structured data accessible via the WASI filesystem
- **FR-017**: The recent sessions list MUST be capped at 20 entries with least-recently-used eviction
- **FR-018**: When a session's Claude process exits, cc-deck MUST keep the pane visible with exit status and mark the session as "exited" in the status bar
- **FR-019**: The fuzzy picker MUST order sessions with most recently used first

### Key Entities

- **Session**: Represents a running Claude Code instance. Attributes: unique ID, display name, working directory, activity status, project group, Zellij pane ID, creation timestamp, last activity timestamp
- **Project Group**: Logical grouping of sessions sharing the same git repository or directory. Attributes: name, assigned color
- **Recent Entry**: A previously used session configuration. Attributes: directory path, last-used name, last-used timestamp

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can create 5 Claude Code sessions and switch between any two in under 2 seconds using the fuzzy picker
- **SC-002**: Sessions are correctly auto-named from git repository names without manual configuration in 100% of cases where a git repository exists
- **SC-003**: The fuzzy picker opens within 100ms of the activation keybinding
- **SC-004**: Activity status transitions (working, waiting, idle) are reflected in the status bar within 2 seconds of the underlying state change
- **SC-005**: The plugin loads in under 500ms and does not add perceptible delay to Zellij startup
- **SC-006**: cc-deck handles 10+ concurrent sessions without UI degradation or increased response times
- **SC-007**: Recently used directories persist across cc-deck restarts and appear as suggestions when creating new sessions

## Error Handling

- If `claude` binary is not found when creating a session, cc-deck MUST display an error message in the status bar and not create the pane
- If a Claude Code session process exits with a non-zero code, cc-deck MUST keep the pane visible with the exit code displayed and mark the session as "exited" in the status bar
- If Claude Code hooks are not configured (no pipe messages received), cc-deck MUST fall back gracefully to pane title monitoring and timer-based idle detection without user intervention
- If the recent sessions persistence file is corrupted or unreadable, cc-deck MUST start with an empty recent list without crashing
- If the fuzzy picker is toggled rapidly (multiple Ctrl-T presses), cc-deck MUST not corrupt internal state or leave orphan floating panes

## Dependencies

- Zellij 0.42.0+ (required for floating pane pinning and stable plugin API)
- Rust toolchain with wasm32-wasip1 target (build-time only)
- Claude Code hooks (optional, recommended for smart status detection; plugin works without them with degraded idle-only detection)

## Assumptions

- Users have Zellij installed and are comfortable running it inside their preferred terminal emulator (Ghostty, Kitty, Alacritty, etc.)
- The `claude` binary is available on the system PATH
- Users typically run 3-10 Claude Code sessions simultaneously
- Most session working directories contain a git repository
- The Zellij plugin WASI sandbox allows writing to a configuration directory for persistence

## Out of Scope

- Split-pane layouts within cc-deck (Zellij handles this natively)
- Session sharing or collaboration features
- Claude Code output parsing or summarization
- Integration with specific IDEs or editors
- Managing non-Claude-Code terminal sessions (use Zellij's built-in tab/pane management)
- Auto-updating or plugin version management
- **Container/remote backends (e.g., paude integration)**: Future consideration for v2. The session model should include an extensible backend field to support container-based autonomous sessions later, but v1 covers local interactive sessions only.

## Open Questions

- The exact Claude Code hook event names and pipe message format for status detection need to be validated against Claude Code's current hook API. The zellij-attention plugin's approach is a reference implementation.
- The WASI filesystem path for persisting `recent.json` needs testing against Zellij's sandbox restrictions.
- The default prefix key (Ctrl-B) needs verification that it does not conflict with Claude Code's own keybindings.
