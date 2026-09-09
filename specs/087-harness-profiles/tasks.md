# Tasks: Harness Profiles

**Input**: Design documents from `/specs/087-harness-profiles/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (harness-translator.md, profile-schema.md, wrapper-script.md, hook-payload.md), quickstart.md

**Tests**: Included. Constitution principle I requires unit tests for new code; the translator contract (contracts/harness-translator.md section 2.5) mandates a contract test that runs against every registered translator.

**Organization**: Tasks are grouped by user story. Paths are repository-relative. Go code lives under `cc-deck/`, Rust under `cc-zellij-plugin/`. Build only via `make test`, `make lint`, `make install` (constitution III).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1 (launch under profile), US2 (sidebar), US3 (SSH and OpenShell), US4 (snapshot), US5 (CLI and compat)

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package skeletons and the embedded template so later tasks compile independently.

- [x] T001 Create package `cc-deck/internal/profile/` with `doc.go` (package comment describing translator, wrapper and sync responsibilities) and an empty `templates/` directory holding `templates/wrapper.sh.tmpl` with the shebang and marker line only (contracts/wrapper-script.md)
- [x] T002 [P] Create package `cc-deck/internal/shellrc/` with `doc.go` describing the managed block markers `# >>> cc-deck >>>` and `# <<< cc-deck <<<`
- [x] T003 [P] Add a `docs/modules/guides/pages/harness-profiles.adoc` stub with title only and register it in `docs/modules/guides/nav.adoc` so Antora builds from the first commit

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Schema, validation, agent interface additions, credential resolution and the translator contract. Every user story depends on these.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Extend `Profile` in `cc-deck/internal/config/profile.go` with `Harness`, `Auth *AuthConfig`, `Env`, `Color`, `Icon`; add `AuthConfig` and `CredentialSource` types with the YAML tags from data-model.md; make `Backend` `omitempty`
- [x] T005 Add accessors `HarnessName()`, `EffectiveBackend()`, `EffectiveAuth()`, `CredentialSource.Kind()` and `WrapperName(binary)` in `cc-deck/internal/config/profile.go` mapping legacy `api_key_secret` and `credentials_secret` into the auth view (research R1)
- [x] T006 Rewrite `Profile.Validate()` in `cc-deck/internal/config/profile.go` to accept legacy anthropic and vertex profiles unchanged and to require, for anthropic and openai backends, one of legacy secret, `auth.api_key` (any source) or `auth.login`
- [x] T007 Add the profile validation rules from data-model.md ("Validation rules" table: name pattern, known harness, backend per harness, exactly-one source, login exclusivity, login only for claude, color syntax, icon width via `unicode/utf8` plus `golang.org/x/text` or `github.com/mattn/go-runewidth` if already a dependency, env key identifier, wrapper name shadowing) to `validateProfiles` in `cc-deck/internal/config/validate.go` using `Finding` with `CategoryProfiles`
- [x] T008 [P] Write `cc-deck/internal/config/profile_test.go`: YAML round-trip of the worked example in contracts/profile-schema.md, `EffectiveAuth()` for legacy and new profiles, `Save()` omits unset new fields, pre-feature fixture loads and validates without findings (SC-005)
- [x] T009 [P] Extend `cc-deck/internal/config/validate_test.go` with one sub-test per new rule in T007 using the existing `findFinding` helper
- [x] T010 Add `Binary() string`, `InstallHooksAt(configDir string) error` and `ResumeArgs(sessionID string) []string` to the `Agent` interface in `cc-deck/internal/agent/agent.go`; add `Profile` and `ProfileColor` (`json:"profile,omitempty"`, `json:"profile_color,omitempty"`) to `NormalizedPayload`
- [x] T011 [P] Implement the three methods in `cc-deck/internal/agent/claude.go`: `Binary()` returns `claude`, `InstallHooksAt` refactors the existing settings.json merge to take a directory (writing through symlinks per contract section 1), `InstallHooks()` delegates to it with the default dir, `ResumeArgs` returns `["--resume", id]`
- [x] T012 [P] Implement the three methods in `cc-deck/internal/agent/codex.go` (`codex`, hooks.json at the given dir, `["resume", id]`)
- [x] T013 [P] Implement the three methods in `cc-deck/internal/agent/opencode.go` (`opencode`, plugin and config under the given dir, `["--session", id]`)
- [x] T014 [P] Add tests for `Binary`, `InstallHooksAt` (temp dir, symlinked settings file written through, foreign hooks preserved, idempotent) and `ResumeArgs` in `cc-deck/internal/agent/claude_test.go`, `codex_test.go`, `opencode_test.go`
- [x] T015 Create `cc-deck/internal/credential/profile.go` with `ResolveProfile(name string, p config.Profile) (*ResolvedCredentials, error)`: env source reads the named host variable into `EnvVars[NAME]`, file source reads the host file into a `ResolvedFile` destined for `~/.config/cc-deck/profiles/<name>/<field>`, secret source returns a typed `ErrSecretSourceUnsupported`, login yields empty credentials; missing env or file returns a typed `ErrCredentialUnavailable` carrying profile and reference
- [x] T016 [P] Write `cc-deck/internal/credential/profile_test.go` covering all four source kinds and both error types
- [x] T017 Create `cc-deck/internal/profile/translator.go` with the `Translator` interface, `ResolvedProfile`, `WrapperScript`, `Register`, `Lookup`, `All`, and `Resolve(name string, p config.Profile, a agent.Agent) (ResolvedProfile, error)` that computes `ConfigDir`, `CredDir`, `BinDir` from `internal/xdg` and fills `Color` via `color.go` (contracts/harness-translator.md section 2)
- [x] T018 [P] Create `cc-deck/internal/profile/color.go` with the 8-entry palette from data-model.md, `Derive(name string) string` (FNV-1a 32 modulo 8) and `ParseHex(s string) (r, g, b uint8, err error)`; add `color_test.go` asserting determinism, palette membership, and that every palette entry has contrast ratio >= 3:1 against `(25,45,55)` and `(0,0,0)` using the WCAG relative-luminance formula
- [x] T019 Create `cc-deck/internal/profile/contract_test.go` implementing the six behaviors of contracts/harness-translator.md section 2.5 as a table over `profile.All()`, plus the `agent.All()` pairing check; it must compile and fail with "no translators registered" until Phase 3 adds them

