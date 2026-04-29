# Quickstart: Voice Sidebar Integration

## What Changed

Voice relay now shows a ♫ indicator in the sidebar and supports mute/unmute from both the sidebar and voice TUI. Push-to-talk (PTT) mode has been removed and replaced with the simpler mute toggle.

## Using Voice Mute

### From the Sidebar (any pane)
- **Alt+v**: Toggle mute (configurable via `voice_key` in plugin config)
- **Navigation mode + v**: Toggle mute while navigating sessions
- **Click ♫**: Toggle mute by clicking the indicator

### From the Voice TUI
- **m**: Toggle mute

### Visual Feedback
- **♫ bright green**: Voice relay is connected and listening
- **♫ dim**: Voice relay is connected but muted
- **No ♫**: Voice relay is not connected

## Command Protocol

Voice relay now uses a structured protocol instead of raw text for control signals:
- Dictated text is sent as plain text (unchanged behavior)
- Command words ("send" by default) result in `[[enter]]` being sent instead of the raw text
- Connection status (`[[voice:on]]`/`[[voice:off]]`) and mute state (`[[voice:mute]]`/`[[voice:unmute]]`) are communicated as control messages

## Configuration

Add to your Zellij layout KDL plugin config:

```kdl
plugin location="file:cc-deck.wasm" {
    voice_key "Alt v"    // default, change to preferred key
}
```

## Removed: PTT Mode

The `--mode ptt` flag and F8 keybinding for push-to-talk have been removed. Use the mute toggle instead, which provides the same "stop listening" functionality with less complexity.
