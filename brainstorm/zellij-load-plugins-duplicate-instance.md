# Zellij Bug: `load_plugins` creates duplicate WASM instances

**Date**: 2026-05-14
**Status**: Draft issue, needs standalone reproducer
**Related**: [#4982](https://github.com/zellij-org/zellij/issues/4982), [PR #4990](https://github.com/zellij-org/zellij/pull/4990)
**Zellij version**: 0.44.1 (commit 120b649c)

## Summary

A single entry in `load_plugins` creates TWO WASM instances of the same plugin.
Both instances have identical `plugin_id` and `client_id`. The first instance
becomes orphaned in memory (still running, receives cached events) while the
second overwrites it in the `plugin_map`.

## Root Cause (from source analysis)

Race between background plugin loading and `AddClient` in the plugin thread:

1. `plugin_thread_main` iterates `background_plugins` and calls
   `load_background_plugin` BEFORE entering the event loop
   (`zellij-server/src/plugins/mod.rs:343-351`)

2. `load_plugin` uses `initiating_client_id` (e.g., 1) as a "dummy" client
   because `connected_clients` is still empty at this point
   (`wasm_bridge.rs:292-318`). It dispatches WASM loading asynchronously
   to the pinned executor.

3. The plugin thread enters its event loop. `AddClient(1)` arrives
   (`mod.rs:513`).

4. `add_client` checks `client_is_connected(1)` (`wasm_bridge.rs:723`).
   This returns `false` because the background load used client_id=1 as a
   dummy but never pushed it to `connected_clients` (that only happens at
   the END of `add_client`, line 820).

5. `add_client` iterates `plugin_ids()` from the plugin_map
   (`wasm_bridge.rs:728`). If the executor has already completed the
   background load, `plugin_id=0` is in the map. It creates a SECOND
   WASM instance for `(plugin_id=0, client_id=1)` via `execute_for_plugin`.

6. The HashMap insert at `plugin_map.insert((0, 1), ...)` overwrites the
   first instance. The first WASM instance is orphaned in memory (its
   `load()` was already called, it may have subscribed to events and
   requested permissions).

## Observable Symptoms

- Plugin's `load()` function is called twice
- Plugin's `update(PermissionRequestResult(Granted))` is called twice
- Both instances report the same `plugin_id` and `client_id` from
  `get_plugin_ids()`
- Background plugin processes events from the orphaned instance during
  the brief window before it is overwritten
- If the plugin writes to shared storage (e.g., a log file in the WASI
  cache), both instances write concurrently

## Reproducer Idea (standalone, no cc-deck)

Minimal Zellij plugin that logs each `load()` and `update()` call to
`/cache/load-count.txt` with an incrementing counter:

```rust
use std::sync::atomic::{AtomicU32, Ordering};
use zellij_tile::prelude::*;
use std::collections::BTreeMap;

static INSTANCE_COUNTER: AtomicU32 = AtomicU32::new(0);

#[derive(Default)]
struct DuplicateDetector {
    instance_id: u32,
}

impl ZellijPlugin for DuplicateDetector {
    fn load(&mut self, _config: BTreeMap<String, String>) {
        self.instance_id = INSTANCE_COUNTER.fetch_add(1, Ordering::SeqCst);
        // Write immediately to file (not buffered)
        let msg = format!("LOAD instance={} at {:?}\n", self.instance_id, std::time::SystemTime::now());
        let _ = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open("/cache/duplicate-detect.log")
            .and_then(|mut f| {
                use std::io::Write;
                f.write_all(msg.as_bytes())
            });

        subscribe(&[EventType::PermissionRequestResult]);
        request_permission(&[
            PermissionType::ReadApplicationState,
        ]);
        set_selectable(false);
    }

    fn update(&mut self, event: Event) -> bool {
        if let Event::PermissionRequestResult(status) = &event {
            let ids = get_plugin_ids();
            let msg = format!(
                "PERMISSION instance={} status={:?} plugin_id={} client_id={}\n",
                self.instance_id, status, ids.plugin_id, ids.client_id
            );
            let _ = std::fs::OpenOptions::new()
                .create(true)
                .append(true)
                .open("/cache/duplicate-detect.log")
                .and_then(|mut f| {
                    use std::io::Write;
                    f.write_all(msg.as_bytes())
                });
        }
        false
    }

    fn render(&mut self, _rows: usize, _cols: usize) {}
}

register_plugin!(DuplicateDetector);
```

Config (config.kdl):
```kdl
load_plugins {
    "file:/path/to/duplicate_detector.wasm" {
        mode "background"
    }
}
```

Expected: ONE `LOAD` line in the log.
Actual: TWO `LOAD` lines (instance=0 and instance=1) and TWO `PERMISSION` lines.

## Suggested Fix

In `wasm_bridge.rs:add_client` (line 722), before creating new instances,
check if the plugin was already loaded with this client_id as the
`initiating_client_id`. Options:

**Option A**: Track which client_id was used for background plugin loading.
Skip re-creation in `add_client` if the client_id matches.

**Option B**: Push `initiating_client_id` to `connected_clients` before
loading background plugins (in `plugin_thread_main`), so `add_client`
sees it as already connected and skips.

**Option C**: In `add_client`, check if `(plugin_id, client_id)` already
exists in `plugin_map` before creating a new instance. Currently it
iterates `plugin_ids()` without checking if the specific
`(plugin_id, new_client_id)` pair already exists.

Option C is the simplest and most defensive:
```rust
// In add_client, around line 731:
for plugin_id in new_plugins {
    // Skip if this (plugin_id, client_id) already exists
    if self.plugin_map.lock().unwrap().contains_key(&(plugin_id, client_id)) {
        continue;
    }
    // ... existing clone logic ...
}
```

## Impact

- Plugins that maintain internal state may see inconsistent behavior
  during the overlap window
- Plugins that write to shared files (WASI `/cache/`) may see
  interleaved or duplicate writes
- Plugins that register keybindings via `reconfigure()` may register
  twice
- Permission requests may be sent twice, though the UI handles this
  gracefully

## Notes

- The `AtomicU32` trick in the reproducer works because WASI is
  single-threaded, but the static is shared across all instances loaded
  from the same module (they share the same WASM linear memory? Actually
  no, each instance gets its own Store. Need to verify this.)
- Alternative reproducer approach: write a unique random ID to
  `/cache/` on each `load()` call and count the files.
- The `thread_local!` approach would NOT detect duplicates because each
  WASM instance has its own thread-local storage.
