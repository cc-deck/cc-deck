# Quickstart: Keyboard Navigation

**Feature**: 013-keyboard-navigation

## Prerequisites

- cc-deck plugin installed (`make install`)
- Zellij 0.43.1+ with cc-deck layout

## Build & Install

```bash
make dev-install
# Kill existing Zellij sessions to pick up new plugin
zellij kill-all-sessions
zellij --layout cc-deck
```

## Test Navigation Mode

1. Start one or more Claude sessions in different tabs
2. Press `Alt+s` to enter navigation mode (cursor `▶` appears)
3. Use `j`/`k` or arrow keys to move the cursor
4. Press `Enter` to switch to the selected session
5. Press `Esc` to exit navigation mode

## Test Smart Attend

1. Have multiple Claude sessions in different states
2. Press `Alt+a` to jump to the highest-priority session
3. Press `Alt+a` again to cycle to the next

## Test Search

1. Enter navigation mode (`Alt+s`)
2. Press `/` to enter search mode
3. Type a session name fragment
4. Press `Enter` to confirm, `Esc` to cancel

## Test Rename/Delete

1. Enter navigation mode, move cursor to a session
2. Press `r` to rename, type new name, press `Enter`
3. Press `d` to delete, press `y` to confirm

## Custom Keybindings

Configure in the layout file:

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Alt s"
    attend_key "Alt a"
}
```
