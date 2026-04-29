# Brainstorm: Voice Relay Sidebar Integration

**Date:** 2026-04-29
**Status:** active

## Problem Framing

Voice relay runs as a separate CLI process with its own TUI, but has no visual presence in the cc-deck sidebar. There is no way to see whether voice relay is connected, whether it is listening or muted, or to control it from the Zellij session. The current PTT mode adds complexity (long-poll pipe) for limited benefit compared to a simpler mute/unmute model.

Additionally, the pipe protocol mixes control signals with dictation text (e.g., sending raw `\r` for command words), which makes it harder to extend with new commands.

## Approaches Considered

### A: Minimal indicator only
- Add ♫ to status line, no interaction
- Pros: Simple, no protocol changes
- Cons: No mute control from sidebar, no extensible protocol

### B: Full sidebar integration with command protocol
- ♫ indicator with mute toggle (shortcut + click)
- `[[command]]` protocol for control signals
- Remove PTT, replace with mute/unmute
- Bidirectional state sync between CLI and plugin
- Pros: Complete solution, extensible, clean architecture
- Cons: Larger scope, touches both Rust plugin and Go CLI

### C: Keep PTT alongside mute
- Both PTT and mute modes available
- Pros: Maximum flexibility
- Cons: Complex state machine, long-poll pipe stays, confusing UX

## Decision

**Approach B** chosen. Full sidebar integration with command protocol.

## Design Decisions

### Voice indicator
- **Symbol:** ♫ (beamed eighth notes)
- **Placement:** Right-aligned on the status line (top row, next to session counts)
- **States:** Bright color = listening, dim color = muted, absent = disconnected
- **Rationale:** Consistent with existing abstract symbol language (●, ○, ⚠, ✓, ⏸)

### Mute toggle interactions
All three interaction methods:
1. **Global shortcut** `Alt+v` (configurable via `voice_key` in layout plugin config)
2. **Navigation mode** `v` key
3. **Click** on the ♫ symbol

### Command protocol
- Control signals use `[[command]]` syntax on the `cc-deck:voice` pipe
- Plain text payloads (no `[[` prefix) are injected via `write_chars_to_pane_id` as before
- Commands:
  - `[[enter]]` sends carriage return to attended pane
  - `[[voice:on]]` / `[[voice:off]]` set connection state in plugin
  - `[[voice:mute]]` / `[[voice:unmute]]` synchronize mute state
- Rationale: Wrapping text in `[[...]]` is redundant since text is the default. Only control signals need the special syntax.

### PTT removal
- Remove PTT mode and `--mode` CLI flag
- Remove voice-control long-poll pipe
- Repurpose `m` key in voice TUI for mute/unmute toggle
- Display mute state visually in TUI header
- Rationale: Mute/unmute is simpler (no long-poll pipe), covers the same use case (stopping dictation temporarily), and integrates naturally with the sidebar indicator

### Bidirectional mute sync
- Voice CLI sends `[[voice:mute]]` / `[[voice:unmute]]` when TUI mute changes
- Sidebar sends mute toggle back to voice CLI via pipe when toggled from sidebar shortcut or click
- Mute state is consistent across sidebar and voice TUI at all times

### Voice lifecycle
- Voice CLI sends `[[voice:on]]` on startup, `[[voice:off]]` on shutdown
- Plugin tracks connection state and re-renders sidebar accordingly

## Open Threads
- Exact color values for ♫ bright and dim states (should match existing indicator palette)
- Whether `[[command]]` protocol should be extensible to non-voice features in the future
- Click region sizing for the ♫ symbol (single character hit target)
- How mute toggle pipe response reaches the voice CLI (reverse pipe direction)
