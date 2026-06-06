# Tasks: Agent Abstraction Layer

**Input**: Design documents from `specs/066-agent-abstraction/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/agent-interface.md

**Tests**: Included per spec requirements (SC-001, SC-005, SC-008).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the Agent interface, registry, and package structure

- [x] T001 Create `cc-deck/internal/agent/agent.go` with the `Agent` interface (Name, DisplayName, Indicator, IsInstalled, DetectConfig, InstallHooks, UninstallHooks, HooksInstalled, TranslateEvent), `NormalizedPayload` struct (with `agent` field), and registry (Register, Get, All) per `contracts/agent-interface.md`
- [x] T002 Create `cc-deck/internal/agent/registry_test.go` with tests for Register (success, duplicate name panic, duplicate indicator panic), Get (found, not found), and All (stable ordering)

**Checkpoint**: Agent interface and registry compile and tests pass

---

## Phase 2: Foundational (ClaudeAgent Adapter)

**Purpose**: Extract all Claude Code-specific logic into the ClaudeAgent adapter. This is the blocking prerequisite: all existing functionality must work through the adapter before other stories can proceed.

**CRITICAL**: No user story work can begin until this phase is complete and SC-001 (zero regressions) is confirmed.

- [x] T003 Create `cc-deck/internal/agent/claude.go` implementing the Agent interface by extracting logic from `cc-deck/internal/plugin/hooks.go`: move `ClaudeSettingsPath()` to `DetectConfig()`, `RegisterHooks()` to `InstallHooks()`, `RemoveHooks()` to `UninstallHooks()`, `HasHooks()` to `HooksInstalled()`. Set Name="claude", DisplayName="Claude Code", Indicator="CC". Implement `IsInstalled()` via `exec.LookPath("claude")`. Implement `TranslateEvent()` to parse the existing `hookPayload` struct and produce `NormalizedPayload` with `agent: "claude"`
- [x] T004 [P] Create `cc-deck/internal/agent/claude_test.go` with unit tests per contract B1-B8: identity values, IsInstalled with mock PATH, InstallHooks idempotency (mock filesystem), UninstallHooks safety, TranslateEvent for all 11 event types, TranslateEvent error for malformed input
- [x] T005 Refactor `cc-deck/internal/cmd/hook.go`: add `--agent` flag (default "claude"), look up agent in registry, call `TranslateEvent()`, then proceed with existing pane ID resolution and pipe forwarding. Keep `--pane-id` flag unchanged. Ensure existing hook behavior is byte-identical for Claude Code payloads (FR-016)
- [x] T006 Refactor `cc-deck/internal/plugin/hooks.go`: replace direct Claude Code logic with calls through `agent.Get("claude")`. The file becomes a thin delegation layer. Remove `ClaudeSettingsPath()` (moved to adapter)
- [x] T007 Refactor `cc-deck/internal/plugin/install.go`: replace hardcoded Claude hook registration with iteration over `agent.All()`, calling `IsInstalled()` and `InstallHooks()` on each detected agent
- [x] T008 Add `agent.Register()` call in `cc-deck/internal/agent/claude.go` init function and ensure `cc-deck/cmd/cc-deck/main.go` imports the agent package (blank import `_ "internal/agent"`)
- [x] T009 Run all existing tests (`make test`) and verify zero regressions (SC-001). Fix any failures introduced by the refactoring

**Checkpoint**: All existing Claude Code hook functionality works identically through the Agent interface. `make test` and `make lint` pass. SC-001 confirmed.

---

## Phase 3: User Story 3 - Hook Events from Claude Code (Priority: P1) MVP

**Goal**: Confirm that the refactored hook path produces identical behavior for all Claude Code events.

**Independent Test**: Run existing hook integration tests; pipe a Claude Code hook payload through `cc-deck hook --agent claude` and verify the pipe message matches pre-refactoring output.

- [x] T010 [US3] Verify `cc-deck hook --agent claude` accepts all 11 event types (SessionStart, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStart, SubagentStop, SessionEnd) and produces identical pipe messages to the pre-refactoring `cc-deck hook` command
- [x] T011 [US3] Verify backward compatibility: `cc-deck hook` without `--agent` flag defaults to "claude" and works identically to before (FR-009)
- [x] T012 [US3] Verify SC-007: grep Go source outside `internal/agent/` for Claude-specific constants (event names, `.claude/settings.json`, credential env vars). All references must go through the Agent interface

**Checkpoint**: US3 acceptance scenarios 1-4 all pass. Zero regressions confirmed.

---

## Phase 4: User Story 1 - Install Plugin with Multiple Agents (Priority: P1)

**Goal**: `cc-deck plugin install` auto-detects all installed agents and installs hooks for each.

**Independent Test**: With both `claude` and `opencode` binaries available, run `cc-deck plugin install` and verify both are reported as configured.

- [x] T013 [US1] Refactor `cc-deck/internal/cmd/plugin.go`: update `plugin install` to iterate `agent.All()`, call `IsInstalled()` on each, call `InstallHooks()` on detected agents, and report results. Add `--agents` flag for filtering by name (FR-008)
- [x] T014 [US1] Implement `cc-deck plugin uninstall` subcommand in `cc-deck/internal/cmd/plugin.go`: iterate agents, call `UninstallHooks()` on each (or filtered by `--agents`). Report results
- [x] T015 [US1] Implement `cc-deck plugin status` subcommand in `cc-deck/internal/cmd/plugin.go`: iterate `agent.All()`, show detection state (`IsInstalled()`), hook state (`HooksInstalled()`), and config path (`DetectConfig()`) for each agent
- [x] T016 [US1] Add acceptance test: verify `plugin install` with only Claude Code installed sets up hooks and reports success; verify with no agents installed prints warning

**Checkpoint**: US1 acceptance scenarios 1-4 all pass. `cc-deck plugin install/status/uninstall` work for Claude Code.

---

## Phase 5: User Story 5 - Raw Hook Payload Ingestion (Priority: P2)

**Goal**: `cc-deck hook --raw` accepts pre-normalized JSON payloads and forwards to the Zellij plugin.

**Independent Test**: Pipe a normalized JSON payload to `cc-deck hook --raw` and verify it reaches the plugin as a pipe message.

- [x] T017 [US5] Create `cc-deck/internal/cmd/hook_raw.go`: implement `--raw` flag handler that reads stdin, validates required `event` field, validates JSON structure, and forwards directly to Zellij pipe (skipping TranslateEvent). Add error handling per spec Error Handling section (malformed JSON → stderr + non-zero exit)
- [x] T018 [US5] Add unit tests for `--raw` handler: valid payload forwarded, missing `event` field rejected, malformed JSON rejected, unknown agent name accepted with generic indicator
- [x] T019 [US5] Verify Zellij pipe failure handling: when Zellij is not running or the plugin is not loaded, `cc-deck hook --raw` and `cc-deck hook --agent claude` MUST print a warning to stderr and exit with non-zero status. Test by running the hook command outside a Zellij session

**Checkpoint**: US5 acceptance scenarios 1-3 all pass.

---

## Phase 6: User Story 2 - Sidebar Shows Agent Identity (Priority: P1)

**Goal**: Sidebar displays agent indicators ([CC], [OC]) when multiple agent types are active.

**Independent Test**: Send hook events with different `agent` values and verify the sidebar shows distinct indicators.

- [x] T020 [P] [US2] Add `agent: Option<String>` field to `HookPayload` in `cc-zellij-plugin/src/pipe_handler.rs` with `#[serde(default)]` for backward compatibility
- [x] T021 [P] [US2] Add `agent_name: Option<String>` field to `Session` in `cc-zellij-plugin/src/session.rs`. Set it from the first hook event's `agent` field in `cc-zellij-plugin/src/controller/hooks.rs`
- [x] T022 [P] [US2] Add `agent_indicator: Option<String>` field to `RenderSession` in `cc-zellij-plugin/src/lib.rs`. Add `show_agent_indicators: bool` field to `RenderPayload`
- [x] T023 [US2] Update `cc-zellij-plugin/src/controller/render_broadcast.rs`: when building `RenderPayload`, count distinct `agent_name` values across sessions. Set `show_agent_indicators = true` when count > 1. Populate `agent_indicator` from session's `agent_name` (map "claude" to "CC", "opencode" to "OC", unknown to first 2 chars uppercased)
- [x] T024 [US2] Update `cc-zellij-plugin/src/sidebar_plugin/render.rs`: when `show_agent_indicators` is true, prepend `[CC]` or `[OC]` (from `agent_indicator`) before session display name on line 1 of each session entry. When false, render as before
- [x] T025 [US2] Replace 3 hardcoded "Claude Code" strings in `cc-zellij-plugin/src/sidebar_plugin/render.rs` (lines ~225, ~244, ~288 in `render_loading`, `render_permission_prompt`, `render_header`) with agent-agnostic text (e.g., "cc-deck") per FR-014
- [x] T026 [US2] Add Rust unit tests: verify HookPayload deserialization with and without `agent` field; verify indicator display logic (show when >1 type, hide when all same)

