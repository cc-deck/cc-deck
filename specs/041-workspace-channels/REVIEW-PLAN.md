# Plan Review: Workspace Channels (041)

**Reviewed**: 2026-04-22
**Artifacts**: spec.md, plan.md, tasks.md, research.md, data-model.md, contracts/channel-interfaces.md

## Coverage Matrix

### Functional Requirements → Tasks

| Requirement | Description | Tasks | Status |
|-------------|-------------|-------|--------|
| FR-001 | PipeChannel with request-response pattern | T002, T015-T016, T017-T021 | COVERED (SendReceive stub for now) |
| FR-002 | DataChannel Push/Pull | T002, T024-T027, T029-T033 | COVERED |
| FR-003 | DataChannel PushBytes | T028 | COVERED |
| FR-004 | GitChannel interface | T002, T042-T044 | COVERED |
| FR-005 | All workspace types supported | T007-T011, T017-T021, T029-T033, T045-T049 | GAP: k8s-sandbox |
| FR-006 | Push/Pull delegate to DataChannel | T034-T038 | COVERED |
| FR-007 | Harvest delegates to GitChannel | T050-T053 | COVERED |
| FR-008 | Available when first used (lazy init) | Plan describes sync.Once | COVERED (implicit) |
| FR-009 | ChannelError structured type | T004, T005 | COVERED |
| FR-010 | CLI verbose error display | T058 | COVERED |
| FR-011 | Native transport per type | T024-T027, T042-T044 | COVERED |
| FR-012 | GitChannel complete round-trip | T003, T042-T044 | COVERED |

### Success Criteria → Tasks

| Criterion | Description | Tasks | Status |
|-----------|-------------|-------|--------|
| SC-001 | PipeChannel latency targets | T022 (unit tests) | WEAK: no performance measurement |
| SC-002 | DataChannel speed parity | T040 | COVERED |
| SC-003 | GitChannel identical results | T057 | COVERED |
| SC-004 | One location per channel type | Architectural (code structure) | COVERED |
| SC-005 | Identical CLI behavior | T040, T057 | COVERED |
| SC-006 | Human-readable errors | T004, T058 | COVERED |

### User Stories → Phases

| User Story | Priority | Phase | Tasks | Independent? |
|-----------|----------|-------|-------|-------------|
| US1: PipeChannel | P1 | Phase 3 | T015-T023 (9 tasks) | Yes |
| US2: DataChannel | P2 | Phase 4 | T024-T040 (17 tasks) | Yes |
| US3: GitChannel | P3 | Phase 5 | T041-T057 (17 tasks) | Yes |

### Edge Cases → Coverage

| Edge Case | Resolution | Covered By |
|-----------|-----------|------------|
| Workspace restart mid-transfer | Error on in-flight, transparent reconnect after | Stateless channel design |
| Concurrent operations | Independent per-call, undefined ordering | contracts/channel-interfaces.md |
| Pipe name not found | ChannelError | FR-009 / T004 |
| Large file transfers | Inherit from existing transport | DataChannel implementations |
| Remote path issues | ChannelError with path context | FR-009 / T004 |
| Git merge conflicts | Fail and report conflict | contracts/channel-interfaces.md |

## Red Flags

### FLAG-1: k8s-sandbox has no workspace implementation (MEDIUM)

FR-005 lists k8s-sandbox as a required workspace type for channel support. However, research confirmed that k8s-sandbox has no factory implementation (no `case` in `factory.go`). The type is defined in `types.go` but creating a k8s-sandbox workspace returns `ErrNotImplemented`.

**Impact**: Channel implementations cannot be tested for k8s-sandbox since no workspace of that type can be created.

**Recommendation**: Either remove k8s-sandbox from FR-005 scope (note as "deferred until workspace type is implemented") or add a prerequisite task to implement the k8s-sandbox workspace factory case. Since k8s-sandbox channels would be identical to k8s-deploy channels (same kubectl exec transport), no additional channel code is needed once the workspace type exists.

