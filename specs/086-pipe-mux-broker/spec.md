# Feature Specification: Pipe Mux Broker

**Feature Branch**: `086-pipe-mux-broker`
**Created**: 2026-08-04
**Status**: Draft
**Input**: Brainstorm 087 revisit (2026-08-04): cc-deck mux architecture decisions

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Transparent Pipe Storm Prevention (Priority: P1)

A developer runs multiple Claude Code sessions in parallel Zellij panes. Each session fires hooks that send pipe messages to the cc-deck plugin. Without the mux broker, these concurrent pipe calls overwhelm the Zellij server, causing thousands of CliPipe timeouts and driving CPU to 187%. With the broker enabled, all hook pipe messages route through a single daemon that deduplicates and rate-limits delivery, keeping the Zellij server responsive.

**Why this priority**: This is the core problem. Without solving it, multi-session workflows become unusable due to Zellij server CPU saturation and timeout spam.

**Independent Test**: Enable `mux.enabled: true` in config, run 5+ Claude Code sessions with active hooks, verify zero CliPipe timeouts in the Zellij server log over a 30-minute window.

**Acceptance Scenarios**:

1. **Given** mux is enabled and the broker is not running, **When** a hook fires for the first time, **Then** the hook starts the broker as a background process and delivers the message within 50ms total.
2. **Given** the broker is running, **When** 5 hooks fire simultaneously for the same session, **Then** the broker deduplicates identical messages and delivers at most one per unique (session, pipe_name, args) combination per flush cycle.
3. **Given** the broker is running, **When** hooks across 3 different Zellij sessions send messages concurrently, **Then** each message is routed to the correct session via `zellij pipe --session <name>`.

---

### User Story 2 - Graceful Fallback (Priority: P1)

A developer has the mux enabled but the broker crashes or fails to start. The hook detects the failure and falls back to calling `zellij pipe` directly, preserving current behavior. No message is lost due to broker unavailability.

**Why this priority**: The broker must never make things worse than the current direct-pipe approach. Fallback is a safety requirement, not a nice-to-have.

**Independent Test**: Kill the broker process while hooks are firing. Verify subsequent hooks fall back to direct pipe calls and messages still reach the plugin.

**Acceptance Scenarios**:

1. **Given** the broker is not running and the socket file does not exist, **When** a hook fires, **Then** the hook attempts to start the broker, and if startup fails, falls back to direct `zellij pipe` within 50ms.
2. **Given** the broker crashed leaving a stale socket file, **When** a hook fires and `connect()` fails, **Then** the hook removes the stale socket, attempts to start a fresh broker, and if that also fails, falls back to direct `zellij pipe`.
3. **Given** mux is disabled in config (`mux.enabled: false`), **When** a hook fires, **Then** the hook calls `zellij pipe` directly without attempting any broker interaction.

---

### User Story 3 - Opt-In Configuration (Priority: P2)

A developer enables or disables the mux broker and tunes its parameters through the existing cc-deck config file. All operational parameters (flush interval, idle timeout, queue size, dedup toggle) are configurable without recompiling.

**Why this priority**: Configuration allows gradual rollout and per-environment tuning. Less critical than the core broker but required for production readiness.

**Independent Test**: Change `mux.flush_interval` in config, restart the broker, verify the new interval takes effect by observing flush timing in debug logs.

**Acceptance Scenarios**:

1. **Given** no mux section in config.yaml, **When** a hook fires, **Then** the hook calls `zellij pipe` directly (mux defaults to disabled).
2. **Given** `mux.enabled: true` and `mux.flush_interval: 500ms` in config, **When** the broker starts, **Then** it flushes the queue every 500ms instead of the default 200ms.
3. **Given** `mux.queue_size: 50` in config, **When** 60 unique messages arrive before a flush, **Then** the oldest 10 are dropped and the broker logs a drop event.

---

### User Story 4 - Debug Logging (Priority: P3)

A developer enables debug logging to inspect which pipe messages flow through the broker, which get deduplicated, and which get dropped. This supports troubleshooting and tuning.

**Why this priority**: Logging is an operational aid. The broker works without it, but it is valuable for debugging and validating behavior during rollout.

**Independent Test**: Set `mux.log: true`, run several hooks, verify the log file at `~/.local/state/cc-deck/mux.log` contains entries for incoming messages, dedup hits, and flush events.

**Acceptance Scenarios**:

1. **Given** `mux.log: true` in config, **When** the broker receives a message, **Then** it appends a log entry with timestamp, session name, pipe name, and args summary.
2. **Given** `mux.log: true` and two identical messages arrive within the same flush window, **When** the broker deduplicates them, **Then** it logs a dedup event identifying the collapsed key.
3. **Given** `mux.log: false` (or absent) in config, **When** the broker runs, **Then** no log file is created or written to.

---

### User Story 5 - Broker Self-Termination (Priority: P2)

The broker process shuts itself down after a configurable idle period (default 30 seconds) of receiving no messages. No orphaned daemon processes remain after all Zellij sessions end.

**Why this priority**: Without self-termination, the broker would become a zombie process. This is a lifecycle hygiene requirement.

**Independent Test**: Start the broker, wait 35 seconds without sending messages, verify the process has exited and the socket file has been removed.

**Acceptance Scenarios**:

