# Tasks: Pipe Mux Broker

**Input**: Design documents from `specs/086-pipe-mux-broker/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: XDG path extension and project scaffolding

- [ ] T001 Add `RuntimeDir` to `internal/xdg/xdg.go` resolving `$XDG_RUNTIME_DIR` with fallback to `/tmp/cc-deck-<uid>/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Message protocol, config struct, and command wiring that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 [P] Create `internal/mux/message.go` with `Message` struct (session_name, pipe_name, args), JSON serialization, `DedupKey()` method using sha256 truncated to 16 hex chars, and `message_test.go` with unit tests
- [ ] T003 [P] Add `MuxConfig` struct to `internal/config/config.go` with fields: enabled (bool), flush_interval (time.Duration), idle_timeout (time.Duration), queue_size (int), dedup (*bool), log (bool). Add `Mux MuxConfig` field to `Config` struct. Add defaults function returning enabled=false, flush_interval=200ms, idle_timeout=30s, queue_size=1000, dedup=true, log=false
- [ ] T004 Create `internal/cmd/mux.go` with cobra `NewMuxCmd` returning a command that starts the broker. Register it in `cmd/cc-deck/main.go` as a top-level command

**Checkpoint**: Foundation ready, message types and config available for broker implementation

---

## Phase 3: User Story 1+5 - Core Broker with Self-Termination (Priority: P1+P2)

**Goal**: A standalone daemon that accepts pipe messages on a Unix domain socket, deduplicates by (session, pipe_name, hash(args)), flushes to `zellij pipe` every 200ms, and self-terminates after 30s idle

**Independent Test**: Start `cc-deck mux` manually, send messages via `nc -U <socket>`, verify they appear as `zellij pipe` calls. Wait 35s idle, verify process exits and socket is removed.

### Implementation

- [ ] T005 [US1] Create `internal/mux/broker.go` with `Broker` struct: Unix socket listener via `net.ListenUnix` (SOCK_STREAM), accept loop goroutine reading newline-delimited JSON. Two queue modes controlled by `config.Dedup`: when true, use dedup map (`map[string]Message` with mutex, last-writer-wins), atomic map swap on flush (replace with empty map, iterate old); when false (FR-014), use `[]Message` slice preserving arrival order, swap with empty slice on flush. Flush timer (200ms ticker) iterates the drained collection and calls `zellij pipe --session <session_name> --name <pipe_name> -- <args>` sequentially. If a `zellij pipe` call fails during flush, log the error (via logger, if enabled) and discard the message (no retry, per spec edge case)
- [ ] T006 [US5] Add idle timer to `Broker` in `internal/mux/broker.go`: reset on every incoming message, fire shutdown on expiry (default 30s). Add signal handling (`SIGTERM`, `SIGINT`) triggering clean shutdown. Shutdown sequence: stop accept loop, final flush of remaining messages, close and remove socket file, exit
- [ ] T007 [US1] Create `internal/mux/client.go` with `Send(socketPath, msg Message) error` function: dial Unix socket, write JSON + newline, close connection. Fire-and-forget, no response expected. Return error if connect or write fails
- [ ] T008 [US1] Wire `internal/cmd/mux.go` to create `Broker` from `MuxConfig`, resolve socket path via `xdg.RuntimeDir + "/cc-deck/mux.sock"`, create socket directory if needed, call `broker.Run()` which blocks until shutdown
- [ ] T009 [P] [US1] Create `internal/mux/broker_test.go` with unit tests: broker starts and accepts connections, dedup collapses identical messages, flush delivers all unique messages, queue_size limit drops oldest, map swap is atomic (no lost messages during concurrent write+flush), dedup=false preserves all messages in arrival order, failed zellij pipe call during flush is logged and message discarded (no retry)
- [ ] T010 [P] [US1] Create `internal/mux/client_test.go` with unit tests: successful send to running broker, send failure returns error when no broker, connection close after send