**Checkpoint**: `make test` passes for `internal/config`, `internal/agent`, `internal/credential`; `internal/profile` compiles with the contract test skipped or failing on an empty registry.

---

## Phase 3: User Story 1 - Launch a Session Under a Named Profile (Priority: P1) 🎯 MVP

**Goal**: `cc-deck config profile sync` renders one wrapper per valid profile into `~/.local/share/cc-deck/bin`, prepares per-profile config dirs with hooks, adds the bin dir to `PATH`, and `claude-<name>` starts the harness with the profile's credential and model.

**Independent Test**: Define a `claude` profile with an env-sourced key and a model, run `sync`, run `claude-<name>` in a Zellij pane, verify the model, `CC_DECK_PROFILE`, and hook events in the sidebar (quickstart sections 1 and 2).

### Implementation for User Story 1

- [x] T020 [US1] Implement `cc-deck/internal/profile/wrapper.go`: `go:embed templates/wrapper.sh.tmpl`, `renderWrapper(rp ResolvedProfile, lines wrapperLines) (WrapperScript, error)` with single-quote escaping helper and sorted env keys; complete `templates/wrapper.sh.tmpl` to the layout in contracts/wrapper-script.md (marker line, `CC_DECK_PROFILE` first, config dir export, model, env, credential checks, `exec <binary> "$@"`)
- [x] T021 [US1] Implement `cc-deck/internal/profile/configdir.go`: `PrepareSharedDir(configDir, defaultDir string, isolated []string) ([]string, error)` creating the dir (0700), relative symlinks for non-isolated top-level entries, repointing stale links, warning on real files, never deleting (contract section 2.3)
- [x] T022 [US1] Implement the Claude translator in `cc-deck/internal/profile/claude.go` per contract section 2.4: `CLAUDE_CONFIG_DIR`, isolated `.credentials.json` and `statsig`, backends anthropic (default) and vertex, `SupportsLogin` true, `Render` producing the anthropic, vertex and login layouts from contracts/wrapper-script.md, `PrepareConfigDir` calling `PrepareSharedDir`, `ProviderType` mapping; register in `init()`
- [x] T023 [P] [US1] Implement `cc-deck/internal/shellrc/block.go`: `Ensure(path, content string) (changed bool, err error)` writing or replacing the marker-delimited block, creating the file if absent, preserving everything else byte for byte; `EnsureAll(home string) (changed bool, err error)` for `.bashrc` and `.zshrc`; add `block_test.go` for create, update, no-op, and preservation of surrounding content
- [x] T024 [US1] Implement `cc-deck/internal/profile/sync.go`: `Sync(cfg *config.Config, home string) (SyncResult, error)` that resolves every profile, skips invalid ones and secret-only sources with reasons, checks `Agent.IsInstalled()`, calls `PrepareConfigDir` then `Agent.InstallHooksAt`, copies file credentials into `CredDir` (0600), renders wrappers, writes changed ones (0755) and removes stale cc-deck wrappers (marker line check), calls `shellrc.EnsureAll`, and warns per unavailable credential without failing
- [x] T025 [P] [US1] Write `cc-deck/internal/profile/sync_test.go` against a temp `HOME` with `XDG_*` overrides: first run writes, second run is a no-op, deleted profile removes its wrapper, non-cc-deck file in bin dir is untouched, rendered wrapper passes `sh -n`, wrapper contains no credential value, missing env var is reported in `Warnings`, no file named after a bare harness binary (`claude`, `codex`, `opencode`) is ever written to the bin dir (FR-012), and a sync of 10 profiles completes in under 2 s wall-clock (plan performance goal)
- [x] T026 [US1] Add `sync` subcommand to `cc-deck/internal/cmd/profile.go` (local mode; `--workspace` flag registered but returns "not yet supported" until T050) printing written, removed, skipped, warnings and the new-shell hint when `RCChanged`
- [x] T027 [US1] Call a lightweight `profile.EnsureLocal(cfg)` (sync without output) from `LocalWorkspace.Attach` in `cc-deck/internal/ws/local.go` before the Zellij session starts, so wrappers exist even if the user never ran `sync` explicitly; log warnings at debug level
- [x] T028 [P] [US1] Implement the Codex translator in `cc-deck/internal/profile/codex.go` per contract section 2.4 (`CODEX_HOME`, isolated `auth.json`, `config.toml`, `sessions`; generate `config.toml` with `model = "<m>"` in `PrepareConfigDir`; backend openai; `Render` per the Codex layout); register in `init()`
- [x] T029 [P] [US1] Implement the OpenCode translator in `cc-deck/internal/profile/opencode.go` per contract section 2.4 (`OPENCODE_CONFIG` pointing at a generated `opencode.json` that extends the default config with `model` and the cc-deck plugin path; backends openai (default) and anthropic); register in `init()`
- [x] T030 [US1] Make `contract_test.go` (T019) pass for all three translators; add per-translator golden tests in `cc-deck/internal/profile/claude_test.go`, `codex_test.go`, `opencode_test.go` comparing `Render` output to the layouts in contracts/wrapper-script.md

