# Tasks: Single Binary Merge

**Input**: Design documents from `/specs/031-single-binary-merge/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No new project structure needed. This is a refactoring of an existing codebase. Setup verifies the starting state.

- [ ] T001 Verify all existing tests pass before starting refactoring with `cargo test` in cc-zellij-plugin/
- [ ] T002 Verify Go tests pass with `go test ./...` in cc-deck/

---

## Phase 2: Rust Plugin (Single Binary)

**Purpose**: Merge two WASM binaries into one with runtime mode dispatch. This is the foundational change that all other phases depend on.

**CRITICAL**: No Go CLI or Makefile changes can be validated until this phase produces a working single `cc_deck.wasm`.

- [ ] T003 Remove `[features]` section and both `[[bin]]` targets from cc-zellij-plugin/Cargo.toml. Add single `[[bin]]` target named `cc_deck` with `path = "src/main.rs"` (no `required-features`)
- [ ] T004 Remove `#[cfg(feature = "controller")]` and `#[cfg(feature = "sidebar")]` gates on `mod controller` and `mod sidebar_plugin` declarations in cc-zellij-plugin/src/main.rs. Both modules must always be compiled.
- [ ] T005 Add `UnifiedPlugin` enum to cc-zellij-plugin/src/main.rs with variants `Controller(controller::ControllerPlugin)`, `Sidebar(sidebar_plugin::SidebarRendererPlugin)`, and `Uninitialized`. Implement `Default` returning `Uninitialized`.
- [ ] T006 Implement `ZellijPlugin` for `UnifiedPlugin` in cc-zellij-plugin/src/main.rs: `load()` reads `configuration.get("mode")`, initializes the appropriate variant, delegates to its `load()`. Default to `Sidebar` when mode is absent. All other trait methods (`update`, `pipe`, `render`) delegate to the active variant.
- [ ] T007 Replace all `register_plugin!` calls and the legacy no-feature-flag registration path (thread_local STATE, manual load/update/pipe/render functions) with single `register_plugin!(UnifiedPlugin)` in cc-zellij-plugin/src/main.rs
- [ ] T008 Verify `cargo test` passes in cc-zellij-plugin/ after all Rust changes
- [ ] T009 Verify `cargo build --target wasm32-wasip1 --release` produces a single cc-zellij-plugin/target/wasm32-wasip1/release/cc_deck.wasm

**Checkpoint**: Single WASM binary builds and all Rust tests pass.

---

## Phase 3: User Story 1 - Permission Dialog Appears Automatically (Priority: P1) + User Story 3 - Behavior Unchanged (Priority: P1)

**Goal**: The single binary works correctly as both controller and sidebar with permissions granted through one dialog.

**Independent Test**: Install the plugin, start Zellij, grant permissions on sidebar dialog, verify controller functions without additional prompts. Verify all interactive features work.

Note: US1 and US3 are coupled at implementation level. The permission fix is a natural consequence of the single binary, and behavioral parity is verified by the same test suite.

### Implementation

- [ ] T010 [US1] Update Makefile: replace `WASM_CTRL_SRC`, `WASM_CTRL_DST`, `WASM_SIDE_SRC`, `WASM_SIDE_DST` variables with single `WASM_SRC` and `WASM_DST` pointing to cc_deck.wasm
- [ ] T011 [US1] Update Makefile: replace `build-wasm-controller` and `build-wasm-sidebar` targets with single `build-wasm` target running `cargo build --target wasm32-wasip1 --release` (no feature flags). Include conditional `wasm-opt` step.
- [ ] T012 [US1] Update Makefile: simplify `copy-wasm` target to copy single binary from `WASM_SRC` to `WASM_DST`
- [ ] T013 [US1] Update Makefile: simplify `build-wasm-debug` target to build without feature flags using `dev-opt` profile
- [ ] T014 [US1] Update Makefile: add cleanup step in `install` target to remove old `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` from `~/.config/zellij/plugins/`
- [ ] T015 [P] [US1] Update cc-deck/internal/plugin/layout.go: change `sidebarPluginBlock()` to reference `cc_deck.wasm` instead of `cc_deck_sidebar.wasm`
- [ ] T016 [P] [US1] Update cc-deck/internal/plugin/layout.go: change `controllerConfigBlock()` to reference `cc_deck.wasm` instead of `cc_deck_controller.wasm`
- [ ] T017 [US1] Remove `EnsureControllerPermissions()` function from cc-deck/internal/plugin/zellij.go
- [ ] T018 [US1] Remove `EnsureControllerPermissions()` call from cc-deck/internal/plugin/install.go
- [ ] T019 [US1] Update cc-deck/internal/plugin/layout_test.go: fix test assertions to expect `cc_deck.wasm` references instead of `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm`
- [ ] T020 [US1] Verify `go test ./internal/plugin/...` passes in cc-deck/ after layout and install changes

**Checkpoint**: Permission workaround removed. Layout and config reference single binary. Go tests pass.

---

## Phase 4: User Story 2 - Simplified Build and Install (Priority: P2)

**Goal**: Build produces one binary, install writes one file, Go CLI uses simplified PluginInfo.

**Independent Test**: Run `make install`, verify single `cc_deck.wasm` in plugins directory, no legacy files present.

### Implementation

