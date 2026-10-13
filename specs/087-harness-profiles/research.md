# Research: Harness Profiles

**Feature**: 087-harness-profiles | **Date**: 2026-09-09

Findings come from four parallel codebase surveys (config and CLI, agent adapters and hooks, workspace provisioning and credential transport, plugin rendering and snapshots). Each decision lists the alternatives considered.

## R1. Profile schema extension and the `auth` block

**Decision**: Extend `config.Profile` in place. Add `Harness`, `Auth *AuthConfig`, `Env map[string]string`, `Color`, `Icon`. `AuthConfig` holds `APIKey *CredentialSource`, `Credentials *CredentialSource` (Vertex ADC file) and `Login bool`. `CredentialSource` is a struct with three optional string fields `Env`, `File`, `Secret`; validation enforces exactly one. Legacy fields (`api_key_secret`, `credentials_secret`, `project`, `region`, `backend`) stay and are mapped by an `EffectiveAuth()` accessor: `api_key_secret: X` is treated as `auth.api_key.secret: X`, `credentials_secret: Y` as `auth.credentials.secret: Y`. `Backend` keeps its role as the auth backend; the allowed set becomes per harness (claude: `anthropic`, `vertex`; codex: `openai`; opencode: `openai`, `anthropic`) with a per-harness default when absent.

**Rationale**: The codebase has no custom `UnmarshalYAML` anywhere and evolves schemas additively with `omitempty` (config research, item 5). Three optional fields plus validation is the smallest change that gives the "exactly one source" semantics, keeps `yaml.v3` round-tripping intact for `Save()`, and produces validation errors through the existing `Finding` mechanism (spec 065). `EffectiveAuth()` avoids touching any legacy consumer: the only readers of legacy fields are `cmd/profile.go` (display), `validate.go` and `ws/repos.go` (git credentials only).

**Alternatives considered**: a custom `UnmarshalYAML` accepting a scalar or a map (rejected: first of its kind in the codebase, harder to `Save()`); a separate `HarnessProfile` type (rejected by the brainstorm: two profile concepts).

**Backward-compat note**: The Kubernetes deploy path never reads `APIKeySecret` directly; it only calls `loadActiveGitCredentials()` (`ws/repos.go:214`). Existing profiles therefore keep working as long as `Validate()` still accepts `backend: anthropic` with `api_key_secret`, which the new rule "at least one of legacy secret, `auth.api_key`, or `auth.login`" preserves.

## R2. Where profiles are rendered: a new `internal/profile` package

**Decision**: New package `cc-deck/internal/profile` with a `Translator` interface (one implementation per harness: `claude.go`, `codex.go`, `opencode.go`), a `Registry`, a wrapper renderer using `text/template` with an embedded `wrapper.sh.tmpl`, a `Sync` function (local bin dir, per-profile config dirs, stale cleanup, rc block), and a `color.go` palette helper. The `agent.Agent` interface gains three small methods the translators need: `Binary() string`, `InstallHooksAt(configDir string) error`, and `ResumeArgs(sessionID string) []string`.

**Rationale**: The adapters resolve their config paths through package-level function variables (`claudeSettingsPathFunc`, `codexHooksPathFunc`, `opencodeConfigDirFunc`), which is a test seam and not safe to repoint at runtime. `InstallHooksAt` gives a real parameter. Keeping the translator separate from the adapter keeps `internal/agent` focused on hook translation and lets the translator own the shared-versus-isolated directory list (FR-009a). Embedded `text/template` matches the existing Containerfile snippet generator (`internal/build/containerfile.go`).

**Alternatives considered**: putting rendering into each adapter (rejected: bloats the adapter and mixes concerns); a single generic template driven by a data table (rejected: Codex and OpenCode need config files written, not just env vars).

## R3. Wrapper script content and credential references

**Decision**: POSIX `sh` script, mode 0755, name `<binary>-<profile>`, in `$XDG_DATA_HOME/cc-deck/bin/` (`~/.local/share/cc-deck/bin`). It exports `CC_DECK_PROFILE`, the harness config-dir variable pointing at `$XDG_DATA_HOME/cc-deck/profiles/<profile>/<harness>/`, the model variable, the profile `env` map, then resolves credentials and `exec`s the real binary with `"$@"`. Credential references only:

- `{env: NAME}`: the wrapper checks `${NAME:?...}` and exports the harness variable from it. Locally the user's shell provides `NAME`; remotely the credential transport exports `NAME` in `credentials.env` (SSH) or the sandbox rc files (OpenShell). The transport already carries arbitrary named env vars (`ResolvedCredentials.EnvVars` is a `map[string]string` consumed by every `Inject*` function).
- `{file: PATH}`: the wrapper reads `$XDG_CONFIG_HOME/cc-deck/profiles/<profile>/<field>` (mode 0600). Locally `profile sync` copies the host file there; remotely the transport uploads the content to the same relative path. The wrapper is therefore byte-identical on every backend, which is what makes snapshot restore and SC-003 trivial.
- `{secret: NAME}`: only meaningful for the Kubernetes deploy path. On local, SSH and OpenShell, `sync` skips the profile with a warning naming the source.
- `login: true`: no credential lines; the isolated config dir holds whatever the harness stores after its own login.

Missing credentials fail inside the wrapper with `cc-deck profile '<name>': credential <ref> is not available` and exit 1 (FR-013).

**Rationale**: Scripts never contain values (FR-008, SC-006). Using the same relative path on every backend removes per-backend branching from the template.

**Alternatives considered**: `cc-deck run --profile` as the wrapper body (rejected by the brainstorm); embedding values with 0700 permissions (rejected: violates FR-008).

## R4. Per-profile harness config directories (FR-009a)

**Decision**: Symlink sharing. `PrepareConfigDir` creates `$XDG_DATA_HOME/cc-deck/profiles/<profile>/<harness>/` and, for every top-level entry of the harness's default config dir that is not on the translator's isolated list, creates a relative symlink to the default entry. Isolated entries are created fresh. Re-running sync refreshes the links (adds new shared entries, leaves isolated ones alone).

Per harness:

| Harness | Config dir variable | Isolated (per profile) | Shared (symlinked) |
|---------|--------------------|------------------------|--------------------|
| claude | `CLAUDE_CONFIG_DIR` | `.credentials.json`, `statsig/` | `settings.json`, `settings.local.json`, `commands/`, `skills/`, `agents/`, `plugins/`, `CLAUDE.md`, `projects/`, `todos/`, `history.jsonl`, everything else |
| codex | `CODEX_HOME` | `auth.json`, `config.toml` (generated per profile with `model`), `sessions/` | `hooks.json`, `AGENTS.md`, `instructions.md`, `skills/`, everything else |
| opencode | `OPENCODE_CONFIG` (points at a generated per-profile `opencode.json`) | the generated config file | the default config dir is left untouched; the generated file extends the default config with `model` and the cc-deck plugin path |

Because `settings.json` is shared for Claude, the hooks installed in the default directory already apply; `InstallHooksAt` verifies they are present and installs them if not. For Codex, `hooks.json` is shared for the same reason. For OpenCode, the generated config registers the plugin path from the default plugin directory.

**Rationale**: Sharing `projects/` for Claude keeps conversation transcripts in one place, which is required for `--resume` to work when a session is restored under a different profile than the one the transcript was written with. Symlinks are visible to the harness without a re-sync (FR-009a) and cost nothing on disk.

**Verification required (quickstart)**: (a) Claude Code on macOS keys its Keychain entry by config dir in current versions; if it does not, two `login: true` profiles on macOS share one login and the guide must say so. (b) Codex reads `CODEX_HOME` for both `config.toml` and `auth.json`. (c) OpenCode honors `OPENCODE_CONFIG` as a file path and merges it with project config. Each is a task with a manual check; the translator contract records the outcome.

**Alternatives considered**: copying the default dir once (rejected: drift, secrets duplicated); a fully fresh dir (rejected: drops every user customization).

## R5. Getting the bin directory on `PATH` (spec deviation)

**Finding**: The spec assumed "the shell rc snippet cc-deck already manages". That snippet exists only as a Containerfile `RUN sed` for container and OpenShell images (`internal/build/templates/containerfile/05-shell-finalize.tmpl`). Local and SSH workspaces have no managed rc block today.

