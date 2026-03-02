# Research: cc-deck

**Date:** 2026-03-02
**Feature:** specs/001-cc-deck/spec.md

## Decision 1: Claude Code Hook Integration for Activity Detection

### Decision
Use Claude Code hooks with the following event-to-state mapping:

| cc-deck State | Hook Events | Pipe Message |
|---------------|-------------|--------------|
| **working** | `UserPromptSubmit`, `PreToolUse`, `PostToolUse` | `cc-deck::working::$ZELLIJ_PANE_ID` |
| **waiting** | `Notification` (permission_prompt, idle_prompt), `PermissionRequest` | `cc-deck::waiting::$ZELLIJ_PANE_ID` |
| **done** | `Stop` | `cc-deck::done::$ZELLIJ_PANE_ID` |
| **idle** | (timer-based, no hook event) | Derived from time since last `done` event |

### Hook Configuration
Users add to `~/.claude/settings.json`:
```json
{
  "hooks": {
    "UserPromptSubmit": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "zellij pipe --name 'cc-deck::working::$ZELLIJ_PANE_ID'" }]
    }],
    "PreToolUse": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "zellij pipe --name 'cc-deck::working::$ZELLIJ_PANE_ID'" }]
    }],
    "Notification": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "zellij pipe --name 'cc-deck::waiting::$ZELLIJ_PANE_ID'" }]
    }],
    "PermissionRequest": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "zellij pipe --name 'cc-deck::waiting::$ZELLIJ_PANE_ID'" }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "zellij pipe --name 'cc-deck::done::$ZELLIJ_PANE_ID'" }]
    }]
  }
}
```

### Pipe Message Format
`cc-deck::EVENT_TYPE::PANE_ID` where:
- `cc-deck` is the plugin prefix (matched in `pipe()` method)
- `EVENT_TYPE` is `working`, `waiting`, or `done`
- `PANE_ID` is the numeric Zellij pane ID from `$ZELLIJ_PANE_ID`

Uses `--name` (broadcast pipe), not `--plugin` (which spawns new instances).

### Rationale
- Proven pattern: zellij-attention plugin uses the same pipe mechanism successfully
- claude-code-zellij-status shows even richer state detection is possible
- Fallback when hooks are not configured: timer-based idle detection via `set_timeout` + `PaneUpdate` title monitoring

### Alternatives Considered
- **Screen-scraping PTY output**: Not possible. WASM sandbox isolates plugin memory from other panes.
- **Pane title monitoring only**: Too limited. Claude Code updates pane title, but not with enough granularity to distinguish working vs waiting.
- **File-based state sharing**: claude-code-zellij-status uses `/tmp/claude-zellij-status/`. Viable but pipes are more natural in the Zellij plugin model.

## Decision 2: Persistent Storage via WASI `/cache` Directory

### Decision
Use the WASI `/cache` virtual directory for persisting `recent.json`.

### Details
Zellij mounts four virtual directories into each plugin's WASI sandbox:

| Path | Scope | Persistent? | Use For |
|------|-------|-------------|---------|
| `/host` | CWD of focused terminal | N/A (real fs) | Reading git repo info |
| `/data` | Per plugin instance | No (deleted on unload) | Session-scoped temp state |
| `/cache` | Shared per plugin URL | **Yes** | recent.json, user prefs |
| `/tmp` | Shared all plugins | No | Temporary files |

Host-side location of `/cache`:
- **macOS**: `~/Library/Caches/org.Zellij-Contributors.zellij/<hash>/plugin_cache/`
- **Linux**: `~/.cache/zellij/<hash>/plugin_cache/`

Standard Rust `std::fs` operations work normally. No special permissions needed.

```rust
const RECENT_FILE: &str = "/cache/recent.json";

fn save_recent(recent: &RecentEntries) {
    if let Ok(data) = serde_json::to_string_pretty(recent) {
        let _ = std::fs::write(RECENT_FILE, data);
    }
}

fn load_recent() -> RecentEntries {
    std::fs::read_to_string(RECENT_FILE)
        .ok()
        .and_then(|data| serde_json::from_str(&data).ok())
        .unwrap_or_default()
}
```

### Rationale
- `/cache` is the designated persistent storage for Zellij plugins
- No extra permissions needed (unlike `FullHdAccess` for arbitrary paths)
- Survives plugin unloads, session restarts, and Zellij restarts
- Standard Rust file I/O works without WASI-specific APIs

### Alternatives Considered
- **XDG config path**: Not accessible from WASM sandbox. No `HOME` env var available. Would require `FullHdAccess` permission + `change_host_folder`.
- **Plugin config (KDL)**: Read-only. Good for user preferences but not for state persistence.
- **`/data` directory**: Deleted on plugin unload. Not suitable for cross-session persistence.

## Decision 3: Keybinding Strategy (Major Spec Update Required)

### Decision
Replace the prefix key model with direct keybindings using `MessagePluginId` via `reconfigure`. Use `Ctrl+Shift` modifiers instead of single `Ctrl+letter` keys.

