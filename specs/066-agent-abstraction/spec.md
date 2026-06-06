# Feature Specification: Agent Abstraction Layer

**Feature Branch**: `066-agent-abstraction`
**Created**: 2026-06-06
**Status**: Draft
**Input**: User description: "Core Agent Abstraction Layer: Define a Go Agent interface with registry, refactor existing Claude Code logic into a ClaudeAgent adapter, implement an OpenCodeAgent adapter as stress-test canary, add cc-deck hook --raw, generalize the Rust Zellij plugin."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install Plugin with Multiple Agents (Priority: P1)

A user has both Claude Code and OpenCode installed on their machine. They run `cc-deck plugin install` and the system auto-detects both agents, installs Claude Code hooks (via `settings.json`) and an OpenCode plugin (via `.opencode/plugins/`), and reports both as configured.

**Why this priority**: This is the entry point for multi-agent support. Users must be able to install cc-deck and have it discover and configure all installed agents automatically.

**Independent Test**: Can be fully tested by having both agent binaries available, running `cc-deck plugin install`, and verifying hooks are installed for Claude Code and a plugin is created for OpenCode.

**Acceptance Scenarios**:

1. **Given** Claude Code and OpenCode are both installed, **When** the user runs `cc-deck plugin install`, **Then** the system detects both agents, installs Claude Code hooks into `~/.claude/settings.json`, and reports both as configured.
2. **Given** only Claude Code is installed, **When** the user runs `cc-deck plugin install`, **Then** only Claude Code hooks are installed and no error is raised about missing agents.
3. **Given** no supported agents are installed, **When** the user runs `cc-deck plugin install`, **Then** a warning is printed that no agents were detected.
4. **Given** Claude Code hooks are already installed, **When** the user runs `cc-deck plugin install` again, **Then** the existing hooks are updated to the current version without duplication.

---

### User Story 2 - Sidebar Shows Agent Identity (Priority: P1)

A user runs multiple agent sessions in different Zellij panes. The sidebar displays an agent indicator next to each session so the user can distinguish which agent is running in each pane.

**Why this priority**: Without agent identification, users cannot tell which pane runs which agent. This is the minimum visual requirement for multi-agent support.

**Independent Test**: Can be tested by running one Claude Code session and one OpenCode session (via native plugin), and verifying the sidebar shows distinct indicators for each.

**Acceptance Scenarios**:

1. **Given** a Claude Code session is active, **When** the sidebar renders, **Then** a `[CC]` indicator appears before the session name.
2. **Given** an OpenCode session is active via its native plugin, **When** the sidebar renders, **Then** an `[OC]` indicator appears before the session name.
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

### User Story 4 - OpenCode Plugin Integration (Priority: P2)

A user has OpenCode installed. When they run `cc-deck plugin install`, the system creates a TypeScript plugin in OpenCode's plugin directory that calls `cc-deck hook --agent opencode` on lifecycle events. OpenCode sessions then appear in the sidebar with rich status information (tool use, permissions, idle).

**Why this priority**: OpenCode is the first non-Claude agent to validate the abstraction. Its plugin system supports all the lifecycle events needed for full Tier 1 integration, not just start/stop.

**Independent Test**: Can be tested by verifying the generated plugin file exists in `.opencode/plugins/` or `~/.config/opencode/plugins/`, and that OpenCode session events appear in the sidebar.

**Acceptance Scenarios**:

1. **Given** OpenCode is installed, **When** the user runs `cc-deck plugin install`, **Then** a TypeScript plugin file is created in the OpenCode plugins directory.
2. **Given** the cc-deck OpenCode plugin is installed, **When** an OpenCode session starts, **Then** the session appears in the sidebar with status `Init`.
3. **Given** the cc-deck OpenCode plugin is installed, **When** OpenCode executes a tool, **Then** the sidebar shows `Working` with the tool name.
4. **Given** the plugin is already installed, **When** the user runs `cc-deck plugin install` again, **Then** the plugin file is updated to the current version without duplication.

---

### User Story 5 - Raw Hook Payload Ingestion (Priority: P2)

A developer building a custom agent integration sends normalized hook payloads directly to cc-deck via the `cc-deck hook --raw` command. This allows any external process to feed events into the cc-deck sidebar without implementing a full agent adapter.

