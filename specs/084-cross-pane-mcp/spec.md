# Feature Specification: Cross-Pane Agent Communication via MCP Server

**Feature Branch**: `084-cross-pane-mcp`

**Created**: 2026-07-26

**Status**: Draft

**Input**: User description: "Cross-Pane Agent Communication via MCP Server"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover Active Sessions (Priority: P1)

A user running multiple AI agent sessions in parallel wants to understand what each session is working on without manually switching between panes. An agent in one session can list all other active sessions with their current context: session name, agent type, working directory, git branch, current topic, and recent activity.

**Why this priority**: Session discovery is the foundation for all cross-pane communication. Without knowing what sessions exist and what they're doing, no other interaction is possible.

**Independent Test**: Can be fully tested by running two or more agent sessions, then calling the sessions listing tool from one of them. Delivers immediate value by providing cross-session awareness.

**Acceptance Scenarios**:

1. **Given** two Claude Code sessions are running in separate Zellij panes, **When** an agent calls the sessions listing tool, **Then** it receives a list containing both sessions with their name, agent type, activity state, working directory, and git branch.
2. **Given** a session where the user recently submitted a prompt, **When** another agent lists sessions, **Then** the listing includes the first ~100 characters of the most recent prompt as the session's topic.
3. **Given** a session working in a project with a CLAUDE.md file, **When** another agent lists sessions, **Then** the listing includes a project summary derived from that CLAUDE.md.
4. **Given** three sessions are running (one Claude Code, one Codex, one idle), **When** an agent lists sessions, **Then** each session shows the correct agent type and activity state.

---

### User Story 2 - Read Another Session's Scrollback (Priority: P1)

A user wants one agent to read what another session has been doing by examining its terminal scrollback. This enables state observation without disrupting the target session.

**Why this priority**: Reading scrollback is the simplest form of cross-session observation and requires no cooperation from the target session. It provides immediate utility for understanding what other sessions have done.

**Independent Test**: Can be tested by running a session that performs some work, then reading its scrollback from another session. Delivers value by enabling passive observation.

**Acceptance Scenarios**:

1. **Given** a session that has produced terminal output, **When** another agent requests 30 lines of that session's scrollback, **Then** it receives the last 30 non-empty lines from the target pane's terminal buffer.
2. **Given** a session identifier that does not match any active session, **When** an agent requests scrollback, **Then** it receives a clear error indicating the session was not found.
3. **Given** a request for 0 or negative lines, **When** the tool is called, **Then** it returns an error with a helpful message about valid line counts.

---

### User Story 3 - Ask Another Session a Question (Priority: P2)

A user wants one agent to ask another agent a question and receive a response. The question is injected into the target session's input when it is idle, and the response is captured via a file-based handshake. This enables true agent-to-agent consultation where each agent reasons from its own full context.

**Why this priority**: This is the core differentiating feature, enabling the "different contexts, different results" value proposition. It depends on session discovery (P1) and builds on the text injection infrastructure already used for voice relay.

**Independent Test**: Can be tested by having one idle session receive a question from another session, with the response captured and returned to the caller. Delivers value by enabling cross-context reasoning.

**Acceptance Scenarios**:

1. **Given** a target session that is idle, **When** a source agent asks it a question, **Then** the question is injected into the target, the target processes it, writes a response file, and the source receives the response content.
2. **Given** a target session that is currently working (not idle), **When** a source agent tries to ask it a question, **Then** the source receives a "session busy" error without disrupting the target.
3. **Given** a question is asked but the target does not respond within the timeout period, **When** the timeout expires, **Then** the source receives a timeout error and the temporary files are cleaned up.
4. **Given** a successful ask/response exchange, **When** the response is returned, **Then** both the query and response temporary files are removed from disk.

---

### User Story 4 - Query Structured Session State (Priority: P2)

A user wants to query detailed structured state about another session, including its git changes, recent tool activity, and working directory details. This provides richer information than raw scrollback for programmatic decision-making.

**Why this priority**: Structured state enables agents to make informed decisions about whether and what to ask other sessions. It complements scrollback (raw observation) with structured metadata.

**Independent Test**: Can be tested by running a session that makes git changes and uses tools, then querying its state from another session. Delivers value by providing structured cross-session awareness.

**Acceptance Scenarios**:

1. **Given** a session that has modified files tracked by git, **When** another agent queries its state, **Then** the response includes a summary of modified files and git diff information.
2. **Given** a session that has been actively using tools, **When** another agent queries its state, **Then** the response includes a list of recently used tools with timestamps.

---

### User Story 5 - Automatic MCP Configuration (Priority: P3)

When a user installs the cc-deck plugin, the MCP server configuration is automatically added to the agent's configuration so the cross-pane tools are available without manual setup.

**Why this priority**: Without automatic configuration, users would need to manually edit MCP configuration files. This is a friction-reduction feature that makes the core tools accessible.

**Independent Test**: Can be tested by running `cc-deck plugin install` and verifying the MCP server entry appears in the agent's configuration file.

**Acceptance Scenarios**:

1. **Given** a fresh installation via `cc-deck plugin install` for Claude Code, **When** the install completes, **Then** the Claude Code MCP configuration (`.mcp.json`) includes an entry for `cc-deck mcp serve` as a stdio-based MCP server.
2. **Given** an existing MCP configuration with other servers, **When** `cc-deck plugin install` runs, **Then** the cc-deck MCP entry is added without disturbing existing entries.
3. **Given** cc-deck MCP is already configured, **When** `cc-deck plugin install` runs again, **Then** the configuration is updated (not duplicated).

---

