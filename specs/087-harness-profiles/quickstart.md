# Quickstart: Harness Profiles validation

**Feature**: 087-harness-profiles

Runnable checks that prove the feature end to end. Contracts are in `contracts/`, field definitions in `data-model.md`.

## Prerequisites

- `make install` succeeded (CLI on `PATH`, plugin installed into Zellij).
- `claude` installed; `codex` optional for scenario 3.
- Two Anthropic credentials available: an API key in `ANTHROPIC_API_KEY_WORK` (or a Vertex ADC file) and a subscription you can log in with.

## 1. Define profiles and sync (US1, US5)

```sh
cc-deck config profile add work --harness claude --model claude-sonnet-5 --api-key-env ANTHROPIC_API_KEY_WORK --color '#4FC1E9'
cc-deck config profile add private --harness claude --model claude-opus-5 --login
cc-deck config profile list
cc-deck config profile sync
```

Expected: `list` shows both with HARNESS `claude` and AUTH `env:ANTHROPIC_API_KEY_WORK` / `login`. `sync` reports `written: claude-work, claude-private`, creates `~/.local/share/cc-deck/bin/`, the rc block in `~/.zshrc` (or `~/.bashrc`), and the two config dirs under `~/.local/share/cc-deck/profiles/`. Open a new shell.

```sh
sh -n ~/.local/share/cc-deck/bin/claude-work
grep -c "$ANTHROPIC_API_KEY_WORK" ~/.local/share/cc-deck/bin/claude-work   # expect 0 (SC-006)
ls -l ~/.local/share/cc-deck/profiles/work/claude/                          # settings.json, commands, projects are symlinks; no .credentials.json
```

## 2. Launch and identify (US1)

In a Zellij tab: `claude-work`, then in Claude run `/status` (or `echo $CC_DECK_PROFILE` from a shell pane launched by the same wrapper). Expected: model is the configured one, `CC_DECK_PROFILE=work`, sidebar shows the session. Then `claude-private`: Claude asks for its login once (subscription), then runs with the frontier model. Plain `claude` still behaves as before.

Negative: `ANTHROPIC_API_KEY_WORK= claude-work` prints `cc-deck profile 'work': credential ANTHROPIC_API_KEY_WORK is not available` and exits 1 without starting Claude (FR-013).

## 3. Sidebar (US2)

With `claude-work` and `claude-private` running: both entries show the `✳` glyph, one in `#4FC1E9`, one in the derived color. Stop one: the glyph disappears. Press `?`: the help overlay lists `Profiles` with both names and colors. Add `icon: "P"` to `private`, run `sync`, start a new `claude-private` session: its glyph is `P`.

Optional: `codex-team` next to `claude-work` shows `◆` and `✳` with profile colors.

## 4. Snapshot and restore (US4)

```sh
cc-deck snapshot save mixed
jq '.sessions[] | {agent, profile}' ~/.local/state/cc-deck/sessions/mixed.json
```

Expected: entries carry `"agent": "claude"` and `"profile": "work"` / `"private"`. Close the tabs, `cc-deck snapshot restore mixed`: each restored pane runs `claude-<profile> --resume <id>` and the sidebar colors match immediately. Delete profile `private`, restore again: that pane runs plain `claude --resume` and the CLI prints the missing-profile warning. Restore a snapshot saved before this feature: unchanged behavior.

## 5. SSH workspace (US3)

```sh
cc-deck ws new --type ssh <host> --name remote
cc-deck ws attach remote
```

Inside the remote shell: `command -v claude-work`, `cat ~/.zshrc | grep -A1 '>>> cc-deck'`, `claude-work` starts with the transported key (`credentials.env` exports `ANTHROPIC_API_KEY_WORK`). `claude-private` prompts for login once on the remote; a second launch does not.

## 6. OpenShell workspace (US3, FR-026)

Create an OpenShell workspace whose manifest lists `agents: [claude]`. Expected in `cc-deck ws status`: providers `cc-deck-<ws>-work` and `cc-deck-<ws>-private`. Inside the sandbox: `claude-work` and `claude-private` on `PATH` under `/sandbox/.local/share/cc-deck/bin`, both start. Add a `vertex` profile afterwards and run `sync --workspace <ws>`: the output includes the re-create hint.

## 7. Backward compatibility (SC-005)

Take a pre-feature `config.yaml` with `backend: anthropic` and `api_key_secret`. `cc-deck config profile list` shows it as HARNESS `claude`, AUTH `secret`; `cc-deck config validate` (or any command) reports no errors; `sync` lists it under skipped with reason `secret source is only available on Kubernetes`. Git credential resolution for workspaces still uses `default_profile`.

## 8. Harness assumptions to verify manually (research R4)

Record the outcome in `contracts/harness-translator.md` section 2.4 before closing the feature:

1. macOS: with two `login: true` profiles, log in to each; confirm `security find-generic-password -s "Claude Code-credentials"` (or the versioned service name) shows two entries, or that Claude Code stores tokens per `CLAUDE_CONFIG_DIR`. If not, document that macOS shares one subscription login across profiles.
2. Codex: `CODEX_HOME=/tmp/x codex --version` then check `/tmp/x` receives `config.toml`/`auth.json` on login; `model` in the generated `config.toml` is honored (`codex` shows it in its banner).
3. OpenCode: `OPENCODE_CONFIG=<generated file> opencode` picks the `model` from the file and loads the cc-deck plugin from the default plugin directory.

## Automated checks

```sh
make test      # Go unit tests (config, profile, shellrc, agent, session) and cargo tests (hooks, render_broadcast, session serde)
make lint
make verify
```
