# Tasks: Harness-Scoped Profile Names

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [identity contract](contracts/profile-identity.md). Every task inherits the plan's **Global Constraints** and **Interfaces Between Tasks**.

**Tests**: Required by the feature spec and project constitution. Include focused tests with each behavior change.

**Organization**: Tasks follow the four user stories, then documentation, full verification, and the explicit local migration.

## Phase 1: Setup

**Goal**: Establish current behavior and the migration inventory without reading credential contents into logs.

- [X] T001 Record the current flat profile names, effective harnesses, default selector, and existing credential/data directory names; establish focused baseline tests in `cc-deck/internal/config/profile_test.go` and `cc-deck/internal/profile/sync_test.go`.

## Phase 2: Foundation

**Goal**: Provide one strict identity and selection model for every consumer.

- [X] T002 Add `ProfileKey`, nested profile storage, `ListProfileKeys`, `GetProfileFor`, `ResolveProfileRef`, pair mutation helpers, and strict YAML validation in `cc-deck/internal/config/config.go`, `profile.go`, `validate.go`, `profile_test.go`, and `validate_test.go`; test empty maps, malformed selectors, duplicate harness and name keys, unknown harnesses, redundant inner `harness`, legacy flat entries, and missing or ambiguous defaults.

## Phase 3: User Story 1 - Reuse a name across harnesses (P1) 🎯 MVP

**Goal**: Both same-named harness wrappers can coexist and use separate credentials in local and remote workspaces.

**Independent Test**: Configure Claude and Codex `private`, sync, then verify both wrappers and distinct credential paths.

- [X] T003 [US1] Update profile resolution, translators, wrapper credential references, sync/provision loops, and credential transport to use `ProfileKey` in `cc-deck/internal/profile/translator.go`, `sync.go`, `claude.go`, `codex.go`, `opencode.go`, `cc-deck/internal/credential/profile.go`, and related package tests; assert distinct paths, preserved login/data directories, qualified warnings, and one profile's missing credential not suppressing the other.
- [X] T004 [US1] Include harness in OpenShell provider names and deduplication, and update local/SSH/OpenShell workspace profile consumers in `cc-deck/internal/ws/openshell.go`, `profile_target.go`, `ssh.go`, `local.go`, `openshell_test.go`, and `profile_target_test.go`; assert both same-named providers survive.
- [X] T005 [US1] Add same-name wrapper and credential isolation tests for local sync and SSH/OpenShell provisioning in `cc-deck/internal/profile/sync_test.go` and `provision_test.go`; add coverage for the currently supported container and compose paths in `cc-deck/internal/ws/container_test.go` and `compose_test.go`, and for Kubernetes credential behavior in `cc-deck/internal/ws/k8s_credentials_test.go`.

## Phase 4: User Story 2 - Select the intended profile (P1)

**Goal**: Commands and defaults choose exactly one profile, with errors for ambiguous bare names.

**Independent Test**: With two `private` profiles, bare show/use/delete fail with both choices; qualified commands work and the saved default is qualified.

- [X] T006 [US2] Update add/list/show/use/delete/sync to use pair identity and qualified defaults in `cc-deck/internal/cmd/profile.go` and `profile_test.go`; assert JSON/YAML retain separate harness and short-name fields, the human table name matches the synced wrapper, ambiguous bare show/use/delete leaves state unchanged and lists every candidate, qualified selection is exact, duplicate add rejects only the existing pair, and deleting one pair leaves the other profile and credentials intact.
- [X] T007 [US2] Update default-profile consumers for Kubernetes, git credentials, and config validation in `cc-deck/internal/ws/k8s_deploy.go`, `repos.go`, `cc-deck/internal/config/validate.go`, and their tests; assert adding a second harness with the same short name cannot change the persisted qualified default or its Kubernetes/git credential selection.

## Phase 5: User Story 3 - Migrate the current setup once (P2)

**Goal**: Define and rehearse a safe local migration; execute the live change after all code verification.

**Independent Test**: A copied flat config and credential tree become nested without changed settings, overwritten files, or lost login state.

- [X] T008 [US3] Add a migration fixture test and explicit, reversible migration procedure for the current configuration in `cc-deck/internal/config/profile_test.go` and `specs/091-harness-scoped-profile-names/quickstart.md`; assert settings and default are preserved, the existing data/login path stays valid, duplicate destination or unreadable source aborts before live changes, and normal config load remains strict.

