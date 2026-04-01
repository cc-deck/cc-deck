# Review Guide: Single-Instance Architecture (030)

## Overview

This feature splits the cc-deck Zellij plugin from a single WASM binary (one instance per tab) into two binaries: a background controller (single instance, owns all state) and a thin sidebar renderer (one per tab, displays cached payloads). The primary goal is eliminating O(N) event processing overhead that causes UI sluggishness at 10+ tabs.

## Architecture Summary

```
CLI hooks ──> cc-deck:hook ──> Controller (single instance)
                                    │
Zellij events ──> PaneUpdate ──> Controller
                   TabUpdate        │
                   Timer            v
                               Process state, build RenderPayload
                                    │
                               cc-deck:render broadcast
                                    │
                    ┌───────────────┼───────────────┐
                    v               v               v
              Tab 1 sidebar   Tab 2 sidebar   Tab N sidebar
              (thin render)   (thin render)   (thin render)
```

## Key Review Areas

### 1. Pipe Protocol (HIGH PRIORITY)

**Files**: `cc-zellij-plugin/src/controller/render_broadcast.rs`, `cc-zellij-plugin/src/sidebar/mod.rs`, `cc-zellij-plugin/src/lib.rs`

**What to check**:
- All pipe messages use broadcast+filter pattern (NO plugin_url targeting). See `specs/030-single-instance-arch/contracts/pipe-protocol.md`.
- Controller ignores its own broadcast messages (cc-deck:render, cc-deck:navigate)
- Sidebars ignore controller-only messages (cc-deck:hook, cc-deck:action, cc-deck:sidebar-hello)
- RenderPayload serialization is backwards-compatible (new fields default correctly)

**Why this matters**: plugin_url targeting is broken in Zellij (matches URL+config, causes spurious floating panes). The broadcast+filter pattern is the only reliable approach.

### 2. Controller State Management (HIGH PRIORITY)

**Files**: `cc-zellij-plugin/src/controller/state.rs`, `cc-zellij-plugin/src/controller/events.rs`, `cc-zellij-plugin/src/controller/hooks.rs`

**What to check**:
- Controller is the SOLE writer to sessions BTreeMap
- Render coalescing works (100ms debounce for automatic events, immediate for user actions)
- Persistence uses single-writer pattern (no merge_sessions, no timestamp dominance)
- Startup grace period prevents premature session cleanup
- PID-based stale cache detection on reattach

**Why this matters**: This replaces the entire 3-level sync system (~527 LOC). Any regression here means data loss or inconsistent state.

### 3. Sidebar Mode State Machine (MEDIUM PRIORITY)

**Files**: `cc-zellij-plugin/src/sidebar/modes.rs`, `cc-zellij-plugin/src/sidebar/input.rs`, `cc-zellij-plugin/src/sidebar/rename.rs`

**What to check**:
- All mode transitions from current state.rs are preserved (Passive, Navigate, NavigateFilter, NavigateDeleteConfirm, NavigateRename, RenamePassive, Help)
- Grace period mechanism (300ms ENTER_GRACE_MS) prevents stale PaneUpdate auto-exit
- Focus loss auto-exits navigate mode
- Mode state is entirely sidebar-local (no controller involvement)

**Why this matters**: The mode state machine is complex and any missing transition means broken UX.

### 4. Sidebar Discovery Protocol (MEDIUM PRIORITY)

**Files**: `cc-zellij-plugin/src/controller/sidebar_registry.rs`, `cc-zellij-plugin/src/sidebar/mod.rs`

**What to check**:
- Hello/init handshake only triggers after first render payload (not on load, avoids race)
- Tab reindexing broadcasts cc-deck:sidebar-reindex on tab count change
- Dead sidebar cleanup removes registry entries for missing plugin_ids
- Sidebar handles missing my_tab_index gracefully (renders anyway, just cannot self-filter)

### 5. Build System (MEDIUM PRIORITY)

**Files**: `cc-zellij-plugin/Cargo.toml`, `Makefile`, `cc-deck/internal/plugin/embed.go`

**What to check**:
- Two `[[bin]]` targets with required-features gates
- Feature flags: `controller` and `sidebar` are mutually exclusive in practice
- Makefile builds both binaries and copies both to Go embed location
- WASM filenames use underscores: `cc_deck_controller.wasm`, `cc_deck_sidebar.wasm`
- `make install` produces working two-binary setup

### 6. Go CLI Changes (LOW PRIORITY)

**Files**: `cc-deck/internal/plugin/install.go`, `cc-deck/internal/plugin/layout.go`, `cc-deck/internal/cmd/hook.go`

**What to check**:
- Install writes both WASM files, removes old single binary
- Layout generation adds `load_plugins` block for controller
- Hook command adds `--plugin` flag for controller targeting
- Status reports both binaries

### 7. Sync Elimination (LOW PRIORITY)

**What to check**:
- sync.rs is completely deleted
- No references to sync_dirty, session-meta.json, merge_sessions, broadcast_and_save remain
- No references to cc-deck:sync or cc-deck:request pipe messages remain
- /cache/last_click file operations removed (double-click detection is sidebar-local now)

## Test Strategy

- **Unit tests**: Controller state transitions, sidebar mode transitions, protocol serialization
- **Integration tests**: Controller-sidebar render payload flow, action message processing
- **Manual testing**: Launch with 15+ tabs, verify responsiveness, verify state consistency across tabs

## Constitution Compliance

| Principle | Compliance |
|-----------|-----------|
| II. Plugin Installation via make install | Must work with two binaries |
| III. WASM Filename Convention | Underscores: cc_deck_controller.wasm |
| IV. WASM Host Function Gating | All host functions cfg-gated |
| V. Zellij API Research | plugin_url targeting avoided per research |
| VI. Build via Makefile | No direct cargo build |
| VII. Interface Contracts | Pipe protocol contract defined |
| IX. Documentation Freshness | README updated |

## Risk Areas

1. **Zellij background plugin permissions**: Background plugins need explicit permission grants on first load. Verify the UX flow for granting permissions to both controller and sidebar.
2. **Pipe message ordering**: While Zellij guarantees in-tick ordering, verify that render payloads arrive before sidebar-init responses during startup.
3. **Tab reindexing race**: If a tab is added while a reindex is in progress, sidebars could briefly have wrong tab assignments. Verify the reindex protocol handles this.