**Checkpoint**: US2 acceptance scenarios 1-4 all pass. `cargo test` passes.

---

## Phase 7: User Story 4 - OpenCode Plugin Integration (Priority: P2)

**Goal**: OpenCodeAgent adapter generates a TypeScript plugin for OpenCode and translates its lifecycle events.

**Independent Test**: Verify generated plugin file exists in `~/.config/opencode/plugins/` and contains handlers for all mapped events.

- [x] T027 [P] [US4] Create `cc-deck/internal/agent/opencode_plugin.ts` (the TypeScript plugin template) embedded via `go:embed`. The plugin must: import from `@opencode-ai/plugin`, export a `server` function, use `event` hook to filter `session.next.step.started` and `session.next.step.ended`, use `tool.execute.before`/`tool.execute.after` hooks, use `permission.ask` hook. Each handler calls `cc-deck hook --agent opencode` via Bun `$` with JSON on stdin. Handle errors silently (log to stderr, do not crash OpenCode)
- [x] T028 [US4] Create `cc-deck/internal/agent/opencode.go` implementing the Agent interface: Name="opencode", DisplayName="OpenCode", Indicator="OC". `IsInstalled()` via `exec.LookPath("opencode")`. `DetectConfig()` returns `~/.config/opencode/` via `internal/xdg`. `InstallHooks()` writes embedded template to `~/.config/opencode/plugins/cc-deck.ts`. `UninstallHooks()` deletes the file. `HooksInstalled()` checks file existence. `TranslateEvent()` maps OpenCode event names to normalized events per data-model.md event mapping table
- [x] T029 [US4] Add `agent.Register()` call in `opencode.go` init function. Ensure blank import in `cmd/cc-deck/main.go`
- [x] T030 [P] [US4] Create `cc-deck/internal/agent/opencode_test.go` with unit tests: identity values, IsInstalled with mock PATH, InstallHooks creates plugin file with correct content (SC-008), UninstallHooks removes file, HooksInstalled detects presence/absence, TranslateEvent maps all 5 OpenCode event types correctly, TranslateEvent returns error for malformed input
- [x] T031 [US4] Verify end-to-end: `cc-deck plugin install` with OpenCode in PATH creates plugin file; `cc-deck plugin status` shows OpenCode as detected and hooked; `cc-deck plugin uninstall --agents opencode` removes the plugin file