## Phase 6: User Story 4 - Preserve identity through the session lifecycle (P2)

**Goal**: Hooks, sidebar, and snapshot restore retain harness plus short-name identity.

**Independent Test**: Launch, display, save, and restore Claude and Codex `private`; both identities and wrapper commands remain distinct.

- [X] T009 [US4] Use known harness context and `GetProfileFor` for profile lookup in `cc-deck/internal/cmd/hook.go`, `codex_attribution.go`, `claude_bg_attribution.go`, `hook_profile_test.go`, `codex_attribution_test.go`, and `claude_bg_attribution_test.go`; assert same-named sessions retain their own color/icon and missing credentials are attributed correctly.
- [X] T010 [P] [US4] Key and label sidebar legend entries by `harness/name` in `cc-zellij-plugin/src/controller/render_broadcast.rs` and Rust integration tests, keeping existing profile color and icon behavior.
- [X] T011 [US4] Use stored agent plus short profile for snapshot restore and color resolution in `cc-deck/internal/session/restore.go` and `restore_test.go`; assert old snapshots and two same-named sessions resolve the intended wrapper.

## Phase 7: Polish and Migration

**Goal**: Document, verify, and migrate the current installation.

- [X] T012 [P] Update nested-schema and selector examples in `README.md`, `docs/modules/reference/pages/cli.adoc`, `docs/modules/reference/pages/configuration.adoc`, `docs/modules/using/pages/harness-profiles.adoc`, and `docs/quickstart-k8s.md`; run the project's prose check.
- [X] T013 Run focused Go/Rust tests, `make test`, `make lint`, and the [quickstart](quickstart.md); resolve failures and run `git diff --check`.
- [X] T014 Preflight every live credential source and destination, make a recoverable copy of the current user's `~/.config/cc-deck/config.yaml`, stage and validate the nested config, then migrate its profile entries/default and credential files without overwrites; validate the live config, verify the existing wrappers and login state, and record the backup location in `specs/091-harness-scoped-profile-names/VERIFY.md` without credential values.

## Interfaces Consumed by Tasks

- **T003–T005:** T002 supplies `config.ProfileKey{Harness, Name string}`, `ProfileKey.String() string`, and `Config.ListProfileKeys() []ProfileKey`; T003 passes that pair to rendering and credential transport instead of a bare name. T004 consumes the same pair for provider identity.
- **T006–T008:** T002 supplies `Config.ResolveProfileRef(ref string) (ProfileKey, error)`, `Config.AddProfile(key ProfileKey, profile Profile) error`, `Config.DeleteProfile(key ProfileKey) error`, and `Config.SetDefaultProfile(ref string) error`. T007 resolves the persisted default to a key before Kubernetes or git credential lookup.
- **T009–T011:** T002 supplies `Config.GetProfileFor(harness, name string) (Profile, error)` for known-harness runtime lookups. Rust and snapshots retain separate `agent` and `profile` fields; their combined identity is the qualified `harness/name` label.

## Dependencies and Execution Order

- T001 → T002 → T003–T005 (US1) → T006–T007 (US2) → T008 (US3 rehearsal) → T009–T011 (US4) → T012–T014.
- US1 and US2 are P1; US2 relies on the foundation but can be validated independently from remote provisioning.
- The local migration T014 waits for all code and documentation verification so the live config is not changed by incomplete behavior.
- T010 touches Rust and can proceed alongside Go session work after T002. T012 touches documentation and can proceed alongside code work once the schema is fixed.

## Parallel Examples

- **US1**: Wrapper path work in `cc-deck/internal/profile/` and OpenShell provider naming in `cc-deck/internal/ws/` can proceed after the T002 identity contract is stable, with T005 integrating both.
- **US2**: CLI work and Kubernetes/git consumer updates can proceed in separate packages after T002, then be checked together for default selection.
- **US3**: Migration rehearsal is isolated from live user files; T014 is deliberately serial after validation.
- **US4**: Rust legend work (T010) can run alongside Go hook and restore work (T009, T011).

## Implementation Strategy

Complete T001–T005 to deliver the two-wrapper MVP, then make selection safe with T006–T007. Rehearse migration on fixtures, complete session identity, and run full verification. Apply the one-time live migration only after the implementation passes.
