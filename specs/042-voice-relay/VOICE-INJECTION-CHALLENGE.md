# Voice Relay Text Injection: Investigation Summary

**Date**: 2026-04-27
**Branch**: `042-voice-relay`
**Status**: Blocked on pane injection

## What works

- **Audio pipeline**: Capture (malgo/ffmpeg), VAD, 16kHz mono, adjustable threshold
- **Transcription**: whisper-server HTTP, JSON response parsing, artifact filtering
- **Stopword detection**: "submit"/"enter" command words with filler stripping
- **PipeChannel delivery**: Go CLI sends text via `zellij pipe --name cc-deck:voice`, plugin receives it reliably. Works across all workspace types (local, SSH, container, k8s) via `ZELLIJ_SESSION_NAME` env var targeting.
- **Plugin pane resolution**: Plugin correctly resolves target pane (attended -> focused -> first session)
- **TUI**: Braille level bar, color gradient, threshold controls, device picker, verbose logging
- **`zellij action write-chars`**: The Zellij CLI command works perfectly for text injection, both with and without `--pane-id`

## What does not work

**Plugin `write_chars_to_pane_id()` API**: The plugin calls `zellij_tile::prelude::write_chars_to_pane_id(chars, PaneId::Terminal(pane_id))`. The call does not error. Debug logs confirm it executes. But no text appears in the target pane.

## Approaches tried

### 1. Plugin-side injection (current, broken)

Flow: Go -> `zellij pipe` -> plugin pipe handler -> `write_chars_to_pane_id()`

- Plugin receives text, resolves attended pane, calls the API
- Logs show `CTRL VOICE injected N chars to pane=X`
- No text appears in the pane
- Added `WriteToStdin` permission: no change
- Tried after fresh `make install` + session restart: no change

### 2. CLI write-chars without pane-id

Flow: Go -> `zellij action write-chars <text>` with `ZELLIJ_SESSION_NAME`

- Works, but targets the **focused** pane, not the attended session
- If the voice relay pane has focus, text goes to the voice relay itself
- Not useful for the intended workflow

### 3. CLI write-chars with pane-id from dump-state

Flow: Go -> `zellij pipe cc-deck:dump-state` -> parse response -> `zellij action write-chars --pane-id terminal_N`

- `dump-state` returns session data from the **wrong controller instance** because multiple Zellij sessions share the same plugin WASM cache
- Picks a pane from a different Zellij session entirely
- Fundamentally broken for multi-session setups

### 4. Hybrid: plugin resolves pane, CLI injects

Flow: Go -> `zellij pipe cc-deck:voice` (with SendReceive) -> plugin responds with `pane:N` -> Go calls `zellij action write-chars --pane-id terminal_N`

- Solves the pane resolution problem (plugin knows the right pane)
- But requires local `zellij` CLI, breaking remote workspace support
- Remote workspaces (SSH, k8s, container) have no local `zellij` binary

## Root tension

| Capability | Plugin API | CLI action |
|---|---|---|
| Inject text into specific pane | `write_chars_to_pane_id` (broken) | `write-chars --pane-id` (works) |
| Know which pane is attended | Yes (has state) | No (needs dump-state, unreliable) |
| Work remotely | Yes (runs inside Zellij) | No (needs local `zellij` binary) |

The plugin has the knowledge (which pane) but not the working injection mechanism. The CLI has the working injection but not the knowledge or remote capability.

## Key question

**Why does `write_chars_to_pane_id` in the WASM plugin produce no visible output while `zellij action write-chars --pane-id` from the CLI does the same thing successfully?**

Possibilities to investigate:
1. **Permission not granted**: The plugin requests `WriteToStdin` but Zellij may not prompt for re-grant on session restart if previously granted without it. Try a completely fresh session name.
2. **API behavioral difference**: `write_chars_to_pane_id` might write to the pane's stdin but not to the PTY. The CLI action might use a different code path internally.
3. **Zellij version issue**: The API might have changed behavior between versions. Current: Zellij 0.44.1, zellij-tile 0.43.1.
4. **Pane ID mismatch**: The plugin's pane ID (from its session tracking) might differ from what Zellij internally uses. The plugin tracks pane IDs from hook events, but the PaneManifest might assign different IDs.
5. **Cross-tab restriction**: If the target pane is on a different tab than the controller plugin, `write_chars_to_pane_id` might silently fail.

## Recommended next step

Write a minimal test: add a new pipe action (e.g., `cc-deck:test-inject`) that calls `write_chars_to_pane_id` on the focused pane with hardcoded text. Compare the pane ID from the manifest with the one from session tracking. This isolates whether the issue is the API itself or the pane ID resolution.