**Decision**: Add a small `internal/shellrc` helper that writes an idempotent managed block delimited by `# >>> cc-deck >>>` and `# <<< cc-deck <<<` into `~/.bashrc` and `~/.zshrc` (creating the file if absent), containing `export PATH="$HOME/.local/share/cc-deck/bin:$PATH"`. Used by: `profile sync` (local), SSH `Attach` after wrapper upload (executed remotely through the SSH client), OpenShell `Create` after credential injection (appended to `/sandbox/.bashrc` and `.zshrc` the same way `InjectOpenShell` appends exports). The Containerfile template additionally prepends the bin dir at build time so images built after this feature do not need the runtime append. `sync` prints "open a new shell or run `source ~/.zshrc`" when it created or changed the block.

**Spec impact**: FR-006 wording "the shell rc snippet it already manages" becomes "a managed shell rc block (new for local and SSH, baked into images for container and OpenShell)". Recorded here; the spec assumption paragraph is updated in the plan commit.

## R6. Delivering wrappers to SSH and OpenShell

**Decision**: A shared `profile.Provision(target)` step runs after credential injection in `SSHWorkspace.Attach` (`ws/ssh.go:190-203`) and in `OpenShellWorkspace.Create` after `InjectOpenShell` (`ws/openshell.go:410-415`). It renders wrappers for every valid profile whose harness the workspace lists, uploads them (SSH: `client.Upload`, OpenShell: `openShellDataChannel.PushBytes`), creates the per-profile config dirs remotely (a small generated `prepare.sh` executed once, since symlinks must be created on the remote side), uploads profile credential files, and writes the rc block. Harness availability on the remote is checked with `command -v <binary>`; profiles whose harness is missing are skipped with a message (edge case in spec).

**Rationale**: Both backends already have a natural point after credentials are injected and before the session is attached. `DataChannel.PushBytes` exists for OpenShell; SSH uses the same scp path as credential files.

**Alternatives considered**: baking wrappers into the image at build time (rejected: profiles change more often than images and contain per-user names); a `cc-deck` call inside the workspace to self-generate (rejected: needs config.yaml in the workspace, which leaks host paths).

## R7. Hook payload, sidebar, and color derivation

**Decision**: `cc-deck hook` reads `CC_DECK_PROFILE`; if set, loads the profile from config and adds `profile`, `profile_color` (`#RRGGBB`, declared or derived) and, when the profile declares `icon`, replaces `agent_indicator` with the icon. Color derivation lives in Go (`profile/color.go`): FNV-1a of the profile name modulo an 8-entry palette chosen for at least 3:1 contrast against `ACTIVE_BG (25,45,55)` and a dark terminal background: `(255,170,50)` is reserved for unprofiled Claude, so the palette is `#4FC1E9 #A0D468 #ED5565 #AC92EC #FFCE54 #48CFAD #EC87C0 #F6BB42`. The plugin stores `profile: Option<String>` and `profile_color: Option<(u8,u8,u8)>` on `Session` (both `#[serde(default)]`, so they persist in `sessions-<pid>.json` and flow through `dump-state` unchanged). `render_broadcast.rs` computes `show_agent_indicators` from the set of distinct `(agent_name, profile)` pairs. `RenderSession` gains `agent_color: Option<(u8,u8,u8)>`; `render.rs` uses it when present, else `agent_indicator_color()`. `RenderPayload` gains `profile_legend: Vec<LegendEntry{indicator, color, name}>` and the help overlay appends a "Profiles" section when it is non-empty.

**Rationale**: Deriving color in Go keeps the palette in one place (the CLI also prints it in `profile show`) and the plugin stays a renderer. Replacing `agent_indicator` with the icon reuses the existing prefix-width logic, which already handles width-2 glyphs via `unicode_width`.

**Alternatives considered**: deriving color in the plugin (rejected: two palettes to keep in sync); a separate profile column (rejected by the brainstorm: screen real estate).

## R8. Snapshot and restore

**Decision**: Go `pluginSession` and `SessionEntry` gain `Agent string` and `Profile string` (both `omitempty`, so version stays 1 and old files load). `restore.go` replaces the hardcoded `claude --resume` with: binary from `agent.Get(entry.Agent)` (default `claude` when empty), wrapper name `<binary>-<profile>` when `Profile` is set and exists in config (otherwise plain binary plus a warning listing the missing profile), and `ResumeArgs(sessionID)` from the adapter (`claude`: `--resume <id>`; `codex`: `resume <id>`; `opencode`: `--session <id>`). `PendingOverride` (restore-meta pipe) carries `profile` and `profile_color` so the sidebar colors the entry before the first hook arrives.

