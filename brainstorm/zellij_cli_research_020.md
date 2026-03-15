# Zellij CLI Actions Research Report
## For 020-Demo-Recordings Feature Planning

**Research Date**: 2026-03-14
**Researcher**: Claude Code (Haiku)
**Zellij Version**: Current (zellij --version for exact)

---

## 1. Available Zellij Actions for Demo Scripting

### Complete Action List (Relevant to Demo Recording)

#### Tab Management
- **`new-tab`** - Create a new tab with optional layout and name
  - `zellij action new-tab -n "name" -l layout_name`
  - Supports custom cwd with `-c`
  - **Demo use**: Programmatically create tabs for demo projects

- **`go-to-tab`** - Navigate to tab by index
  - `zellij action go-to-tab <INDEX>`
  - Index starts at 1

- **`go-to-tab-name`** - Navigate to tab by name
  - `zellij action go-to-tab-name "tab_name"`
  - **Demo use**: Switch between demo project tabs

- **`go-to-next-tab`** / **`go-to-previous-tab`** - Navigate adjacently
  - No arguments required

- **`rename-tab`** - Rename the current tab
  - `zellij action rename-tab "new_name"`
  - **Demo use**: Label tabs with demo project names

- **`close-tab`** - Close current tab

- **`move-tab`** - Move tab left or right
  - `zellij action move-tab [left|right]`

#### Pane Management
- **`new-pane`** - Open pane in specified direction
  - `zellij action new-pane -d [right|down]`
  - Supports: `-f` (floating), `-c` (custom cwd), `-n` (name)
  - **Demo use**: Split panes for multi-project view

- **`focus-next-pane`** / **`focus-previous-pane`** - Move focus between panes

- **`move-focus`** - Move focus in direction
  - `zellij action move-focus [right|left|up|down]`

- **`move-focus-or-tab`** - Move focus, or switch tabs on screen edge
  - `zellij action move-focus-or-tab [right|left|up|down]`

- **`close-pane`** - Close focused pane

- **`rename-pane`** - Rename focused pane
  - `zellij action rename-pane "name"`

- **`toggle-fullscreen`** - Toggle focused pane fullscreen

- **`toggle-floating-panes`** - Toggle floating pane visibility

- **`toggle-pane-embed-or-floating`** - Switch pane embedding mode

#### Terminal I/O (CRITICAL FOR DEMO SCRIPTS)
- **`write-chars <CHARS>`** - Write text to focused pane
  - `zellij action write-chars "text"`
  - Text is NOT automatically executed (must append with Enter)
  - **Limitation**: No arguments passed to command, raw string only
  - **Demo use**: Send shell commands to terminals

- **`write [BYTES]...`** - Write raw bytes to pane
  - Lower-level than write-chars
  - Useful for special characters/escape sequences

#### Query/Inspection
- **`query-tab-names`** - List all tab names in current session
  - Output: Tab names, one per line
  - **Demo use**: Verify tab creation, script checkpoints

- **`dump-layout`** - Output current layout as KDL
  - Shows full pane tree, plugins, configurations
  - **Demo use**: Debugging, verify layout structure

- **`dump-screen`** - Dump focused pane contents to file
  - `zellij action dump-screen <FILENAME>`

#### Scroll/Display
- **`scroll-up`** / **`scroll-down`** - Scroll in focused pane
- **`page-scroll-up`** / **`page-scroll-down`** - Page scrolling
- **`scroll-to-top`** / **`scroll-to-bottom`** - Jump to edges
- **`half-page-scroll-up`** / **`half-page-scroll-down`**

#### Swap Layouts / Fullscreen
- **`next-swap-layout`** / **`previous-swap-layout`** - Cycle layout modes
- **`toggle-pane-frames`** - Toggle decorative frames

#### Mode Switching
- **`switch-mode [locked|pane|tab|resize|move|search|session]`** - Change input mode

#### Other
- **`run <COMMAND>...`** - SPECIAL: Launch command in new pane
  - `zellij run -d [right|down] -- command arg1 arg2`
  - Supports: `-c` (cwd), `-n` (pane name), `-f` (floating), `--close-on-exit`
  - **Demo use**: Spawn Claude Code sessions in panes directly

