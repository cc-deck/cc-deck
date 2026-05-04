# Tasks: Voice Transcript Recording

**Input**: Design documents from `specs/048-voice-transcript-recording/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Included per spec SC-007 requirement for unit test coverage.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No project initialization needed. All changes extend existing files or create new files in existing packages.

(No setup tasks required.)

---

## Phase 2: Foundational (Relay Recording Support + Transcript Helpers)

**Purpose**: Add the recording flag to the relay and create the transcript helper file. These are prerequisites for all user stories.

- [ ] T001 [P] Add `recording` bool field, `SetRecording(bool)` and `IsRecording() bool` methods to `VoiceRelay` in `cc-deck/internal/voice/relay.go` following the existing `muted`/`IsMuted()` pattern (lines 59-84)
- [ ] T002 [P] Create `cc-deck/internal/tui/voice/transcript.go` with `recStatus` type (idle/prompting/recording/paused), `defaultTranscriptDir()` returning `xdg.DataHome + "/cc-deck/transcripts/"`, `resolveTranscriptPath(name string) string` that prepends default dir for relative names and creates dir with `os.MkdirAll`, `defaultTranscriptName()` returning `transcript-YYYY-MM-DDTHH-MM-SS.txt`, `writeTranscriptLine(f *os.File, text string) error` writing text + newline, and `(m *Model) closeTranscript()` that closes file and resets state
- [ ] T003 [P] Add unit tests for transcript helpers in `cc-deck/internal/tui/voice/transcript_test.go`: `TestResolveTranscriptPath` (relative vs absolute), `TestDefaultTranscriptName` (format validation), `TestWriteTranscriptLine` (writes text + newline only)

**Checkpoint**: Relay has recording flag. Transcript helpers are tested and ready.

---

## Phase 3: User Story 1 - Start and Stop Recording (Priority: P1)

**Goal**: Users can press `r` to open a filename prompt, enter a name, start recording, and press `R` to stop.

**Independent Test**: Start voice relay, press `r`, enter filename, speak, press `R`, verify file contains transcribed utterances one per line.

### Implementation for User Story 1

- [ ] T004 [US1] Add recording state fields to `Model` struct in `cc-deck/internal/tui/voice/model.go`: `recState recStatus`, `recFile *os.File`, `recPath string`, `recCount int`, `recInput textinput.Model`. Initialize textinput in `New()` with placeholder "transcript.txt" and CharLimit 256. Import `"github.com/charmbracelet/bubbles/textinput"`.
- [ ] T005 [US1] Add prompt sub-mode routing in `Update()` in `cc-deck/internal/tui/voice/update.go`: after the `devicePick` check, add `if m.recState == recPrompting { return m.updateFilenamePrompt(msg) }`. Add `"r"` key handler that initializes textinput with default name, focuses it, and sets `recState = recPrompting` when idle. Add `"R"` key handler that calls `m.closeTranscript()` when recording or paused. In quit handler (`"q"`, `"ctrl+c"`), call `m.closeTranscript()` before `tea.Quit`.
- [ ] T006 [US1] Implement `updateFilenamePrompt()` method in `cc-deck/internal/tui/voice/update.go`: handle `"enter"` (resolve path, open file with `os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644`, set `recRecording`, call `m.relay.SetRecording(true)`, on error set `m.err` and return to idle), `"esc"` (blur textinput, set `recIdle`), other keys delegate to `m.recInput.Update(msg)`. Also handle `tea.WindowSizeMsg` and `relayEventMsg` to keep consuming events.
- [ ] T007 [US1] Add transcript capture in `relayEventMsg` / `"transcription"` case in `cc-deck/internal/tui/voice/update.go`: after appending to history, if `recState == recRecording && recFile != nil`, call `writeTranscriptLine(m.recFile, msg.Text)` and increment `m.recCount`.
- [ ] T008 [US1] Add recording indicator in `renderHeader()` in `cc-deck/internal/tui/voice/view.go`: after the mode display on the device line, add red `● REC` when recording and yellow `⏸ REC` when paused. Add `recStyle` and `pauseStyle` lipgloss styles.
- [ ] T009 [US1] Add filename prompt rendering in `renderFooter()` in `cc-deck/internal/tui/voice/view.go`: when `recState == recPrompting`, render `labelStyle.Render("  Transcript: ") + m.recInput.View()` with hint line `"enter: start  esc: cancel"`. Update `footerHeight()` to add 1 line when prompting.
- [ ] T010 [US1] Update footer hints in `renderFooter()` in `cc-deck/internal/tui/voice/view.go`: when idle append `r: record`, when recording show `r: pause  R: stop`, when paused show `r: resume  R: stop`.
- [ ] T011 [US1] Add state machine test in `cc-deck/internal/tui/voice/transcript_test.go`: `TestRecordingStateMachine` simulating key sequences (r -> enter filename -> R to stop), verify state transitions. `TestTranscriptionCapturedDuringRecording` sending relayEventMsg and verifying file content. `TestQuitClosesTranscript` verifying file is closed on quit.

**Checkpoint**: Core recording works. Users can start, record transcriptions to file, and stop.

---

## Phase 4: User Story 2 - Pause and Resume (Priority: P2)

**Goal**: Users can press `r` while recording to pause, press `r` again to resume. Paused transcriptions appear in TUI but are not written to file.

**Independent Test**: Start recording, speak, pause, speak more, resume, speak, stop. File should only contain text from recording periods.

### Implementation for User Story 2

- [ ] T012 [US2] Extend `"r"` key handler in `cc-deck/internal/tui/voice/update.go`: when `recState == recRecording`, set `recPaused`. When `recState == recPaused`, set `recRecording`. (Note: relay.SetRecording stays true during pause since relay should continue transcribing.)
- [ ] T013 [US2] Add test `TestTranscriptionSkippedWhilePaused` in `cc-deck/internal/tui/voice/transcript_test.go`: verify that relayEventMsg with type "transcription" does NOT write to file when paused, but DOES appear in history.

**Checkpoint**: Pause/resume works. Paused transcriptions are not written to file.

---

## Phase 5: User Story 3 - Record While Muted (Priority: P3)

**Goal**: When muted and recording, relay still transcribes speech and writes to file, but does not send to pane.

**Independent Test**: Start recording, mute, speak, stop. File should contain muted transcriptions.

### Implementation for User Story 3

- [ ] T014 [US3] Modify `handleUtterance()` in `cc-deck/internal/voice/relay.go` (line 382-388): change the early-return mute check to: if muted AND not recording, discard as before. If muted AND recording, continue to transcribe via Whisper, emit `"transcription"` event with the text, but skip stopword processing and pipe send.
- [ ] T015 [US3] Add test `TestRelayTranscribesWhileMutedAndRecording` in `cc-deck/internal/voice/relay_test.go`: set relay to muted + recording, process an utterance, verify transcription event is emitted but no pipe send occurs. Add test `TestRelayDiscardsWhileMutedNotRecording` verifying existing discard behavior is preserved.

**Checkpoint**: Muted+recording transcribes to file. Muted without recording discards as before.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation.

- [ ] T016 [P] Update keyboard controls table in `docs/modules/using/pages/voice.adoc` with `r` (record/pause/resume) and `R` (stop recording) keys
- [ ] T017 [P] Add "Transcript Recording" subsection in `docs/modules/using/pages/voice.adoc` explaining usage, file format, default directory, and mute+recording behavior
- [ ] T018 Run `make test` and `make lint` to verify all changes pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies, can start immediately. T001-T003 are all parallelizable (different files).
- **User Story 1 (Phase 3)**: Depends on T001 (relay recording flag) and T002 (transcript helpers)
- **User Story 2 (Phase 4)**: Depends on User Story 1 completion (pause extends the recording key handler)
- **User Story 3 (Phase 5)**: Depends on T001 (relay recording flag). Can run in parallel with US1/US2 since it modifies relay.go only.
- **Polish (Phase 6)**: Depends on all user stories being complete

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (different files)
- T016, T017 can run in parallel (same file but different sections)
- US3 (T014, T015) can run in parallel with US1/US2 since it only touches relay.go

---

## Parallel Example: Foundational Phase

```bash
# All three foundational tasks touch different files:
Task: "Add recording field to relay in relay.go"
Task: "Create transcript.go with types and helpers"
Task: "Create transcript_test.go with helper tests"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (relay + transcript helpers)
2. Complete Phase 3: User Story 1 (TUI integration)
3. **STOP and VALIDATE**: Press `r`, enter filename, speak, press `R`, check file
4. Feature is usable with start/stop only

### Incremental Delivery

1. Foundational -> Relay recording flag + transcript helpers ready
2. User Story 1 -> Start/stop recording works (MVP)
3. User Story 2 -> Pause/resume adds control
4. User Story 3 -> Muted recording enables notes-to-self
5. Polish -> Documentation updated

---

## Notes

- Total tasks: 18
- Tasks per user story: US1=8, US2=2, US3=2, Foundational=3, Polish=3
- Parallel opportunities: T001-T003 (all parallel), T016-T017 (parallel)
- MVP scope: Phases 2+3 (11 tasks: T001-T011)
- US2 adds 2 tasks on top of US1 (extends existing key handler)
- US3 is independent of US1/US2 (only touches relay.go)
