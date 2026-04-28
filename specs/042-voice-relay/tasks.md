# Tasks: Voice Relay

**Input**: Design documents from `/specs/042-voice-relay/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Add dependencies and create package structure for voice relay

- [X] T001 Add charmbracelet/bubbletea, lipgloss, and bubbles dependencies to cc-deck/go.mod
- [X] T002 Add gen2brain/malgo dependency to cc-deck/go.mod (CGo audio capture)
- [X] T003 [P] Create cc-deck/internal/voice/ package directory with audio.go defining AudioSource interface, DeviceInfo struct, Utterance struct, and VADConfig struct per contracts/voice-pipeline.md
- [X] T004 [P] Create cc-deck/internal/voice/transcriber.go defining Transcriber interface and TranscriptionResult struct per contracts/voice-pipeline.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: PipeChannel.SendReceive and plugin voice handlers. MUST complete before any user story.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Implement localPipeChannel.SendReceive in cc-deck/internal/ws/channel_pipe.go using exec.CommandContext with cmd.Output() per research decision 1
- [X] T006 Add execOutputFn field to execPipeChannel struct and implement execPipeChannel.SendReceive in cc-deck/internal/ws/channel_pipe.go per research decision 1
- [X] T007 Update PipeChannel factory methods in all workspace types (container.go, compose.go, ssh.go, k8s_deploy.go) to pass ExecOutput as execOutputFn when creating execPipeChannel
- [X] T008 Add unit tests for SendReceive in cc-deck/internal/ws/channel_pipe_test.go (local mock, exec mock, nil execOutputFn, empty pipe name, context cancellation)
- [X] T009 [P] Add VoiceText, VoiceControl, VoiceToggle variants to PipeAction enum in cc-zellij-plugin/src/pipe_handler.rs and extend parse_pipe_message for cc-deck:voice, cc-deck:voice-control, cc-deck:voice-toggle pipe names
- [X] T010 [P] Add voice_control_pipe (Option<String>), voice_buffer (Vec<String>), and voice_enabled (bool) fields to ControllerState in cc-zellij-plugin/src/controller/state.rs
- [X] T011 Implement VoiceText handler in cc-zellij-plugin/src/controller/mod.rs: find attended pane, check permission state, call write_chars_to_pane_id or buffer text, unblock pipe
- [X] T012 Implement VoiceControl handler in cc-zellij-plugin/src/controller/mod.rs: store pipe_id, set voice_enabled, DO NOT unblock pipe. Exclude VoiceControl from automatic unblock_cli_pipe_input
- [X] T013 Implement VoiceToggle handler in cc-zellij-plugin/src/controller/mod.rs: respond to held pipe via cli_pipe_output, unblock_cli_pipe_input, clear voice_control_pipe
- [X] T014 Add F8 keybinding registration for cc-deck:voice-toggle in cc-zellij-plugin/src/controller/events.rs register_keybindings KDL block
- [X] T015 [P] Add WASM-gated wrappers for write_chars_to_pane_id in cc-zellij-plugin/src/ (similar to existing switch_and_focus pattern in attend.rs)
- [X] T016 [P] Add unit tests for parse_pipe_message voice actions in cc-zellij-plugin/src/pipe_handler.rs tests module

**Checkpoint**: PipeChannel.SendReceive works across all workspace types; plugin handles voice pipe messages and F8 keybinding.

---

## Phase 3: User Story 1 - Voice Dictation to Active Agent Session (Priority: P1) MVP

**Goal**: Developer speaks into microphone, transcribed text appears in attended agent pane of any workspace type.

**Independent Test**: Start a workspace, run `cc-deck ws voice <workspace>`, speak into microphone, verify text appears in the attended agent pane. Say "submit" to press Enter.

### Implementation for User Story 1

- [X] T017 [P] [US1] Implement malgo AudioSource backend in cc-deck/internal/voice/audio_malgo.go (//go:build cgo): Start with OnRecvFrames callback at 16kHz mono s16le, Stop, Level via RMS, ListDevices via malgo context
- [X] T018 [P] [US1] Implement ffmpeg AudioSource backend in cc-deck/internal/voice/audio_ffmpeg.go (//go:build !cgo): Start ffmpeg subprocess piping stdout, parse s16le PCM frames, Stop kills process, ListDevices returns empty
- [X] T019 [US1] Implement energy-based VAD in cc-deck/internal/voice/vad.go: RMS threshold 0.015, pre-roll 0.3s, silence duration 1.5s, max utterance 30s, produces Utterance structs from AudioSource channel
- [X] T020 [US1] Implement stopword engine in cc-deck/internal/voice/stopword.go: strip filler words ("um", "uh", "hmm", "ah", "er"), check if remaining text equals exactly "submit" or "enter", return TranscriptionResult with IsCommand flag
- [X] T021 [P] [US1] Implement whisper-server HTTP transcriber in cc-deck/internal/voice/transcriber_http.go: POST multipart/form-data to /inference endpoint, parse text response, respect context cancellation
- [X] T022 [P] [US1] Implement whisper-cli subprocess transcriber in cc-deck/internal/voice/transcriber_cli.go: write temp WAV file from PCM, invoke whisper-cli, parse stdout, clean up temp file
- [X] T023 [US1] Implement whisper-server lifecycle manager in cc-deck/internal/voice/server.go: auto-start whisper-server with model path and port, health check, auto-stop on Close, retry on crash (up to 3 attempts with notification)
- [X] T024 [US1] Implement VoiceRelay orchestrator in cc-deck/internal/voice/relay.go: connect AudioSource -> VAD -> Transcriber -> StopwordEngine -> PipeChannel.Send pipeline, manage lifecycle, track attended session via state query
- [X] T025 [US1] Implement Bubbletea TUI model in cc-deck/internal/tui/voice/model.go: state struct with mode, audio level, transcription history, delivery status, target session info, verbose flag
- [X] T026 [US1] Implement TUI view in cc-deck/internal/tui/voice/view.go: render audio level meter, mode indicator, transcription history with delivery status, target session, keyboard shortcuts
- [X] T027 [US1] Implement TUI update handler in cc-deck/internal/tui/voice/update.go: handle audio level ticks, transcription results, delivery confirmations, quit key, mode toggle key
- [X] T028 [US1] Implement cc-deck ws voice command in cc-deck/internal/cmd/ws_voice.go: cobra command with --mode, --model, --device, --verbose flags, workspace resolution, VoiceRelay + TUI startup
- [X] T029 [US1] Implement cc-deck ws pipe command in cc-deck/internal/cmd/ws_pipe.go: cobra command with --name and --payload flags, workspace resolution, PipeChannel.Send call
- [X] T030 [US1] Register ws voice and ws pipe commands in cc-deck/internal/cmd/ws.go addToGroup calls

**Checkpoint**: Full voice dictation pipeline works end-to-end. Developer speaks, text appears in attended pane. "submit"/"enter" sends newline. Works across all workspace types.

---

## Phase 4: User Story 2 - Voice-Triggered Prompt Submission (Priority: P1)

**Goal**: "submit" and "enter" as standalone utterances send newline to agent pane.

**Independent Test**: Dictate a prompt, pause, say "submit". Verify newline is sent. Say "please submit the form" and verify full text is relayed without newline.

### Implementation for User Story 2

> **NOTE**: Stopword detection is implemented in T020 (US1). This phase adds edge case tests and refinement.

- [X] T031 [US2] Add stopword unit tests in cc-deck/internal/voice/stopword_test.go: test "submit", "enter", "please submit the form", "press enter to continue", "okay submit", "submit it", "um, submit" (filler stripping), empty string, whitespace-only
- [X] T032 [US2] Verify VoiceRelay sends "\n" for standalone command words and full text for non-commands in cc-deck/internal/voice/relay.go (adjust relay logic if needed)

**Checkpoint**: Stopword detection handles all edge cases from spec US2 scenarios 1-7.

---

## Phase 5: User Story 3 - Push-to-Talk via Zellij Keybinding (Priority: P2)

**Goal**: F8 key toggles recording from any pane without focus switching.

**Independent Test**: Start voice relay in PTT mode, press F8 from a Claude Code pane, speak, press F8 again. Verify text transcribed and delivered without focus switching.

### Implementation for User Story 3

- [X] T033 [US3] Implement PTT mode in VoiceRelay orchestrator in cc-deck/internal/voice/relay.go: SendReceive long-poll loop to cc-deck:voice-control, toggle recording on "toggle" response, re-establish long-poll
- [X] T034 [US3] Add PTT state to TUI model in cc-deck/internal/tui/voice/model.go and update view in view.go: show PTT recording state, "press F8 to toggle" hint, mode indicator
- [X] T035 [US3] Add mode switching support in TUI update handler in cc-deck/internal/tui/voice/update.go: 'm' key toggles between VAD and PTT modes, stops/starts appropriate audio gating

**Checkpoint**: PTT mode works via F8 keybinding from any pane, including remote workspaces.

---

## Phase 6: User Story 4 - Permission-Safe Voice Relay (Priority: P2)

**Goal**: Voice text is not injected during permission prompts, preventing accidental responses.

**Independent Test**: Start voice relay, trigger a permission prompt, dictate text. Verify text is buffered and discarded when permission resolves.

### Implementation for User Story 4

- [X] T036 [US4] Implement permission-state buffer discard in cc-zellij-plugin/src/controller/mod.rs: clear voice_buffer when session transitions from Waiting(Permission) to any other state
- [X] T037 [US4] Add permission pause notification to TUI: query session state periodically via PipeChannel, show "Voice paused: permission prompt active" warning in cc-deck/internal/tui/voice/view.go

**Checkpoint**: Voice relay pauses during permission prompts, buffered text is discarded.

---

## Phase 7: User Story 5 - Voice Setup and Model Management (Priority: P2)

**Goal**: Setup command checks dependencies, downloads model, verifies readiness.

**Independent Test**: Run `cc-deck ws voice --setup` on a clean machine. Verify it checks for whisper-server/whisper-cli, downloads model with progress, and reports readiness.

### Implementation for User Story 5

- [ ] T038 [P] [US5] Implement model info registry in cc-deck/internal/voice/setup.go: ModelInfo structs for tiny.en, base.en, small.en, medium with URLs, expected sizes, and file names
- [ ] T039 [P] [US5] Implement dependency checker in cc-deck/internal/voice/setup.go: check for whisper-server and whisper-cli in PATH, report availability, print platform-specific install instructions
- [ ] T040 [US5] Implement model download with progress in cc-deck/internal/voice/setup.go: HTTP GET from Hugging Face URL to ~/.cache/cc-deck/models/, progress bar output, file size validation, re-download on corruption
- [ ] T041 [US5] Add --setup flag handling to cc-deck ws voice command in cc-deck/internal/cmd/ws_voice.go: run dependency check, model download, report readiness
- [ ] T042 [US5] Add startup validation to VoiceRelay in cc-deck/internal/voice/relay.go: check model exists and is valid before starting, error with setup instructions if missing

**Checkpoint**: Setup flow works end-to-end. Model downloads with progress. Missing dependencies show clear instructions.

---

## Phase 8: User Story 6 - Session Focus Tracking (Priority: P3)

**Goal**: Voice relay automatically follows session focus changes.

**Independent Test**: Start voice relay, switch to different session via attend, dictate text. Verify text arrives in newly attended session.

### Implementation for User Story 6

- [ ] T043 [US6] Implement session focus tracking in VoiceRelay orchestrator in cc-deck/internal/voice/relay.go: query attended session state before each utterance delivery, update target pane ID on change
- [ ] T044 [US6] Update TUI to show current target session in cc-deck/internal/tui/voice/view.go: display attended session name, show warning when no session is attended

**Checkpoint**: Voice relay follows session focus changes. Text always goes to the currently attended session.

---

## Phase 9: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and quality improvements

- [ ] T045 [P] Update README.md with voice relay feature description, usage examples, and spec table entry for 042-voice-relay
- [ ] T046 [P] Update CLI reference in docs/modules/reference/pages/cli.adoc with ws voice and ws pipe command documentation (flags, usage, examples)
- [ ] T047 Add --list-devices flag implementation to ws voice command in cc-deck/internal/cmd/ws_voice.go: enumerate devices via AudioSource.ListDevices(), print list, exit
- [ ] T048 Verify make test passes for both Go and Rust test suites
- [ ] T049 Verify make lint passes for both Go and Rust linting
- [ ] T050 Run quickstart.md validation: execute the quickstart steps manually to verify end-to-end flow

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1. BLOCKS all user stories
- **Phase 3 (US1)**: Depends on Phase 2. MVP target
- **Phase 4 (US2)**: Depends on T020 (stopword engine from US1). Primarily testing/refinement
- **Phase 5 (US3)**: Depends on Phase 2 (SendReceive, voice-control handler). Independent of US1 audio pipeline details
- **Phase 6 (US4)**: Depends on T011 (VoiceText permission check in plugin). Independent of US1 audio pipeline
- **Phase 7 (US5)**: Depends on Phase 1 (dependencies). Independent of other stories
- **Phase 8 (US6)**: Depends on T024 (VoiceRelay orchestrator). Extends existing relay logic
- **Phase 9 (Polish)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational phase only. No dependencies on other stories. **MVP**
- **US2 (P1)**: Depends on US1 T020 (stopword engine). Primarily edge case refinement
- **US3 (P2)**: Depends on Foundational phase (SendReceive). Can start in parallel with US1 audio pipeline tasks
- **US4 (P2)**: Depends on Foundational phase (plugin handlers). Can start in parallel with US1
- **US5 (P2)**: Independent of other stories. Only depends on Phase 1 setup
- **US6 (P3)**: Depends on US1 T024 (VoiceRelay orchestrator). Extends it with focus tracking

### Within Each User Story

- Interfaces/contracts before implementations
- Audio pipeline before transcription
- Core implementation before TUI integration
- Commit after each task or logical group

### Parallel Opportunities

**Phase 2 parallelism** (Go + Rust in parallel):
- T005-T008 (Go PipeChannel) can run in parallel with T009-T016 (Rust plugin)

**Phase 3 (US1) parallelism**:
- T017 + T018 (audio backends, different files with build tags)
- T021 + T022 (transcriber backends, different files)
- T025 + T026 + T027 (TUI files, after T024)

**Cross-story parallelism** (after Phase 2):
- US5 (setup/model management) can run in parallel with US1 (audio pipeline)
- US3 (PTT) can start plugin-side work in parallel with US1 Go-side work

---

## Parallel Example: Phase 2 (Foundational)

```
# Go PipeChannel (agent A):
T005: localPipeChannel.SendReceive in channel_pipe.go
T006: execPipeChannel.SendReceive in channel_pipe.go
T007: Update workspace PipeChannel factories
T008: Unit tests for SendReceive

