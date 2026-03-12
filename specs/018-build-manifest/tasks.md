# Tasks: cc-deck Build Pipeline

**Input**: Design documents from `/specs/018-build-manifest/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/, quickstart.md

**Tests**: No automated tests requested. Verification via CLI smoke tests.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the build package structure and shared utilities.

- [x] T001 (cc-mux-yuq.1) Create `internal/build/` package directory
- [x] T002 (cc-mux-yuq.2) [P] Create `internal/build/manifest.go` with Manifest struct and YAML field tags matching the `cc-deck-build.yaml` schema (image, tools, sources, plugins, mcp, github_tools, settings sections)
- [x] T003 (cc-mux-yuq.3) [P] Create `internal/build/runtime.go` with `DetectRuntime() string` function that checks for podman first, then docker, returns the binary name or error

**Checkpoint**: Build package exists with manifest model and runtime detection.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Embed command and script assets into the Go binary.

**⚠️ CRITICAL**: The init command extracts these embedded files.

- [x] T004 (cc-mux-dwj.1) Create `internal/build/commands/` directory with the 5 Claude Code command files: `cc-deck.extract.md`, `cc-deck.plugin.md`, `cc-deck.mcp.md`, `cc-deck.containerfile.md`, `cc-deck.publish.md`
- [x] T005 (cc-mux-dwj.2) [P] Create `internal/build/scripts/` directory with helper scripts: `validate-manifest.sh`, `update-manifest.sh`
- [x] T006 (cc-mux-dwj.3) Create `internal/build/embed.go` with `go:embed` directives for `commands/*` and `scripts/*` directories, plus accessor functions
- [x] T007 (cc-mux-dwj.4) [P] Create `internal/build/templates/` with `cc-deck-build.yaml.tmpl` scaffold template (commented examples for each section)

**Checkpoint**: All embeddable assets exist. `go build` succeeds with embedded assets.

---

## Phase 3: User Story 1 - Initialize a Build Directory (Priority: P1) 🎯 MVP

**Goal**: `cc-deck build init <dir>` creates a scaffolded build directory with manifest, commands, scripts, and gitignore.

**Independent Test**: Run `cc-deck build init /tmp/test-build`, verify directory structure, manifest validity, and command availability.

### Implementation for User Story 1

- [x] T008 (cc-mux-t9z.1) [US1] Create `internal/build/init.go` with `InitBuildDir(dir string, force bool) error` that creates directory, extracts manifest template, commands, scripts, and generates `.gitignore`
- [x] T009 (cc-mux-t9z.2) [US1] Create `internal/cmd/build.go` with cobra parent command `build` and subcommand `init` (args: `<dir>`, flags: `--force`)
- [x] T010 (cc-mux-t9z.3) [US1] Register the `build` command in `cmd/cc-deck/main.go`
- [x] T011 (cc-mux-t9z.4) [US1] Verify: run `cc-deck build init /tmp/test-build`, confirm directory structure matches plan.md layout, manifest is valid YAML, commands are extracted

**Checkpoint**: `cc-deck build init` works. Build directory is ready for AI-driven population.

---

## Phase 4: User Story 2 - Analyze Repositories for Tool Dependencies (Priority: P1)

**Goal**: `/cc-deck.extract` analyzes repos and populates the manifest tools and sources sections.

**Independent Test**: Run `/cc-deck.extract` on a Go project, verify manifest is updated with Go version and source provenance.

### Implementation for User Story 2

- [x] T012 (cc-mux-kn5.1) [US2] Write `cc-deck/internal/build/commands/cc-deck.extract.md` command file: YAML frontmatter + numbered steps for repository analysis, tool discovery, deduplication, conflict resolution, and manifest update
- [x] T013 (cc-mux-kn5.2) [US2] Write `cc-deck/internal/build/scripts/update-manifest.sh` helper script for safe YAML section updates (tools, sources) without LLM touching raw YAML structure
- [ ] T014 (cc-mux-kn5.3) [US2] Verify: init a build dir, open in Claude Code, run `/cc-deck.extract` on the cc-deck Go project itself, confirm tools (Go, Rust) are detected and written to manifest

**Checkpoint**: Repository analysis works for Go and Rust projects.

---

## Phase 5: User Story 3 - Generate a Containerfile from the Manifest (Priority: P1)

**Goal**: `/cc-deck.containerfile` generates a valid, build-ready Containerfile from the manifest.

**Independent Test**: Populate manifest with known tools, run `/cc-deck.containerfile`, verify Containerfile builds successfully.

### Implementation for User Story 3

- [x] T015 (cc-mux-ho5.1) [US3] Write `cc-deck/internal/build/commands/cc-deck.containerfile.md` command file: reads manifest, resolves free-form tools to install commands, generates layered Containerfile with DO NOT EDIT header, cc-deck self-embedding, Claude Code install, and github_tools downloads
- [x] T016 (cc-mux-ho5.2) [US3] Add manifest validation to `cc-deck/internal/build/manifest.go`: `Validate() error` method checking required fields (version, image.name) and YAML structure
- [ ] T017 (cc-mux-ho5.3) [US3] Verify: populate a manifest manually, run `/cc-deck.containerfile`, confirm generated Containerfile is syntactically valid and includes all manifest entries

**Checkpoint**: Containerfile generation works for standard tool combinations.

---

## Phase 6: User Story 4 - Build and Push the Container Image (Priority: P2)

**Goal**: `cc-deck build <dir>` builds the image, `cc-deck push <dir>` publishes it.

**Independent Test**: Given a valid Containerfile, run `cc-deck build`, verify image is produced with correct name/tag.

### Implementation for User Story 4

- [x] T018 (cc-mux-6ed.1) [US4] Add `build` subcommand logic in `cc-deck/internal/cmd/build.go`: validate manifest, verify Containerfile exists, copy cc-deck binary to `.build-context/`, invoke container runtime with correct tags
- [x] T019 (cc-mux-6ed.2) [US4] Add `push` subcommand in `cc-deck/internal/cmd/build.go`: read image name/tag from manifest, invoke container runtime push
- [x] T020 (cc-mux-6ed.3) [US4] Add cc-deck self-embedding logic: `os.Executable()` to find own binary, copy to `.build-context/cc-deck`, add `.build-context/` to `.gitignore`
- [ ] T021 (cc-mux-6ed.4) [US4] Verify: init a build dir, generate a Containerfile (manually or via AI), run `cc-deck build`, confirm image is created with correct name:tag

**Checkpoint**: Build and push commands work end-to-end.

---

## Phase 7: User Story 5 - Add Plugins and MCP Servers (Priority: P2)

**Goal**: `/cc-deck.plugin` and `/cc-deck.mcp` add entries to the manifest.

**Independent Test**: Run `/cc-deck.plugin` to add a plugin, verify manifest is updated.

### Implementation for User Story 5

- [x] T022 (cc-mux-a31.1) [P] [US5] Write `cc-deck/internal/build/commands/cc-deck.plugin.md` command file: list current plugins, add from marketplace or git URL, update manifest
- [x] T023 (cc-mux-a31.2) [P] [US5] Write `cc-deck/internal/build/commands/cc-deck.mcp.md` command file: accept image reference, read `cc-deck.mcp/*` labels via container runtime inspect, auto-populate MCP entry, handle missing labels interactively
- [ ] T024 (cc-mux-a31.3) [US5] Verify: run `/cc-deck.plugin` to add "sdd", run `/cc-deck.mcp` with an image reference, confirm manifest has both entries

**Checkpoint**: Plugin and MCP management works through AI commands.

---

## Phase 8: User Story 6 - Verify and Compare Builds (Priority: P3)

**Goal**: `cc-deck build verify` smoke-tests the image, `cc-deck build diff` shows changes.

**Independent Test**: Build an image, run verify, confirm pass/fail report.

### Implementation for User Story 6

- [x] T025 (cc-mux-9gm.1) [US6] Add `verify` subcommand in `cc-deck/internal/cmd/build.go`: start container from built image, check tool availability, Claude Code startup, cc-deck version, report pass/fail
- [x] T026 (cc-mux-9gm.2) [US6] Add `diff` subcommand in `cc-deck/internal/cmd/build.go`: compare manifest tools/plugins/mcp against last generated Containerfile, report added/removed/changed
- [ ] T027 (cc-mux-9gm.3) [US6] Verify: build an image, run `cc-deck build verify`, confirm it reports all tools present

**Checkpoint**: Verify and diff commands provide build confidence.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Publish command, documentation, cleanup.

- [x] T028 (cc-mux-e7g.1) [P] Write `cc-deck/internal/build/commands/cc-deck.publish.md` command file: convenience wrapper that runs `cc-deck build` then `cc-deck push`
- [x] T029 (cc-mux-e7g.2) [P] Write `cc-deck/internal/build/scripts/validate-manifest.sh` helper that validates manifest YAML schema (called by commands before updates)
- [x] T030 (cc-mux-e7g.3) Update `cc-deck/internal/build/templates/cc-deck-build.yaml.tmpl` with final commented examples reflecting all sections from data-model.md
- [x] T031 (cc-mux-e7g.4) Add `--install-zellij` flag to `cc-deck plugin install` in `cc-deck/internal/cmd/plugin.go` for downloading and installing the matching Zellij binary inside containers

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs package directory)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (needs embedded assets)
- **User Story 2 (Phase 4)**: Depends on US1 (needs initialized build directory)
- **User Story 3 (Phase 5)**: Depends on US2 (needs populated manifest)
- **User Story 4 (Phase 6)**: Depends on US3 (needs Containerfile)
- **User Story 5 (Phase 7)**: Depends on US1 (needs build directory), independent of US2-4
- **User Story 6 (Phase 8)**: Depends on US4 (needs built image)
- **Polish (Phase 9)**: Can start after US1

### User Story Dependencies

- **US1 (P1)**: Blocked by Foundational only
- **US2 (P1)**: Depends on US1
- **US3 (P1)**: Depends on US2 (or manual manifest population)
- **US4 (P2)**: Depends on US3
- **US5 (P2)**: Depends on US1 only (independent of US2-4)
- **US6 (P3)**: Depends on US4

### Parallel Opportunities

- T002 and T003 can run in parallel (different files)
- T004 and T005 can run in parallel (different directories)
- T022 and T023 can run in parallel (different command files)
- T028 and T029 can run in parallel (different files)

---

## Implementation Strategy

### MVP First (User Stories 1-3)

1. Complete Phases 1-2: Setup + Foundational
2. Complete Phase 3: US1 (build init)
3. Complete Phase 4: US2 (extract)
4. Complete Phase 5: US3 (containerfile)
5. **STOP and VALIDATE**: Init → extract → containerfile → manual build
6. This gives a working end-to-end flow

### Incremental Delivery

1. Setup + Foundational + US1 → `cc-deck build init` works (MVP entry point)
2. Add US2 → Repository analysis works
3. Add US3 → Containerfile generation works
4. Add US4 → `cc-deck build` + `push` works (full CLI flow)
5. Add US5 → Plugin and MCP management
6. Add US6 → Verify and diff commands

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Claude Code commands (`.md` files) are the primary deliverable for US2, US3, US5
- CLI commands (Go code) are the primary deliverable for US1, US4, US6
- All commands and scripts are embedded in the Go binary via `go:embed`
- Commit after each task or logical group


<!-- SDD-TRAIT:beads -->
## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
