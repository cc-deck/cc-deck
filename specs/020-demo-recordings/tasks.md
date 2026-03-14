# Tasks: Demo Recording System

**Input**: Design documents from `/specs/020-demo-recordings/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/pipe-commands.md

**Tests**: Not explicitly requested. Test tasks omitted.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create directory structure and shared infrastructure

- [x] T001 Create `demos/` directory structure: `demos/scripts/`, `demos/projects/`, `demos/narration/`, `demos/recordings/`
- [x] T002 [P] Add `demos/recordings/.gitkeep` and add `demos/recordings/*.cast`, `demos/recordings/*.gif`, `demos/recordings/*.mp4` to `.gitignore`
- [x] T003 [P] Create demo runner framework in `demos/runner.sh` with helper functions: `scene()`, `pause()`, `wait_for()`, `cc_pipe()`, `type_command()`

---

## Phase 2: Foundational (Plugin Pipe Handlers)

**Purpose**: Add pipe message support to the plugin. BLOCKS User Stories 1 and 3.

**CRITICAL**: No demo script can use plugin control until this phase is complete.

- [x] T004 Add new PipeAction variants (NavToggle, NavUp, NavDown, NavSelect, Pause, Help) to enum in `cc-zellij-plugin/src/pipe_handler.rs`
- [x] T005 Add pipe name parsing for `cc-deck:nav-toggle`, `cc-deck:nav-up`, `cc-deck:nav-down`, `cc-deck:nav-select`, `cc-deck:pause`, `cc-deck:help` in `cc-zellij-plugin/src/pipe_handler.rs`
- [x] T006 Add match arms for new PipeAction variants in `pipe()` method in `cc-zellij-plugin/src/main.rs`, calling existing action methods
- [x] T007 Verify pipe handlers work: `make install`, launch Zellij, test `zellij pipe --name "cc-deck:nav-toggle"` and other commands manually

**Checkpoint**: Plugin responds to all pipe commands. Demo scripts can now control the plugin programmatically.

---

## Phase 3: User Story 2 - Create Demo Projects (Priority: P2)

**Goal**: Three small, recognizable demo projects that Claude Code can work on predictably.

**Independent Test**: Run `demos/projects/setup.sh`, verify 3 repos created with git history and CLAUDE.md, run `cleanup.sh` to remove them.

**Note**: Placed before US1 because demo scripts depend on having projects to work with.

### Implementation

- [x] T008 [P] [US2] Create Python demo project template in `demos/projects/todo-api/`: FastAPI TODO app (~50 lines), README.md, CLAUDE.md with task "Add a /search endpoint", requirements.txt
- [x] T009 [P] [US2] Create Go demo project template in `demos/projects/weather-cli/`: simple CLI that prints weather (~40 lines), README.md, CLAUDE.md with task "Add --format json flag", go.mod
- [x] T010 [P] [US2] Create HTML/CSS demo project template in `demos/projects/portfolio/`: static portfolio page (~60 lines HTML + CSS), README.md, CLAUDE.md with task "Add dark mode toggle"
- [x] T011 [US2] Create setup script `demos/projects/setup.sh` that copies templates to `/tmp/cc-deck-demo/`, initializes git repos with 2 commits each
- [x] T012 [US2] Create cleanup script `demos/projects/cleanup.sh` that removes `/tmp/cc-deck-demo/`

**Checkpoint**: Demo projects can be set up and torn down in under 30 seconds.

---

## Phase 4: User Story 1 - Record a Scripted Plugin Demo (Priority: P1) MVP

**Goal**: A fully scripted demo recording showing plugin installation, session creation, sidebar navigation, and smart attend.

**Independent Test**: Run `make demo-record DEMO=plugin`, verify .cast file is produced with all scenes captured.

### Implementation

- [x] T013 [US1] Write plugin demo screenplay in `demos/scripts/plugin-demo.sh` using runner.sh helpers: scenes for install, launch, create sessions, navigate, smart attend
- [x] T014 [US1] Add asciinema integration to `demos/runner.sh`: `start_recording()` and `stop_recording()` functions wrapping `asciinema rec`
- [x] T015 [US1] Add checkpoint-based wait function to `demos/runner.sh`: `wait_for_output()` that polls `zellij action query-tab-names` or checks terminal output patterns
- [x] T016 [US1] Add Makefile targets in `Makefile`: `demo-setup`, `demo-record`, `demo-gif`, `demo-clean` in a new "Demo" section
- [ ] T017 [US1] Test end-to-end: run `demos/projects/setup.sh`, then `demos/scripts/plugin-demo.sh`, verify recording captures sidebar interactions

**Checkpoint**: A maintainer can produce a complete plugin demo recording by running a single command.

---

## Phase 5: User Story 3 - Deploy and Image Demos (Priority: P2)

**Goal**: Two additional demo scripts covering image deployment and custom image creation.

**Independent Test**: Run each demo script independently, verify .cast files are produced.

### Implementation

- [x] T018 [P] [US3] Write deployment demo screenplay in `demos/scripts/deploy-demo.sh`: scenes for container launch, session creation, reconnection
- [x] T019 [P] [US3] Write image builder demo screenplay in `demos/scripts/image-demo.sh`: scenes for manifest review, build, container launch
- [x] T020 [US3] Create pre-built image builder manifest in `demos/projects/cc-deck-build.yaml` for the three demo projects

**Checkpoint**: All three demo scripts produce complete recordings.

---

## Phase 6: User Story 4 - Generate Voiceover Audio (Priority: P3)

**Goal**: Convert narration scripts to audio using OpenAI TTS API.

**Independent Test**: Run voiceover script with a narration file, verify audio file is produced.

### Implementation

- [x] T021 [P] [US4] Write narration script for plugin demo in `demos/narration/plugin-demo.txt` with `## scene:` chapter markers
- [x] T022 [P] [US4] Write narration scripts for deploy and image demos in `demos/narration/deploy-demo.txt` and `demos/narration/image-demo.txt`
- [x] T023 [US4] Create voiceover generation script `demos/voiceover.sh` that reads narration files, calls OpenAI TTS API (`tts-1-hd`), outputs audio files to `demos/recordings/`
- [x] T024 [US4] Add `demo-voiceover` Makefile target

**Checkpoint**: Narration scripts produce aligned audio files.

---

## Phase 7: User Story 5 - Multiple Output Formats (Priority: P3)

**Goal**: Convert recordings to GIF (landing page), MP4 with voiceover (team), embeddable (docs).

**Independent Test**: Run conversion pipeline on a .cast file, verify all output formats are produced.

### Implementation

- [x] T025 [US5] Add GIF conversion to Makefile `demo-gif` target using agg with landing-page-optimized settings (idle-time-limit, fps-cap, last-frame-duration)
- [x] T026 [US5] Add MP4 conversion to Makefile `demo-mp4` target using ffmpeg to combine GIF with voiceover audio
- [x] T027 [US5] Add `demo-mp4` Makefile target that runs voiceover generation + video conversion
- [x] T028 [US5] Document embedding options for Antora docs in `demos/README.md`

**Checkpoint**: All three output formats produced from a single recording.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and integration

- [x] T029 Update README.md with demo recording instructions and spec table entry for 020-demo-recordings
- [x] T030 [P] Add `demos/README.md` with usage instructions, prerequisites, and troubleshooting
- [ ] T031 Run quickstart.md validation: execute the quickstart steps end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: No dependencies on Phase 1 (different files), can start in parallel
- **US2 Demo Projects (Phase 3)**: No dependencies on Phase 2 (no pipe commands needed)
- **US1 Plugin Demo (Phase 4)**: Depends on Phase 2 (pipe handlers) AND Phase 3 (demo projects)
- **US3 Deploy/Image Demos (Phase 5)**: Depends on Phase 4 (runner framework)
- **US4 Voiceover (Phase 6)**: Depends on Phase 4 (completed recordings to narrate)
- **US5 Output Formats (Phase 7)**: Depends on Phase 4 (recordings to convert)
- **Polish (Phase 8)**: Depends on all desired phases being complete

### User Story Dependencies

- **US2 (Demo Projects)**: Independent, can start after Setup
- **US1 (Plugin Demo)**: Depends on Foundational (pipe handlers) + US2 (projects)
- **US3 (Deploy/Image)**: Depends on US1 (runner framework)
- **US4 (Voiceover)**: Depends on US1 (recordings exist)
- **US5 (Output Formats)**: Depends on US1 (recordings exist)

### Parallel Opportunities

**Phase 1 + Phase 2**: Can run in parallel (different files)

**Within Phase 3 (US2)**: T008, T009, T010 can run in parallel (independent project templates)

**Phase 6 + Phase 7**: Can run in parallel once recordings exist (voiceover and format conversion are independent)

---

## Parallel Example: Setup + Foundational

```bash
# These can run simultaneously (different files):
Task T001: "Create demos/ directory structure"
Task T004: "Add PipeAction variants in cc-zellij-plugin/src/pipe_handler.rs"

# These can run simultaneously (independent project templates):
Task T008: "Create Python demo project in demos/projects/todo-api/"
Task T009: "Create Go demo project in demos/projects/weather-cli/"
Task T010: "Create HTML demo project in demos/projects/portfolio/"
```

---

## Implementation Strategy

### MVP First (US2 + Foundational + US1)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational pipe handlers (T004-T007)
3. Complete Phase 3: Demo projects (T008-T012)
4. Complete Phase 4: Plugin demo recording (T013-T017)
5. **STOP and VALIDATE**: Run the plugin demo end-to-end
6. Convert to GIF for landing page

### Incremental Delivery

1. Setup + Foundational + Demo Projects -> Foundation ready
2. Add US1 (Plugin Demo) -> Test, produce first recording (MVP!)
3. Add US3 (Deploy/Image Demos) -> Three complete recordings
4. Add US4 (Voiceover) -> Narrated versions
5. Add US5 (Output Formats) -> All delivery formats ready
6. Polish -> Documentation and integration

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Commit after each task or logical group
- Demo projects are templates, not live repos (copied to /tmp during recording)
- All pipe commands use existing cc-deck:* namespace
- Recordings directory is gitignored (generated output)