**Rationale**: The plugin already exports the full `Session` on `dump-state`; only the Go side needs new fields. Today restore launches `claude` for every session regardless of harness; routing through the adapter fixes that latent bug for free.

## R9. OpenShell provider list (FR-026)

**Decision**: In `OpenShellWorkspace.Create`, after the existing single-spec provider resolution, iterate all valid profiles whose harness is in the workspace's agent list, resolve each profile's credentials (via the source variants) into a `ResolvedCredentials`, map the backend to a provider with the existing `mapToOpenShellProvider()` (`anthropic` and `login` map to `claude`, `vertex` to `google-cloud`, `openai` to `openai`), name it `cc-deck-<ws>-<profile>`, and `Ensure` it. The `allProviders` merge already accepts a list. `sync` compares the workspace's recorded provider set with the current profile set and prints the re-create hint from FR-026 when a backend is missing.

**Rationale**: Network endpoints are resolved gateway-side per provider type; adding one provider per profile is the only lever cc-deck has and matches spec 085's delegation model.

## R10. CLI surface (spec deviation)

**Finding**: Profile commands live under `cc-deck config profile`, not `cc-deck profile`. The spec text uses the short form.

**Decision**: Keep the existing location. Add `delete <name>` and `sync [--workspace <name>]` under `cc-deck config profile`. `add` gains flags (`--harness`, `--backend`, `--api-key-env`, `--api-key-file`, `--login`, `--model`, `--env K=V`, `--color`, `--icon`) with the interactive prompt as fallback. `list` adds HARNESS and AUTH columns; `show` prints every field with source references and masks nothing because it never holds values. The spec is updated to the `cc-deck config profile` form in the plan commit.

## R11. Testing approach

- Go: stdlib `testing` with table-driven sub-tests in `internal/config` and `internal/profile` (matching `validate_test.go`); wrapper rendering is snapshot-tested against golden strings; `Sync` is tested against a temporary `HOME` and `XDG_*` dirs; `shellrc` block idempotency is unit-tested; restore command construction is unit-tested per harness.
- Rust: `cargo test` via `make test`; new tests for the `(agent, profile)` show rule in `render_broadcast`, `Session` serde round-trip with the new fields, and `HookPayload` parsing with and without the new fields (following `test_helpers.rs`).
- Manual verification in `quickstart.md` for the three harness-specific assumptions in R4 and for SSH and OpenShell delivery.

---

## Revision 2026-10-12

Decisions R12 to R19 implement the spec revision recorded in the Clarifications session of 2026-10-12 (brainstorm 093 revisit). They supersede the sidebar parts of R7 (glyph coloring, `(agent, profile)` show rule) and extend R1, R3 and R4.

## R12. Edge bar uses the existing left margin column

**Finding**: Both text lines of a sidebar entry already start with a blank margin column (`render.rs`: line 1 is `" {indicator} …"`, `format_line2` starts every variant with at least one space). On the active and cursor rows the highlight background is applied from that margin column onward.

**Decision**: The edge bar replaces the margin space with `▌` rendered in the profile color on the default background. Profiled rows start the highlight background at column 2, so the bar never carries `ACTIVE_BG` or `CURSOR_BG` (FR-016a). Unprofiled rows keep the margin space exactly as today, including the highlight on the active row. Rename mode, faded status and paused dimming change only columns 2 and later. Because no column is added, the layout is identical with and without profiles, and no width is lost on narrow sidebars. The spec edge case about truncation is corrected in the same commit.

**Rationale**: Zero layout cost, no conditional column reservation, no change in truncation math (`pad`, `truncate_name`, `BRANCH_COL` stay as they are).

**Alternatives considered**: a new leading column reserved only when profiles exist (rejected: shifts every name by one column when the first profiled session starts, and needs a payload flag); coloring the existing margin with a background block (rejected: indistinguishable from the highlight on active rows).

## R13. Sidebar data flow after the revision

