# Research: Cross-Pane MCP Server

## Go MCP Server Library

- **Decision**: Use `github.com/mark3labs/mcp-go` for the MCP server implementation
- **Rationale**: Most popular Go MCP library, actively maintained, supports stdio transport, clean API for registering tools with schemas
- **Alternatives considered**: Hand-rolling MCP protocol (too much work), `github.com/anthropics/anthropic-sdk-go` (doesn't include MCP server), `github.com/metoro-io/mcp-golang` (less active)

## Prompt Field in Hook Payloads

- **Decision**: Add `Prompt` field to `claudeHookPayload`, `codexHookPayload`, and `NormalizedPayload`
- **Rationale**: Claude Code's UserPromptSubmit hook sends a `prompt` field in its JSON payload. cc-deck currently ignores it. Adding it to the structs is a minimal change that enables topic tracking.
- **Verification needed**: Confirm Codex sends the same `prompt` field in UserPromptSubmit. OpenCode hook format may differ.

## Plugin-to-MCP Communication Pattern

- **Decision**: Extend the existing `DumpState` pattern with new pipe message types for MCP queries
- **Rationale**: `DumpState` already demonstrates the pattern: CLI sends a pipe message, plugin responds via `cli_pipe_output`, CLI reads the response. New MCP operations follow the same pattern with new pipe names.
- **New pipe messages needed**:
  - `cc-deck:mcp-sessions` - request session listing with extended metadata
  - `cc-deck:mcp-scrollback` - request pane scrollback (payload: pane_id, lines)
  - `cc-deck:mcp-state` - request detailed session state (payload: pane_id)
  - `cc-deck:mcp-inject` - inject text into a pane (payload: pane_id, text)

## Scrollback Reading

- **Decision**: Use `zellij_tile::shim::get_pane_scrollback` API
- **Rationale**: Available in zellij-tile 0.44.1 API but not currently used by the plugin. It returns the full scrollback content as a string. The plugin will read it, truncate to the requested line count, and return via cli_pipe_output.
- **Risk**: The function may return very large strings. The 500-line cap in FR-016 mitigates this.

## MCP Server Lifecycle

- **Decision**: Each agent session spawns its own `cc-deck mcp serve` process via stdio
- **Rationale**: Standard MCP server lifecycle. Claude Code manages the process start/stop. The server connects to the Zellij plugin via pipe messages (same as `cc-deck hook`). No long-lived daemon needed.
- **MCP config entry** (added to .mcp.json):
  ```json
  {
    "mcpServers": {
      "cc-deck": {
        "command": "cc-deck",
        "args": ["mcp", "serve"]
      }
    }
  }
  ```

## File-Based Ask Handshake

- **Decision**: Use `os.TempDir()` with UUID-named files
- **Rationale**: Platform-agnostic temp directory. UUID prevents collisions between concurrent asks. The MCP server writes the query file, injects the prompt, then polls for the response file with configurable timeout.
- **Injected prompt format**: The prompt instructs the target agent to read the query file and write a response file. Must be agent-agnostic (work with Claude Code, Codex, OpenCode).
- **Cleanup**: Response file deleted immediately after reading. Query file deleted after response received or on timeout. Orphaned files cleaned up on MCP server shutdown.

## Session Context Aggregation

- **Decision**: MCP server reads context files directly from disk, not via the plugin
- **Rationale**: The MCP server process runs on the same machine and has direct filesystem access. Reading CLAUDE.md, MEMORY.md, and running `git` commands is simpler than routing through the plugin. The plugin provides session metadata (pane_id, display_name, activity, CWD); the MCP server enriches it with file-based context.
- **Context sources**:
  - `{cwd}/CLAUDE.md` - first 500 chars as project summary
  - `~/.claude/projects/{path-encoded}/memory/MEMORY.md` - memory index entries
  - `git -C {cwd} branch --show-current` - current branch
  - `git -C {cwd} diff --name-only` - modified files
  - Hook history from plugin state (recent tool names)
