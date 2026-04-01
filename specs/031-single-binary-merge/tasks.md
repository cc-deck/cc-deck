# Tasks: Single Binary Merge

**Input**: Design documents from `/specs/031-single-binary-merge/`
**Prerequisites**: plan.md (required), spec.md (required)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Cargo.toml and Entry Point (Blocking)

**Purpose**: Transform the two-binary architecture into a single binary with runtime mode dispatch

- [ ] T001 [US1] Rewrite `cc-zellij-plugin/Cargo.toml`: remove `[features]` section (controller/sidebar/default), remove both `[[bin]]` targets, add single `[[bin]] name = "cc_deck" path = "src/main.rs"` (no required-features)
- [ ] T002 [US1] Rewrite `cc-zellij-plugin/src/main.rs` entry point: remove all `#[cfg(feature = "controller")]` and `#[cfg(feature = "sidebar")]` gates on `mod controller` and `mod sidebar_plugin`; remove the entire `#[cfg(not(any(feature = "controller", feature = "sidebar")))]` legacy block (thread_local STATE, load/update/pipe/render/plugin_version functions); remove the `PluginState` `ZellijPlugin` impl block (lines 558-1955); add a unified plugin struct that reads `config.get("mode")` in `load()` and delegates to either `ControllerPlugin` or `SidebarRendererPlugin`; use `register_plugin!` with the unified struct

**Checkpoint**: Single binary compiles with `make build-wasm`

---

## Phase 2: Named Pipe Channels (Blocking)

**Purpose**: Rename all pipe messages to the `cc-deck:ctrl:*` / `cc-deck:side:*` convention

- [ ] T003 [P] [US2] Add pipe channel name constants to `cc-zellij-plugin/src/lib.rs`: define `pub mod channels { pub const CTRL_HOOK: &str = "cc-deck:ctrl:hook"; pub const CTRL_ACTION: &str = "cc-deck:ctrl:action"; ... }` for all 10 channels from the plan's rename map
- [ ] T004 [P] [US2] Update `cc-zellij-plugin/src/controller/mod.rs`: replace all pipe name string literals with channel constants; update the pipe handler match arms (`"cc-deck:sidebar-hello"` -> `channels::CTRL_HELLO`, `"cc-deck:action"` -> `channels::CTRL_ACTION`, `"cc-deck:render"` -> `channels::SIDE_RENDER` in the ignore filter, etc.)
- [ ] T005 [P] [US2] Update `cc-zellij-plugin/src/controller/render_broadcast.rs`: replace `"cc-deck:render"` with `channels::SIDE_RENDER` in both `MessageToPlugin::new()` calls (lines 123, 139)
- [ ] T006 [P] [US2] Update `cc-zellij-plugin/src/controller/sidebar_registry.rs`: replace `"cc-deck:sidebar-init"` with `channels::SIDE_INIT` (line 102), `"cc-deck:sidebar-reindex"` with `channels::SIDE_REINDEX` (line 113)
- [ ] T007 [P] [US2] Update `cc-zellij-plugin/src/controller/events.rs`: replace `"cc-deck:navigate"` with `channels::SIDE_NAVIGATE` and `"cc-deck:navigate-prev"` with `channels::SIDE_NAVIGATE_PREV` in keybinding registration (lines 244, 254)
- [ ] T008 [P] [US2] Update `cc-zellij-plugin/src/sidebar_plugin/mod.rs`: replace all pipe name string literals with channel constants in the pipe handler match arms (lines 90, 145, 159, 166, 209, 223, 228, 231, 269)
- [ ] T009 [P] [US2] Update `cc-zellij-plugin/src/sidebar_plugin/input.rs`: replace `"cc-deck:action"` with `channels::CTRL_ACTION` in `send_action_to_controller()` (line 516)
- [ ] T010 [P] [US2] Update `cc-zellij-plugin/src/pipe_handler.rs`: replace pipe name literals (`"cc-deck:hook"`, `"cc-deck:navigate"`, `"cc-deck:navigate-prev"`) with channel constants; update test assertions
- [ ] T011 [US2] Update keybinding registration in `cc-zellij-plugin/src/main.rs`: replace `"cc-deck:navigate"`, `"cc-deck:navigate-prev"`, `"cc-deck:attend"`, `"cc-deck:attend-prev"` with channel constants in the `register_keybindings()` KDL template and in `broadcast_action()` calls

