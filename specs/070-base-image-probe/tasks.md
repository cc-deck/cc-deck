# Tasks: Base Image Probe

**Input**: Design documents from `specs/070-base-image-probe/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the imageprobe package structure and shared data types

- [ ] T001 Create `cc-deck/internal/build/imageprobe/` package directory
- [ ] T002 [P] Define ProbeResult, OSInfo, ToolInfo, UserInfo structs in `cc-deck/internal/build/imageprobe/types.go` per data-model.md
- [ ] T003 [P] Define default tool set (30 tools from base-image/scripts/install-tools.sh) and merge logic for manifest `probe_tools:` override in `cc-deck/internal/build/imageprobe/tools.go`
- [ ] T004 [P] Implement semver-like version parsing and major.minor compatibility comparison in `cc-deck/internal/build/imageprobe/version.go` (same major, installed minor >= required minor)
- [ ] T005 [P] Add `ProbeTools` field to Manifest struct (optional `probe_tools:` YAML key) in `cc-deck/internal/build/manifest.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Probe script generation and output parsing, needed by all user stories

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T006 Implement probe shell script generator in `cc-deck/internal/build/imageprobe/probe.go`: generate a POSIX shell script that detects OS info (/etc/os-release), package manager (dnf/apt-get/apk/yum), tool presence+version, user info, and shell availability. Output is JSON-per-line per contracts/probe-script.md
- [ ] T007 Implement probe output parser in `cc-deck/internal/build/imageprobe/parse.go`: parse JSON-per-line stdout into a ProbeResult struct. Silently skip non-JSON lines. Extract semver from version output strings using regex
- [ ] T008 Implement probe executor in `cc-deck/internal/build/imageprobe/probe.go`: RunProbe function that calls `podman run --rm --entrypoint /bin/sh <image>` with the generated script piped via stdin, enforces timeout via context.WithTimeout, returns parsed ProbeResult or error
- [ ] T009 [P] Write unit tests for probe script generation in `cc-deck/internal/build/imageprobe/probe_test.go`: verify script contains OS detection, package manager detection, tool checks for default set + manifest tools
- [ ] T010 [P] Write unit tests for output parsing in `cc-deck/internal/build/imageprobe/parse_test.go`: verify parsing of all JSON line types (os, pkgmgr, tool, user, shells), handling of non-JSON noise, version extraction from various `--version` output formats
- [ ] T011 [P] Write unit tests for version comparison in `cc-deck/internal/build/imageprobe/version_test.go`: same major + newer minor = compatible, different major = incompatible, older minor = incompatible, missing patch = 0, unparseable version = assume compatible

**Checkpoint**: Core probe infrastructure ready. Probe can generate scripts, execute them, and parse results.

---

## Phase 3: User Story 1 - Build adapts to a new base image automatically (Priority: P1) MVP

**Goal**: Switching the manifest's base image produces a working build on the first attempt. The generated Containerfile uses the correct package manager and skips pre-installed tools.

**Independent Test**: Run `/cc-deck.build` against two different base images (Fedora 41, UBI 9) with the same manifest. Both produce working images with correct tools installed. Fedora build skips tools already in the base; UBI build installs missing ones.

### Implementation for User Story 1