**Checkpoint**: Broker runs standalone, accepts messages, deduplicates, flushes, and self-terminates

---

## Phase 4: User Story 2 - Hook Integration and Graceful Fallback (Priority: P1)

**Goal**: Hooks route pipe messages through the broker when enabled, falling back to direct `zellij pipe` on failure. No message lost due to broker unavailability.

**Independent Test**: Enable mux, kill broker mid-session, verify hooks fall back to direct pipe without errors.

### Implementation

- [ ] T011 [US2] Modify `internal/cmd/hook.go` at the pipe delivery point (around line 151): before calling `exec.CommandContext(ctx, zellijPath, "pipe", ...)`, check `config.Mux.Enabled` and `$ZELLIJ_SESSION_NAME`. If both set, construct a `mux.Message{SessionName: sessionName, PipeName: "cc-deck:hook", Args: string(payloadJSON)}` and call `mux.SendOrStart(socketPath, msg)`. If it returns error, fall through to existing direct `zellij pipe` call
- [ ] T011b [US2] Modify `internal/cmd/hook_raw.go` at the pipe delivery point (around line 52): apply the same mux routing as T011. Before calling `exec.CommandContext(ctx, zellijPath, "pipe", ...)`, check `config.Mux.Enabled` and `$ZELLIJ_SESSION_NAME`. If both set, construct a `mux.Message` and call `mux.SendOrStart(socketPath, msg)`. If it returns error, fall through to existing direct `zellij pipe` call. Load config via `config.Load("")` (same pattern as `runHook`)
- [ ] T012 [US2] Add `SendOrStart(socketPath string, msg Message) error` to `internal/mux/client.go`: try `Send()`, if fails check for stale socket (file exists but connect refused), remove stale socket, spawn `cc-deck mux` as detached process (`exec.Cmd` with `SysProcAttr{Setsid: true}`, `cmd.Start()` + `cmd.Process.Release()`), retry `Send()` 3 times with 10ms sleep between attempts, return error if all retries fail
- [ ] T013 [US2] Add stale socket detection to `internal/mux/client.go`: if `os.Stat(socketPath)` succeeds but `net.DialUnix` fails, call `os.Remove(socketPath)` before starting fresh broker
- [ ] T014 [P] [US2] Add tests to `internal/mux/client_test.go`: SendOrStart starts broker when socket missing, SendOrStart cleans stale socket and starts fresh broker, SendOrStart falls back after 3 failed retries

**Checkpoint**: Hooks transparently route through broker with fallback to direct pipe

---

## Phase 5: User Story 3 - Configuration Validation (Priority: P2)

**Goal**: Config validation catches invalid mux parameters, defaults applied correctly for missing values

**Independent Test**: Add invalid `mux.flush_interval: -1s` to config, run `cc-deck config check`, verify validation error reported.

### Implementation

- [ ] T015 [US3] Add mux validation to `internal/config/validate.go`: check flush_interval > 0, idle_timeout > 0, queue_size > 0, warn if enabled but flush_interval > 1s. Add `CategoryMux` constant for validation findings
- [ ] T016 [P] [US3] Add unit tests for mux config validation in `internal/config/validate_test.go`: valid config passes, negative flush_interval fails, zero queue_size fails, missing mux section uses defaults

**Checkpoint**: Config validation catches all invalid mux parameters

---

## Phase 6: User Story 4 - Debug Logging (Priority: P3)

**Goal**: Optional logging of all events flowing through the broker for debugging and tuning

**Independent Test**: Set `mux.log: true`, run broker, send messages, verify `~/.local/state/cc-deck/mux.log` contains incoming, dedup, flush, and lifecycle entries.

### Implementation