**Checkpoint**: quickstart sections 1 and 2 pass on the host for `claude`; `make test` and `make lint` pass.

---

## Phase 4: User Story 2 - Distinguish Profiles in the Sidebar (Priority: P1)

**Goal**: Hook events carry profile and color; the sidebar colors the harness glyph per profile, shows glyphs when more than one (harness, profile) pair runs, honors `icon`, and lists a legend under `?`.

**Independent Test**: Run `claude-work` and `claude-private`, observe two colored glyphs; stop one, glyph disappears; `?` lists both profiles (quickstart section 3).

### Implementation for User Story 2

- [x] T031 [US2] In `cc-deck/internal/cmd/hook.go` read `CC_DECK_PROFILE`; when set, load config, set `normalized.Profile`, `normalized.ProfileColor` (declared or `profile.Derive`), and replace `AgentIndicator` with the profile `icon` when declared; unknown profile still sends the name with a derived color (contracts/hook-payload.md section 1)
- [x] T032 [P] [US2] Add `cc-deck/internal/cmd/hook_profile_test.go` covering: no env var, known profile with color, known profile without color, unknown profile, icon override
- [x] T033 [P] [US2] Add `profile: Option<String>` and `profile_color: Option<String>` with `#[serde(default)]` to `HookPayload` in `cc-zellij-plugin/src/pipe_handler.rs`; update `make_hook()`-style test helpers and inline literals in `cc-zellij-plugin/src/controller/hooks.rs` tests and `cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs`
- [x] T034 [P] [US2] Add `profile: Option<String>` and `profile_color: Option<(u8,u8,u8)>` with `#[serde(default)]` to `Session` in `cc-zellij-plugin/src/session.rs` and a serde round-trip test proving old JSON without the fields still loads
- [x] T035 [US2] In `cc-zellij-plugin/src/controller/hooks.rs` set `profile`/`profile_color` on the first hook carrying them (same guard as `agent_name`), update `profile_color` on change, and reset or overwrite them in both session-replacement branches; parse `#RRGGBB` into the tuple with a small helper
- [x] T036 [US2] Add `agent_color: Option<(u8,u8,u8)>` to `RenderSession` and `profile_legend: Vec<LegendEntry>` plus `LegendEntry` to `RenderPayload` in `cc-zellij-plugin/src/lib.rs`; update `test_helpers.rs` constructors
- [x] T037 [US2] Change the show rule in `cc-zellij-plugin/src/controller/render_broadcast.rs` to count distinct `(agent_name, profile)` pairs, populate `agent_color` from `profile_color`, and build `profile_legend` sorted by name (contracts/hook-payload.md section 2)
- [x] T038 [US2] In `cc-zellij-plugin/src/sidebar_plugin/render.rs` use `agent_color` when present in both the plain and highlighted line-1 branches, and append a dynamic `Profiles` section (indicator, color swatch, name) to the help overlay when `profile_legend` is non-empty
- [x] T039 [P] [US2] Add controller tests in `cc-zellij-plugin/src/controller/integration_tests.rs`: one harness two profiles shows indicators; two harnesses one profile each shows indicators; same harness same profile hides; unprofiled plus profiled shows; legend content and order; profile_color update on later hook
- [x] T040 [P] [US2] Add sidebar tests in `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs` asserting `agent_color` reaches the rendered session and the help overlay contains the legend names

