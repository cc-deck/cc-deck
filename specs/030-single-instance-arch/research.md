# Research: Single-Instance Architecture

## Decision: Pipe Targeting Strategy

**Decision**: Use broadcast+filter pattern for all plugin-to-plugin communication. Use CLI `--plugin` flag for external hook routing.

**Rationale**: The current codebase explicitly avoids `plugin_url` targeting in `pipe_message_to_plugin()` because Zellij matches by both URL AND configuration. When configs differ (even slightly), the match fails and Zellij spawns a spurious floating pane. The broadcast pattern (no URL, no destination_plugin_id) is proven reliable across all Zellij versions the project supports.

**Alternatives considered**:
- `plugin_url` targeting: Rejected. Known broken in current code (main.rs lines 244-254, explicit comment).
- `destination_plugin_id` targeting: Viable for controller→specific-sidebar, but requires tracking all sidebar plugin_ids. Adds complexity without meaningful performance benefit since pipe messages are lightweight.
- Named pipe channels: Not a Zellij feature.

**Implementation**: All pipe messages use `pipe_message_to_plugin(MessageToPlugin::new(name))` with no URL or destination. Controller and sidebars filter by message name prefix. CLI hook routing uses `zellij pipe --plugin "file:.../cc_deck_controller.wasm"` which routes correctly from external commands.

## Decision: Two-Binary Build Strategy

**Decision**: Use Cargo feature flags with two `[[bin]]` targets sharing `src/lib.rs` for common types.

**Rationale**: Keeps the codebase unified. Common types (Session, Activity, RenderPayload, ActionMessage, HookPayload) live in `src/lib.rs`. Controller-specific code (state management, event handling, git detection) and sidebar-specific code (rendering, mouse/key handling, mode management) are feature-gated in their respective modules.

**Alternatives considered**:
- Two separate crates in a workspace: More isolation but duplicates type definitions. Cross-crate dependencies add build complexity.
- Single binary with runtime mode: Simpler build but loads unnecessary code in each instance. WASM size matters for plugin load time.

## Decision: Event Subscription Split

**Decision**: Controller subscribes to PaneUpdate, TabUpdate, Timer, RunCommandResult, PaneClosed, CommandPaneOpened. Sidebar subscribes to Mouse, Key only.

**Rationale**: This is the core performance optimization. PaneUpdate and TabUpdate fire on every pane/tab change across the entire Zellij session. Under the current architecture, N sidebar instances each process these events (O(N)). With the split, only the controller processes them (O(1)). Mouse and Key events are already per-pane, so they naturally belong to the sidebar that receives them.

**Alternatives considered**:
- Sidebar subscribes to PaneUpdate for self-filtering: Rejected. Defeats the purpose of reducing WASM calls.
- Controller subscribes to Mouse/Key and forwards: Rejected. Adds pipe round-trip latency to every user interaction.

## Decision: Sidebar State Management

**Decision**: Sidebar maintains local interactive state (SidebarMode, cursor, scroll, filter, click regions, notifications). All session data comes from cached render payloads.

**Rationale**: Interactive state is inherently per-instance (different tabs can have different cursor positions, filter states, or mode). Sending keystrokes to the controller and waiting for a response would add perceptible latency. The sidebar caches the last render payload and applies local transformations (filtering, cursor highlight) without round trips.

**Alternatives considered**:
- Controller manages all state including mode: Rejected. Adds latency to every keypress. Mode state is per-tab, not global.
- Sidebar maintains session state copy: Rejected. This is the current architecture that creates sync problems.

## Decision: Controller Crash Recovery

**Decision**: Controller restores from `/cache/sessions.json` on restart. Sidebars detect absence of render updates via timeout and display "controller unavailable" status. Upon receiving the next render payload after recovery, sidebars reconnect automatically.

**Rationale**: The controller already persists state to `/cache/sessions.json` (inherited from current single-instance persistence). Sidebars have no persistent state to recover. The timeout detection gives users visibility into the failure state rather than showing stale data.

## Decision: Render Payload Coalescing

**Decision**: Controller coalesces rapid state changes within a 100ms window before broadcasting render payloads. Immediate sync for user-initiated actions (rename, delete, pause).

**Rationale**: Inherits the debounce pattern from the current `sync_dirty` flag and 1-second timer. Shortened to 100ms because the controller processes events faster (no N-instance overhead). User-initiated actions bypass the coalesce window for immediate feedback.

## Decision: Keybinding Registration

**Decision**: Controller registers keybindings using `reconfigure()` with `MessagePluginId` targeting its own plugin_id. Keybinding pipe messages arrive at the controller, which either handles them directly (attend, new session) or forwards to the active-tab sidebar.

**Rationale**: Current code already uses this pattern (only the active-tab instance registers keybindings). Moving registration to the controller simplifies the logic since there is exactly one controller. The controller knows the active tab from TabUpdate events and can forward navigation commands to the correct sidebar.

## Decision: Migration Strategy

**Decision**: Clean replacement during `cc-deck plugin install`. Remove old `cc_deck.wasm` single binary, write both `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm`. No data migration needed.

**Rationale**: The cache file format (`/cache/sessions.json`) is unchanged. The Session struct serialization is backwards-compatible. The only change is which binary reads/writes the file (controller instead of every instance).

## Codebase Metrics

| Component | Current LOC | Change |
|-----------|------------|--------|
| main.rs | 1860 | Split into controller.rs + sidebar.rs |
| state.rs | 736 | Split: controller keeps session state, sidebar keeps UI state |
| sync.rs | 527 | Eliminated entirely |
| session.rs | 252 | Moves to lib.rs (shared) |
| sidebar.rs | 611 | Moves to sidebar module |
| pipe_handler.rs | 198 | Split: hook parsing → controller, action parsing → both |
| attend.rs | 479 | Moves to controller module |
| rename.rs | 342 | Moves to sidebar module |
| git.rs | 156 | Moves to controller module |
| config.rs | 137 | Shared via lib.rs |
| perf.rs | 184 | Shared via lib.rs |
| notification.rs | 73 | Split: controller notifications in payload, sidebar local notifications |

**Net elimination**: ~527 LOC (sync.rs) + ~200 LOC (sync integration points across other files)
**New code**: ~300 LOC (render payload serialization, pipe protocol, sidebar hello/init handshake)
