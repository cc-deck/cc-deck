# 043: Dead Code Cleanup - Legacy PluginState

## Problem

`cc-zellij-plugin/src/main.rs` contains ~600 lines of dead code from the legacy monolithic plugin (`PluginState`). Since the 030-single-instance-arch refactor, `register_plugin!(UnifiedPlugin)` is the entry point. `PluginState` and its `ZellijPlugin` impl are never called.

This dead code caused a real bug in 042: when adding `WriteToStdin` to the sidebar's permission request, the fix was applied to `PluginState::load()` (line 598) instead of the actual `SidebarRendererPlugin::load()` in `sidebar_plugin/mod.rs`. The stale code path silently accepted the change while the real code path remained unfixed.

## Scope

### Files to modify

- `cc-zellij-plugin/src/main.rs`: Remove `PluginState`, its `ZellijPlugin` impl, all supporting types and functions that are only used by the legacy path
- Related test modules that test `PluginState` behavior (these should already be covered by `UnifiedPlugin` / `SidebarRendererPlugin` / `ControllerPlugin` tests)

### What to keep

- `UnifiedPlugin` and `register_plugin!(UnifiedPlugin)` (the active entry point)
- `sidebar_plugin/` module (the real sidebar implementation)
- `controller/` module (the real controller implementation)
- Shared types used by both paths (check before deleting)

## Approach

1. Identify all types, functions, and impls only referenced by `PluginState`
2. Check each for shared usage with `UnifiedPlugin` / sidebar / controller
3. Remove dead items
4. Run `cargo test` and `cargo clippy` to confirm nothing breaks
5. Verify the remaining test count is reasonable (some tests may have been testing dead code)

## Risk

Low. `register_plugin!(UnifiedPlugin)` means Zellij never instantiates `PluginState`. The only risk is accidentally removing a shared type or helper function. Compiler errors will catch this immediately.

## Priority

Medium. Not urgent, but the dead code is a maintenance hazard (proven by the 042 permission bug). Good candidate for a quiet cleanup PR.
