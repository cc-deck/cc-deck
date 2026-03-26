# Research: Session Save and Restore

## R1: Plugin State Query Protocol

**Decision**: Add a new `cc-deck:dump-state` pipe message. The plugin serializes all sessions as JSON and responds via `cli_pipe_output()` / `unblock_cli_pipe_input()`.

**Rationale**: The Zellij pipe API supports bidirectional communication. The plugin already serializes sessions for sync broadcasts (`serde_json::to_string(&state.sessions)`). The CLI can invoke `zellij pipe --name cc-deck:dump-state` and read JSON from stdout.

**Alternatives considered**:
- Reading WASI `/cache/` files directly: Rejected because WASI cache paths are session-scoped UUIDs and not discoverable from the host.
- Having the plugin write to a host file via `run_command`: Works but adds unnecessary indirection. The pipe API is designed for this exact use case.

## R2: Auto-save Integration Point

**Decision**: Add auto-save side-effect in the Go `cc-deck hook` command (hook.go), after the `zellij pipe --name cc-deck:hook` call succeeds.

**Rationale**: The hook command is the single point through which all Claude events flow. It already has access to session context and runs on the host filesystem. Adding auto-save here avoids WASM sandbox limitations and keeps the logic in Go where file I/O is straightforward.

**Alternatives considered**:
- Auto-save from the Rust plugin via `run_command`: Works but requires spawning a subprocess from WASM, adding complexity. The Go CLI is the natural place for host filesystem operations.

## R3: CLI Command Structure

**Decision**: Follow the existing `cc-deck plugin` / `cc-deck profile` cobra command group pattern. Create `cc-deck/internal/cmd/session.go` for command constructors and `cc-deck/internal/session/` for business logic.

**Rationale**: The codebase consistently separates cobra commands (thin wrappers in `internal/cmd/`) from business logic (dedicated packages). The `plugin` command group is the closest analog.

**Alternatives considered**:
- Inline business logic in cmd files: Rejected; violates established project pattern.

## R4: State File Location

**Decision**: Use `$XDG_CONFIG_HOME/cc-deck/sessions/` (typically `~/.config/cc-deck/sessions/`). The project already uses `github.com/adrg/xdg` for XDG paths.

**Rationale**: Session snapshots are user configuration (workspace layouts), not cache data. XDG config is the appropriate location. The `adrg/xdg` library is already a dependency.

**Alternatives considered**:
- `$XDG_DATA_HOME/cc-deck/sessions/`: Reasonable but config is more appropriate since snapshots are user-managed workspace definitions, not application data.

## R5: Restore Tab Creation

**Decision**: Use `zellij action new-tab` to create tabs, then `zellij action write-chars` to send `cd` and `claude --resume` commands to the terminal pane.

**Rationale**: `new-tab` creates tabs from the `new_tab_template` in the layout, which includes the cc-deck sidebar. `write-chars` sends keystrokes to the focused pane, which is the terminal pane in the new tab. A brief delay between tabs allows plugin initialization.

**Alternatives considered**:
- Using `zellij action new-tab --cwd <dir>`: Would set CWD but doesn't allow starting Claude automatically. Still need `write-chars` for the Claude command.

## R6: Pipe Response for CLI-initiated Dump

**Decision**: Use `PipeSource::Cli(pipe_id)` to detect CLI-initiated pipes and respond via `cli_pipe_output(&pipe_id, &state_json)` followed by `unblock_cli_pipe_input(&pipe_id)`.

**Rationale**: The Zellij SDK provides explicit APIs for CLI pipe responses. The `PipeSource::Cli` variant carries the pipe_id needed to route the response back to the correct CLI process. Multiple plugin instances will all receive the message, but only one needs to respond (use `is_on_active_tab()` or respond from the first instance).

**Alternatives considered**:
- Having all instances respond: Would produce duplicate output. Only one instance should respond.
