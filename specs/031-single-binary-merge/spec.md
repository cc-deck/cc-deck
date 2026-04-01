# Feature Specification: Single Binary Merge

**Feature Branch**: `031-single-binary-merge`  
**Created**: 2026-04-01  
**Status**: Draft  
**Input**: User description: "Merge controller and sidebar WASM binaries into a single binary with runtime mode selection"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Permission Dialog Appears Automatically (Priority: P1)

A user installs cc-deck and launches Zellij for the first time. The sidebar plugin (visible in a tab) triggers a permission dialog. Because the controller uses the same WASM binary, Zellij's permission cache (keyed by plugin URL) covers the controller instance automatically. The user never needs to manually edit `permissions.kdl` or deal with a broken permission flow for the background plugin.

**Why this priority**: The current two-binary architecture makes background plugin permissions unworkable in Zellij 0.44. Users must manually edit cache files, which is fragile and undocumented. This is the primary motivation for the merge.

**Independent Test**: Install the plugin, start a fresh Zellij session, grant permissions on the sidebar dialog, and verify the controller instance functions without additional permission prompts.

**Acceptance Scenarios**:

1. **Given** a fresh installation with no permission cache, **When** the user opens Zellij with the cc-deck layout, **Then** the sidebar shows a permission dialog and granting it enables both sidebar and controller functionality.
2. **Given** permissions were previously granted for `cc_deck.wasm`, **When** the user opens a new Zellij session, **Then** both controller and sidebar instances start without any permission prompts.
3. **Given** the user has never installed cc-deck before, **When** they run `cc-deck plugin install` and start Zellij, **Then** no manual editing of `permissions.kdl` or any cache file is required.

---

### User Story 2 - Simplified Build and Install (Priority: P2)

A developer builds and installs cc-deck. The build produces a single WASM binary (`cc_deck.wasm`) instead of two. The install command copies one file instead of two, generates layout and config referencing the same binary, and removes any permission pre-population workarounds.

**Why this priority**: The two-binary build adds complexity to the Makefile, Go embed logic, and install flow. A single binary reduces maintenance surface and makes the build more predictable.

**Independent Test**: Run `make install`, verify a single `cc_deck.wasm` is placed in the plugins directory, and confirm the layout and config files both reference it.

**Acceptance Scenarios**:

1. **Given** the developer runs `make install`, **When** the build completes, **Then** exactly one WASM binary (`cc_deck.wasm`) is placed in `~/.config/zellij/plugins/`.
2. **Given** a previous installation with two binaries, **When** the user upgrades via `cc-deck plugin install`, **Then** the old `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` are removed and replaced by `cc_deck.wasm`.
3. **Given** the install completes, **When** inspecting generated config files, **Then** both `load_plugins` (config.kdl) and `default_tab_template` (layout.kdl) reference `cc_deck.wasm` with different `mode` config values.

---

### User Story 3 - Controller and Sidebar Behavior Unchanged (Priority: P1)

An existing cc-deck user upgrades to the single-binary version. All functionality works identically: session tracking, hook processing, sidebar rendering, keyboard navigation, mouse interaction, rename, delete, pause, and attend. The controller and sidebar roles are determined at runtime from KDL configuration, not at compile time.

**Why this priority**: The merge must be transparent to users. No behavioral regressions are acceptable.

**Independent Test**: Run the full existing test suite and perform manual verification of all interactive features (navigate mode, rename, delete, attend, session switching).

**Acceptance Scenarios**:

1. **Given** a running Zellij session with cc-deck, **When** Claude Code sessions are active, **Then** the sidebar displays session status identically to the two-binary version.
2. **Given** the plugin loads with `mode "controller"` in config, **When** hook events arrive, **Then** the controller processes them and broadcasts render payloads to sidebars.
3. **Given** the plugin loads with `mode "sidebar"` in config, **When** the user interacts via mouse or keyboard, **Then** the sidebar handles input locally and forwards actions to the controller.
4. **Given** no `mode` is specified in config, **When** the plugin loads, **Then** it defaults to sidebar behavior (backward compatibility).

---

### Edge Cases

