# Research: WASM Plugin Dead Code Removal

**Feature**: 049-wasm-dead-code-cleanup | **Date**: 2026-05-06

## Decision: Module Classification (Dead vs Active)

**Decision**: Classify modules as dead (delete), mostly-dead (relocate then delete), or active (keep).

**Rationale**: Traced all `crate::` references from the active controller/ and sidebar_plugin/ modules. Any root-level module not referenced by active code (directly or transitively) is dead.

**Alternatives considered**:
- Compiler-based dead code analysis: Not feasible because the `#![allow(dead_code)]` suppression hides everything, and WASM/native cfg splits make static analysis unreliable.
- Conservative approach (keep all, just remove PluginState impl): Would leave dead modules accumulating warnings once the global allow is removed.

### Classification Results

| Module | Classification | Active Code to Relocate | Lines |
|--------|---------------|------------------------|-------|
| state.rs | DEAD | None | 789 |
| sidebar.rs | MOSTLY DEAD | `HELP_LINES` constant | 601 |
| rename.rs | DEAD | None | 342 |
| attend.rs | DEAD | None | 479 |
| sync.rs | MOSTLY DEAD | `cleanup_orphaned_state_files()` + helpers, `is_sync_message()`, `is_request_message()`, `extract_pid_from_message_name()` | 825 |
| notification.rs | DEAD | None | ~50 |
| state_machine_tests.rs | DEAD | Audit for unique coverage | 1,080 |
| fuzz_tests.rs | DEAD | Audit for unique coverage | 324 |

## Decision: Keybinding Function Handling

**Decision**: Delete `register_keybindings()` and `shift_variant()` from main.rs. Do NOT create a shared `keybindings.rs` module.

**Rationale**: The controller already has its own copies of both functions in `controller/events.rs` with their own tests. The sidebar does not need keybinding registration. Creating a shared module would add unnecessary abstraction for code used by exactly one consumer.

**Alternatives considered**:
- Extract to shared `keybindings.rs` (spec suggestion): Adds indirection for a single consumer. Rejected.
- Keep duplicates in both places: The main.rs copies are dead code being deleted. Not applicable.

## Decision: sync.rs Relocation Strategy

**Decision**: Split sync.rs utilities across two destinations based on consumers.

**Rationale**: The message-parsing utilities (`is_sync_message`, `is_request_message`, `extract_pid_from_message_name`) are consumed by `pipe_handler.rs`, so they belong there. The file-cleanup utilities (`cleanup_orphaned_state_files`, `extract_pid_from_filename`) are consumed by the controller, so they belong in `controller/state.rs`.

**Alternatives considered**:
- Move everything to a new `sync_utils.rs`: Creates a grab-bag module with no cohesion.
- Move everything to controller/state.rs: Would couple pipe_handler to the controller module.

## Decision: Debug Code Extraction

**Decision**: Extract `DEBUG_ENABLED`, `debug_init()`, `debug_log()`, and `install_panic_hook()` to `src/debug.rs`.

**Rationale**: These are used extensively by both controller and sidebar via `crate::debug_log()`. Keeping them in main.rs alongside UnifiedPlugin mixes unrelated concerns.

**Alternatives considered**:
- Keep in main.rs: Increases main.rs line count and mixes concerns.
- Use a logging crate: Over-engineering for a WASM plugin that writes to `/cache/debug.log`.
