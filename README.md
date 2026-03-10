# cc-deck

A Zellij sidebar plugin for managing multiple Claude Code sessions. Track activity, switch between sessions, and navigate with keyboard shortcuts.

## Install

```bash
make install
```

This installs the WASM plugin, layout files, and Claude Code hooks.

## Usage

```bash
zellij --layout cc-deck
```

Or set as default in `~/.config/zellij/config.kdl`:

```kdl
default_layout "cc-deck"
```

## Layout Variants

Three layout styles are installed:

| Layout | Command | Description |
|--------|---------|-------------|
| `standard` | `zellij --layout cc-deck` | Sidebar + tab-bar + status-bar (default) |
| `minimal` | `zellij --layout cc-deck-minimal` | Sidebar + compact-bar |
| `clean` | `zellij --layout cc-deck-clean` | Sidebar only, no bars |

To change the default variant:

```bash
cc-deck plugin install --layout standard
```

## Keyboard Shortcuts

### Global (from any tab)

| Key | Action |
|-----|--------|
| `Alt+s` | Open session list / cycle through sessions |
| `Alt+a` | Jump to next session needing attention |

### Session List (navigation mode)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Switch to selected session |
| `Esc` | Cancel (return to original session) |
| `r` | Rename session |
| `d` | Delete session (with confirmation) |
| `p` | Pause/unpause session |
| `n` | New tab |
| `/` | Search/filter by name |
| `?` | Show keyboard help |

### Mouse

| Action | Effect |
|--------|--------|
| Left-click session | Switch to that session |
| Right-click session | Rename session |
| Click [+] | New tab |

## Customizing Keybindings

### Plugin Shortcuts (Alt+s, Alt+a)

Edit the plugin config in the layout file (`~/.config/zellij/layouts/cc-deck.kdl`):

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Super s"    // default: "Alt s"
    attend_key "Super n"      // default: "Alt a"
}
```

Key syntax follows [Zellij key format](https://zellij.dev/documentation/keybindings.html): `Alt`, `Ctrl`, `Super` (Cmd on macOS), `Shift` as modifiers, followed by the key character.

After editing, restart Zellij to apply.

> **Note**: `make install` overwrites the managed layout files (`cc-deck.kdl`, `cc-deck-standard.kdl`, `cc-deck-clean.kdl`). Use a **personal layout file** to preserve custom keybindings across reinstalls.

### Personal Layout (Recommended)

Create a personal layout that won't be overwritten by `make install`:

```bash
# Copy the base layout
cp ~/.config/zellij/layouts/cc-deck.kdl ~/.config/zellij/layouts/cc-deck-personal.kdl
```

Edit `~/.config/zellij/layouts/cc-deck-personal.kdl` and add your custom keys to the plugin blocks:

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Super s"
    attend_key "Super n"
}
```

Then set it as default in `~/.config/zellij/config.kdl`:

```kdl
default_layout "cc-deck-personal"
```

Now `zellij` (without `--layout`) uses your personal keybindings automatically.

### Using Cmd Keys (macOS + Ghostty)

To use Cmd-based shortcuts, configure Ghostty to pass Cmd keys through to Zellij:

**Ghostty** (`~/.config/ghostty/config`):

```
keybind = cmd+s=unbind
keybind = cmd+n=unbind
```

### Other Useful Cmd Key Bindings

For a native macOS feel inside Zellij, add to `~/.config/zellij/config.kdl`:

```kdl
keybinds {
    shared_except "locked" {
        bind "Super t" { NewTab; }
        bind "Super w" { CloseTab; }
        bind "Super Alt Left" { GoToPreviousTab; }
        bind "Super Alt Right" { GoToNextTab; }
    }
}
```

And unbind in Ghostty:

```
keybind = cmd+t=unbind
keybind = cmd+w=unbind
keybind = cmd+opt+left=unbind
keybind = cmd+opt+right=unbind
```

## Session States

| Icon | State | Description |
|------|-------|-------------|
| ◆ | Init | Session just started |
| ● | Working | Actively processing |
| ⚙ | ToolUse | Using a tool (shows tool name) |
| ⚠ | Waiting | Needs user input (permission request) |
| ○ | Idle | No recent activity |
| ✓ | Done | Task completed |
| ⏸ | Paused | Excluded from attend cycling |

## Smart Attend (Alt+a)

Cycles through sessions in priority order:

1. **Permission requests** (oldest first, most urgent)
2. **Idle/done/init sessions** (newest first)
3. **Skips** working sessions and paused sessions

Subsequent presses cycle round-robin through the list.

## Build from Source

```bash
# Prerequisites
rustup target add wasm32-wasip1
go install (Go 1.22+)

# Build and install
make install
```

## Uninstall

```bash
cc-deck plugin remove
```
