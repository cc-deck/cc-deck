# Feature Specification: Agent Abstraction Layer

**Feature Branch**: `066-agent-abstraction`
**Created**: 2026-06-06
**Status**: Draft
**Input**: User description: "Core Agent Abstraction Layer: Define a Go Agent interface with registry, refactor existing Claude Code logic into a ClaudeAgent adapter, implement an OpenCodeAgent adapter as stress-test canary, create cc-deck-agent-wrapper and cc-deck hook --raw, generalize the Rust Zellij plugin."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install Plugin with Multiple Agents (Priority: P1)

A user has both Claude Code and OpenCode installed on their machine. They run `cc-deck plugin install` and the system auto-detects both agents, installs hooks for Claude Code (native hooks), and reports that OpenCode will use the agent wrapper for lifecycle events.

**Why this priority**: This is the entry point for multi-agent support. Users must be able to install cc-deck and have it discover and configure all installed agents automatically.

**Independent Test**: Can be fully tested by having both agent binaries available, running `cc-deck plugin install`, and verifying hooks are installed for Claude Code and a wrapper setup is reported for OpenCode.

**Acceptance Scenarios**:

1. **Given** Claude Code and OpenCode are both installed, **When** the user runs `cc-deck plugin install`, **Then** the system detects both agents, installs Claude Code hooks into `~/.claude/settings.json`, and reports both as configured.
2. **Given** only Claude Code is installed, **When** the user runs `cc-deck plugin install`, **Then** only Claude Code hooks are installed and no error is raised about missing agents.
3. **Given** no supported agents are installed, **When** the user runs `cc-deck plugin install`, **Then** a warning is printed that no agents were detected.
4. **Given** Claude Code hooks are already installed, **When** the user runs `cc-deck plugin install` again, **Then** the existing hooks are updated to the current version without duplication.

---

### User Story 2 - Sidebar Shows Agent Identity (Priority: P1)

A user runs multiple agent sessions in different Zellij panes. The sidebar displays an agent indicator next to each session so the user can distinguish which agent is running in each pane.

**Why this priority**: Without agent identification, users cannot tell which pane runs which agent. This is the minimum visual requirement for multi-agent support.

**Independent Test**: Can be tested by running one Claude Code session and one OpenCode session (via wrapper), and verifying the sidebar shows distinct indicators for each.

**Acceptance Scenarios**:

1. **Given** a Claude Code session is active, **When** the sidebar renders, **Then** a `[C]` indicator appears before the session name.
2. **Given** an OpenCode session is active via the agent wrapper, **When** the sidebar renders, **Then** an `[O]` indicator appears before the session name.
3. **Given** all sessions use the same agent type, **When** the sidebar renders, **Then** the agent indicator is hidden to save horizontal space.
4. **Given** a mix of Claude Code and OpenCode sessions, **When** the sidebar renders, **Then** agent indicators are shown for all sessions.

---

### User Story 3 - Hook Events from Claude Code (Priority: P1)

A user is running Claude Code in a workspace managed by cc-deck. All existing hook events (SessionStart, PreToolUse, PostToolUse, PermissionRequest, Notification, Stop, etc.) continue to work exactly as before, with no regressions.

**Why this priority**: The refactoring must not break existing Claude Code functionality. Backward compatibility with the current hook system is a hard requirement.

**Independent Test**: Can be tested by running all existing hook integration tests and verifying identical behavior before and after the refactoring.

**Acceptance Scenarios**:

1. **Given** a Claude Code session emits a SessionStart hook event, **When** the event is processed by cc-deck, **Then** the session appears in the sidebar with status `Init`.
2. **Given** a Claude Code session emits a PreToolUse event with tool name, **When** the event is processed, **Then** the sidebar shows `Working` with the tool name.
3. **Given** a Claude Code session emits a PermissionRequest event, **When** the event is processed, **Then** the sidebar shows `Waiting(Permission)` and smart attend may switch focus.
4. **Given** the same Claude Code hook JSON payload as before the refactoring, **When** processed through the new agent adapter, **Then** the resulting plugin pipe message is byte-identical to the pre-refactoring output.

---

### User Story 4 - Wrapper-Based Agent Sessions (Priority: P2)

A user launches an agent that does not have native cc-deck hook support (e.g., OpenCode, Aider, or any arbitrary CLI tool). They wrap the agent command with `cc-deck-agent-wrapper`, which emits lifecycle events so the session appears in the sidebar.

**Why this priority**: The wrapper is the extension mechanism for any agent without native hooks. It enables the two-tier status model (Tier 1: full hooks, Tier 2: wrapper lifecycle only).

**Independent Test**: Can be tested by wrapping a simple command (e.g., `sleep 10`) with the agent wrapper and verifying it appears in the sidebar with Init and End events.

