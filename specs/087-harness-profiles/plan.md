# Implementation Plan: Harness Profiles

**Branch**: `087-harness-profiles` | **Date**: 2026-09-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/087-harness-profiles/spec.md`

## Summary

Introduce named, agent-specific profiles in `config.yaml` (harness, auth source variants, model, env, color, icon) and a per-harness translator that renders each profile into a wrapper command (`claude-work`, `codex-team`) placed in a cc-deck-managed bin directory inside local, SSH and OpenShell workspaces. The wrapper exports `CC_DECK_PROFILE`, sets the harness environment and an isolated-auth config directory, and execs the real binary. The hook carries the profile name and color to the Zellij plugin, which colors the harness glyph and lists a legend; snapshots record agent and profile per session and restore relaunches through the matching wrapper. Details and alternatives are in [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.25 (CLI, `cc-deck/`), Rust stable edition 2021 targeting `wasm32-wasip1` (plugin, `cc-zellij-plugin/`)

**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (config), `text/template` and `go:embed` (wrapper rendering), `internal/xdg` (paths), `internal/credential` (transport), `internal/ssh` and `internal/openshell` (delivery), zellij-tile 0.43.1, serde/serde_json, unicode-width (plugin)

**Storage**: `~/.config/cc-deck/config.yaml` (profiles, existing), `~/.config/cc-deck/profiles/<name>/` (credential files, 0600, new), `~/.local/share/cc-deck/bin/` (wrappers, new), `~/.local/share/cc-deck/profiles/<name>/<harness>/` (per-profile harness config dirs, new), `~/.local/state/cc-deck/sessions/*.json` (snapshots, extended), plugin `/cache/sessions-<pid>.json` (extended)

**Testing**: `make test` (Go stdlib `testing`, testify available; `cargo test`), `make lint`, `make verify`

**Target Platform**: macOS and Linux hosts; Linux remote workspaces (SSH, OpenShell sandbox at `/sandbox`)

**Project Type**: CLI plus WASM plugin (existing monorepo layout)

**Performance Goals**: `profile sync` completes in under 2 s for 10 profiles; no measurable change to hook latency (one config load per hook event, already done for badges)

**Constraints**: wrappers must be POSIX `sh` and identical across backends; no secrets in generated files; existing `config.yaml` and snapshot files load unchanged; no new external Go or Rust dependencies

**Scale/Scope**: up to 10 profiles, 3 harnesses, 3 wrapper-capable backends; roughly 25 Go files touched or added, 8 Rust files, 4 documentation pages

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Tests and documentation | PASS (planned) | Unit tests per new package and per changed Rust module; README, `cli.adoc`, `configuration.adoc`, new guide `guides/harness-profiles.adoc` in the same branch; prose plugin with `cc-deck` voice |
| II. Interface contracts | PASS | `contracts/harness-translator.md` is written in this plan before any translator exists; `agent.Agent` additions are documented there; existing adapters are read before extension (research R2, R4) |
| III. Build and tool rules | PASS | Only `make test`, `make lint`, `make install`; `internal/xdg` for all paths; no container work |
| IV. Plugin debug logging | PASS | Existing `debug!` logger reused for the new show rule; no new logging path |
| V. Command files are code | N/A | No `internal/build/commands/*.md` change; the Containerfile template change is covered by Go template tests |

No violations. Complexity Tracking stays empty.

## Project Structure

### Documentation (this feature)

```text
specs/087-harness-profiles/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1 to R11
├── data-model.md        # Phase 1: Profile, AuthConfig, CredentialSource, Session fields, SessionEntry
├── quickstart.md        # Phase 1: end-to-end validation walkthrough
├── contracts/
│   ├── harness-translator.md   # Behavioral contract for Translator and Agent additions
│   ├── profile-schema.md       # config.yaml profile schema and validation rules
│   ├── wrapper-script.md       # Generated wrapper layout and failure behavior
│   └── hook-payload.md         # Hook JSON, RenderPayload, dump-state and snapshot additions
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/
├── config/
│   ├── profile.go              # extend Profile; AuthConfig, CredentialSource, EffectiveAuth()
│   ├── profile_test.go         # NEW: schema round-trip, EffectiveAuth, legacy compat
│   └── validate.go             # new profile rules (harness, sources, color, icon, name)
├── profile/                    # NEW package
│   ├── translator.go           # Translator interface, Registry, ResolvedProfile
│   ├── claude.go               # Claude translator (env, CLAUDE_CONFIG_DIR, shared/isolated list)
│   ├── codex.go                # Codex translator (CODEX_HOME, config.toml with model)
│   ├── opencode.go             # OpenCode translator (OPENCODE_CONFIG file)
│   ├── wrapper.go              # text/template rendering of wrapper.sh.tmpl
│   ├── templates/wrapper.sh.tmpl
│   ├── configdir.go            # PrepareConfigDir: symlink sharing, isolated entries
│   ├── sync.go                 # Sync(local) and Provision(remote target): bin dir, config dirs, files, rc block, stale cleanup
│   ├── color.go                # palette, Derive(name), Parse(#RRGGBB)
│   └── *_test.go
├── shellrc/                    # NEW package
│   ├── block.go                # idempotent managed block writer (bash and zsh)
│   └── block_test.go
├── agent/
│   ├── agent.go                # Binary(), InstallHooksAt(), ResumeArgs(); NormalizedPayload.Profile/ProfileColor
│   ├── claude.go, codex.go, opencode.go   # implement the three methods
│   └── *_test.go
├── credential/
│   ├── profile.go              # NEW: ResolveProfile(p) -> ResolvedCredentials (env var, file content)
│   └── transport.go            # profile file destination path support
├── cmd/
│   ├── profile.go              # add delete, sync; extend add/list/show
│   ├── hook.go                 # read CC_DECK_PROFILE, set profile fields, icon override
│   └── snapshot.go             # unchanged CLI, restore behavior via session package
├── ws/
│   ├── local.go                # Attach: ensure local sync
│   ├── ssh.go                  # Attach: profile.Provision after InjectSSH
│   └── openshell.go            # Create: per-profile providers (FR-026), Provision after InjectOpenShell
├── session/
│   ├── snapshot.go             # SessionEntry.Agent/Profile
│   ├── save.go                 # pluginSession.Agent/Profile mapping
│   └── restore.go              # wrapper-aware launch command, missing-profile warning, PendingOverride.Profile
└── build/templates/containerfile/05-shell-finalize.tmpl   # prepend cc-deck bin dir

cc-zellij-plugin/src/
├── pipe_handler.rs             # HookPayload.profile, profile_color
├── session.rs                  # Session.profile, profile_color (serde default)
├── controller/hooks.rs         # set-once and replacement handling for profile fields
├── controller/render_broadcast.rs   # show rule on (agent_name, profile); legend; agent_color
├── lib.rs                      # RenderSession.agent_color, RenderPayload.profile_legend, LegendEntry
├── sidebar_plugin/render.rs    # use agent_color; help overlay Profiles section
├── sidebar_plugin/test_helpers.rs
└── controller/hooks.rs tests, sidebar_plugin/integration_tests.rs

docs/modules/
├── reference/pages/configuration.adoc   # profile schema
├── reference/pages/cli.adoc             # config profile delete, sync, extended add/list/show
└── guides/pages/harness-profiles.adoc   # NEW guide (nav.adoc entry)
README.md                                # feature mention
```

**Structure Decision**: Existing monorepo layout. Two new Go packages (`internal/profile`, `internal/shellrc`) keep rendering and rc management out of `internal/agent` and `internal/ws`. No new Rust modules; the plugin change is additive fields plus one rule change in `render_broadcast.rs`.

## Phase 0: Research

Complete. See [research.md](research.md). All Technical Context items are resolved; two spec deviations are recorded there (R5: no existing rc snippet for local and SSH; R10: CLI path is `cc-deck config profile`) and the spec text is updated in the same commit as this plan.

## Phase 1: Design

Complete. Artifacts:

- [data-model.md](data-model.md): `Profile` (extended), `AuthConfig`, `CredentialSource`, `ResolvedProfile`, plugin `Session` additions, `RenderSession`/`RenderPayload` additions, `SessionEntry` additions, validation rules and state transitions for sync.
- [contracts/harness-translator.md](contracts/harness-translator.md): behavioral contract for `profile.Translator` and the three `agent.Agent` additions, written before the first implementation per constitution II.
- [contracts/profile-schema.md](contracts/profile-schema.md): YAML schema with examples for the two-account use case, validation error messages.
- [contracts/wrapper-script.md](contracts/wrapper-script.md): the generated script layout, exit codes, and what must never appear in it.
- [contracts/hook-payload.md](contracts/hook-payload.md): JSON additions on the hook pipe, `RenderPayload`, `dump-state`, restore-meta, and the snapshot file.
- [quickstart.md](quickstart.md): validation walkthrough for local, SSH, OpenShell, sidebar and snapshot, plus the three harness assumptions that need a manual check.

### Constitution re-check after design

Unchanged: PASS on all principles. The translator contract exists before implementation; tests and documentation are enumerated per file in the structure above and will become tasks.

## Implementation Order (for /speckit-tasks)

1. Config schema, `EffectiveAuth`, validation, tests (US5 groundwork, backward compat).
2. `agent.Agent` additions and adapter implementations, `credential.ResolveProfile`.
3. `internal/profile`: translator interface and registry, Claude translator, wrapper template, color, config dir preparation, `shellrc`, local `Sync`; `config profile sync`, `delete`, extended `add/list/show` (US1, US5).
4. Hook payload and plugin: Go hook fields, Rust `HookPayload`/`Session`/show rule/legend/render (US2).
5. Codex and OpenCode translators against the contract (US1 scenarios 2 and 2a).
6. Snapshot save and restore (US4).
7. SSH and OpenShell provisioning, FR-026 providers, Containerfile template (US3).
8. Documentation: configuration reference, CLI reference, guide, README; `/prose:check`.

## Complexity Tracking

No constitution violations to justify.
