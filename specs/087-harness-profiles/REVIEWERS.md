# Review Guide: Harness Profiles

**Generated**: 2026-10-12 | **Spec**: [spec.md](spec.md)

## Why This Change

Many developers run the same agent harness under more than one account, for example Claude Code through company Vertex access and through a personal subscription. cc-deck cannot run such sessions side by side in one workspace: shell aliases do not travel to remote workspaces, snapshot restore always relaunches the plain `claude` command, and the sidebar gives no hint which account a session uses. Running a session under the wrong account costs money or breaks compliance, and the user only notices later.

## What Changes

Profiles in `config.yaml` become agent-specific: each names a harness, an auth source (`api_key`, a subscription `oauth_token`, or an interactive `login`), a model, extra environment and an optional color and icon. cc-deck renders one wrapper command per profile (`claude-work`, `claude-private`) into a managed bin directory in local, SSH and OpenShell workspaces. Every profiled session shows a one-column edge bar in its profile color in the sidebar, with a legend in the help overlay. Snapshots record agent and profile per session and restore relaunches through the matching wrapper. Existing `config.yaml` files and snapshots load unchanged; the profile schema only gains optional fields.

## How It Works

- **Config** (`internal/config`): `Profile` gains `harness`, `auth`, `env`, `color`, `icon`. `EffectiveAuth()` maps the legacy secret fields into the new auth view, so old profiles validate without edits.
- **Translators** (`internal/profile`): one `Translator` per harness (Claude, Codex, OpenCode) turns a resolved profile into a POSIX `sh` wrapper. The wrapper:
  - exports `CC_DECK_PROFILE`, `CC_DECK_PROFILE_COLOR` and the optional icon;
  - runs one `unset` line for competing auth variables;
  - points the harness at a per-profile config directory that symlinks shared settings and isolates only credential state;
  - execs the real binary.
  
  Credentials appear only as variable references or file paths, never as values.
- **Sync and provisioning**: `cc-deck config profile sync` writes wrappers, config dirs, credential files (0600) and a managed rc block for `PATH`. `Provision` does the same through a `Target` interface for SSH and OpenShell.
- **Hook to plugin**: the hook reads the profile, color and icon from the wrapper environment and falls back to `config.yaml` only for older wrappers. The plugin stores `profile` and `profile_color` per session. The renderer replaces the existing blank margin column with `▌`, so the layout does not shift.
- **Snapshots**: `SessionEntry` stores `agent` and `profile`. Restore builds `<wrapper> <resume args>` from the agent adapter and warns when a profile no longer exists.

## When It Applies

**Applies when**:
- A user defines profiles with a `harness` and starts sessions through the generated wrapper commands
- Local, SSH and OpenShell workspaces (wrappers are delivered on attach or create)
- Snapshot save and restore of sessions started through a wrapper

**Does not apply when**:
- The plain `claude`, `codex` or `opencode` command is used: it keeps the default account, unchanged
- Kubernetes deploy and compose backends: they keep a single workspace-level profile, and a Kubernetes deploy rejects `oauth_token` profiles up front
- Directory-to-profile mapping, profile inheritance, switching a running session's profile, and subscription tokens for Codex or OpenCode (all deferred)
- OpenShell credential masking of renamed variables, tracked separately in [#38](https://github.com/cc-deck/cc-deck/issues/38)

## Key Decisions

1. **Wrapper commands instead of a `cc-deck run --profile` launcher.** Wrappers work in any shell, survive in remote workspaces and are what snapshot restore can call. A launcher subcommand would need cc-deck installed and on `PATH` everywhere.
2. **Per-profile config directory with symlinked shared entries.** Only login and credential state is isolated, so skills, settings, plugins and MCP configuration apply to every profile. A fully separate directory per profile was rejected because customizations would need to be duplicated.
3. **An edge bar instead of glyph coloring or row shading** (revision 2026-10-12). The bar sits in the existing margin column, so it costs no width. It also never carries the active or cursor highlight. Row shading was rejected because the highlight hides it on exactly the row the user looks at. Glyph coloring was rejected because the glyph is hidden when only one harness runs.
4. **`auth.oauth_token` for subscriptions.** A token created with `claude setup-token` travels through the existing credential transport, so remote workspaces need no interactive login. `login: true` stays for users who prefer the browser flow.
5. **Wrappers clear competing auth variables.** Claude Code gives an explicit `ANTHROPIC_API_KEY` or `CLAUDE_CODE_USE_VERTEX` precedence over a token. Without the `unset` line, a variable left in the calling shell would silently switch the account. `GOOGLE_APPLICATION_CREDENTIALS` is deliberately kept, because Vertex profiles that rely on ambient gcloud credentials need it.
6. **Palette with 45-degree hue spacing and `Pick` on `add`.** The earlier palette had two near-identical hues. `add` now writes the farthest free hue into `config.yaml`. `sync` only warns about collisions and never rewrites config.

## Areas Needing Attention

- **Unset list completeness**: the R16 table of auth variables per harness is hand-maintained. A new Claude Code auth variable would not be cleared until the table is updated.
- **Hook trust in the environment**: the hook takes the color and icon from `CC_DECK_PROFILE_*` without checking them against `config.yaml`. A stale wrapper therefore shows a stale color until the next `sync`. This is intentional (zero config reads per event, SC-010).
- **Env-override warning placement**: the warning about `env` keys that override the credential is emitted by `sync`, not by `config.Validate()`, because `config` cannot import the translators. Users who never run `sync` explicitly only see it at local attach, at debug level.
- **Rename and visual tests in the plugin**: the bar must look right in six entry states (plain, active, cursor, rename, faded, paused). The render tests check the escape sequences, but a manual look in a real terminal is still worthwhile.
- **Rebased branch**: the branch was rebased onto `main` on 2026-10-12. The only conflict was in `validate_test.go`, resolved by keeping both test sets.

## Open Questions

- Does macOS Keychain share one Claude login across `CLAUDE_CONFIG_DIR` values? The quickstart manual checks record the outcome. If it does, the guide must tell `login` users to prefer `oauth_token` on macOS.

## Review Checklist

- [ ] Key decisions are justified
- [ ] Breaking changes are documented with migration guidance (none expected: legacy profiles and snapshots load unchanged)
- [ ] Scope matches the stated boundaries
- [ ] Success criteria are achievable
- [ ] No unstated assumptions
- [ ] Generated wrappers contain no credential values (SC-006)
- [ ] Unprofiled sessions render byte-for-byte as before the feature

---

<!-- Code phase sections are appended below this line by the phase-manager command -->
