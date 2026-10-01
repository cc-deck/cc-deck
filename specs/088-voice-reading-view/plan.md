# Implementation Plan: Voice Reading View with Turn Detection

**Branch**: `088-voice-reading-view` | **Date**: 2026-10-01 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/088-voice-reading-view/spec.md`

## Summary

Add a pager-style reading view to the voice relay TUI that shows the active recording as wrapped chat blocks grouped into speaker turns, updates live, and follows new text.
The relay gains structured transcription segments (text, timing, turn start flag, empty speaker slot), recording-specific VAD settings that keep silence detection active (short passages instead of 30 second chunks), and an opt-in `tdrz` turn mode that transcribes recordings through `whisper-cli -tdrz -oj` with the tinydiarize model while `whisper-server` keeps serving dictation.
Basic turn detection uses Whisper dash markers plus a pause-break threshold.
The transcript file gets blank lines between turns.

## Technical Context

**Language/Version**: Go 1.25 (from `cc-deck/go.mod`)

**Primary Dependencies**: bubbletea v1.3.10, bubbles (viewport, textinput), lipgloss v1.1.0 (styling and word wrap), cobra (CLI), gopkg.in/yaml.v3 (config); external tools `whisper-server` and `whisper-cli` from whisper.cpp 1.9.x

**Storage**: Model files in the existing model cache (`voice.ModelDir()`, `~/.cache/cc-deck/models`), transcript files in `~/.local/share/cc-deck/transcripts/` (unchanged), config in `~/.config/cc-deck/config.yaml` (`defaults.voice`)

**Testing**: `go test` via `make test`; table-driven unit tests with stub transcribers, a stub `whisper-cli` runner, and direct bubbletea `Update`/`View` calls (existing pattern in `internal/tui/voice/transcript_test.go`)

**Target Platform**: macOS and Linux terminals (the relay runs on the host)

**Project Type**: CLI with an interactive TUI (`cc-deck ws voice`)

**Performance Goals**: Recording text visible within 5 seconds of a 1 second pause (SC-001); `whisper-cli` small.en measured at about 0.5 seconds per call on the reference machine; reading view stays responsive for a one-hour recording (about 300 to 400 passages)

**Constraints**: Release builds use `CGO_ENABLED=0` (no in-process whisper bindings); dictation behavior and delivered text unchanged (FR-017, FR-024); never `go build` directly (constitution III)

**Scale/Scope**: One TUI package (about 1,200 lines today) and one relay package touched; about 10 new or modified Go files plus 4 documentation pages

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | How the plan satisfies it |
|-----------|--------|---------------------------|
| I. Tests and documentation | PASS | FR-035 test coverage mapped to `*_test.go` files below; README, `cli.adoc`, `configuration.adoc`, and the `using/pages/voice.adoc` guide are updated in this branch (FR-032) |
| II. Interface contracts | PASS (n/a) | No new backend for an existing interface. The new `TurnTranscriber` is a new interface; its contract is written in `contracts/whisper-cli-tdrz.md` before implementation |
| III. Build and tool rules | PASS | Builds and tests via `make test` / `make lint` / `make install` only; paths through `internal/xdg`; no container work |
| IV. Plugin debug logging | n/a | No plugin (Rust) changes |
| V. Command files | n/a | No `internal/build/commands/*.md` changes |

Post-design re-check (after Phase 1): PASS. The design adds no new external services beyond the already-required whisper.cpp tools, and documentation tasks are part of the same branch.

## Project Structure

### Documentation (this feature)

```text
specs/088-voice-reading-view/
├── plan.md              # This file
├── research.md          # Phase 0 decisions
├── data-model.md        # Segment, Passage timing, Turn block, settings
├── quickstart.md        # Manual validation scenarios
├── contracts/
│   ├── tui-keys.md          # Key bindings, header and footer contract
│   ├── whisper-cli-tdrz.md  # TurnTranscriber contract and whisper-cli invocation
│   ├── transcript-format.md # Transcript file turn separation
│   └── config.md            # New defaults.voice keys and validation
└── tasks.md             # Created by /speckit-tasks
```

### Source Code (repository root)

```text
cc-deck/internal/voice/
├── audio.go               # MODIFIED: Utterance gains Start/End offsets; RecordingConfig + defaults
├── vad.go                 # MODIFIED: live parameter snapshot per frame, sample-based passage timing
├── turns.go               # NEW: Segment type, dash-marker splitting, tdrz JSON parsing, pause-break rule
├── turns_test.go          # NEW
├── transcriber_tdrz.go    # NEW: TurnTranscriber via whisper-cli -tdrz -oj (injectable runner)
├── transcriber_tdrz_test.go # NEW
├── relay.go               # MODIFIED: StartRecording/StopRecording, recording branch emits Segments, fallback, verbose logs
├── relay_test.go          # MODIFIED: recording settings save/restore, segments, fallback, timeout
├── setup.go               # MODIFIED: model registry with per-repo tree API, small.en-tdrz entry, DownloadModel with progress, TdrzStatus
└── setup_test.go          # NEW or MODIFIED: registry, status checks, download integrity with httptest

cc-deck/internal/config/
├── config.go              # MODIFIED: VoiceDefaults.TurnMode, VoiceDefaults.Recording
└── validate.go            # MODIFIED: validateVoice covers turn_mode and recording ranges

cc-deck/internal/cmd/
└── ws_voice.go            # MODIFIED: wire recording config, tdrz transcriber, initial turn mode

cc-deck/internal/tui/voice/
├── model.go               # MODIFIED: turn mode, reading state, recording buffer, download state
├── update.go              # MODIFIED: shared relay-event handler, v/g/esc gating, download prompt
├── view.go                # MODIFIED: turn mode in header, v/g hints, reading view dispatch
├── transcript.go          # MODIFIED: segment-aware writer with blank lines between turns
├── reading.go             # NEW: turn blocks, wrapping, gutter bars, follow mode, new-block counter
├── reading_test.go        # NEW
├── turnmode.go            # NEW: g toggle, availability checks, background download commands
├── turnmode_test.go       # NEW
└── transcript_test.go     # MODIFIED: turn separation and timestamp format

README.md                                       # MODIFIED: voice relay section
docs/modules/reference/pages/cli.adoc           # MODIFIED: ws voice keys, tdrz setup
docs/modules/reference/pages/configuration.adoc # MODIFIED: turn_mode, recording keys
docs/modules/using/pages/voice.adoc             # MODIFIED: reading view and turn detection guide
```

**Structure Decision**: The feature stays inside the two existing voice packages. Turn logic that does not depend on the TUI (segment model, dash splitting, JSON parsing, pause-break rule) lives in `internal/voice` so it is unit-testable without bubbletea. Everything about presentation (blocks, wrapping, follow mode, keys) lives in `internal/tui/voice`.

## Design Overview

### Relay data flow while recording

```text
mic -> VAD (recording settings, passage Start/End offsets)
    -> handleUtterance
         recording && tdrz  -> TurnTranscriber (30 s timeout) -> []Segment
                                 on error/timeout -> configured Transcriber -> SplitDashTurns  (+ error event)
         recording && basic -> configured Transcriber -> SplitDashTurns -> []Segment
         apply pause-break rule (gap = u.Start - lastRecordingEnd > PauseBreak, or first passage) -> segs[0].TurnStart
         artifact / repeat / latency filters on the joined text (unchanged)
    -> RelayEvent{Type: "transcription", Text: joined, Segments: segs, Latency}
dictation (not recording): unchanged path, Segments == nil, delivered text sanitized as today
```

### Recording settings lifecycle

`StartRecording(mode)` saves the dictation `Threshold`, `SilenceDuration`, `MaxUtteranceDuration`, and mute state, applies `RecordingConfig`, mutes delivery, resolves the effective turn mode (falling back to `basic` with an error if `whisper-cli` or the model is missing), and records the recording start (wall clock and sample offset).
`StopRecording()` restores all saved values.
`SetVADThreshold` keeps working on the live config, so `+`/`-` during a recording adjust only the recording threshold.
The VAD reads a snapshot of the config under the relay mutex for every frame, which makes silence and maximum chunk changes take effect immediately and removes the existing unsynchronized threshold read.

### TUI structure

`Update` routes relay events through one shared handler for all modes (normal, filename prompt, device picker, reading view, download prompt), replacing the duplicated handling in `updateFilenamePrompt`.
While `recState == recRecording`, incoming segments are appended to the transcript file (segment-aware writer) and to the recording buffer that backs the reading view.
The reading view owns a second `viewport.Model`; it re-renders on new segments and on resize, keeps the offset when the user scrolled up, and counts blocks that arrived below.

## Complexity Tracking

No constitution violations to justify.