**Decision**:
- `RenderSession.agent_color` is renamed to `profile_color: Option<(u8,u8,u8)>`. It is set from `Session.profile_color` whenever the session has a profile and drives only the bar.
- The harness glyph always uses `agent_indicator_color()`. `show_agent_indicators` reverts to "more than one distinct `agent_name` among visible sessions" (the pre-feature rule).
- A profile `icon` still arrives as `agent_indicator` from the hook, so it shows only when the glyph is shown (FR-018).
- `LegendEntry` keeps `{indicator, color, name}`. The help overlay renders `▌ <name>` in the profile color, followed by the icon when the profile declares one. Entries are distinct by profile name, sorted by name.
- `Session` fields, the `HookPayload`, `dump-state` and restore-meta are unchanged from R7 and R8.

**Rationale**: Only the render inputs change; the controller state and the pipe contracts stay compatible with the earlier implementation on this branch.

## R14. Palette with guaranteed hue separation

**Finding**: The current palette violates SC-011. Yellow `#FFCE54` and amber `#F6BB42` are about 4 degrees apart in hue, and red and pink about 30.

**Decision**: Replace the palette with eight hues spaced 45 degrees apart (HSL saturation 70 percent, lightness 62 percent; the blue entry uses lightness 72 percent for contrast): `#E27C5A` (15), `#E2E25A` (60), `#7CE25A` (105), `#5AE29E` (150), `#5AC0E2` (195), `#8686EA` (240), `#C05AE2` (285), `#E25A9E` (330). Every entry has a contrast ratio of at least 4.5:1 against black and against a typical dark terminal background (`#1E1E2E`).
- `Derive(name)` keeps FNV-1a modulo 8 over the new palette.
- New `Pick(used []string) string` returns the palette entry that maximizes the minimum hue distance to the `used` colors, with ties broken by the lowest index; `cc-deck config profile add` calls it with the effective colors of all existing profiles and writes the result into `color` (FR-017).
- New `HueDistance(a, b string) float64` is circular (0 to 180). Colors with HSL saturation below 0.15 count as achromatic: two achromatic colors are "similar" when their lightness differs by less than 0.15; an achromatic and a chromatic color are never similar.
- `Similar(a, b string) bool` (hue distance under 40, or the achromatic rule) feeds the sync warning (FR-017b).

**Rationale**: Fixed 45-degree steps satisfy SC-011 by construction; `Pick` makes `add` deterministic and testable.

**Alternatives considered**: golden-angle hue generation (rejected: unbounded palette, harder to test contrast); keeping the old palette and only adding `Pick` (rejected: two entries are near-identical, so `Pick` could still choose them once six colors are in use).

The palette change alters derived colors for profiles without `color`. The feature is unreleased, so no migration is needed.

## R15. `auth.oauth_token`

**Decision**:
- `AuthConfig` gains `OAuthToken *CredentialSource \`yaml:"oauth_token,omitempty"\``.
- Validation:
  - `oauth_token` allows `env` or `file`; `secret` is an error.
  - It is allowed only for harness `claude`, and only with the `anthropic` backend.
  - `login`, `api_key` and `oauth_token` are mutually exclusive.
  - The rule "anthropic backend requires api_key or login" becomes "requires api_key, oauth_token or login".
- `credential.ResolveProfile` resolves it with the logical name `oauth_token`, so a file source lands at `~/.config/cc-deck/profiles/<name>/oauth_token` (0600) on remotes. Env sources travel through the existing transport unchanged.
- The Claude translator renders the same check-then-export pattern as `api_key`, exporting `CLAUDE_CODE_OAUTH_TOKEN` instead of `ANTHROPIC_API_KEY`.
- FR-026 maps an `oauth_token` profile to provider type `claude`, like `api_key`.
- `cc-deck config profile add` gains `--oauth-token-env` and `--oauth-token-file`, and the interactive prompt offers "subscription token" as an auth choice for Claude.
- `list` shows `token:env:NAME` or `token:file` in the AUTH column.

**Rationale**: Reuses every existing path for env and file credentials; the only new knowledge is the target variable name.

