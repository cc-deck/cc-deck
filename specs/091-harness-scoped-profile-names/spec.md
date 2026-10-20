# Feature Specification: Harness-Scoped Profile Names

**Feature Branch**: `091-harness-scoped-profile-names`
**Created**: 2026-10-19
**Status**: Draft
**Input**: Brainstorm 098: profile names are unique within a harness, with a strict nested schema and a one-time migration for the current user.

## Purpose

Let a developer use the same meaningful profile name with different agent harnesses while keeping account selection, credentials, and session identity exact. The new configuration has one nested shape. The current installation is migrated once.

## Clarifications

### Session 2026-10-19

- Q: Is migration automatic during normal startup or an explicit one-time task? → A: It is an explicit one-time task for this installation; normal startup only accepts the new schema.
- Q: How is an equal short name distinguished in session reporting? → A: Session identity is the harness and short name together; the existing short name may remain in the wrapper environment, but reporting and lookup must use the harness context as well.
- Q: How should the sidebar legend label two equal short names? → A: Show the qualified harness/name label for every profiled session, consistently.
- Q: How should the human profile list display a profile name? → A: Show the synced wrapper command name (for example, `claude-private`) in the `NAME` column. Keep selectors, defaults, JSON/YAML fields, and sidebar identity in their existing qualified or structured forms.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reuse a name across harnesses (Priority: P1)

A developer defines `private` for Claude Code and Codex, then starts `claude-private` and `codex-private` in the same workspace. Each session uses its own account, model, and profile settings.

**Why this priority**: This is the central request.

**Independent Test**: Configure both profiles, sync a workspace, and launch both wrappers; verify distinct credentials and profiles.

**Acceptance Scenarios**:

1. **Given** `profiles.claude.private` and `profiles.codex.private`, **when** the developer synchronizes a workspace, **then** both wrappers are available and isolated.
2. **Given** one Claude `private`, **when** a Codex `private` is added, **then** both remain valid.
3. **Given** one Claude `private`, **when** a second Claude `private` is defined, **then** validation rejects the duplicate pair rather than replacing a definition.
4. **Given** `claude/private` and `codex/private`, **when** the developer lists profiles in human-readable output, **then** the names are displayed as `claude-private` and `codex-private`, matching the synced wrapper commands.

---

### User Story 2 - Select the intended profile (Priority: P1)

The developer can inspect, use, and delete either `private` profile. A bare name works while unique; an ambiguous name fails and lists its qualified choices. The default profile always points to one exact profile.

**Why this priority**: Ambiguous selection could delete the wrong profile or select the wrong credentials.

**Independent Test**: Exercise add, list, show, use, and delete with one and then two `private` definitions.

**Acceptance Scenarios**:

1. **Given** only Claude `private`, **when** the developer runs `cc-deck config profile show private`, **then** that profile is shown.
2. **Given** both `private` profiles, **when** the developer runs `cc-deck config profile show private`, **then** no profile is selected and `claude/private` and `codex/private` are suggested.
3. **Given** both profiles, **when** the developer runs `cc-deck config profile use codex/private`, **then** the saved default identifies only Codex `private`.
4. **Given** both profiles, **when** Claude `private` is deleted, **then** Codex `private` and its credentials remain intact.

---

### User Story 3 - Migrate the current setup once (Priority: P2)

The current user moves existing definitions and profile data to the nested schema. Account access, login state, defaults, and wrappers keep working. Old flat entries subsequently produce a useful migration error.

**Why this priority**: The user chose a stricter schema and a one-time local migration over permanent legacy parsing.

**Independent Test**: Migrate a copy of the current configuration and data, verify behavior, then migrate the live installation with a recoverable backup.

**Acceptance Scenarios**:

1. **Given** a flat profile and a default reference, **when** they are migrated, **then** settings and the exact default selection are preserved.
2. **Given** existing credential files or login state, **when** their paths change, **then** no destination file is overwritten and the same profile can still use them.
3. **Given** a flat entry after installation, **when** config is loaded, **then** an error identifies the old and expected shapes.

---

### User Story 4 - Preserve identity through the session lifecycle (Priority: P2)

The developer can distinguish same-named sessions in the sidebar and restore a saved workspace without changing harnesses or accounts.

**Why this priority**: Identity must remain correct after launch, remote provisioning, and restore.

**Independent Test**: Launch both wrappers, inspect the sidebar, save and restore a workspace, and verify each pane resumes with its original wrapper.

**Acceptance Scenarios**:

1. **Given** Claude and Codex sessions both named `private`, **when** the sidebar and legend display them, **then** both identities are distinguishable with their assigned color and icon.
2. **Given** a snapshot with both wrappers, **when** it is restored, **then** both original commands and accounts are retained.

### Edge Cases

