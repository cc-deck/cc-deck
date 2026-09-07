---

description: "Task list for workspace sharing via external endpoint"
---

# Tasks: Workspace Sharing via External Endpoint

**Input**: Design documents from `/specs/087-workspace-sharing-endpoint/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Test tasks ARE included and are NOT optional. Constitution Principle I makes tests a
completion condition, and SC-002 requires specific failure modes to fail the build. The lesson
recorded in the source design is that a complete unit suite passed while the feature had never worked
for a human, which is why the end to end test in T032 carries as much weight as the unit matrix.

**Organization**: Grouped by user story so each is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1 through US5)

## Path Conventions

Go CLI at `cc-deck/`, Antora docs at `docs/`. Never run `go build` directly; use `make test`,
`make lint`, `make install`.

## Global Constraints

**Every task inherits the Global Constraints and Shared Interfaces sections of
[plan.md](./plan.md).** Read both before starting any task. The four that catch people out:

- **No new dependencies.** The WebSocket stage is hand written on purpose.
- **No secret reaches disk**, including the probe's own credential.
- **No background work.** Nothing polls, schedules, or supervises.
- **Session-creating Zellij commands** run under `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID`,
  or they silently add a tab to your live session and exit zero.

Tasks below carry an **Interfaces** note wherever they consume a symbol defined by an earlier task.

---

## Phase 1: Setup

**Purpose**: Establish a trustworthy baseline before changing anything.

- [X] T001 Record the pre-existing test baseline by running `make test` and capturing which tests already fail on this branch, expected to be seven `TestVoiceRelay_*` tests in `cc-deck/internal/voice` and the compose smoke tests, so later failures are not misattributed to this feature
- [X] T002 [P] Verify the binary under test is this branch's build, not the stale `~/bin/cc-deck` symlink into the main checkout, by running `make install` and confirming with `ls -l "$(command -v cc-deck)"` as described in `specs/087-workspace-sharing-endpoint/quickstart.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Data model, configuration, and the interface seam. Every user story depends on these.

**⚠️ CRITICAL**: T005 through T008d are ONE atomic change. The `share` package will not compile at any
point inside that window, production code and test code alike. Do not run tests until T008d is done.

The test files matter as much as the production files here. `Provider`, `ProviderHandle`,
`ProviderStatus`, and `Process` are referenced by `service_test.go`, `provider_contract_test.go`,
`test_fakes_test.go`, and `zellij_test.go`. Leaving any of them for a later phase leaves the package
uncompilable for the whole of Phase 3, which destroys the T001 baseline exactly when it is needed most.

- [X] T003 [P] Add `ProbeStage` constants and the `ProbeResult` struct with `OK`, `FailedAt`, `Diagnostic`, and `CheckedAt` fields to `cc-deck/internal/share/model.go` per `data-model.md`
- [X] T004 [P] Replace `SharingConfig` in `cc-deck/internal/config/config.go` with `Endpoint`, `Endpoints`, `Default`, and `VerifyTimeout` fields, deleting `Provider`, `DefaultSharingProvider`, and the `SharingProvider()` method
- [X] T005 Add `EndpointName`, `WebServerOwned`, and `LastProbe` to `SharingOperation` and remove `Provider`, `ProviderHandle`, and `EndpointStopped` in `cc-deck/internal/share/model.go`, and mirror the field changes on `SharingStatus`
- [X] T006 Replace `Provider`, `ProviderHandle`, `ProviderStatus`, and `ProviderRegistry` with the `Endpoint` interface and `EndpointRef` struct in `cc-deck/internal/share/provider.go`, and remove `Process` and the `Start` method from `CommandRunner`
- [X] T007 Delete `cc-deck/internal/share/cloudflare.go` and `cc-deck/internal/share/cloudflare_test.go`
- [X] T008 Update `cc-deck/internal/share/service.go` and `cc-deck/internal/cmd/ws_share.go` so they compile against the new types, removing the process launch and its rollback arm from `Start`, the `commandProcess` type, and the hardcoded `provider != "cloudflare"` rejection at `ws_share.go:96`
- [X] T008a Remove `fakeProcess` and the `Start` method on `fakeRunner` from `cc-deck/internal/share/test_fakes_test.go`, along with the Cloudflare-specific fake wiring, so the test build no longer references the deleted `Process` interface
- [X] T008b Remove the `Process` references from `cc-deck/internal/share/zellij_test.go`, keeping the token and web server coverage that this feature relies on unchanged
- [X] T008c Update `cc-deck/internal/share/service_test.go` so it compiles against `Endpoint`, replacing the provider fake with an endpoint fake; behavioural assertions are rewritten later in T033 through T035 and T040 through T042, so this task only restores compilation
- [X] T008d Replace the `Provider` scaffolding in `cc-deck/internal/share/provider_contract_test.go` with an `Endpoint` skeleton sufficient to compile; the real contract coverage lands in T013
- [X] T009 Replace the provider-name check in `cc-deck/internal/config/validate.go` with validation that each configured address parses as an absolute `http` or `https` URL, that `Default` names a key in `Endpoints`, and that `VerifyTimeout` is positive
- [X] T010 [P] Update `cc-deck/internal/config/config_test.go` and `cc-deck/internal/config/validate_test.go` to cover the new schema and drop `TestSharingProviderDefault`

