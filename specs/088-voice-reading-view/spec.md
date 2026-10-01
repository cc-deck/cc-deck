# Feature Specification: Voice Reading View with Turn Detection

**Feature Branch**: `088-voice-reading-view`

**Created**: 2026-10-01

**Status**: Draft

**Input**: User description: "Voice relay reading view with turn detection. Source brainstorm: brainstorm/095-voice-reading-view.md (GitHub issue #43)."

## Overview

The voice relay TUI doubles as a meeting and chat recorder, but its history pane is built for dictation debugging: one line per utterance with a status icon, a timestamp, and a latency figure.
When attention drifts during a call, there is no comfortable way to read back what was said in the last minutes.

This feature adds a reading view for the active recording that presents the conversation as wrapped paragraphs grouped into speaker turns, updates live, and scrolls like a pager.
It also makes recordings deliver text within seconds instead of in 30 second batches, adds an opt-in turn detection mode backed by a diarization-aware speech model, and writes turn breaks into the transcript file.

Speaker identity (labels such as "Me" or "S2") is explicitly out of scope.
The turn data carries an empty speaker slot so a later feature can add labels without changing the reading view.

## Clarifications

### Session 2026-10-01

- Q: What are the concrete recording defaults? → A: Sensitivity 20% on the existing 0 to 100 logarithmic scale (dictation default is about 44%), 1.0 second silence, 12 second maximum chunk, 3 second pause-break threshold.
- Q: Does the turn-aware model download block the TUI? → A: No. It runs in the background with progress in the footer; dictation and recording keep working; quitting the relay cancels it and removes the partial file; on success the mode becomes `tdrz` for the next recording.
- Q: What happens when the turn-aware transcription tool itself is missing? → A: Toggling to `tdrz` shows an error that names the missing tool and how to install it, offers no download, and the mode stays `basic`.
- Q: How long may one turn-aware transcription take before it counts as failed? → A: 30 seconds per passage; on timeout the passage falls back to the configured model as in any other failure.
- Q: What should verbose logging record for this feature? → A: Turn mode at recording start, each turn-aware invocation with duration and detected turn count, every fallback with its reason, and model download start, end, and failure.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read back the ongoing conversation (Priority: P1)

During a recorded call, the user gets distracted and misses part of the discussion.
They press `v` in the voice relay TUI and see the recording so far as readable chat blocks: one block per turn, each with the time it started, text merged and wrapped to the pane width.
They scroll up to the part they missed, read it, press `G` to jump back to the latest text, and press `esc` to return to the normal relay view.

**Why this priority**: This is the core value of the feature. Without it, the user has to open the transcript file in another tool or read a cluttered one-line-per-utterance list.

**Independent Test**: Start a recording, speak several sentences with pauses, press `v`, and verify the text appears as wrapped blocks that can be scrolled, followed, and left with `esc`.

**Acceptance Scenarios**:

1. **Given** a recording is active, **When** the user presses `v`, **Then** the reading view replaces the history pane and shows all text of the current recording as chat blocks.
2. **Given** no recording is active, **When** the user presses `v`, **Then** nothing happens and the footer does not advertise `v`.
3. **Given** a recording is active, **When** the user looks at the footer of the normal view, **Then** a `v: read` hint is visible.
4. **Given** the reading view is open and scrolled to the bottom, **When** new text is transcribed, **Then** the new text appears and the view stays pinned to the bottom.
5. **Given** the reading view is open and the user scrolled up, **When** new text is transcribed, **Then** the visible position does not move and an indicator shows how many new blocks arrived below.
6. **Given** the user scrolled up, **When** the user presses `G` or `End`, **Then** the view jumps to the last line and resumes following new text.
7. **Given** the reading view is open, **When** the user presses `esc`, **Then** the normal relay view returns and the recording continues uninterrupted.
8. **Given** the reading view is open, **When** the recording stops (by `R` or by a write error), **Then** the TUI returns to the normal view.
9. **Given** the reading view is open, **When** the pane is resized, **Then** the text re-wraps to the new width.

