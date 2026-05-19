# Tasks: OpenShell Build Target

**Input**: Design documents from `specs/056-openshell-build-target/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/policy-schema.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project setup needed. This feature extends the existing cc-deck CLI.

- [X] T001 Verify existing test suite passes with `make test` and `make lint` before starting changes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Manifest schema extensions and policy generation infrastructure that ALL user stories depend on.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 [P] Add `OpenShellTarget` struct and policy-related types (`OpenShellPolicy`, `FilesystemPolicy`, `LandlockConfig`, `ProcessConfig`, `NetworkPolicy`, `PolicyEndpoint`, `PolicyBinary`) to `cc-deck/internal/build/manifest.go`. Add `OpenShell *OpenShellTarget` field to `TargetsConfig`. Add `OpenShellImageRef()` method (returns `name:tag` from `targets.openshell`, defaults tag to `latest`). Add `OpenShellBaseImage()` method (returns base image with default `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`). Add validation in `Validate()`: when `targets.openshell` is present, `Name` is required. See `data-model.md` for struct definitions.
- [X] T003 [P] Create `cc-deck/internal/build/policy.go` with default policy generation. Implement `DefaultPolicy() *PolicyFile` that returns the FR-013/FR-014/FR-015 defaults (filesystem_policy, landlock, process sections). Implement `GeneratePolicy(manifest *Manifest) (*PolicyFile, error)` that builds the default policy and auto-generates `network_policies` entries from `manifest.Network.AllowedDomains` (each domain gets an entry with port 443, no binary restrictions by default). Implement `MarshalPolicy(policy *PolicyFile) ([]byte, error)` that serializes to YAML with `version: 1` header. Use the existing `default-policy.yaml` at `cc-deck/internal/openshell/default-policy.yaml` as the reference for schema structure. Define the `PolicyFile` struct with Version, FilesystemPolicy, Landlock, Process, and NetworkPolicies fields.
- [X] T004 [P] Add commented `openshell` target section to `cc-deck/internal/build/templates/build.yaml.tmpl`. Follow the existing pattern of commented `container` and `ssh` sections. Include `name`, `base`, `tag`, `registry`, and `policy` sub-fields (all commented). The `name` field should use `{{.ImageName}}` template variable. The `base` field should default to `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`.
- [X] T005 Extend `uncommentTargets()` in `cc-deck/internal/build/init.go` to handle `openshell` sections. Add `hasOpenShell := containsTarget(targets, "openshell")` check. Add `#   openshell:` detection branch. When `hasOpenShell` is true, uncomment the openshell sub-section lines. Also create `openshell/` directory in `InitSetupDir()` when openshell target is selected (similar to `container/context/` creation for container target).
- [X] T006 Extend `cc-deck/internal/cmd/build.go`: (1) Update `newBuildInitCmd` to accept `openshell` as a valid `--target` value alongside `container` and `ssh`. (2) Update `detectRunTarget()` to check for `openshell/Containerfile` alongside `container/Containerfile` and `ssh/site.yml`. Handle triple-ambiguity (all three present). (3) Update `newBuildRunCmd` dispatch to handle `"openshell"` target by calling a new `runOpenShellBuild(dir, push)` function. (4) Implement `runOpenShellBuild()` that loads the manifest, reads `OpenShellImageRef()`, and builds via `podman build -t <ref> -f openshell/Containerfile .` (same pattern as `runContainerBuild` but reading from `targets.openshell`). Handle `--push` using `targets.openshell.registry`.
- [X] T007 Update `.gitignore` template in `cc-deck/internal/build/init.go` (`gitignoreContent` constant) to add `openshell/` alongside `container/` in the generated files section.

**Checkpoint**: Foundation ready. Manifest parses `targets.openshell`, policy defaults generate, init scaffolds openshell directory, CLI dispatches to openshell target.

---

## Phase 3: User Story 1 - Build OpenShell Image from Manifest (Priority: P1)