**Checkpoint**: `cargo test` passes, all pipe names use channel constants

---

## Phase 3: Legacy Code Removal

**Purpose**: Remove dead code from the old unified architecture

- [ ] T012 [P] [US3] Delete `cc-zellij-plugin/src/sync.rs` and remove `mod sync;` from main.rs. Remove all `sync::` calls from main.rs (sync_now, broadcast_and_save, broadcast_state, flush_if_dirty, apply_session_meta, write_session_meta, prune_session_meta, restore_sessions, request_state, handle_sync). Remove `sync_dirty`, `last_meta_content_hash`, `pending_overrides` fields and related logic from main.rs
- [ ] T013 [P] [US3] Delete `cc-zellij-plugin/src/state.rs` and remove `mod state;` from main.rs. Remove all `state::` imports (NavigateContext, PluginMode, PluginState, SidebarMode, FilterState, RenameState, PendingOverride). Note: NavigateContext, SidebarMode, FilterState, RenameState are still used by sidebar_plugin; verify they are defined there or move them to lib.rs
- [ ] T014 [P] [US3] Delete `cc-zellij-plugin/src/sidebar.rs` and remove `mod sidebar;` from main.rs. Remove all `sidebar::` calls (render_sidebar, handle_click)
- [ ] T015 [P] [US3] Delete `cc-zellij-plugin/src/state_machine_tests.rs` and remove `#[cfg(test)] mod state_machine_tests;` from main.rs
- [ ] T015b [P] [US3] Delete `cc-zellij-plugin/src/attend.rs`, `cc-zellij-plugin/src/notification.rs`, `cc-zellij-plugin/src/rename.rs`, `cc-zellij-plugin/src/fuzz_tests.rs` and remove their `mod` declarations from main.rs. These modules contained old monolithic logic now absorbed into controller/ and sidebar_plugin/ modules
- [ ] T016 [US3] Clean up main.rs: remove all remaining references to deleted modules; remove functions only used by the old PluginState (enter_navigation_mode, exit_to_passive, switch_to_session, abandon_navigation, start_passive_rename, handle_event, handle_event_inner, handle_key, handle_rename_key, handle_delete_confirm_key, handle_filter_key, handle_navigation_key, handle_git_result, session_names_except, is_on_active_tab, mark_sync_dirty, remove_dead_sessions, rebuild_pane_map, preserve_cursor, cleanup_stale_sessions, filtered_sessions_by_tab_order, filtered_session_count, merge_sessions, sessions_by_tab_order); keep utility functions (debug_init, debug_log, install_panic_hook, PerfTimer, PerfTimerPipe, register_keybindings, broadcast_action, focus_plugin, focus_terminal, switch_to_tab, etc.) and shift_variant_tests

**Checkpoint**: `cargo test` passes with no references to sync, PluginState, or old sidebar

---

## Phase 4: Build Pipeline and Go CLI (US4)

**Purpose**: Simplify build system and Go CLI for single binary