**Why this priority**: This is the stable API boundary for third-party integrations. Agent plugins (like the OpenCode TypeScript plugin) use it to send events, and external tools can also use it directly.

**Independent Test**: Can be tested by piping a JSON payload to `cc-deck hook --raw` and verifying it reaches the Zellij plugin as a pipe message.

**Acceptance Scenarios**:

1. **Given** a valid normalized JSON payload on stdin, **When** the user runs `cc-deck hook --raw`, **Then** the payload is forwarded to the Zellij plugin via the `cc-deck:hook` pipe.
2. **Given** a payload with an unknown agent name, **When** processed via `--raw`, **Then** the event is accepted and displayed with a generic indicator.
3. **Given** a payload missing the required `event` field, **When** processed via `--raw`, **Then** an error is printed to stderr and the event is rejected.

---

### User Story 6 - Plugin Status and Selective Management (Priority: P2)

A user wants to inspect or manage which agents have hooks installed. They use `cc-deck plugin status` to see the current state, `cc-deck plugin install --agents codex` to add hooks for a specific agent, or `cc-deck plugin uninstall` to remove all hooks.

**Why this priority**: Users need visibility and control over hook configuration, especially when troubleshooting or adding new agents.

**Independent Test**: Can be tested by installing hooks, running `cc-deck plugin status`, and verifying the output lists all detected agents with their hook state.

**Acceptance Scenarios**:

1. **Given** hooks are installed for Claude Code, **When** the user runs `cc-deck plugin status`, **Then** the output shows Claude Code as hooked with the config file path.
2. **Given** no hooks are installed, **When** the user runs `cc-deck plugin status`, **Then** the output shows detected agents but no hooks installed.
3. **Given** Claude Code and OpenCode both have hooks installed, **When** the user runs `cc-deck plugin uninstall --agents opencode`, **Then** the OpenCode plugin file is removed and OpenCode is reported as unhooked.

---

### Edge Cases

- What happens when an agent is installed but has no API key configured? Hooks are still installed; missing credentials are not a hook concern.
- What happens when two agent sessions report the same pane ID? The most recent event for that pane ID wins. Each pane runs exactly one agent.
- What happens when a previously hooked agent is uninstalled? `cc-deck plugin status` reports the agent as hooked but not detected. `cc-deck plugin uninstall` still works.
- What happens when the normalized payload contains an event type the plugin does not recognize? The plugin ignores the unknown event and logs a warning.

## Clarifications

### Session 2026-06-06

- Q: How should `cc-deck-agent-wrapper` be delivered to the user? → A: Superseded. Research showed the wrapper is unnecessary; all target agents have native hook/plugin systems. Removed from scope.
- Q: How should indicator conflicts between agents be resolved? → A: Each adapter defines a unique indicator (1-3 chars, e.g., CC, OC, CX). The registry enforces uniqueness at startup (panic on collision).
- Q: How should `plugin install` and `hooks install` relate? → A: Single `plugin` command group only (`plugin install`, `plugin uninstall`, `plugin status`) with `--agents` flag for filtering. No separate `hooks` command group.
- Q: What is `PaneTitlePattern()` used for? → A: No justified use case. Removed from the Agent interface. Hook events already carry the `agent` field; pane-title matching is redundant.
- Q: Is a generic agent wrapper needed? → A: No. Research shows all major AI coding agents (Claude Code, Codex, Cursor, OpenCode, Amp) have native hook/plugin systems that can execute shell commands. OpenCode has a full TypeScript plugin API with 25+ lifecycle events and shell access. The wrapper is removed; each agent gets a native adapter instead. A `cc-deck hook --raw` command remains as the stable API boundary for custom integrations.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST define a Go `Agent` interface with methods for identity (Name, DisplayName, Indicator), detection (IsInstalled, DetectConfig), hook lifecycle (InstallHooks, UninstallHooks, HooksInstalled), and event translation (TranslateEvent). `DetectConfig()` returns the filesystem path to the agent's configuration directory (e.g., `~/.claude/` for Claude Code, `~/.config/opencode/` for OpenCode) so that `InstallHooks()` knows where to write hook artifacts.
- **FR-002**: System MUST provide a central agent registry (Go map) where built-in agents register themselves.
- **FR-003**: System MUST ship a `ClaudeAgent` adapter that implements the Agent interface by extracting and encapsulating all existing Claude Code-specific logic from `internal/plugin/hooks.go`, `internal/cmd/hook.go`, and related files.
- **FR-004**: System MUST ship an `OpenCodeAgent` adapter that implements the Agent interface. OpenCode has a TypeScript plugin API with lifecycle hooks and shell access (via Bun `$`). The adapter's `InstallHooks()` MUST generate a TypeScript plugin file in the global OpenCode plugin directory (`~/.config/opencode/plugins/cc-deck.ts`) that calls `cc-deck hook --agent opencode` via shell on relevant lifecycle events. The generated plugin MUST handle the following OpenCode event-to-cc-deck mappings:
  - `event` hook with `session.next.step.started` → cc-deck `SessionStart` (session init, carries agent and model info)
  - `event` hook with `session.next.step.ended` → cc-deck `Stop` (session done, carries cost/token info)
  - `tool.execute.before` hook → cc-deck `PreToolUse` (tool name from `input.tool`)
  - `tool.execute.after` hook → cc-deck `PostToolUse` (tool result)
  - `permission.ask` hook → cc-deck `PermissionRequest` (permission prompt)
