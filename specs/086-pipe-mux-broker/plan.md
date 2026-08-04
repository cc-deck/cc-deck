# Implementation Plan: Pipe Mux Broker

**Branch**: `086-pipe-mux-broker` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/086-pipe-mux-broker/spec.md`

## Summary

Introduce a `cc-deck mux` daemon that intercepts hook pipe messages via a Unix domain socket, deduplicates them by (session_name, pipe_name, hash(args)), and flushes to `zellij pipe` at 200ms intervals. This eliminates CliPipe timeout storms (58,350 timeouts in 3.5h, 187% CPU) when multiple Claude Code sessions fire hooks concurrently.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (config), testify v1.11.1 (tests)
**Storage**: Unix domain socket at `$XDG_RUNTIME_DIR/cc-deck/mux.sock`, debug log at `~/.local/state/cc-deck/mux.log`
**Testing**: `make test` (go test ./...)
**Target Platform**: macOS, Linux (Unix domain sockets required)
**Project Type**: CLI tool with background daemon subprocess
**Performance Goals**: Hook exit <50ms, flush interval 200ms, zero CliPipe timeouts under 10-session load
**Constraints**: Daemon must self-terminate after 30s idle, socket-as-lock for singleton, fallback to direct pipe on broker failure
**Scale/Scope**: Up to ~10 concurrent Zellij sessions, ~16 pipe messages/second peak

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is template-only (no project-specific principles defined). The project CLAUDE.md constitution principles apply:

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests and documentation | Will comply | Unit tests for broker, hook client, config. CLI reference update. |
| Build rules (make only) | Will comply | No direct `go build`. Use `make test`, `make lint`, `make install`. |
| XDG paths via internal/xdg | Will comply | Add `RuntimeDir` to internal/xdg package |
| Podman only | N/A | No container operations |

## Project Structure

### Documentation (this feature)

```text
specs/086-pipe-mux-broker/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/
├── cmd/cc-deck/main.go              # Add mux command registration
├── internal/
│   ├── cmd/
│   │   ├── mux.go                   # NEW: cc-deck mux subcommand (daemon entry point)
│   │   └── hook.go                  # MODIFY: route through mux client when enabled
│   ├── config/
│   │   ├── config.go                # MODIFY: add MuxConfig struct and field
│   │   └── validate.go              # MODIFY: add mux config validation
│   ├── mux/
│   │   ├── broker.go                # NEW: core broker (socket listener, dedup map, flush loop)
│   │   ├── broker_test.go           # NEW: unit tests
│   │   ├── client.go                # NEW: client library (connect, send, close)
│   │   ├── client_test.go           # NEW: unit tests
│   │   ├── message.go               # NEW: message types and serialization
│   │   ├── message_test.go          # NEW: unit tests
│   │   └── log.go                   # NEW: debug logger
│   └── xdg/
│       └── xdg.go                   # MODIFY: add RuntimeDir
└── test/
    └── mux_integration_test.go      # NEW: integration test (broker + client)
```

**Structure Decision**: New `internal/mux/` package contains the broker and client library. The cmd layer (`internal/cmd/mux.go`) wires cobra commands to the mux package. The hook modification in `internal/cmd/hook.go` is minimal: check config, use mux client if enabled, fallback to direct pipe.

## Key Design Decisions

### 1. Wire Protocol (Unix Socket)

Messages are newline-delimited JSON over the Unix socket. Each message is a single JSON object followed by `\n`. No framing header needed since messages are small (<4KB) and the socket is local.

```json
{"session_name":"my-session","pipe_name":"cc-deck:hook","args":"{...}"}
```

The client writes one JSON line and closes the connection (fire-and-forget, no response expected). The broker accepts connections in a loop, reads one line per connection, and closes.

### 2. Dedup Map

The flush queue is a `map[string]Message` protected by a mutex. The dedup key is computed as `sha256(session_name + "|" + pipe_name + "|" + args)[:16]` (hex-encoded, 16 chars). The map stores the latest message for each key. On flush, the map is swapped atomically (replace with empty map, iterate old map).

### 3. Hook Integration Point

The modification to `hook.go` is at lines 151-153, where `exec.CommandContext` calls `zellij pipe`. Before that call, check:
1. Is `config.Mux.Enabled` true?
2. Is `$ZELLIJ_SESSION_NAME` set?
3. If both yes, call `mux.Send(socketPath, sessionName, pipeName, payload)` instead.
4. If Send fails (broker down), fall through to the existing direct `zellij pipe` call.

### 4. Daemon Lifecycle

The broker starts via `cc-deck mux` (called internally by the hook client as a detached process). It:
1. Creates socket directory if needed
2. Binds Unix socket (atomic lock)
3. Starts accept loop goroutine
4. Starts flush timer goroutine (200ms ticker)
5. Starts idle timer goroutine (30s)
6. Blocks on signal (SIGTERM/SIGINT) or idle timeout

On shutdown: drain remaining messages with a final flush, close socket, remove socket file.

### 5. Config Integration

Add to `Config` struct in `internal/config/config.go`:

```go
type MuxConfig struct {
    Enabled       bool          `yaml:"enabled"`
    FlushInterval time.Duration `yaml:"flush_interval"`
    IdleTimeout   time.Duration `yaml:"idle_timeout"`
    QueueSize     int           `yaml:"queue_size"`
    Dedup         *bool         `yaml:"dedup,omitempty"`
    Log           bool          `yaml:"log"`
}
```

Defaults applied when zero-values detected: enabled=false, flush_interval=200ms, idle_timeout=30s, queue_size=1000, dedup=true, log=false.

### 6. XDG RuntimeDir

Add `RuntimeDir` to `internal/xdg/xdg.go` resolving `$XDG_RUNTIME_DIR`. Fallback to `/tmp/cc-deck-<uid>/` when unset (per spec FR-013). Socket path: `<RuntimeDir>/cc-deck/mux.sock`.

## Complexity Tracking

No constitution violations to justify.