- [ ] T012 [US1] Implement ToolDiff in `cc-deck/internal/build/imageprobe/diff.go`: compare ProbeResult.Tools against manifest tools using version compatibility. Return []ToolDiff with status present/missing/incompatible and install method
- [ ] T013 [US1] Write unit tests for ToolDiff in `cc-deck/internal/build/imageprobe/diff_test.go`: verify present (skip), missing (install), incompatible (shadow), manifest-only tools, default-set-only tools
- [ ] T014 [US1] Update the build skill `cc-deck/internal/build/commands/cc-deck.build.md` Section A2 (Container Build): add a probe step before Containerfile generation that runs `cc-deck build probe <base-image> --setup-dir <setup-dir> --format json`, parses the JSON output, and uses the probed package manager for install commands instead of assuming dnf. On probe failure (exit code 1), fall back to assuming Fedora/dnf per FR-010
- [ ] T015 [US1] Update the build skill `cc-deck/internal/build/commands/cc-deck.build.md` Section C2 (OpenShell Build): replace the ad-hoc `podman run` probing with a call to `cc-deck build probe <base-image> --setup-dir <setup-dir> --format json` and use the structured results for package manager selection and tool skip logic. On probe failure (exit code 1), fall back to assuming Debian/apt-get per FR-010
- [ ] T016 [US1] Update Section A2 and C2 Containerfile generation logic to use ToolDiff results: skip tools with status "present", install tools with status "missing" using the probed package manager, shadow tools with status "incompatible" to `/usr/local/bin`

**Checkpoint**: Building against different base images (Fedora, UBI, OpenShell/Debian) produces correct Containerfiles with the right package manager and no redundant installs.

---

## Phase 4: User Story 2 - Probe results caching (Priority: P2)

**Goal**: Probe results are cached by image reference + digest. Repeat builds skip the probe and use cached results. Cache invalidates when the image digest changes.

**Independent Test**: Run a build twice against the same base image. First build probes (takes seconds). Second build uses cache (sub-second). Change the base image tag, run again: re-probes.

### Implementation for User Story 2

- [ ] T017 [US2] Implement ProbeCache read/write in `cc-deck/internal/build/imageprobe/cache.go`: LoadCache (read probe-cache.json), SaveCache (write), LookupCache (check entry by ref + digest match), StoreResult (add/update entry)
- [ ] T018 [US2] Implement image digest resolution in `cc-deck/internal/build/imageprobe/probe.go`: ResolveDigest function that runs `podman inspect --format {{.Digest}} <image>` (pull first if not local), returns digest string
- [ ] T019 [US2] Integrate caching into RunProbe flow: check cache before executing probe, store results after successful probe, return cached flag in output
- [ ] T020 [P] [US2] Write unit tests for cache operations in `cc-deck/internal/build/imageprobe/cache_test.go`: verify load/save round-trip, cache hit on matching digest, cache miss on different digest, cache miss on missing entry, --no-cache bypass

**Checkpoint**: Repeat builds against unchanged base images skip the probe step with sub-second overhead.

---

## Phase 5: User Story 3 - Probe report shows base image capabilities (Priority: P3)

**Goal**: A standalone `cc-deck build probe` command that prints a summary of what the base image provides. When a manifest is available, it also shows a diff of required vs installed tools.

**Independent Test**: Run `cc-deck build probe registry.fedoraproject.org/fedora:41` and verify the table output shows OS, package manager, tools with versions. Run with `--format json` and verify JSON schema matches contract.

### Implementation for User Story 3

- [ ] T021 [US3] Implement table output formatter in `cc-deck/internal/build/imageprobe/format.go`: FormatTable function that renders ProbeResult as a human-readable summary (OS, package manager, user, shells, tool list with checkmarks) per contracts/probe-cli.md
- [ ] T022 [US3] Implement diff output formatter in `cc-deck/internal/build/imageprobe/format.go`: FormatDiff function that appends a tool diff section (present/skip, missing/install, incompatible/shadow) when manifest tools are available
- [ ] T023 [US3] Implement JSON output formatter in `cc-deck/internal/build/imageprobe/format.go`: FormatJSON function that marshals ProbeResult with a `cached` boolean field
- [ ] T024 [US3] Create `cc-deck build probe` CLI subcommand in `cc-deck/cmd/build_probe.go`: register under `build` parent command with flags (--setup-dir, --format, --no-cache, --timeout), wire to RunProbe + formatters, exit code 0 on success / 1 on failure
- [ ] T025 [US3] Wire the `build probe` subcommand into `cc-deck/cmd/build.go` parent command registration
- [ ] T026 [P] [US3] Write unit tests for table and JSON formatters in `cc-deck/internal/build/imageprobe/format_test.go`: verify table output includes OS info, tool list, checkmarks; JSON output matches schema; diff section shows correct statuses

