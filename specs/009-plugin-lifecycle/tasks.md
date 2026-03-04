# Tasks: Plugin Lifecycle Management

**Input**: Design documents from `/specs/009-plugin-lifecycle/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/cli-commands.md

**Tests**: Not explicitly requested in spec. Test tasks omitted.

**Organization**: Tasks grouped by user story. US1 and US2 are combined (install always produces a layout).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create package structure and embed the WASM binary

- [ ] T001 (cc-mux-0ge.1) Create `internal/plugin/` package directory in cc-deck/internal/plugin/
- [ ] T002 (cc-mux-0ge.2) Add cc_deck.wasm to .gitignore in cc-deck/.gitignore
- [ ] T003 (cc-mux-0ge.3) Build WASM binary from cc-zellij-plugin/ and copy to cc-deck/internal/plugin/cc_deck.wasm
- [ ] T004 (cc-mux-0ge.4) Create embed.go with `//go:embed cc_deck.wasm` directive and PluginInfo struct (Version, SDKVersion, MinZellij, BinarySize) in cc-deck/internal/plugin/embed.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Zellij detection and layout template infrastructure used by all commands

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 (cc-mux-qp8.1) [P] Implement ZellijInfo struct and DetectZellij() function that runs `zellij --version`, parses output, resolves config directory (ZELLIJ_CONFIG_DIR env or default ~/.config/zellij/), and derives PluginsDir/LayoutsDir paths in cc-deck/internal/plugin/zellij.go
- [ ] T006 (cc-mux-qp8.2) [P] Implement layout template constants (MinimalLayout, FullLayout as KDL strings), injection sentinel markers, and helper functions HasInjection(content) / InjectPlugin(content) / RemoveInjection(content) in cc-deck/internal/plugin/layout.go
- [ ] T007 (cc-mux-qp8.3) [P] Implement InstallState struct and DetectInstallState(zellijInfo, pluginInfo) function that checks filesystem for installed binary, layout file, and default layout injection in cc-deck/internal/plugin/state.go
- [ ] T008 (cc-mux-qp8.4) Implement CheckCompatibility(zellijVersion, sdkVersion) returning "compatible", "untested", or "incompatible" based on version comparison (min 0.40, SDK 0.43 range) in cc-deck/internal/plugin/zellij.go

**Checkpoint**: Foundation ready, all three commands can now be implemented in parallel

---

## Phase 3: User Story 1+2 - Install Plugin with Layout Choice (Priority: P1) MVP

**Goal**: User runs `cc-deck plugin install` and gets the WASM binary + a layout file written to disk. Supports `--layout minimal|full` and `--force` flags.

**Independent Test**: Run install, verify cc_deck.wasm exists in plugins dir and cc-deck.kdl exists in layouts dir. Launch `zellij --layout cc-deck` to confirm plugin loads.

### Implementation

- [ ] T009 (cc-mux-b4q.1) [US1] Implement Install() function with atomic write (temp file + rename) for the WASM binary to PluginsDir, directory creation via os.MkdirAll, and overwrite confirmation prompt (skipped with force flag) in cc-deck/internal/plugin/install.go
- [ ] T010 (cc-mux-b4q.2) [US1] [US2] Add layout file writing to Install(): write selected layout template (minimal or full) to LayoutsDir/cc-deck.kdl in cc-deck/internal/plugin/install.go
- [ ] T011 (cc-mux-b4q.3) [US1] Add install summary output formatting (installed paths, file sizes, launch instructions per CLI contract) in cc-deck/internal/plugin/install.go
- [ ] T012 (cc-mux-b4q.4) [US1] Create NewPluginCmd() cobra parent command with install/status/remove subcommands, and NewPluginInstallCmd() with --force, --layout, --inject-default flags per CLI contract in cc-deck/internal/cmd/plugin.go
- [ ] T013 (cc-mux-b4q.5) [US1] Register plugin command in main.go via rootCmd.AddCommand(cmd.NewPluginCmd(gf)) in cc-deck/cmd/cc-deck/main.go