### Problem Found
Both proposed defaults conflict:
- **Ctrl-T**: Conflicts with Zellij (enters Tab mode) AND Claude Code (toggle task list)
- **Ctrl-B**: Conflicts with Zellij (enters Tmux mode) AND Claude Code (background task)

In fact, every single `Ctrl+letter` is consumed by either Zellij's mode system or standard terminal conventions.

### New Keybinding Defaults

| Action | Key | Conflicts |
|--------|-----|-----------|
| Fuzzy picker | `Ctrl+Shift+T` | None |
| New session | `Ctrl+Shift+N` | None |
| Rename session | `Ctrl+Shift+R` | None |
| Close session | `Ctrl+Shift+X` | None |
| Switch by number | `Ctrl+Shift+1-9` | None |

Requires Kitty Keyboard Protocol support (Ghostty, Kitty, WezTerm, Alacritty, foot all support this). Zellij 0.41+ handles these correctly.

### Implementation
Plugin registers keybindings at load time via `reconfigure`:
```rust
fn load(&mut self, _config: BTreeMap<String, String>) {
    request_permission(&[PermissionType::Reconfigure]);
    // After permission granted:
    let my_id = get_plugin_ids().plugin_id;
    reconfigure(format!(r#"
        keybinds {{
            shared {{
                bind "Ctrl Shift t" {{
                    MessagePluginId {my_id} {{ name "open_picker" }}
                }}
                bind "Ctrl Shift n" {{
                    MessagePluginId {my_id} {{ name "new_session" }}
                }}
            }}
        }}
    "#));
}
```

### Rationale
- Eliminates the prefix key model entirely (simpler UX, one keystroke instead of two)
- `Ctrl+Shift` keys have zero conflicts with Zellij, Claude Code, or terminal conventions
- `reconfigure` is the official Zellij API for plugin keybindings
- Temporary bindings (not persisted to user config)

### Alternatives Considered
- **Prefix key (Ctrl-B + key)**: Conflicts with Zellij tmux mode. Two keystrokes instead of one.
- **Alt+key**: Alt+B moves cursor back one word in readline. Many Alt conflicts.
- **F-keys**: Zero conflicts but unusual and hard to remember.
- **Ctrl+Space**: Some terminals may not pass it through.

### Spec Impact
FR-014 and FR-015 need updating to reflect the new keybinding model (direct bindings, no prefix key).

## Decision 4: Git Repo Detection Method

### Decision
Use `run_command` API to execute `git rev-parse --show-toplevel` in the session's working directory.

### Details
The `run_command` API runs a command asynchronously and delivers results via `RunCommandResult` event:

```rust
run_command(
    &["git", "rev-parse", "--show-toplevel"],
    BTreeMap::from([
        ("pane_id".to_string(), pane_id.to_string()),
    ]),
);

// In update():
Event::RunCommandResult(exit_code, stdout, stderr, context) => {
    if exit_code == Some(0) {
        let repo_path = String::from_utf8_lossy(&stdout).trim().to_string();
        let repo_name = Path::new(&repo_path).file_name().unwrap().to_str().unwrap();
        // Set session name to repo_name
    } else {
        // Not a git repo, use directory basename
    }
}
```

### Rationale
- `run_command` is async and non-blocking (perfect for WASM)
- Context dictionary allows correlating results back to the specific pane/session
- Falls back cleanly on non-git directories (non-zero exit code)
- No filesystem access needed (git binary does the work)

## Decision 5: Zellij Plugin Rendering Model

### Decision
Plugin operates in two rendering modes depending on context:

1. **Status bar mode**: Plugin renders as a compact bottom bar (always visible). Uses the `render()` method with ANSI escape codes and `print!()` macro.
2. **Picker mode**: Plugin renders a floating pane overlay. Uses `intercept_key_presses()` to capture all input while the picker is active, then `clear_key_presses_intercepts()` when dismissed.

### Details
The plugin is loaded once but needs to serve both the persistent status bar and the popup picker. Approach:

- The plugin instance renders the status bar by default in its `render()` method
- When triggered via pipe message (`open_picker`), it:
  1. Stores the current state
  2. Calls `intercept_key_presses()` to capture keystrokes
  3. Switches internal rendering mode to show the picker UI
  4. On selection or Escape, calls `focus_terminal_pane(selected_id)` and restores status bar mode

Alternatively, the picker could be a second instance of the same plugin launched as a floating pane via keybinding config:
```kdl
bind "Ctrl Shift t" {
    LaunchOrFocusPlugin "file:cc-deck.wasm" {
        floating true
        name "cc-deck-picker"
    }
}
```

The `LaunchOrFocusPlugin` approach is cleaner because Zellij manages the floating pane lifecycle. The two instances communicate via pipe messages.

### Rationale
- Using `LaunchOrFocusPlugin` for the picker leverages Zellij's floating pane management
- No orphan pane risk (Zellij handles show/hide/close)
- Status bar instance and picker instance communicate via pipes
- This effectively gives us the two-plugin architecture (Approach B from brainstorm) without two separate WASM binaries
