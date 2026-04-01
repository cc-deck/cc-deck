# Feature Specification: Single Binary Merge

**Feature Branch**: `031-single-binary-merge`
**Created**: 2026-04-01
**Status**: Draft
**Input**: Brainstorm documents: `brainstorm/031-single-binary-merge.md`, `brainstorm/030-single-instance-architecture.md`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Single Binary Runtime Mode Dispatch (Priority: P1)

A developer installs cc-deck and gets a single WASM binary (`cc_deck.wasm`) that serves both the controller and sidebar roles. The binary reads its `mode` configuration parameter at load time and activates the appropriate behavior. The controller instance runs as a background plugin via `load_plugins`, and sidebar instances run per-tab via `default_tab_template`, all from the same binary file.

**Why this priority**: This is the core architectural change. Everything else (channel naming, legacy cleanup) depends on the single binary being in place. It also directly solves the background plugin permission problem: since both roles share the same plugin URL, Zellij's URL-keyed permission cache covers all instances after a single grant.

**Independent Test**: Build the single binary, configure layout with `mode "sidebar"` in `default_tab_template` and `mode "controller"` in `load_plugins`. Start Zellij. Verify the controller processes events and broadcasts render payloads, while sidebars receive and display them.

**Acceptance Scenarios**:

1. **Given** a fresh install with `cc_deck.wasm`, **When** Zellij starts with the cc-deck layout, **Then** one controller instance and one sidebar instance per tab are running, all from the same WASM file
2. **Given** the controller instance loaded via `load_plugins`, **When** it reads `config.get("mode")`, **Then** it returns `"controller"` and the instance activates controller behavior (event subscription, state management, render broadcasting)
3. **Given** a sidebar instance loaded via `default_tab_template`, **When** it reads `config.get("mode")`, **Then** it returns `"sidebar"` and the instance activates sidebar behavior (render display, mouse/keyboard handling)
4. **Given** the sidebar instance receives the permission dialog on a new tab, **When** the user grants permissions, **Then** the controller instance (same URL) inherits the cached permissions and functions correctly

---

### User Story 2 - Named Pipe Channel Protocol (Priority: P1)

All pipe messages between controller and sidebar use a named channel convention with target-encoded prefixes: `cc-deck:ctrl:*` for controller-bound messages, `cc-deck:side:*` for sidebar-bound messages. Each mode's pipe handler ignores messages with the wrong prefix. This makes the protocol self-documenting and handles the self-delivery issue (single URL means broadcasts reach all instances).

**Why this priority**: With a single binary URL, pipe broadcasts reach all instances including the sender. The channel prefix convention is essential for correct message routing and is a prerequisite for the binary merge working correctly.

**Independent Test**: Trigger a hook event from the CLI. Verify the controller receives `cc-deck:ctrl:hook` and processes it, while sidebar instances receive and ignore it. Trigger a render broadcast. Verify sidebars receive `cc-deck:side:render` and display it, while the controller receives and ignores it.

**Acceptance Scenarios**:

1. **Given** the controller broadcasts `cc-deck:side:render`, **When** the controller's own pipe handler receives the message, **Then** it ignores it (wrong prefix for controller mode)
2. **Given** a sidebar sends `cc-deck:ctrl:action`, **When** all instances receive the broadcast, **Then** only the controller processes it; sidebars ignore the `ctrl:` prefix
3. **Given** the CLI sends `cc-deck:ctrl:hook` via `zellij pipe`, **When** all instances receive it, **Then** only the controller handles the hook event
4. **Given** the controller sends `cc-deck:side:init` targeted to a specific sidebar, **When** other sidebars receive the broadcast, **Then** they ignore it (wrong destination plugin ID)

---

### User Story 3 - Legacy Code Removal (Priority: P2)

The old unified PluginState, sync.rs module, and associated test fixtures are removed from the codebase. The no-feature-flag fallback path in main.rs is eliminated. Only the controller and sidebar_plugin modules remain, compiled unconditionally into the single binary.

**Why this priority**: Dead code removal is lower risk than the architectural changes but should happen in the same feature to avoid carrying unused code forward. The sync module is particularly important to remove since it represents an entire subsystem that the controller/sidebar split already made obsolete.

**Independent Test**: Build the single binary and run `cargo test`. Verify no compilation errors from missing modules. Verify the binary size is smaller than the sum of the previous two binaries.

**Acceptance Scenarios**:

1. **Given** the single binary is built, **When** checking for sync.rs, **Then** the file does not exist in the source tree
2. **Given** the single binary is built, **When** checking for old PluginState struct, **Then** it is not present in any source file
3. **Given** the no-feature-flag fallback in main.rs, **When** the single binary is built, **Then** no `#[cfg(not(any(feature = "controller", feature = "sidebar")))]` blocks exist
4. **Given** the old state_machine_tests, **When** checking the test files, **Then** tests that depend on removed PluginState are removed or rewritten against the new architecture
5. **Given** the legacy modules (attend.rs, notification.rs, rename.rs, fuzz_tests.rs), **When** checking the source tree, **Then** none of these files exist (their functionality is either integrated into controller/sidebar modules or no longer needed)

---

### User Story 4 - Simplified Build and Install Pipeline (Priority: P2)

The Makefile has a single `build-wasm` target instead of separate controller/sidebar targets. The Go CLI embeds one WASM binary instead of two. The install command writes one file to the Zellij plugins directory. Layout generation references a single plugin URL for both roles.

**Why this priority**: Build simplification is a direct benefit of the single binary and reduces maintenance burden. It also eliminates the need for the `permissions.kdl` pre-population hack since only one URL needs permission grants.

**Independent Test**: Run `make install` and verify a single `cc_deck.wasm` is installed to `~/.config/zellij/plugins/`. Verify the generated layout references `cc_deck.wasm` for both the sidebar pane and the controller `load_plugins` block.

**Acceptance Scenarios**:

1. **Given** the developer runs `make install`, **When** the build completes, **Then** exactly one WASM file (`cc_deck.wasm`) is copied to the Go embed directory and installed to Zellij plugins
2. **Given** the Go CLI's embed.go, **When** inspecting the file, **Then** it contains a single `//go:embed cc_deck.wasm` directive (no controller/sidebar split)
3. **Given** the install command, **When** it writes the layout, **Then** both the `load_plugins` block and `default_tab_template` reference `cc_deck.wasm` with different `mode` config values
4. **Given** the install command, **When** it runs, **Then** it does NOT write or modify `permissions.kdl` (permission pre-population removed)
5. **Given** a previous installation with two-binary architecture, **When** the user runs `cc-deck plugin install`, **Then** old files (`cc_deck_controller.wasm`, `cc_deck_sidebar.wasm`) are removed from the Zellij plugins directory and replaced with the single `cc_deck.wasm`

---

### User Story 5 - Permission Grant on New Tab (Priority: P3)

On first launch, the sidebar in the initial tab renders empty because Zellij does not show the permission dialog on the first tab's template plugins. When the user opens a second tab, the permission dialog appears. After granting permissions once, all instances (controller + all sidebars) work correctly. This is documented as a one-time first-launch behavior.

**Why this priority**: This is an accepted limitation of Zellij, not a bug in cc-deck. Documenting it is sufficient. The single binary makes this simpler than the two-binary case (one permission grant covers everything vs two separate grants).

**Independent Test**: Start Zellij with cc-deck layout on a clean install (no permission cache). Observe empty sidebar on first tab. Open a second tab and grant permissions. Verify all sidebars and the controller function correctly afterward.

**Acceptance Scenarios**:

1. **Given** a clean install with no permission cache, **When** Zellij starts, **Then** the first tab's sidebar shows a "waiting for permissions" or empty state
2. **Given** the empty first tab sidebar, **When** the user opens a new tab, **Then** Zellij shows the permission dialog for `cc_deck.wasm`
3. **Given** permissions are granted, **When** the user returns to the first tab, **Then** the sidebar displays the session list correctly
4. **Given** permissions were granted in a previous session, **When** Zellij starts a new session, **Then** all instances work immediately (cached permissions)

---

### Edge Cases