**Checkpoint**: `cc-deck plugin install` and `cc-deck plugin install --layout full` work end-to-end

---

## Phase 4: User Story 3 - Inject into Default Layout (Priority: P2)

**Goal**: User runs `cc-deck plugin install --inject-default` and the plugin pane block is appended to their existing default Zellij layout with sentinel markers.

**Independent Test**: Create a sample default.kdl, run install with --inject-default, verify plugin pane block appears between sentinel markers. Run again to verify duplicate detection skips re-injection.

### Implementation

- [ ] T014 (cc-mux-u1q.1) [US3] Implement InjectDefault() that locates default.kdl in ConfigDir/layouts/, reads content, calls HasInjection() to check for duplicates, calls InjectPlugin() to append the sentinel-wrapped pane block, writes back atomically in cc-deck/internal/plugin/install.go
- [ ] T015 (cc-mux-u1q.2) [US3] Handle edge cases: no default layout found (warn + suggest dedicated layout), unparseable file (warn + skip injection, continue with binary/layout install per FR-022) in cc-deck/internal/plugin/install.go
- [ ] T016 (cc-mux-u1q.3) [US3] Wire --inject-default flag to InjectDefault() call in the install command runner in cc-deck/internal/cmd/plugin.go

**Checkpoint**: `cc-deck plugin install --inject-default` works, detects duplicates, handles missing default layout gracefully

---

## Phase 5: User Story 4 - Check Plugin Status (Priority: P2)

**Goal**: User runs `cc-deck plugin status` and sees installation state, Zellij compatibility, and layout information.

**Independent Test**: Run status in different states (not installed, installed, with/without Zellij) and verify output matches CLI contract format.

### Implementation

- [ ] T017 (cc-mux-7vm.1) [P] [US4] Implement Status() that calls DetectZellij(), DetectInstallState(), and CheckCompatibility(), then returns a StatusReport struct with all fields needed for text/JSON/YAML output in cc-deck/internal/plugin/status.go
- [ ] T018 (cc-mux-7vm.2) [US4] Implement text output formatting for status (aligned key-value sections: Plugin Status, Zellij, Layouts per CLI contract) in cc-deck/internal/plugin/status.go
- [ ] T019 (cc-mux-7vm.3) [US4] Implement JSON/YAML output struct with json/yaml tags matching the CLI contract schema in cc-deck/internal/plugin/status.go
- [ ] T020 (cc-mux-7vm.4) [US4] Wire status command to Status() function with --output flag support (text/json/yaml) in cc-deck/internal/cmd/plugin.go

**Checkpoint**: `cc-deck plugin status` and `cc-deck plugin status -o json` work in all installation states

---

## Phase 6: User Story 5 - Remove Plugin (Priority: P2)

**Goal**: User runs `cc-deck plugin remove` and all installed artifacts are cleaned up, including default layout injection reversal.

**Independent Test**: Install with --inject-default, run remove, verify WASM binary gone, layout file gone, default layout reverted to original content.

### Implementation

- [ ] T021 (cc-mux-xvg.1) [US5] Implement Remove() that deletes cc_deck.wasm from PluginsDir, deletes cc-deck.kdl from LayoutsDir, calls RemoveInjection() on default layout if injected, and prints summary of removed/modified files in cc-deck/internal/plugin/remove.go
- [ ] T022 (cc-mux-xvg.2) [US5] Handle edge cases: plugin not installed (report "nothing to remove"), Zellij running (warn about restart per FR-023), partial installation (remove what exists) in cc-deck/internal/plugin/remove.go
- [ ] T023 (cc-mux-xvg.3) [US5] Wire remove command to Remove() function in cc-deck/internal/cmd/plugin.go

