# Tasks: Voice Glossary for Whisper

**Feature**: 077-voice-glossary
**Generated**: 2026-07-03

## Phase 1: Glossary Module

- [ ] T001 Create glossary struct with global terms field and project cache map in `cc-deck/internal/voice/glossary.go`
- [ ] T002 Implement `LoadFile(path)` to parse `.cc-deck/voice-glossary.txt` (terms, comments, blanks) in `cc-deck/internal/voice/glossary.go`
- [ ] T003 Implement `ResolvePrompt(workingDir)` to merge global + project terms, dedup case-insensitively, return comma-separated string in `cc-deck/internal/voice/glossary.go`

## Phase 2: Transcriber Prompt Support

- [ ] T004 Add `prompt` field and `SetPrompt(string)` method to httpTranscriber in `cc-deck/internal/voice/transcriber_http.go`
- [ ] T005 Add `writer.WriteField("prompt", t.prompt)` to Transcribe() when prompt is non-empty in `cc-deck/internal/voice/transcriber_http.go`

## Phase 3: Relay Integration

- [ ] T006 Add `glossary *Glossary` field to relay struct and initialize from config on startup in `cc-deck/internal/voice/relay.go`
- [ ] T007 Extract attended session's `working_dir` from state dump response in `cc-deck/internal/voice/relay.go`
- [ ] T008 On session switch, call `glossary.ResolvePrompt(workingDir)` and set prompt on transcriber in `cc-deck/internal/voice/relay.go`

## Phase 4: Config Support

- [ ] T009 Add `Glossary []string` to Voice config section in `cc-deck/internal/config/config.go`

## Phase 5: Documentation

- [ ] T010 Add voice glossary section to README.md documenting global config and project file

## Phase 6: Tests

- [ ] T011 [P] Add tests for LoadFile (terms, comments, blanks, missing file) in `cc-deck/internal/voice/glossary_test.go`
- [ ] T012 [P] Add tests for ResolvePrompt (merge, dedup, empty, warning) in `cc-deck/internal/voice/glossary_test.go`
- [ ] T013 [P] Add test for prompt form field inclusion/omission in `cc-deck/internal/voice/transcriber_http_test.go`

## Dependencies

```
T001 (no dependencies, foundational)
T002, T003 depend on T001
T004, T005 (independent of T001-T003)
T006 depends on T001, T003, T009
T007, T008 depend on T004, T006
T009 (independent)
T010 (independent)
T011, T012 depend on T001-T003
T013 depends on T004-T005
```

## Parallel Execution

- T001-T003 and T004-T005 can run in parallel (different files)
- T009 can run in parallel with everything
- T011, T012, T013 are all parallelizable

## Implementation Strategy

**MVP (Phase 1-2)**: T001-T005 deliver the core glossary loading and prompt injection. Testable with a hardcoded prompt.

**Full feature**: T006-T008 add automatic session switching. T009 adds config support. T010-T013 add docs and tests.

Total: 13 tasks across 6 phases.
