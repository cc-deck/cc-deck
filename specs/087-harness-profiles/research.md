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