**Checkpoint**: Full install-remove cycle leaves no artifacts behind

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Error handling, edge cases, and build pipeline

- [ ] T024 (cc-mux-3u1.1) [P] Add actionable error messages for permission failures (include exact path and required permissions per FR-021) across all plugin operations in cc-deck/internal/plugin/install.go and cc-deck/internal/plugin/remove.go
- [ ] T025 (cc-mux-3u1.2) [P] Add Zellij-not-found warning to install and status commands (stderr warning per FR-014) in cc-deck/internal/plugin/install.go and cc-deck/internal/plugin/status.go
- [ ] T026 (cc-mux-3u1.3) Create Makefile at repo root with targets: `build-wasm` (cargo build), `copy-wasm` (cp to embed location), `build-cli` (go build), `build` (all three), `clean` in Makefile
- [ ] T027 (cc-mux-3u1.4) Run quickstart.md validation: execute the build and usage steps from specs/009-plugin-lifecycle/quickstart.md end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T004 specifically for PluginInfo)
- **US1+2 Install (Phase 3)**: Depends on Phase 2 completion
- **US3 Inject (Phase 4)**: Depends on Phase 3 (extends install logic)
- **US4 Status (Phase 5)**: Depends on Phase 2 only (independent of install implementation)
- **US5 Remove (Phase 6)**: Depends on Phase 2 only (independent of install implementation)
- **Polish (Phase 7)**: Depends on Phases 3-6

### User Story Dependencies

- **US1+2 (P1)**: Depends on Foundational only. MVP target.
- **US3 (P2)**: Depends on US1+2 (extends the install command)
- **US4 (P2)**: Independent of US1, US3, US5. Can start after Foundational.
- **US5 (P2)**: Independent of US1, US3, US4. Can start after Foundational.

### Within Each User Story

- Domain logic before cobra wiring
- Core path before edge cases
- Story complete before moving to next priority

### Parallel Opportunities

- T005, T006, T007 can all run in parallel (different files, no dependencies)
- T017 (status logic) can run in parallel with Phase 3 install work
- US4 and US5 can be developed in parallel after Foundational
- T024, T025 can run in parallel (different error scenarios)

---

## Parallel Example: Foundational Phase

```bash
# Launch all foundational tasks together (different files):
Task: "Implement ZellijInfo + DetectZellij() in zellij.go"
Task: "Implement layout templates + injection helpers in layout.go"
Task: "Implement InstallState + DetectInstallState() in state.go"
```

## Parallel Example: After Foundational

```bash
# US4 and US5 can start in parallel with US1+2:
Developer A: Phase 3 (US1+2 Install)
Developer B: Phase 5 (US4 Status)
Developer C: Phase 6 (US5 Remove)
```

---

## Implementation Strategy

### MVP First (US1+2 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T008)
3. Complete Phase 3: US1+2 Install (T009-T013)
4. **STOP and VALIDATE**: Run `cc-deck plugin install`, verify binary + layout exist, launch `zellij --layout cc-deck`
5. This is a shippable MVP

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1+2 Install -> Test independently -> MVP!
3. Add US3 Inject -> Test default layout injection
4. Add US4 Status -> Test status reporting
5. Add US5 Remove -> Test full lifecycle (install -> status -> remove)
6. Polish -> Error messages, build pipeline

### Parallel Team Strategy

With multiple developers after Foundational is done:
- Developer A: US1+2 Install (Phase 3) then US3 Inject (Phase 4)
- Developer B: US4 Status (Phase 5) then US5 Remove (Phase 6)
- Both: Polish (Phase 7)

---

## Notes

- No new Go dependencies needed (stdlib only: os, os/exec, embed, path/filepath, strings, fmt)
- WASM binary is .gitignored; must be built before `go build`
- Atomic writes (temp + rename) for plugin binary only; layout files are small enough for direct write
- All file paths respect ZELLIJ_CONFIG_DIR env var fallback

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
