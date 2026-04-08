# Tasks: Unified Setup Command

**Input**: Design documents from `/specs/034-unified-setup-command/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Test tasks included for foundational components (manifest, probe). Integration testing is explicit.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **CLI (Go)**: `cc-deck/internal/`, `cc-deck/cmd/cc-deck/`
- **Claude Commands**: `cc-deck/internal/setup/commands/`
- **Specs**: `specs/034-unified-setup-command/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package rename and CLI restructuring. No functional changes.

- [ ] T001 Rename `cc-deck/internal/build/` to `cc-deck/internal/setup/` and update all import paths
- [ ] T002 Rename `cc-deck/internal/cmd/build.go` to `cc-deck/internal/cmd/setup.go` and change cobra command from `image` to `setup` in `cc-deck/cmd/cc-deck/main.go`
- [ ] T003 Delete `cc-deck/internal/setup/commands/cc-deck.push.md` and remove embed reference in `cc-deck/internal/setup/embed.go`

**Checkpoint**: `make test` and `make lint` pass. `cc-deck setup init` works (same as old `cc-deck image init`). `cc-deck image` no longer exists.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Manifest evolution and template updates that ALL user stories depend on.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Evolve `Manifest` struct in `cc-deck/internal/setup/manifest.go`: replace `Image ImageConfig` with `Targets *TargetsConfig`, add `ContainerTarget` and `SSHTarget` structs per `data-model.md`
- [ ] T005 Update `Validate()`, `ImageRef()`, `BaseImage()` in `cc-deck/internal/setup/manifest.go` to read from `Targets.Container`
- [ ] T006 Rename manifest filename constant from `cc-deck-image.yaml` to `cc-deck-setup.yaml` in `cc-deck/internal/setup/manifest.go`
- [ ] T007 Update manifest template in `cc-deck/internal/setup/init.go` to include both `targets.container` and `targets.ssh` sections (commented out by default)
- [ ] T008 Write unit tests for manifest loading, validation, and helper methods with new `Targets` struct in `cc-deck/internal/setup/manifest_test.go`

**Checkpoint**: Manifest v2 loads, validates, and passes all tests. `make test` passes.

---

## Phase 3: User Story 1 - Initialize a Setup Profile (Priority: P1) MVP

**Goal**: A developer can scaffold a setup directory with manifest template, Claude commands, and target-specific file structures.

**Independent Test**: Run `cc-deck setup init --target container`, `--target ssh`, and `--target container,ssh`. Verify directory structure, manifest template sections, and Claude command installation.

### Implementation for User Story 1

- [ ] T009 [US1] Add `--target` flag to init command accepting `container`, `ssh`, or comma-separated combination in `cc-deck/internal/cmd/setup.go`
- [ ] T010 [US1] Implement Ansible role skeleton scaffolding when `--target ssh`: create `roles/{base,tools,zellij,claude,cc_deck,shell_config,mcp}/tasks/main.yml` and `defaults/main.yml`, `group_vars/`, `site.yml` skeleton, `inventory.ini` template in `cc-deck/internal/setup/init.go`
- [ ] T011 [US1] Implement target section commenting/uncommenting logic: uncomment `targets.container` when `--target container`, uncomment `targets.ssh` when `--target ssh` in `cc-deck/internal/setup/init.go`
- [ ] T012 [US1] Update `/cc-deck.capture` command to read `cc-deck-setup.yaml` instead of `cc-deck-image.yaml`, ensure it only modifies shared sections in `cc-deck/internal/setup/commands/cc-deck.capture.md`

**Checkpoint**: `cc-deck setup init --target container,ssh` creates correct directory structure. `/cc-deck.capture` populates shared sections without touching targets. All 4 acceptance scenarios from US-1 pass.

---

## Phase 4: User Story 2 - Build a Container Image (Priority: P1)

**Goal**: A developer can build a container image from the unified manifest using `/cc-deck.build --target container`.

**Independent Test**: Initialize with `--target container`, populate manifest, run `/cc-deck.build --target container`, verify image builds with correct tools.

### Implementation for User Story 2

- [ ] T013 [US2] Update `/cc-deck.build` command to require `--target container` or `--target ssh` dispatch, read from `targets.container` instead of `image` in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T014 [US2] Add `--push` support using `targets.container.registry` field in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T015 [US2] Update Containerfile generation to handle existing file conflicts (show diff, ask user) in `cc-deck/internal/setup/commands/cc-deck.build.md`