---

### User Story 2 - See text within seconds while recording (Priority: P2)

While recording, the user wants the reading view to keep up with the conversation so it works for live catch-up.
Instead of receiving text in fixed 30 second batches, each spoken passage appears a few seconds after a natural pause or after a short maximum chunk length.

**Why this priority**: The reading view is useful even with batch delay, but live catch-up after a distraction needs text that is at most a few seconds old.

**Independent Test**: Start a recording, speak one sentence followed by a pause, and measure the time until the text appears in the TUI.

**Acceptance Scenarios**:

1. **Given** a recording is active, **When** a speaker finishes a sentence and pauses briefly, **Then** the text appears within a few seconds, without waiting for a 30 second chunk to fill.
2. **Given** a recording is active and a speaker talks without pausing, **When** the recording maximum chunk length elapses, **Then** the text so far is transcribed and shown.
3. **Given** a quiet remote voice is played through the laptop speakers, **When** the recording runs with default settings, **Then** that voice is still captured and transcribed.
4. **Given** a recording stops, **When** the user dictates again, **Then** the dictation sensitivity and pause settings in effect before the recording are restored.

---

### User Story 3 - Opt into model-based turn detection (Priority: P2)

For English conversations, the user wants more reliable turn breaks than pauses and Whisper's occasional dash markers provide.
Before starting a recording, they press `g` to switch the turn mode from `basic` to `tdrz`.
The header always shows the current turn mode.
When the recording starts in `tdrz` mode, transcription switches to the turn-aware speech model for the duration of the recording and switches back afterwards.

**Why this priority**: Better turn breaks make the reading view far easier to follow, but the reading view already works with basic turn breaks, so this is an enhancement.

**Independent Test**: With the turn-aware model installed, toggle to `tdrz`, record a two-person exchange without long pauses between speakers, and verify that each speaker change starts a new block.

**Acceptance Scenarios**:

1. **Given** the normal view and no recording, **When** the user presses `g`, **Then** the turn mode toggles between `basic` and `tdrz` and the header shows the new mode.
2. **Given** a recording is active, **When** the user presses `g`, **Then** the turn mode does not change.
3. **Given** the turn-aware model is not installed, **When** the user toggles to `tdrz`, **Then** an error explains that the model is missing and asks whether to download it now (y/n); nothing downloads without confirmation.
4. **Given** the download prompt, **When** the user confirms, **Then** the model downloads with visible progress and the mode switches to `tdrz` on success; on failure an error is shown and the mode stays `basic`.
5. **Given** the download prompt, **When** the user declines, **Then** the mode stays `basic`.
6. **Given** the turn-aware transcription tool is not installed, **When** the user toggles to `tdrz`, **Then** an error names the missing tool and how to install it, no download is offered, and the mode stays `basic`.
7. **Given** `tdrz` mode is selected, **When** a recording starts, **Then** recording audio is transcribed with the turn-aware model and every detected speaker change starts a new block.
8. **Given** a `tdrz` recording stops, **When** the user dictates again, **Then** dictation uses the configured model as before, with no restart delay.
9. **Given** `tdrz` mode is selected, **When** the user dictates without recording, **Then** dictation uses the configured model.

---

### User Story 4 - Turn breaks in the transcript file (Priority: P3)

After a meeting, the user (or the `/meeting-notes` skill) reads the transcript file.
Turns are separated by blank lines, so the structure of the conversation survives outside the TUI.

**Why this priority**: Valuable for post-meeting processing, but secondary to the live reading experience.

**Independent Test**: Record an exchange with at least two turns, stop the recording, and verify the transcript file contains a blank line between the turns.

**Acceptance Scenarios**:

1. **Given** a recording with two or more turns, **When** the recording stops, **Then** the transcript file separates turns with exactly one blank line.
2. **Given** timestamps are enabled for the recording, **When** text is written, **Then** the existing timestamp format is kept for each written line.
3. **Given** a single passage contains a turn change, **When** it is written, **Then** it is split at the turn boundary into separate lines with a blank line between them.

---

### Edge Cases

- **Empty recording**: The reading view opened before any text arrives shows a waiting placeholder instead of an empty pane.
- **Paused recording**: `v` stays available while the recording is paused. Text transcribed during the pause is not written to the transcript file and does not appear in the reading view. The reading view header shows the paused state.
- **Filename prompt**: While the transcript filename prompt is open, `v` and `g` are not interpreted as commands (they are typed into the filename).
- **Long recordings**: The reading view shows the complete current recording, regardless of the 200-entry limit of the normal history pane.
- **New recording**: Starting a new recording begins a fresh reading view; text from earlier recordings or from dictation does not appear.
- **Mid-text dash markers**: A dash counts as a turn marker only at the start of a passage or after sentence-ending punctuation (`.`, `?`, `!`) followed by optional whitespace, as in `"Sure. - What about Friday?"`. A dash inside a sentence ("well - you know") does not start a turn.
- **Turns across chunk boundaries**: A new passage continues the previous block unless it carries a turn marker or the silence before it exceeds the pause-break threshold. This rule applies in both modes, because the turn-aware model only sees one passage at a time.
- **Turn-aware transcription fails for a passage**: The passage is transcribed with the configured model instead, an error is shown in the footer, and the passage is treated as having no turn markers. The recording continues.
- **Turn-aware transcription tool missing**: Toggling to `tdrz` shows an error naming the missing tool and how to install it; no model download is offered and the mode stays `basic`.
- **Download in progress**: Dictation, recording, and all keys keep working while the model downloads. Quitting the relay cancels the download and removes the partial file.
- **Turn-aware transcription hangs**: A passage that takes longer than 30 seconds counts as failed and falls back to the configured model.
- **Turn-aware model removed after toggling**: If the model is missing when the recording starts, the recording starts in `basic` mode, the header shows `basic`, and an error explains why.
- **Non-English speech in `tdrz` mode**: The turn-aware model only supports English. Other languages produce degraded text. This limitation is documented, not detected.
- **Keys inside the reading view**: `r` (pause/resume), `R` (stop), and `q` (quit) keep working. `+`/`-`, `d`, `m`, and `g` do nothing in the reading view. `↑`/`↓` scroll instead of adjusting the threshold.
- **Threshold adjustment during recording**: `+`/`-` in the normal view during a recording adjust the recording sensitivity for the rest of that recording; the dictation sensitivity is restored when the recording stops.

## Requirements *(mandatory)*

### Functional Requirements

**Reading view**

- **FR-001**: The TUI MUST open a reading view when the user presses `v` while a recording is active or paused, and MUST ignore `v` otherwise.
- **FR-002**: The footer MUST show the `v` hint only while a recording is active or paused.
- **FR-003**: The reading view MUST show all text that was written to the transcript file for the current recording, formatted according to FR-004, and MUST retain the full recording without the history pane's entry limit.
- **FR-004**: The reading view MUST group text into turn blocks. Each block MUST show the start time of the turn (`HH:MM:SS`) on its own line, followed by the turn's text merged into one paragraph and wrapped to the pane width. Every text line MUST be prefixed by a gutter bar; consecutive blocks alternate between two visually distinct bars (`▌` and `┃`, in two distinct colors) so adjacent turns are easy to tell apart. Blocks MUST be separated by one blank line.
- **FR-005**: The reading view MUST replace the multi-line header with a single line showing the reading mode, the recording state (recording or paused), the transcript file name, and the turn mode.
- **FR-006**: The reading view MUST support scrolling by line (`↑`/`↓`, `j`/`k`), by page (`PgUp`/`PgDn`), and jumping to the last line (`G`, `End`).
- **FR-007**: The reading view MUST follow new text while scrolled to the bottom, MUST keep the visible position when the user has scrolled up, and MUST show a count of new blocks that arrived below the visible area until the user returns to the bottom.
- **FR-008**: The reading view MUST return to the normal view on `esc`, and automatically when the recording stops.
- **FR-009**: Inside the reading view, `r`, `R`, and `q` MUST behave as in the normal view; `+`, `-`, `d`, `m`, and `g` MUST have no effect.
- **FR-010**: The reading view MUST show a placeholder when the recording has no text yet.
- **FR-011**: The reading view footer MUST list its key bindings and the follow state.

