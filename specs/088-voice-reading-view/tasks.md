---

description: "Task list for 088-voice-reading-view"
---

# Tasks: Voice Reading View with Turn Detection

**Input**: Design documents from `specs/088-voice-reading-view/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (tui-keys, whisper-cli-tdrz, transcript-format, config), quickstart.md

**Tests**: Required. FR-035 and constitution principle I demand unit tests for all new behavior.

**Organization**: Tasks are grouped by user story. All Go paths are relative to the repository root; Go code lives in `cc-deck/`.

**Build rule**: Never run `go build` or `cargo build`. Use `make test`, `make lint`, `make install` from the repository root.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label (US1 to US4)

---

## Phase 1: Setup

**Purpose**: Establish a clean baseline before changing behavior

- [ ] T001 Run `make test` and `make lint` on the unchanged branch and note any pre-existing failures at the bottom of specs/088-voice-reading-view/tasks.md under "Baseline notes" (no code changes)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Segment model, passage timing, recording API, and the shared TUI event path that every story builds on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 [P] Add `Start` and `End` (`time.Duration`, offsets from audio stream start) to `Utterance`, and add `RecordingConfig` (`Threshold` RMS, `SilenceDuration`, `MaxUtteranceDuration`, `PauseBreak time.Duration`) with `DefaultRecordingConfig()` returning `PercentToThreshold(20)`, `1.0`, `12`, `3*time.Second` in cc-deck/internal/voice/audio.go
- [ ] T003 Track the total sample count in `VAD.Process` and stamp each emitted `Utterance` with `Start` (onset sample index minus the pre-roll samples included) and `End` (`Start` plus the emitted length after hangover trim), including the flush at input close, in cc-deck/internal/voice/vad.go
- [ ] T004 [P] Add VAD timing tests: two utterances separated by silence get increasing, non-overlapping `Start`/`End`; pre-roll is included in `Start`; `End-Start` equals `len(Audio)/SampleRate` in cc-deck/internal/voice/vad_test.go
- [ ] T005 [P] Create cc-deck/internal/voice/turns.go with `Segment` (`Text`, `Start`, `End`, `At time.Time`, `TurnStart bool`, `Speaker string`), `TurnMode` (`TurnModeBasic = "basic"`, `TurnModeTdrz = "tdrz"`), `ParseTurnMode` (case-insensitive, error on other values), `SplitDashTurns(raw string) []Segment` splitting at `(^|[.?!])\s*-\s+` and sanitizing each piece with `sanitizeTerminalText`, `stripBracketedAnnotations`, `stripLeadingDash`, and whitespace normalization, dropping empty pieces while carrying their `TurnStart` to the next kept piece, and `ApplyPauseBreak(segs []Segment, gap, threshold time.Duration, first bool)` that sets `segs[0].TurnStart` when `first` or `gap > threshold` (see data-model.md Segment rules)
- [ ] T006 [P] Add table-driven tests in cc-deck/internal/voice/turns_test.go: leading `- ` marker, `- Name:` speaker label, mid-text marker after `.`, `?`, `!` with and without extra spaces, parenthetical `well - you know` NOT split, bracket annotations removed, empty pieces dropped with TurnStart carry-over, `ParseTurnMode` valid and invalid inputs, `ApplyPauseBreak` for first passage, gap above and below threshold, empty slice
- [ ] T007 In cc-deck/internal/voice/relay.go add `Segments []Segment` to `RelayEvent`, add `RelayConfig.Recording RecordingConfig` (default `DefaultRecordingConfig()` in `DefaultRelayConfig`), and replace `SetRecording(bool)` with `StartRecording(mode TurnMode) (TurnMode, error)` and `StopRecording()` that keep today's threshold and mute save/restore and mute events, record `recStartWall`, `recStartOffset` (set from the first recording passage `Start`), and reset `lastRecEnd`; a `TurnModeTdrz` request with no turn transcriber configured returns `TurnModeBasic` and an error explaining the fallback (FR-022)
- [ ] T008 In `handleUtterance` in cc-deck/internal/voice/relay.go add the recording branch: transcribe with the configured transcriber, run `SplitDashTurns` on the raw text BEFORE the existing stripping, apply `ApplyPauseBreak` with `gap = u.Start - lastRecEnd` (first passage forced), set segment `Start`/`End` relative to recording start and `At = recStartWall + Start`, join segment texts with spaces for the existing artifact, repeat, and latency filters, update `lastRecEnd = u.End`, and emit a `transcription` event with `Segments`; the dictation (not recording) path stays unchanged and emits `Segments == nil` (FR-024)
- [ ] T009 Update cc-deck/internal/voice/relay_test.go: replace `SetRecording` usages; add tests that a recording event carries segments with `TurnStart` on the first passage, on a dash marker, and after a gap above `PauseBreak` (use mock utterances with explicit `Start`/`End`), that dictation events have nil `Segments` and the same `Text` as before, that `StopRecording` restores threshold and mute, and that `StartRecording(TurnModeTdrz)` without a turn transcriber returns basic plus an error
- [ ] T010 Refactor cc-deck/internal/tui/voice/update.go: extract one `handleRelayEvent(msg relayEventMsg) (Model, tea.Cmd)` used by `Update`, `updateFilenamePrompt`, and `updateDevicePicker` (replacing the duplicated switch blocks), and switch recording start and stop to `StartRecording(voice.TurnModeBasic)` / `StopRecording()` in cc-deck/internal/tui/voice/update.go and `closeTranscript` in cc-deck/internal/tui/voice/transcript.go; existing tests in cc-deck/internal/tui/voice/transcript_test.go must keep passing

**Checkpoint**: `make test` passes; recording events carry segments; TUI behavior unchanged.

---

## Phase 3: User Story 1 - Read back the ongoing conversation (Priority: P1) 🎯 MVP

**Goal**: `v` opens a live, scrollable chat-block view of the current recording.

**Independent Test**: Start a recording, speak several sentences with pauses, press `v`, scroll, press `G`, press `esc` (quickstart.md Scenario 1).

- [ ] T011 [US1] Add reading state to `Model` in cc-deck/internal/tui/voice/model.go: `turnBlock` (`at time.Time`, `parts []string`, `speaker string`), `recBuffer []turnBlock`, `reading bool`, `readView viewport.Model`, `readReady bool`, `follow bool` (default true), `newBlocks int`; reset `recBuffer`, `newBlocks`, and `follow` when a recording starts
- [ ] T012 [US1] Create cc-deck/internal/tui/voice/reading.go with `appendSegments(segs []voice.Segment) (added int)` (new block when `TurnStart` or buffer empty, else append text to the last block) and `renderBlocks(width int) string` producing per contracts/tui-keys.md: `HH:MM:SS` line, text joined and wrapped with `lipgloss.NewStyle().Width(width-3)`, every wrapped line prefixed `▌ ` (color 39) for even block index and `┃ ` (color 105) for odd, one blank line between blocks, and `Waiting for speech...` when empty
- [ ] T013 [US1] Add `syncReading(added int)` in cc-deck/internal/tui/voice/reading.go: check `readView.AtBottom()` before `SetContent`; when following call `GotoBottom()`, otherwise keep `YOffset` and add `added` to `newBlocks`; on `tea.WindowSizeMsg` resize the reading viewport (height minus 1 header line, 2 footer lines, 2 separators) and re-render
- [ ] T014 [US1] In `handleRelayEvent` in cc-deck/internal/tui/voice/update.go append `msg.Segments` to `recBuffer` only while `recState == recRecording` (paused text excluded, FR-003) and call `syncReading` when the reading view is open
- [ ] T015 [US1] Add key handling in cc-deck/internal/tui/voice/update.go: `v` opens the reading view only when `recState` is `recRecording` or `recPaused` (initialize `readView` on first open, `GotoBottom`, `follow = true`); new `updateReading` routes `↑`/`k`, `↓`/`j`, `pgup`/`pgdown` to the viewport (following turns off when not at bottom and back on at bottom), `G`/`end` jump to bottom and reset `newBlocks`, `esc` closes the view, `r`/`R`/`q`/`ctrl+c` reuse the normal handlers, all other keys ignored; relay events keep flowing through `handleRelayEvent`; `closeTranscript` in cc-deck/internal/tui/voice/transcript.go sets `reading = false`
- [ ] T016 [US1] Render the reading view in cc-deck/internal/tui/voice/view.go: `View()` dispatches to `viewReading()` when `reading`; single header line (`Reading`, `● REC` or `⏸ PAUSED`, transcript file base name, `Turns: <mode>`), separator, viewport with the existing scrollbar style, separator, footer `↑↓/jk scroll  PgUp/PgDn page  G end  esc back` plus `● following` or `N new ↓`; in the normal footer append `  v: read` only while recording or paused
- [ ] T017 [P] [US1] Create cc-deck/internal/tui/voice/reading_test.go: block grouping by `TurnStart`, wrapping respects width, gutter alternation, placeholder text, follow keeps bottom, scroll-up freezes offset and counts new blocks, `G` resumes and resets the counter, `v` ignored when idle and typed into the filename prompt, `esc` returns to the normal view, `R` closes the view, paused segments are not buffered, 500 passages all retained (no 200 cap), `+`/`-`/`d`/`m`/`g` ignored in the reading view

**Checkpoint**: US1 works with basic turn breaks (dash markers and pause gaps).

---

## Phase 4: User Story 2 - See text within seconds while recording (Priority: P2)

**Goal**: Recording keeps silence detection with recording-specific settings, so passages end after about one second of silence.

**Independent Test**: quickstart.md Scenario 2 (text within 5 seconds; dictation settings restored after stop).

- [ ] T018 [US2] Add `NewVADFunc(params func() VADConfig, sampleRate int) *VAD` in cc-deck/internal/voice/vad.go that takes a config snapshot per frame and recomputes threshold, silence, maximum, hangover, and minimum speech sample counts from it; keep `NewVAD(*VADConfig, int)` as a wrapper returning a snapshot of the pointed-to config so existing tests pass
- [ ] T019 [US2] In cc-deck/internal/voice/relay.go add `vadSnapshot()` returning `r.config.VADConfig` under `r.mu`, use `NewVADFunc(r.vadSnapshot, ...)` in `startVAD`, make `StartRecording` save dictation `Threshold`, `SilenceDuration`, `MaxUtteranceDuration` and apply `r.config.Recording` values, make `StopRecording` restore all three plus mute, and keep `SetVADThreshold` operating on the live config so `+`/`-` during a recording only change the recording threshold (FR-028)
- [ ] T020 [P] [US2] Tests: in cc-deck/internal/voice/vad_test.go a silence-duration change through the snapshot function takes effect mid-stream and a 12 s maximum splits continuous speech; in cc-deck/internal/voice/relay_test.go `StartRecording` applies recording threshold, silence, and maximum, `StopRecording` restores all dictation values, and a threshold change during recording does not survive `StopRecording`
- [ ] T021 [P] [US2] Add `VoiceRecordingDefaults` (`Threshold *int`, `Silence *float64`, `MaxChunk *float64` yaml `max_chunk`, `PauseBreak *float64` yaml `pause_break`) as `VoiceDefaults.Recording *VoiceRecordingDefaults` yaml `recording` in cc-deck/internal/config/config.go, extend `validateVoice` with the ranges from contracts/config.md in cc-deck/internal/config/validate.go, and add cases to cc-deck/internal/config/validate_test.go
- [ ] T022 [US2] In cc-deck/internal/cmd/ws_voice.go map `defaults.voice.recording` onto `config.Recording` (percent via `voice.PercentToThreshold`, seconds to durations; out-of-range values keep defaults) and include the recording settings in the existing verbose VAD config log line

**Checkpoint**: US1 view now updates within seconds; dictation unchanged after stop.

---

## Phase 5: User Story 3 - Opt into model-based turn detection (Priority: P2)

**Goal**: `g` selects `tdrz`; recordings in `tdrz` mode use `whisper-cli -tdrz` with the tinydiarize model; dictation keeps the configured model.

**Independent Test**: quickstart.md Scenario 3.

- [ ] T023 [P] [US3] In cc-deck/internal/voice/setup.go add `TreeAPI` to `ModelInfo` (default `hfTreeAPI`), register `small.en-tdrz` (`ggml-small.en-tdrz.bin`, URL `https://huggingface.co/akashmjn/tinydiarize-whisper.cpp/resolve/main/ggml-small.en-tdrz.bin`, tree API `https://huggingface.co/api/models/akashmjn/tinydiarize-whisper.cpp/tree/main`), add `const TdrzModelName = "small.en-tdrz"`, make `fetchRemoteSHA` take the `ModelInfo`, export `DownloadModel(ctx, name string, progress func(done, total int64)) error` reusing the temp-file, SHA-256, and rename flow (progress through a counting writer), route `RunSetupWithContext` through it, and add `TdrzStatus() (toolErr, modelErr error)` checking `exec.LookPath("whisper-cli")` and the model file
- [ ] T024 [P] [US3] Create cc-deck/internal/voice/setup_test.go (or extend the existing setup tests) using `httptest`: SHA lookup uses the model's own tree API, checksum mismatch leaves no final model file, context cancellation removes the temp file, progress callback reaches `total`, `TdrzStatus` reports a missing tool (empty `PATH` via `t.Setenv`) and a missing model (temp model dir)
- [ ] T025 [P] [US3] Add `ParseTdrzJSON(data []byte) ([]Segment, error)` to cc-deck/internal/voice/turns.go (offsets in ms to `Start`/`End`, `speaker_turn_next` of segment i sets `TurnStart` on i+1, missing field is false, text sanitized, empty texts dropped with TurnStart carry-over) and create cc-deck/internal/voice/transcriber_tdrz.go with the `TurnTranscriber` interface and `tdrzTranscriber` per contracts/whisper-cli-tdrz.md (private temp dir with WAV via `writeWAVFile`, args `-m <model> -f <wav> -tdrz -oj -of <tmp>/out -np` plus `--prompt` when set, injectable `runCmd`, errors for non-zero exit, missing or malformed JSON, temp dir removed)
- [ ] T026 [P] [US3] Create cc-deck/internal/voice/transcriber_tdrz_test.go: `ParseTdrzJSON` turn flags, missing field, empty transcription, malformed JSON; fake `runCmd` asserting arguments (including `--prompt`), writing canned JSON, returning errors, honoring context deadline; temp dir removed after success and failure
- [ ] T027 [US3] In cc-deck/internal/voice/relay.go add `SetTurnTranscriber(TurnTranscriber, status func() error)` and a `TurnTimeout` field (default 30 s); `StartRecording(TurnModeTdrz)` returns `TurnModeTdrz` only when a turn transcriber is set and `status()` is nil, else basic plus the reason; in the recording branch for `tdrz` call `TranscribeTurns` with `context.WithTimeout(ctx, TurnTimeout)`, shift segment offsets by the passage start, apply `ApplyPauseBreak`, and on error or timeout transcribe the same audio with the configured transcriber, use `SplitDashTurns`, and emit an `error` event (FR-021); with `Verbose` log the effective mode at recording start, each tdrz call with duration and turn count, and every fallback with its reason (FR-034); close the turn transcriber in `Stop`
- [ ] T028 [US3] Add tdrz relay tests to cc-deck/internal/voice/relay_test.go with a stub `TurnTranscriber`: turn flags propagate, pause-break still applies, error falls back with an error event, a stub that blocks until its context ends falls back after a short `TurnTimeout`, `status()` error at start yields basic, and dictation while tdrz is selected (not recording) uses the configured transcriber only
- [ ] T029 [US3] Create cc-deck/internal/tui/voice/turnmode.go: `Model` gets `turnMode`, injectable `tdrzStatus func() (error, error)` and `download func(ctx, progress) error` (defaults `voice.TdrzStatus` and `voice.DownloadModel(ctx, voice.TdrzModelName, ...)`), download state (`dlPrompt`, `dlRunning`, `dlDone`, `dlTotal`, `dlCancel`, `dlCh`); `toggleTurnMode()` per contracts/tui-keys.md (tool missing error, model missing error plus prompt, otherwise switch; back to basic always allowed); `startDownload()` runs in a goroutine sending progress and completion on a channel consumed by a `waitForDownload` command; completion sets `tdrz`, failure sets the error and keeps `basic`; `q` cancels a running download
- [ ] T030 [US3] Wire turn mode into the TUI: `voicetui.New` takes the initial `voice.TurnMode`; in cc-deck/internal/tui/voice/update.go route `g` (only when `recState == recIdle`, no prompt, no running download) to `toggleTurnMode`, route `y`/`n`/`esc` while `dlPrompt`, handle download messages in `Update`, pass `m.turnMode` to `StartRecording` and on fallback set `m.turnMode = basic` and show the returned error; in cc-deck/internal/tui/voice/view.go show `Turns: <mode>` on the device line, `g: turns` in the idle footer, the download prompt, and `Downloading tdrz model: NN% (X/Y MB)` while running
- [ ] T031 [P] [US3] Create cc-deck/internal/tui/voice/turnmode_test.go: `g` toggles when idle; ignored while recording, paused, prompting, or downloading; tool missing shows the error without prompt; model missing shows the prompt, `n` keeps basic, `y` starts the stub download, success switches to tdrz, failure keeps basic with error; `q` during download calls cancel; header contains `Turns: tdrz`; `StartRecording` fallback resets the mode to basic
- [ ] T032 [US3] Add `VoiceDefaults.TurnMode *string` yaml `turn_mode` in cc-deck/internal/config/config.go with validation in cc-deck/internal/config/validate.go (basic or tdrz, case-insensitive) and a test in cc-deck/internal/config/validate_test.go; in cc-deck/internal/cmd/ws_voice.go build the tdrz transcriber with `voice.ModelPath(voice.TdrzModelName)` and the glossary prompt, call `relay.SetTurnTranscriber` with a status func wrapping `voice.TdrzStatus`, and pass the parsed initial turn mode (default basic) to `voicetui.New`