**Checkpoint**: `/cc-deck.build --target container` generates Containerfile, builds image, and self-corrects. `--push` works with registry. All 4 acceptance scenarios from US-2 pass.

---

## Phase 5: User Story 3 - Provision SSH Remote via Ansible (Priority: P1)

**Goal**: A developer can provision a remote machine with Ansible playbooks generated from the manifest using `/cc-deck.build --target ssh`.

**Independent Test**: Initialize with `--target ssh`, populate manifest, run `/cc-deck.build --target ssh` against a real SSH target, verify tools installed.

### Implementation for User Story 3

- [ ] T016 [US3] Add `--target ssh` section to `/cc-deck.build` command: Ansible availability check (FR-015), inventory generation from `targets.ssh`, `group_vars/all.yml` from manifest in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T017 [US3] Add Ansible role generation instructions to `/cc-deck.build`: 7 roles (base, tools, zellij, claude, cc_deck, shell_config, mcp) with idempotent tasks in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T018 [US3] Add `create_user` handling in base role: sudo access, SSH authorized keys from `identity_file` `.pub` counterpart (FR-013) in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T019 [US3] Add credential sourcing snippet to shell_config role: `[ -f ~/.config/cc-deck/credentials.env ] && source ...` (FR-014) in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T020 [US3] Add `cc-deck plugin install` execution in cc_deck role (FR-016) and `ansible-playbook` execution with self-correction loop (FR-009) in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T021 [US3] Add role conflict handling: show diff of changed roles, ask user to choose (per clarification, mirrors US-2 AS-4) in `cc-deck/internal/setup/commands/cc-deck.build.md`
- [ ] T022 [P] [US3] Create lightweight probe function `Probe()` in `cc-deck/internal/ssh/probe.go` running `which zellij && which cc-deck && which claude` via SSH
- [ ] T023 [P] [US3] Write unit tests for probe with mock SSH client in `cc-deck/internal/ssh/probe_test.go`
- [ ] T024 [US3] Simplify `SSHEnvironment.Create()` in `cc-deck/internal/env/ssh.go` to use probe instead of bootstrap, delete `cc-deck/internal/ssh/bootstrap.go`
- [ ] T025 [US3] Allow multiple environments targeting same SSH host (FR-019) by removing single-host uniqueness checks in `cc-deck/internal/env/ssh.go`

**Checkpoint**: `/cc-deck.build --target ssh` generates and runs Ansible playbooks. Converged playbooks run standalone. Probe replaces bootstrap. All 6 acceptance scenarios from US-3 pass.

---

## Phase 6: User Story 4 - Reuse Single Capture for Both Targets (Priority: P2)

**Goal**: A single `/cc-deck.capture` run populates a manifest that drives both container and SSH builds.

**Independent Test**: Initialize with `--target container,ssh`, run capture once, then build for each target separately. Verify both produce correct results from same manifest.

### Implementation for User Story 4

- [ ] T026 [US4] Validate that `/cc-deck.capture` does not modify `targets` section when both targets are present by reviewing and testing `cc-deck/internal/setup/commands/cc-deck.capture.md`
- [ ] T027 [US4] End-to-end validation: init with both targets, capture, build container, build ssh (manual test against Hetzner VM)

**Checkpoint**: Both acceptance scenarios from US-4 pass. Single capture drives both backends.

---

## Phase 7: User Story 5 - Detect Manifest Drift (Priority: P3)

**Goal**: Diff command compares manifest against generated artifacts and reports what changed.

**Independent Test**: Generate artifacts, modify manifest, run diff, verify report shows expected changes.

### Implementation for User Story 5

- [ ] T028 [US5] Add `--target` flag to `cc-deck setup diff` in `cc-deck/internal/cmd/setup.go`
- [ ] T029 [US5] Implement SSH target diff: compare manifest against Ansible role task files, auto-detect target from `roles/` directory existence in `cc-deck/internal/cmd/setup.go`

**Checkpoint**: Diff reports drift for both container and SSH targets. All 3 acceptance scenarios from US-5 pass.

---

## Phase 8: User Story 6 - Verify a Provisioned Target (Priority: P3)

**Goal**: Smoke-test a provisioned target (container or SSH remote) for expected tool availability.

**Independent Test**: Run verify against a built image and a provisioned SSH host, check pass/fail report.

### Implementation for User Story 6

- [ ] T030 [US6] Add `--target` flag to `cc-deck setup verify` in `cc-deck/internal/cmd/setup.go`
- [ ] T031 [US6] Implement SSH target verify: connect via SSH, run tool checks (cc-deck, Claude Code, Zellij, manifest tools) in `cc-deck/internal/cmd/setup.go`

