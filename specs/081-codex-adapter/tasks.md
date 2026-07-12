# Tasks: Codex CLI Agent Adapter

**Branch**: `081-codex-adapter` | **Generated**: 2026-07-12

## Dependencies

```
Task 1 (Agent implementation) ──> Task 2 (Tests) ──> Task 3 (Documentation)
```

Sequential: each task builds on the previous.

## Task List

- [x] **Task 1: Implement CodexAgent struct with all Agent interface methods**

  **Files**: `cc-deck/internal/agent/codex.go`

  **Interfaces produced**:
  - `CodexAgent` struct implementing `agent.Agent` interface
  - `codexHookCommand() string` returning the cc-deck hook command for Codex

  **What to do**:

  1. Create `cc-deck/internal/agent/codex.go` following the `claude.go` pattern. The file structure:
     - `CodexAgent` struct (empty, like `ClaudeAgent`)
     - `init()` function calling `Register(&CodexAgent{})`
     - Identity methods: `Name() = "codex"`, `DisplayName() = "Codex CLI"`, `Indicator() = "◆"`
     - `IsInstalled()`: `exec.LookPath("codex")`
     - `DetectConfig()`: check `~/.codex/` directory exists
     - `InstallHooks()`: Read `~/.codex/hooks.json` (create if missing), parse JSON, add cc-deck hook entries for each event (`SessionStart`, `PreToolUse`, `PostToolUse`, `PermissionRequest`, `UserPromptSubmit`, `Stop`, `SubagentStart`, `SubagentStop`). Use the `readCodexHooks`/`writeCodexHooks` helper pattern. The hooks.json format wraps events in a `{"hooks": {...}}` object. Remove existing cc-deck entries before adding new ones (idempotent). Handle EH-001 and EH-002.
     - `UninstallHooks()`: Read hooks.json, remove cc-deck entries, write back. Return nil if file missing (EH-004).
     - `HooksInstalled()`: Read hooks.json, check if any entry has the cc-deck hook command.
     - `TranslateEvent()`: Parse JSON payload with `session_id`, `hook_event_name`, `tool_name`, `cwd`. Produce `NormalizedPayload`. Handle EH-003 (missing `hook_event_name`).
     - `CredentialSpecs()`: Return one spec named "openai" with `OPENAI_API_KEY` (required).
     - `RequiredDomainGroups()`: Return `[]string{"openai"}`.

  2. Helper functions (private):
     - `codexHookCommand() string`: returns `"cc-deck hook --agent codex --pane-id \"$ZELLIJ_PANE_ID\""`
     - `codexHooksPath() string`: returns `~/.codex/hooks.json` (with testable override var like `codexHooksPathFunc`)
     - `readCodexHooks(path string) (map[string]any, error)`: read and parse hooks.json (handles the `{"hooks": {...}}` wrapper)
     - `writeCodexHooks(path string, hooks map[string]any) error`: write hooks.json via `fileutil.AtomicWrite`
     - **Reuse existing helpers** from `claude.go` (same package): `isCCDeckEntry`, `removeCCDeckHooks`, `containsCCDeckHook`, `structToMap`. These match on the `"cc-deck hook"` prefix and work for any agent.

  3. Ensure the `cc-deck/cmd/cc-deck/main.go` blank import includes the agent package (it should already via `_ "github.com/cc-deck/cc-deck/internal/agent"` from the existing Claude/OpenCode adapters).

  **Acceptance**: `make test` passes. `codex.go` compiles. `agent.Get("codex")` returns the CodexAgent. `agent.All()` includes codex in alphabetical order.

---

- [x] **Task 2: Add comprehensive unit tests**

  **Files**: `cc-deck/internal/agent/codex_test.go`

  **What to do**:

  1. Create `cc-deck/internal/agent/codex_test.go` following the `claude_test.go` pattern. Test cases:

     - **Identity**: Name="codex", DisplayName="Codex CLI", Indicator="◆"
     - **InstallHooks**: Creates hooks.json with correct structure, all 8 events registered
     - **InstallHooks idempotency**: Calling twice produces no duplicates
     - **InstallHooks preserves other hooks**: Add a non-cc-deck hook, install, verify it survives
     - **InstallHooks corrupted file**: Write invalid JSON to hooks.json, verify error returned (EH-001)
     - **UninstallHooks**: Removes cc-deck entries, leaves others
     - **UninstallHooks no file**: Returns nil when hooks.json missing (EH-004)
     - **HooksInstalled**: Returns true when installed, false when not
     - **TranslateEvent all events**: Parse valid payloads for each event type, verify NormalizedPayload fields
     - **TranslateEvent malformed**: Missing hook_event_name returns error (EH-003)
     - **TranslateEvent invalid JSON**: Returns error
     - **CredentialSpecs**: Returns one spec named "openai" with OPENAI_API_KEY required

  2. Use `codexHooksPathFunc` override for test isolation (temp directory), same pattern as `claudeSettingsPathFunc` in claude_test.go.

  **Acceptance**: `make test` passes. All test cases listed above are covered.

---

- [x] **Task 3: Update documentation**

  **Files**: `README.md`, `docs/modules/reference/pages/configuration.adoc`

  **What to do**:

  1. In `README.md`, find the section listing supported agents (near "Agents: Claude Code, OpenCode") and add "Codex CLI" to the list.

  2. In `docs/modules/reference/pages/configuration.adoc`, add a section documenting the Codex hook configuration:
     - Location: `~/.codex/hooks.json`
     - Auto-configured by `cc-deck config plugin install`
     - Lists the registered events
     - Notes that Codex CLI v0.143+ is required for hooks support

  3. Use the prose plugin with the cc-deck voice profile for documentation content.

  **Acceptance**: `make verify` passes. README and config reference mention Codex CLI.