- **`edit <FILE>`** - Open file in new pane with $EDITOR

---

## 2. Pipe Command Syntax & Protocol

### Basic Pipe Syntax

```bash
# Send to all running plugins listening on "pipe_name"
zellij pipe --name "pipe_name" -- "arbitrary_payload"

# Send to specific plugin by URL
zellij pipe --name "pipe_name" \
  --plugin "file:/path/to/plugin.wasm" \
  -- "payload"

# With args (passed to plugin)
zellij pipe --name "pipe_name" --args "arg1 arg2" -- "payload"

# With plugin configuration
zellij pipe --name "pipe_name" \
  --plugin-configuration "key=value" \
  -- "payload"

# STDIN mode: Pipe data through, get output on STDOUT
tail -f logfile | zellij pipe --name "logs" --plugin "..." | wc -l
```

### cc-deck Plugin Integration Points

From `cc-zellij-plugin/src/pipe_handler.rs`, the cc-deck plugin recognizes:
- **`cc-deck:navigate`** - Enter/toggle navigation mode
- **`cc-deck:attend`** - Run smart attend algorithm
- **`cc-deck:hook`** - Hook event (internal)
- **`cc-deck:sync`** - Synchronize state
- **`cc-deck:request`** - Request state
- **`cc-deck:rename`** - Rename session

### Pipe Message Format

Messages are delivered as JSON or plain text depending on implementation. The cc-deck plugin parses:
```rust
PipeAction::Navigate
PipeAction::Attend
PipeAction::HookEvent(hook)
PipeAction::SyncState(payload)
PipeAction::RequestState
PipeAction::Rename
```

**Important**: The pipe_name MUST match the handler in the plugin. For cc-deck, use names like `cc-deck:navigate`, `cc-deck:attend`, etc.

---

## 3. cc-deck Layout Structure (from dump-layout output)

The cc-deck plugin operates in **sidebar mode**: each tab gets a split layout with:

```kdl
tab name="example" hide_floating_panes=true {
    pane size=1 borderless=true {
        plugin location="zellij:tab-bar"   // Top bar
    }
    pane split_direction="vertical" {
        pane size=22 borderless=true {
            plugin location="file:.../cc_deck.wasm" {
                attend_key "Alt a"
                mode "sidebar"
                navigate_key "Alt s"
            }
        }
        pane cwd="..." {
            // Main working pane (where commands run)
        }
    }
    pane size=2 borderless=true {
        plugin location="zellij:status-bar"  // Bottom bar
    }
}
```

**Layout Characteristics**:
- Sidebar plugin (22 char wide) on left of every tab
- Main pane receives write-chars commands
- Tab bar + status bar are built-in Zellij UI
- Mode is "sidebar" (configurable at install time)
- Navigation/attend keybindings are registered globally

---

## 4. Key Limitations for Demo Scripting

### Cannot Do (Must Use Alternative)
1. **No direct pane ID targeting in write-chars**
   - `write-chars` writes to **focused pane only**
   - **Workaround**: Use `move-focus` or `go-to-tab-name` first, then write-chars

2. **No keybinding simulation without plugin cooperation**
   - Cannot directly press Alt+S or Alt+A from script
   - **Workaround**: Use `zellij pipe` to send commands to cc-deck plugin

3. **No conditional waits based on output**
   - Actions don't wait for command completion
   - **Workaround**: Use `write-chars` with shell `&& next_action` chaining, or external polling

4. **No return values from actions**
   - Actions complete silently
   - **Workaround**: Use `query-tab-names` or `dump-layout` for inspection, parse output

5. **No timeout control**
   - Long-running commands block recording
   - **Workaround**: Use shell `timeout` command or background `&` with explicit waits