**Turn detection**

- **FR-012**: The TUI MUST offer two turn modes, `basic` and `tdrz`, and MUST show the current mode permanently in the header of the normal view and in the reading view header.
- **FR-013**: The `g` key in the normal view MUST toggle the turn mode when no recording is active or paused, and MUST be ignored otherwise. The new mode applies to the next recording.
- **FR-014**: The initial turn mode MUST come from configuration (default `basic`). Toggling MUST only affect the current relay session.
- **FR-015**: In `basic` mode, a new turn MUST start at a Whisper dash marker (a dash at the start of a passage, or a dash that follows `.`, `?`, or `!` with only whitespace in between) and when the silence before a passage exceeds the pause-break threshold.
- **FR-016**: In `tdrz` mode, recording audio MUST be transcribed with the turn-aware model, and a new turn MUST start at every speaker change the model reports and when the silence before a passage exceeds the pause-break threshold.
- **FR-017**: The configured transcription model MUST remain in use for dictation at all times; selecting `tdrz` MUST NOT restart or reconfigure the dictation transcription service.
- **FR-018**: When the user toggles to `tdrz`, the turn-aware transcription tool is installed, and the turn-aware model is not installed, the TUI MUST show an error and offer a y/n download prompt. The model MUST NOT download without explicit confirmation. FR-033 covers the case where the tool itself is missing.
- **FR-019**: A confirmed download MUST run in the background without blocking dictation, recording, or other keys, MUST show progress in the footer, MUST verify integrity before the model is used, MUST NOT leave a partial file that later passes as installed (quitting the relay cancels the download and removes the partial file), and MUST switch the mode to `tdrz` only on success. The new mode applies to the next recording.
- **FR-033**: When the user toggles to `tdrz` and the turn-aware transcription tool is not installed, the TUI MUST show an error naming the missing tool and how to install it, MUST NOT offer a model download, and the mode MUST stay `basic`.
- **FR-020**: The existing setup flow (`cc-deck ws voice --setup`) MUST be able to install the turn-aware model on request.
- **FR-021**: If turn-aware transcription fails for a passage, or does not finish within 30 seconds, the system MUST transcribe that passage with the configured model, MUST report the error in the footer, and MUST continue the recording.
- **FR-034**: When verbose logging is enabled, the relay MUST log the turn mode at recording start, each turn-aware invocation with its duration and detected turn count, every fallback with its reason, and the start, end, and failure of model downloads.
- **FR-022**: If the turn-aware model is missing when a `tdrz` recording starts, the recording MUST start in `basic` mode with an error explaining the fallback.
- **FR-023**: Each transcription event MUST carry structured segments with the segment text, timing relative to the recording, a turn-start flag, and an empty speaker field reserved for future speaker labels.
- **FR-024**: Text delivered to agent panes (dictation) MUST remain sanitized exactly as today; turn markers MUST NOT leak into delivered text.

**Recording chunking**

- **FR-025**: While recording, the relay MUST keep silence detection active, using recording-specific settings for sensitivity, silence duration, and maximum chunk length instead of forcing the minimum sensitivity (0%), which sits below typical room noise and prevents silence from ever ending a chunk.
- **FR-026**: The recording settings MUST default to sensitivity 20% (on the existing 0 to 100 logarithmic scale), 1.0 second of silence to end a chunk, a 12 second maximum chunk, and a 3 second pause-break threshold. These defaults are intended to capture quiet remote voices played through laptop speakers.
- **FR-027**: The recording settings and the pause-break threshold MUST be configurable in the voice section of the configuration file.
- **FR-028**: `+`/`-` during a recording MUST adjust the recording sensitivity for that recording; stopping the recording MUST restore the dictation sensitivity, silence duration, maximum chunk length, and mute state that were active before.