**Checkpoint**: Verify reports pass/fail per tool for both targets. All 3 acceptance scenarios from US-6 pass.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, testing, and quality improvements across all stories.

- [ ] T032 [P] Update `README.md`: add spec 034 to feature table, replace `cc-deck image` with `cc-deck setup`, document SSH provisioning workflow (use prose plugin)
- [ ] T033 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc`: add `cc-deck setup` command group with all subcommands and flags (use prose plugin)
- [ ] T034 [P] Create Antora guide page `docs/modules/using/pages/setup.adoc`: overview, prerequisites, container workflow, SSH workflow, dual-target, troubleshooting (use prose plugin)
- [ ] T035 [P] Update landing page at `cc-deck/cc-deck.github.io`: add feature card for unified setup command (use prose plugin)
- [ ] T036 Integration test against SSH target (Hetzner VM): full workflow init/capture/build/verify, validate F-001 through F-010 resolved, verify standalone playbook re-run
- [ ] T037 Run `quickstart.md` validation: verify all steps from quickstart produce expected results

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup (Phase 1) completion, BLOCKS all user stories
- **US1 Init (Phase 3)**: Depends on Foundational (Phase 2)
- **US2 Container Build (Phase 4)**: Depends on US1 (needs init + capture working)
- **US3 SSH Provisioning (Phase 5)**: Depends on US1 (needs init + capture working). T022-T025 (probe + bootstrap removal) can run in parallel with T016-T021 (Claude command).
- **US4 Dual Target (Phase 6)**: Depends on US2 AND US3 (needs both backends working)
- **US5 Drift (Phase 7)**: Depends on US2 or US3 (needs generated artifacts to diff against)
- **US6 Verify (Phase 8)**: Depends on US2 or US3 (needs provisioned target to verify)
- **Polish (Phase 9)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: Start after Foundational. No dependencies on other stories.
- **US2 (P1)**: Start after US1. Independently testable.
- **US3 (P1)**: Start after US1. Independently testable. Can run in parallel with US2 (different code paths).
- **US4 (P2)**: Requires US2 AND US3 complete. Validation-only phase.
- **US5 (P3)**: Requires at least one of US2/US3. Independently testable.
- **US6 (P3)**: Requires at least one of US2/US3. Independently testable.

### Within Each User Story

- Models before services
- Core implementation before integration
- Story complete before moving to next priority
- Commit after each task or logical group

### Parallel Opportunities

- T001, T002, T003 in Setup can run sequentially (same package rename)
- T004-T008 in Foundational are sequential (struct depends on prior changes)
- T022-T023 (probe) can run in parallel with T016-T021 (Claude command SSH section)
- T028-T029 (drift) can run in parallel with T030-T031 (verify)
- T032-T035 (documentation) can all run in parallel

---

## Parallel Example: User Story 3

```bash
# Probe implementation runs in parallel with Claude command updates:
# Agent A:
Task: "T022 Create lightweight probe in cc-deck/internal/ssh/probe.go"
Task: "T023 Write unit tests for probe in cc-deck/internal/ssh/probe_test.go"

# Agent B:
Task: "T016 Add --target ssh section to /cc-deck.build"
Task: "T017 Add Ansible role generation instructions"

# After both complete:
Task: "T024 Simplify SSHEnvironment.Create() to use probe"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (package rename)
2. Complete Phase 2: Foundational (manifest v2)
3. Complete Phase 3: User Story 1 (init + capture)
4. Complete Phase 4: User Story 2 (container build)
5. **STOP and VALIDATE**: Container workflow works end-to-end (preserves existing functionality)
6. This MVP proves the rename works without breaking the container path

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 (init) + US2 (container) -> Test independently -> Container path preserved (MVP)
3. Add US3 (SSH provisioning) -> Test against Hetzner VM -> SSH provisioning works
4. Add US4 (dual target) -> Validate single capture drives both backends
5. Add US5 (drift) + US6 (verify) -> Quality tooling complete
6. Polish: Documentation across all stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (init) then US2 (container build)
   - Developer B: US3 probe + bootstrap (T022-T025)
3. After US1 done, Developer B adds Claude command SSH section (T016-T021)
4. US4 validation after both US2 and US3 complete
5. US5 and US6 can be done by either developer in parallel

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- All documentation must use prose plugin with cc-deck voice profile
- Use `make install`, `make test`, `make lint` (never direct `go build`)
- Stop at any checkpoint to validate story independently
