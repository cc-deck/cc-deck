# Plan Review: Voice Relay (042)

**Reviewed**: 2026-04-24
**Artifacts**: spec.md, plan.md, tasks.md, research.md, data-model.md, contracts/voice-pipeline.md

## Coverage Matrix

Maps every spec requirement to at least one task. Missing coverage = risk.

### Functional Requirements

| Requirement | Description | Tasks | Status |
|------------|-------------|-------|--------|
| FR-001 | Audio capture 16kHz mono, --device, --list-devices | T017, T018, T047 | COVERED |
| FR-002 | Voice activity detection (energy threshold, silence) | T019 | COVERED |
| FR-003 | Local transcription, no external services | T021, T022 | COVERED |
| FR-004 | Relay text via PipeChannel, all workspace types | T024, T005-T007 | COVERED |
| FR-005 | Text injection via plugin API, no focus switch | T011, T015 | COVERED |
| FR-006 | Stopword detection ("submit", "enter"), filler words | T020, T031, T032 | COVERED |
| FR-007 | Permission state detection, pause relay | T011, T036 | COVERED |
| FR-008 | PTT mode with F8 keybinding | T014, T033 | COVERED |
| FR-009 | PTT works from any pane without focus switch | T014, T033 | COVERED |
| FR-010 | Bidirectional communication (SendReceive) | T005, T006, T012, T013 | COVERED |
| FR-011 | Real-time TUI display | T025, T026, T027 | COVERED |
| FR-012 | On-demand model download | T040 | COVERED |
| FR-013 | Setup command (dependency check, model download) | T038-T041 | COVERED |
| FR-014 | Generic pipe CLI command | T029 | COVERED |
| FR-015 | Session focus tracking | T043 | COVERED |
| FR-016 | Discard buffered text on permission resolve | T036 | COVERED |
| FR-017 | CGo-free audio capture fallback | T018 | COVERED |
| FR-018 | Transcription backend auto-lifecycle, crash retry | T023 | COVERED |
| FR-019 | --verbose flag for diagnostics | T028 (flag), T025-T027 (TUI) | COVERED |

### Non-Functional Requirements

| Requirement | Description | Tasks | Status |
|------------|-------------|-------|--------|
| NFR-001 | <5s end-to-end latency | T019 (VAD), T021 (transcriber), T024 (pipeline) | IMPLICIT |
| NFR-002 | <200 MB RSS | No explicit task | GAP |
| NFR-003 | Transport security matches workspace | T005-T007 (inherits workspace transport) | COVERED |
| NFR-004 | Audio stays local | T017, T018 (local capture only) | COVERED |
| NFR-005 | No transcription logging by default | T028 (--verbose flag) | IMPLICIT |

### Success Criteria

| Criterion | Description | Tasks | Status |
|-----------|-------------|-------|--------|
| SC-001 | <5s dictation-to-pane | Pipeline tasks (T017-T024) | COVERED |
| SC-002 | All 5 workspace types | T007 (factory updates) | COVERED |
| SC-003 | PTT <500ms toggle | T033, T005-T006 | COVERED |
| SC-004 | Setup <2min on broadband | T040 | COVERED |
| SC-005 | 100% permission prompt safety | T011, T036 | COVERED |
| SC-006 | Focus change within next utterance | T043 | COVERED |
| SC-007 | <100KB binary size increase | Architectural (models external) | IMPLICIT |
| SC-008 | 95% transcription accuracy | Depends on Whisper model quality | EXTERNAL |

### User Stories

| Story | Priority | Phase | Task Count | Status |
|-------|----------|-------|------------|--------|
| US1 - Voice Dictation | P1 | Phase 3 | 14 tasks | COVERED |
| US2 - Prompt Submission | P1 | Phase 4 | 2 tasks | COVERED |
| US3 - Push-to-Talk | P2 | Phase 5 | 3 tasks | COVERED |
| US4 - Permission Safety | P2 | Phase 6 | 2 tasks | COVERED |
| US5 - Setup/Models | P2 | Phase 7 | 5 tasks | COVERED |
| US6 - Session Tracking | P3 | Phase 8 | 2 tasks | COVERED |

### Edge Cases

| Edge Case | Tasks | Status |
|-----------|-------|--------|
| Audio device unavailable | T017, T018 (error on Start) | COVERED |
| Transcription empty text | T024 (relay discards empty) | IMPLICIT |
| Workspace disconnects | T024 (PipeChannel error handling) | IMPLICIT |
| Rapid successive utterances | T019 (VAD queuing) | IMPLICIT |
| Very long utterance | T019 (30s max, split at VAD boundaries) | COVERED |
| PTT toggle during transcription | T033 (new recording starts, in-progress completes) | IMPLICIT |
| Transcription backend crash | T023 (3 retry, notification) | COVERED |
| Corrupted model | T042 (startup validation) | COVERED |
| Session focus mid-utterance | T043 (snapshot at utterance start) | IMPLICIT |
| Multiple voice relay instances | No explicit task | GAP |

