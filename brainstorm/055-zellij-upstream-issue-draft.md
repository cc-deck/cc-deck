# Draft: Zellij Issue - `load_plugins` creates duplicate WASM instance

**Target repo:** zellij-org/zellij
**URL:** https://github.com/zellij-org/zellij/issues/new
**Related:** #4982 (background plugin permissions, same plugin architecture)

---

## Title

`load_plugins` background plugin gets instantiated twice on startup

## Body

Following up from #4982 where I switched to a single-WASM-binary architecture (controller via `load_plugins`, sidebars via layout template). This uncovered a different startup issue: the background plugin gets two WASM instances.

### What happens

When a background plugin is configured in `load_plugins`, two independent WASM instances are created on Zellij startup. Both receive `load()` and `PermissionRequestResult(Granted)`, both process pipe messages, and both subscribe to events. The second instance silently overwrites the first in the plugin map, but the first keeps running as an orphan.

### Reproducer

I built a minimal standalone reproducer: [rhuss/zellij-duplicate-instance-repro](https://github.com/rhuss/zellij-duplicate-instance-repro)

It counts `load()` calls by writing uniquely-named marker files to `/cache/`. Two markers means the bug triggered.

```bash
git clone https://github.com/rhuss/zellij-duplicate-instance-repro
cd zellij-duplicate-instance-repro
make run       # starts Zellij, exit with Ctrl-q
make check     # shows marker count
```

The bug is timing-dependent (~10% trigger rate in my testing). A stress test runs multiple iterations:

```bash
./stress.sh 10   # must run from outside Zellij
```

Example output:

```
  PASS  run 1/10
  PASS  run 2/10
  PASS  run 3/10
  FAIL  run 4/10  (2 instances loaded)
  PASS  run 5/10
  ...
BUG CONFIRMED: 1/10 runs created duplicate instances
```

### Config

```kdl
load_plugins {
    "file:/path/to/my_plugin.wasm"
}
```

**Expected:** One `load()` call, one WASM instance.
**Actual:** Sometimes two `load()` calls, two WASM instances with the same `plugin_id`.

### Where I think this happens

I traced this through the source (0.44.1, commit 120b649c):

1. `plugin_thread_main` calls `load_background_plugin` for each entry in `background_plugins` **before** entering the event loop (`zellij-server/src/plugins/mod.rs`, around line 345).

2. `load_plugin` uses `initiating_client_id` (e.g. 1) as a dummy client because `connected_clients` is empty at this point (`wasm_bridge.rs`, around line 295). It dispatches the WASM load to the pinned executor.

3. The event loop starts. `AddClient(1)` arrives from the server (`mod.rs`, around line 513).

4. `add_client` checks `client_is_connected(1)` (`wasm_bridge.rs`, around line 723). This returns `false` because the dummy client_id was never pushed to `connected_clients` (that happens at the end of `add_client` itself, around line 820).

5. `add_client` iterates `plugin_ids()` from `plugin_map`. If the executor has already completed the background load, `plugin_id=0` is in the map. It creates a **second** WASM instance for `(plugin_id=0, client_id=1)`.

6. The `plugin_map.insert((0, 1), ...)` overwrites the first instance. The first WASM store stays in memory, still receiving cached events.

### Possible fix

The simplest guard would be checking in `add_client` whether `(plugin_id, client_id)` already exists before creating a new instance:

```rust
// In add_client, around line 731:
for plugin_id in new_plugins {
    if self.plugin_map.lock().unwrap().contains_key(&(plugin_id, client_id)) {
        continue;
    }
    // ... existing clone logic ...
}
```

This prevents the duplicate without changing the loading order or client tracking.

### Environment

- Zellij 0.44.1 (macOS, also tested on Fedora 41)
- zellij-tile 0.44

### Impact for plugin authors

Any background plugin that maintains internal state sees inconsistent behavior during the overlap window. Plugins that register keybindings via `reconfigure()` register them twice. Plugins that write to `/cache/` see concurrent writes from both instances. My workaround is a leader election protocol in the plugin itself (lowest `plugin_id` wins, loser goes dormant), but that adds significant complexity for something that should be prevented at the host level.