**Checkpoint**: `make install` then quickstart section 3 passes.

---

## Phase 5: User Story 3 - Profiles Travel to SSH and OpenShell Workspaces (Priority: P2)

**Goal**: Attaching an SSH workspace or creating an OpenShell workspace delivers wrappers, profile config dirs, credential files and the rc block; OpenShell gets one provider per applicable profile (FR-026).

**Independent Test**: quickstart sections 5 and 6.

### Implementation for User Story 3

- [x] T041 [US3] Extend `cc-deck/internal/credential/transport.go` so `InjectSSH` and `InjectOpenShell` honor a per-file destination (`ResolvedFile.Dest`, relative to the remote home) and upload all entries of `FileCredentials`, not only `files[0]` (closes the multi-file limitation for profile files)
- [x] T042 [P] [US3] Add tests in `cc-deck/internal/credential/transport_test.go` for multi-file upload and custom destinations using the existing fake SSH client pattern
- [x] T043 [US3] Add `Provision(cfg *config.Config, t Target) (SyncResult, error)` to `cc-deck/internal/profile/sync.go` with a `Target` interface (`Upload(path string, content []byte, mode os.FileMode) error`, `Run(cmd string) (string, error)`, `Home() string`, `Agents() []string`): renders wrappers for profiles whose harness is in `Agents()` and present remotely (`command -v`), uploads them, generates and runs a `prepare.sh` that creates config dirs and symlinks remotely (same rules as `configdir.go`), uploads credential files, and appends the rc block through `Run`
- [x] T044 [P] [US3] Add `cc-deck/internal/profile/provision_test.go` with an in-memory `Target` recording uploads and commands: correct wrapper set, `prepare.sh` content, rc block appended once, missing harness skipped with reason
- [x] T045 [US3] Implement an SSH `Target` adapter over `ssh.Client` in `cc-deck/internal/ws/ssh.go` and call `profile.Provision` in `SSHWorkspace.Attach` right after `credential.InjectSSH`; surface `SyncResult` skips and warnings in the attach output
- [x] T046 [US3] Implement an OpenShell `Target` adapter over `openShellDataChannel.PushBytes` and `Exec` in `cc-deck/internal/ws/openshell.go` and call `profile.Provision` in `Create` after `credential.InjectOpenShell`
- [x] T047 [US3] In `OpenShellWorkspace.Create` (`cc-deck/internal/ws/openshell.go`) iterate valid profiles whose harness is in the workspace agent list, resolve each with `credential.ResolveProfile`, map through the translator's `ProviderType` and `mapToOpenShellProvider`, name providers `cc-deck-<ws>-<profile>`, and `Ensure` them alongside the existing providers (FR-026); record the provider set in workspace state
- [x] T048 [P] [US3] Add tests in `cc-deck/internal/ws/openshell_test.go` (fake SDK client) asserting one provider per applicable profile, deduplication with the existing single-spec provider, and no provider for profiles of other harnesses
- [x] T049 [P] [US3] Prepend `{{.HomeDir}}/.local/share/cc-deck/bin` to the `PATH` export in `cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl` and update the corresponding golden test in `cc-deck/internal/build/`
- [x] T050 [US3] Implement `--workspace <name>` in the `sync` subcommand (`cc-deck/internal/cmd/profile.go`): look up the workspace, build the matching `Target`, call `Provision`, and for OpenShell compare the recorded provider set with the current profiles to print the re-create hint from FR-026

