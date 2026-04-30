# Research: Voice Attend Stop Word

**Date:** 2026-04-30

## Decision: Command Protocol Extension

- **Decision**: Add `"attend"` action mapped to `[[attend]]` command on the `cc-deck:voice` pipe
- **Rationale**: Consistent with existing `[[command]]` protocol from spec 045. The pipe handler already parses `[[...]]` syntax and dispatches via `handle_voice_command()`. Adding a new match arm is minimal.
- **Alternatives considered**:
  - Generic `[[action:NAME]]` protocol: Over-engineered for two actions, breaks existing `[[enter]]` convention
  - Sending on `cc-deck:attend` pipe directly: Mixes pipe routing into stop word processing

## Decision: DefaultCommands Extension

- **Decision**: Add `"attend": {"next"}` to `DefaultCommands` alongside `"submit": {"send"}`
- **Rationale**: The `BuildCommandMap` function already supports multiple actions with multiple words. No structural changes needed.
- **Alternatives considered**: Separate config mechanism. Rejected because the existing `commands` map already handles this.

## Decision: Attend Function Access

- **Decision**: Call `perform_attend()` from `handle_voice_command()` in `controller/mod.rs`
- **Rationale**: `handle_attend()` in `controller/actions.rs` wraps `perform_attend_directed()`. The voice command handler in `controller/mod.rs` has access to `&mut self` (ControllerState), so it can call the same attend logic directly.
- **Alternatives considered**: Routing through the action dispatch system. Rejected as unnecessary indirection for a simple function call.

## Code Locations

| Change | File | Lines |
|--------|------|-------|
| Add "attend" to DefaultCommands | `cc-deck/internal/voice/stopword.go` | 17-19 |
| Add "attend" case to relay dispatch | `cc-deck/internal/voice/relay.go` | 429-440 |
| Add "attend" match to handle_voice_command | `cc-zellij-plugin/src/controller/mod.rs` | 456-507 |
| Add stopword tests | `cc-deck/internal/voice/stopword_test.go` | append |
| Add relay dispatch test | `cc-deck/internal/voice/relay_test.go` | append |