### Can Do (Fully Supported)
1. ✓ Create/destroy tabs with custom names
2. ✓ Create/destroy panes with direction
3. ✓ Navigate between tabs and panes programmatically
4. ✓ Send arbitrary text to focused pane
5. ✓ Rename tabs and panes dynamically
6. ✓ Query current session structure
7. ✓ Launch commands in new panes (via `zellij run`)
8. ✓ Toggle fullscreen, floating panes
9. ✓ Send pipe messages to plugins (if they listen)

---

## 5. Demo Script Architecture Recommendations

### Proposed Structure

```bash
#!/bin/bash
# demo-plugin.sh - Scripted cc-deck plugin demo

SESSION_NAME="cc-deck-demo-$$"
DEMO_DIR="/tmp/cc-deck-demo-projects"

# Phase 1: Setup (one-time, no recording)
setup_demo_environment() {
    # Create demo projects (Python, Go, HTML)
    # Each with git history, CLAUDE.md, pre-staged task
    mkdir -p "$DEMO_DIR"
    # ... project scaffolding ...
}

# Phase 2: Start recording
start_recording() {
    asciinema rec --overwrite --command "zellij --session $SESSION_NAME" recording.json
}

# Phase 3: Execute demo scenes (during recording)
scene_plugin_demo() {
    # Scene 1: Install plugin
    zellij action write-chars "cc-deck plugin install"
    sleep 2

    # Scene 2: Create multiple Claude Code sessions
    zellij action new-tab -n "python-project" -c "$DEMO_DIR/python"
    zellij action write-chars "claude"
    sleep 5  # Wait for Claude Code to start

    zellij action new-tab -n "go-project" -c "$DEMO_DIR/go"
    zellij action write-chars "claude"
    sleep 5

    zellij action new-tab -n "web-project" -c "$DEMO_DIR/web"
    zellij action write-chars "claude"
    sleep 5

    # Scene 3: Navigation mode demo
    zellij pipe --name "cc-deck:navigate" -- ""   # Toggle nav mode
    sleep 1
    zellij pipe --name "cc-deck:navigate" -- "down"   # Move cursor down (if supported)
    sleep 1

    # Scene 4: Smart attend demo
    zellij pipe --name "cc-deck:attend" -- ""   # Trigger attend
    sleep 3

    # ... more scenes ...
}

# Phase 4: Cleanup (after recording stops)
cleanup() {
    rm -rf "$DEMO_DIR"
    zellij kill-session -y "$SESSION_NAME" 2>/dev/null || true
}
```

### Key Techniques

**Using write-chars effectively:**
```bash
# Bad: Commands won't execute
zellij action write-chars "ls -la"

# Good: Explicit Enter key (via special sequence)
zellij action write-chars "ls -la"
zellij action write-chars ""  # Empty string simulates Enter (implementation-dependent)

# Better: Use shell && chaining
zellij action write-chars "cd myproject && claude --model opus"

# Best: Use shell constructs for sync
zellij action write-chars "command && echo DONE"
# Then poll for DONE in subsequent output checks
```

**Creating interactive demo projects:**
```bash
# Each demo project needs:
# 1. Git repository with history
git init demo-project
git config user.email "demo@example.com"
git config user.name "Demo"
echo "# Demo Project" > README.md
git add README.md
git commit -m "Initial commit"
# ... more commits to build history ...

# 2. CLAUDE.md with pre-staged task
cat > .claude/CLAUDE.md << 'EOF'
# Demo Task

Fix the bug in src/main.py where the function returns None instead of the sum.
EOF

# 3. Small source file with intentional bug
mkdir -p src
cat > src/main.py << 'EOF'
def add_numbers(a, b):
    """Add two numbers."""
    # BUG: Missing return statement
    result = a + b

def main():
    print(add_numbers(3, 5))

if __name__ == "__main__":
    main()
EOF
git add .
git commit -m "Add buggy implementation"
```

**Checkpoint-based timing (instead of fixed sleeps):**
```bash
# For Claude Code tasks, wait for "Done" status
wait_for_claude_done() {
    local timeout=120
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        # Check if Claude finished (look for completion marker)
        if grep -q "Work complete" <(zellij action dump-screen /tmp/pane-$$.txt); then
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1  # Timeout
}

# For Zellij state changes, query tabs
wait_for_tab_created() {
    local tab_name="$1"
    local timeout=10
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        if zellij action query-tab-names | grep -q "^$tab_name$"; then
            return 0
        fi
        sleep 0.5
        elapsed=$((elapsed + 0.5))
    done
    return 1
}
```