**Checkpoint**: US1 with tdrz turn breaks; missing tool and model paths handled; dictation unaffected.

---

## Phase 6: User Story 4 - Turn breaks in the transcript file (Priority: P3)

**Goal**: The transcript file separates turns with one blank line.

**Independent Test**: quickstart.md Scenario 4.

- [ ] T033 [US4] In cc-deck/internal/tui/voice/transcript.go add `writeSegments(f *os.File, segs []voice.Segment, timestamps bool, hasText *bool) (lines int, err error)` per contracts/transcript-format.md (blank line before a `TurnStart` segment when `*hasText`, one line per segment, existing `[HH:MM:SS] ` prefix when timestamps are on), reset `hasText` when a recording starts, and use it from `handleRelayEvent` in cc-deck/internal/tui/voice/update.go instead of `writeTranscriptLine` for recording events (keep `recCount` counting written segments)
- [ ] T034 [P] [US4] Add tests to cc-deck/internal/tui/voice/transcript_test.go: two turns produce exactly one blank line, no leading blank line, a passage with an internal turn is split into two lines with a blank line between, consecutive non-turn segments produce no blank lines, timestamp prefix format unchanged, paused segments are not written

**Checkpoint**: All four stories complete.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation (constitution I, FR-032) and final verification. All prose uses the prose plugin with the cc-deck voice; no em or en dashes; AsciiDoc uses one sentence per line.

