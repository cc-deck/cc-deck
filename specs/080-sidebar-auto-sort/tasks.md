# Tasks: Sidebar Auto-Sort with Pause Zone

**Branch**: `080-sidebar-auto-sort` | **Generated**: 2026-07-11

## Dependencies

```
Task 1 (Config + ordering) ──> Task 2 (Separator rendering) ──> Task 3 (Manual sort coexistence + tests) ──> Task 4 (Documentation)
```

Sequential: each task builds on the previous. Task 4 can start after Task 3 passes `make verify`.

## Task List

- [X] **Task 1: Add auto_sort config and active/paused partition in render broadcast**

  **Files**: `cc-zellij-plugin/src/config.rs`, `cc-zellij-plugin/src/lib.rs`, `cc-zellij-plugin/src/controller/render_broadcast.rs`, `cc-zellij-plugin/src/controller/state.rs`, `cc-zellij-plugin/src/controller/actions.rs`

  **Interfaces produced**:
  - `PluginConfig::auto_sort: bool` (consumed by Task 1 partition logic, Task 3 sort coexistence)
  - `RenderPayload::separator_after_index: Option<usize>` (consumed by Task 2 sidebar rendering)
  - `ControllerState::auto_sort_tail: Vec<u32>` (consumed by Task 1 partition logic, updated by pause/unpause handlers)

  **What to do**:

  1. In `config.rs`, add `pub auto_sort: bool` to `PluginConfig` struct (after `perf_interval`, line ~45). Default to `true` in `Default` impl. Parse from KDL config in `from_configuration()`:
     ```rust
     auto_sort: config.get("auto_sort").map(|v| v != "false").unwrap_or(true),
     ```
     Add unit test for parsing (true by default, "false" disables, "true" enables).

  2. In `lib.rs`, add `#[serde(default)] pub separator_after_index: Option<usize>` to `RenderPayload` (after `sort_active`, line ~55). This tells the sidebar where to draw the separator line. `None` means no separator.

  3. In `state.rs`, add `pub auto_sort_tail: Vec<u32>` to `ControllerState` (after `sort_order`, line ~131). Default to `Vec::new()`. This tracks pane IDs of sessions that transitioned from paused to active, so they appear at the end of the active zone per FR-002.

  4. In `actions.rs`, update `handle_pause()` (line ~146): when toggling `s.paused` from `true` to `false` (unpausing), append `pane_id` to `state.auto_sort_tail`. When toggling from `false` to `true` (pausing), remove `pane_id` from `auto_sort_tail`. Also update the auto-unpause-on-switch block (line ~37-38): when `s.paused` is set to `false`, append `s.pane_id` to `state.auto_sort_tail`.

  5. In `controller/render_broadcast.rs`, in `build_render_payload()` (line ~13), after the existing sort-by-sort_order-or-tab_index block (lines 14-24): if `state.config.auto_sort` is true, apply a two-step reorder:
     a. Within the active (non-paused) sessions, move any whose pane_id is in `state.auto_sort_tail` to the end of the active group (preserving `auto_sort_tail` order among them). This satisfies FR-002: unpaused sessions appear at the bottom of the active zone.
     b. Stable-partition the full list so non-paused sessions come first and paused sessions come last.
     c. Compute `separator_after_index`: if there are both non-paused and paused sessions, set it to `(count_of_non_paused - 1)`. Otherwise `None`.

     The partition must be stable (preserve relative order within each group). Use the existing sorted order as input, so manual sort order (if active) is preserved within the active zone.

  **Acceptance**: `make test` passes. Config parsing tests cover auto_sort. RenderPayload serialization tests include separator_after_index. When auto_sort is false, no partition is applied and separator_after_index is None. When a session is unpaused, it appears at the end of the active zone (not its original tab position).

---

