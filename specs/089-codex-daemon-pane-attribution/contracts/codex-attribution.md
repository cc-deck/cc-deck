# Contract: Codex hook attribution

**Feature**: 089-codex-daemon-pane-attribution

Binding behavior of `cc-deck hook --agent codex` after this feature. Claude Code and OpenCode (`--agent claude`, `--agent opencode`) are not affected except for the cache write rules in section 4.

## 1. Inputs

- stdin: Codex hook payload JSON with `hook_event_name`, `session_id`, `cwd`, optional `tool_name`, optional `client_env` (object of string values).
- `--pane-id`: shell-expanded `$ZELLIJ_PANE_ID` of the hook process (inherited from the daemon when daemon-hosted).
- Environment of the hook process (`ZELLIJ`, `ZELLIJ_SESSION_NAME`, `CODEX_HOME`, `HOME`, `CC_DECK_PROFILE*`).
- Process table of the current user (through `procinfo.Source`).

## 2. Resolution order (first match wins)

1. **Explicit client identity** (`client-env`): `client_env.ZELLIJ_PANE_ID` parses as a pane id. Target session is `client_env.ZELLIJ_SESSION_NAME`. When that is absent, a hook that is not daemon-hosted uses its own `ZELLIJ_SESSION_NAME`, and a daemon-hosted hook skips this step and continues with step 3, because its own session is the inherited one. Delivered.
2. **Not daemon-hosted** (`from-arg` and the existing reasons): no ancestor within 8 levels is the shared daemon, that is a `codex` process whose arguments are `app-server` followed by `--managed-daemon`, a `--listen unix:...` address, or the `daemon` helper subcommand. A stdio `app-server` of an IDE extension is not the shared daemon. The walk uses `procinfo.Source.Ancestor`, which reads only identity and arguments (Linux `status`, `stat`, `cmdline`), so an ancestor with an unreadable environment is still classified. An ancestor of another user ends the walk as not daemon-hosted (an unreadable `status` counts as such only when the `/proc/<pid>` owner is neither the user nor root); any other inspection error drops the event (`no-client`). Today's code path runs unchanged, including the `not-in-zellij` exit and `pane-map.json` recording.
3. **Cached client mapping** (`from-client-cache`): `pane-map.json[session_id]` has `client_pid` and `client_start`, and the process with that pid exists, has that start time, is an interactive Codex client, and its environment names the same `ZELLIJ_PANE_ID` and `ZELLIJ_SESSION_NAME`. Delivered to that pane and session. When the entry names a client that is no longer live, the event is dropped (`no-client`, detail `stale-mapping`) without discovery, unless the event is `SessionStart`. On `SessionStart` discovery runs first; a `discovered` result replaces the mapping. Any other result falls back to a live mapping only when its client is among the matches (same directory and Codex home); otherwise the event is dropped and the mapping removed (detail `session-moved` when discovery gave none).
4. **Discovery**: matching clients are all Codex processes of the current user (executable basename `codex`, except the shared daemon's processes) whose Codex home equals the hook's Codex home and whose directory equals the payload `cwd` (both symlink-resolved). The directory of a client is its `-C`/`--cd` argument, else its real working directory (Linux), else its `PWD`. Only interactive clients (first non-flag argument absent, `resume`, `fork`, or an initial prompt) are bindable; other app-servers, `codex exec` and other subcommands count as matching but are not bindable. A single matching client that is not bindable yields `no-client`.
   - 0 candidates: `no-client`, dropped.
   - 2 or more candidates: `ambiguous`, dropped. This holds whether or not some candidates already own mappings.
   - 1 candidate without `ZELLIJ_PANE_ID`: `client-not-in-zellij`, dropped.
   - 1 candidate inside Zellij: `discovered`. Mapping recorded (section 4), event delivered.
5. Any inspection error during steps 3 or 4 is treated as "no live mapping" or "no candidate" respectively. It never falls back to the inherited `--pane-id`. A client's `ZELLIJ_SESSION_NAME` that is empty, starts with `-`, contains `/` or control characters, or exceeds 256 bytes counts as "outside Zellij"; a profile name that is not a valid cc-deck profile name is ignored.

For daemon-hosted events, the inherited `--pane-id` and the hook's own `ZELLIJ_SESSION_NAME` are never used to choose the target.

## 3. Delivery

- The payload sent on `cc-deck:hook` keeps its existing schema (`agents/NormalizedPayload`). `pane_id` is the resolved pane.
- The pipe is sent with `zellij --session <target session> pipe --name cc-deck:hook -- <payload>`.
- Profile fields (`profile`, `profile_color`, `profile_icon`) come from the client's `CC_DECK_PROFILE*` environment for `client-env`, `from-client-cache` (profile name from the mapping) and `discovered`; from the hook's environment otherwise.
- Badges are evaluated from the payload `cwd`, as today.

## 4. `pane-map.json` rules

- All read-modify-write cycles hold an exclusive `flock` on `pane-map.lock`; a lock not acquired within 2 s skips the write (the event is still delivered).
- Writes are atomic: temp file in the same directory, `fsync`, `rename`.
- Daemon-hosted Codex events write entries only on `discovered` (with `client_pid`, `client_start`, `profile`) and refresh them on `from-client-cache` (hourly, as today). They never write the inherited pane.
- `SessionEnd` removes the session's entry, as today.
- Entries without `client_pid` are ignored by steps 3 and 4.

## 5. Debug log

When `~/.local/state/cc-deck/hook-debug` exists, each Codex invocation appends one line containing `event=`, `session=`, `resolution=`, an optional `detail=` (why a `no-client` drop happened: inspection error, stale mapping, single non-interactive match), `pane=` (resolved or empty), `target_session=` and the existing environment fields. `resolution` takes the values listed in data-model.md.

## 6. Contract tests

1. Daemon-hosted, inherited pane names an existing pane, one matching client in another pane: delivered to the client's pane, inherited pane never appears.
2. Daemon-hosted, two matching clients, one already mapped: dropped as `ambiguous`; the mapped session's later events still use `from-client-cache`.
3. Daemon-hosted, client in another Zellij session: delivered with `--session` set to the client's session.
4. Daemon-hosted, single client outside Zellij: dropped as `client-not-in-zellij`.
5. Cached mapping whose pid now belongs to a process with a different start time: not used. An ordinary event is dropped (`no-client`, detail `stale-mapping`) without discovery; a `SessionStart` event runs discovery and rebinds.
6. Not daemon-hosted: identical output to the current implementation for the existing hook test table.
7. `client_env` present: wins over cache and discovery.
8. 20 concurrent writers: file always parses, all 20 sessions present afterwards.