- **FR-005**: System MUST provide a `cc-deck hook --raw` subcommand that accepts a pre-normalized JSON payload on stdin and forwards it to the Zellij plugin via the `cc-deck:hook` pipe message. Unlike `cc-deck hook --agent <name>` (which accepts agent-specific payloads and calls `TranslateEvent()`), `--raw` expects the payload to already be in the normalized format and skips translation.
- **FR-006**: The `cc-deck plugin install` command MUST auto-detect all installed agents by iterating the registry and calling `IsInstalled()` on each.
- **FR-007**: The `cc-deck plugin install` command MUST install hooks for all detected agents by calling each adapter's `InstallHooks()` method.
- **FR-008**: The `cc-deck plugin install` command MUST accept an optional `--agents` flag to filter by agent name. The `cc-deck plugin uninstall` command MUST remove hooks for all (or filtered) agents. The `cc-deck plugin status` command MUST show all detected agents with their hook installation state. No separate `hooks` command group exists.
- **FR-009**: The existing `cc-deck hook` command MUST accept an `--agent <name>` flag that identifies the calling agent. Each agent's hook installation MUST include this flag in the hook command (e.g., `cc-deck hook --agent claude`). The command MUST delegate to the named adapter's `TranslateEvent()` method. If `--agent` is omitted, the command MUST default to "claude" for backward compatibility with existing hook installations.
- **FR-010**: The normalized hook payload MUST include an `agent` field (string, e.g., "claude", "opencode") alongside the existing event, session_id, pane_id, and other fields.
- **FR-011**: The Rust Zellij plugin MUST accept and store the `agent` field from incoming hook payloads.
- **FR-012**: The Rust plugin sidebar MUST display an agent indicator (e.g., `[CC]`, `[OC]`) before the session name when multiple agent types are active. Each adapter defines a unique 1-3 character indicator. The agent registry MUST enforce indicator uniqueness at startup.
- **FR-013**: The Rust plugin sidebar MUST hide agent indicators when all sessions use the same agent type.
- **FR-014**: The Rust plugin MUST remove all hardcoded "Claude Code" text from user-visible UI strings. Static UI text (labels, headers) MUST use agent-agnostic language (e.g., "sessions" instead of "Claude Code sessions"). Dynamic references to a specific session MUST use the agent's DisplayName from the payload.
- **FR-015**: The Go CLI MUST remove "Claude Code" from help text and command descriptions where it implies exclusivity (e.g., "Manage Claude Code workspaces" becomes "Manage AI agent workspaces").
- **FR-016**: All existing Claude Code hook functionality MUST continue to work identically after the refactoring. This is a zero-regression requirement.

### Key Entities

