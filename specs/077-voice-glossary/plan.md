# Implementation Plan: Voice Glossary for Whisper

**Branch**: `077-voice-glossary` | **Date**: 2026-07-03 | **Spec**: [spec.md](spec.md)

## Summary

Add a two-tier glossary system (global config + project-local file) that passes domain-specific terms to Whisper via the `prompt` form field, improving recognition of technical vocabulary. The glossary switches automatically when the user attends a different session.

## Technical Context

**Language/Version**: Go 1.25 (CLI/relay), Rust stable wasm32-wasip1 (plugin)
**Framework**: cobra (CLI), zellij-tile 0.43.1 (plugin SDK)
**Storage**: `~/.config/cc-deck/config.yaml` (global), `.cc-deck/voice-glossary.txt` (project)
**Build**: `make install`, `make test`, `make lint`

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests for glossary parsing/merge. README update for voice glossary config. |
| II. Interface contracts | N/A | No new interfaces. |
| III. Build and tool rules | PASS | Use `make install`, `make test`, `make lint`. |

## Implementation Phases

### Phase 1: Glossary Module (FR-001, FR-002, FR-003, FR-006, FR-007, FR-008)

**New file**: `cc-deck/internal/voice/glossary.go`

Create a `Glossary` struct that handles:
- Loading global terms from a string slice (from config)
- Loading project terms from `.cc-deck/voice-glossary.txt` (one term per line, skip blank lines and `#` comments)
- Caching project glossaries by directory path (`map[string][]string`)
- Merging: global first, project last
- Deduplication: case-insensitive, project casing wins (last occurrence keeps its casing)
- Building the final comma-separated prompt string
- Warning when merged prompt exceeds ~800 characters

### Phase 2: Transcriber Prompt Support (FR-009, FR-010)

**File**: `cc-deck/internal/voice/transcriber_http.go`

Add a `prompt string` field to `httpTranscriber`. Add a `SetPrompt(string)` method. In `Transcribe()`, when `prompt` is non-empty, add `writer.WriteField("prompt", t.prompt)` to the multipart form before closing. When empty, omit the field (current behavior preserved).

### Phase 3: Relay Integration (FR-005, FR-011)

**File**: `cc-deck/internal/voice/relay.go`

- Add a `glossary *Glossary` field to the relay struct
- On relay startup: create `Glossary` with global terms from config
- In `parseDumpStateResponse`: extract the attended session's `working_dir` from the sessions map
- On session switch: call `glossary.ResolvePrompt(workingDir)` to get the merged prompt, set it on the transcriber via `SetPrompt()`

### Phase 4: Config Support (FR-001)

**File**: `cc-deck/internal/config/config.go`

Add `Glossary []string` to the Voice config section. Parse from `voice.glossary` in config YAML.

### Phase 5: Documentation

**File**: `README.md`

Add a subsection under the voice relay section documenting:
- Global glossary config (`voice.glossary` list in config.yaml)
- Project glossary file (`.cc-deck/voice-glossary.txt`)
- Token limit (~50 terms, ~800 chars)
- Automatic switching on session change

### Phase 6: Tests

- `glossary_test.go`: LoadFile (terms, comments, blanks), Merge, Dedup, ResolvePrompt, empty cases
- Existing `relay_test.go` patterns for integration behavior

## Key Decisions

- **Inject prompt via setter, not interface change**: The `Transcriber` interface stays unchanged. `httpTranscriber` gets a `SetPrompt(string)` method. The relay calls it when the glossary changes.
- **Cache never invalidated at runtime**: Project glossaries are cached by directory path. Changes require relay restart.
- **Comma-separated format**: Terms are joined with `, ` for the Whisper prompt.
- **State dump already includes working_dir**: The `DumpStateResponse` serializes full Session structs which include `working_dir`. The relay just needs to parse it.
