# Feature Specification: Voice Glossary for Whisper

**Feature Branch**: `077-voice-glossary`
**Created**: 2026-07-03
**Status**: Draft
**Input**: Add two-tier glossary system for Whisper speech recognition to improve technical term accuracy

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Global Glossary Improves Recognition (Priority: P1)

A developer configures a global glossary of technical terms in cc-deck's config file. When they dictate prompts via voice relay, Whisper correctly recognizes terms like "Kubernetes", "gRPC", and "OpenShell" instead of producing phonetic guesses.

**Why this priority**: This is the core value. Without the glossary, Whisper consistently misrecognizes domain-specific terms, making voice dictation unreliable for technical work.

**Independent Test**: Can be tested by adding terms to the global config, starting the voice relay, and verifying that dictated terms match the glossary entries.

**Acceptance Scenarios**:

1. **Given** a global glossary containing "Kubernetes, gRPC, OpenShell", **When** the user dictates a prompt mentioning these terms, **Then** Whisper transcribes them correctly.
2. **Given** no glossary is configured, **When** the user dictates a prompt, **Then** the voice relay behaves identically to the current behavior (no regression).

---

### User Story 2 - Project Glossary Activates on Session Switch (Priority: P2)

A developer has project-specific terms in `.cc-deck/voice-glossary.txt` for their Kubernetes project. When they switch to that session via "next" or "ship", the voice relay automatically loads the project glossary and merges it with the global glossary.

**Why this priority**: Different projects use different terminology. Automatic switching eliminates manual reconfiguration when moving between sessions.

**Independent Test**: Can be tested by creating two projects with different glossary files, switching between their sessions, and verifying that dictation accuracy reflects the active project's terms.

**Acceptance Scenarios**:

1. **Given** a project with `.cc-deck/voice-glossary.txt` containing "kubectl, Helm, Ingress", **When** the user switches to that session, **Then** subsequent dictation uses both global and project terms.
2. **Given** a session switch to a project without a glossary file, **When** the user dictates, **Then** only the global glossary is used.
3. **Given** a session switch between two projects with different glossaries, **When** the user switches, **Then** the active glossary updates to the new project's terms.

---

### User Story 3 - Glossary File Format (Priority: P3)

A developer creates a `.cc-deck/voice-glossary.txt` file in their project with one term per line, supporting blank lines and comment lines (starting with #) for organization.

**Why this priority**: The file format must be simple enough to edit by hand and committable to a shared repo so team members benefit from the same glossary.

**Independent Test**: Can be tested by creating a glossary file with terms, comments, and blank lines, and verifying that only the terms are loaded.

**Acceptance Scenarios**:

1. **Given** a glossary file with terms, blank lines, and comment lines, **When** the relay loads it, **Then** only non-empty, non-comment lines are treated as terms.
2. **Given** a glossary file with duplicate terms (same word, different casing), **When** merged with the global glossary, **Then** only unique terms remain (case-insensitive deduplication).

---

### Edge Cases

- What happens when the merged glossary exceeds Whisper's 224-token limit? The relay logs a warning at startup or on session switch, but still passes the full list (Whisper truncates silently from the beginning, so project-specific terms at the end are preserved).
- What happens when the glossary file changes while the relay is running? The cached version is used until the relay restarts. Changes require a relay restart.
- What happens when the attended session has no working_dir set (e.g., session just created)? The relay uses only the global glossary.
- What happens when the glossary file path doesn't exist? The relay silently uses only the global glossary (no error, no warning).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support a global glossary configured as a list of terms under `voice.glossary` in the cc-deck config file.
- **FR-002**: The system MUST support a project-local glossary file at `.cc-deck/voice-glossary.txt` in the project root directory.
- **FR-003**: The project glossary file MUST support one term per line, blank lines (ignored), and comment lines starting with `#` (ignored).
- **FR-004**: The plugin state dump response MUST include the `working_dir` field for the attended session so the relay can locate the project glossary. *(Note: the existing `DumpStateResponse` already serializes the full `Session` struct which includes `working_dir`. This requirement is satisfied by current code and is listed here for traceability.)*
- **FR-005**: The relay MUST load the project glossary lazily on session switch, using the attended session's working directory from the state dump.
- **FR-006**: The relay MUST cache loaded project glossaries by directory path to avoid repeated file reads. The cache is not invalidated while the relay is running; changes to glossary files require a relay restart to take effect.
- **FR-007**: The relay MUST merge global and project glossaries with global terms first and project terms last (project terms receive higher priority in Whisper's 224-token window).
- **FR-008**: The merged glossary MUST contain only unique terms (case-insensitive deduplication). When a term appears in both the global and project glossaries with different casing, the project glossary's casing wins (last occurrence takes precedence).
- **FR-009**: The relay MUST pass the merged glossary as a comma-separated string in the `prompt` form field of the multipart request to whisper-server's `/inference` endpoint. The prompt is injected by setting it as state on the `httpTranscriber` (via a setter method or constructor parameter), not by changing the `Transcriber` interface signature.
- **FR-010**: When no glossary is configured (neither global nor project), the relay MUST omit the `prompt` field entirely (preserving current behavior).
- **FR-011**: The relay MUST log a warning when the merged glossary exceeds approximately 224 tokens (~800 characters).

### Key Entities

- **Global Glossary**: A list of terms in the cc-deck config file (`voice.glossary`), loaded once at relay startup.
- **Project Glossary**: A text file (`.cc-deck/voice-glossary.txt`) in a project directory, loaded on first session switch to that project.
- **Glossary Cache**: An in-memory map from directory path to parsed term list, populated lazily.
- **Merged Prompt**: The final comma-separated string of deduplicated terms passed to Whisper.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Technical terms listed in the glossary are recognized correctly by Whisper in at least 80% of dictation attempts (compared to baseline without glossary).
- **SC-002**: Session switches load and apply the correct project glossary within the same transcription cycle (no lag or missed dictation).
- **SC-003**: The relay starts and operates without errors when no glossary is configured (zero regression from current behavior).
- **SC-004**: Project glossary files with up to 50 terms load in under 10ms.

## Test Strategy

- **Unit tests** for glossary file parsing (blank lines, comments, deduplication, empty files, missing files).
- **Unit tests** for merge logic (global-only, project-only, both, deduplication with casing precedence, token limit warning).
- **Unit tests** for the `prompt` field in the multipart request (present when glossary exists, absent when no glossary configured).
- **Integration tests** for the relay's session-switch glossary loading (using the mock transcriber pattern already established in `relay_test.go`).

## Documentation Impact

- **README.md**: Add a subsection under voice relay documentation explaining the glossary configuration (global and project-local).
- **Configuration reference**: Document the `voice.glossary` config key.
- No CLI command changes (no new flags or subcommands).

## Assumptions

- Whisper's `initial_prompt` / `prompt` parameter accepts a comma-separated list of terms and biases the decoder toward those spellings.
- The whisper-server HTTP API accepts a `prompt` form field in the multipart `/inference` request.
- The 224-token limit (approximately 800 characters or 40-50 terms) is sufficient for typical project vocabularies.
- The relay already polls the plugin state dump on session switches (the glossary loading piggybacks on this existing mechanism).
- The `working_dir` field on Session is already populated by the hook system (it just needs to be included in the state dump response).

## Clarifications

### Session 2026-07-03

- Q: Should the relay warn or silently truncate when glossary exceeds 224 tokens? -> A: Log a warning but still pass the full list (Whisper truncates from the beginning, preserving project terms at the end).
- Q: Should .cc-deck/voice-glossary.txt support comments? -> A: Yes, lines starting with # are ignored.
