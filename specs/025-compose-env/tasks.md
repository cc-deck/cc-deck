# Tasks: Compose Environment

**Input**: Design documents from `specs/025-compose-env/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Unit tests for all new code. Integration tests for end-to-end flows requiring podman + podman-compose.

**Organization**: Tasks grouped by user story. Each story is independently implementable and testable after foundational phase completes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No project initialization needed. Existing Go project with established structure.

- [x] T001 Verify compose runtime availability: confirm `podman-compose` is installed and accessible for development via `which podman-compose`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Type system changes, shared helpers, and CLI wiring that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T002 Extract `detectAuthMode()`, `detectAuthCredentials()`, and `containerHasZellijSession()` from `cc-deck/internal/env/container.go` into new `cc-deck/internal/env/auth.go` (auth functions) and export `ContainerHasZellijSession()` for shared use. Update `container.go` to call the exported versions. Run `make test` to verify no regressions.
- [x] T003 [P] Add `EnvironmentTypeCompose` constant and `ComposeFields` struct to `cc-deck/internal/env/types.go`. Add `Type EnvironmentType` field and `Compose *ComposeFields` to `EnvironmentInstance`. Ensure backward compatibility: instances without Type default to "container".
- [x] T004 [P] Add `AllowedDomains []string` and `ProjectDir string` fields to `EnvironmentDefinition` in `cc-deck/internal/env/definition.go`
- [x] T005 Add `EnvironmentTypeCompose` case to `NewEnvironment()` in `cc-deck/internal/env/factory.go`, returning a `ComposeEnvironment` struct
- [x] T006 [P] Create compose runtime detection in `cc-deck/internal/compose/runtime.go`: detect `podman-compose`, `docker compose` (v2 plugin), and `docker-compose` (legacy) in PATH. Export `Available() (string, error)` returning the detected binary path.
- [x] T007 Update compose YAML generator in `cc-deck/internal/compose/generate.go`: add workspace volume mount option, `stdin_open`/`tty` fields, and optional secrets directory volume mount. Add `Volumes []string` and `SecretsDir string` to `GenerateOptions`.
- [x] T008 Add compose-specific CLI flags to `cc-deck/internal/cmd/env.go`: `--allowed-domains` (string slice), `--path` (string, defaults to cwd), `--gitignore` (bool). Wire `ComposeEnvironment` options in `runEnvCreate()`. Default `--storage` to `host-path` when `--type compose` (FR-003).
- [x] T009 Update `resolveEnvironment()` in `cc-deck/internal/cmd/env.go` to check `EnvironmentInstance.Type` field (or `Compose` field presence) and return compose environment for compose instances. Reconciliation wiring is deferred to T017.

**Checkpoint**: Foundation ready. All type changes, shared helpers, and CLI wiring in place.

---

## Phase 3: User Story 1 - Create and Use a Compose Environment (Priority: P1) MVP

**Goal**: User can create a compose environment in a project directory, attach to an interactive Zellij session, and see project files at `/workspace`.

**Independent Test**: Run `cc-deck env create mydev --type compose` in a project directory, then `cc-deck env attach mydev`. Verify sidebar loads and `/workspace` contains project files.

### Implementation for User Story 1

- [x] T010 [US1] Create `ComposeEnvironment` struct with `Name()`, `Type()` methods and constructor in `cc-deck/internal/env/compose.go`. Include fields: name, store, defs, Auth, Ports, AllPorts, Credentials, Mounts, AllowedDomains, ProjectDir, Gitignore.
- [x] T011 [US1] Implement `ComposeEnvironment.Create()` in `cc-deck/internal/env/compose.go`: validate name, check compose runtime available, resolve image/storage/project dir (default to `host-path` for compose), create `.cc-deck/` directory, generate compose.yaml using `internal/compose` generator with workspace volume, write files, run `podman-compose up -d`, write definition and instance to stores. On failure, clean up partially created resources (`.cc-deck/` directory, secrets) before returning error (FR-020).
- [x] T012 [US1] Implement `ComposeEnvironment.Attach()` in `cc-deck/internal/env/compose.go`: nested Zellij check, auto-start stopped environment, update LastAttached timestamp, check for existing Zellij session inside container using shared helper, exec into container with `zellij -n cc-deck` or `zellij attach`.
- [x] T013 [P] [US1] Implement `ComposeEnvironment.Exec()`, `Push()`, `Pull()`, `Harvest()` in `cc-deck/internal/env/compose.go`. Exec delegates to `podman.Exec()` on session container name. Push/Pull use `podman.Cp()` on session container name. Harvest returns `ErrNotSupported`.
- [x] T014 [US1] Unit tests for Create and Attach in `cc-deck/internal/env/compose_test.go`: test name validation, compose runtime check, file generation, instance recording. Use temp directories for project dir and state files.

**Checkpoint**: A user can create a compose environment, attach to it, see project files at `/workspace`. MVP is functional.

---

## Phase 4: User Story 3 - Full Lifecycle Management (Priority: P2)

**Goal**: User can stop, start, and delete compose environments. State is preserved across stop/start cycles. Delete cleans up all artifacts.

**Independent Test**: Create a compose environment, write a file inside, stop it, start it, verify file persists. Delete and verify all artifacts removed.

### Implementation for User Story 3

- [x] T015 [P] [US3] Implement `ComposeEnvironment.Start()` and `Stop()` in `cc-deck/internal/env/compose.go`: use compose runtime to start/stop the compose project, update instance state in store.
- [x] T016 [US3] Implement `ComposeEnvironment.Delete()` in `cc-deck/internal/env/compose.go`: running check (refuse unless force), `podman-compose down`, remove `.cc-deck/` directory, remove secrets, remove instance and definition from stores. Best-effort cleanup with warnings on partial failures.
- [x] T017 [US3] Implement `ComposeEnvironment.Status()` and `ReconcileComposeEnvs()` in `cc-deck/internal/env/compose.go`: reconcile stored state against `podman inspect` on session container. Wire `ReconcileComposeEnvs()` into list command in `cc-deck/internal/cmd/env.go`.
- [x] T018 [US3] Unit tests for Start, Stop, Delete, Status in `cc-deck/internal/env/compose_test.go`: test state transitions, cleanup behavior, reconciliation logic.

**Checkpoint**: Full lifecycle works. Compose environments can be stopped, restarted, and cleanly deleted.

---

## Phase 5: User Story 4 - Credential Passthrough (Priority: P2)

**Goal**: Host authentication credentials are auto-detected and injected into the session container, matching container type behavior.

**Independent Test**: Set `ANTHROPIC_API_KEY` on host, create a compose environment, exec into container and verify the key is available.

### Implementation for User Story 4

- [x] T019 [US4] Integrate auth detection in `ComposeEnvironment.Create()` in `cc-deck/internal/env/compose.go`: call shared `DetectAuthMode()` and `DetectAuthCredentials()`, resolve credential values from definition and host environment.
- [x] T020 [US4] Generate `.cc-deck/.env` file with detected environment variable credentials in `cc-deck/internal/env/compose.go`. Handle file-based credentials: copy to `.cc-deck/secrets/` directory, add volume mount in compose.yaml, set env var pointing to `/run/secrets/<name>`.
- [x] T021 [US4] Unit tests for credential detection and .env generation in `cc-deck/internal/env/compose_test.go`: test API key injection, Vertex ADC file handling, Bedrock credentials, explicit --credential flags.

**Checkpoint**: Credential passthrough works. Same detection as container type, compose-native injection via .env file and volume mounts.

---

## Phase 6: User Story 2 - Network Filtering via Proxy Sidecar (Priority: P2)

**Goal**: When `--allowed-domains` is specified, a tinyproxy sidecar is added that enforces domain allowlisting. Requests to unlisted domains are blocked.

**Independent Test**: Create with `--allowed-domains anthropic`, attach, verify `curl https://api.anthropic.com` succeeds and `curl https://example.com` is blocked.