# Rust Plugin (agent B, in parallel):
T009: PipeAction enum + parse_pipe_message extensions
T010: ControllerState voice fields
T011-T013: Voice pipe handlers in controller/mod.rs
T014: F8 keybinding in events.rs
T015: WASM wrappers
T016: Unit tests
```

## Parallel Example: Phase 3 (US1 Audio Pipeline)

```
# Audio backends (parallel, different files):
T017: malgo backend in audio_malgo.go (//go:build cgo)
T018: ffmpeg backend in audio_ffmpeg.go (//go:build !cgo)

# Transcriber backends (parallel, different files):
T021: whisper-server HTTP in transcriber_http.go
T022: whisper-cli subprocess in transcriber_cli.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (add dependencies)
2. Complete Phase 2: Foundational (PipeChannel.SendReceive + plugin handlers)
3. Complete Phase 3: User Story 1 (full voice dictation pipeline)
4. **STOP and VALIDATE**: Test voice dictation end-to-end with a running workspace
5. Ship as initial voice relay capability

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 (voice dictation) -> Test independently -> MVP!
3. US2 (prompt submission edge cases) -> Refinement of stopword detection
4. US5 (setup/model management) -> Better first-time experience
5. US3 (PTT mode) -> Noisy environment support
6. US4 (permission safety) -> Safety improvement
7. US6 (session tracking) -> Multi-session support
8. Polish -> Documentation, cleanup

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Constitution requires: make install (never go build/cargo build), prose plugin for docs, XDG paths via internal/xdg
- Two-component architecture: Go changes in cc-deck/, Rust changes in cc-zellij-plugin/
- WASM host functions must be #[cfg(target_family = "wasm")] gated
- Commit after each task or logical group