### FLAG-2: No plugin-side task for pipe dispatch (LOW)

The plan notes `pipe_handler.rs` may need changes for new channel pipe names, but no task covers plugin-side modifications. If PipeChannel sends to pipe names the plugin does not recognize, messages are silently dropped (`PipeAction::Unknown`).

**Impact**: PipeChannel may appear to work but messages are discarded by the plugin.

**Recommendation**: Either (a) add a task to register new pipe names in `pipe_handler.rs`, or (b) document that PipeChannel consumers must use existing registered pipe names (e.g., `cc-deck:hook`). Option (b) is simpler since the plugin already handles unknown pipes by unblocking the CLI pipe, and consumer features can register their own pipe names when they are implemented.

### FLAG-3: Parallel tasks targeting same file (LOW)

Several [P]-marked task groups target the same file:
- T015 + T016 both write to `channel_pipe.go`
- T024-T027 all write to `channel_data.go`
- T042-T044 all write to `channel_git.go`

**Impact**: If actually executed in parallel by separate agents, merge conflicts are possible.

**Recommendation**: These are fine for sequential execution within a single agent. If using parallel agents, split by file or have one agent create the file skeleton first. This is an execution detail, not a plan flaw.

### FLAG-4: No concurrency tests (LOW)

The spec and contracts state channels must be safe for concurrent use (PipeChannel, DataChannel) and serialized for GitChannel. No task explicitly tests concurrent access.

**Impact**: Concurrency bugs may not be caught by unit tests.

**Recommendation**: Add a test case in the relevant test file that calls Send/Push from multiple goroutines to verify no data races. This can be covered by running `go test -race` which `make test` likely already does.

## Task Quality Assessment

### Format Compliance

- All 65 tasks follow the `- [ ] [ID] [P?] [Story?] Description with file path` format: **PASS**
- Sequential task IDs (T001-T065): **PASS**
- Story labels present for US phases only: **PASS**
- File paths in all implementation tasks: **PASS**
- Checkpoints at phase boundaries: **PASS**

### Task Specificity

- All tasks reference specific files: **PASS**
- Tasks are small enough for single-agent execution: **PASS**
- Dependencies are clear (implementations before wiring before refactoring): **PASS**

### Concerns

- T013 covers 5 files in one task (stub accessors for all workspace types). This is acceptable since stubs are trivial (return ErrNotSupported).
- T028 covers PushBytes for all implementations in one task. Could be split per transport type for parallel execution, but is acceptable since PushBytes is a simple method.

## Constitution Alignment

| Principle | Plan Status | Notes |
|-----------|------------|-------|
| VII. Interface Behavioral Contracts | PASS | contracts/channel-interfaces.md exists with full behavioral specs |
| VIII. Simplicity | PASS | Transport grouping is minimal; no framework, no generic dispatcher |
| IX. Documentation Freshness | PENDING | T061-T063 cover documentation updates |
| XII. Prose Plugin | PENDING | T061-T063 should use prose plugin (noted in plan Phase 6) |

## Verdict

**APPROVED with notes**. The plan is well-structured with clear phases, good parallel opportunities, and comprehensive coverage of spec requirements. The three flagged issues are manageable:

1. **k8s-sandbox (FLAG-1)**: Add a note to FR-005 that k8s-sandbox channels are deferred until the workspace type is implemented. The channel code (k8s transport group) will work for k8s-sandbox without changes once the factory case exists.
2. **Plugin dispatch (FLAG-2)**: Document that PipeChannel consumers use existing registered pipe names. New names are registered by the consuming feature, not by the channel layer.
3. **Parallel file conflicts (FLAG-3)** and **concurrency tests (FLAG-4)**: Execution-level details, not plan flaws.

## Recommended Next Steps

1. Address FLAG-1 by updating FR-005 in spec.md (note k8s-sandbox deferral)
2. Commit all spec artifacts to the feature branch
3. Ask user before creating a spec PR
4. Clear context before implementation (`/clear`)
