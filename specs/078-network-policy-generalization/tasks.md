# Tasks: Network Policy Generalization

**Input**: Design documents from `specs/078-network-policy-generalization/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add the Agent interface method and manifest field that all stories depend on

- [x] T001 Add `RequiredDomainGroups() []string` method to the `Agent` interface in `cc-deck/internal/agent/agent.go`
- [x] T002 [P] Implement `RequiredDomainGroups()` on `ClaudeAgent` returning `["anthropic"]` in `cc-deck/internal/agent/claude.go`
- [x] T003 [P] Implement `RequiredDomainGroups()` on `OpenCodeAgent` returning `["openai"]` in `cc-deck/internal/agent/opencode.go`
- [x] T004 Add `"openai"` builtin domain group with `api.openai.com`, `.openai.com`, `.oaiusercontent.com` in `cc-deck/internal/network/builtin.go`
- [x] T005 Add `Agents []string` field to the `Manifest` struct in `cc-deck/internal/build/manifest.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend the match condition system with agent-based matching

**Warning**: No user story work can begin until this phase is complete

- [x] T006 Add `Agents []string` field to `MatchCondition` struct in `cc-deck/internal/build/component.go`
- [x] T007 Update `MatchComponent()` in `cc-deck/internal/build/component.go` to check `comp.Match.Agents` against `manifest.Agents` (with default `["claude"]` when manifest agents is empty)
  - **Interfaces**: Consumes `MatchCondition.Agents []string` (T006), `Manifest.Agents []string` (T005). Signature: `func MatchComponent(comp *PolicyComponent, manifest *Manifest) bool`.
- [x] T008 Update `ValidateComponent()` in `cc-deck/internal/build/component.go` to accept `Agents` as a valid match field
  - **Interfaces**: Consumes `MatchCondition.Agents []string` (T006). Update the validation check at line 90 to include `len(comp.Match.Agents) == 0` in the condition.

**Checkpoint**: Match condition system now supports agent-based matching

---

## Phase 3: User Story 1 - Agent-Aware Policy Composition (Priority: P1)

**Goal**: Policy assembly includes domain entries based on which agents are in the manifest

**Independent Test**: Build a policy with two agents declared, verify each agent's domains appear and no extraneous domains are present

- [x] T009 [US1] Change `cc-deck/internal/build/policies/claude-code.yaml` from `match: always: true` to `match: agents: [claude]`
- [x] T010 [P] [US1] Create `cc-deck/internal/build/policies/opencode.yaml` policy component with `match: agents: [opencode]`, OpenAI API endpoints, and probe/runtime binaries for the opencode binary
- [x] T011 [US1] Add unit tests for single-agent policy assembly (Claude only, no agents field) verifying backward-compatible output in `cc-deck/internal/build/policy_test.go`
- [x] T012 [US1] Add unit tests for multi-agent policy assembly (Claude + OpenCode) verifying both agents' domains appear in `cc-deck/internal/build/policy_test.go`
- [x] T013 [US1] Add unit test for empty manifest agents defaulting to Claude behavior in `cc-deck/internal/build/policy_test.go`
- [x] T013a [US1] Add unit test verifying that when two agents share a domain group (e.g., both declare endpoints for a common domain), the policy output contains no duplicate endpoints in `cc-deck/internal/build/policy_test.go`

**Checkpoint**: Policy assembly is agent-aware. Builds with Claude-only produce identical output.

---

## Phase 4: User Story 2 - Agent Domain Group Declaration (Priority: P1)

**Goal**: Agent adapters declare domain groups, build system resolves them

- [x] T014 [US2] Add unit tests for `ClaudeAgent.RequiredDomainGroups()` verifying it returns `["anthropic"]` in `cc-deck/internal/agent/claude_test.go`
- [x] T015 [P] [US2] Add unit tests for `OpenCodeAgent.RequiredDomainGroups()` verifying it returns `["openai"]` in `cc-deck/internal/agent/opencode_test.go`
- [x] T016 [US2] Add unit test verifying a warning is emitted when an agent declares a domain group not present in builtin groups in `cc-deck/internal/build/policy_test.go`
- [x] T017 [US2] Add unit test for `MatchComponent()` with `Agents` field matching in `cc-deck/internal/build/component_test.go`

**Checkpoint**: Agent domain group declarations are tested and validated

---

## Phase 5: User Story 3 - Removal of Hardcoded Claude Code Coupling (Priority: P2)

**Goal**: No agent-specific hardcoding remains in the policy assembly code

