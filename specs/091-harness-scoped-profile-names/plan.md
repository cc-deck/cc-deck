# Implementation Plan: Harness-Scoped Profile Names

**Branch**: `091-harness-scoped-profile-names` | **Date**: 2026-10-19 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/091-harness-scoped-profile-names/spec.md`

## Summary

Make `(harness, name)` the profile identity. Replace the flat YAML profile map with nested harness groups, add one selector resolver for CLI/default references, and use exact pair lookup throughout provisioning, credentials, hooks, snapshot restore, and sidebar legend. Migrate the current user's config and credential files once after the implementation is verified. Keep existing wrapper names and the already-isolated login/data directory.

## Technical Context

**Language/Version**: Go 1.25; Rust for the Zellij plugin

**Primary Dependencies**: Existing `gopkg.in/yaml.v3`, Cobra CLI, profile translators, credential transport, and Zellij plugin session model

**Storage**: `~/.config/cc-deck/config.yaml`; credential files under `~/.config/cc-deck/profiles/`; login/data under `~/.local/share/cc-deck/profiles/`; workspace state and snapshots

**Testing**: Existing Go and Rust test suites via `make test`; project lint and prose checks

**Target Platform**: Existing local, SSH, OpenShell, Kubernetes, container, and compose workspaces on supported hosts

**Project Type**: CLI plus Go runtime and Rust sidebar plugin

**Performance Goals**: Profile resolution remains bounded by the small configured profile set; no new network calls in CLI or hook paths

**Constraints**: Preserve wrapper names and existing snapshot shape; never embed secrets in wrappers; no overwrites during one-time migration

**Scale/Scope**: A small user-maintained profile set across three supported harnesses; one local user migration

## Constitution Check

| Principle | Design response | Status |
|---|---|---|
| Tests and documentation ship together | Add Go and Rust coverage, README, CLI/config references, harness guide, and prose checks | Pass |
| Interface behavioral contracts | Review translator, credential, and workspace behaviors; update relevant profile contracts | Pass |
| Build and tool rules | Use project `make` targets, existing XDG package, and Podman-only paths | Pass |
| Zellij is an implementation detail | User validation uses `cc-deck` commands only | Pass |

The design introduces no constitution exceptions. Rechecked after the data model and contracts: all four gates still pass.

## Phase 0: Research

[research.md](research.md) records decisions on profile representation, strict YAML validation, selectors, file paths, session identity, workspace provider naming, and migration. All technical-context choices are resolved.

## Phase 1: Design

The [data model](data-model.md) defines the pair identity and file ownership. The [profile identity contract](contracts/profile-identity.md) fixes config and CLI behavior. [Quickstart](quickstart.md) provides end-to-end checks, including the local migration gate.

## Global Constraints

- Supported harness groups: `claude`, `codex`, `opencode` (spec assumption).
- Qualified selector: `<harness>/<name>` (FR-004). Bare selector resolves only when unique (FR-005).
- Persisted default is qualified (FR-006). Normal startup never migrates flat config (FR-013).
- Wrapper command and the human profile-list `NAME` remain `<harness command>-<name>` (FR-007, FR-008); JSON/YAML list fields stay separate, selectors and defaults stay `harness/name`, and the short `CC_DECK_PROFILE` value and snapshot fields remain stable (FR-010, FR-011).
- Credential destination includes harness and name; data/login directory remains at its existing pair-isolated path (FR-009, FR-013).

## Interfaces Between Tasks

- `config.ProfileKey{Harness, Name string}` is the canonical in-memory identity. `String()` yields `harness/name`.
- `Config.Profiles` holds harness-to-name maps; `ListProfileKeys() []ProfileKey` provides deterministic traversal.
- `Config.GetProfileFor(harness, name string) (Profile, error)` is for runtime callers with a known harness. `Config.ResolveProfileRef(ref string) (ProfileKey, error)` handles CLI/default qualified and unique-bare selection.
- `Config.AddProfile(key ProfileKey, profile Profile) error`, `DeleteProfile(key ProfileKey) error`, and `SetDefaultProfile(ref string) error` centralize mutation rules. Existing `Profile.HarnessName()` may remain as an in-memory convenience populated from the outer group, but YAML must not accept an inner `harness` field.
- Profile rendering and credential transport receive `ProfileKey` or both identity components; wrapper names and config/data/credential paths derive from that key. OpenShell provider names use workspace plus both key parts.
- Hook and snapshot consumers already have the agent/harness plus short profile. They use `GetProfileFor` rather than bare resolution; Rust legend keys combine `agent` and `profile`.

## File Responsibility Map

| File group | Responsibility |
|---|---|
| `cc-deck/internal/config/{config,profile,validate}.go` | Nested schema, selector API, strict validation |
| `cc-deck/internal/cmd/profile.go` | Profile command parsing, output, and default persistence |
| `cc-deck/internal/profile/{translator,sync,claude,codex,opencode}.go` | Pair-aware rendering, sync, and credential path references |
| `cc-deck/internal/credential/profile.go` | Resolve one profile's credential sources with qualified diagnostics |
| `cc-deck/internal/ws/{openshell,profile_target,ssh,local,k8s_deploy,repos}.go` | Workspace provisioning, provider identity, and exact default lookup |
| `cc-deck/internal/cmd/{hook,codex_attribution,claude_bg_attribution}.go` | Use harness context for profile enrichment and background sessions |
| `cc-deck/internal/session/restore.go` | Restore wrapper and color for the stored agent/profile pair |
| `cc-zellij-plugin/src/controller/render_broadcast.rs` | Qualified legend key and label |
| README and docs listed below | Explain schema, CLI, and one-time migration |

## Review Focus

- Empty `profiles` and missing selectors produce the existing no-profile behavior or a clear error, without a panic.
- Malformed `harness/name` selectors, extra slashes, unsupported harnesses, and invalid names fail before mutation.
- Repeated YAML harness keys and repeated short-name keys fail rather than silently replacing data.
- Two same-named profiles have distinct credential destinations and OpenShell provider names; sync cannot suppress one.
- Missing credentials for one profile do not cause lookup or warning text to refer to the other profile.
- Existing snapshots with agent plus short name still restore correctly after config migration.
- Deleting one of two same-named profiles leaves the other definition, credential files, and selected default untouched; deleting the selected default clears only that reference.
- Local, SSH, OpenShell, Kubernetes, container, and compose paths each retain their supported wrapper and credential behavior with equal short names.
- Migration preflights every source and destination before changing live files, validates a staged config, and retains the backup and old credentials for recovery.

## Implementation Sequence

1. Add the nested config model, strict YAML validation, typed identity, exact pair lookup, and selector resolution. Update config tests first so ambiguity and duplicate behavior are pinned down.
2. Update CLI profile commands and default selection. Add JSON/YAML/table output coverage and clear errors for ambiguous bare names.
3. Update profile resolver, translators, sync/provision, credential paths, and OpenShell provider identity. Keep data/login directories in place. Cover same-name isolation locally and remotely.
4. Update hook fallback and background attribution to use harness context, and update snapshot restore to use agent plus short profile. Update the Rust legend key and label to pair identity.
5. Update all other config consumers, docs, and contract tests. Run focused and full project verification.
6. After code verification, inventory and preflight the current user's config and every credential source/destination. Make a recoverable copy, validate a staged nested config, then perform the explicit one-time migration with no destination overwrites. Inspect only names and paths in logs, validate the live config, and verify existing wrappers. Retain backup and old credential files.

## Project Structure

### Documentation (this feature)

```text
specs/091-harness-scoped-profile-names/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/profile-identity.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
cc-deck/internal/config/       # nested model, selectors, validation
cc-deck/internal/cmd/          # profile CLI and hook/background attribution
cc-deck/internal/profile/      # render, sync, provision, credential paths
cc-deck/internal/credential/   # credential resolution and transport identity
cc-deck/internal/session/      # snapshot restore lookup
cc-deck/internal/ws/           # provider names, K8s/git/default lookup
cc-zellij-plugin/src/         # sidebar legend and session display
README.md
docs/modules/reference/pages/cli.adoc
docs/modules/reference/pages/configuration.adoc
docs/modules/using/pages/harness-profiles.adoc
```

**Structure Decision**: Extend existing packages and documentation. No new service or storage system is needed.