**Checkpoint**: `make test` compiles and every test that passed at the T001 baseline still passes, with
the exception of sharing behaviour tests reduced to compilation stubs in T008c and T008d. Sharing is
temporarily non-functional by design. If the package does not compile here, do not start Phase 3.

---

## Phase 3: User Story 1 - Share through an endpoint I already run (Priority: P1) 🎯 MVP

**Goal**: A developer with a working endpoint shares a workspace and gets an invitation that reaches a
live terminal, with verification proving it first.

**Independent Test**: Point a static endpoint at a local Zellij web server, share a workspace, open the
printed invitation, and reach a live terminal. Then break the WebSocket upgrade and confirm no
invitation is printed.

### Tests for User Story 1

> **Write these first and confirm they fail before implementing.**

> **Note on `[P]`**: T011, T012, T012a, T014, and T014a all write `probe_test.go`, so they are
> sequential with respect to each other despite being conceptually independent. Only T013 is
> genuinely parallel here, because it is the sole task touching `provider_contract_test.go`.

- [X] T011 [US1] Build the reusable `httptest` endpoint fake in `cc-deck/internal/share/probe_test.go` that can independently fail at each of the five stages, including the handler that serves the web client correctly and refuses to hijack the connection
- [X] T012 [US1] Write the ten-case stage matrix from `contracts/endpoint-contract.md` as table-driven tests in `cc-deck/internal/share/probe_test.go`, asserting the exact `FailedAt` stage for each case
- [X] T012a [US1] Add an exhaustiveness assertion in `cc-deck/internal/share/probe_test.go` that fails if any declared `ProbeStage` constant has no failure case in the T012 matrix, which is the measurement method for SC-003's claim of correct attribution in 100% of layer-specific cases
- [X] T013 [P] [US1] Rewrite `cc-deck/internal/share/provider_contract_test.go` against the `Endpoint` interface, covering contracts C-1 through C-9, including that `Resolve` performs no network access and that `Probe` never mutates sharing state
- [X] T014 [US1] Write tests in `cc-deck/internal/share/probe_test.go` asserting the probe credential is minted per probe, is revoked on success, on failure, and on deadline expiry, and never appears in persisted state
- [X] T014a [US1] Add a test in `cc-deck/internal/share/probe_test.go` that fails every stage against a fake configured with a recognizable sentinel secret, then asserts the sentinel appears in no `ProbeResult.Diagnostic`, so the no-secrets property of T021 is red before it is implemented

### Implementation for User Story 1

