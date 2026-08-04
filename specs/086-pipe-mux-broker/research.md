# Research: Pipe Mux Broker

## Unix Domain Socket Daemon Pattern in Go

**Decision**: Use `net.ListenUnix` with `SOCK_STREAM` for the broker, `net.DialUnix` for clients.

**Rationale**: Go's `net` package provides clean Unix socket support. `SOCK_STREAM` (connection-oriented) is simpler than `SOCK_DGRAM` for newline-delimited JSON since each client connection delivers exactly one message. No need for message framing or length prefixes.

**Alternatives considered**:
- `SOCK_DGRAM` (datagram): Simpler send semantics but no connection backpressure and message size limits. Rejected because connection-oriented semantics make client error detection cleaner.
- Named pipes (FIFO): Single-reader only, no concurrent writers without external coordination. Rejected.
- TCP localhost: Works but requires port allocation and is slower than Unix sockets. Rejected.

## Detached Process Spawning in Go

**Decision**: Use `os/exec.Cmd` with `SysProcAttr{Setsid: true}` and `cmd.Start()` (no `cmd.Wait()`). Call `cmd.Process.Release()` to detach.

**Rationale**: This is the standard Go pattern for fire-and-forget background processes. The existing `internal/session/autosave.go` uses the same pattern (`cmd.Start()` + `cmd.Process.Release()`). `Setsid: true` creates a new session so the broker survives parent exit.

**Alternatives considered**:
- `syscall.ForkExec`: Lower-level, harder to set up env/args. Rejected in favor of `os/exec`.
- systemd/launchd user service: Correct for production daemons but overkill for a self-terminating broker. Rejected.

## Socket-as-Lock Singleton Pattern

**Decision**: `net.ListenUnix` with `syscall.Unlink` for stale recovery. The `bind()` call fails with `EADDRINUSE` if another broker owns the socket.

**Rationale**: The socket file serves as both communication endpoint and lock. No separate pidfile or flock needed. Stale detection: if `net.DialUnix` fails on an existing socket file, the file is stale (broker crashed). Remove it and retry.

**Alternatives considered**:
- `syscall.Flock` on a pidfile: The autosave code uses this, but it requires two artifacts (pidfile + socket). Rejected for simplicity.
- Advisory file lock (`os.OpenFile` with `O_EXCL`): Race-prone across processes. Rejected.

## Dedup Key Hashing

**Decision**: Use `crypto/sha256` truncated to 16 hex chars (64 bits) as the dedup key.

**Rationale**: The dedup map holds at most `queue_size` (default 1000) entries. At 1000 entries, the collision probability for 64-bit hashes is ~2.7e-14 (birthday bound). Truncated SHA-256 is fast and has excellent distribution. The full key material (`session|pipe_name|args`) can be 1-4KB, so hashing is necessary to keep map keys short.

**Alternatives considered**:
- FNV-1a (32-bit): Faster but higher collision rate. Rejected.
- Full SHA-256 (32 bytes): Unnecessary, 8 bytes provides ample collision resistance for <10K entries. Rejected.
- String concatenation as key: Works but wastes memory (duplicate storage of the full args string in both key and value). Rejected.

## Config Duration Parsing

**Decision**: Use `time.Duration` with yaml.v3's built-in support. Users write `200ms`, `30s`, etc.

**Rationale**: Go's `time.ParseDuration` handles all standard suffixes. yaml.v3 supports custom unmarshalers, but `time.Duration` already implements `encoding.TextUnmarshaler` in Go 1.25, so `flush_interval: 200ms` works directly.

**Alternatives considered**:
- Integer milliseconds (`flush_interval: 200`): Less readable, requires documentation about the unit. Rejected.
- String with custom parser: Unnecessary given `time.Duration` support. Rejected.

## Debug Logging

**Decision**: Custom lightweight logger writing to `~/.local/state/cc-deck/mux.log`. No log rotation in V1; truncate on broker start.

**Rationale**: The broker is short-lived (30s idle timeout) and restarts frequently. Truncating on start prevents unbounded growth without rotation complexity. Log format: `2026-08-04T12:34:56.789Z [EVENT] details`. The `log` stdlib package with a custom writer is sufficient.

**Alternatives considered**:
- `slog` (Go structured logging): More capable but the broker only logs to a file, not structured output. Rejected as overkill.
- Log rotation via `lumberjack`: Adds a dependency for a log that resets every broker lifecycle. Rejected.
- Append mode (no truncation): Risk of unbounded growth if the user forgets logging is enabled. Rejected.

## Signal Handling

**Decision**: Listen for `SIGTERM` and `SIGINT` via `signal.Notify`. On signal, perform a final flush and clean shutdown (remove socket file).

**Rationale**: Standard Go daemon pattern. The broker must remove its socket file on exit to prevent stale socket detection from triggering unnecessarily. No `SIGHUP` reload needed since config is read once at startup.

**Alternatives considered**:
- No signal handling (rely on OS cleanup): Socket file would remain after kill, requiring stale detection on every hook call. Rejected because clean shutdown is better.