1. **Given** the broker is running with default 30s idle timeout, **When** no messages arrive for 30 seconds, **Then** the broker closes the socket, removes the socket file, and exits cleanly.
2. **Given** the broker idle timer is at 25 seconds, **When** a new message arrives, **Then** the idle timer resets to 0 and the broker continues running.
3. **Given** the broker self-terminated, **When** the next hook fires, **Then** a new broker instance starts via the normal lazy startup flow.

---

### Edge Cases

- What happens when the socket directory (`$XDG_RUNTIME_DIR/cc-deck/`) does not exist? The broker creates it on startup.
- What happens when `$XDG_RUNTIME_DIR` is not set? Fall back to `/tmp/cc-deck-$UID/`.
- What happens when two hooks race to start the broker simultaneously? The socket `bind()` is atomic: one wins, the other gets `EADDRINUSE` and exits. The losing hook retries `connect()`.
- What happens when a message targets a Zellij session that no longer exists? The `zellij pipe --session` call fails. The broker logs the error (if logging enabled) and discards the message. No retry.
- What happens when the config file is malformed? The broker uses compiled defaults for any value it cannot parse.
- What happens when the queue is full and a high-value message arrives? The oldest message is dropped regardless of content. No priority classification in V1.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `cc-deck mux` subcommand that runs as a standalone daemon process accepting pipe messages on a Unix domain socket.
- **FR-002**: System MUST deduplicate messages using the key `(session_name, pipe_name, hash(args))` with last-writer-wins semantics within each flush window.
- **FR-003**: System MUST flush queued messages to `zellij pipe --session <name>` at a configurable interval (default 200ms), delivering one `zellij pipe` call per unique dedup key.
- **FR-004**: System MUST self-terminate after a configurable idle timeout (default 30 seconds) with no incoming messages, removing the socket file on exit.
- **FR-005**: System MUST use socket `bind()` as the exclusive locking mechanism to prevent multiple broker instances.
- **FR-006**: The `cc-deck hook` command MUST check `mux.enabled` in config before attempting broker communication. When disabled or absent, hooks MUST call `zellij pipe` directly.
- **FR-007**: The hook client MUST attempt to connect to the broker socket, start a new broker if the socket is unavailable, and fall back to direct `zellij pipe` after 3 failed connection attempts (10ms apart).
- **FR-008**: The hook client MUST detect stale socket files (connect fails on existing file), remove them, and attempt a fresh broker start.
- **FR-009**: Each pipe message sent to the broker MUST include a `session_name` field populated from the `$ZELLIJ_SESSION_NAME` environment variable.
- **FR-010**: System MUST drop the oldest messages when the queue exceeds the configured `queue_size` limit (default 1000) and log the drop event when logging is enabled.
- **FR-011**: System MUST support configurable parameters via `~/.config/cc-deck/config.yaml` under a `mux` section: `enabled`, `flush_interval`, `idle_timeout`, `queue_size`, `dedup`, `log`.
- **FR-012**: System MUST write debug logs to `~/.local/state/cc-deck/mux.log` when `mux.log` is `true`, covering incoming messages, dedup hits, flush events, drop events, and lifecycle events.
- **FR-013**: System MUST create the socket directory if it does not exist, and fall back to `/tmp/cc-deck-$UID/` if `$XDG_RUNTIME_DIR` is not set.

### Key Entities

- **Broker**: The `cc-deck mux` daemon process. Owns the Unix socket, the dedup map, and the flush timer. Exactly one instance per machine.
- **Hook Client**: The pipe-sending logic within `cc-deck hook`. Connects to the broker socket, sends a message, and exits. Many instances, short-lived.
- **Dedup Key**: The tuple `(session_name, pipe_name, hash(args))` used to identify duplicate messages within a flush window.
- **Message**: A structured payload containing `session_name`, `pipe_name`, and `args` that represents a single pipe delivery request.
- **Flush Queue**: An in-memory map of dedup keys to their latest messages. Drained every flush interval.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero CliPipe timeout errors in the Zellij server log during a 30-minute multi-session workload (5+ concurrent Claude Code sessions with active hooks). Baseline: 58,350 timeouts in 3.5 hours.
- **SC-002**: Zellij server CPU usage stays below 30% during the same workload. Baseline: 187% CPU.
- **SC-003**: Hook execution time (wall clock from hook start to hook exit) remains under 50ms with broker enabled, comparable to the current direct-pipe path.
- **SC-004**: No orphaned broker processes remain 60 seconds after all Zellij sessions are closed.
- **SC-005**: Fallback to direct pipe completes within 50ms when the broker is unavailable, with no message loss.
- **SC-006**: Configuration changes take effect on the next broker restart without requiring recompilation.

## Clarifications

### Session 2026-08-04

- Q: What is the default queue_size? → A: 1000 messages (large enough to absorb bursts without unbounded memory risk; the brainstorm config example used this value)

## Assumptions

- Zellij's `zellij pipe --session <name>` command is the only supported delivery mechanism to the plugin. The broker does not interact with the plugin directly.
- The `$ZELLIJ_SESSION_NAME` environment variable is reliably set in all hook execution contexts.
- Unix domain sockets are available on all supported platforms (macOS, Linux). Windows is not a target.
- The existing `~/.config/cc-deck/config.yaml` configuration infrastructure (parsing, XDG path resolution) is already in place and can be extended with a `mux` section.
- The hook process (`cc-deck hook`) already handles pipe message construction. The broker receives fully-formed messages and relays them without interpretation.
- A single shared broker instance across all Zellij sessions is sufficient for the expected load (up to ~10 concurrent sessions).
