# Quickstart: Harness Profiles validation

**Feature**: 087-harness-profiles

Runnable checks that prove the feature end to end. Contracts are in `contracts/`, field definitions in `data-model.md`.

## Prerequisites

- `make install` succeeded (CLI on `PATH`, plugin installed into Zellij).
- `claude` installed; `codex` optional for scenario 3.
- Two Anthropic credentials available: an API key in `ANTHROPIC_API_KEY_WORK` (or a Vertex ADC file) and a subscription. Create a subscription token with `claude setup-token` and export it as `CLAUDE_TOKEN_PRIVATE`. Optionally keep a second subscription for the `login: true` scenario.

## 1. Define profiles and sync (US1, US5)

```sh
cc-deck config profile add work --harness claude --model claude-sonnet-5 --api-key-env ANTHROPIC_API_KEY_WORK --color '#5AC0E2'
cc-deck config profile add private --harness claude --model claude-opus-5 --oauth-token-env CLAUDE_TOKEN_PRIVATE
cc-deck config profile add home --harness claude --login
cc-deck config profile list
cc-deck config profile sync
```

Expected: `list` shows all three with HARNESS `claude` and AUTH `env:ANTHROPIC_API_KEY_WORK` / `token:env:CLAUDE_TOKEN_PRIVATE` / `login`. `config.yaml` now has a `color` for `private` and `home`, chosen by `add` as the palette hues farthest from the colors in use (FR-017); `work` keeps its declared color. `sync` reports `written: claude-work, claude-private, claude-home`, creates `~/.local/share/cc-deck/bin/`, the rc block in `~/.zshrc` (or `~/.bashrc`), and the three config dirs under `~/.local/share/cc-deck/profiles/`. Open a new shell.

```sh
sh -n ~/.local/share/cc-deck/bin/claude-work
grep -c "$ANTHROPIC_API_KEY_WORK" ~/.local/share/cc-deck/bin/claude-work   # expect 0 (SC-006)
grep -c "$CLAUDE_TOKEN_PRIVATE" ~/.local/share/cc-deck/bin/claude-private  # expect 0 (SC-006)
sed -n '3,6p' ~/.local/share/cc-deck/bin/claude-private                     # CC_DECK_PROFILE, CC_DECK_PROFILE_COLOR, unset line (contracts/wrapper-script.md)
ls -l ~/.local/share/cc-deck/profiles/work/claude/                          # settings.json, commands, projects are symlinks; no .credentials.json
```

## 2. Launch and identify (US1)

In a Zellij tab: `claude-work`, then in Claude run `/status` (or `echo $CC_DECK_PROFILE` from a shell pane launched by the same wrapper). Expected: model is the configured one, `CC_DECK_PROFILE=work`, sidebar shows the session. Then `claude-private`: Claude starts authenticated with the subscription token, without a login prompt, and runs with the frontier model. Then `claude-home`: Claude asks for its login once. Plain `claude` still behaves as before.

Competing credentials (FR-007a): `ANTHROPIC_API_KEY=bogus claude-private` still starts on the subscription (`/status` shows the subscription account, not an API key).

Negative: `ANTHROPIC_API_KEY_WORK= claude-work` prints `cc-deck profile 'work': credential ANTHROPIC_API_KEY_WORK is not available` and exits 1 without starting Claude (FR-013).

## 3. Sidebar (US2)

Start only `claude-work`: its entry shows a `▌` bar in `#5AC0E2` at the left edge of both text lines, although no other profile runs (FR-016). No `✳` glyph appears, because only one harness runs. Start `claude-private` and a plain `claude`: the private entry has a bar in its own color, and the plain entry has the usual blank margin, so names stay aligned. Move the cursor onto the work entry, make it active, pause it, and wait for it to fade to idle: the bar color never changes and never takes the highlight background (FR-016a, SC-008). Stop every profiled session: the sidebar looks exactly as before the feature. Press `?` with profiled sessions running: the help overlay lists `Profiles` with a bar swatch and name per profile.

Hook path (FR-014a, SC-010): with `claude-work` running, `chmod 000 ~/.config/cc-deck/config.yaml` and trigger a few tool calls; the bar keeps its color, which proves the hook did not need the config. Restore the mode afterwards.

Color warning (FR-017b): set `color: "#5AC8E8"` on `private` by hand and run `sync`; it warns that `work` and `private` are too similar.

Optional: `codex-team` next to `claude-work` shows `◆` and `✳` glyphs in their brand colors, with the bars carrying the profile colors.

## 4. Snapshot and restore (US4)

```sh
cc-deck snapshot save mixed
jq '.sessions[] | {agent, profile}' ~/.local/state/cc-deck/sessions/mixed.json
```

Expected: entries carry `"agent": "claude"` and `"profile": "work"` / `"private"`. Close the tabs, `cc-deck snapshot restore mixed`: each restored pane runs `claude-<profile> --resume <id>` and the sidebar bars match immediately. Delete profile `private`, restore again: that pane runs plain `claude --resume` and the CLI prints the missing-profile warning. Restore a snapshot saved before this feature: unchanged behavior.

## 5. SSH workspace (US3)

```sh
cc-deck ws new --type ssh <host> --name remote
cc-deck ws attach remote
```

Inside the remote shell: `command -v claude-work`, `cat ~/.zshrc | grep -A1 '>>> cc-deck'`, `claude-work` starts with the transported key (`credentials.env` exports `ANTHROPIC_API_KEY_WORK`). `claude-private` starts without any login prompt, because the token travels through the credential transport (SC-009). `claude-home` prompts for login once on the remote; a second launch does not.

## 6. OpenShell workspace (US3, FR-026)

Create an OpenShell workspace whose manifest lists `agents: [claude]`. Expected in `cc-deck ws status`: providers `cc-deck-<ws>-work` and `cc-deck-<ws>-private`. Inside the sandbox: `claude-work` and `claude-private` on `PATH` under `/sandbox/.local/share/cc-deck/bin`, both start. Add a `vertex` profile afterwards and run `sync --workspace <ws>`: the output includes the re-create hint. A subscription token profile is subject to the OpenShell credential masking limitation tracked in issue #38 (research R15) and is not expected to start in the sandbox until that fix lands.

Kubernetes guard (FR-025): with `private` set as the default profile (`cc-deck config profile use private`), `cc-deck ws new --type k8s-deploy …` fails before creating resources with the subscription-token message.

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