- [ ] T017 [P] [US4] Update `Makefile`: ensure single `build-wasm` target using `cargo build --target $(WASM_TARGET) --release` with single wasm-opt pass; ensure `copy-wasm` copies single file; use only WASM_SRC and WASM_DST variables (no controller/sidebar split variables)
- [ ] T018 [P] [US4] Rewrite `cc-deck/internal/plugin/embed.go`: remove `controllerWasm` and `sidebarWasm` embeds; keep single `//go:embed cc_deck.wasm` with `wasmBinary`; simplify `PluginInfo` struct to remove ControllerSize/SidebarSize/ControllerBinary/SidebarBinary fields; update `EmbeddedPlugin()` to return only the single binary info; update `SDKVersion` from "0.43" to "0.44" to match the actual zellij-tile dependency in Cargo.toml
- [ ] T019 [P] [US4] Update `cc-deck/internal/plugin/layout.go`: change `sidebarPluginBlock()` to reference `cc_deck.wasm` instead of `cc_deck_sidebar.wasm`; change `controllerConfigBlock()` to reference `cc_deck.wasm` instead of `cc_deck_controller.wasm`; update `InjectControllerConfig()` and `HasControllerConfig()` to check for `cc_deck.wasm`; update sentinel comments to reflect single binary
- [ ] T020 [P] [US4] Update `cc-deck/internal/plugin/install.go`: change install to write single `cc_deck.wasm` file; remove `sidebarPath` and `controllerPath` separate handling; remove the `EnsureControllerPermissions` call (step 4b); keep the migration cleanup (remove old controller/sidebar binaries); update the install summary output to show single binary info
- [ ] T021 [P] [US4] Update `cc-deck/internal/plugin/zellij.go`: remove `EnsureControllerPermissions()` function entirely
- [ ] T022 [P] [US4] Update `cc-deck/internal/plugin/remove.go`: change removal to delete `cc_deck.wasm` instead of (or in addition to) `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm`
- [ ] T023 [P] [US4] Update `cc-deck/internal/cmd/hook.go`: change pipe name from `"cc-deck:hook"` to `"cc-deck:ctrl:hook"` (line 158)
- [ ] T024 [P] [US4] Update `cc-deck/internal/plugin/layout_test.go`: update all assertions from `cc_deck_controller.wasm` to `cc_deck.wasm`; update test strings
- [ ] T025 [US4] Delete old embedded binaries: remove `cc-deck/internal/plugin/cc_deck_controller.wasm` and `cc-deck/internal/plugin/cc_deck_sidebar.wasm` if they exist

**Checkpoint**: `make build` succeeds, `make test` passes, `make install` installs single binary

---

## Phase 5: Polish and Verification

**Purpose**: Final verification and documentation

- [ ] T026 [US5] Update README.md using `/prose:write` with cc-deck voice profile: document first-tab permission behavior ("on first launch, open a second tab to trigger the permission dialog; after granting once, all subsequent sessions work immediately"); update the Feature Specifications table with 031-single-binary-merge entry and status (Constitution X)
- [ ] T027 Verify end-to-end: run `make install && zellij kill-all-sessions -y 2>/dev/null; zellij --layout cc-deck`, confirm controller + sidebar instances work from single binary, test session switching/renaming/deleting, verify pipe messages use new channel names in debug log
- [ ] T028 Run `make lint` and fix any warnings
- [ ] T029 Run `make test` and verify all tests pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1** (Cargo.toml + entry point): No dependencies, start immediately
- **Phase 2** (Pipe channels): Can start after T001-T002 (needs compiling codebase)
- **Phase 3** (Legacy removal): Can start after T001-T002 (needs the legacy fallback removed first)
- **Phase 4** (Build + CLI): Can start after Phase 1 (needs single binary target defined); Go side (T018-T025) is independent of Rust pipe renames
- **Phase 5** (Polish): After all other phases

### Parallel Opportunities

- Phase 2 tasks T003-T011 are all independent file changes (different files), can run in parallel
- Phase 3 tasks T012-T015b are independent deletions, can run in parallel (T016 depends on them)
- Phase 4 tasks T017-T024 are all independent files, can run in parallel (T025 depends on T018)
- Rust changes (Phase 1-3) and Go changes (Phase 4) can largely proceed in parallel after Phase 1
