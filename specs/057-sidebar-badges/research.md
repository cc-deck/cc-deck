# Research: Configurable Sidebar Badges

## JSON Dot-Path Extraction in Go

**Decision**: Implement a minimal dot-path extractor using `encoding/json` and `map[string]interface{}` traversal.

**Rationale**: The extraction needs are simple (`.mode`, `.status`, `.result.outcome`). A full jq implementation would be overkill. Go's `encoding/json` can unmarshal into `map[string]interface{}` and we split the dot-path on `.` to walk nested maps. Non-string leaf values are converted via `fmt.Sprintf("%v", val)`.

**Alternatives considered**:
- `github.com/itchyny/gojq`: Full jq in Go. Too heavy a dependency for simple key access.
- `github.com/tidwall/gjson`: Lightweight JSON path library. Good but adds an external dependency when stdlib is sufficient.
- Custom parser: Exactly what we're doing, keeping it to ~30 lines.

## Badge Payload Transport

**Decision**: Add a `badges` field (`[]string`) to both the Go `pipePayload` and the Rust `HookPayload`. Each string is a resolved emoji.

**Rationale**: Sending pre-resolved emojis keeps the plugin simple (no config parsing, no file I/O). The CLI already has access to `working_dir` (from the hook's CWD field) and config (via `internal/config`). The badge list is small (1-5 items) so serialization overhead is negligible.

**Alternatives considered**:
- Sending badge rule names and letting the plugin resolve: Requires plugin to access filesystem and parse YAML. Not feasible in WASM sandbox.
- Sending raw JSON values and letting the plugin map: Splits the mapping logic across two codebases.

## Config Integration

**Decision**: Add a `Badges []BadgeRule` field to the existing `Config` struct in `internal/config/config.go`.

**Rationale**: The config file is already loaded by the CLI. Adding a `badges:` top-level key follows the existing pattern. The config is loaded once at CLI startup and reused across hook invocations.

## Badge Evaluation Timing

**Decision**: Evaluate badges on every hook event, reading files synchronously.

**Rationale**: Hook events are frequent enough to catch state changes promptly. Badge state files are small (<10KB). File reads add <1ms each. With 5 rules, total evaluation time is well under the 50ms target. Caching by mtime is not needed in v1 but could be added later.

## Rendering Position

**Decision**: Render badges on line 2, before the git branch icon (⎇).

**Rationale**: Line 1 is reserved for the activity indicator and project name. Line 2 currently only shows the branch, leaving space for badges. Placing badges before the branch keeps them visually prominent. Multiple badges are concatenated with no separator (emoji characters are self-delimiting).