- [x] T018 [US3] Replace the `if comp.Key == "claude_code"` block in `cc-deck/internal/build/policy.go` (MCP binary lookup) with a generic loop that collects binaries from all agent-matched components
  - **Interfaces**: Consumes `MatchCondition.Agents []string` (T006) to identify agent-matched components. Iterates `matched []PolicyComponent` and collects `comp.Binaries` from components where `comp.Match.Agents` is non-empty.
- [x] T019 [US3] Update the `pkg_node` binary augmentation logic in `cc-deck/internal/build/policy.go` to use agent-matched component binaries instead of `claudeCodeBinaries`
- [x] T020 [US3] Update the MCP warning message in `cc-deck/internal/build/policy.go` from "claude_code component not found" to a generic "no agent component found" message
- [x] T021 [US3] Add unit test verifying MCP endpoint processing works with OpenCode agent binaries (not just Claude) in `cc-deck/internal/build/policy_test.go`
- [x] T022 [US3] Add unit test verifying multi-agent MCP binary merging (both Claude and OpenCode binaries appear) in `cc-deck/internal/build/policy_test.go`

**Checkpoint**: Zero hardcoded `claude_code` references remain in `policy.go`

---

## Phase 6: User Story 4 - Per-Agent Domain Group Configuration (Priority: P3)

**Goal**: Users can associate additional domain groups with specific agents in workspace config

- [x] T023 [US4] Extend `NetworkConfig` in `cc-deck/internal/build/manifest.go` to support per-agent domain group entries (e.g., `allowed_domains_per_agent: {claude: [custom-group]}`)
- [x] T024 [US4] Update policy assembly in `cc-deck/internal/build/policy.go` to merge per-agent domain groups from manifest with agent-declared groups, filtering by manifest agents list. This task is where `RequiredDomainGroups()` (from T001-T003) is consumed: look up each agent in the registry, call `RequiredDomainGroups()`, and combine with per-agent user groups before resolving endpoints via `network.Resolver.ExpandGroup()`.
- [x] T025 [US4] Add unit test for per-agent domain group inclusion when agent is in manifest in `cc-deck/internal/build/policy_test.go`
- [x] T026 [US4] Add unit test for per-agent domain group exclusion when agent is NOT in manifest in `cc-deck/internal/build/policy_test.go`

**Checkpoint**: Per-agent domain configuration works for power users

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and cleanup

- [x] T027 [P] Update `./README.md` (project root) with multi-agent manifest `agents` field documentation
- [x] T028 [P] Update CLI reference in `docs/modules/reference/pages/configuration.adoc` with `agents` field and per-agent domain group syntax
- [x] T029 Run `make verify` to confirm all tests pass and linting is clean
- [x] T030 Verify backward compatibility: build a policy with an existing manifest (no `agents` field) and diff against the pre-change output

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on T001 (interface change) and T005 (manifest field)
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 1 completion (can run parallel with US1 after Phase 2)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (needs agent-matched components to exist)
- **User Story 4 (Phase 6)**: Depends on Phase 3 (needs agent-aware assembly)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational - No dependencies on other stories
- **US2 (P1)**: Depends on Setup - Can run parallel with US1
- **US3 (P2)**: Depends on US1 (needs the agent-matched components from Phase 3)
- **US4 (P3)**: Depends on US1 (needs agent-aware assembly from Phase 3)

### Within Each User Story

- Policy YAML changes before test additions
- Test additions before or alongside implementation
- Implementation before integration tests

### Parallel Opportunities

- T002 and T003 can run in parallel (different agent files)
- T010 can run in parallel with T009 (different policy YAML files)
- T014 and T015 can run in parallel (different test files)
- T027 and T028 can run in parallel (different doc files)
- US1 and US2 can run in parallel after Phase 2

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (interface + manifest changes)
2. Complete Phase 2: Foundational (match condition extension)
3. Complete Phase 3: US1 - Agent-Aware Policy Composition
4. Complete Phase 4: US2 - Agent Domain Group Declaration
5. **STOP and VALIDATE**: Test with Claude-only and Claude+OpenCode manifests
6. Verify backward compatibility

### Incremental Delivery

1. Setup + Foundational → Core infrastructure ready
2. Add US1 → Agent-aware policy composition works (MVP)
3. Add US2 → Domain group declarations validated
4. Add US3 → Hardcoded coupling removed (clean architecture)
5. Add US4 → Per-agent configuration for power users
6. Polish → Documentation and final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Backward compatibility is the primary constraint: SC-001 requires byte-identical output for Claude-only builds
- The `vertex-ai.yaml` component is intentionally NOT changed (stays credential-matched per research.md R5)
- Shared ecosystem components (git-hosting, go, python, node, rust) retain their existing match conditions
