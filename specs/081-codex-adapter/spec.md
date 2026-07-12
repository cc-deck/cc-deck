# Feature Specification: Codex CLI Agent Adapter

**Feature Branch**: `081-codex-adapter`
**Created**: 2026-07-12
**Status**: Draft
**Input**: Brainstorm 081 - Codex CLI Agent Adapter

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Codex Sessions Appear in Sidebar (Priority: P1)

A user has Codex CLI installed and runs it in a Zellij pane alongside Claude Code sessions. After installing cc-deck hooks, Codex sessions appear in the sidebar with a distinct indicator, showing real-time activity states (Working, Idle, Waiting) just like Claude Code sessions.

**Why this priority**: Sidebar visibility is the core value. Without it, Codex sessions are invisible to cc-deck.

**Independent Test**: Install hooks with `cc-deck config plugin install`. Start Codex in a pane (`codex`). Verify the sidebar shows a new session with the Codex indicator. Type a prompt, verify the session shows as Working. Wait for the response, verify it transitions to Idle.

**Acceptance Scenarios**:

1. **Given** Codex CLI is installed and hooks are configured, **When** the user starts a Codex session, **Then** the sidebar shows a new entry with the Codex-specific indicator.
2. **Given** a Codex session is active in the sidebar, **When** the user submits a prompt, **Then** the session transitions to Working state. When the agent finishes (Stop event), the session transitions to Idle.
3. **Given** Codex requests permission for a tool use, **When** the PermissionRequest event fires, **Then** the session shows as Waiting.

---

### User Story 2 - Hook Installation and Removal (Priority: P2)

Running `cc-deck config plugin install` detects that Codex CLI is installed and automatically adds cc-deck hooks to Codex's hook configuration. Running `cc-deck config plugin uninstall` removes the hooks cleanly, leaving other Codex hooks intact.

**Why this priority**: Hook installation is the prerequisite for sidebar integration. It must be idempotent and non-destructive.

**Independent Test**: Run `cc-deck config plugin install`. Verify `~/.codex/hooks.json` contains cc-deck hook entries. Run the command again, verify no duplicate entries are created. Run `cc-deck config plugin uninstall`, verify cc-deck entries are removed but the file structure is preserved.

**Acceptance Scenarios**:

1. **Given** Codex CLI is installed, **When** `cc-deck config plugin install` runs, **Then** `~/.codex/hooks.json` is created (or updated) with cc-deck hook entries for all supported events.
2. **Given** cc-deck hooks are already installed, **When** `cc-deck config plugin install` runs again, **Then** no duplicate entries are created (idempotent).
3. **Given** `~/.codex/hooks.json` has other hooks from other tools, **When** `cc-deck config plugin uninstall` runs, **Then** only cc-deck entries are removed and the other hooks remain intact.

---

### User Story 3 - Credential Detection for Codex (Priority: P3)

When creating a workspace that uses Codex, the system detects `OPENAI_API_KEY` from the host environment and injects it into the workspace. The credential detection uses the same agent-declared `CredentialSpec` model as Claude Code and OpenCode.

**Why this priority**: Workspace credential injection for Codex sessions ensures the agent can authenticate with OpenAI's API.

**Independent Test**: Set `OPENAI_API_KEY` in the environment. Run `cc-deck ws new` with Codex as the agent. Verify the credential is detected and available in the workspace.

**Acceptance Scenarios**:

1. **Given** `OPENAI_API_KEY` is set, **When** credential detection runs for a Codex workspace, **Then** the key is detected as available for the "api" auth mode.
2. **Given** `OPENAI_API_KEY` is not set, **When** credential detection runs, **Then** no credentials are detected for Codex (the "api" mode is unavailable).

---

### Edge Cases