- [X] T015 [US1] Implement `StaticEndpoint` with `Name`, `Resolve`, and endpoint resolution precedence in `cc-deck/internal/share/endpoint.go`, reading from the new `SharingConfig` per `data-model.md`
- [X] T016 [US1] Implement the DNS stage in `cc-deck/internal/share/probe.go`, including the public-resolver second opinion that distinguishes a locally filtered name from a missing one, with the second opinion's own failure never converting a successful primary resolution into an error
- [X] T017 [US1] Implement the TLS and HTTP stages in `cc-deck/internal/share/probe.go`, confirming the Zellij web client is what is being served rather than merely receiving a 200
- [X] T018 [US1] Implement the Auth stage in `cc-deck/internal/share/probe.go`, minting an observer-role credential through `Zellij.CreateToken`, posting to `/command/login`, asserting a `session_token` cookie comes back, and revoking the credential in a `defer` that runs on every exit path including panic
- [X] T019 [US1] Implement the WebSocket stage in `cc-deck/internal/share/probe.go` as a hand-written upgrade handshake over `net`/`crypto/tls` per research R1, asserting `101` and `Sec-WebSocket-Accept` on `/ws/control`, never a terminal socket, then closing
- [X] T020 [US1] Wire the five stages into a single fail-fast `Probe` bounded by one `context.WithTimeout` derived from `VerifyTimeout`, following the deadline convention already used at `cc-deck/internal/share/zellij.go:34`

  > **Interfaces**: implements `Endpoint.Probe(ctx context.Context, ref EndpointRef, session string) (ProbeResult, error)` from T006. Returns `ProbeResult{OK, FailedAt, Diagnostic, CheckedAt}` from T003. `CheckedAt` is always set; `FailedAt` is empty only when `OK` is true.

- [X] T021 [US1] Ensure no probe diagnostic built in `cc-deck/internal/share/probe.go` can contain a token, cookie value, or authorization header, per contract C-9, turning the T014a sentinel test green
- [X] T022 [US1] Make `Start` in `cc-deck/internal/share/service.go` run the probe as a blocking gate before building invitations, persisting `LastProbe` and `WebServerOwned` on the operation

  > **Interfaces**: consumes `Endpoint.Probe` from T020 and `Zellij.EnsureWebServer(ctx) (localURL string, started bool, err error)`, which already exists. `WebServerOwned` is set from `started`. `LastProbe` is `*ProbeResult`, left nil when verification is skipped.

- [X] T023 [US1] Add `--endpoint`, `--endpoint-name`, and `--no-verify` flags to `ws start --share` in `cc-deck/internal/cmd/ws_share.go`, making `--endpoint` and `--endpoint-name` mutually exclusive
- [X] T024 [US1] Make `ws invite` run the same blocking gate in `cc-deck/internal/cmd/ws_share.go` and `cc-deck/internal/share/service.go`, leaving no credential behind when verification fails
- [X] T025 [US1] Implement the `--no-verify` path in `cc-deck/internal/share/service.go` so the share is created with `LastProbe` left nil, which is what makes a listing show no verification age per FR-014
- [X] T026 [US1] Implement the failure message shape from `contracts/cli-contract.md` in `cc-deck/internal/cmd/ws_share.go`, formatting a `ProbeResult` whose `OK` is false into the user-facing error with the failing stage named first and a pointer to the sharing guide; the stage-to-explanation text lives here, not in `probe.go`, so the probe stays free of presentation concerns
- [X] T027 [US1] Refuse to share with actionable guidance when no endpoint resolves, listing the configured endpoint names, in `cc-deck/internal/cmd/ws_share.go`
- [X] T028 [US1] Change the failed-share path in `cc-deck/internal/cmd/ws_share.go` so a workspace session created by the same command survives a verification failure, replacing the current `workspace.KillSession` call at `ws_share.go:110` per FR-031
- [X] T029 [US1] Add a test in `cc-deck/internal/cmd/ws_share_test.go` asserting that a failed `start --share` leaves the workspace running and usable
- [X] T030 [US1] Add a test in `cc-deck/internal/cmd/ws_share_test.go` asserting `--endpoint` overrides configuration for one command only and leaves the configured value unchanged
- [X] T031 [US1] Add a test in `cc-deck/internal/cmd/ws_share_test.go` asserting a `--no-verify` share reports no verification age and does not introduce a third sharing state
- [X] T032 [US1] Write the end to end acceptance test in `cc-deck/internal/share/e2e_test.go` that starts a real Zellij web server and background session, points a static endpoint at the local address, and runs the real five stage probe; it must skip when `zellij` is absent and must run every session-creating command under `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID` per research R8

**Checkpoint**: Sharing works end to end and the blank-terminal failure is caught automatically.

---

## Phase 4: User Story 2 - Find out why a share stopped working (Priority: P2)

**Goal**: A broken share reports which layer failed, quickly, and changes nothing.

**Independent Test**: Share successfully, break one layer at a time, and confirm each break produces a
distinct correctly attributed report with the share still intact afterwards.

### Tests for User Story 2