**Transcript file**

- **FR-029**: The transcript file MUST separate turns with exactly one blank line.
- **FR-030**: When a passage contains a turn change, the transcript file MUST split it at the turn boundary into separate lines with a blank line between them.
- **FR-031**: The per-line timestamp option MUST keep its current format.

**Documentation and tests (constitution)**

- **FR-032**: README, CLI reference, configuration reference, and the voice guide MUST document the reading view keys, the turn modes, the turn-aware model installation, the English-only limitation, and the recording settings.
- **FR-035**: Unit tests MUST cover basic turn detection (dash markers and pause breaks), turn-aware output parsing and fallback, reading view block grouping, wrapping and follow behavior, key gating (`v` and `g`), recording settings save and restore, and transcript turn separation.

### Key Entities

- **Passage**: One piece of audio that silence detection (or the maximum chunk length) cuts off and sends to transcription as a single unit, together with its transcribed text. A passage yields one or more segments; it yields more than one when it contains a turn change.
- **Segment**: A piece of transcribed text with its start and end time relative to the recording, a flag marking whether it starts a new turn, and an empty speaker field reserved for later labeling.
- **Turn**: A sequence of consecutive segments attributed to one speaker change interval, rendered as one block. It has a start time and merged text.
- **Turn mode**: The selected detection strategy for the next recording, `basic` or `tdrz`. Session-scoped, initialized from configuration.
- **Recording settings**: Sensitivity, silence duration, maximum chunk length, and pause-break threshold used while recording, separate from the dictation settings.
- **Turn-aware model**: An English-only speech model that reports speaker changes. Installed on request into the existing model location.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: During a recording, a sentence followed by a one second pause appears in the TUI within 5 seconds of the speaker finishing, in at least 9 out of 10 attempts on the reference machine.
- **SC-002**: After a distraction, a user can find and read the last two minutes of conversation in under 30 seconds using only the reading view.
- **SC-003**: In a scripted two-voice English exchange without pauses between speakers, `tdrz` mode starts a new block at no fewer than 7 of 10 speaker changes, while `basic` mode is expected to miss most of them.
- **SC-004**: Toggling turn mode, opening and closing the reading view, and finishing a `tdrz` recording never interrupt dictation for more than one second.
- **SC-005**: A one-hour recording remains fully readable in the reading view, with scrolling staying responsive (no visible lag on key press).
- **SC-006**: Transcript files of recordings with two or more turns contain blank-line turn separators that the `/meeting-notes` workflow can use without changes.

## Assumptions

- Conversations recorded with turn detection are mostly English; the turn-aware model's English-only limitation is acceptable and documented.
- A single microphone captures all participants; remote voices come through the laptop speakers.
- The turn-aware model is the community tinydiarize model (`ggml-small.en-tdrz.bin`, about 488 MB) from the Hugging Face repository `akashmjn/tinydiarize-whisper.cpp`, transcribed through `whisper-cli` with its turn detection option, because `whisper-server` does not expose speaker changes in its responses.
- `whisper-cli` is installed alongside `whisper-server` (both ship in the same whisper.cpp package).
- The recording defaults (sensitivity 20%, 1.0 second silence, 12 second maximum chunk, 3 second pause-break threshold) are fixed in FR-026.
- The turn mode toggle is session-scoped; the configuration provides the initial value.
- Speaker labels, non-English turn detection, reopening older transcript files, and search inside the reading view are out of scope.
- The recording defaults are starting points that need tuning on a real call; the configuration makes tuning possible without a new release.