### Edge Cases

- What happens when a session exits while another session is mid-ask? The ask should timeout and return an error, cleaning up temp files.
- How does the system handle multiple simultaneous asks to the same target? Each ask uses a unique UUID for its temp files, so they do not conflict. However, only one can be injected at a time since the target must be idle.
- What happens when the Zellij plugin is not running (e.g., cc-deck not installed)? The MCP server should return a clear error indicating it cannot communicate with the plugin.
- How does the system behave when a session's working directory does not contain a CLAUDE.md? The project summary field is omitted or empty in the session listing.
- What happens if the temp file directory (/tmp) is not writable? The ask tool returns an error indicating it cannot create temporary files.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST implement an MCP server as a `cc-deck mcp serve` Go CLI subcommand that communicates via stdio transport.
- **FR-002**: System MUST provide a `cc_deck_sessions` tool that returns a list of all active sessions with: display name, agent type, activity state, working directory, git branch, topic, project summary, recent activity, and modified files.
- **FR-003**: System MUST provide a `cc_deck_read_scrollback` tool that reads the last N lines from a specified session's terminal scrollback buffer.
- **FR-004**: System MUST provide a `cc_deck_session_state` tool that returns structured state for a session including git diff summary, recent tool calls, working directory, and activity timeline.
- **FR-005**: System MUST provide a `cc_deck_ask` tool that injects a question into an idle target session and returns the response via a file-based handshake.
- **FR-006**: The `cc_deck_ask` tool MUST verify the target session is idle before injecting a question and return a "session busy" error if the target is not idle.
- **FR-007**: The `cc_deck_ask` tool MUST use unique temporary files per request (UUID-based naming) to prevent conflicts between concurrent requests.
- **FR-008**: The `cc_deck_ask` tool MUST support a configurable timeout and return a timeout error if the target does not respond within the specified period.
- **FR-009**: System MUST clean up temporary query and response files after each ask interaction completes (success, timeout, or error).
- **FR-010**: The MCP server MUST communicate with the Zellij plugin via the existing `zellij pipe` infrastructure for all pane operations (scrollback reads, text injection, state queries).
- **FR-011**: System MUST parse the `prompt` field from UserPromptSubmit hook events and store the first ~100 characters as the session's current topic.
- **FR-012**: The session listing MUST aggregate context from existing files: CLAUDE.md for project description, MEMORY.md index for project knowledge, and git state for branch and modified files.
- **FR-013**: The `cc-deck plugin install` command MUST automatically configure the MCP server in the agent's configuration file (e.g., `.mcp.json` for Claude Code).
- **FR-014**: Sessions MUST be identifiable by their display name in all MCP tool calls. If multiple sessions share the same display name, the system MUST return an error listing the ambiguous matches with their pane IDs, allowing the caller to retry with a pane ID.
- **FR-015**: The MCP server MUST operate within a single Zellij instance (no cross-machine communication).
- **FR-016**: The `cc_deck_read_scrollback` tool MUST enforce a maximum of 500 lines per request to prevent excessive data transfer.
- **FR-017**: The `cc_deck_ask` tool MUST construct the injected prompt to include: the question text, the path to the response file, and clear instructions for the target agent to write its answer to that file.

### Key Entities

- **Session**: A tracked AI agent pane in Zellij, identified by display name, associated with a pane ID, agent type, activity state, working directory, topic, and context metadata.
- **MCP Server**: A Go process spawned by the agent (one per session), serving MCP tools over stdio and communicating with the Zellij plugin via pipe messages.
- **Query/Response Pair**: A file-based handshake consisting of a query file and a response file in /tmp, identified by a shared UUID, with lifecycle management (creation, monitoring, cleanup).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can discover all active sessions and their current work context in under 2 seconds.
- **SC-002**: An agent can read another session's scrollback (up to 100 lines) in under 1 second.
- **SC-003**: A question asked to an idle session receives a response within 120 seconds (default timeout).
- **SC-004**: The MCP server introduces zero overhead to sessions that do not use cross-pane tools (lazy communication, no polling).
- **SC-005**: Plugin installation automatically configures the MCP server without requiring manual configuration file edits.
- **SC-006**: All temporary files from ask/response exchanges are cleaned up within 5 seconds of completion.

## Clarifications

### Session 2026-07-26

- Q: How should sessions be identified when display names conflict? → A: Display name is primary; on conflict, return error with pane IDs for disambiguation.
- Q: What is the maximum scrollback that can be read in one request? → A: 500 lines maximum to prevent excessive data transfer.
- Q: What format should the injected prompt use for the ask flow? → A: Structured prompt including question text, response file path, and clear instructions for the target to write its answer to that file.

## Assumptions

- All target sessions run within the same Zellij instance on the same machine. Cross-machine communication is out of scope for this version.
- The Zellij plugin (cc-deck controller) is running and responsive to pipe messages. If the plugin is not running, MCP tools return clear errors.
- The `get_pane_scrollback` Zellij plugin API is available and functional for reading pane content.
- The `write_chars_to_pane_id` Zellij plugin API is available for text injection (already used by voice relay).
- Target sessions must be idle (not working or waiting) to receive injected questions. Busy sessions reject ask requests.
- The `/tmp` directory is writable and available for temporary file storage.
- Each Claude Code, Codex, or OpenCode session can be configured to use an MCP server via its native MCP configuration mechanism.
- The `prompt` field is available in the UserPromptSubmit hook payload for Claude Code. Availability for Codex and OpenCode should be verified during implementation.
- Agent sessions have unique display names within a Zellij instance. If names conflict, the system uses the first match.