**Checkpoint**: US4 acceptance scenarios 1-4 all pass. SC-008 confirmed.

---

## Phase 8: User Story 6 - Plugin Status and Selective Management (Priority: P2)

**Goal**: Users can inspect and selectively manage agent hooks via `plugin status/install/uninstall --agents`.

**Independent Test**: Install hooks, run `cc-deck plugin status`, verify output lists agents with correct state.

- [x] T032 [US6] Verify `cc-deck plugin status` output distinguishes: Claude Code (hooked, config path), OpenCode (hooked, plugin path), agents not installed (not detected). Format output as a table or structured list
- [x] T033 [US6] Verify `cc-deck plugin install --agents opencode` installs only the OpenCode plugin without touching Claude Code hooks
- [x] T034 [US6] Verify `cc-deck plugin uninstall --agents opencode` removes only the OpenCode plugin. Verify `cc-deck plugin uninstall` (no filter) removes all hooks

**Checkpoint**: US6 acceptance scenarios 1-3 all pass.

---

## Phase 9: Polish and Cross-Cutting Concerns

**Purpose**: CLI text cleanup, documentation, and final validation

- [x] T035 [P] Update Go CLI help text and command descriptions: grep for "Claude Code" in `cc-deck/internal/cmd/` and `cc-deck/cmd/cc-deck/main.go`, replace with agent-agnostic language per FR-015 (e.g., "Manage Claude Code workspaces" becomes "Manage AI agent workspaces")
- [x] T036 [P] Update `README.md` to describe multi-agent support, the Agent interface concept, and the `--agent` flag
- [x] T037 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc`: document `cc-deck hook --agent`, `cc-deck hook --raw`, `cc-deck plugin status`, `cc-deck plugin uninstall`, and the `--agents` filter flag
- [x] T038 [P] Create multi-agent setup guide as Antora page in `docs/modules/guides/pages/`: cover installing hooks for multiple agents, verifying with `plugin status`, running OpenCode with the cc-deck plugin
- [x] T039 [P] Update configuration reference in `docs/modules/reference/pages/configuration.adoc`: document the generated OpenCode plugin file location (`~/.config/opencode/plugins/cc-deck.ts`) and format
- [x] T040 Run `make test` and `make lint` across both Go and Rust projects. Verify all success criteria SC-001 through SC-008
- [x] T041 Run quickstart.md validation: execute all test commands listed in quickstart.md and verify expected outcomes

**Checkpoint**: All documentation updated per constitution Principle I. All tests pass. All success criteria met.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1. BLOCKS all user stories
- **Phase 3 (US3 - Hook Events)**: Depends on Phase 2. Validates the refactoring before proceeding
- **Phase 4 (US1 - Multi-Agent Install)**: Depends on Phase 2
- **Phase 5 (US5 - Raw Hook)**: Depends on Phase 2
- **Phase 6 (US2 - Sidebar Indicators)**: Independent of Go phases. Can start after Phase 1 (only needs Rust changes)
- **Phase 7 (US4 - OpenCode Adapter)**: Depends on Phase 2
- **Phase 8 (US6 - Plugin Management)**: Depends on Phase 4 and Phase 7
- **Phase 9 (Polish)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US3 (Hook Events)**: Must complete first to confirm zero regressions
- **US1 (Multi-Agent Install)**: Can start after US3 confirmed
- **US5 (Raw Hook)**: Independent of other stories after Phase 2
- **US2 (Sidebar Indicators)**: Independent (Rust-only), can be developed in parallel with Go stories
- **US4 (OpenCode Adapter)**: Independent after Phase 2
- **US6 (Plugin Management)**: Depends on US1 and US4 (needs both agents available)

### Parallel Opportunities

- **Phase 2**: T003 and T004 can run in parallel (implementation vs tests)
- **Phase 6 (Rust)**: T020, T021, T022 can all run in parallel (different files)
- **Phase 6 + Phase 4/5/7**: Rust work (Phase 6) is fully independent of Go work (Phases 4, 5, 7) and can run in parallel
- **Phase 7**: T027 and T030 can run in parallel (template vs tests)
- **Phase 9**: T035, T036, T037, T038, T039 can all run in parallel (different files)

---

## Parallel Example: Phase 6 (Rust Plugin)

```bash
# Launch all independent Rust modifications together:
Task: "Add agent field to HookPayload in cc-zellij-plugin/src/pipe_handler.rs"
Task: "Add agent_name field to Session in cc-zellij-plugin/src/session.rs"
Task: "Add agent_indicator to RenderSession in cc-zellij-plugin/src/lib.rs"
```

## Parallel Example: Go + Rust

```bash
# After Phase 2, Go and Rust tracks can run in parallel:
# Track A (Go): Phase 4 (US1) → Phase 5 (US5) → Phase 7 (US4) → Phase 8 (US6)
# Track B (Rust): Phase 6 (US2) - fully independent
```

---

## Implementation Strategy

### MVP First (US3 Only)

1. Complete Phase 1: Setup (interface + registry)
2. Complete Phase 2: Foundational (ClaudeAgent adapter)
3. Complete Phase 3: US3 (verify zero regressions)
4. **STOP and VALIDATE**: `make test` passes, all existing behavior preserved
5. This is the minimum viable refactoring

### Incremental Delivery

1. Setup + Foundational + US3 → Zero-regression refactoring confirmed (MVP)
2. Add US1 (multi-agent install) → `plugin install` works for multiple agents
3. Add US5 (raw hook) → External integrations possible
4. Add US2 (sidebar indicators) → Visual multi-agent support (can parallel with 2-3)
5. Add US4 (OpenCode adapter) → First non-Claude agent
6. Add US6 (plugin management) → Full management UI
7. Polish → Documentation complete

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Constitution Principle I: documentation tasks in Phase 9 are mandatory, not optional
- Constitution Principle III: all builds via `make install`/`make test`/`make lint`
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