**Goal**: A user adds `targets.openshell` to `build.yaml`, runs `/cc-deck.build --target openshell`, and gets a Containerfile using the OpenShell base image with tools installed and a policy.yaml embedded at `/etc/openshell/policy.yaml`.

**Independent Test**: Add `targets.openshell` to an existing build.yaml with tools and `allowed_domains`. Run `/cc-deck.build --target openshell`. Verify the generated Containerfile uses the OpenShell base image, installs the declared tools, and COPYs `policy.yaml` to `/etc/openshell/policy.yaml`. Run `cc-deck build run --target openshell` and verify the image builds.

### Implementation for User Story 1

- [X] T008 [US1] Add Section C (OpenShell Build) to `cc-deck/internal/build/commands/cc-deck.build.md`. This is the AI-driven command spec that Claude Code follows when generating the Containerfile. Section C must: (C1) Read and validate `targets.openshell` from manifest (name required, tag defaults to latest, base defaults to OpenShell community image). (C2) Generate Containerfile with OpenShell base image, `sandbox` user, `/sandbox` workdir. Tool installation follows the same logic as Section A (dnf for packages, curl for github-release). Include Claude Code native installer layer (`curl -fsSL https://claude.ai/install.sh | sh` as sandbox user). Create skills directories (`/sandbox/.agents/skills/`, `/sandbox/.claude/skills/`). COPY `openshell/policy.yaml` to `/etc/openshell/policy.yaml`. Set `USER sandbox`, `WORKDIR /sandbox`, `ENTRYPOINT ["/bin/bash"]`. Do NOT include the 3 mandatory cc-deck/Zellij layers from Section A. (C3) Check for existing Containerfile (same diff-and-ask pattern as A3). (C4) Generate `openshell/policy.yaml` from `network.allowed_domains` with the default policy structure (see contracts/policy-schema.md). (C5) Build the image with `podman build`. (C6) Self-correction loop (same as A6). (C7) Generate `openshell/build.sh` (same pattern as A7). (C8) Handle `--push` (same as A8). (C9) Report results including usage hint for OpenShell workspace creation. Add Step 0 dispatch entry for `--target openshell`.
- [X] T009 [US1] Add unit tests for manifest OpenShell extensions in `cc-deck/internal/build/manifest_test.go`. Test: (1) `LoadManifest` with `targets.openshell` section parses correctly. (2) `Validate` requires `name` when `targets.openshell` is present. (3) `OpenShellImageRef()` returns correct `name:tag`. (4) `OpenShellBaseImage()` returns default when not specified. (5) Parsing with full policy overrides deserializes all nested types.
- [X] T010 [US1] Add unit tests for default policy generation in `cc-deck/internal/build/policy_test.go`. Test: (1) `DefaultPolicy()` returns correct filesystem_policy, landlock, and process sections per FR-013/FR-014/FR-015. (2) `GeneratePolicy` with empty `AllowedDomains` produces policy with empty `network_policies`. (3) `GeneratePolicy` with `AllowedDomains: ["api.anthropic.com", "github.com"]` produces two `network_policies` entries. (4) `MarshalPolicy` produces valid YAML with `version: 1` header.
- [X] T011 [US1] Add tests for init scaffolding in `cc-deck/internal/build/init_test.go`. Test: (1) `InitSetupDir` with `targets: ["openshell"]` creates `openshell/` directory. (2) `uncommentTargets` with openshell=true uncomments the openshell section in the manifest template. (3) `containsTarget` recognizes "openshell".
- [X] T012 [US1] Add tests for detectRunTarget extension in `cc-deck/internal/cmd/build_test.go` (or add to existing test file). Test: (1) `detectRunTarget` with only `openshell/Containerfile` returns "openshell". (2) `detectRunTarget` with both `container/Containerfile` and `openshell/Containerfile` returns error asking user to specify. (3) `detectRunTarget` with explicit "openshell" returns "openshell". (4) `validateRunFlags` allows `--push` with openshell target.