- [ ] T021 [P] [US2] Simplify cc-deck/internal/plugin/embed.go: remove `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` embeds. Keep single `//go:embed cc_deck.wasm`. Simplify `PluginInfo` to single `Binary []byte` and `BinarySize int64`, remove `ControllerBinary`, `SidebarBinary`, `ControllerSize`, `SidebarSize` fields.
- [ ] T022 [US2] Update cc-deck/internal/plugin/install.go: replace two-binary atomic write logic with single `atomicWrite()` call for `cc_deck.wasm`. Remove rollback logic. Add removal of old `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` if present.
- [ ] T023 [P] [US2] Update cc-deck/internal/plugin/remove.go: simplify to remove single `cc_deck.wasm` instead of separate controller/sidebar files. Keep controller config removal from config.kdl.
- [ ] T024 [P] [US2] Update cc-deck/internal/plugin/state.go: change `InstallState` to check for `cc_deck.wasm` instead of `cc_deck_controller.wasm`
- [ ] T025 [US2] Fix all compile errors in cc-deck/ from PluginInfo struct changes (update all callers of `EmbeddedPlugin()` that reference removed fields)
- [ ] T026 [US2] Verify `go test ./...` passes in cc-deck/ after all Go changes
- [ ] T027 [US2] Remove old embedded WASM files: delete cc-deck/internal/plugin/cc_deck_controller.wasm and cc-deck/internal/plugin/cc_deck_sidebar.wasm from the repository

**Checkpoint**: Go CLI fully simplified. Single embed, single install, single removal.

---

## Phase 5: Integration Verification

**Purpose**: End-to-end validation across both components.

- [ ] T028 Run `make install` from project root and verify it succeeds
- [ ] T029 Verify exactly one `cc_deck.wasm` exists in `~/.config/zellij/plugins/` and no `cc_deck_controller.wasm` or `cc_deck_sidebar.wasm` are present
- [ ] T030 Verify generated layout files reference `cc_deck.wasm` with `mode "sidebar"`
- [ ] T031 Verify config.kdl `load_plugins` references `cc_deck.wasm` with `mode "controller"`
- [ ] T032 Run `make test` and `make lint` to confirm all tests and linting pass

**Checkpoint**: All automated checks pass. Ready for manual Zellij testing.

---

## Phase 6: Polish & Documentation

**Purpose**: Documentation updates and final cleanup.

- [ ] T033 [P] Update README.md with single-binary architecture description (replace references to two binaries)
- [ ] T034 [P] Update specs table in README.md with 031-single-binary-merge entry
- [ ] T035 Remove legacy backward-compatibility comments from Makefile (the "Keep legacy single-binary path" comment and similar)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, verifies starting state
- **Phase 2 (Rust Plugin)**: Depends on Phase 1. BLOCKS all subsequent phases.
- **Phase 3 (US1+US3)**: Depends on Phase 2 (needs single binary to exist)
- **Phase 4 (US2)**: Depends on Phase 2 (needs single binary for embed). Can run in parallel with Phase 3.
- **Phase 5 (Integration)**: Depends on Phase 3 AND Phase 4
- **Phase 6 (Polish)**: Depends on Phase 5

### User Story Dependencies

- **US1 (Permission Dialog) + US3 (Behavior Unchanged)**: Combined because they share the same implementation (single binary). Can start after Phase 2.
- **US2 (Simplified Build)**: Independent of US1/US3. Can start after Phase 2 in parallel with Phase 3.

### Within Phases

- Phase 2: T003 → T004 → T005 → T006 → T007 → T008 → T009 (sequential, each depends on previous)
- Phase 3: T010-T014 sequential (Makefile), T015-T016 parallel (layout.go), T017-T018 sequential, T019-T020 sequential
- Phase 4: T021, T023, T024 parallel (different files), T022 and T025 sequential after T021

### Parallel Opportunities

- Phase 3 and Phase 4 can run in parallel after Phase 2 completes
- Within Phase 3: T015 and T016 can run in parallel (same file but independent functions)
- Within Phase 4: T021, T023, T024 can run in parallel (different files)

---

## Parallel Example: Phase 3 + Phase 4

```bash
# After Phase 2 completes, launch both phases:

# Phase 3 agent (Makefile + layout + install changes):
Task: "Update Makefile build targets for single binary"
Task: "Update layout.go references"
Task: "Remove permission workaround"

# Phase 4 agent (embed + PluginInfo simplification):
Task: "Simplify embed.go to single binary"
Task: "Simplify remove.go and state.go"
```

---

## Implementation Strategy

### MVP First (Phase 1 + Phase 2 Only)

1. Complete Phase 1: Verify starting state
2. Complete Phase 2: Produce single cc_deck.wasm
3. **STOP and VALIDATE**: `cargo test` and manual `cargo build --target wasm32-wasip1` both succeed
4. This is the core deliverable. Everything else is cleanup.

### Incremental Delivery

1. Phase 2 → Single binary exists (core)
2. Phase 3 → Makefile and install updated (usable)
3. Phase 4 → Go CLI simplified (clean)
4. Phase 5 → Integration verified (confident)
5. Phase 6 → Documentation updated (complete)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Commit after each phase completion
- The Rust changes (Phase 2) are the riskiest part. Validate thoroughly before proceeding.
- Legacy code cleanup (sync.rs, old PluginState, broken test fixtures) is explicitly out of scope per spec.
