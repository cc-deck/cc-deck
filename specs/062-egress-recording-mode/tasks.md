# Tasks: Egress Recording Mode

**Input**: Design documents from `specs/062-egress-recording-mode/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: New package structure and shared infrastructure

- [X] T001 Create `cc-deck/internal/record/` package directory with `doc.go`
- [X] T002 [P] Add Podman pod operations (`PodCreate`, `PodRemove`, `PodExists`) in `cc-deck/internal/podman/pod.go`
- [X] T003 [P] Add unit tests for pod operations in `cc-deck/internal/podman/pod_test.go`
- [X] T004 [P] Add `SaveManifest()` function to `cc-deck/internal/build/manifest.go` that marshals the Manifest struct and writes to the given path
- [X] T005 [P] Add unit test for `SaveManifest()` round-trip (load, modify `allowed_domains`, save, reload, verify) in `cc-deck/internal/build/manifest_test.go`

**Checkpoint**: Pod primitives and manifest write-back are available for user story implementation.

---

## Phase 2: Foundational (DNS Log Parsing + Noise Filtering)

**Purpose**: Core data processing that all user stories depend on. Must be complete before story work begins.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Implement `ParseDNSLog()` in `cc-deck/internal/record/dns.go`. Parse CoreDNS log format, extract domain names and query types (A, AAAA, CNAME). Strip trailing dots. Return `[]DNSLogEntry` per data-model.md.
- [X] T007 [P] Implement `DeduplicateDomains()` in `cc-deck/internal/record/dns.go`. Case-insensitive dedup, return sorted unique domain list.
- [X] T008 [P] Implement `FilterNoise()` in `cc-deck/internal/record/dns.go`. Hardcoded deny-list: suffixes `.local`, `.internal`, `.podman`, `.localhost`; exact match `localhost`; reverse DNS patterns `*.in-addr.arpa`, `*.ip6.arpa`; AAAA-only queries (no corresponding A record).
- [X] T009 Add unit tests for `ParseDNSLog`, `DeduplicateDomains`, `FilterNoise` in `cc-deck/internal/record/dns_test.go`. Use hardcoded CoreDNS log samples including noise domains, duplicates, and AAAA-only entries.

**Checkpoint**: DNS log parsing and filtering are tested and ready. All downstream stories can use these functions.

---

## Phase 3: User Story 1 - Record Egress for a New Project (Priority: P1)

**Goal**: A user runs `cc-deck build record`, works interactively in a Podman pod with a DNS sidecar, and on exit sees which domains were accessed. New domains are appended to `build.yaml` `network.allowed_domains`.

**Independent Test**: Run `cc-deck build record` on a project with known dependencies (e.g., Python with pip). Verify DNS log captures expected domains (`pypi.org`, `files.pythonhosted.org`). Verify they are added to `build.yaml`.

### Implementation for User Story 1

- [X] T010 [US1] Implement CoreDNS Corefile generation in `cc-deck/internal/record/record.go`. Generate a minimal Corefile string that enables `forward` (to upstream DNS) and `log` (all queries to stdout or file). Include a constant for the CoreDNS sidecar image reference.
- [X] T011 [US1] Implement `RunRecordingSession()` orchestrator in `cc-deck/internal/record/record.go`. This is the largest task, implementing the full session lifecycle as a single function with deferred cleanup. Steps within the function: (1) validate image exists via `podman image inspect` (error with guidance if not found), (2) create temp dir and write Corefile, (3) create Podman volume for DNS log, (4) create Podman pod with `--dns 127.0.0.1`, (5) start CoreDNS sidecar container in pod with Corefile bind-mount and log volume, (6) start workspace container in pod with log volume, (7) attach user interactively via `podman attach`, (8) on exit copy DNS log from volume to host, (9) call `ParseDNSLog` + `DeduplicateDomains` + `FilterNoise`, (10) clean up pod and volume via deferred `PodRemove`. Register SIGINT/SIGTERM handler that triggers the same deferred cleanup.
- [X] T012 [US1] Implement manifest update logic in `cc-deck/internal/record/record.go`. After processing: load manifest, initialize `Network` field if nil, append new domains to `AllowedDomains` (deduplicating against existing entries), call `SaveManifest()`.
- [X] T013 [US1] Implement summary report output in `cc-deck/internal/record/record.go`. Print to stdout: total domains observed, domains filtered as noise, new domains added to `allowed_domains`, path to modified `build.yaml`, reminder to run `cc-deck build refresh`.
- [X] T014 [US1] Register `build record` subcommand in `cc-deck/internal/cmd/build.go`. Add `newBuildRecordCmd()` that accepts optional `[dir]` argument (defaults to `.cc-deck/setup/`), loads manifest, resolves image ref via `OpenShellImageRef()`, calls `record.RunRecordingSession()`.
- [X] T015 [US1] Add unit test for Corefile generation and manifest update logic in `cc-deck/internal/record/record_test.go`. Test: Corefile contains `forward` and `log` directives. Test: manifest round-trip with `allowed_domains` append and dedup.

**Checkpoint**: User Story 1 is functional. A user can run `cc-deck build record`, work in a recording session, exit, and see new domains added to their manifest.

---

## Phase 4: User Story 2 - Catalog Reverse Matching (Priority: P2)

**Goal**: After recording, observed domains are matched against existing catalog components. Domains already covered by the catalog are reported separately from truly new ones.

**Independent Test**: Run a recording session on a project with Python and Go tools. Verify output shows `pypi.org` as "covered by python catalog component" and unknown domains as "new, added to allowed_domains."

### Implementation for User Story 2

- [X] T016 [US2] Implement `BuildDomainIndex()` in `cc-deck/internal/record/catalog.go`. Load all catalog components (embedded, cached, user-local) using existing `LoadEmbeddedComponents()` and `LoadComponentTier()`. Build a map from endpoint host to component name for reverse lookup.
- [X] T017 [US2] Implement `MatchAgainstCatalog()` in `cc-deck/internal/record/catalog.go`. Accept a list of observed domains and the domain index. Return a `RecordingResult` (per data-model.md) with `CoveredDomains` and `NewDomains` separated. Also check existing `allowed_domains` from the manifest.
- [X] T018 [US2] Integrate catalog matching into `RunRecordingSession()` in `cc-deck/internal/record/record.go`. After parsing and filtering, call `MatchAgainstCatalog()` before manifest update. Only append `NewDomains` to `allowed_domains`. Update summary report to show covered vs. new breakdown.
- [X] T019 [P] [US2] Add unit tests for `BuildDomainIndex` and `MatchAgainstCatalog` in `cc-deck/internal/record/catalog_test.go`. Test: domains matching embedded components are classified as covered. Test: unknown domains are classified as new. Test: domains already in `allowed_domains` are classified as covered.

**Checkpoint**: User Stories 1 and 2 are both functional. Recording sessions now show catalog-aware output.

---

## Phase 5: User Story 3 - Integrate Recorded Domains into Build Pipeline (Priority: P2)

**Goal**: Recorded domains in `allowed_domains` are picked up by `cc-deck build refresh` and assembled into the OpenShell policy.

**Independent Test**: Run a recording session, then run `cc-deck build refresh`, then inspect `openshell/policy.yaml` and verify it contains network policies for the recorded domains.

### Implementation for User Story 3

- [X] T020 [US3] Verification task (no new production code): confirm that `AssemblePolicyWithOptions()` in `cc-deck/internal/build/policy.go` correctly handles domains added to `allowed_domains` by the recording feature. The existing code at lines 140-173 already reads `manifest.Network.AllowedDomains` and generates network policies. Write a focused integration test in `cc-deck/internal/build/policy_test.go` that loads a manifest with recording-style `allowed_domains` entries, runs policy assembly, and verifies the output policy contains the expected network policy entries with port 443.

**Checkpoint**: End-to-end flow works: record -> manifest updated -> refresh picks up new domains -> policy includes them.

---

## Phase 6: User Story 4 - Noise Filtering (Priority: P3)

**Goal**: Infrastructure noise (Podman DNS, mDNS, localhost, reverse DNS) is filtered from recording output with zero false positives.

**Independent Test**: Run a recording session and verify that known noise domains (`dns.podman`, `localhost`, `_dnssd._udp.local`) are excluded while legitimate domains are preserved.

### Implementation for User Story 4

- [X] T021 [US4] Extend `FilterNoise()` test coverage in `cc-deck/internal/record/dns_test.go`. Add test cases for: container registry domains used during image pull (e.g., `registry-1.docker.io`), mDNS service discovery (`_dnssd._udp.local`), duplicate A+AAAA queries for same domain (verify single output), domain that looks like noise but is legitimate (e.g., `api.internal.company.com` should NOT be filtered since `.internal` suffix is in deny-list; document this known limitation).
- [X] T022 [US4] Review and refine the deny-list in `FilterNoise()` based on test findings. If `.internal` suffix causes false positives for corporate domains, consider narrowing to `.podman.internal` only. Update `cc-deck/internal/record/dns.go` and spec FR-009 if deny-list changes.

**Checkpoint**: All four user stories are functional. Noise filtering is validated.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, code quality, and final validation.

- [X] T023 [P] Add `build record` to CLI reference in `docs/modules/reference/pages/cli.adoc`. Document: subcommand usage, arguments, flags, example output.
- [X] T024 [P] Create guide page `docs/modules/using/pages/egress-recording.adoc`. Cover: when to use recording mode, prerequisites, step-by-step walkthrough, interpreting output, next steps after recording. Use prose plugin with cc-deck voice profile.
- [X] T025 [P] Update `docs/modules/reference/pages/configuration.adoc` if `network.allowed_domains` is not already documented. Ensure the field is covered with a description of how recording populates it.
- [X] T026 [P] Update `README.md` with a mention of `build record` in the feature list or commands section.
- [X] T027 Run `make lint` and `make test` to verify all new code passes. Fix any issues.

- [X] T028 Run quickstart.md validation: manually verify the workflow described in `specs/062-egress-recording-mode/quickstart.md` works end-to-end.
- [X] T029 Validate NFR timing targets. During end-to-end testing: (1) measure pod creation overhead (SC-002: must be < 30s beyond normal startup), time from `build record` invocation to interactive prompt. (2) measure post-session processing time (SC-003: must be < 10s for up to 500 domains), time from session exit to summary report output. Report results; no automated benchmark needed, manual timing during T028 is sufficient.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on T001 (package dir). T002-T005 from Phase 1 can run in parallel with Phase 2.
- **User Story 1 (Phase 3)**: Depends on Phase 1 (pod ops, manifest save) and Phase 2 (DNS parsing)
- **User Story 2 (Phase 4)**: Depends on User Story 1 (uses `RunRecordingSession` as integration point)
- **User Story 3 (Phase 5)**: Can start after Phase 1 (only needs manifest and policy assembly, no recording code)
- **User Story 4 (Phase 6)**: Can start after Phase 2 (extends existing `FilterNoise` tests)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Phase 1 + Phase 2. No dependencies on other stories.
- **US2 (P2)**: Depends on US1 (integrates into `RunRecordingSession`).
- **US3 (P2)**: Independent of US1/US2. Only needs manifest + policy assembly code.
- **US4 (P3)**: Independent of US1/US2/US3. Extends foundational `FilterNoise`.

### Within Each User Story

- Core logic before integration
- Integration before CLI wiring
- Tests alongside implementation

### Parallel Opportunities

- Phase 1: T002, T003, T004, T005 all touch different files, run in parallel
- Phase 2: T007, T008 can run in parallel (both in dns.go but independent functions)
- Phase 3-6: US3 and US4 can run in parallel with US1/US2
- Phase 7: T023, T024, T025, T026 all touch different files, run in parallel

---

## Parallel Example: Phase 1

```bash
# Launch all setup tasks together (different files):
Task: "Add pod operations in cc-deck/internal/podman/pod.go"
Task: "Add pod tests in cc-deck/internal/podman/pod_test.go"
Task: "Add SaveManifest in cc-deck/internal/build/manifest.go"
Task: "Add SaveManifest test in cc-deck/internal/build/manifest_test.go"
```

## Parallel Example: Phase 7

```bash
# Launch all documentation tasks together (different files):
Task: "CLI reference in docs/modules/reference/pages/cli.adoc"
Task: "Guide page in docs/modules/using/pages/egress-recording.adoc"
Task: "Config reference in docs/modules/reference/pages/configuration.adoc"
Task: "README update in README.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (pod ops, manifest save)
2. Complete Phase 2: Foundational (DNS parsing, filtering)
3. Complete Phase 3: User Story 1 (recording session, manifest update)
4. **STOP and VALIDATE**: Run `cc-deck build record` on a real project, verify domains are captured and manifest is updated
5. This is a usable, shippable increment

### Incremental Delivery

1. Setup + Foundational -> foundation ready
2. Add US1 -> test independently -> functional recording (MVP)
3. Add US2 -> test independently -> catalog-aware output
4. Add US3 -> test independently -> verified pipeline integration
5. Add US4 -> test independently -> noise filtering validated
6. Polish -> docs, lint, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All tests use `make test` (never `go test` directly, per constitution)
- All builds use `make install` (never `go build` directly)
- Container operations use Podman exclusively (never Docker)
- Documentation uses prose plugin with cc-deck voice profile
- Commit after each task or logical group
