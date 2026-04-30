# Tasks: Voice Attend Stop Word

**Input**: Design documents from `specs/046-voice-attend-stopword/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Included per spec SC-005 requirement for unit test coverage.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No project initialization needed. All changes extend existing files.

(No setup tasks required. The project structure already exists.)

---

## Phase 2: Foundational (Go Side: Stop Word + Relay)

**Purpose**: Extend the Go voice relay with the new "attend" action. These changes are prerequisites for all user stories since the plugin handler depends on receiving `[[attend]]` payloads.

- [ ] T001 [P] Add `"attend": {"next"}` to `DefaultCommands` map in `cc-deck/internal/voice/stopword.go`
- [ ] T002 [P] Add `case "attend": payload = "[[attend]]"` to the command dispatch switch in `cc-deck/internal/voice/relay.go` (around line 431)
- [ ] T003 [P] Add unit tests for "next" triggering "attend" action in `cc-deck/internal/voice/stopword_test.go` (test standalone detection, filler stripping, sentence embedding)
- [ ] T004 [P] Add unit test verifying "next" produces `[[attend]]` payload on the voice pipe in `cc-deck/internal/voice/relay_test.go`

**Checkpoint**: Go side complete. `make test` passes. "next" utterance produces `[[attend]]` on the voice pipe.

---

## Phase 3: User Story 1 - Voice-Driven Session Cycling (Priority: P1)

**Goal**: Saying "next" as a standalone utterance cycles to the next attended session using tiered attend logic.

**Independent Test**: Start voice relay with multiple sessions in various states, say "next", verify correct session receives focus.

### Implementation for User Story 1

- [ ] T005 [US1] Add `"attend"` match arm in `handle_voice_command()` in `cc-zellij-plugin/src/controller/mod.rs` that calls the attend logic (perform_attend)

**Checkpoint**: Core feature works. Saying "next" cycles to the next attended session.

---

## Phase 4: User Story 2 - Custom Trigger Word Configuration (Priority: P2)

**Goal**: Users can configure a different trigger word for the "attend" action.

**Independent Test**: Configure custom word in commands map, say that word, verify attend triggers.

### Implementation for User Story 2

(No additional code changes needed. The existing `commands` map and `BuildCommandMap()` already support custom words per action. When a user overrides `DefaultCommands` with `"attend": ["switch"]`, "switch" maps to the "attend" action automatically. This is covered by existing test infrastructure in `TestProcessStopwords_CustomCommands`.)

- [ ] T006 [US2] Add unit test in `cc-deck/internal/voice/stopword_test.go` verifying custom words for "attend" action work correctly (e.g., "switch" triggers "attend")

**Checkpoint**: Configuration works. Custom trigger words correctly map to the attend action.

---

## Phase 5: User Story 3 - Multiple Trigger Words (Priority: P3)

**Goal**: Users can configure multiple trigger words for the attend action.

**Independent Test**: Configure multiple words, verify each triggers attend.

### Implementation for User Story 3

(No additional code changes needed. `BuildCommandMap()` already iterates over word slices, so `"attend": ["next", "switch"]` maps both words to "attend" automatically.)

- [ ] T007 [US3] Add unit test in `cc-deck/internal/voice/stopword_test.go` verifying multiple words for "attend" action all trigger correctly

**Checkpoint**: Multiple trigger words work.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation.

- [ ] T008 [P] Update configuration reference in `docs/modules/reference/pages/configuration.adoc` to document the "attend" command word and how to customize it
- [ ] T009 [P] Update voice relay guide in docs to mention the "next" command alongside "send"
- [ ] T010 Run `make test` and `make lint` to verify all changes pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies, can start immediately. All T001-T004 are parallelizable (different files).
- **User Story 1 (Phase 3)**: Depends on T001 and T002 (Go side must produce `[[attend]]` before plugin can handle it)
- **User Story 2 (Phase 4)**: Depends on T001 (DefaultCommands must exist for override testing)
- **User Story 3 (Phase 5)**: Depends on T001 (DefaultCommands must exist)
- **Polish (Phase 6)**: Depends on all user stories being complete

### Parallel Opportunities

- T001, T002, T003, T004 can all run in parallel (different files)
- T006, T007 can run in parallel (both append tests to same file, but independent test functions)
- T008, T009 can run in parallel (different doc files)

---

## Parallel Example: Foundational Phase

```bash
# All four foundational tasks touch different files and can run simultaneously:
Task: "Add attend to DefaultCommands in stopword.go"
Task: "Add attend case to relay dispatch in relay.go"
Task: "Add stopword tests in stopword_test.go"
Task: "Add relay dispatch test in relay_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (Go side)
2. Complete Phase 3: User Story 1 (Rust plugin handler)
3. **STOP and VALIDATE**: Say "next" and verify session cycling works
4. Feature is usable with default "next" trigger word

### Incremental Delivery

1. Foundational → Go side produces `[[attend]]` payloads
2. User Story 1 → Plugin handles `[[attend]]`, core feature works (MVP)
3. User Story 2 → Custom words verified via tests (already works architecturally)
4. User Story 3 → Multiple words verified via tests (already works architecturally)
5. Polish → Documentation updated

---

## Notes

- Total tasks: 10
- Tasks per user story: US1=1, US2=1, US3=1, Foundational=4, Polish=3
- Parallel opportunities: T001-T004 (all parallel), T008-T009 (parallel)
- US2 and US3 require no production code changes, only test verification
- MVP scope: Phases 2+3 (5 tasks: T001-T005)
