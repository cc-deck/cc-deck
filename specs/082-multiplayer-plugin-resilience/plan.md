# Implementation Plan: Multiplayer Plugin Resilience

**Branch**: `082-multiplayer-plugin-resilience` | **Date**: 2026-07-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/082-multiplayer-plugin-resilience/spec.md`

## Summary

Add client_id awareness to the cc-deck Zellij plugin so that multiplayer sessions (multiple terminals attached to the same Zellij session) do not cause render broadcast storms, sidebar duplication, or controller election instability. The controller filters render broadcasts to only target sidebars from its own client, and uses (client_id, plugin_id) priority for leader election.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: N/A (in-memory registry, no persistent state changes)
**Testing**: `cargo test` (Rust), `make test` / `make lint`
**Target Platform**: WASM wasm32-wasip1 (Zellij plugin)
**Project Type**: Zellij plugin (dual-language monorepo: Go CLI + Rust plugin)
**Performance Goals**: Render broadcast completes in <10ms regardless of zombie count (SC-003)
**Constraints**: Zero regression on single-client operation (FR-010, SC-001); build via `make install`/`make test`/`make lint` only
**Scale/Scope**: 7 files modified, ~80 lines changed, 0 new files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + Documentation | PASS | Unit tests for registry dedup, broadcast filtering, election comparison. README update for multiplayer limitations. No new CLI commands/flags, no config changes. |
| II. Interface contracts | PASS | SidebarHello protocol modified with backward-compatible optional field. Contract document created. |
| III. Build and tool rules | PASS | Uses `make install`/`make test`/`make lint`. No direct cargo build. |
| IV. Plugin debug logging | N/A | Existing debug logging sufficient. |
| V. Command files | N/A | No build command files affected. |

## Project Structure

### Documentation (this feature)

```text
specs/082-multiplayer-plugin-resilience/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/
│   └── sidebar-registration.md
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/                    # Rust Zellij plugin
└── src/
    ├── lib.rs                       # MODIFY: Add client_id to SidebarHello
    ├── controller/
    │   ├── state.rs                 # MODIFY: Add client_id to ControllerState
    │   ├── mod.rs                   # MODIFY: Store client_id on permission grant, parse election ping
    │   ├── events.rs                # MODIFY: Include client_id in election ping payload
    │   ├── sidebar_registry.rs      # MODIFY: Registry type change, dedup logic, discover_sidebars
    │   ├── render_broadcast.rs      # MODIFY: Filter broadcast by client_id, guard untargeted fallback
    │   └── integration_tests.rs     # MODIFY: Update sidebar_registry value access for new tuple type
    └── sidebar_plugin/
        └── mod.rs                   # MODIFY: Include client_id in SidebarHello
```

**Structure Decision**: All changes are within the existing `cc-zellij-plugin/src/` tree. No new files needed. The sidebar plugin sends client_id; the controller stores, filters, and elects based on it.

## Global Constraints

- **Rust edition**: 2021
- **Target**: wasm32-wasip1
- **Build commands only**: `make install`, `make test`, `make lint` (never `cargo build` directly)
- **Plugin SDK**: zellij-tile 0.43.1
- **Serialization**: serde/serde_json 1.x
- **Backward compatibility**: All protocol changes must use `#[serde(default)]` for new optional fields
- **Zero regression**: All existing tests must pass without modification (SC-001, FR-010)

## Complexity Tracking

No constitution violations to justify.

## Implementation Strategy

### Layer 1: Protocol Change (FR-001, FR-002)

Add `client_id: u16` with `#[serde(default)]` to `SidebarHello` in `lib.rs`. Update the sidebar's `mod.rs` to read `get_plugin_ids().client_id` and include it in the hello payload.

### Layer 2: Controller State (FR-006)

Add `client_id: u16` field to `ControllerState` in `state.rs`. Set it from `get_plugin_ids().client_id` during permission grant in `mod.rs`.

### Layer 3: Registry Change (FR-003, FR-004, FR-009, FR-011)

Change `sidebar_registry` from `HashMap<u32, usize>` to `HashMap<u32, (usize, u16)>` in `state.rs`. Update `sidebar_registry.rs`:
- `handle_sidebar_hello`: extract `client_id` from hello, dedup by (tab_index, client_id)
- `discover_sidebars_from_manifest`: assign controller's `client_id` to auto-discovered entries
- `cleanup_dead_sidebars`: unchanged logic (still removes by plugin_id presence in manifest)

Update all call sites that access `sidebar_registry` values (currently `&usize`, will become `&(usize, u16)`).

### Layer 4: Render Broadcast Filtering (FR-005)

In `render_broadcast.rs`:
- `broadcast_render`: filter `sidebar_registry.keys()` to only include entries where `client_id == state.client_id`
- Guard `broadcast_render_all()` to only fire when registry is empty

### Layer 5: Election Hardening (FR-007, FR-008)

In `events.rs`:
- `broadcast_controller_ping`: change payload from `"{plugin_id}"` to `"{client_id}:{plugin_id}"`
- In `mod.rs` `PipeAction::ControllerPing` handler: parse `client_id:plugin_id`, compare as tuple `(client_id, plugin_id)` (lowest wins)
- Backward compatibility: if payload has no `:`, treat as plugin_id-only with client_id=0

### Layer 6: Tests

- Unit test: `SidebarHello` deserialization with and without `client_id`
- Unit test: registry dedup by (tab_index, client_id)
- Unit test: broadcast filtering by client_id
- Unit test: election comparison with (client_id, plugin_id) tuples
- Unit test: backward-compatible ping parsing
- Existing fuzz tests: no changes needed (fuzz tests don't exercise client_id paths)

### Layer 7: Documentation

- Update README.md with multiplayer session limitations

### Dependency Order

```
Layer 1 (protocol) → Layer 3 (registry)
Layer 2 (state)    → Layer 3 (registry)
Layer 3 (registry) → Layer 4 (broadcast)
Layer 1 (protocol) → Layer 5 (election)
Layer 2 (state)    → Layer 5 (election)
Layer 6 (tests) - after all code layers
Layer 7 (docs) - independent
```

Layers 1 and 2 can be developed in parallel. Layers 4 and 5 can be developed in parallel after Layer 3.
