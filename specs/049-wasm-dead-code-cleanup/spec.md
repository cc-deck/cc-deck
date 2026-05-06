# Feature Specification: WASM Plugin Dead Code Removal and Code Health

**Feature Branch**: `049-wasm-dead-code-cleanup`
**Created**: 2026-05-06
**Status**: Draft
**Input**: Remove dead code from legacy monolithic PluginState architecture and improve WASM plugin code health through module reorganization and type consolidation

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Remove Legacy Dead Code (Priority: P1)

As a plugin developer, I want all legacy monolithic `PluginState` code removed from the codebase so that I never accidentally modify dead code paths instead of the live controller/sidebar architecture.

**Why this priority**: Dead code has already caused a real bug (spec 042, where a permission fix was applied to the dead `PluginState::load()` instead of the live `SidebarRendererPlugin::load()`). This is the primary maintenance hazard and the core motivation for the cleanup.

**Independent Test**: Can be fully verified by confirming `make test` and `make lint` pass after deleting all legacy modules, and that no references to `PluginState` remain in the codebase.

**Acceptance Scenarios**:

1. **Given** the plugin codebase contains legacy modules (`state.rs`, `sidebar.rs`, `rename.rs`, `attend.rs`, `sync.rs`), **When** the cleanup is complete, **Then** none of these files exist and all `mod` declarations for them are removed from `main.rs`
2. **Given** `sync.rs` contains `cleanup_orphaned_state_files()` used by the controller, **When** `sync.rs` is deleted, **Then** `cleanup_orphaned_state_files()` has been relocated to `controller/state.rs` and the controller continues to function correctly
3. **Given** `main.rs` contains `PluginState` impl blocks spanning ~1,600 lines, **When** the cleanup is complete, **Then** no references to `PluginState`, `PluginMode`, or legacy `SidebarMode` remain anywhere in the codebase
4. **Given** legacy test modules test only dead code, **When** the cleanup is complete, **Then** `state_machine_tests.rs` and `fuzz_tests.rs` are deleted, and any unique test scenarios they covered have been ported to controller/sidebar tests

---

### User Story 2 - Reorganize main.rs Into Focused Modules (Priority: P2)

As a plugin developer, I want `main.rs` to contain only the `UnifiedPlugin` dispatcher and module declarations so that I can quickly find and navigate code by purpose rather than scrolling through a 2,000-line file mixing unrelated concerns.

**Why this priority**: After dead code removal, `main.rs` will still contain ~450 lines of mixed concerns (debug logging, WASM shims, keybinding registration, perf timers). Extracting these into focused modules improves navigability and supports the single-responsibility principle.

**Independent Test**: Can be verified by confirming `main.rs` is under 500 lines, each extracted module compiles independently, and `make test` passes.

**Acceptance Scenarios**:

1. **Given** `main.rs` contains debug logging and panic hook code, **When** reorganization is complete, **Then** these live in a dedicated `debug.rs` module
2. **Given** `main.rs` contains 15+ WASM/native conditional function pairs, **When** reorganization is complete, **Then** these are consolidated in `wasm_compat.rs`
3. **Given** `main.rs` contains `register_keybindings()` and `shift_variant()`, **When** reorganization is complete, **Then** these live in a dedicated `keybindings.rs` module
4. **Given** the reorganization is complete, **When** line count is measured, **Then** `main.rs` is under 500 lines

---

### User Story 3 - Consolidate Types and Remove Warning Suppressions (Priority: P3)

As a plugin developer, I want the global `#![allow(dead_code, unused_imports)]` removed and redundant type definitions consolidated so that the compiler catches real dead code and unused imports going forward.

**Why this priority**: The global suppression was a workaround for the legacy code coexisting with the new architecture. Once dead code is removed, this suppression hides real issues. Type consolidation ensures a single source of truth for shared types.

**Independent Test**: Can be verified by confirming `make lint` passes with zero warnings after removing the global allow, with only targeted `#[allow(...)]` on genuinely conditional WASM/native code.

**Acceptance Scenarios**:

1. **Given** `main.rs` starts with `#![allow(dead_code, unused_imports)]`, **When** the cleanup is complete, **Then** this global suppression is removed
2. **Given** WASM/native conditional function pairs legitimately need dead_code allows, **When** the global suppression is removed, **Then** only targeted `#[allow(dead_code)]` attributes are applied to specific items that need them
3. **Given** types may be duplicated between `lib.rs` and the module tree, **When** consolidation is complete, **Then** each shared type has a single canonical definition
4. **Given** all changes are complete, **When** `make lint` runs, **Then** zero warnings are reported

---

### Edge Cases

- What happens if a legacy module contains helper functions used by both the dead `PluginState` path and the live controller/sidebar path? Each function must be audited before deletion; shared helpers are relocated rather than deleted.
- What happens if removing the global `#![allow(dead_code)]` exposes dead code in the active controller/sidebar modules? This dead code should be evaluated and either removed or given a targeted allow with justification.
- What happens if legacy test scenarios cover behaviors not tested by the new architecture's tests? These scenarios must be ported before the legacy test files are deleted.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All legacy modules MUST be deleted: `src/state.rs`, `src/sidebar.rs`, `src/rename.rs`, `src/attend.rs`, `src/sync.rs`
- **FR-002**: `cleanup_orphaned_state_files()` from `sync.rs` MUST be relocated to `controller/state.rs` before `sync.rs` is deleted
- **FR-003**: All `PluginState` impl blocks and the `ZellijPlugin for PluginState` impl MUST be removed from `main.rs`
- **FR-004**: All `mod` declarations and `use` imports for deleted modules MUST be removed from `main.rs`
- **FR-005**: Legacy test files (`state_machine_tests.rs`, `fuzz_tests.rs`) MUST be audited for unique coverage before deletion
- **FR-006**: Unique test scenarios from legacy tests MUST be ported to controller/sidebar test modules before legacy test files are deleted
- **FR-007**: Debug logging, panic hook, and `DEBUG_ENABLED` MUST be extracted from `main.rs` into `src/debug.rs`
- **FR-008**: All WASM/native conditional function pairs MUST be consolidated in `src/wasm_compat.rs`
- **FR-009**: Keybinding registration (`register_keybindings()`, `shift_variant()`) MUST be extracted into `src/keybindings.rs`
- **FR-010**: The global `#![allow(dead_code, unused_imports)]` MUST be removed from `main.rs`
- **FR-011**: Targeted `#[allow(dead_code)]` MUST be applied only to WASM/native conditional code that legitimately triggers the lint
- **FR-012**: Redundant type definitions between `lib.rs` and the module tree MUST be consolidated to a single canonical location
- **FR-013**: The plugin MUST compile, pass `make test`, and pass `make lint` with zero warnings after all changes
- **FR-014**: The existing behavior of the controller and sidebar plugins MUST remain unchanged (no functional modifications)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `main.rs` contains fewer than 500 lines (down from 2,083)
- **SC-002**: Total plugin line count drops by at least 4,000 lines (from ~14,400)
- **SC-003**: Zero references to `PluginState`, `PluginMode`, or legacy `SidebarMode` exist in the codebase
- **SC-004**: `make test` passes with zero failures
- **SC-005**: `make lint` passes with zero warnings (no global suppression)
- **SC-006**: WASM binary size is measured before and after; any reduction is documented
- **SC-007**: No legacy module files (`state.rs`, `sidebar.rs`, `rename.rs`, `attend.rs`, `sync.rs`, `state_machine_tests.rs`, `fuzz_tests.rs`) exist in `src/`

## Assumptions

- LTO (link-time optimization) may have already been stripping dead code from the binary, so the WASM binary size reduction may be modest. The primary benefit is developer experience and compile-time improvement, not binary size.
- The controller and sidebar plugin modules (`controller/`, `sidebar_plugin/`) have sufficient test coverage to replace the legacy test files. An audit will confirm this before deletion.
- `cleanup_orphaned_state_files()` is the only function from `sync.rs` still needed by active code. An audit will confirm this during implementation.
- The `UnifiedPlugin` dispatcher, its tests, and the `register_plugin!` macro invocation in `main.rs` are active code and must be preserved.
- The `lib.rs` shared protocol types (`RenderPayload`, `ActionMessage`, `SidebarHello`, `SidebarInit`) are actively used and must be preserved.