> **Note on `[P]`**: T033 through T035 all write `service_test.go`, so they are sequential with
> respect to each other, as are T040 through T042 in Phase 5.

- [X] T033 [US2] Write tests in `cc-deck/internal/share/service_test.go` asserting that a failing probe during `Status` revokes nothing, stops nothing, and deletes no state, covering every stage failure plus timeout
- [X] T034 [US2] Write a test in `cc-deck/internal/share/service_test.go` asserting that a confirmed-absent session during `Status` still triggers teardown, so severing the endpoint path does not disable the one legitimate teardown trigger
- [X] T035 [US2] Write a test in `cc-deck/internal/share/service_test.go` asserting an endpoint that recovers moves the reported state back to shared with no user intervention

### Implementation for User Story 2

- [X] T036 [US2] Sever the endpoint failure path from `reconcileLocked` in `Status` in `cc-deck/internal/share/service.go`, so only a confirmed-absent session reaches teardown, per research R5; the existing `reportUnverified` at `service.go:382` already models the correct behaviour
- [X] T037 [US2] Make `Status` in `cc-deck/internal/share/service.go` run the probe and derive the reported state from its result, reporting shared or degraded and never any third value, per FR-016
- [X] T038 [US2] Persist the completed probe result to `LastProbe` on every status check in `cc-deck/internal/share/service.go`, pass or fail, without moving `SharingOperation.State` and without triggering teardown, per FR-047 and FR-023

  > **Interfaces**: writes `SharingOperation.LastProbe *ProbeResult` from T005 and surfaces it on `SharingStatus.LastProbe` for the listing in T048. `SharingOperation.State` is untouched by this write.

- [X] T039 [US2] Narrow `StateDegraded` in `cc-deck/internal/share/service.go` so it is written only as a teardown residual marker and never because an endpoint probe failed, updating the doc comment on `reconcileLocked` to say so

**Checkpoint**: A broken endpoint is diagnosable and non-destructive. This closes the worst observed defect.

---

## Phase 5: User Story 3 - Stop sharing without collateral damage (Priority: P2)

**Goal**: Unsharing removes only what cc-deck created.

**Independent Test**: Start a Zellij web server manually, share, then unshare, and confirm the web
server and the workspace both survive while every invitation is dead.

### Tests for User Story 3

- [X] T040 [US3] Write a test in `cc-deck/internal/share/service_test.go` asserting that a web server cc-deck did not start survives teardown, and one asserting that a web server cc-deck did start is stopped
- [X] T041 [US3] Write a test in `cc-deck/internal/share/service_test.go` asserting teardown never ends the workspace session and never touches the user's endpoint
- [X] T042 [US3] Write a test in `cc-deck/internal/share/service_test.go` asserting an interrupted teardown resumes correctly on the next invocation and strands nothing

### Implementation for User Story 3

- [X] T043 [US3] Set `WebServerOwned` from the `started` return value of `Zellij.EnsureWebServer` when the operation begins in `cc-deck/internal/share/service.go`, recording ownership at the time cc-deck acts rather than inferring it later

  > **Interfaces**: `EnsureWebServer(ctx context.Context) (localURL string, started bool, err error)` already exists at `cc-deck/internal/share/zellij.go:153`. The `started` value is currently used for rollback at `service.go:127` and discarded; this task persists it to `SharingOperation.WebServerOwned` from T005.

- [X] T044 [US3] Gate the web server teardown step in `teardownLocked` on `WebServerOwned` in `cc-deck/internal/share/service.go:453`, fixing the unconditional stop
- [X] T045 [US3] Remove the endpoint stop step from `teardownLocked` in `cc-deck/internal/share/service.go:420` entirely, since cc-deck no longer owns any endpoint process

**Checkpoint**: Unshare is safe. Both folded-in teardown defects are closed.

---

## Phase 6: User Story 4 - Keep listing fast and honest (Priority: P3)

**Goal**: Listing performs no network access and reports what it actually knows.

**Independent Test**: Share, wait, then list. The listing matches an unshared listing in speed and
shows the stored result with its age.

### Tests for User Story 4