### Implementation for User Story 2

- [x] T022 [US2] Integrate domain resolution in `ComposeEnvironment.Create()` in `cc-deck/internal/env/compose.go`: use `internal/network.Resolver.ExpandAll()` to resolve group names to domain lists. Pass resolved domains to compose generator.
- [x] T023 [US2] Wire proxy sidecar generation in Create: when AllowedDomains is non-empty, generate proxy config files (tinyproxy.conf, whitelist) in `.cc-deck/proxy/` and include proxy service in compose.yaml via existing generator.
- [x] T024 [US2] Unit tests for domain resolution integration and proxy file generation in `cc-deck/internal/env/compose_test.go`: test domain group expansion, proxy config output, compose YAML with proxy service.

**Checkpoint**: Network filtering works. Proxy sidecar enforces domain allowlist.

---

## Phase 7: User Story 5 - Gitignore and Project Hygiene (Priority: P3)

**Goal**: Users are warned when `.cc-deck/` is not in `.gitignore`. The `--gitignore` flag auto-adds it.

**Independent Test**: Create compose environment in a git repo without `.cc-deck/` in `.gitignore`, verify warning printed. Test with `--gitignore` to verify auto-addition.

### Implementation for User Story 5

- [x] T025 [US5] Implement gitignore detection, warning, and `--gitignore` auto-addition in `cc-deck/internal/env/compose.go`: check if project is git-tracked, check if `.cc-deck/` is in `.gitignore`, warn if not, append if `--gitignore` flag set. Skip if already present or not a git repo.
- [x] T026 [US5] Unit tests for gitignore handling in `cc-deck/internal/env/compose_test.go`: test warning output, auto-addition, skip when already present, skip when not a git repo.