- Adding another harness's profile makes a formerly unique bare name ambiguous; no command silently changes its selection.
- Unknown harness groups, invalid names, repeated YAML keys, and a redundant inner `harness` field are rejected.
- Credential and profile state for equal short names stay separate during sync and deletion.
- Migrated default selection stays stable after another harness adds the same name.
- Profiles with existing Kubernetes or git credential fields remain usable under a harness group.

### Error Handling

- Configuration errors identify the harness and short name when available, and show the expected nested shape for legacy entries.
- Ambiguous bare selectors name every matching qualified choice and leave config, credentials, and running sessions unchanged.
- Migration stops before changing live files if a destination exists or a source cannot be copied safely; the backup remains available for recovery.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Configuration MUST group profiles by supported harness and then by name. The pair identifies one profile, and profile bodies MUST NOT repeat `harness`.
- **FR-002**: The same short name MUST be allowed under different harnesses. Duplicate pairs, including repeated YAML keys, MUST be rejected rather than silently replaced.
- **FR-003**: Flat legacy entries, unknown harness groups, malformed names, and inconsistent nested values MUST yield actionable validation errors. The old shape MUST NOT be silently accepted.
- **FR-004**: The qualified selector MUST be `<harness>/<name>`. Commands and `default_profile` MUST accept it.
- **FR-005**: A bare selector MUST resolve only when exactly one profile has that name. Ambiguous names MUST fail without selection and list qualified candidates.
- **FR-006**: `cc-deck config profile use` MUST persist a qualified default, even when selected by a unique bare name. Missing or ambiguous defaults MUST be invalid.
- **FR-007**: Profile add, list, show, use, delete, and sync MUST operate on the intended pair. Add MUST obtain the harness from its existing `--harness` option or the interactive harness choice, and MUST reject an existing pair even when the short name exists under another harness. Human-readable list output MUST display the synced wrapper command (`<harness command>-<name>`) in `NAME` for every profile. JSON and YAML list output MUST retain separate short-name and harness fields. Selectors and defaults MUST continue to use `harness/name`.
- **FR-008**: Wrapper commands MUST retain `<harness command>-<name>` and coexist in every workspace type that currently supports profile wrappers.
- **FR-009**: Credential material, login state, and per-profile configuration MUST be isolated by both harness and name. Sync or deletion of one pair MUST NOT alter another.
- **FR-010**: Hook reports, background attribution, sidebar entries, and the legend MUST use harness plus short name as session identity and preserve configured color and icon. The legend MUST show qualified `harness/name` labels for all profiled sessions.
- **FR-011**: Snapshot save and restore MUST preserve each session's harness and profile when names repeat.
- **FR-012**: Local, SSH, OpenShell, Kubernetes, container, and compose paths MUST keep their currently supported profile and credential behavior under the new identity.
- **FR-013**: An explicit one-time migration on this machine MUST transform flat definitions, defaults, and existing credential or login data without losing settings or overwriting user-owned files. A recoverable copy MUST be made first. Normal application startup MUST NOT perform automatic migration.
- **FR-014**: Tests MUST cover the new schema, qualified selectors, ambiguity, isolation, migration, wrappers, and session identity. README, CLI reference, configuration reference, and an Antora guide MUST document the user-facing changes; documentation MUST pass the project's prose checks.

### Key Entities

- **Profile identity**: The harness and short name together; written as `<harness>/<name>` when qualified.
- **Profile definition**: Existing auth, model, color, icon, and legacy deployment or git fields stored under one harness and name.
- **Default profile reference**: A stable qualified pointer to one profile.
- **Profile state**: Credentials and harness data owned by one identity.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Both `claude-private` and `codex-private` can launch in one workspace with their intended accounts in every acceptance run.
- **SC-002**: Every tested ambiguous bare reference fails with both qualified choices; every qualified reference selects exactly one profile.
- **SC-003**: The current user's profiles, default selection, and credential or login state work after migration, with no overwritten destinations.
- **SC-004**: A saved workspace with two same-named profiles restores both under their original harnesses in every supported snapshot test.
- **SC-005**: README and reference documentation explain the new schema and command syntax with a two-harness example.

## Assumptions

- Supported harnesses remain Claude Code, Codex, and OpenCode.
- One-time local migration is acceptable because this installation currently has one user; old flat config need not remain loadable.
- Existing Kubernetes and git credential fields keep their meanings after nesting under a harness.
- Wrapper commands remain the way to start profiled sessions.

## Dependencies and Scope

- This builds on the existing harness profile behavior from feature 087 and its wrapper, credential, hook, sidebar, and snapshot paths.
- The change includes the current user's one-time config and profile-data migration. An automatic loader for old flat config and a general migration command are outside scope.
- Adding new harnesses, changing authentication methods, and choosing profiles automatically by directory are outside scope.