- [X] T046 [US4] Write a test in `cc-deck/internal/cmd/ws_share_test.go` asserting that listing performs zero probes and resolves sharing state once for the whole listing rather than once per row
- [X] T046a [US4] Add a benchmark or timed test in `cc-deck/internal/cmd/ws_share_test.go` that lists N shared and N unshared workspaces and asserts the shared listing takes no more than ten percent longer, which is the measurement method for SC-008; use a counting fake rather than wall-clock alone so the assertion is not flaky under load
- [X] T047 [US4] Write tests in `cc-deck/internal/cmd/ws_share_test.go` covering the three listing output shapes from `contracts/cli-contract.md`: verified with age, failed with the stage named and its age, and no age at all

### Implementation for User Story 4

- [X] T048 [US4] Render `LastProbe` in the listing sharing column in `cc-deck/internal/cmd/ws.go`, showing the age for a passed check, the failing stage plus age for a failed one, and no age when `LastProbe` is nil

  > **Interfaces**: reads `SharingStatus.LastProbe *ProbeResult` from T005, populated by T038. Renders `ProbeResult.OK`, `ProbeResult.FailedAt` (a `ProbeStage` string), and the age derived from `ProbeResult.CheckedAt`. Output shapes are fixed in `contracts/cli-contract.md`. This task can be written against a hand-constructed `LastProbe` before T038 lands.
- [X] T049 [US4] Confirm no listing code path in `cc-deck/internal/cmd/ws.go` calls `Probe`, and add a guard test in `cc-deck/internal/cmd/ws_share_test.go` that fails if one is introduced

**Checkpoint**: Listings are fast and honest about staleness.

---

## Phase 7: User Story 5 - Name and switch between endpoints (Priority: P3)

**Goal**: Multiple named endpoints with a declared default.

**Independent Test**: Configure two named endpoints and a default, share using the default, then share
another workspace naming the other endpoint explicitly.

### Tests for User Story 5

- [X] T050 [US5] Write tests in `cc-deck/internal/share/endpoint_test.go` covering the full resolution precedence: explicit flag, then named selection, then declared default, then the single address
- [X] T051 [US5] Write a test in `cc-deck/internal/share/endpoint_test.go` asserting an unknown endpoint name fails with a message listing the configured names

### Implementation for User Story 5

- [X] T052 [US5] Complete named endpoint resolution and the `--endpoint-name` selection path in `cc-deck/internal/share/endpoint.go` and `cc-deck/internal/cmd/ws_share.go`
- [X] T053 [US5] Record `EndpointName` on the operation in `cc-deck/internal/share/service.go` so status re-probes the address recorded at share time rather than the current configured value

**Checkpoint**: All five user stories are independently functional.

---

## Phase 8: Documentation, Folded Defects & Validation

**Purpose**: Constitution Principle I makes the documentation tasks completion conditions, not polish.
This phase is mandatory before the feature is considered done.

- [ ] T054 [P] Update `docs/modules/reference/pages/cli.adoc` to cover `--endpoint`, `--endpoint-name`, and `--no-verify`, per FR-038
- [ ] T055 [P] Update `docs/modules/reference/pages/configuration.adoc` to cover the redefined sharing schema and to record that `sharing.provider` is removed, per FR-039
- [ ] T056 [P] Update `README.md` to reflect that an endpoint must exist before a workspace can be shared, per FR-040
- [ ] T057 Update `docs/modules/using/pages/sharing.adoc` with the endpoint requirements from `contracts/endpoint-contract.md`, recipes for producing a conforming endpoint with common external tools, and a plain statement that cc-deck verifies reachability only from its own host and cannot detect a filter on the guest's network, per FR-041, FR-034, FR-035, and FR-036
- [ ] T058 Run `/prose:check` with the `cc-deck` voice profile over every documentation file changed in T054 through T057, per FR-042
- [X] T059 Add the missing `ZELLIJ` environment guard to `EnsureSession` in `cc-deck/internal/ws/local.go`, matching the check `Attach` already performs, so creating a workspace from inside a Zellij session no longer silently appends a tab and reports success
- [X] T060 [P] Add a test asserting `EnsureSession` refuses when `ZELLIJ` is set in `cc-deck/internal/ws/local.go`'s test file
- [ ] T061 Run `make verify` and confirm the only failures are the pre-existing ones recorded in T001
- [ ] T062 Walk `specs/087-workspace-sharing-endpoint/quickstart.md` manually, steps 1 through 7, and confirm each expected outcome

---

## Requirement Traceability

Every requirement in `spec.md` maps to at least one task. Requirements satisfied by deletion or by
construction are marked, because a task list that invents work for a negative requirement is worse
than one that states why none is needed.

