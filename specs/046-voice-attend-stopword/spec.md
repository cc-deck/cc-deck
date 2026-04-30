# Feature Specification: Voice Attend Stop Word

**Feature Branch**: `046-voice-attend-stopword`
**Created**: 2026-04-30
**Status**: Draft
**Input**: User description: "Add a configurable stop word (default: 'next') that triggers attend-next session cycling (same as Alt+a)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Voice-Driven Session Cycling (Priority: P1)

A user is dictating into a Claude Code session via voice relay. They finish giving instructions and want to move to the next session that needs attention. Instead of reaching for the keyboard to press Alt+a, they say "next" and the system cycles to the next attended session using the same tiered logic as Alt+a (waiting sessions first, then done, then idle).

**Why this priority**: This is the core feature. Without it, users must break their hands-free workflow to switch sessions.

**Independent Test**: Can be fully tested by starting the voice relay, having multiple sessions in various states, saying "next", and verifying the correct session receives focus.

**Acceptance Scenarios**:

1. **Given** voice relay is connected and multiple sessions exist with one in "waiting" state, **When** the user says "next" as a standalone utterance, **Then** the system cycles to the next waiting session (same behavior as Alt+a)
2. **Given** voice relay is connected, **When** the user says "um, next" (with filler words), **Then** the filler words are stripped and the attend action triggers
3. **Given** voice relay is connected, **When** the user says "the next step is to refactor", **Then** the text is treated as normal dictation (not a command) because "next" is not standalone

---

### User Story 2 - Custom Trigger Word Configuration (Priority: P2)

A user finds that the word "next" conflicts with their natural speech patterns (they frequently say "next" in sentences). They configure a different trigger word (e.g., "switch" or "hop") for the attend action via the same configuration mechanism used for the "send" stop word.

**Why this priority**: Configurability prevents the feature from becoming unusable for users whose speech patterns include frequent standalone use of the default word.

**Independent Test**: Can be tested by configuring a custom word in the commands map, saying that word, and verifying attend-next triggers while "next" no longer triggers it.

**Acceptance Scenarios**:

1. **Given** the user has configured `"attend": ["switch"]` in their commands config, **When** they say "switch" as a standalone utterance, **Then** the attend-next action triggers
2. **Given** the user has configured a custom word for "attend", **When** they say "next" as a standalone utterance, **Then** "next" is treated as normal dictation text

---

### User Story 3 - Multiple Trigger Words for Attend (Priority: P3)

A user configures multiple trigger words for the attend action (e.g., both "next" and "switch") so they have flexibility in their voice commands.

**Why this priority**: Nice-to-have flexibility, but the architecture supports it naturally since the commands map already allows multiple words per action.

**Independent Test**: Can be tested by configuring multiple words in the attend action's word list and verifying each triggers the attend behavior.

**Acceptance Scenarios**:

1. **Given** the user has configured `"attend": ["next", "switch"]`, **When** they say either "next" or "switch" as standalone utterances, **Then** the attend-next action triggers for both

---

### Edge Cases

- What happens when voice relay is connected but no sessions are eligible for attend cycling? The command is a no-op (same as pressing Alt+a with no eligible sessions).
- What happens when the user configures the same word for both "submit" and "attend"? The last-wins behavior of the command map applies (map key overwrite). Only one action will fire.
- What happens when the user says "next" while voice relay is muted? No transcription occurs, so no command is processed.
- What happens when the word is spoken with different casing ("Next", "NEXT")? Case-insensitive matching applies (existing behavior in `stripFillers`).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST add a default command mapping `"attend": {"next"}` to the `DefaultCommands` map alongside the existing `"submit": {"send"}`
- **FR-002**: The voice relay MUST map the `"attend"` action to the `"[[attend]]"` payload sent on the `cc-deck:voice` pipe
- **FR-003**: The plugin voice pipe handler MUST recognize the `"[[attend]]"` command and invoke the existing `perform_attend_next()` function
- **FR-004**: The `"attend"` action MUST use the same tiered attend logic as Alt+a (Tier 1: waiting, Tier 2: done, Tier 3: idle)
- **FR-005**: The trigger word for the "attend" action MUST be configurable via the same `commands` map mechanism used for the "send" stop word
- **FR-006**: The "next" trigger word MUST follow the same standalone detection rules as "send" (only fires when the entire utterance, after filler stripping, equals the trigger word)
- **FR-007**: The existing "submit"/"send" stop word behavior MUST remain unchanged

### Key Entities

- **Command Action**: A named action (e.g., "submit", "attend") that maps to a specific `[[command]]` payload
- **Trigger Word**: A spoken word that, when detected as a standalone utterance, activates its associated command action
- **Command Map**: The bidirectional mapping from trigger words to actions, built by `BuildCommandMap()`

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can cycle to the next attended session by voice command without touching the keyboard
- **SC-002**: The voice command triggers the same session as pressing Alt+a would in the same state
- **SC-003**: Users can change the trigger word through configuration and have it take effect immediately
- **SC-004**: Existing "send" stop word continues to function identically after the change
- **SC-005**: Unit tests cover the new action mapping, relay dispatch, and standalone detection for "next"

## Assumptions

- Voice relay and the `[[command]]` protocol from spec 045 are implemented and functional
- The `cc-deck:voice` pipe handler in the plugin already processes `[[enter]]` commands and can be extended to handle `[[attend]]`
- The `perform_attend_next()` function is accessible from the voice pipe handler context
- No reverse direction ("prev"/"back") stop word is needed; keyboard Alt+A covers that use case
