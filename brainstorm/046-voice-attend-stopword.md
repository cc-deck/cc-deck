# Brainstorm: Voice Attend Stop Word

**Date:** 2026-04-30
**Status:** active

## Problem Framing

The voice relay supports a configurable stop word ("send") that triggers the submit action (sends Enter to the attended pane).
There is no voice command to cycle to the next attended session.
Users must reach for Alt+a to switch sessions, breaking the hands-free voice workflow.

## Approaches Considered

### A: New action + new [[attend]] command
- Add `"attend": {"next"}` to `DefaultCommands` in `stopword.go`
- Relay maps `"attend"` action to `"[[attend]]"` payload on `cc-deck:voice` pipe
- Plugin voice pipe handler recognizes `[[attend]]` and calls `perform_attend_next()`
- Configurable via the same `commands` map as "send"
- Pros: Clean separation of actions, minimal new code, consistent with `[[command]]` protocol from spec 045
- Cons: Requires plugin-side handler for the new command

### B: Generic [[action:NAME]] protocol
- Replace specific commands (`[[enter]]`, `[[attend]]`) with `[[action:submit]]`, `[[action:attend]]`
- Plugin dispatches generically based on action name
- Pros: Extensible, single protocol pattern
- Cons: Over-engineered for two actions, breaks compatibility with existing `[[enter]]` from spec 045

### C: Reuse existing pipe routing
- Stop word "next" causes relay to send on `cc-deck:attend` pipe (same as keybinding) instead of `cc-deck:voice`
- No plugin voice handler changes needed
- Pros: Reuses existing attend infrastructure completely
- Cons: Mixes pipe routing logic into stop word processing, relay needs to know about multiple pipes

## Decision

**Approach A** chosen. New action with `[[attend]]` command on the existing voice pipe.

Rationale: Consistent with the `[[command]]` protocol established in spec 045, clean action separation, and the `commands` map already supports multiple actions with configurable trigger words.

## Design Decisions

### Behavior matches Alt+a
- Uses the same tiered attend logic: waiting sessions first, then done, then idle
- Consistent behavior whether using voice or keyboard
- No reverse direction ("prev"/"back") for voice, keyboard Alt+A covers that

### Configuration
- Extends the existing `DefaultCommands` map with `"attend": {"next"}`
- Users can override the trigger word via the same config mechanism as "send"
- Same filler tolerance applies ("um, next" triggers the command)

### Standalone detection
- "next" embedded in a sentence (e.g., "the next step is") does NOT trigger the command
- Only standalone utterances (after filler stripping) activate the action

## Open Threads
- Whether additional voice actions beyond "submit" and "attend" will be needed in the future