| Requirements | Tasks | Notes |
|--------------|-------|-------|
| FR-001, FR-002 | T006, T007, T008, T008a through T008d | Satisfied by deletion. Once `Provider`, `Process`, and the Cloudflare implementation are gone from production *and* test code, there is no code path that can start or stop a tunnel. |
| FR-003, FR-004, FR-005 | T015, T023, T027, T052, T053 | Resolution precedence and the refusal path. |
| FR-006, FR-007, FR-008 | T011, T012, T016 through T020 | The five stages and their attribution. |
| FR-009 | T016 | The public-resolver second opinion. |
| FR-010, FR-044, FR-045, FR-046 | T014, T018 | Probe credential lifecycle, tested before implemented. |
| FR-011 | T013, T019 | Control channel only, never a terminal socket. |
| FR-012, FR-032 | T020 | One deadline for the whole probe. |
| FR-013, FR-014 | T025, T031 | Skip path leaves `LastProbe` nil. |
| FR-015 | T022, T024 | The blocking gate on both share and invite. |
| FR-016, FR-017 | T037, T046, T049 | Status probes, listing never does. |
| FR-018, FR-019 | T047, T048 | Listing output, resolved once per listing. |
| FR-020 | T049 | Satisfied by construction. Nothing schedules or polls; the guard test prevents one being added. |
| FR-021, FR-022 | T033, T034, T036 | The change that stops a listing destroying a live share. |
| FR-023, FR-047 | T038, T039 | Observation separated from state machine. |
| FR-024 | T034 | Confirmed absence remains the one teardown trigger. |
| FR-025 | T040, T041 | Revocation on unshare. |
| FR-026, FR-045 (endpoint) | T045 | Endpoint stop step removed outright. |
| FR-027 | T040, T043, T044 | Ownership recorded when cc-deck acts, not inferred later. |
| FR-028 | T041 | Unshare never ends the session. |
| FR-029 | T042 | Interrupted teardown resumes. |
| FR-030, FR-031 | T028, T029 | The workspace survives a failed share. |
| FR-033 | T012, T033 | Process liveness is never accepted as health; the degraded-tunnel row proves it. |
| FR-034 through FR-037 | T057 | Endpoint requirements, recipes, and the host-only limitation, stated plainly. |
| FR-038 through FR-042 | T054, T055, T056, T057, T058 | Constitution Principle I artifacts. |
| FR-043 | none | Satisfied by construction. No expiry field is added; the requirement is that none exists. |

### Success criteria and their measurement methods

Every success criterion needs a way to be measured, not just a task that plausibly delivers it.

| Criterion | Tasks | Measurement method |
|-----------|-------|--------------------|
| SC-001 first-attempt success | T032, T062 | End to end test plus quickstart steps 2 and 3 walked by hand. |
| SC-002 failure modes caught | T012, T032 | Each of the five failure modes fails the build independently. |
| SC-003 correct layer named | T012, T012a | The exhaustiveness assertion fails if any `ProbeStage` lacks a failure case, which is what makes "100%" measurable rather than aspirational. |
| SC-004 no command hangs | T012, T020 | The silent-endpoint matrix row asserts return within the deadline while the fake never answers. |
| SC-005 fifteen seconds, configurable | T020 | Deadline derived from `VerifyTimeout`; a shortened value in tests proves configurability. |
| SC-006 never stops what it did not start | T040, T041, T045 | Three tests: externally started web server, externally provided endpoint, workspace session. |
| SC-007 failure never revokes | T033 | Asserted across every stage failure and timeout. |
| SC-008 listing cost | T046, T046a | Zero-probe assertion plus the ten percent bound with a counting fake. |
| SC-009 staleness visible | T047, T048 | The three output shapes from `contracts/cli-contract.md`. |
| SC-010 cc-deck commands only | T062 | Quickstart is written entirely in `cc-deck` commands; walking it is the check. |
| SC-011 no workspace lost | T029 | Failed share leaves the workspace running. |
| SC-012 documentation shipped | T054 through T058, T061 | Five named artifacts plus the voice check. |
| SC-013 no credential survives | T014, T014a | Lifecycle test plus the sentinel-secret leak test. |