- [ ] T017 [US4] Create `internal/mux/log.go` with `Logger` interface and two implementations: `fileLogger` (writes to `xdg.StateHome + "/cc-deck/mux.log"`, truncates on open, timestamped entries) and `noopLogger` (no-op when logging disabled). Log methods: `Incoming(msg)`, `Dedup(key, count)`, `Flush(count, duration)`, `Drop(msg)`, `Lifecycle(event)`
- [ ] T018 [US4] Integrate logger into `Broker` in `internal/mux/broker.go`: call `logger.Incoming()` on accept, `logger.Dedup()` when map key already exists, `logger.Flush()` after each flush cycle with message count and duration, `logger.Drop()` when queue_size exceeded, `logger.Lifecycle()` on start, idle timeout, signal shutdown
- [ ] T019 [P] [US4] Add unit tests in `internal/mux/log_test.go`: fileLogger writes to expected path, noopLogger produces no output, log entries contain timestamp and event type

**Checkpoint**: Debug logging captures all broker events when enabled

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration testing, documentation, and final validation

- [ ] T020 Create `test/mux_integration_test.go`: start broker in-process, send 100 messages from 5 concurrent goroutines across 3 session names, verify correct dedup behavior, verify flush delivers to mock zellij pipe, verify idle timeout shutdown, verify stale socket recovery
- [ ] T021 Update CLI reference documentation in `docs/modules/reference/pages/cli.adoc` with `cc-deck mux` command description
- [ ] T022 Update configuration reference in `docs/modules/reference/pages/configuration.adoc` with `mux` config section and all parameters
- [ ] T023 Run `make verify` to ensure all tests pass and linting is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001 for XDG RuntimeDir)
- **US1+US5 (Phase 3)**: Depends on Phase 2 (message types, config, command wiring)
- **US2 (Phase 4)**: Depends on Phase 3 (broker and client must exist for hook integration)
- **US3 (Phase 5)**: Depends on Phase 2 (config struct), can run in parallel with Phase 3/4
- **US4 (Phase 6)**: Depends on Phase 3 (broker must exist to integrate logging)
- **Polish (Phase 7)**: Depends on all user stories

### User Story Dependencies

- **US1+US5 (Broker)**: Can start after Foundational - no dependencies on other stories
- **US2 (Fallback)**: Depends on US1 (broker must exist to integrate with hooks)
- **US3 (Config Validation)**: Independent of US1/US2, only needs foundational config struct
- **US4 (Logging)**: Depends on US1 (logger integrates into broker)

### Parallel Opportunities

- T002 and T003 can run in parallel (message types and config are independent files)
- T009 and T010 can run in parallel (broker tests and client tests are independent)
- T015 and T016 can run in parallel with Phase 3/4 work
- T019 can run in parallel with T017/T018

---

## Parallel Example: Phase 2

```
# Launch foundational tasks together:
Task T002: "Create internal/mux/message.go with Message struct and tests"
Task T003: "Add MuxConfig struct to internal/config/config.go"
```

## Parallel Example: Phase 3

```
# After T005-T008 complete, launch test tasks together:
Task T009: "Create internal/mux/broker_test.go"
Task T010: "Create internal/mux/client_test.go"
```

---

## Implementation Strategy

### MVP First (US1+US5 Only)

1. Complete Phase 1: Setup (XDG RuntimeDir)
2. Complete Phase 2: Foundational (message, config, command)
3. Complete Phase 3: Broker with self-termination
4. **STOP and VALIDATE**: Test broker standalone with manual socket writes
5. Manually test with `cc-deck hook` (before hook integration)

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1+US5 (Broker) -> Test standalone -> MVP daemon works
3. Add US2 (Hook Integration) -> Test with real hooks -> End-to-end working
4. Add US3 (Config Validation) -> Test config check -> Production-ready config
5. Add US4 (Debug Logging) -> Test with logging -> Operational observability
6. Polish -> Integration tests, docs -> Ship-ready

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US5 are combined in Phase 3 because self-termination is integral to the broker lifecycle
- Tests are included inline with implementation (Go convention: *_test.go alongside source)
- Commit after each task or logical group