- What happens if `config.get("mode")` returns an unrecognized value or is missing? The plugin defaults to sidebar mode (safe fallback, the user sees a renderer rather than nothing).
- What happens if the controller is not loaded (missing `load_plugins` in config.kdl)? Sidebars show "loading..." indefinitely since no render payload arrives. The install command must always generate the `load_plugins` block.
- What happens during upgrade from the two-binary architecture? The install command removes `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` from the Zellij plugins directory and writes the single `cc_deck.wasm`.
- What happens if old pipe names (without channel prefix) are received? The handler ignores them (no match in the pipe name routing).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST produce a single WASM binary (`cc_deck.wasm`) that determines its role from the `mode` configuration parameter at runtime
- **FR-002**: The system MUST support two mode values: `"controller"` (headless background plugin) and `"sidebar"` (per-tab renderer), defaulting to `"sidebar"` for unrecognized or missing values
- **FR-003**: All pipe messages MUST use a named channel convention with target prefix: `cc-deck:ctrl:*` for controller-bound messages, `cc-deck:side:*` for sidebar-bound messages
- **FR-004**: Each mode's pipe handler MUST ignore messages with the wrong prefix (controller ignores `cc-deck:side:*`, sidebar ignores `cc-deck:ctrl:*`)
- **FR-005**: The CLI hook command MUST send hooks via broadcast pipe (`zellij pipe --name cc-deck:ctrl:hook`), without `--plugin` targeting, to avoid URL mismatch issues that spawn duplicate plugin instances
- **FR-006**: The Cargo.toml MUST define a single `[[bin]]` target with no feature flags for binary selection
- **FR-007**: Both controller and sidebar modules MUST be compiled unconditionally (no `#[cfg(feature)]` gates for module inclusion)
- **FR-008**: The legacy sync module (`sync.rs`), old `PluginState` struct, associated test fixtures (`state_machine_tests.rs`, `fuzz_tests.rs`), and obsolete modules (`attend.rs`, `notification.rs`, `rename.rs`, `sidebar.rs`, `state.rs`) MUST be removed
- **FR-009**: The Go CLI MUST embed a single WASM binary via one `//go:embed cc_deck.wasm` directive
- **FR-010**: The install command MUST write one WASM file to the Zellij plugins directory and remove any old two-binary files (`cc_deck_controller.wasm`, `cc_deck_sidebar.wasm`) if present
- **FR-011**: Layout generation MUST reference `cc_deck.wasm` for both the sidebar pane (in `default_tab_template`) and the controller (in `load_plugins`), differentiated by `mode` config
- **FR-012**: The install command MUST NOT write or modify `permissions.kdl` (permission pre-population removed)
- **FR-013**: The Makefile MUST have a single `build-wasm` target that produces `cc_deck.wasm`

### Key Entities

- **PluginMode**: Runtime enum (`Controller`, `Sidebar`) parsed from `config.get("mode")` during `load()`. Determines which `ZellijPlugin` implementation is activated.
- **Pipe Channel**: Named message channel with target prefix (`cc-deck:ctrl:*` or `cc-deck:side:*`). Each channel name encodes its intended recipient role.

### Pipe Channel Mapping

| Old Name | New Name | Direction |
|----------|----------|-----------|
| `cc-deck:hook` | `cc-deck:ctrl:hook` | CLI -> Controller |
| `cc-deck:action` | `cc-deck:ctrl:action` | Sidebar -> Controller |
| `cc-deck:sidebar-hello` | `cc-deck:ctrl:hello` | Sidebar -> Controller |
| `cc-deck:attend` | `cc-deck:ctrl:attend` | Keybinding -> Controller |
| `cc-deck:attend-prev` | `cc-deck:ctrl:attend-prev` | Keybinding -> Controller |
| `cc-deck:render` | `cc-deck:side:render` | Controller -> Sidebars |
| `cc-deck:sidebar-init` | `cc-deck:side:init` | Controller -> Sidebar |
| `cc-deck:sidebar-reindex` | `cc-deck:side:reindex` | Controller -> Sidebars |
| `cc-deck:navigate` | `cc-deck:side:navigate` | Controller -> Sidebar |
| `cc-deck:navigate-prev` | `cc-deck:side:navigate-prev` | Controller -> Sidebar |

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `make install` produces and installs exactly one WASM binary (`cc_deck.wasm`)
- **SC-002**: All existing sidebar functionality (session display, switching, renaming, deleting, pausing, filtering, keyboard navigation) works identically to the two-binary architecture
- **SC-003**: `cargo test` passes with no references to removed modules (sync.rs, old PluginState)
- **SC-004**: The Go CLI embed.go contains exactly one `//go:embed` directive for the plugin binary
- **SC-005**: No `#[cfg(feature = "controller")]` or `#[cfg(feature = "sidebar")]` exists in the codebase
- **SC-006**: All 10 pipe message names use the new `ctrl:`/`side:` channel prefix convention
- **SC-007**: The install command does not write or reference `permissions.kdl`

## Assumptions

- Zellij 0.44's permission cache is keyed by plugin URL string, so a single binary URL covers all instances regardless of config parameters
- Both controller and sidebar modules can coexist in a single binary without code conflicts (they already share types via lib.rs)
- The binary size of the unified binary will be smaller than the sum of the two individual binaries (shared code compiled once)
- Zellij delivers pipe messages to all instances matching a `plugin_url`, so the channel prefix filtering is necessary for correct routing
- The first-tab permission dialog limitation is a Zellij behavior, not something cc-deck can fix at the plugin level
