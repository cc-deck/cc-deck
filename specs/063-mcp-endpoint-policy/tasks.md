# Tasks: MCP Endpoint Policy Integration

**Input**: Design documents from `/specs/063-mcp-endpoint-policy/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Add the Endpoint field to the manifest schema and create helper functions used by all user stories

- [ ] T001 Add `Endpoint string` field with `yaml:"endpoint,omitempty"` tag to `MCPEntry` struct in cc-deck/internal/build/manifest.go
- [ ] T002 [P] Add `slugifyMCPName()` function in cc-deck/internal/build/policy.go that replaces hyphens, spaces, and non-alphanumeric characters with underscores and lowercases the result
- [ ] T003 [P] Add `parseMCPEndpoint()` function in cc-deck/internal/build/policy.go that splits a `host:port` string, validates both parts, and returns (host string, port int, error)
- [ ] T004 [P] Add unit tests for `slugifyMCPName()` in cc-deck/internal/build/policy_test.go: test hyphen replacement (`google-work` -> `google_work`), spaces, mixed case, non-alphanumeric characters
- [ ] T005 [P] Add unit tests for `parseMCPEndpoint()` in cc-deck/internal/build/policy_test.go: valid `host:port`, missing port (error), malformed string (error), port as non-number (error)

**Checkpoint**: Helper functions ready and tested. `make test` passes.

---

## Phase 2: User Story 1 - Policy Assembly Includes MCP Endpoints (Priority: P1)

**Goal**: Generate network policy entries for MCP servers with endpoints during policy assembly

**Independent Test**: Assemble a policy from a manifest with MCP entries (with and without endpoints) and verify correct policy output

### Implementation for User Story 1

- [ ] T006 [US1] Add MCP endpoint processing block in `assemblePolicyCore()` in cc-deck/internal/build/policy.go: after the credentials block (~line 242), iterate `manifest.MCP` entries with non-empty `Endpoint`, parse endpoint, look up `claude_code` component binaries from `matched` slice, generate `NetworkPolicy` keyed as `mcp_<slugifyMCPName(name)>` with description as name (fallback to name field)
- [ ] T007 [P] [US1] Add test `TestAssemblePolicy_MCPEndpointGeneratesPolicy` in cc-deck/internal/build/policy_test.go: manifest with MCP entry having endpoint `mcp-google-work.int-tichny.org:8443`, verify policy has key `mcp_google_work` with correct host, port, and claude_code binaries
- [ ] T008 [P] [US1] Add test `TestAssemblePolicy_MCPWithoutEndpointSkipped` in cc-deck/internal/build/policy_test.go: manifest with MCP entry having empty endpoint (like `playwright`), verify no MCP policy entry generated
- [ ] T009 [P] [US1] Add test `TestAssemblePolicy_MCPMultipleEntries` in cc-deck/internal/build/policy_test.go: manifest with 3 MCP entries (2 with endpoints, 1 without), verify exactly 2 MCP policy entries with correct keys
- [ ] T010 [P] [US1] Add test `TestAssemblePolicy_MCPMalformedEndpointSkipped` in cc-deck/internal/build/policy_test.go: manifest with MCP entry having malformed endpoint (no port), verify entry skipped without error (warning logged)
- [ ] T011 [P] [US1] Add test `TestAssemblePolicy_MCPDeterminismWithMCP` in cc-deck/internal/build/policy_test.go: manifest with MCP entries produces byte-identical output across multiple runs
- [ ] T012 [US1] Run `make test` and `make lint` to verify all tests pass with no regressions

**Checkpoint**: MCP policy entries are generated correctly. US1 is independently testable.

---

## Phase 3: User Story 3 - Node Binary Augmentation (Priority: P3)

**Goal**: Append Claude Code binaries to `pkg_node` component when MCP entries exist

**Independent Test**: Assemble a policy where `pkg_node` is matched (node tool in manifest) and MCP entries exist, verify `pkg_node` binary list includes Claude Code paths

### Implementation for User Story 3

- [ ] T013 [US3] Add `pkg_node` binary augmentation in `assemblePolicyCore()` in cc-deck/internal/build/policy.go: after MCP processing, if `pkg_node` exists in `networkPolicies` and manifest has MCP entries with endpoints, append `claude_code` binaries to `pkg_node`'s binary list with path deduplication
- [ ] T014 [P] [US3] Add test `TestAssemblePolicy_PkgNodeAugmentedWithMCP` in cc-deck/internal/build/policy_test.go: manifest with node tool and MCP entries with endpoints, verify `pkg_node` binaries include claude_code paths
- [ ] T015 [P] [US3] Add test `TestAssemblePolicy_PkgNodeNotAugmentedWithoutMCP` in cc-deck/internal/build/policy_test.go: manifest with node tool but no MCP entries, verify `pkg_node` binaries unchanged
- [ ] T016 [P] [US3] Add test `TestAssemblePolicy_NoPkgNodeNoAugmentation` in cc-deck/internal/build/policy_test.go: manifest with MCP entries but no node tool, verify no `pkg_node` key and no error
- [ ] T017 [US3] Run `make test` and `make lint` to verify all tests pass

**Checkpoint**: pkg_node augmentation works correctly. US3 is independently testable.

---

## Phase 4: User Story 2 - Capture Command Extracts MCP Endpoints (Priority: P2)

**Goal**: Extend the capture command to extract and present MCP endpoint URLs for user confirmation

**Independent Test**: Run capture against Claude Code settings with HTTP and stdio MCP servers, verify endpoints extracted and written to manifest

### Implementation for User Story 2

- [ ] T018 [US2] Extend Step 9 in cc-deck/internal/build/commands/cc-deck.capture.md: after discovering MCP servers, extract endpoint from HTTP/SSE servers by parsing the `url` field (extract host:port), from stdio servers with `mcp-remote` by scanning `args` for HTTPS URLs, present extracted endpoints alongside server info for user confirmation, write confirmed endpoints to manifest `mcp[].endpoint` field

**Checkpoint**: Capture command extracts and writes MCP endpoints. US2 is independently testable.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [ ] T019 [P] Document the `endpoint` field in docs/modules/reference/pages/configuration.adoc: add endpoint field description in the MCP entry section, include example showing MCP entries with and without endpoints, use prose plugin with cc-deck voice profile
- [ ] T020 [P] Update README.md with MCP endpoint policy feature description
- [ ] T021 Run full validation: `make test` and `make lint`, verify backward compatibility by assembling policy from a manifest with no MCP endpoint fields

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies, can start immediately
- **US1 (Phase 2)**: Depends on Phase 1 (needs Endpoint field and helper functions)
- **US3 (Phase 3)**: Depends on Phase 2 (needs MCP processing loop in assemblePolicyCore)
- **US2 (Phase 4)**: Depends on Phase 1 (needs Endpoint field in MCPEntry), independent of US1/US3
- **Polish (Phase 5)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Depends on foundational helpers (T001-T005)
- **User Story 3 (P3)**: Depends on US1 (T006) since augmentation happens after MCP processing
- **User Story 2 (P2)**: Independent of US1/US3, only depends on Endpoint field (T001)

### Parallel Opportunities

- T002, T003, T004, T005 can all run in parallel (different functions, no file conflicts between T002/T003)
- T007, T008, T009, T010, T011 can all run in parallel (independent test functions)
- T014, T015, T016 can all run in parallel (independent test functions)
- T019, T020 can run in parallel (different files)

---

## Parallel Example: Phase 1

```bash
# After T001 completes, launch helpers in parallel:
Task: "Add slugifyMCPName() in cc-deck/internal/build/policy.go"
Task: "Add parseMCPEndpoint() in cc-deck/internal/build/policy.go"
Task: "Test slugifyMCPName() in cc-deck/internal/build/policy_test.go"
Task: "Test parseMCPEndpoint() in cc-deck/internal/build/policy_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (T001-T005)
2. Complete Phase 2: User Story 1 (T006-T012)
3. **STOP and VALIDATE**: `make test` passes, policy output includes MCP entries
4. This alone unblocks the primary problem (MCP connections denied)

### Incremental Delivery

1. Foundational -> US1 -> Validate (MVP: MCP endpoints in policy)
2. Add US3 -> Validate (npm-based MCP servers can install packages)
3. Add US2 -> Validate (capture command extracts endpoints automatically)
4. Polish -> Documentation complete

---

## Notes

- [P] tasks = different files or independent test functions, no dependencies
- [Story] label maps task to specific user story for traceability
- T002 and T003 both modify policy.go but add independent functions (no conflict)
- Test tasks (T004, T005, T007-T011, T014-T016) all add independent test functions to policy_test.go
- The capture command (T018) is a Markdown file, not Go code, so it has no compile-time dependency on the Go changes