---

## 6. Pipe Command Design for cc-deck Plugin

### Current Plugin Message Handlers

From `cc-zellij-plugin/src/pipe_handler.rs`, the plugin can receive:
- **`cc-deck:navigate`** - Toggle navigation mode
- **`cc-deck:attend`** - Execute smart attend
- Additional messages parsed via `PipeAction` enum

### CRITICAL ISSUE: No Demo-Specific Commands Yet

**The spec (020-demo-recordings) requires new pipe commands for**:
- `navigate:toggle` - Enter/exit navigation mode
- `navigate:down` / `navigate:up` - Move cursor in navigation
- `navigate:select` - Select session and switch
- `attend` - Run smart attend

**These DO NOT exist yet in the plugin code.** The spec notes:
> "The plugin must accept pipe messages that trigger navigation mode toggle, cursor movement (up/down), session selection, smart attend, pause toggle, and help display."

This is captured in FR-001 of the spec and is a BLOCKING REQUIREMENT for demo scripts.

---

## 7. Complete Zellij Actions Reference (Alphabetical)

| Action | Purpose | Demo Use |
|--------|---------|----------|
| change-floating-pane-coordinates | Move floating pane | Position floating help |
| clear | Clear pane buffer | Clean up output |
| close-pane | Close focused pane | Cleanup after demo |
| close-tab | Close tab | Cleanup |
| dump-layout | Export current layout | Debugging, checkpoints |
| dump-screen | Export pane contents | Capture output for analysis |
| edit | Open file in editor | N/A for demo |
| edit-scrollback | Edit scrollback buffer | N/A |
| focus-next-pane | Move focus forward | Navigate between panes |
| focus-previous-pane | Move focus backward | Navigate between panes |
| go-to-next-tab | Next tab | Adjacent navigation |
| go-to-previous-tab | Previous tab | Adjacent navigation |
| go-to-tab | Jump to tab by index | Jump to specific demo project |
| go-to-tab-name | Jump to tab by name | **BEST FOR DEMO**: Named projects |
| half-page-scroll-{up,down} | Scroll half-page | Show more output |
| launch-or-focus-plugin | Launch/focus floating plugin | Picker mode (future) |
| launch-plugin | Launch plugin | N/A |
| list-clients | List connected clients | N/A |
| move-focus | Move focus in direction | Navigate workspace |
| move-focus-or-tab | Move/tab-switch on edge | Smart navigation |
| move-pane | Rearrange pane | Layout changes |
| move-pane-backwards | Rotate panes | Layout changes |
| move-tab | Move tab left/right | Reorder projects |
| new-pane | Create pane | Split workspace |
| new-tab | Create tab | **CRITICAL FOR DEMO**: Project tabs |
| next-swap-layout | Cycle layout | Toggle fullscreen |
| page-scroll-{up,down} | Full-page scroll | Show output |
| pipe | Send message to plugin | **CRITICAL FOR DEMO**: Plugin control |
| previous-swap-layout | Previous layout | Layout cycling |
| query-tab-names | List tabs | **FOR SCRIPTING**: Checkpoints |
| rename-pane | Rename pane | Label panes |
| rename-session | Rename session | N/A |
| rename-tab | Rename tab | **FOR DEMO**: Label projects |
| resize | Resize pane border | Layout adjustments |
| scroll-{up,down} | Single-line scroll | Show output |
| scroll-to-{top,bottom} | Jump to edge | Output navigation |
| stack-panes | Stack pane IDs | Complex layouts |
| start-or-reload-plugin | Plugin lifecycle | N/A |
| switch-mode | Change input mode | Change Zellij mode |
| toggle-active-sync-tab | Sync all panes | Type in multiple panes |
| toggle-floating-panes | Show/hide floating | UI toggles |
| toggle-fullscreen | Toggle focused fullscreen | Focus mode |
| toggle-pane-embed-or-floating | Embed/float pane | Layout mode |
| toggle-pane-frames | Show/hide pane borders | UI toggles |
| toggle-pane-pinned | Pin floating pane | UI control |
| undo-rename-pane | Revert pane name | Undo |
| undo-rename-tab | Revert tab name | Undo |
| write | Write raw bytes | Terminal control |
| write-chars | **Write text to pane** | **CRITICAL FOR DEMO**: Send commands |