| Other | Tasks | Notes |
|-------|-------|-------|
| Folded defects | T028, T044, T059 | The `KillSession` on failed share, the unconditional web server stop, and the missing `ZELLIJ` guard. |

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies
- **Foundational (Phase 2)**: depends on Setup; blocks every user story. T005 through T008d are one
  atomic non-compiling window covering production *and* test code; the phase is not done until the
  package compiles again
- **US1 (Phase 3)**: depends on Foundational; blocks US2 and US4 because both consume `LastProbe`
- **US2 (Phase 4)**: depends on US1
- **US3 (Phase 5)**: depends on Foundational only; genuinely parallel with US1 and US2
- **US4 (Phase 6)**: depends on US1 for `LastProbe` being populated
- **US5 (Phase 7)**: depends on US1 for the resolution scaffolding in `endpoint.go`
- **Phase 8**: depends on all stories being complete

### Story Dependency Notes

US3 is the exception worth exploiting. Web server ownership and teardown touch `teardownLocked` and
nothing the probe owns, so a second person can take Phase 5 the moment Phase 2 lands without waiting
for the probe.

US4's dependency on US1 is data, not code. The rendering can be written against a hand-constructed
`LastProbe` before the probe exists.

### Within Each Story

Tests before implementation. The stage matrix in T011 and T012 must exist and fail before T016
through T020 are written; the upgrade-refusing fake in particular should be red before the stage that
catches it is written.

### Parallel Opportunities

`[P]` means a different file **and** no dependency. Most of this feature's test tasks cluster into a
handful of files, so genuine parallelism is narrower than the task count suggests. Honest list:

| Tasks | Why parallel |
|-------|--------------|
| T003, T004 | Different packages: `internal/share` and `internal/config` |
| T010 | `internal/config` tests, independent of the `share` window |
| T013 | The only Phase 3 task touching `provider_contract_test.go` |
| T054, T055, T056 | Three separate documentation files |
| T060 | `internal/ws` tests, untouched by everything else |

**Not parallel despite being conceptually independent**, because each cluster shares one file:
T011/T012/T012a/T014/T014a in `probe_test.go`; T029/T030/T031 and T046/T046a/T047 in
`ws_share_test.go`; T033/T034/T035 and T040/T041/T042 in `service_test.go`; T050/T051 in
`endpoint_test.go`. Dispatching these to parallel agents produces edit conflicts, not speed.

The real parallelism in this feature is at the story level, not the task level. See the note on US3
above.

---

## Parallel Example: Story-Level Split

Task-level parallelism is limited here, so the useful split is by story once Phase 2 lands:

```bash
# Developer A: the probe and the sharing gate
Phase 3 (US1), T011 through T032, sequential within probe_test.go

# Developer B: teardown ownership, touches only teardownLocked
Phase 5 (US3), T040 through T045

# Developer C: documentation, three independent files
Phase 8, T054 through T057
```

US3 and the documentation work depend on Foundational only, so neither waits for the probe.

---

## Implementation Strategy

### MVP: User Story 1 only

1. Phase 1 Setup
2. Phase 2 Foundational, treating T005 through T008 as one atomic non-compiling window
3. Phase 3 User Story 1
4. **STOP and VALIDATE**: run T032, then walk quickstart steps 2 and 3 by hand

At that point sharing works and the reported defect is caught automatically. That alone is worth
shipping, because it is the first time the feature will have worked for a human.

### Incremental delivery

Add US2 next, since diagnosis without destruction is what makes the feature trustworthy in daily use,
and it closes the defect where a single listing revoked live credentials. Then US3 to stop the
collateral damage on unshare. US4 and US5 are quality of life and can follow at any point.

### Risk sequencing note

Phase 4's T036 is the highest regression risk in the feature, because it changes what `Status` does
on the most common failure path. It deliberately follows a proven probe rather than preceding it, so
that when teardown is severed there is already a trustworthy signal to replace it.

---

## Notes

- `[P]` means different files and no dependency on incomplete work
- Never run `go build` or `cargo build`; use `make test`, `make lint`, `make install`
- Every session-creating Zellij command in a test runs under `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID`, or it will silently add tabs to the developer's live session and report success
- `ZellijCLI.CreateToken` ignores its label argument because `zellij web --create-token` rejects being combined with `--token-name`, so revoke by the name Zellij returns, never by one you chose
- Commit after each task or logical group
