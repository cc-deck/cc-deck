# Tasks: Voice Glossary for Whisper

**Feature**: 077-voice-glossary
**Generated**: 2026-07-03

## Phase 1: Glossary Module

- [x] T001 Create glossary struct with global terms field and project cache map in `cc-deck/internal/voice/glossary.go`

  **Interfaces provided (consumed by T002, T003, T006):**
  ```go
  type Glossary struct {
      globalTerms  []string
      projectCache map[string][]string // dir path -> parsed terms
  }
  func NewGlossary(globalTerms []string) *Glossary
  ```

- [x] T002 Implement `LoadFile(path string) ([]string, error)` to parse `.cc-deck/voice-glossary.txt` (one term per line, skip blank lines and `#` comment lines, return nil/empty for missing file) in `cc-deck/internal/voice/glossary.go`

- [x] T003 Implement `ResolvePrompt(workingDir string) string` to merge global + project terms (global first, project last), dedup case-insensitively (project casing wins), return comma-separated string. Log a warning via `log.Printf` when merged prompt exceeds 800 characters (FR-011). Return empty string when no terms are configured. File: `cc-deck/internal/voice/glossary.go`

  **Interfaces provided (consumed by T006, T008):**
  ```go
  func (g *Glossary) ResolvePrompt(workingDir string) string
  ```

## Phase 2: Transcriber Prompt Support

- [x] T004 Add `prompt string` field and `SetPrompt(prompt string)` method to `httpTranscriber` in `cc-deck/internal/voice/transcriber_http.go`. The `Transcriber` interface is NOT changed; `SetPrompt` is a concrete method only.

  **Interfaces provided (consumed by T008):**
  ```go
  func (t *httpTranscriber) SetPrompt(prompt string)
  ```

- [x] T005 In `httpTranscriber.Transcribe()`, add `writer.WriteField("prompt", t.prompt)` after the file part and before `writer.Close()`, but only when `t.prompt != ""` (FR-010: omit field entirely when no glossary). File: `cc-deck/internal/voice/transcriber_http.go`

## Phase 3: Relay Integration

- [x] T006 Add `glossary *Glossary` field to `VoiceRelay` struct. In `NewVoiceRelay`, accept `globalTerms []string` parameter and call `NewGlossary(globalTerms)`. File: `cc-deck/internal/voice/relay.go`

  **Interfaces consumed:** `NewGlossary([]string) *Glossary` from T001

- [x] T007 Add `workingDir string` field to `dumpStateResult` struct. In `parseDumpStateResponse`, extract the attended session's `working_dir` from the session JSON (the Session struct already serializes it). Use the same pane-ID resolution logic as `resolveSessionName` but read `working_dir` instead of `display_name`. File: `cc-deck/internal/voice/relay.go`

- [x] T008 In `statePoll`, detect when `state.workingDir` changes from the previous value. On change, call `r.glossary.ResolvePrompt(state.workingDir)` and set the result on the transcriber via type assertion: `if ht, ok := r.transcriber.(*httpTranscriber); ok { ht.SetPrompt(prompt) }`. File: `cc-deck/internal/voice/relay.go`

  **Interfaces consumed:** `ResolvePrompt(string) string` from T003, `SetPrompt(string)` from T004

## Phase 4: Config Support

- [x] T009 Add `Glossary []string \`yaml:"glossary,omitempty"\`` field to `VoiceDefaults` struct in `cc-deck/internal/config/config.go`. YAML path is `defaults.voice.glossary` (list of strings).

## Phase 5: Documentation

- [x] T010 Add voice glossary section to README.md documenting global config (`defaults.voice.glossary`) and project file (`.cc-deck/voice-glossary.txt`). Also update `docs/modules/reference/pages/configuration.adoc` to document the `glossary` key under the existing `defaults.voice` section.

## Phase 6: Tests

- [x] T011 [P] Add tests for LoadFile (terms, comments, blanks, missing file) in `cc-deck/internal/voice/glossary_test.go`
- [x] T012 [P] Add tests for ResolvePrompt (merge, dedup, empty, warning) in `cc-deck/internal/voice/glossary_test.go`
- [x] T013 [P] Add test for prompt form field inclusion/omission in `cc-deck/internal/voice/transcriber_http_test.go`

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