---

## 8. Testing Capabilities

All actions can be tested directly:

```bash
# Test tab creation
zellij action new-tab -n "test-tab"
zellij action query-tab-names | grep "test-tab"  # Should find it

# Test write-chars
zellij action go-to-tab-name "test-tab"
zellij action write-chars "echo hello"
zellij action write-chars ""  # Press Enter

# Test pipe (if plugin running)
zellij pipe --name "cc-deck:navigate" -- "toggle"

# Test layout inspection
zellij action dump-layout | grep -A 5 "test-tab"
```

---

## 9. Blockers & Recommendations for Implementation

### BLOCKERS
1. **Plugin must implement demo-specific pipe handlers** (FR-001)
   - Status: NOT YET IMPLEMENTED
   - Spec says plugin must accept `navigate:toggle`, `navigate:down`, `navigate:up`, `navigate:select`, `attend`
   - These are critical for demo scripts to control plugin without key simulation

2. **write-chars behavior with Enter key unclear**
   - Testing shows `write-chars ""` may not work as expected on all systems
   - Recommend testing with explicit shell `\n` or using shell constructs: `command && echo DONE`

### RECOMMENDATIONS

1. **Demo Script Helper Library** (shell functions)
   - Abstracts tab/pane creation, common patterns
   - Provides checkpoint-based waits
   - Handles error cases (timeout, Zellij crash)

2. **Plugin Demo Command Set** (pipe handlers)
   - Implement at minimum: navigate-toggle, navigate-down, navigate-up, navigate-select, attend
   - Consider: pause-toggle, help-toggle for complete control

3. **Asciinema Integration**
   - Stable recording tool with JSON output
   - Can be trimmed/converted to MP4/GIF easily
   - Provides chapter markers for voiceover alignment

4. **Checkpoint Strategy**
   - Use `query-tab-names` to verify tab creation
   - Use `dump-screen` + grep to check output patterns
   - Use shell `timeout` for long-running commands
   - Chain actions with `&&` for sequential guarantees

5. **Demo Project Template**
   - Create generator script (Python, Go, Web templates)
   - Each has git history (3+ commits)
   - Each has CLAUDE.md with clear task
   - Each sized for ~2min Claude Code task

---

## Summary: What Works & What Doesn't

### ✓ Fully Functional for Demos
- Tab creation/navigation/renaming
- Pane creation/navigation/layout control
- Terminal I/O via `write-chars`
- Session introspection via `query-tab-names` and `dump-layout`
- Plugin communication via `pipe` (once plugin handlers exist)
- Direct command execution via `zellij run`

### ✗ NOT Functional for Demos (Requires Alternatives)
- Direct plugin keybinding simulation (use `pipe` instead)
- Conditional waits on output (use `dump-screen` + polling)
- Direct pane targeting in write-chars (navigate first, then write)

### ⚠ CRITICAL DEPENDENCY
- **Plugin must implement demo pipe handlers** - currently missing
- This blocks full remote-control capability
- Estimated effort: ~2-3 hours for plugin implementation

---

## References
- Zellij CLI help: `zellij action --help`, `zellij pipe --help`, `zellij run --help`
- cc-deck plugin code: `/Users/rhuss/Development/ai/mcp/cc-deck/cc-zellij-plugin/src/`
- 020-demo-recordings spec: `/Users/rhuss/Development/ai/mcp/cc-deck/specs/020-demo-recordings/spec.md`
- Test environment: macOS 25.3.0 with Zellij latest, cc-deck main branch
