# Tasks: Cross-Pane Agent Communication via MCP Server

**Input**: Design documents from `/specs/084-cross-pane-mcp/`

**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/mcp-tools.md

**Tests**: Tests included as part of implementation tasks (constitution requires tests for every feature).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Add MCP dependency

- [x] T001 Add `github.com/mark3labs/mcp-go` dependency to cc-deck/go.mod and run `go mod tidy`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend plugin state, hook payloads, and pipe message infrastructure that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T003 Add `topic: Option<String>` and `recent_tools: Vec<String>` fields (with `#[serde(default)]`) to Session struct in cc-zellij-plugin/src/session.rs, initialize in `Session::new()`
- [x] T004 Add `Prompt` field to `NormalizedPayload` struct in cc-deck/internal/agent/agent.go (json tag: `"prompt,omitempty"`)
- [x] T005 [P] Add `Prompt` field to `claudeHookPayload` struct in cc-deck/internal/agent/claude.go and update `TranslateEvent` to map it to `NormalizedPayload.Prompt`. **Depends on T004.**
- [x] T006 [P] Add `Prompt` field to `codexHookPayload` struct in cc-deck/internal/agent/codex.go and update `TranslateEvent` to map it to `NormalizedPayload.Prompt`. **Depends on T004.**
- [x] T006b [P] Add `Prompt` field to `opencodeHookPayload` struct in cc-deck/internal/agent/opencode.go and update `TranslateEvent` to map it to `NormalizedPayload.Prompt`. **Depends on T004.**
- [x] T007 Update `process_hook` in cc-zellij-plugin/src/controller/hooks.rs to store topic from hook payload prompt field (first ~100 chars) and append tool_name to recent_tools ring buffer (max 10)
- [x] T008 Add MCP pipe actions (`McpSessions`, `McpScrollback`, `McpState`, `McpInject`) to `PipeAction` enum in cc-zellij-plugin/src/pipe_handler.rs and update `parse_pipe_message`
- [x] T009 Create cc-zellij-plugin/src/controller/mcp_handlers.rs module with handler functions for each MCP pipe action, register module in cc-zellij-plugin/src/lib.rs
- [x] T010 Add MCP pipe action dispatch to controller `pipe()` method in cc-zellij-plugin/src/controller/mod.rs, routing to mcp_handlers
- [x] T011 Create `cc-deck/internal/mcp/` package and implement MCP server core in cc-deck/internal/mcp/server.go: create MCP server with stdio transport, register 4 tool schemas (cc_deck_sessions, cc_deck_read_scrollback, cc_deck_session_state, cc_deck_ask) with JSON Schema parameters per contracts/mcp-tools.md
- [x] T012 Implement pipe communication helpers in cc-deck/internal/mcp/pipe.go: send pipe messages to Zellij plugin via `zellij pipe` and read responses, with timeout handling
- [x] T013 Add `cc-deck mcp serve` subcommand in cc-deck/internal/cmd/mcp.go that starts the MCP server from T011

**Checkpoint**: Foundation ready. Plugin tracks topics and recent tools. MCP server starts and registers tools. Pipe communication works bidirectionally.

---

## Phase 3: User Story 1 - Discover Active Sessions (Priority: P1)

**Goal**: An agent can list all active sessions with rich metadata including name, agent type, state, CWD, branch, topic, project summary, recent activity, and modified files.

**Independent Test**: Run two agent sessions, call cc_deck_sessions from one, verify both appear with correct metadata.

- [x] T014 [US1] Implement `handle_mcp_sessions` in cc-zellij-plugin/src/controller/mcp_handlers.rs: serialize all sessions with pane_id, display_name, activity, working_dir, agent_name, topic, recent_tools, paused, badges via `cli_pipe_output`
- [x] T015 [US1] Implement session context aggregation in cc-deck/internal/mcp/context.go: read CLAUDE.md (first 500 chars), MEMORY.md entries, git branch, and modified files for a given CWD
- [x] T016 [US1] Implement `cc_deck_sessions` tool handler in cc-deck/internal/mcp/tools.go: call plugin via mcp-sessions pipe, enrich each session with context from T015, return JSON array per contract
- [x] T017 [US1] Add unit tests for session listing in cc-deck/internal/mcp/tools_test.go (context aggregation, empty sessions, JSON response shape)
- [x] T017b [US1] Add unit tests for `handle_mcp_sessions` serialization in cc-zellij-plugin/src/controller/mcp_handlers.rs (session serialization, empty session list, field coverage)

**Checkpoint**: Session listing works end-to-end. Agents can discover all active sessions with rich context.

---

## Phase 4: User Story 2 - Read Another Session's Scrollback (Priority: P1)

**Goal**: An agent can read the last N lines of another session's terminal scrollback.

**Independent Test**: Run a session that produces output, read its scrollback from another session, verify content matches.

- [x] T018 [US2] Implement `handle_mcp_scrollback` in cc-zellij-plugin/src/controller/mcp_handlers.rs: parse pane_id and lines from payload, call `get_pane_scrollback`, truncate to requested lines, return via `cli_pipe_output`
- [x] T019 [US2] Implement `cc_deck_read_scrollback` tool handler in cc-deck/internal/mcp/tools.go: resolve session name to pane_id, validate line count (1-500), call plugin via mcp-scrollback pipe, return text content
- [x] T020 [US2] Implement session name resolution in cc-deck/internal/mcp/tools.go: resolve display name to pane_id from session list, handle ambiguous names per FR-014 (return error listing matches with pane IDs)
- [x] T021 [US2] Add unit tests for scrollback reading in cc-deck/internal/mcp/tools_test.go (valid request, session not found, ambiguous name, invalid line count, max 500 cap)