**Acceptance Scenarios**:

1. **Given** a user runs `cc-deck-agent-wrapper opencode -- opencode`, **When** OpenCode starts, **Then** an Init event is sent and the session appears in the sidebar.
2. **Given** a wrapped agent is running, **When** the agent process exits normally, **Then** an End event is sent and the session status updates to Done.
3. **Given** a wrapped agent is running, **When** the agent process is killed (SIGTERM), **Then** the wrapper catches the signal, sends an End event, and exits.
4. **Given** a wrapped agent is running, **When** the wrapper process itself is killed (SIGKILL), **Then** no End event is sent (best-effort only, not guaranteed).

---

### User Story 5 - Raw Hook Payload Ingestion (Priority: P2)

A developer building a custom agent integration sends normalized hook payloads directly to cc-deck via the `cc-deck hook --raw` command. This allows any external process to feed events into the cc-deck sidebar without implementing a full agent adapter.

**Why this priority**: This is the stable API boundary for third-party integrations. The agent wrapper uses it internally, but external tools can also use it directly.

**Independent Test**: Can be tested by piping a JSON payload to `cc-deck hook --raw` and verifying it reaches the Zellij plugin as a pipe message.

**Acceptance Scenarios**:

1. **Given** a valid normalized JSON payload on stdin, **When** the user runs `cc-deck hook --raw`, **Then** the payload is forwarded to the Zellij plugin via the `cc-deck:hook` pipe.
2. **Given** a payload with an unknown agent name, **When** processed via `--raw`, **Then** the event is accepted and displayed with a generic indicator.
3. **Given** a payload missing the required `event` field, **When** processed via `--raw`, **Then** an error is printed to stderr and the event is rejected.

---

### User Story 6 - Hook Management Commands (Priority: P2)

A user wants to inspect or manage which agents have hooks installed. They use `cc-deck hooks status` to see the current state, `cc-deck hooks install --agents codex` to add hooks for a specific agent, or `cc-deck hooks uninstall` to remove all hooks.

**Why this priority**: Users need visibility and control over hook configuration, especially when troubleshooting or adding new agents.

**Independent Test**: Can be tested by installing hooks, running `cc-deck hooks status`, and verifying the output lists all hooked agents with their status.

**Acceptance Scenarios**:

1. **Given** hooks are installed for Claude Code, **When** the user runs `cc-deck hooks status`, **Then** the output shows Claude Code as hooked with the config file path.
2. **Given** no hooks are installed, **When** the user runs `cc-deck hooks status`, **Then** the output shows no agents hooked.
3. **Given** Claude Code has native hooks installed and OpenCode uses the wrapper, **When** the user runs `cc-deck hooks uninstall --agents opencode`, **Then** an informational message is printed explaining that OpenCode uses the wrapper and has no hooks to uninstall.

---

### Edge Cases

