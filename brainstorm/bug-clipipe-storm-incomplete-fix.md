# Bug: CliPipe storm fix is incomplete, and the Zellij log that reports it is wrong

**Filed**: 2026-09-07
**Severity**: High. One session server sat at 25-30% CPU for two days, and `zellij attach` and
`zellij kill-all-sessions` both hung until the relay was killed by hand.
**Related**: `ad9f570` ("Fix CliPipe timeout storm causing 70-187% CPU on Zellij server")

## Two separate problems

### 1. The storm fix reduced the symptom without removing it

Commit `ad9f570` applied three layered fixes: dormant controllers unblock `DumpState` pipes,
the voice relay poll went from 1s to 3s, and `debug.rs` stopped panicking on re-entrant borrows.

Both fixes were present on the affected machine:

| Artifact | Built | Fix included |
|---|---|---|
| `~/bin/cc-deck` -> main checkout binary | 2026-08-06 15:13 | yes (fix committed 08:07 the same day) |
| `~/.config/zellij/plugins/cc_deck.wasm` | 2026-08-17 10:30 | yes |

The storm still occurred, at roughly 30% instead of the 70-187% in the commit title. The fix
lowered the amplitude; it did not remove the mechanism.

### 2. The Zellij log line that reports it is factually wrong

`zellij-server/src/route.rs` handles `Action::CliPipe` by deliberately dropping the completion
channel:

```rust
Action::CliPipe { .. } => {
    drop(completion_tx); // releasing pipes is handled by the plugins, so we don't want
                         // this to block additionallu
```

The waiter then treats a dropped sender and a real timeout identically:

```rust
match runtime.block_on(async { tokio::time::timeout(ACTION_COMPLETION_TIMEOUT, receiver).await }) {
    Ok(Ok(result)) => result,
    Err(_) | Ok(Err(_)) => {
        log::error!("Action {} did not complete within {:?} timeout", action_name, ACTION_COMPLETION_TIMEOUT);
```

`Ok(Err(_))` resolves immediately. So **every** `zellij pipe` logs a one-second timeout that never
happened. The log is a red herring that costs real debugging time; it sent this investigation down
the wrong path before the timestamps gave it away.

**The tell**: three consecutive "1s timeout" entries at `14:45:50.719`, `.740`, `.770` — 51ms
apart. A one-second timeout cannot fire three times in 51 milliseconds.

This is worth reporting upstream to Zellij: the two cases need distinct messages, or the
sender-dropped case should not log at error level at all.

## Evidence

Controlled, reversible experiment. `kill -STOP` the voice relay, measure, `kill -CONT`:

| Condition | Server CPU per 6s | Effective | New CliPipe lines per 4s |
|---|---|---|---|
| Relay running | 1.80s | ~30% | ~5 |
| Relay paused | 0.44s | ~7% | **0** |
| Relay resumed | 1.25s | ~21% | 4 |

Supporting facts:

- Voice relay `cc-deck ws voice local --model medium` started `Sat Sep 5 07:54:13`; the first
  CliPipe entry in the rotated log is `Sep 5 11:03:59`.
- Accumulated server CPU when found: **119 minutes** over a 6.5 hour session.
- The two other Zellij sessions on the same machine, with no relay attached, sat at 0.0-0.1% CPU.
- After killing the relay, the affected server dropped to 0.03s per 5s (~0.6%).

## Ruled out

- **Plugin permissions.** All seven permissions the plugin requests
  (`ReadApplicationState`, `ChangeApplicationState`, `RunCommands`, `ReadCliPipes`,
  `MessageAndLaunchOtherPlugins`, `Reconfigure`, `WriteToStdin`) are granted in
  `permissions.kdl` under the exact path the layout loads from.
- **Stale plugin copy.** No `~/.local/share/zellij/plugins/cc_deck.wasm` present.
- **The 087 sharing work.** The affected session ran the main-branch binary and plugin. The storm
  predates that branch by two days, and the branch's demo session never appears in either log.

## Secondary failure: hangs cascade from the pipes, not the server

When the user quit the session with Ctrl-Q, `zellij attach` and `zellij kill-all-sessions` both
hung indefinitely. The cause was leftover `zellij pipe` children still blocked against the dying
server, with the relay spawning more. Every subsequent Zellij command queued behind them.

Recovery order matters, and is not obvious:

1. kill the voice relay
2. kill leftover `zellij pipe` processes
3. kill the stuck `zellij kill-all-sessions`
4. only then SIGTERM/SIGKILL the wedged session server

Killing the server first leaves the pipes in place and the next command hangs the same way.

## Suggested work

1. **Stop polling.** A 3-second poll that fans out to every plugin instance is the mechanism. An
   event-driven state push, or a single broker (the pipe mux broker in `5919b11` looks relevant)
   would remove it rather than slow it down.
2. **Bound the fan-out.** Investigate why four plugin instances receive each broadcast; this may be
   the same duplication as the known dual-controller bug.
3. **Make the relay's cost visible.** Two days at 30% CPU passed unnoticed. The relay should be
   able to report its own pipe rate.
4. **Report the log bug upstream** to Zellij, with the 51ms-apart timestamps as evidence.
5. **Add a guard.** Nothing in the test suite would have caught a background poller quietly
   burning a third of a core.
