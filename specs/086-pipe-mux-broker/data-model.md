# Data Model: Pipe Mux Broker

## Entities

### Message

The unit of communication between hook clients and the broker.

| Field | Type | Description |
|-------|------|-------------|
| session_name | string | Zellij session name (from `$ZELLIJ_SESSION_NAME`) |
| pipe_name | string | Pipe destination name (e.g., `cc-deck:hook`) |
| args | string | JSON-encoded payload (opaque to the broker) |

**Serialization**: JSON with newline delimiter over Unix socket.

**Lifecycle**: Created by hook client, sent to broker, stored in flush queue (keyed by dedup key), consumed during flush (delivered via `zellij pipe`), discarded after delivery.

### DedupKey

Computed identifier for message deduplication.

| Field | Type | Description |
|-------|------|-------------|
| hash | string | `sha256(session_name + "|" + pipe_name + "|" + args)[:16]` (hex, 16 chars) |

**Uniqueness**: Identifies a unique (session, pipe, payload) tuple. Two messages with the same dedup key are considered duplicates; only the latest is kept.

### FlushQueue

In-memory map holding pending messages for the next flush cycle.

| Field | Type | Description |
|-------|------|-------------|
| entries | map[string]Message | Keyed by DedupKey hash, value is the latest Message |
| mu | sync.Mutex | Protects concurrent access from accept goroutine |

**Capacity**: Bounded by `queue_size` config (default 1000). When full, the oldest entry is evicted before inserting a new one.

**Flush behavior**: Every `flush_interval` (default 200ms), the map is swapped with an empty map. The old map is iterated and each message is delivered via `zellij pipe --session <session_name> --name <pipe_name> -- <args>`. Delivery is sequential within a flush cycle.

### MuxConfig

Configuration for the broker, stored as a section in `~/.config/cc-deck/config.yaml`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Whether hooks route through the broker |
| flush_interval | duration | 200ms | How often the broker drains the queue |
| idle_timeout | duration | 30s | Self-terminate after this idle period |
| queue_size | int | 1000 | Max messages before dropping oldest |
| dedup | bool | true | Enable/disable dedup (when false, all messages delivered in order) |
| log | bool | false | Enable debug logging |

### Broker (runtime state, not persisted)

| Field | Type | Description |
|-------|------|-------------|
| listener | net.UnixListener | Bound Unix socket accepting connections |
| queue | FlushQueue | Pending messages for next flush |
| config | MuxConfig | Loaded once at startup |
| logger | Logger | Writes to mux.log when enabled, no-op otherwise |
| idleTimer | time.Timer | Resets on each incoming message, fires shutdown |
| done | chan struct{} | Closed on shutdown signal or idle timeout |

## State Transitions

### Broker Lifecycle

```
[not running] ---(hook fires, connect fails)---> [starting]
[starting] ---(bind succeeds)---> [running]
[starting] ---(bind fails, EADDRINUSE)---> [not running] (another instance won)
[running] ---(message received)---> [running] (idle timer reset)
[running] ---(idle timeout)---> [shutting down]
[running] ---(SIGTERM/SIGINT)---> [shutting down]
[shutting down] ---(final flush, socket removed)---> [not running]
```

### Hook Client Flow

```
[hook fires] ---(config: mux disabled?)---> [direct zellij pipe]
[hook fires] ---(config: mux enabled)---> [try connect to socket]
[try connect] ---(success)---> [send message, exit]
[try connect] ---(fail, no socket file)---> [start broker, retry connect x3]
[try connect] ---(fail, stale socket)---> [remove socket, start broker, retry x3]
[retry connect] ---(success)---> [send message, exit]
[retry connect] ---(3 failures)---> [fallback: direct zellij pipe]
```

## Relationships

```
Hook Client ---sends---> Message ---stored in---> FlushQueue ---flushed to---> zellij pipe
                                                      |
                                            keyed by DedupKey
                                                      |
                                          configured by MuxConfig
```

## File Locations

| Artifact | Path | Lifecycle |
|----------|------|-----------|
| Socket | `$XDG_RUNTIME_DIR/cc-deck/mux.sock` (fallback: `/tmp/cc-deck-$UID/mux.sock`) | Created on broker start, removed on shutdown |
| Config | `$XDG_CONFIG_HOME/cc-deck/config.yaml` (mux section) | Persistent, user-managed |
| Debug log | `$XDG_STATE_HOME/cc-deck/mux.log` | Truncated on each broker start, only written when enabled |