- What happens when an agent is installed but has no API key configured? Hooks are still installed; missing credentials are not a hook concern.
- What happens when two agent sessions report the same pane ID? The most recent event for that pane ID wins. Each pane runs exactly one agent.
- What happens when the agent wrapper is started outside a Zellij session? The wrapper detects `ZELLIJ_PANE_ID` is absent and skips sending pipe events, only running the wrapped command.
- What happens when a previously hooked agent is uninstalled? `cc-deck hooks status` reports the agent as hooked but not detected. `cc-deck hooks uninstall` still works.
- What happens when the normalized payload contains an event type the plugin does not recognize? The plugin ignores the unknown event and logs a warning.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST define a Go `Agent` interface with methods for identity (Name, DisplayName, Indicator), detection (IsInstalled, DetectConfig), hook lifecycle (InstallHooks, UninstallHooks, HooksInstalled, HasNativeHooks), event translation (TranslateEvent), and pane identification (PaneTitlePattern).
- **FR-002**: System MUST provide a central agent registry (Go map) where built-in agents register themselves.
- **FR-003**: System MUST ship a `ClaudeAgent` adapter that implements the Agent interface by extracting and encapsulating all existing Claude Code-specific logic from `internal/plugin/hooks.go`, `internal/cmd/hook.go`, and related files.
- **FR-004**: System MUST ship an `OpenCodeAgent` adapter that implements the Agent interface. Since OpenCode uses in-process TypeScript hooks (not shell-invocable), this adapter MUST return `HasNativeHooks() = false` and rely on the agent wrapper for lifecycle events.
- **FR-005**: System MUST ship a `cc-deck-agent-wrapper` shell script that wraps an arbitrary command, emits Init and End events in the normalized payload format via `cc-deck hook --raw`, and handles signal forwarding (SIGTERM, SIGINT) for clean shutdown.
- **FR-006**: System MUST provide a `cc-deck hook --raw` subcommand that accepts a normalized JSON payload on stdin and forwards it to the Zellij plugin via the `cc-deck:hook` pipe message.
- **FR-007**: The `cc-deck plugin install` command MUST auto-detect all installed agents by iterating the registry and calling `IsInstalled()` on each.
- **FR-008**: The `cc-deck plugin install` command MUST install hooks for all detected agents that have `HasNativeHooks() = true`.
- **FR-009**: System MUST provide `cc-deck hooks install`, `cc-deck hooks uninstall`, and `cc-deck hooks status` subcommands with optional `--agents` flag to filter by agent name. The `hooks status` output MUST distinguish between agents with native hooks installed and agents that use the wrapper (no hooks to manage). The `hooks install` and `hooks uninstall` commands MUST only operate on agents with `HasNativeHooks() = true`; for wrapper-only agents, they MUST print an informational message explaining the agent uses the wrapper instead.
- **FR-010**: The existing `cc-deck hook` command MUST accept an `--agent <name>` flag that identifies the calling agent. Each agent's hook installation MUST include this flag in the hook command (e.g., `cc-deck hook --agent claude`). The command MUST delegate to the named adapter's `TranslateEvent()` method. If `--agent` is omitted, the command MUST default to "claude" for backward compatibility with existing hook installations.
- **FR-011**: The normalized hook payload MUST include an `agent` field (string, e.g., "claude", "opencode") alongside the existing event, session_id, pane_id, and other fields.
- **FR-012**: The Rust Zellij plugin MUST accept and store the `agent` field from incoming hook payloads.
- **FR-013**: The Rust plugin sidebar MUST display an agent indicator (e.g., `[C]`, `[O]`) before the session name when multiple agent types are active.
- **FR-014**: The Rust plugin sidebar MUST hide agent indicators when all sessions use the same agent type.
- **FR-015**: The Rust plugin MUST remove all hardcoded "Claude Code" text from user-visible UI strings. Static UI text (labels, headers) MUST use agent-agnostic language (e.g., "sessions" instead of "Claude Code sessions"). Dynamic references to a specific session MUST use the agent's DisplayName from the payload.
- **FR-016**: The Go CLI MUST remove "Claude Code" from help text and command descriptions where it implies exclusivity (e.g., "Manage Claude Code workspaces" becomes "Manage AI agent workspaces").
- **FR-017**: All existing Claude Code hook functionality MUST continue to work identically after the refactoring. This is a zero-regression requirement.

### Key Entities

- **Agent**: Interface representing a supported AI coding agent with methods for detection, hook management, and event translation.
- **Agent Registry**: Central Go map that maps agent name strings to Agent interface implementations.
- **Normalized Payload**: The JSON format that all agent events are translated into before being sent to the Zellij plugin via pipe message. Extends the existing `HookPayload` with an `agent` field.
- **Agent Wrapper**: A shell script that wraps arbitrary CLI commands with Init/End lifecycle events for sidebar presence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing Claude Code hook tests pass without modification after the refactoring, confirming zero regressions.
- **SC-002**: A session started via `cc-deck-agent-wrapper` appears in the sidebar within 1 second of the wrapped process starting.
- **SC-003**: The agent indicator is visible and correct when sessions from two different agent types are active simultaneously.
- **SC-004**: `cc-deck plugin install` correctly detects and reports all installed agents in under 2 seconds.
- **SC-005**: Unit test coverage exists for all Agent interface implementations (ClaudeAgent, OpenCodeAgent) including detection, hook installation, and event translation.
- **SC-006**: The `cc-deck hook --raw` command accepts and forwards a normalized payload in under 50ms.
- **SC-007**: No Go source file outside the agent adapter packages (`internal/agent/` and its sub-packages) references Claude Code-specific constants (event names, config paths, credential env vars) directly. All such references go through the Agent interface.

## Assumptions

- The cc-deck naming is retained. Only help text and UI strings that imply Claude Code exclusivity are updated.
- Network policy generalization (per-agent domain declarations) is out of scope for this spec and will be addressed in a follow-up.
- Credential transport abstraction (per-agent credential env vars and provider separation) is out of scope and will be addressed in a follow-up.
- Build system changes (multi-agent Containerfile generation, manifest `agents` section) are out of scope and will be addressed in a follow-up.
- The OpenCode adapter relies on the wrapper approach for this initial implementation. A richer integration (e.g., a TypeScript OpenCode plugin that calls `cc-deck hook --raw` on lifecycle events) may be explored in a follow-up but is not required here.
- The `cc-deck-agent-wrapper` is a POSIX shell script. Windows support (PowerShell wrapper) is deferred.
- Adding a new built-in agent after this spec requires implementing the Go Agent interface and adding it to the registry. This is a code change (PR), not a configuration change.