**Checkpoint**: quickstart sections 5 and 6 pass against a real SSH host and an OpenShell gateway.

---

## Phase 6: User Story 4 - Snapshot and Restore Preserve Profiles (Priority: P2)

**Goal**: Snapshots record agent and profile per session; restore relaunches through the matching wrapper with the harness's resume arguments and warns on missing profiles.

**Independent Test**: quickstart section 4.

### Implementation for User Story 4

- [x] T051 [P] [US4] Add `Agent` and `Profile` (`json:"agent,omitempty"`, `json:"profile,omitempty"`) to `SessionEntry` in `cc-deck/internal/session/snapshot.go` and to `pluginSession` in `cc-deck/internal/session/save.go` (reading `agent_name` and `profile` from the dump-state JSON); map them in the conversion loop
- [x] T052 [US4] Replace the hardcoded `claude --resume` in `cc-deck/internal/session/restore.go` with a `launchCommand(entry SessionEntry, cfg *config.Config) (cmd string, warning string)` helper: binary from `agent.Get(entry.Agent)` (default claude), wrapper name when `Profile` is set and exists in config, plain binary plus a warning when it does not, `ResumeArgs(entry.SessionID)` appended; add `Profile` and `ProfileColor` to `PendingOverride` and send them on the restore-meta pipe
- [x] T053 [P] [US4] Add `cc-deck/internal/session/restore_test.go` covering the five rows of the table in contracts/hook-payload.md section 5 and the pre-feature snapshot fixture (no `agent`, no `profile`)
- [x] T054 [US4] In `cc-zellij-plugin/src/controller/mod.rs` (restore-meta handler) apply `profile` and `profile_color` from the override so the entry is colored before the first hook; add a controller test in `cc-zellij-plugin/src/controller/integration_tests.rs`

**Checkpoint**: quickstart section 4 passes including the missing-profile and pre-feature-snapshot cases.

---

## Phase 7: User Story 5 - Manage Profiles from the CLI (Priority: P3)

**Goal**: `cc-deck config profile add/list/show/delete` cover the new schema, `use` keeps its meaning, and pre-feature configurations keep working.

**Independent Test**: quickstart section 7 plus the CLI walkthrough in section 1.

### Implementation for User Story 5

- [x] T055 [US5] Extend `add` in `cc-deck/internal/cmd/profile.go` with the flags from contracts/profile-schema.md (`--harness`, `--backend`, `--model`, `--api-key-env`, `--api-key-file`, `--credentials-file`, `--login`, `--project`, `--region`, `--env K=V`, `--color`, `--icon`); validate through `Config.Validate()` before saving; keep the interactive fallback and extend `PromptProfile` in `cc-deck/internal/config/profile.go` to ask for harness first
- [x] T056 [US5] Extend `list` (HARNESS, BACKEND, AUTH, MODEL, DEFAULT columns and the JSON/YAML entry struct) and `show` (all fields, source references only, resolved wrapper name and color) in `cc-deck/internal/cmd/profile.go`
- [x] T057 [US5] Add `delete <name>` to `cc-deck/internal/cmd/profile.go` using `Config.DeleteProfile`, printing the `sync` reminder; leave `use` unchanged and add a doc comment stating it does not affect wrappers (FR-024)
- [x] T058 [P] [US5] Add `cc-deck/internal/cmd/profile_test.go` (cobra command execution against a temp config path): add with flags writes the expected YAML, add rejects invalid color and icon, list output columns, show masks nothing but prints references, delete clears `default_profile`
- [x] T059 [P] [US5] Add a backward-compat integration test in `cc-deck/internal/ws/repos_test.go` (or the existing git credential test file) proving `loadActiveGitCredentials` still resolves a legacy profile with `git_credential_*` fields after the schema change

**Checkpoint**: All five stories independently verified; `make verify` passes.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation required by constitution I, final validation.