**Checkpoint**: Scrollback reading works. Agents can observe other sessions' terminal output.

---

## Phase 5: User Story 3 - Ask Another Session a Question (Priority: P2)

**Goal**: An agent can inject a question into an idle target session and receive a response via file-based handshake.

**Independent Test**: Have one idle session receive a question from another, verify response is captured and returned.

- [x] T022 [US3] Implement `handle_mcp_inject` in cc-zellij-plugin/src/controller/mcp_handlers.rs: verify target session is idle, inject text via `write_chars_to_pane_id`, return ok/error status via `cli_pipe_output`
- [x] T023 [US3] Implement ask flow orchestration in cc-deck/internal/mcp/ask.go: create query temp file with UUID naming, construct injection prompt (question + response file path + instructions), send mcp-inject pipe message, poll for response file with configurable timeout, read response, clean up temp files
- [x] T024 [US3] Implement `cc_deck_ask` tool handler in cc-deck/internal/mcp/tools.go: resolve session name, delegate to ask.go orchestration, handle timeout/busy/error cases per contract
- [x] T025 [US3] Add unit tests for ask flow in cc-deck/internal/mcp/ask_test.go (successful handshake with mock files, timeout handling, busy session error, temp file cleanup on success and failure, concurrent request isolation via UUID)

**Checkpoint**: Ask flow works end-to-end. Agents can consult each other and receive responses.

---

## Phase 6: User Story 4 - Query Structured Session State (Priority: P2)

**Goal**: An agent can query detailed structured state about another session including git changes, recent tools, and activity.

**Independent Test**: Run a session that makes git changes, query its state from another session, verify structured response.

- [x] T026 [US4] Implement `handle_mcp_state` in cc-zellij-plugin/src/controller/mcp_handlers.rs: serialize detailed session state (activity timeline, full recent_tools with timestamps) via `cli_pipe_output`
- [x] T027 [US4] Implement `cc_deck_session_state` tool handler in cc-deck/internal/mcp/tools.go: resolve session name, call plugin via mcp-state pipe, enrich with git diff/log from CWD, return JSON per contract
- [x] T028 [US4] Add unit tests for session state in cc-deck/internal/mcp/tools_test.go (state with git changes, state without git, session not found)

**Checkpoint**: Structured state queries work. Agents can make informed decisions about consulting other sessions.

---

## Phase 7: User Story 5 - Automatic MCP Configuration (Priority: P3)

**Goal**: `cc-deck plugin install` automatically configures the MCP server in agent config files.

**Independent Test**: Run `cc-deck plugin install`, verify .mcp.json contains cc-deck MCP entry.

- [x] T029 [US5] Extend Claude agent's `InstallHooks` in cc-deck/internal/agent/claude.go to add MCP server entry to `.mcp.json` (command: "cc-deck", args: ["mcp", "serve"]), handle create/update/idempotent cases
- [x] T030 [P] [US5] Extend Codex agent's `InstallHooks` in cc-deck/internal/agent/codex.go to add MCP server entry to its configuration file
- [x] T031 [US5] Add unit tests for MCP config installation in cc-deck/internal/agent/claude_test.go and codex_test.go (fresh install, existing config preserved, idempotent update)

**Checkpoint**: Plugin install automatically enables cross-pane tools for all supported agents.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, error handling hardening, and cleanup

- [x] T032 [P] Add user guide at docs/modules/guides/pages/cross-pane-communication.adoc covering all 4 MCP tools with examples
- [x] T033 [P] Update CLI reference at docs/modules/reference/pages/cli.adoc with `mcp serve` subcommand documentation
- [x] T034 Update README.md with cross-pane communication feature summary
- [x] T035 Add error handling for plugin-not-running scenario across all tool handlers in cc-deck/internal/mcp/tools.go (detect when zellij pipe fails)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational
- **US2 (Phase 4)**: Depends on Foundational. Uses session name resolution from US1 (T020)
- **US3 (Phase 5)**: Depends on Foundational. Uses session name resolution from US1 (T020)
- **US4 (Phase 6)**: Depends on Foundational. Uses session name resolution from US1 (T020)
- **US5 (Phase 7)**: Depends on MCP server core (T011, T013) only
- **Polish (Phase 8)**: Depends on all user stories being complete

### Within Each User Story

- Plugin handler before Go tool handler
- Tool handler before tests
- Session name resolution (T020) shared across US2, US3, US4

### Parallel Opportunities

- T005, T006, T006b (agent-specific Prompt field) can run in parallel after T004 (NormalizedPayload.Prompt)
- T029, T030 (MCP config for different agents) can run in parallel
- T032, T033 (documentation files) can run in parallel
- US1 and US5 can run in parallel after Foundational completes
- US3 and US4 can run in parallel after US1's T020 (name resolution) completes

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: US1 (session listing)
4. Complete Phase 4: US2 (scrollback reading)
5. **STOP and VALIDATE**: Agents can discover sessions and read scrollback

### Incremental Delivery

1. Setup + Foundational -> Infrastructure ready
2. US1 (session listing) -> Agents have cross-session awareness
3. US2 (scrollback reading) -> Agents can observe other sessions
4. US3 (ask) -> Agents can consult each other (core differentiator)
5. US4 (structured state) -> Richer programmatic queries
6. US5 (auto-config) -> Frictionless setup
7. Polish -> Documentation and hardening

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Session name resolution (T020) is a shared utility used by US2, US3, US4
- Constitution requires tests and documentation with every feature
- Use `make test` and `make lint`, never `go build` or `cargo build` directly
