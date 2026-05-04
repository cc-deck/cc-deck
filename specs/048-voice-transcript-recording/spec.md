# Feature Specification: Voice Transcript Recording

**Feature Branch**: `048-voice-transcript-recording`
**Created**: 2026-05-04
**Status**: Draft
**Input**: User description: "Add transcript recording controls to the voice relay TUI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start and Stop a Transcript Recording (Priority: P1)

A user is dictating into a Claude Code session and wants to save what they say to a text file for later review. They press `r` to start recording, type a filename in the prompt, and press Enter. From that point, every transcribed utterance is appended to the file as plain text. When they are done, they press `R` to stop recording and close the file.

**Why this priority**: This is the core feature. Without start/stop, no transcript can be created.

**Independent Test**: Can be tested by starting the voice relay, pressing `r`, entering a filename, speaking several sentences, pressing `R`, then verifying the file contains each transcribed utterance on its own line.

**Acceptance Scenarios**:

1. **Given** the voice relay is running and no recording is active, **When** the user presses `r`, **Then** a filename prompt appears in the TUI
2. **Given** the filename prompt is visible, **When** the user types a name and presses Enter, **Then** recording begins and a red recording indicator appears in the header
3. **Given** the filename prompt is visible, **When** the user presses Escape, **Then** the prompt disappears and no recording starts
4. **Given** recording is active, **When** the user speaks and the system transcribes text, **Then** each transcription is appended as a new line in the transcript file
5. **Given** recording is active, **When** the user presses `R`, **Then** recording stops, the file is closed, and the recording indicator disappears
6. **Given** recording is active, **When** the user quits the TUI with `q`, **Then** the file is properly closed before exit

---

### User Story 2 - Pause and Resume Recording (Priority: P2)

A user is recording a transcript but needs to have a private conversation or take a break. They press `r` to pause recording. The TUI shows a paused indicator. Transcriptions continue to appear in the TUI history but are not written to the file. When ready, they press `r` again to resume recording.

**Why this priority**: Pause/resume adds essential control over what ends up in the transcript without stopping the entire recording session.

**Independent Test**: Can be tested by starting a recording, speaking, pressing `r` to pause, speaking more, pressing `r` to resume, speaking again, then stopping. The file should contain only the text from the recording (not paused) periods.

**Acceptance Scenarios**:

1. **Given** recording is active, **When** the user presses `r`, **Then** recording pauses and a yellow pause indicator replaces the red recording indicator
2. **Given** recording is paused, **When** the system transcribes speech, **Then** the transcription appears in the TUI history but is NOT written to the file
3. **Given** recording is paused, **When** the user presses `r`, **Then** recording resumes and the red recording indicator returns
4. **Given** recording is paused, **When** the user presses `R`, **Then** recording stops completely and the file is closed

---

### User Story 3 - Record While Muted (Priority: P3)

A user has the voice relay muted (not sending to the attended pane) but still wants to capture what they say in a transcript. When recording is active and the relay is muted, the system continues to transcribe speech locally and writes it to the transcript file, but does not send the text to any pane.

**Why this priority**: Enables capturing speech that the user deliberately does not want sent to the active session, such as notes-to-self or commentary.

**Independent Test**: Can be tested by starting a recording, pressing `m` to mute, speaking, then stopping recording. The file should contain the muted transcriptions.

**Acceptance Scenarios**:

1. **Given** recording is active and the relay is muted, **When** the user speaks, **Then** the speech is still transcribed and written to the transcript file
2. **Given** recording is active and the relay is muted, **When** the user speaks, **Then** the transcribed text is NOT sent to the attended pane
3. **Given** no recording is active and the relay is muted, **When** the user speaks, **Then** the speech is discarded (no transcription occurs, preserving current behavior)

---

### Edge Cases

- What happens when the user enters an empty filename? The system uses a default name based on the current timestamp (e.g., `transcript-2026-05-04T15-04-05.txt`).
- What happens when the file cannot be created (permissions, invalid path)? An error message is displayed in the TUI and recording does not start.
- What happens when disk space runs out during recording? The system displays an error and stops recording gracefully, closing the file.
- What happens when the user presses `R` without an active recording? Nothing happens (the key is ignored).
- What happens when the user presses `r` while the filename prompt is visible? The key is consumed by the text input (the letter "r" is typed into the filename field).
- What happens when the user enters an absolute path as the filename? The system uses the absolute path as-is instead of prepending the default directory.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a keyboard shortcut (`r`) to initiate transcript recording from the voice relay TUI
- **FR-002**: When initiating recording, the system MUST display a text prompt for the user to enter a filename
- **FR-003**: The system MUST suggest a default filename based on the current timestamp
- **FR-004**: The system MUST write each transcribed utterance as a single line of plain text (no timestamps, no metadata)
- **FR-005**: The system MUST allow the user to cancel the filename prompt without starting recording (via Escape)
- **FR-006**: The system MUST display a visual indicator in the TUI header showing the current recording state (recording, paused, or off)
- **FR-007**: The system MUST allow pausing and resuming recording via the `r` key while recording is active
- **FR-008**: While paused, transcriptions MUST continue to appear in the TUI history but MUST NOT be written to the transcript file
- **FR-009**: The system MUST allow stopping recording and closing the file via `R` (Shift+R)
- **FR-010**: When the TUI exits while recording is active, the system MUST close the transcript file properly
- **FR-011**: When the relay is muted AND recording is active, the system MUST continue transcribing speech and writing it to the transcript file (but MUST NOT send it to the attended pane)
- **FR-012**: When the relay is muted AND no recording is active, the system MUST discard audio without transcribing (preserving current behavior)
- **FR-013**: Relative filenames MUST be resolved against a standard transcript storage directory
- **FR-014**: Absolute filenames MUST be used as-is

### Key Entities

- **Transcript File**: A plain text file where each line is one transcribed utterance. Created when recording starts, closed when recording stops or the TUI exits.
- **Recording State**: The current state of the recording system: idle (no recording), prompting (awaiting filename), recording (actively capturing), or paused (file open but not capturing).
- **Filename Prompt**: A text input overlay that appears when the user initiates recording, allowing them to specify or accept the default filename.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can start, pause, resume, and stop transcript recording entirely via keyboard without leaving the voice relay TUI
- **SC-002**: The transcript file contains only the recognized text, with one utterance per line, no metadata
- **SC-003**: Pausing recording prevents new transcriptions from appearing in the file while the file remains open
- **SC-004**: Users can record transcriptions while the relay is muted, capturing speech that is not sent to any pane
- **SC-005**: The recording indicator accurately reflects the current state (recording, paused, or off) at all times
- **SC-006**: The transcript file is properly closed on both explicit stop and TUI exit
- **SC-007**: Unit tests cover the recording state machine, file output, and mute-bypass behavior

## Assumptions

- The voice relay TUI and transcription pipeline are functional (spec 045 voice sidebar integration is implemented)
- The existing TUI has keyboard shortcut infrastructure that can be extended with new key bindings
- The user's filesystem is writable at the default transcript storage location
- No concurrent access to the transcript file from other processes is expected
- Command words ("send", "next") are included in the transcript since they are valid transcriptions
