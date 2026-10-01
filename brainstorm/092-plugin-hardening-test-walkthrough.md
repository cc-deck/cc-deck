# Plugin hardening: manual test walkthrough

Branch: `refactor/plugin-hardening` (5 commits on top of `main`)
Date: 2026-09-07

Run everything from a plain terminal (Terminal.app or iTerm2), not from inside a
Zellij pane. `make install` replaces the plugin under any running session.

Automated checks already pass on the branch: `make verify` (Go tests, 423 Rust
tests including new property tests, `go vet`, clippy with `-D warnings`).
This walkthrough covers what only a live Zellij can verify.

Shortcut used below for the plugin cache directory:

```bash
L=~/Library/Caches/org.Zellij-Contributors.Zellij/file:/Users/$USER/.config/zellij/plugins/cc_deck.wasm/plugin_cache
```

## 0. Build and install the branch

```bash
cd ~/Development/ai/cc-deck/cc-deck
git checkout refactor/plugin-hardening
zellij kill-all-sessions          # or detach and kill only the cc-deck sessions
make install
cc-deck config plugin status      # expect: Compatibility: compatible (Zellij 0.45.1)
```

Confirm the reinstall removed the stale cc-deck entry from PreToolUse. The
`PreToolUse` key itself may stay, because other tools (rtk, the Excalidraw
starter) hook it and cc-deck leaves those alone:

```bash
jq '.hooks | to_entries[] | select(.value[].hooks[].command | test("cc-deck hook")) | .key' ~/.claude/settings.json
```

Expected: SessionStart, PostToolUse, PostToolUseFailure, UserPromptSubmit,
PermissionRequest, Notification, Stop, SubagentStop, SubagentStart, SessionEnd.
`PreToolUse` must not be listed.

## 1. Single controller, no election, fast start

Enable the plugin debug log, truncate it, then start a workspace:

```bash
touch "$L/debug_enabled"; : > "$L/debug.log"
cc-deck ws attach <name>          # or: cc-deck ws new <name> && cc-deck ws attach <name>
```

Detach (Ctrl+o d) and inspect:

```bash
rg -c "CTRL LOAD start" "$L/debug.log"          # expect: 1
rg "ELECTION|dormant" "$L/debug.log"             # expect: no output
rg "CTRL PERMISSION granted|SIDEBAR INIT" "$L/debug.log"
```

Expected in the terminal: the sidebar shows sessions immediately (no two second
blank period), and Alt+s, Alt+a, Alt+w work right away.

## 2. Permission preflight after a wiped cache

```bash
zellij kill-all-sessions
rm ~/Library/Caches/org.Zellij-Contributors.Zellij/permissions.kdl
: > "$L/debug.log"
cc-deck ws attach <name>
```

Expected:

- stderr prints `Restored cc-deck plugin permissions in Zellij's permissions.kdl`
  before Zellij starts
- no permission dialog in the sidebar
- `CTRL PERMISSION granted` appears in the log

Detach and attach a second time. Nothing must be printed this time, because the
entry is already correct (the preflight is idempotent).

## 3. Waiting shows without waiting for the timer

Start Claude Code in a pane and trigger a permission prompt (any tool call that
prompts). The ⚠ indicator should appear at once, not up to a second later.
Answer the prompt: the indicator should clear at once.

Confirm in the log that the hook was applied and not absorbed:

```bash
rg "PermissionRequest|left Waiting|STUCK|absorbed" "$L/debug.log"
```

`STUCK` lines mean a transition out of Waiting was rejected; there should be none
after the prompt was answered.

## 4. Hook volume dropped

With Claude Code running, truncate the log, run three or four tool calls, then:

```bash
rg -c "PIPE name=cc-deck:hook" "$L/debug.log"
rg -o 'hook_event_name":"[A-Za-z]+' "$L/debug.log" | sort | uniq -c
```

Expected: no `PreToolUse` lines, and roughly one hook per tool call (PostToolUse)
instead of two.

## 5. Voice relay costs one process per tick

In a second terminal:

```bash
cc-deck ws voice <name>
```

Then measure for one minute. Either count process spawns with dtrace.
With System Integrity Protection on, dtrace cannot launch a child via `-c`,
so let the script time itself out with a `tick` probe instead:

```bash
sudo dtrace -qn 'proc:::exec-success /execname=="zellij"/ { @[curpsinfo->pr_psargs] = count(); } tick-60s { exit(0); }'
```

The "system integrity protection is on" warning is harmless. Zellij is not a
protected binary, so the `proc` probes still fire for it. If you prefer a
ready-made tool, `sudo execsnoop -a` prints every exec live; stop it with
Ctrl+C after a minute and count the `zellij` lines.

Or count the polls in the plugin log:

```bash
: > "$L/debug.log"; sleep 60
rg -c "PIPE name=cc-deck:dump-state" "$L/debug.log"     # expect: about 12
rg "voice:on:" "$L/debug.log"                            # expect: no per-tick lines
```

Expected: about 12 polls per minute (one every five seconds), all
`cc-deck:dump-state`, and the voice indicator in the sidebar stays on.
Toggle mute with Alt+m and confirm the relay follows within one poll.
Stop the relay (Ctrl+C) and confirm the indicator clears within 15 seconds.

## 6. Two Zellij sessions on the host

```bash
cc-deck ws new second && cc-deck ws attach second
```

Run Claude Code in both sessions. Expected:

- each sidebar shows only its own agents
- attend (Alt+a) in one session does not move focus in the other
- the cache directory holds one `sessions-<pid>.json` per running Zellij server
  and no `attend-state.json` any more:

```bash
ls "$L"
```

## 7. Cleanup

```bash
rm "$L/debug_enabled"
cc-deck ws delete second          # if you created it only for the test
```

## If something fails

Save the relevant slice of `$L/debug.log` together with the step number. The
useful markers are `CTRL LOAD`, `CTRL PERMISSION`, `SIDEBAR INIT`, `CTRL HOOK`,
and `PIPE name=`.

Once every step passes, the branch is ready for a PR against `main`.