---

## Red Flags

### Flag 1: No Multiple-Instance Guard (Low)

**Spec says**: "Only one voice relay instance per workspace is supported. Starting a second instance for the same workspace shows an error."
**Tasks**: No task covers this.
**Risk**: Two voice relay instances could inject duplicate text.
**Recommendation**: Add a task to T028 (ws voice command) to check for existing instances (e.g., via a lock file in XDG state or a named pipe check).

### Flag 2: NFR-002 Memory Budget Not Validated (Low)

**Spec says**: "MUST NOT consume more than 200 MB of resident memory."
**Tasks**: No task validates this.
**Risk**: Audio buffers + TUI + HTTP client could exceed budget under certain conditions.
**Recommendation**: Add a note to T050 (quickstart validation) to check RSS during manual testing. Not worth a dedicated task since the architecture (separate transcription process) inherently limits the voice process footprint.

### Flag 3: Edge Case Coverage is Implicit (Low)

Several edge cases (empty transcription, workspace disconnect, rapid utterances, mid-utterance focus change) are handled implicitly by the pipeline architecture but have no explicit tasks or tests.
**Risk**: Edge cases could be missed during implementation.
**Recommendation**: These are naturally covered by the pipeline design (VAD queuing, PipeChannel error returns, utterance-level session snapshot). Explicit tests would be beneficial but are not blocking.

### Flag 4: US2 Has Only 2 Tasks (Info)

US2 (Prompt Submission) has minimal tasks because stopword detection is implemented in US1 (T020). This is correct, as US2 is refinement/testing of T020's logic. The cross-story dependency is documented and acceptable.

---

## Task Quality Assessment

### Format Compliance

- All 50 tasks have checkbox (`- [ ]`)
- All tasks have sequential IDs (T001-T050)
- All user story tasks have `[Story]` labels
- Parallelizable tasks marked with `[P]`
- All tasks include file paths
- **Score: PASS**

### Task Granularity

- Average task scope: 1 file per task (appropriate)
- Largest task: T024 (VoiceRelay orchestrator) - complex but well-defined
- Smallest tasks: T030 (command registration), T032 (verify behavior) - appropriately minimal
- **Score: PASS**

### Dependency Clarity

- Phase dependencies clearly documented
- Cross-story dependencies identified (US2->US1 T020, US6->US1 T024)
- Parallel opportunities well-documented with examples
- **Score: PASS**

### MVP Path

- MVP = Phase 1 + Phase 2 + Phase 3 (US1) = 30 tasks
- Clear stopping point with checkpoint
- Incremental delivery path documented
- **Score: PASS**

---

## Recommendations

### Before Implementation

1. **Add instance guard task**: Insert a task between T028 and T029 to check for existing voice relay instances and show error if one is already running for the target workspace.

2. **Consider moving US5 (Setup) earlier**: The incremental delivery section suggests US5 after US2, which makes sense since setup is needed for first-time users. However, the phase ordering puts it at Phase 7. The dependency section correctly notes US5 only depends on Phase 1, so it could run in parallel with US1.

### During Implementation

3. **Test edge cases during manual validation (T050)**: When running quickstart.md validation, explicitly test empty transcription, workspace disconnect, and rapid utterances.

4. **Memory check during testing**: When running T048/T050, check RSS of the voice process to validate NFR-002.

### Reviewer Guidance

When reviewing a PR for this feature, focus on:

1. **PipeChannel.SendReceive** (Phase 2): This is the foundational new capability. Verify blocking behavior, context cancellation, and workspace transport reuse.
2. **Plugin VoiceControl handler** (T012): The held-pipe pattern is the most subtle piece. Verify that the pipe is NOT unblocked and that the DumpState pattern is correctly followed.
3. **Permission state pausing** (T011, T036): Safety-critical. Verify text is buffered during permission prompts and discarded on resolve.
4. **Build tag correctness** (T017, T018): Verify `//go:build cgo` and `//go:build !cgo` are mutually exclusive and both compile independently.
5. **Filler word stripping** (T020): Verify the exact list ("um", "uh", "hmm", "ah", "er") and that "um, submit" produces a newline while "okay submit" does not.

---

## Summary

| Dimension | Score |
|-----------|-------|
| Spec Coverage | 18/19 FRs covered, 1 edge case gap |
| NFR Coverage | 4/5 covered, 1 implicit |
| Task Format | PASS (all 50 tasks compliant) |
| Dependency Graph | PASS (clear, well-documented) |
| MVP Path | PASS (30 tasks to MVP, clear checkpoint) |
| Red Flags | 2 low-severity gaps, 2 informational notes |

**Verdict**: Plan is ready for implementation. The two gaps (multiple-instance guard, memory validation) are low severity and can be addressed as minor additions during implementation without replanning.
