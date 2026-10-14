# Brainstorm: Codex shared daemon pane attribution

**Date:** 2026-10-14
**Status:** active

## Problem Framing

A plain `codex` session running in a Zellij pane does not appear in the cc-deck sidebar.
Since Codex 0.157 the TUI hands its work to a shared app-server daemon (`codex app-server --managed-daemon`, one per `CODEX_HOME`), and lifecycle hooks run inside that daemon.
The daemon inherits the environment of whichever TUI started it first, so every hook from every later Codex TUI reports that first pane's `ZELLIJ_PANE_ID` and `ZELLIJ_SESSION_NAME`.

Observed on this machine (Codex 0.162.1): the TUI ran in pane 19, the daemon carried `ZELLIJ_PANE_ID=27`, and the hook log showed `UserPromptSubmit`, `PreToolUse`, `PostToolUse` and `Stop` all with `pane_id_arg="27"`.
Pane 27 no longer existed, so the plugin discarded the events.
When the reported pane still exists, events land in the wrong sidebar row instead.

The idea inbox item `pane-map-write-race` is folded in: the fix relies on the `pane-map.json` session cache, which today is read and rewritten on every hook event without locking and with a non-atomic write.

### Research

- Upstream: [openai/codex#48500](https://github.com/openai/codex/issues/48500) (open) describes the same bug for tmux. A Codex maintainer replied that hooks run inside the app server and are intentionally client-agnostic, so restoring per-client environment is not guaranteed. [openai/codex#44902](https://github.com/openai/codex/issues/44902) (open) proposes a hook-scoped `client_env` allowlist, which would be the clean fix.
- Other tools: [zj-radar #71](https://github.com/marktoda/zj-radar/issues/71) (Zellij, same symptom, `--no-daemon` alias as workaround), [armyknife #943](https://github.com/fohte/armyknife/pull/943) (merged: bind the hook to the unique pane running Codex in the same working directory, skip ambiguous or already-bound panes), [muxflow #10](https://github.com/gal064/muxflow/pull/10) (empty `CODEX_EXEC_SERVER_URL` plus `daemon_auto_start = false` to bypass the daemon), [projmux #1385](https://github.com/crevissepartners/projmux/pull/1385) (start the daemon without pane identity).
- Codex's own state links a thread only to the daemon: the rollout file and `logs_2.sqlite` rows carry the daemon pid, not the TUI. The interactive TUI process does carry the correct `ZELLIJ_PANE_ID`, `ZELLIJ_SESSION_NAME` and working directory.

## Approaches Considered

### A: Hook-side TUI discovery with a session cache
For Codex events, the hook does not trust its inherited pane when it runs under the app-server daemon.
It uses a cached `session_id` to pane mapping when one exists, and otherwise discovers the interactive `codex` TUI processes of the same user whose working directory matches the event's `cwd`, excluding TUIs already bound to another live session.
Exactly one candidate yields the pane (and Zellij session) from that process's environment, and the mapping is cached; zero or several candidates drop the event.
- Pros: works for plain `codex` without changing how it is started; also correct for TUIs in other Zellij sessions or outside Zellij that share the daemon; matches the merged armyknife fix; the scan runs once per session.
- Cons: platform-specific process introspection (macOS and Linux); `cwd` heuristic (a TUI started with `-C <dir>` needs its arguments considered); first event of a session pays one scan.

### B: Plugin-side correlation
The hook sends the event with an unverified pane, and the plugin matches it to a pane by title or working directory.
- Pros: no process scanning in the CLI.
- Cons: the Zellij plugin API does not reliably expose cwd or command for shell-started programs; title matching against Codex's terminal title is fragile; adds logic to the WASM plugin.

### C: Bypass the daemon inside cc-deck sessions
Set `CODEX_EXEC_SERVER_URL=` in the Zellij session environment (the muxflow technique for tmux), so every pane's `codex` skips the daemon and hooks run in-process.
- Pros: exact attribution without heuristics; `codex` remains the command.
- Cons: removes the shared daemon inside cc-deck sessions (`codex agents`, cross-client threads); relies on undocumented behavior; does not help a TUI already attached to a daemon.

### D: Upstream fix in Codex
Hooks receive explicitly allowlisted client environment (`client_env`, #44902).
- Pros: the only fully robust fix.
- Cons: outside cc-deck's control; no timeline.

## Decision

Approach A, plus an upstream comment on the Codex issues describing the Zellij use case.
When Codex ships client context for hooks, the hook prefers it and keeps discovery as the fallback.
Approach C stays out of scope.

## Key Requirements

- A plain `codex` started by hand in any pane, with the shared daemon in its default mode, appears in the sidebar row of its own pane.
- Several Codex TUIs in different panes, sharing one daemon, each appear in their own row.
- When an event cannot be attributed to exactly one TUI (for example two TUIs in the same working directory with neither already bound), the event is dropped until a later event disambiguates. A session may appear late, but never in the wrong row.
- An event whose originating TUI is in a different Zellij session, or outside Zellij, is not delivered to the current session's sidebar.
- The hook never attributes a Codex event from the daemon to the daemon's inherited pane on that basis alone.
- Once a session is bound to a pane, later events use the cached mapping without another process scan; a mapping whose TUI has exited is not reused.
- The pane-map cache is written atomically and safe against concurrent hook processes (inbox item `pane-map-write-race`).
- Claude Code and OpenCode behavior is unchanged.
- Hook debug logging records the resolution path (from-env, from-cache, discovered, ambiguous, none) so attribution problems are diagnosable.
- Documentation explains the Codex daemon behavior and the attribution rules.

## Open Questions

- How does the hook reliably detect that it runs under the app-server daemon (parent process, an environment marker, or always preferring discovery for Codex)?
- Which process attributes identify an interactive Codex TUI across install methods (standalone, npm, Homebrew) and platforms?
- How should `codex -C <dir>` and TUIs that change the thread's working directory be matched?
- What is the acceptable cost of the first-event process scan on the hook hot path?
- Should a mapping bound while ambiguous be revisited when a second TUI exits?
- Should profile wrappers (`codex-<profile>`) export a marker that makes discovery exact for wrapper-launched sessions?