**Checkpoint**: `cc-deck build probe <image>` works standalone with both table and JSON output.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, integration testing, and cleanup

- [ ] T027 [P] Write integration test in `cc-deck/internal/build/imageprobe/integration_test.go`: probe a real Fedora 41 image via podman, verify OS=fedora, package_manager=dnf, git is present with a version. Guard with build tag `//go:build integration` so `make test` skips it
- [ ] T028 [P] Create guide page `docs/modules/guide/pages/base-image-probe.adoc`: explain what the probe does, how caching works, how to customize with `probe_tools:`, examples with different base images. Use prose plugin with cc-deck voice profile
- [ ] T029 [P] Update CLI reference `docs/modules/reference/pages/cli.adoc`: add `build probe` command with all flags and output formats
- [ ] T030 [P] Update configuration reference `docs/modules/reference/pages/configuration.adoc`: document `probe_tools:` manifest key and `probe-cache.json` file location
- [ ] T031 Update README.md with base image probe feature summary
- [ ] T032 Run `make test` and `make lint` to verify all tests pass and no lint issues

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion (T001-T005), BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 completion. This is the MVP.
- **US2 (Phase 4)**: Depends on Phase 2 completion. Can run in parallel with US1 (different files)
- **US3 (Phase 5)**: Depends on Phase 2 completion. Can run in parallel with US1/US2 (different files)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational only. Core feature, MVP scope.
- **US2 (P2)**: Depends on Foundational only. Independent from US1 (cache.go is a separate file). US1 integration (T014-T016) benefits from cache but works without it.
- **US3 (P3)**: Depends on Foundational only. Independent from US1/US2 (format.go and cmd/ are separate files).

### Within Each User Story

- Implementation tasks before integration tasks
- Core logic before CLI wiring
- Tests alongside implementation (same phase)

### Parallel Opportunities

Within Phase 1:
- T002, T003, T004, T005 are all independent files, can run in parallel

Within Phase 2:
- T009, T010, T011 are independent test files, can run in parallel after T006-T008

Across Phases 3-5 (after Phase 2 completes):
- US1, US2, US3 can all start in parallel (different files: diff.go, cache.go, format.go, cmd/)

Within Phase 6:
- T027, T028, T029, T030 are independent files, can run in parallel

---

## Parallel Example: User Story 1

```bash
# After Phase 2 is complete, launch US1 implementation:
Task: "Implement ToolDiff in cc-deck/internal/build/imageprobe/diff.go"
Task: "Write unit tests for ToolDiff in cc-deck/internal/build/imageprobe/diff_test.go"
# Then sequentially:
Task: "Update build skill Section A2"
Task: "Update build skill Section C2"
Task: "Update Containerfile generation logic"
```

## Parallel Example: All User Stories (after Phase 2)

```bash
# These can all start simultaneously:
Agent A: US1 - diff.go + build skill integration
Agent B: US2 - cache.go + digest resolution
Agent C: US3 - format.go + CLI subcommand
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: Foundational (T006-T011)
3. Complete Phase 3: User Story 1 (T012-T016)
4. **STOP and VALIDATE**: Build against Fedora 41 and UBI 9 with the same manifest
5. Verify correct package manager and no redundant installs

### Incremental Delivery

1. Setup + Foundational -> probe infrastructure ready
2. Add US1 -> builds adapt to base images -> MVP
3. Add US2 -> repeat builds are fast -> improved DX
4. Add US3 -> standalone probe command -> visibility
5. Polish -> docs, integration tests -> production ready

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Constitution requires: tests, README update, CLI reference, guide page, config reference
- All documentation uses prose plugin with cc-deck voice profile
- Use `make test` and `make lint`, never `go build` directly
- Container runtime: podman exclusively