- **Agent**: Interface representing a supported AI coding agent with methods for detection, hook management, and event translation.
- **Agent Registry**: Central Go map that maps agent name strings to Agent interface implementations.
- **Normalized Payload**: The JSON format that all agent events are translated into before being sent to the Zellij plugin via pipe message. Extends the existing `HookPayload` with an `agent` field.
- **Agent Plugin**: A generated artifact (e.g., TypeScript plugin for OpenCode, JSON hooks for Claude Code) that an adapter's `InstallHooks()` creates in the agent's native configuration directory to bridge lifecycle events to cc-deck.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing Claude Code hook tests pass without modification after the refactoring, confirming zero regressions.
- **SC-002**: An OpenCode session started with the cc-deck plugin installed appears in the sidebar within 1 second of the session starting. Testing note: CI environments may not have OpenCode installed; unit tests MUST verify the generated plugin file content and the event translation logic independently.
- **SC-003**: The agent indicator is visible and correct when sessions from two different agent types are active simultaneously.
- **SC-004**: `cc-deck plugin install` correctly detects and reports all installed agents in under 2 seconds.
- **SC-005**: Unit test coverage exists for all Agent interface implementations (ClaudeAgent, OpenCodeAgent) including detection, hook installation, and event translation.
- **SC-006**: The `cc-deck hook --raw` command accepts and forwards a normalized payload in under 50ms.
- **SC-007**: No Go source file outside the agent adapter packages (`internal/agent/` and its sub-packages) references Claude Code-specific constants (event names, config paths, credential env vars) directly. All such references go through the Agent interface.
- **SC-008**: The generated OpenCode TypeScript plugin file is syntactically valid and contains handlers for all mapped lifecycle events (step.started, step.ended, tool.execute.before, tool.execute.after, permission.ask).

## Error Handling

- If a generated agent plugin (e.g., the OpenCode TypeScript plugin) calls `cc-deck hook --agent opencode` but `cc-deck` is not in PATH, the plugin MUST fail silently (log to stderr if possible) and not crash the host agent. Agent functionality is unaffected; only sidebar updates are lost.
- If `cc-deck hook --raw` receives malformed JSON, it MUST print a diagnostic to stderr and exit with a non-zero status. It MUST NOT forward partial data to the Zellij plugin.
- If the Zellij pipe message delivery fails (e.g., Zellij is not running, plugin not loaded), `cc-deck hook` MUST print a warning to stderr and exit with a non-zero status. It MUST NOT retry.
- If `cc-deck plugin install` cannot write to an agent's configuration directory (permission denied), it MUST report the failure for that agent and continue installing hooks for other agents.
- If the agent registry detects an indicator collision at startup, it MUST panic with a message naming both conflicting agents. This is a development-time error, not a runtime error.

## Documentation Requirements

Per constitution Principle I, this feature MUST include:

- **README.md**: Updated to describe multi-agent support, the Agent interface concept, and the `--agent` flag.
- **CLI reference** (`docs/modules/reference/pages/cli.adoc`): Document `cc-deck hook --agent`, `cc-deck hook --raw`, `cc-deck plugin status`, `cc-deck plugin uninstall`, and the `--agents` filter flag.
- **Antora guide**: New guide page covering multi-agent setup (installing hooks for multiple agents, verifying with `plugin status`).
- **Configuration reference** (`docs/modules/reference/pages/configuration.adoc`): Document the generated OpenCode plugin file location and format.

## Assumptions

- The cc-deck naming is retained. Only help text and UI strings that imply Claude Code exclusivity are updated.
- Network policy generalization (per-agent domain declarations) is out of scope for this spec and will be addressed in a follow-up.
- Credential transport abstraction (per-agent credential env vars and provider separation) is out of scope and will be addressed in a follow-up.
- Build system changes (multi-agent Containerfile generation, manifest `agents` section) are out of scope and will be addressed in a follow-up.
- The OpenCode adapter generates a TypeScript plugin that calls `cc-deck hook --agent opencode` on lifecycle events. This is the native integration approach; no generic wrapper is needed.
- Most major AI coding agents (Claude Code, Codex CLI, Cursor, OpenCode, Amp) have native hook/plugin systems. Each new agent adapter generates the appropriate hook artifact for that agent's system. A generic wrapper command is deferred as unnecessary for current targets.
- Adding a new built-in agent after this spec requires implementing the Go Agent interface and adding it to the registry. This is a code change (PR), not a configuration change.