- What happens when the user has a mixed installation (old two-binary files alongside the new single binary)?
- How does the system behave if `mode` is omitted from all config entries?
- What happens if two controller instances are accidentally configured (duplicate `load_plugins` entries)? The second controller SHOULD log a warning but continue running.
- How does Zellij handle stale permission cache entries from the old two-binary installation?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The plugin MUST determine its role (controller or sidebar) at runtime from KDL configuration, not at compile time via feature flags.
- **FR-002**: The plugin MUST default to sidebar behavior when no `mode` configuration value is present.
- **FR-003**: The build system MUST produce a single WASM binary (`cc_deck.wasm`) instead of two separate binaries.
- **FR-004**: The build pipeline (`make install`) MUST write one WASM binary to the plugins directory and remove any legacy two-binary files (`cc_deck_controller.wasm`, `cc_deck_sidebar.wasm`). No runtime backward-compatibility detection is needed.
- **FR-005**: The install command MUST generate config.kdl with `load_plugins` referencing `cc_deck.wasm` with `mode "controller"` and layout.kdl with `default_tab_template` referencing `cc_deck.wasm` with `mode "sidebar"`.
- **FR-006**: The build pipeline MUST remove permission pre-population workarounds (the `permissions.kdl` creation hack for background plugins).
- **FR-011**: The Go CLI MUST simplify to a single embedded binary with a single `Binary` field and `BinarySize` in the `PluginInfo` struct. All controller/sidebar-specific embed fields MUST be removed.
- **FR-007**: All existing inter-plugin communication (render broadcasts, action messages, sidebar discovery protocol) MUST continue to work unchanged using `destination_plugin_id` targeting.
- **FR-008**: The CLI hook command MUST continue to route hook events via broadcast (`zellij pipe --name cc-deck:hook`). Sidebar instances MUST ignore hook messages they receive.
- **FR-009**: The single binary MUST include all code for both controller and sidebar roles (no conditional compilation via feature flags for role selection).
- **FR-010**: The plugin MUST subscribe to events selectively based on its runtime mode. Controller mode subscribes to heavy events (PaneUpdate, TabUpdate, Timer, RunCommandResult, etc.). Sidebar mode subscribes only to Mouse and Key events. This preserves the performance benefit of the architecture split.

### Key Entities

- **Plugin Mode**: Runtime role selection ("controller" or "sidebar") read from KDL configuration at load time.
- **WASM Binary**: Single `cc_deck.wasm` file containing both controller and sidebar logic, used by both `load_plugins` and `default_tab_template`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can install cc-deck and have permissions granted through one dialog interaction, with zero manual file edits required.
- **SC-002**: The build pipeline produces one WASM binary instead of two, reducing install artifacts by 50%.
- **SC-003**: All existing automated tests pass without modification (aside from removing feature-flag-specific test configuration).
- **SC-004**: All interactive features (navigate, rename, delete, attend, session switching, filtering) work identically to the two-binary version.
- **SC-005**: Upgrade from two-binary to single-binary installation completes cleanly with legacy files removed.

## Clarifications

### Session 2026-04-01

- Q: Should the CLI hook command target the controller by plugin_id, or keep broadcasting to all instances? → A: Keep broadcasting. Sidebars ignore hook messages with negligible overhead. No CLI change needed for hook routing.
- Q: What should happen if two controller instances are accidentally configured? → A: Log a warning at startup if another controller is detected, but continue running.
- Q: How should upgrade cleanup of old two-binary files be handled? → A: Part of `make install` (Makefile), not runtime detection in the Go CLI. No backward compatibility logic needed.
- Q: Should the plugin subscribe to all events regardless of mode, or selectively based on mode? → A: Selective subscription. Controller subscribes to heavy events (PaneUpdate, TabUpdate, Timer, etc.), sidebar subscribes to Mouse/Key only. Preserves the performance benefit of the split.
- Q: Should the Go CLI PluginInfo struct keep separate controller/sidebar fields for a potential re-split? → A: Simplify fully. Single `Binary` field and `BinarySize`, remove all controller/sidebar-specific fields. YAGNI.

## Assumptions

- Zellij creates independent plugin instances when the same WASM binary is referenced in both `load_plugins` and `default_tab_template` with different configuration values. (Pending confirmation from Zellij maintainer on zellij-org/zellij#4982, but strongly implied by the maintainer's suggestion of this exact pattern.)
- Zellij's permission cache is keyed by plugin URL (file path), so granting permissions to the sidebar instance covers the controller instance using the same binary.
- The combined binary size will be comparable to the larger of the two current binaries, since most code is shared between controller and sidebar.
- Pipe message routing using `destination_plugin_id` (already implemented) works identically regardless of whether plugins share a WASM URL.
- The constitution's WASM filename convention (`cc_deck.wasm`, underscore) applies to the merged binary.
- Legacy code cleanup (removing old `sync.rs`, old `PluginState`, fixing test fixtures for zellij-tile 0.44) is out of scope for this feature and will be handled separately.
