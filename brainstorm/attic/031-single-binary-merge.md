# Brainstorm: Single Binary Merge (031)

## Core Idea

Merge the two WASM binaries (cc_deck_controller.wasm + cc_deck_sidebar.wasm) back into a single binary (cc_deck.wasm) that determines its role from KDL configuration at runtime. This solves the background plugin permission problem cleanly and simplifies the build/install process.

## Problem Statement

Background plugins loaded via `load_plugins` in config.kdl cannot show permission dialogs in Zellij 0.44. The `RequestPluginPermissions` handler only finds plugins in tabs (`tab.has_plugin()`), but background plugins aren't in any tab. This forces us to pre-populate `permissions.kdl` during install, which is a fragile workaround.

With a single binary, the sidebar instance (which IS in a tab) gets the permission dialog. Since Zellij caches permissions by plugin URL, the controller instance (same URL, different config) inherits the granted permissions.

## Proposed Architecture

```
cc_deck.wasm (single binary)
  load() reads config.get("mode")
    "controller" -> ControllerPlugin behavior
    "sidebar"    -> SidebarRendererPlugin behavior (default)
```

### Layout (cc-deck.kdl)
```kdl
layout {
    default_tab_template {
        pane split_direction="vertical" {
            pane size=22 borderless=true {
                plugin location="file:.../cc_deck.wasm" {
                    mode "sidebar"
                }
            }
            children
        }
    }
}
```

### Config (config.kdl)
```kdl
load_plugins {
    "file:.../cc_deck.wasm" {
        mode "controller"
    }
}
```

## Implementation Steps

1. Remove feature flags from Cargo.toml, single `[[bin]]` target
2. Create unified plugin struct that delegates to controller or sidebar based on mode
3. Both controller/ and sidebar_plugin/ modules always compiled (no cfg gates)
4. Makefile back to single build-wasm target
5. Go CLI: single embed, single install path
6. Remove permission pre-population hack from install.go
7. Remove EnsureControllerPermissions and related code

## Open Questions

- Does Zellij deduplicate plugin instances with the same URL but different config? (It shouldn't, each gets its own instance)
- Will the permission cache entry for `cc_deck.wasm` cover both controller and sidebar configs? (Should work since cache is keyed by URL, not URL+config)
- Binary size impact of including both controller and sidebar code? (Minimal, current sizes are 832KB + 769KB, shared code is most of it)

## Remaining Issues from Current Branch

These should be addressed during or after the single-binary merge:

### Functional Issues
- **Mouse click reliability**: First click after window activation is consumed by Zellij focus, second click reaches plugin. Zellij-level behavior, not fixable in plugin.
- **Session flickering on mouse movement**: PaneUpdate with focus changes triggers re-renders. Reduced by only broadcasting on focus change, but still visible during rapid mouse movement.
- **Timer-based coalescing delay**: Hook events use 1s timer for render coalescing. Could be reduced but risks message storms during snapshot restore.

### UX Polish
- **Permission dialog on first tab**: Zellij doesn't show the permission dialog on the initial tab's sidebar. Only appears on new tabs. Single-binary merge should fix this.
- **Rename cursor visibility**: Works but could be refined (reverse video vs block cursor depending on terminal)
- **Help overlay**: Not yet implemented in new sidebar (shows empty)

### Architecture Debt
- **Old sync.rs still compiled**: Legacy sync code is still in the binary under no-feature-flag builds. Should be removed after single-binary merge is stable.
- **Old PluginState still compiled**: The entire old architecture is included when no feature flag is set. Should be removed.
- **Test failures from zellij-tile 0.44**: PaneInfo/TabInfo struct changes broke old state_machine_tests. Need to update test fixtures.

## Decision: Proceed?

The single-binary merge is a clear improvement:
- Solves the permission problem cleanly
- Simplifies the build pipeline
- Reduces install complexity
- No architectural compromise (still controller+sidebar split internally)

Recommend: proceed after merging the current branch to main.