**Checkpoint**: User Story 1 fully functional. Users can generate and build OpenShell images from manifests with auto-generated policies.

---

## Phase 4: User Story 2 - Policy Merge with Explicit Overrides (Priority: P2)

**Goal**: A developer defines per-binary network rules under `targets.openshell.policy` that override auto-generated rules from `network.allowed_domains`. Explicit entries replace auto-generated entries matched by endpoint host.

**Independent Test**: Define `targets.openshell.policy.network_policies` with a rule for `github.com` scoped to `/usr/bin/git`. Also have `github.com` in `network.allowed_domains`. Build and verify the explicit per-binary rule replaces the auto-generated all-binaries rule. Verify entries for domains NOT in `allowed_domains` are additive.

### Implementation for User Story 2

- [X] T013 [US2] Implement policy merge logic in `cc-deck/internal/build/policy.go`. Add `MergePolicy(base *PolicyFile, overrides *OpenShellPolicy) *PolicyFile` function. Merge semantics: (1) If overrides has `filesystem_policy`, replace base's entirely. (2) If overrides has `landlock`, replace base's entirely. (3) If overrides has `process`, replace base's entirely. (4) For `network_policies`: iterate override entries, collect all endpoint hosts. For each auto-generated entry in base, if any of its endpoints match an override entry's endpoints by host, replace the auto-generated entry with the override. Add override entries for hosts not in base. See contracts/policy-schema.md for full merge semantics.
- [X] T014 [US2] Update Section C in `cc-deck/internal/build/commands/cc-deck.build.md` step C4 to describe the merge behavior. When `targets.openshell.policy` is defined, the AI command should: generate the default policy first, then apply overrides per the merge semantics. Document that explicit `network_policies` entries override auto-generated ones by endpoint host match.
- [X] T015 [US2] Add unit tests for policy merge in `cc-deck/internal/build/policy_test.go`. Test: (1) Override `filesystem_policy` replaces default entirely. (2) Override `network_policies` entry for `github.com` replaces auto-generated `github.com` entry. (3) Override entry for domain NOT in `allowed_domains` is additive. (4) Non-overridden auto-generated entries are preserved. (5) Override with `process` section replaces default process config. (6) Empty overrides produce unchanged base policy.

**Checkpoint**: Policy overrides work correctly. Explicit per-binary rules replace auto-generated permissive rules.

---

## Phase 5: User Story 3 - Binary Path Discovery During Build (Priority: P2)

**Goal**: When the build system generates install instructions for tools, it simultaneously resolves binary paths. These paths are used in the OpenShell policy's `network_policies` for per-binary scoping.

**Independent Test**: Define tools including `git`, `node`, and `Claude Code` in the manifest. Generate the OpenShell Containerfile. Verify the policy contains network_policies entries with correct binary paths (`/usr/bin/git`, `/usr/bin/node`, `/usr/local/bin/claude`).

### Implementation for User Story 3

- [X] T016 [US3] Update Section C in `cc-deck/internal/build/commands/cc-deck.build.md` step C2 and C4 to describe binary path discovery. During Containerfile generation, the AI command should track which binary path each tool installs to: (1) dnf packages install to `/usr/bin/<binary>` (2) npm global packages to `/usr/local/bin/<name>` (3) github-release tools to their `install_path` field or `/usr/local/bin/<name>` (4) Well-known defaults: Claude Code at `/usr/local/bin/claude`, git at `/usr/bin/git`, node at `/usr/bin/node`, python3 at `/usr/bin/python3`, go at `/usr/bin/go`. When generating `policy.yaml`, the AI command should associate discovered binary paths with the appropriate `network_policies` entries based on which tools logically use which domains (inferred during generation).
- [X] T017 [US3] Add well-known binary path defaults as a reference table in `cc-deck/internal/build/policy.go`. Create a `WellKnownBinaries` map constant mapping tool names to binary paths. This is not used programmatically by Go code (the AI command does the association during generation), but serves as a reference for the command spec and tests. Document the mapping in a code comment.