**Checkpoint**: Gitignore hygiene works. Users get clear guidance about generated files.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, list/status output updates, edge cases

- [x] T027 [P] Update README.md with compose environment documentation and add spec 025 to feature specifications table
- [x] T028 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc` with compose flags and commands
- [x] T029 [P] Create compose environment guide page in `docs/modules/running/pages/compose.adoc` covering overview, quickstart, network filtering, and credential passthrough. Add to `docs/modules/running/nav.adoc`.
- [x] T030 Handle edge cases in `ComposeEnvironment.Create()` in `cc-deck/internal/env/compose.go`: existing `.cc-deck/` directory (regenerate with warning, FR-014), non-writable project directory (clear error), proxy sidecar failure (session should not start)
- [x] T031 [P] Add compose runtime detection unit tests in `cc-deck/internal/compose/runtime_test.go`
- [x] T032 [P] Add compose YAML generator tests for updated options (volumes, stdin/tty, secrets) in `cc-deck/internal/compose/generate_test.go`
- [x] T033 Run `make test` and `make lint` from project root to verify all tests pass and no lint issues across `cc-deck/` packages

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Foundational (Phase 2)
- **US3 (Phase 4)**: Depends on US1 (Phase 3) for ComposeEnvironment struct
- **US4 (Phase 5)**: Depends on US1 (Phase 3) for Create method. Can run in parallel with US3.
- **US2 (Phase 6)**: Depends on US1 (Phase 3) for Create method. Can run in parallel with US3/US4.
- **US5 (Phase 7)**: Depends on US1 (Phase 3) for Create method. Can run in parallel with US2/US3/US4.
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

```text
Phase 2 (Foundational)
    │
    ▼
Phase 3 (US1: Create and Use) ◄── MVP
    │
    ├──► Phase 4 (US3: Lifecycle)     ─┐
    ├──► Phase 5 (US4: Credentials)    │── Can run in parallel
    ├──► Phase 6 (US2: Filtering)      │
    └──► Phase 7 (US5: Gitignore)     ─┘
                                       │
                                       ▼
                              Phase 8 (Polish)
```

### Within Each User Story

- Implementation tasks before tests (tests validate the implementation)
- Core Create logic before auxiliary methods (Exec, Push, Pull)
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 2: T003, T004, T006 can run in parallel (different files)
- Phase 3: T013 can run in parallel with T011-T012 (different methods, no dependencies)
- Phase 4-7: All four user story phases can run in parallel after US1 completes
- Phase 8: T027, T028, T029, T031, T032 can all run in parallel (different files)

---

## Parallel Example: Foundational Phase

```bash
# Launch independent foundational tasks together:
Task: "Add ComposeFields and EnvironmentTypeCompose to types.go"         # T003
Task: "Add AllowedDomains, ProjectDir to definition.go"                   # T004
Task: "Create compose runtime detection in compose/runtime.go"            # T006
```

## Parallel Example: After US1 Completes

```bash
# Launch all P2/P3 stories in parallel:
Task: "Implement Start/Stop in compose.go"                                # T015 (US3)
Task: "Integrate auth detection in Create"                                # T019 (US4)
Task: "Integrate domain resolution in Create"                             # T022 (US2)
Task: "Implement gitignore detection"                                     # T025 (US5)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (verify tooling)
2. Complete Phase 2: Foundational (types, factory, CLI wiring, generator updates)
3. Complete Phase 3: User Story 1 (Create, Attach, Exec, Push/Pull)
4. **STOP and VALIDATE**: Test compose create/attach end-to-end
5. A user can create a compose environment, attach, and work with project files

### Incremental Delivery

1. Setup + Foundational: Type system and CLI wiring ready
2. Add US1 (Create/Attach): MVP, test independently
3. Add US3 (Lifecycle): Stop/start/delete, test independently
4. Add US4 (Credentials): Auth passthrough, test independently
5. Add US2 (Filtering): Network filtering, test independently
6. Add US5 (Gitignore): Project hygiene, test independently
7. Polish: Documentation, edge cases, cleanup

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable after Phase 2
- Commit after each task or logical group
- All compose files generated in `.cc-deck/` within project directory (gitignored)
- Compose generator in `internal/compose/` is reused, not rewritten
- Auth detection in `internal/env/auth.go` is shared with container type
- Session container name follows `cc-deck-<env-name>` convention