**Known limitation**: Under OpenShell credential masking a token profile hits the same renamed-variable problem as `api_key` profiles (brainstorm 094, issue #38). That fix is out of scope here and is noted in the guide.

## R16. Clearing competing auth variables (FR-007a)

**Decision**: Each translator declares `AuthVars() map[AuthKind][]string`, the harness environment variables that select or carry each auth kind:

| Harness | Kind | Variables |
|---------|------|-----------|
| claude | api_key | `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` |
| claude | oauth_token | `CLAUDE_CODE_OAUTH_TOKEN` |
| claude | vertex | `CLAUDE_CODE_USE_VERTEX` |
| claude | bedrock | `CLAUDE_CODE_USE_BEDROCK` |
| codex, opencode | api_key | `OPENAI_API_KEY` (opencode with anthropic backend: `ANTHROPIC_API_KEY`) |

Kinds and their wrappers:
- The profile's own kind is derived from its auth and backend. A Claude login profile owns no variable, so all of them are cleared.
- Vertex profiles count as kind `vertex`, because they export `CLAUDE_CODE_USE_VERTEX`, while their credential file lives in `GOOGLE_APPLICATION_CREDENTIALS`.
- `GOOGLE_APPLICATION_CREDENTIALS`, `ANTHROPIC_VERTEX_PROJECT_ID` and `CLOUD_ML_REGION` are never cleared. They are inert without `CLAUDE_CODE_USE_VERTEX`, and clearing `GOOGLE_APPLICATION_CREDENTIALS` would break Vertex profiles that rely on ambient ADC.

`Render` emits one `unset` line, right after the `CC_DECK_PROFILE*` exports. It lists, in sorted order, every variable of every kind other than the profile's own. The existing order puts `env` entries after the unset line, which satisfies "env map applied after clearing". Validation adds a warning, emitted by `Sync` and `Provision` rather than `config.Validate()` because the `config` package cannot import the translators: a profile `env` key that appears in `AuthVars()` under a different kind produces `profile "%s": env %q belongs to auth kind %q and overrides the profile's credential`.

**Rationale**: A table per translator keeps the knowledge with the harness, and it is directly testable through the contract test (every rendered wrapper contains the expected `unset` line).

**Alternatives considered**: `env -i` style clean environment (rejected: drops PATH, locale, terminal and user tooling variables); clearing every auth variable including the profile's own (rejected: an `env` source deliberately reads a host variable such as `ANTHROPIC_API_KEY_WORK`, which is distinct, but a user may legitimately source from `ANTHROPIC_API_KEY` itself).

## R17. Profile color and icon in the wrapper environment (FR-014a)

**Decision**: `ResolvedProfile.Color` is already the effective color (declared or derived). The wrapper template exports it, plus the icon when one is declared, right after `CC_DECK_PROFILE`:

```sh
export CC_DECK_PROFILE='work'
export CC_DECK_PROFILE_COLOR='#5AC0E2'
export CC_DECK_PROFILE_ICON='W'      # only when declared
```

`cmd/hook.go` order:
1. If `CC_DECK_PROFILE` is empty, do nothing (unchanged).
2. If `CC_DECK_PROFILE_COLOR` passes `profile.ParseHex`, use it, and take the icon from `CC_DECK_PROFILE_ICON` when it is non-empty. Do not load `config.yaml`.
3. Otherwise run the existing config lookup (declared color, else `Derive`), including the unknown-profile path that already derives from the name (clarification Q1).

A test asserts that the env path performs no config load by pointing `XDG_CONFIG_HOME` at an unreadable directory.

**Rationale**: Removes the per-event YAML parse (idea inbox `hook-profile-env-cache`) while keeping wrappers generated before this revision working through the fallback.

## R18. Sync color-collision warning (FR-017b)

**Decision**: After rendering, `Sync` and `Provision` compute the effective color of every valid profile and append one warning per similar pair (R14 `Similar`), naming both profiles and suggesting `color:`. Warnings do not change the exit code. `Sync` never writes `config.yaml`.

## R19. Kubernetes deploy guard for `oauth_token`

**Finding**: On this branch the Kubernetes deploy backend does not render or inject profile auth. It consumes the selected profile only for git credentials (`ws/repos.go`), and agent credentials come from the adapters' `CredentialSpecs`.

**Decision**: `K8sDeployWorkspace.Create` resolves the active profile through `cfg.ResolveProfile("")`, the same selection `loadActiveGitCredentials` uses (there is no cc-deck `--profile` flag on this backend). When that profile has `auth.oauth_token`, it returns `profile "<name>": subscription tokens (auth.oauth_token) are not supported on the Kubernetes deploy backend` before creating any resource (FR-025). A unit test covers the guard without a cluster.
