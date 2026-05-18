# Brainstorm: Configurable Sidebar Badges

**Date:** 2026-05-18
**Status:** active

## Problem Framing

The sidebar displays two-line entries per session: line 1 has the activity indicator and project name, line 2 has the git branch. There is no way to show additional workflow state like whether a spex ship pipeline is active, a deployment is running, or a security scan is in progress.

The spex plugin already tracks its state in `.specify/.spex-state` (a JSON file with a `mode` field), and the cc-deck status line reads this file to show ship/flow progress. But the sidebar has no equivalent. Adding a hardcoded spex indicator would solve one case but miss the general opportunity: any tool that writes a state file could surface a badge in the sidebar.

## Approaches Considered

### A: Full JSON path extraction (chosen)

Badge rules in `~/.config/cc-deck/config.yaml` define file-to-emoji mappings. The Go CLI hook command evaluates all rules against the session's `working_dir` on each hook event, extracts a value using a simple dot-path expression, maps it to an emoji, and sends resolved badges in the hook payload. The Rust WASM plugin renders them on line 2 before the branch icon.

```yaml
badges:
  - name: spex
    file: .specify/.spex-state
    format: json
    extract: .mode
    values:
      ship: "🚢"
      flow: "🌊"
    default: "📦"
  - name: lock
    file: .security-scan.json
    format: json
    extract: .status
    values:
      pass: "✅"
      fail: "🔴"
```

- Pros: Flexible, covers spex and future tools. No external dependencies (Go stdlib JSON parsing). Simple dot-path avoids jq complexity. Multiple badges supported.
- Cons: Only supports JSON files. Dot-path is limited compared to full jq.

### B: File-exists only with fixed emoji

Each rule checks whether a file exists and shows a fixed emoji. No content inspection.

- Pros: Trivial to implement. Fast.
- Cons: Cannot distinguish between modes (ship vs flow show the same emoji). Much less useful.

### C: Pluggable evaluator with shell command

Each rule can run a shell command that outputs the emoji.

- Pros: Maximum flexibility, any file format, any logic.
- Cons: Running shell commands on every hook event adds latency. Security concerns. Harder to debug.

## Decision

Approach A: JSON dot-path extraction. It hits the right balance of flexibility and simplicity. The CLI already reads config YAML and has filesystem access. The dot-path extraction (`.mode`, `.status`, `.result.outcome`) is implementable in ~50 lines of Go using `encoding/json` and string splitting, with no external dependencies.

Key architectural choice: the Go CLI evaluates badge rules and sends resolved emojis in the hook payload. The Rust plugin just renders what it receives. This keeps file I/O and config parsing in Go (where it's natural) and rendering in Rust (where it already lives).

## Key Requirements

- Badge rules defined in `~/.config/cc-deck/config.yaml` under a `badges:` section
- Each rule specifies: name, file (relative to working_dir), format (json), extract (dot-path), values (map of extracted value to emoji), default emoji
- CLI hook command evaluates all rules on each hook event using the session's `working_dir`
- Resolved badges sent as a list of emoji strings in the hook payload to the plugin
- Plugin renders multiple badges on line 2, before the branch icon (e.g., `🚢 ⎇ feat/sidebar`)
- Multiple badges can display simultaneously per session
- Missing files or extraction failures silently skip the badge (no errors shown)

## Open Questions

- Should badge evaluation be cached (skip re-evaluation if file mtime hasn't changed)?
- Should there be a maximum number of badges displayed to prevent line overflow?
- Should the `format` field support YAML in a future iteration, or keep it JSON-only?
- How should the dot-path handle arrays (e.g., `.items[0].status`)?