- What happens when Codex CLI is not installed? `IsInstalled()` returns false, hook installation skips Codex, and no Codex entries appear in the sidebar.
- What happens when `~/.codex/` directory does not exist? `DetectConfig()` returns empty string. `InstallHooks()` creates `~/.codex/hooks.json` (Codex CLI creates `~/.codex/` on first run, but the hooks file may not exist yet).
- What happens when the user has an older Codex version without hooks support? The hooks are written to `~/.codex/hooks.json` but Codex ignores them. The user sees no sidebar integration until they upgrade. No error or crash occurs.
- What happens when Codex uses tool names different from Claude Code (e.g., `apply_patch` instead of `Write`)? `TranslateEvent()` passes the tool name through as-is. The sidebar and plugin do not depend on specific tool names for activity state tracking.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A new Codex agent adapter MUST implement the full Agent interface (Name, DisplayName, Indicator, IsInstalled, DetectConfig, InstallHooks, UninstallHooks, HooksInstalled, TranslateEvent, CredentialSpecs, RequiredDomainGroups).
- **FR-002**: `InstallHooks()` MUST write hook entries to `~/.codex/hooks.json` using the Codex hook configuration format (JSON with `hooks` wrapper object containing event name keys mapping to arrays of matcher groups).
- **FR-003**: `InstallHooks()` MUST register hooks for these events: `SessionStart`, `PreToolUse`, `PostToolUse`, `PermissionRequest`, `UserPromptSubmit`, `Stop`, `SubagentStart`, `SubagentStop`.
- **FR-004**: `InstallHooks()` MUST be idempotent: calling it when hooks exist updates them without creating duplicates. It MUST preserve hooks installed by other tools.
- **FR-005**: `UninstallHooks()` MUST remove only cc-deck hook entries from `~/.codex/hooks.json`, leaving other hooks intact. It MUST return nil if no hooks are installed.
- **FR-006**: `TranslateEvent()` MUST parse the Codex hook JSON payload (containing `session_id`, `cwd`, and event-specific fields) and produce a `NormalizedPayload` with the agent field set to "codex". Fields present in the Codex payload but absent from `NormalizedPayload` (e.g., `model`) are silently ignored.
- **FR-007**: `CredentialSpecs()` MUST declare at least one auth mode named "openai" (consistent with the OpenCode adapter's naming) with `OPENAI_API_KEY` as a required env var.
- **FR-008**: The Codex indicator MUST be "◆" (U+25C6, black diamond), visually distinct from Claude Code ("✳") and OpenCode ("❯").
- **FR-009**: `IsInstalled()` MUST check for `codex` in PATH. `DetectConfig()` MUST check for `~/.codex/` directory.
- **FR-010**: The adapter MUST register itself via `agent.Register()` in an init function, following the same pattern as Claude Code and OpenCode adapters.

### Error Handling

- **EH-001**: If `~/.codex/hooks.json` exists but contains invalid JSON, `InstallHooks()` MUST return an error describing the parse failure. It MUST NOT overwrite or truncate the corrupted file.
- **EH-002**: If writing to `~/.codex/hooks.json` fails due to permission errors, `InstallHooks()` MUST return a wrapped error with the path and underlying OS error.
- **EH-003**: `TranslateEvent()` MUST return an error for malformed JSON input or payloads missing the required `hook_event_name` field.
- **EH-004**: `UninstallHooks()` MUST return nil (no error) if hooks are not installed or `~/.codex/hooks.json` does not exist.

### Key Entities

- **CodexAgent**: The Go struct implementing the Agent interface for Codex CLI.
- **Codex Hook Payload**: The JSON structure received on stdin during hook events, containing `session_id`, `cwd`, `model`, and event-specific fields.
- **hooks.json**: The Codex CLI hook configuration file at `~/.codex/hooks.json`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After hook installation, Codex sessions appear in the cc-deck sidebar with correct activity state transitions (Init, Working, Waiting, Idle).
- **SC-002**: `cc-deck config plugin install` followed by `cc-deck config plugin uninstall` leaves `~/.codex/hooks.json` in a clean state with no cc-deck entries and no corruption of other hooks.
- **SC-003**: All existing Claude Code and OpenCode tests pass without modification (zero regression).
- **SC-004**: The Codex adapter has unit tests covering: identity values, hook installation/removal idempotency, TranslateEvent for all registered event types, TranslateEvent error for malformed input, and CredentialSpecs declarations.

## Documentation Requirements

Per the project constitution, the following documentation MUST be updated as part of this feature:

- **README.md**: Update the supported agents list to include Codex CLI.
- **Antora agent guide**: Add Codex to the agent integration documentation (alongside Claude Code and OpenCode).
- **Configuration reference** (`docs/modules/reference/pages/configuration.adoc`): Document the `~/.codex/hooks.json` file location and hook configuration format.

No new CLI commands or flags are introduced (the existing `cc-deck config plugin install/uninstall` and `cc-deck hook --agent codex` commands cover Codex automatically via the agent registry).

## Out of Scope

- Codex-specific features beyond sidebar integration (e.g., session resume, fork, cost tracking).
- Build system changes for multi-agent container images (that's spec 070).
- Gemini CLI adapter (separate brainstorm).
- Tool name normalization (Codex tool names pass through as-is).

## Assumptions

- Codex CLI's hooks system (confirmed in source at `codex-rs/hooks/`, `codex-rs/config/src/hook_config.rs`) is available in recent versions. Users on older versions (e.g., v0.46.0) will need to upgrade for hooks support.
- The Codex hook JSON payload structure is compatible with the existing `NormalizedPayload` model. Fields like `session_id` and `cwd` map directly.
- The `~/.codex/hooks.json` format uses a `{"hooks": {EventName: [MatcherGroup]}}` structure where each `MatcherGroup` has a `hooks` array of handler configs with `type`, `command`, `async`, and optional `matcher` fields.
- The existing `cc-deck hook --agent <name>` command accepts any registered agent name, so `--agent codex` works with no changes to the hook command itself.