**Checkpoint**: Binary path discovery documented in command spec. Generated policies include per-binary scoping.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, validation, and cleanup.

- [X] T018 [P] Update CLI reference documentation in `docs/modules/reference/pages/cli.adoc` to document `--target openshell` for `cc-deck build init`, `cc-deck build run`, and `cc-deck build verify` commands. Use one sentence per line (AsciiDoc semantic line breaks).
- [X] T019 [P] Update `README.md` to mention the OpenShell build target alongside existing container and SSH targets. Use the prose plugin with cc-deck voice profile.
- [X] T020 Run `make test` and `make lint` to verify all tests pass and no linting issues exist.
- [X] T021 Run quickstart.md validation: verify the documented workflow (`build.yaml` with `targets.openshell`, `/cc-deck.build --target openshell`, `cc-deck build run --target openshell`) produces a valid image with `/etc/openshell/policy.yaml` embedded.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion. BLOCKS all user stories.
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 2 completion. Can run in parallel with US1 (policy merge is independent of Containerfile generation command spec)
- **User Story 3 (Phase 5)**: Depends on US1 (the command spec from T008 must exist before T016 can update it)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2. No dependencies on other stories.
- **US2 (P2)**: Can start after Phase 2. Independent of US1 (merge logic is in Go code, command spec update in T014 is small).
- **US3 (P2)**: Depends on US1 (T008 creates the command spec that T016 modifies).

### Within Each User Story

- Go code changes before command spec changes
- Tests alongside or after implementation
- Each story independently testable

### Parallel Opportunities

- T002, T003, T004 can run in parallel (different files: manifest.go, policy.go, build.yaml.tmpl)
- T009, T010, T011, T012 can run in parallel (different test files)
- US1 and US2 implementation can run in parallel after Phase 2
- T018, T019 can run in parallel (different doc files)

---

## Parallel Example: Phase 2 Foundational

```bash
# Launch foundational tasks in parallel (different files):
Task: "Add OpenShellTarget struct to cc-deck/internal/build/manifest.go"
Task: "Create policy.go with default policy generation in cc-deck/internal/build/policy.go"
Task: "Add openshell section to cc-deck/internal/build/templates/build.yaml.tmpl"
```

## Parallel Example: User Story 1 Tests

```bash
# Launch US1 test tasks in parallel (different test files):
Task: "Manifest OpenShell tests in cc-deck/internal/build/manifest_test.go"
Task: "Policy generation tests in cc-deck/internal/build/policy_test.go"
Task: "Init scaffolding tests in cc-deck/internal/build/init_test.go"
Task: "detectRunTarget tests in cc-deck/internal/cmd/build_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (verify baseline)
2. Complete Phase 2: Foundational (schema + policy + init + CLI)
3. Complete Phase 3: User Story 1 (command spec + tests)
4. **STOP and VALIDATE**: Generate an OpenShell image from a test manifest
5. The image builds and contains `/etc/openshell/policy.yaml` with defaults

### Incremental Delivery

1. Setup + Foundational -> Schema and CLI ready
2. Add US1 -> Core build works, auto-generated policy -> MVP
3. Add US2 -> Policy overrides work -> Per-binary scoping
4. Add US3 -> Binary path discovery integrated -> Full policy accuracy
5. Polish -> Documentation complete -> Ship

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- The AI-driven command spec (`cc-deck.build.md`) is the central artifact for US1 and US3
- Go code changes (manifest.go, policy.go, build.go, init.go) are the core of Phase 2
- Policy merge (US2) is pure Go logic in policy.go, independent of command spec
- Binary path discovery (US3) is primarily a command spec documentation task
- Use `make test` and `make lint` (never direct `go build`)
- Use `podman` for all image operations