- [X] **Task 2: Render separator line in sidebar**

  **Files**: `cc-zellij-plugin/src/sidebar_plugin/render.rs`

  **Interfaces consumed**:
  - `RenderPayload::separator_after_index: Option<usize>` (from Task 1)

  **What to do**:

  1. In `render_sidebar()` (`sidebar_plugin/render.rs`, line ~23), locate the session rendering loop that iterates over the sessions returned by `state.filtered_sessions()` and renders each `RenderSession`.

  2. After rendering the session at index `i`, check if `payload.separator_after_index == Some(i)`. If so, render a separator line. The separator should be a thin horizontal rule using box-drawing character `─` (U+2500) repeated to fill the sidebar width, rendered in a dim/muted color (e.g., `\x1b[38;2;60;60;70m`).

  3. The separator line consumes one row of sidebar height. In the available-rows calculation (currently `content_end - content_start` at line ~59), subtract 1 when `separator_after_index` is `Some` to account for the separator row. When the sidebar truncates sessions because height is limited, the separator counts as one consumed row.

  4. When `separator_after_index` is `None`, no separator is rendered (auto-sort disabled or all sessions in one zone). No height adjustment needed.

  **Acceptance**: `make test` passes. With auto-sort enabled and at least one paused session, a separator line appears between the last active and first paused session. With all sessions active or all paused, no separator appears.

---

- [X] **Task 3: Manual sort coexistence and comprehensive tests**

  **Files**: `cc-zellij-plugin/src/controller/actions.rs`, `cc-zellij-plugin/src/controller/render_broadcast.rs`

  **Interfaces consumed**:
  - `PluginConfig::auto_sort: bool` (from Task 1)
  - `ControllerState::auto_sort_tail: Vec<u32>` (from Task 1)
  - `RenderPayload::separator_after_index: Option<usize>` (from Task 1)

  **What to do**:

  1. In `actions.rs`, update `handle_sort()` (line ~287): when `state.config.auto_sort` is true, the three-tier sort should only reorder sessions within the active zone. Paused sessions should not be included in the sort targets. The `sort_order` produced by `handle_sort` should list active sessions in tier-sorted order followed by paused sessions in their current relative order.

  2. In `render_broadcast.rs`, ensure the partition logic in `build_render_payload()` works correctly when both `sort_order` (from manual sort) and `auto_sort` are active: the sort_order already has paused sessions at the end (from step 1), so the partition is naturally satisfied.

  3. Add unit tests:
     - Auto-sort partitions sessions correctly (3 active, 2 paused -> active first, separator at index 2)
     - Auto-sort with no paused sessions -> no separator
     - Auto-sort with all paused -> no separator
     - Auto-sort preserves relative tab order within each zone (stable partition)
     - FR-002: session unpaused appears at end of active zone (auto_sort_tail ordering), not at original tab position
     - FR-002: session unpaused via auto-unpause-on-switch also appears at end of active zone
     - Manual sort + auto-sort: S keybinding sorts active zone by tier, paused zone stays at bottom
     - Auto-sort disabled: sessions in tab order, no separator
     - New session added while auto-sort active: appears in active zone
     - Session transitions from active to paused: moves below separator, removed from auto_sort_tail

  **Acceptance**: `make verify` passes. All test cases from the acceptance scenarios in the spec are covered by unit tests. Test for FR-002: unpaused session appears at end of active zone (via `auto_sort_tail`), not at original tab position.

---

- [X] **Task 4: Documentation updates**

  **Files**: `README.md`, `docs/modules/reference/pages/configuration.adoc`

  **What to do**:

  1. In `README.md`, add a description of the auto-sort feature in the sidebar features section. Describe the two-zone layout (active above, paused below separator), the default-on behavior, and how to disable it.

  2. In `docs/modules/reference/pages/configuration.adoc`, add the `auto_sort` KDL plugin parameter. Follow the existing format used by `sidebar_width` and other parameters. Document:
     - Parameter name: `auto_sort`
     - Type: string (`"true"` or `"false"`)
     - Default: `"true"` (enabled)
     - Effect: When enabled, paused sessions are automatically moved below a separator line in the sidebar. When disabled, all sessions appear in tab order with no separator.

  3. Use the prose plugin with the `cc-deck` voice profile for all documentation text.

  **Acceptance**: Documentation accurately describes the auto-sort feature. Configuration reference includes the `auto_sort` parameter with type, default, and description.
