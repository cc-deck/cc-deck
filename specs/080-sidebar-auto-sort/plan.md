# Implementation Plan: Sidebar Auto-Sort with Pause Zone

**Branch**: `080-sidebar-auto-sort` | **Date**: 2026-07-11 | **Spec**: [spec.md](spec.md)

## Summary

Add automatic two-zone sidebar ordering: active sessions on top, paused sessions on bottom, with a visual separator line between them. The auto-sort is virtual (display-only), configurable via KDL plugin parameter (`auto_sort`, default true), and coexists with the existing manual S keybinding for three-tier sorting within the active zone.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Testing**: `make test` (cargo test + go test), `make verify` (tests + lint)
**Target Platform**: WASM plugin for Zellij terminal multiplexer

## Global Constraints

- Rust edition 2021, target `wasm32-wasip1`
- zellij-tile 0.43.1, serde/serde_json 1.x
- Build via `make install`, `make test`, `make verify`. Never `cargo build` directly.
- Auto-sort is virtual (display-only). No physical tab reordering.
- `auto_sort` config follows the existing KDL parameter pattern (`sidebar_width`, etc.)

## File Structure

| File | Responsibility |
|------|---------------|
| `cc-zellij-plugin/src/config.rs` | Add `auto_sort: bool` to `PluginConfig`, parse from KDL |
| `cc-zellij-plugin/src/lib.rs` | Add `separator_after_index: Option<usize>` to `RenderPayload` |
| `cc-zellij-plugin/src/controller/render_broadcast.rs` | Active/paused partition logic in `build_render_payload()` |
| `cc-zellij-plugin/src/controller/state.rs` | Add `auto_sort_tail: Vec<u32>` for tracking recently-unpaused session ordering |
| `cc-zellij-plugin/src/controller/actions.rs` | Update `handle_pause()` and auto-unpause to maintain `auto_sort_tail`; update `handle_sort()` for coexistence |
| `cc-zellij-plugin/src/sidebar_plugin/render.rs` | Render separator line between active and paused zones |

## Constitution Check

- **Tests**: Required. Unit tests for auto-sort ordering logic and config parsing.
- **Documentation**: User-facing behavior change (new config option, separator line). README and config reference need updates.
- **Build rules**: Use `make test`, `make verify`. Never `cargo build` directly.

## Research Findings

### Key Integration Points

1. **`build_render_payload()`** in `controller/render_broadcast.rs` (line 13): This is where session ordering happens. Currently sorts by `sort_order` (if set by manual S sort) or `tab_index`. Auto-sort inserts the active/paused partition here.

2. **`PluginConfig`** in `config.rs` (line 16): Struct with KDL plugin parameters. Has `sidebar_width` as the pattern for adding `auto_sort: bool`.

3. **`RenderPayload`/`RenderSession`** in `lib.rs` (line 36/15): `RenderSession` already has `paused: bool`. The payload needs a new field to tell the sidebar where to draw the separator.

4. **`sort_order: Option<Vec<u32>>`** in `controller/state.rs` (line 131): The manual sort stores pane IDs in sorted order. Auto-sort must respect this within the active zone when both are active.

5. **`sort_tier()`** in `controller/actions.rs` (line 271): Classifies sessions into tiers (0=Working/Waiting, 1=Idle/Done/Init, 2=Paused). The auto-sort uses a simplified version: tier 0 = not paused, tier 1 = paused.

### Rendering Path

The sidebar receives `RenderPayload` via pipe message, deserializes it, and renders each `RenderSession` in order. To draw a separator, the payload needs to indicate where the separator should appear. Two options:
- **A**: Add `separator_after_index: Option<usize>` to `RenderPayload` (controller computes it)
- **B**: Let the sidebar detect the boundary by scanning `paused` fields

Option A is cleaner (controller owns ordering logic, sidebar just renders).

### Config Flow

KDL layout parameter `auto_sort "true"` -> `PluginConfig::from_configuration()` -> `state.config.auto_sort` -> checked in `build_render_payload()`.

## Implementation Approach

### Phase 1: Config + Ordering Logic (Rust plugin)

Add `auto_sort: bool` to `PluginConfig` with default `true`. Add `auto_sort_tail: Vec<u32>` to `ControllerState` to track sessions that were recently unpaused (FR-002 requires these to appear at the end of the active zone, not at their original tab position).

In `handle_pause()` (actions.rs:146): when a session transitions from paused to not-paused, append its pane_id to `auto_sort_tail`. When it transitions from not-paused to paused, remove it from `auto_sort_tail`. Same for auto-unpause on switch (actions.rs:37-38).

In `build_render_payload()`, after sorting by sort_order or tab_index, apply the active/paused partition: stable-partition sessions so non-paused come first, paused come last. Within the active zone, sessions whose pane_id is in `auto_sort_tail` are moved to the end (in `auto_sort_tail` order), ensuring unpaused sessions appear at the bottom of the active zone per FR-002. Compute `separator_after_index`. Add the field to `RenderPayload`.

### Phase 2: Separator Rendering (Rust sidebar)

In the sidebar rendering code, check `payload.separator_after_index`. If set, draw a thin horizontal line after that session entry. The separator consumes one row of sidebar height.

### Phase 3: Manual Sort Coexistence

When the S keybinding triggers `handle_sort()`, the three-tier sort applies within the active zone only (filter out paused sessions from the sort targets). The auto-sort partition is reapplied after the manual sort.

### Phase 4: Documentation

Update README.md with the auto-sort feature description. Update configuration reference (`docs/modules/reference/pages/configuration.adoc`) with the `auto_sort` KDL parameter. Update CLI reference if the cc-deck CLI gains a `sidebar.auto_sort` config option.