- [x] T060 [P] Document the extended profile schema (`harness`, `auth` sources, `model`, `env`, `color`, `icon`, legacy defaults, file locations for wrappers and profile dirs) in `docs/modules/reference/pages/configuration.adoc` using the prose plugin with the `cc-deck` voice, one sentence per line
- [x] T061 [P] Document `cc-deck config profile sync`, `delete`, and the extended `add`, `list`, `show` in `docs/modules/reference/pages/cli.adoc`
- [x] T062 [P] Write the guide `docs/modules/guides/pages/harness-profiles.adoc`: the two-account use case end to end, sidebar legend, snapshot behavior, the one-time login step for `login` profiles in remote workspaces, and the OpenShell re-create hint
- [x] T063 [P] Add a "Harness profiles" paragraph to `README.md` under the features section
- [x] T064 Run `/prose:check` on the three AsciiDoc pages and README changes and fix findings
- [x] T065 Record the outcome of the three manual harness checks (quickstart section 8) in `specs/087-harness-profiles/contracts/harness-translator.md` section 2.4 and adjust the guide if macOS shares one login across profiles
- [x] T066 Run quickstart sections 1 to 7 and `make verify`; fix anything that fails

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies
- **Foundational (Phase 2)**: depends on Phase 1; blocks every story. Internal order: T004 → T005 → T006 → T007; T010 → T011/T012/T013; T015 after T004; T017 after T010 and T015; T018 independent; T019 after T017
- **US1 (Phase 3)**: after Phase 2. T020 → T022 → T024 → T026 → T027; T021 before T022; T023 before T024; T028 and T029 after T020/T021; T030 last
- **US2 (Phase 4)**: after Phase 2 and T018 (color); independent of US1 at the code level, testable with a manually exported `CC_DECK_PROFILE`
- **US3 (Phase 5)**: after US1 (needs `Render`, `configdir.go`); T041 → T043 → T045/T046; T047 after T015; T049 independent; T050 last
- **US4 (Phase 6)**: after Phase 2 (`ResumeArgs`, `Binary`) and T034/T035 for the plugin fields; independent of US3
- **US5 (Phase 7)**: after Phase 2; T055 to T057 touch the same file sequentially; T058 and T059 parallel
- **Polish (Phase 8)**: after all stories

### Parallel Opportunities

- Phase 2: T008, T009, T011, T012, T013, T014, T016, T018 run in parallel once their listed predecessors exist
- Phase 3: T023, T025, T028, T029 in parallel with each other
- Phase 4: T032, T033, T034, T039, T040 in parallel; T035 to T038 sequential in the plugin
- Phase 5: T042, T044, T048, T049 in parallel
- Phase 8: T060 to T063 in parallel

---

## Parallel Example: Phase 2

```bash
Task: "Implement Binary/InstallHooksAt/ResumeArgs in cc-deck/internal/agent/claude.go"
Task: "Implement Binary/InstallHooksAt/ResumeArgs in cc-deck/internal/agent/codex.go"
Task: "Implement Binary/InstallHooksAt/ResumeArgs in cc-deck/internal/agent/opencode.go"
Task: "Create cc-deck/internal/profile/color.go with palette and contrast test"
Task: "Write cc-deck/internal/config/profile_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "Add profile fields to HookPayload in cc-zellij-plugin/src/pipe_handler.rs"
Task: "Add profile fields to Session in cc-zellij-plugin/src/session.rs"
Task: "Add hook_profile_test.go in cc-deck/internal/cmd/"
```

---

## Implementation Strategy

### MVP First (User Story 1 plus User Story 2)

1. Phase 1 and Phase 2 (schema, agent additions, translator contract)
2. Phase 3 (local wrappers for Claude, then Codex and OpenCode)
3. Phase 4 (sidebar identity), because two profiles without a visual cue are hard to use
4. Validate with quickstart sections 1 to 3, `make verify`

### Incremental Delivery

1. US4 (snapshot) next: small, self-contained, closes the restore gap
2. US3 (SSH and OpenShell): needs real infrastructure to verify
3. US5 (CLI polish and compat tests)
4. Phase 8 documentation before merge (constitution I: same branch)

---

## Notes

- Never run `go build` or `cargo build` directly; use `make test`, `make lint`, `make install`
- Use `internal/xdg` for every path; `$HOME`-relative strings inside rendered scripts
- Generated wrappers must never contain credential values (SC-006); the sync test greps for the test key
- Commit after each task or logical group with the required attribution trailers