- [ ] T035 [P] Update the voice relay section of README.md: reading view (`v`, keys), turn modes (`g`, `basic` vs `tdrz`, English only), installing the tdrz model (`cc-deck ws voice --setup --model small.en-tdrz` or the in-TUI prompt), faster recording updates and the `defaults.voice.recording` settings
- [ ] T036 [P] Update `==== ws voice` in docs/modules/reference/pages/cli.adoc: key table for the normal view (`v`, `g`) and the reading view, the `--setup --model small.en-tdrz` example, and the `whisper-cli` requirement for `tdrz`
- [ ] T037 [P] Document `defaults.voice.turn_mode` and `defaults.voice.recording.{threshold,silence,max_chunk,pause_break}` with defaults, ranges, and validation behavior in docs/modules/reference/pages/configuration.adoc
- [ ] T038 [P] Add a "Reading view and turn detection" section to docs/modules/using/pages/voice.adoc: reading a recording live, basic vs tdrz turn detection and how each decides a new block, model download, English-only limitation, tuning recording sensitivity for quiet remote voices, transcript file turn separators, troubleshooting (missing `whisper-cli`, failed download, fallback errors)
- [ ] T039 Run `/prose:check` on README.md and the three AsciiDoc pages changed in T035 to T038 and fix all findings
- [ ] T040 Run `make lint` and `make test` from the repository root and fix every failure introduced by this branch
- [ ] T041 Run `make install` to confirm the CLI builds and installs; leave the interactive quickstart.md scenarios (audio required) for the smoke test

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none
- **Foundational (Phase 2)**: depends on Setup; blocks all stories
- **US1 (Phase 3)**: depends on Foundational
- **US2 (Phase 4)**: depends on Foundational; independent of US1 (relay, VAD, config only)
- **US3 (Phase 5)**: depends on Foundational; T030 edits update.go and view.go after US1 (T014 to T016) to avoid conflicts
- **US4 (Phase 6)**: depends on Foundational (segments, `handleRelayEvent`); T033 edits update.go after US1
- **Polish (Phase 7)**: after all stories

### Within Phases

- T003 before T004; T005 before T006; T007 before T008 before T009; T010 after T007
- T011 before T012 before T013; T014 to T016 after T013; T017 after T016
- T018 before T019 before T020; T021 before T022
- T023 before T024 and T029; T025 before T026 and T027; T027 before T028; T029 before T030 before T031; T032 after T027 and T030
- T033 before T034

### Parallel Opportunities

- Phase 2: T002, T005 (different files); then T004 and T006 alongside T007
- US2 can run in parallel with US1 (different packages) once Phase 2 is done
- US3: T023, T025 in parallel; T024, T026 in parallel after their implementations
- Polish: T035 to T038 in parallel

## Parallel Example: User Story 3

```text
Task: "T023 model registry and DownloadModel in cc-deck/internal/voice/setup.go"
Task: "T025 ParseTdrzJSON and tdrzTranscriber in cc-deck/internal/voice/turns.go and transcriber_tdrz.go"
then
Task: "T024 setup tests in cc-deck/internal/voice/setup_test.go"
Task: "T026 tdrz transcriber tests in cc-deck/internal/voice/transcriber_tdrz_test.go"
```

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1 and Phase 2
2. Phase 3 (US1): reading view with basic turn breaks
3. Validate with quickstart.md Scenario 1 (30 second updates are acceptable for the MVP)

### Incremental Delivery

1. US2: fast recording updates
2. US3: opt-in tdrz turn detection
3. US4: transcript file turn breaks
4. Polish: documentation and verification

## Baseline notes

(filled by T001)
