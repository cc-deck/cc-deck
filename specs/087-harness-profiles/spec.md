# Feature Specification: Harness Profiles

**Feature Branch**: `087-harness-profiles`

**Created**: 2026-09-09

**Status**: Draft

**Input**: Brainstorm 093 - Harness Profiles (`brainstorm/093-harness-profiles.md`)

## Purpose

A developer often has more than one account for the same agent harness (for example company Vertex access and a personal subscription for Claude Code), with different credentials, login methods and model availability. cc-deck has no way to run sessions under different accounts side by side in one workspace, and shell aliases neither travel to remote workspaces nor survive snapshot restore. This feature introduces named, agent-specific profiles in `config.yaml` and generates one wrapper command per profile (`claude-work`, `codex-team`) inside every workspace, so that each session picks its account and model by the command it was started with, the sidebar shows which profile each session runs under, and a restored snapshot brings every session back under the same profile.

## Clarifications

### Session 2026-09-09

- Q: Should cc-deck transport an existing subscription (OAuth) login from the host to remote workspaces, or is a one-time in-workspace login per profile acceptable? → A: One-time in-workspace login per profile per workspace. cc-deck does not transport OAuth tokens.
- Q: Does a per-profile harness config directory start empty, or does it share the user's existing non-auth configuration? → A: It shares non-auth configuration (settings, skills, commands, plugins, MCP config) with the default directory and isolates only login and credential state. The harness translator owns the per-harness list of what is shared and what is isolated.
- Q: What are the semantics of the free-form `settings` map? → A: Renamed to `env`: a map of additional environment variables the wrapper exports before executing the harness.
- Q: How do the wrappers become reachable on `PATH`? → A: They live in a cc-deck-managed bin directory that cc-deck adds to `PATH` through a managed shell rc block. `cc-deck config profile sync` reports when a new shell is required.
- Q: How does a per-session profile influence the network access of an OpenShell workspace? → A: The workspace's provider list is the union of the endpoints required by all valid profiles whose harness is in the workspace's agent list, resolved at workspace creation. Adding a profile with a new auth backend requires re-creating the workspace.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch a Session Under a Named Profile (Priority: P1)

A developer has two ways to reach Anthropic models: company access through Vertex AI for work projects, and a personal subscription for private projects. They define two profiles in `~/.config/cc-deck/config.yaml`, `work` (harness Claude Code, Vertex auth, a model the company allows) and `private` (harness Claude Code, subscription login, the newest model). After running the profile sync, two wrapper commands exist on `PATH` inside their local workspace: `claude-work` and `claude-private`. Typing `claude-work` in a pane starts Claude Code with the Vertex credentials and the company model; typing `claude-private` starts it with the subscription login and the frontier model. The plain `claude` command behaves exactly as before.

**Why this priority**: This is the core value. Without a working wrapper for one profile on the local backend, nothing else in the feature matters.

**Independent Test**: Define one profile with `harness: claude`, an `api_key` sourced from an environment variable, and a `model`. Run the profile sync. Start `claude-<name>` in a terminal and verify the session reports the configured model, the session appears in the sidebar, and the hooks fire (the session transitions through Init, Working, Idle).

**Acceptance Scenarios**:

1. **Given** a profile `work` with `harness: claude`, `auth: {api_key: {env: ANTHROPIC_API_KEY_WORK}}` and `model: <model-id>`, **When** the user runs `claude-work`, **Then** Claude Code starts, uses the key from `ANTHROPIC_API_KEY_WORK`, uses the configured model, and the process environment contains `CC_DECK_PROFILE=work`.
2. **Given** a profile whose harness is `codex`, **When** the user runs `codex-<name>`, **Then** Codex starts with the profile's credential and model, and `CC_DECK_PROFILE` is set.
2a. **Given** a profile whose harness is `opencode`, **When** the user runs `opencode-<name>`, **Then** OpenCode starts with the profile's credential and model, and `CC_DECK_PROFILE` is set.
3. **Given** profiles are defined, **When** the user runs plain `claude`, **Then** the session starts exactly as it did before this feature, with no profile and no `CC_DECK_PROFILE` variable.
4. **Given** a profile references an environment variable that is not set on the host, **When** the user runs the profile sync, **Then** the sync reports which profile and which variable is missing, still generates the wrapper, and the wrapper prints a clear error naming the missing credential when launched.
5. **Given** the user edits a profile in `config.yaml`, **When** the profile sync runs again, **Then** the wrapper reflects the new settings and no duplicate wrappers or stale files remain.

---

### User Story 2 - Distinguish Profiles in the Sidebar (Priority: P1)

The developer runs `claude-work` in one pane and `claude-private` in another pane of the same workspace. The sidebar shows both sessions. Because two different profiles are running, the harness glyph appears on both entries, each colored with its profile's color. When only sessions of one profile (or no profile) are running, the sidebar looks as it does today. Pressing `?` shows a legend that maps each glyph and color to a profile name.

**Why this priority**: Mixing profiles inside one workspace is the whole point. Without a visual cue the user cannot tell which pane talks to which account.

**Independent Test**: Start two sessions with different profiles. Verify the sidebar shows the harness glyph on both with different colors. Stop one. Verify the glyph disappears when only one (harness, profile) pair remains. Open the help overlay and verify the legend lists both profiles.

**Acceptance Scenarios**:

1. **Given** sessions with profiles `work` and `private` are running, **When** the sidebar renders, **Then** each entry shows the harness glyph in the color assigned to its profile.
2. **Given** all running sessions share the same harness and the same profile (or all have no profile), **When** the sidebar renders, **Then** no harness glyph is shown, matching the current behavior.
3. **Given** one Claude Code session without a profile and one with profile `work`, **When** the sidebar renders, **Then** glyphs are shown on both; the unprofiled one keeps the harness brand color.
4. **Given** a profile declares `color: "#RRGGBB"`, **When** its session renders, **Then** the glyph uses that color. **Given** a profile declares no color, **Then** a palette color is derived from the profile name and stays stable across restarts.
5. **Given** a profile declares `icon`, **When** its session renders, **Then** that icon replaces the harness glyph for that session.
6. **Given** profiled sessions are running, **When** the user opens the help overlay, **Then** a legend lists each running profile with its glyph and color.

---

### User Story 3 - Profiles Travel to SSH and OpenShell Workspaces (Priority: P2)

The developer creates an SSH workspace and an OpenShell workspace. The same `claude-work` and `claude-private` commands are available inside each of them, and they behave as they do locally. Credential material reaches the workspace through the existing credential transport; the wrapper scripts themselves contain no secrets.

**Why this priority**: The requirement is "any backend". Local first, remote second, but remote is what makes the feature more than a shell alias.

**Independent Test**: Create an SSH workspace, run the profile sync (or start the workspace, which syncs), open a pane and run `claude-work`. Verify the session uses the profile's credential and model and reports to the sidebar. Repeat for an OpenShell workspace.

**Acceptance Scenarios**:

1. **Given** profiles are defined and an SSH workspace is started, **When** the user opens a shell in the workspace, **Then** `claude-<profile>` commands are on `PATH` for every profile whose harness is installed there.
2. **Given** an OpenShell workspace is started, **When** the user opens a shell in the sandbox, **Then** the same wrapper commands are on `PATH` and launching one uses the profile's credential and model.
3. **Given** a profile with `api_key: {file: <path>}`, **When** the workspace is provisioned, **Then** the file content reaches the workspace through the credential transport and the wrapper script on the remote side contains no credential value.
4. **Given** a profile with `auth: login` (subscription), **When** the user launches the wrapper in a remote workspace for the first time, **Then** the harness prompts for its own login inside that workspace, and subsequent launches reuse that login without prompting again.
5. **Given** a profile whose harness is not installed in a workspace, **When** the workspace is provisioned, **Then** no wrapper is generated for it and the sync output says why.

---

### User Story 4 - Snapshot and Restore Preserve Profiles (Priority: P2)

The developer saves a snapshot of a workspace running `claude-work` in one tab and `claude-private` in another. Later they restore it. Each tab comes back with a session launched through the same wrapper it was started with, resuming the same conversation.

**Why this priority**: Without this, restore silently downgrades every session to the unprofiled default, which for the company account means the wrong credential and possibly a model the account cannot use.

**Independent Test**: Start two sessions under different profiles, save a snapshot, close the workspace, restore the snapshot. Verify each restored pane runs the matching wrapper (visible through `CC_DECK_PROFILE` and the sidebar color) and resumes its conversation.

**Acceptance Scenarios**:

1. **Given** a session was started with `claude-work`, **When** a snapshot is saved, **Then** the snapshot records that the session ran under profile `work` with harness `claude`.
2. **Given** a snapshot entry records profile `work`, **When** the snapshot is restored, **Then** the pane is launched with `claude-work --resume <id>`.
3. **Given** a snapshot entry records no profile, **When** it is restored, **Then** the pane is launched with the plain harness command, as today.
4. **Given** a snapshot entry records a profile that no longer exists in `config.yaml`, **When** it is restored, **Then** the restore falls back to the plain harness command for that entry and reports the missing profile.
5. **Given** a snapshot saved before this feature (no profile field), **When** it is restored, **Then** it restores as before.

---

### User Story 5 - Manage Profiles from the CLI (Priority: P3)

The developer adds, lists, shows, and deletes profiles through `cc-deck config profile ...` and triggers wrapper generation with a sync command. Existing profiles written for the Kubernetes deploy path keep working unchanged.

**Why this priority**: Editing YAML by hand is acceptable for early adopters; the CLI is convenience plus validation.

**Independent Test**: Run `cc-deck config profile add` for a Claude and a Codex profile, `cc-deck config profile list`, `cc-deck config profile show <name>`, `cc-deck config profile delete <name>`, and `cc-deck config profile sync`. Verify config changes and wrapper generation. Deploy a Kubernetes session with a pre-existing profile and verify it still works.

**Acceptance Scenarios**:

1. **Given** a `config.yaml` written before this feature, **When** cc-deck loads it, **Then** every existing profile is treated as `harness: claude` and the Kubernetes deploy path uses it as before.
2. **Given** the user runs `cc-deck config profile add work`, **When** they answer the prompts (harness, auth source, model), **Then** a valid profile is written to `config.yaml`.
3. **Given** a profile with an unknown harness or an incomplete auth block, **When** cc-deck loads the config, **Then** it reports a validation error naming the profile and the field.
4. **Given** the user runs `cc-deck config profile sync`, **When** it completes, **Then** the local workspace bin directory contains exactly one wrapper per valid profile whose harness is installed, and stale wrappers for deleted profiles are removed.
5. **Given** the user runs `cc-deck config profile delete work`, **When** a session under `work` is running, **Then** the profile is removed from config, the wrapper is removed at the next sync, and the running session is unaffected.

---

### Edge Cases

- What happens when two profiles produce the same wrapper name (for example profile `work` for harness `claude` and a second profile also named `work` but for `codex`)? Profile names are unique keys in `config.yaml`; the wrapper name is `<harness-command>-<profile-name>`, so the two would be `claude-work` and `codex-work`. A profile name that collides with an existing command on `PATH` (for example a profile named `code` producing `claude-code`) is rejected at validation time.
- What happens when the harness is not installed on the host at sync time? No wrapper is generated and the sync output names the profile and the missing harness.
- What happens when a credential source resolves to an empty value? The sync warns; the wrapper is still generated and fails at launch with a message naming the profile and the missing credential, instead of starting the harness with a wrong or empty key.
- What happens when a profile changes color while sessions are running? The next hook event from that session carries the new color; the sidebar updates on the next render.
- What happens when the per-profile harness config directory lacks the cc-deck hooks (for example the user created it by hand)? The sync installs or updates the hooks in every per-profile config directory, the same way `cc-deck config plugin install` does for the default directory.
- What happens when two profiles have the same auto-derived palette color? The palette is chosen from a fixed set of distinguishable hues indexed by profile name; collisions are possible beyond the palette size. The legend still shows names, and the user can set `color` explicitly.
- What happens on a workspace backend that does not support wrappers (Kubernetes deploy, compose)? Those backends keep their current behavior; profiles there continue to apply at the workspace level. Wrapper generation targets local, SSH and OpenShell in this feature.
- What happens when `CC_DECK_PROFILE` is set in the shell environment before running plain `claude`? The hook reports whatever `CC_DECK_PROFILE` says. This is by design: the environment variable is the contract.

## Requirements *(mandatory)*

### Functional Requirements

**Profile definition**

- **FR-001**: A profile in `config.yaml` MUST accept a `harness` field naming a supported agent (`claude`, `codex`, `opencode`). When absent, `harness` defaults to `claude`, so profiles written before this feature remain valid.
- **FR-002**: A profile MUST accept an `auth` block. The block MUST support an `api_key` credential with exactly one source: `{env: <name>}`, `{file: <path>}`, or `{secret: <k8s-secret-name>}`. The block MUST also support `login: true`, meaning the harness's own interactive login stored in a per-profile config directory.
- **FR-003**: A profile MUST accept the harness-agnostic fields `model` (optional), `env` (optional map of additional environment variables the wrapper exports before executing the harness), `color` (optional, `#RRGGBB`) and `icon` (optional, a single display glyph).
- **FR-004**: The existing Vertex fields (`project`, `region`, `credentials_secret`) and git credential fields MUST remain supported and MUST be expressible as sources under the `auth` block for the Claude harness.
- **FR-005**: cc-deck MUST validate profiles at load time: known harness, at most one `api_key` source, `login` and `api_key` mutually exclusive, valid `color` syntax, `icon` limited to a single grapheme cluster with a terminal display width of 1 or 2, and profile names limited to lowercase letters, digits and hyphens. Validation errors MUST name the profile and the offending field.

**Wrapper generation**

- **FR-006**: For every valid profile whose harness is installed in a target workspace, cc-deck MUST generate a wrapper command named `<harness-command>-<profile-name>` in a cc-deck-managed bin directory inside that workspace. cc-deck MUST add that directory to `PATH` through a managed shell rc block (written at sync or provisioning time for local and SSH workspaces, and baked into images built by `cc-deck build` for container and OpenShell workspaces), and `cc-deck config profile sync` MUST tell the user when a new shell is required for the change to take effect.
- **FR-007**: Each wrapper MUST export `CC_DECK_PROFILE=<profile-name>`, set the harness-specific environment for model and auth, point the harness at a per-profile config directory, and execute the real harness binary with all user-supplied arguments passed through unchanged.
- **FR-008**: Wrappers MUST NOT contain credential values. Credential material MUST reach the workspace through the existing credential transport and be referenced by the wrapper (by environment variable name or file path).
- **FR-009**: cc-deck MUST install or update its hooks in every per-profile harness config directory it creates, so sessions launched through a wrapper report to the sidebar exactly like unprofiled sessions.
- **FR-009a**: A per-profile harness config directory MUST share the user's non-auth configuration (settings, skills, commands, plugins, MCP configuration) with the harness's default config directory, and MUST isolate only login and credential state. The harness translator defines, per harness, which entries are shared and which are isolated. A change to shared configuration in the default directory MUST be visible to profiled sessions without a re-sync. The plan MUST choose the sharing mechanism and verify, per harness, that it works with that harness's configuration loading and file-watching behavior.
- **FR-010**: Wrapper generation MUST be idempotent: rerunning it updates changed wrappers, leaves unchanged ones alone, and removes wrappers for profiles that no longer exist.
- **FR-011**: A `cc-deck config profile sync` command MUST generate wrappers for the local workspace on demand. Workspace start for SSH and OpenShell MUST perform the same generation as part of provisioning.
- **FR-012**: The plain harness command (`claude`, `codex`, `opencode`) MUST remain untouched and MUST mean "no profile".
- **FR-013**: When a wrapper is launched and a referenced credential is missing or empty, the wrapper MUST exit with a message naming the profile and the missing credential instead of starting the harness.

**Session identity**

- **FR-014**: The hook payload sent to the plugin MUST carry the profile name and the profile's display color (declared or derived) whenever `CC_DECK_PROFILE` is present in the session's environment.
- **FR-015**: The session registry used by the sidebar MUST record the profile name per session.

**Sidebar**

- **FR-016**: The sidebar MUST show the harness glyph on every entry whenever the set of running sessions contains more than one distinct (harness, profile) pair, where "no profile" counts as a distinct value. Otherwise it MUST hide the glyph, matching current behavior.
- **FR-017**: The glyph color MUST be the profile's declared color, or a deterministic palette color derived from the profile name when none is declared. Sessions without a profile MUST keep the harness brand color.
- **FR-018**: When a profile declares `icon`, the sidebar MUST render that icon in place of the harness glyph for that profile's sessions.
- **FR-019**: The help overlay MUST include a legend listing every running profile with its glyph and color, shown only when at least one profiled session is running.

**Snapshot and restore**

- **FR-020**: Snapshot save MUST record, per session, the harness and the profile name the session was started with (empty when unprofiled).
- **FR-021**: Snapshot restore MUST launch each session through `<harness-command>-<profile>` when a profile is recorded, and through the plain harness command otherwise.
- **FR-022**: When a recorded profile no longer exists, restore MUST fall back to the plain harness command for that entry and report the missing profile by name.
- **FR-023**: Snapshots written before this feature MUST restore unchanged.

**CLI**

- **FR-024**: `cc-deck config profile add`, `list` and `show` MUST cover the new fields (`list` shows harness and auth kind per profile, `show` prints every field with credential values masked). A `cc-deck config profile delete <name>` command MUST be added. The existing `cc-deck config profile use <name>` keeps its current meaning: it sets `default_profile`, which the Kubernetes deploy path consumes. It MUST NOT influence which wrapper a session runs under and MUST NOT change the behavior of the plain harness command.
- **FR-025**: The Kubernetes deploy path MUST continue to accept profiles as before; a profile with `{secret: ...}` sources is the equivalent of today's secret-name fields.

**OpenShell network access**

- **FR-026**: When an OpenShell workspace is created, cc-deck MUST include in the workspace's provider list the network endpoints required by every valid profile whose harness is in the workspace's agent list (the union across profiles), so that any of those profiles can be launched inside the sandbox. A profile added after workspace creation whose auth backend needs endpoints the workspace does not have MUST be reported at sync time with a hint to re-create the workspace.

### Key Entities

- **Profile**: A named, declarative description of how one agent session authenticates and which model it uses. Attributes: name, harness, auth (source variants), model, env, color, icon, plus the existing Vertex and git credential fields. Stored in the `profiles` map of `config.yaml`.
- **Credential Source**: Where a credential value comes from: host environment variable, host file, Kubernetes Secret, or the harness's own login. Exactly one per credential.
- **Wrapper**: A generated command named `<harness-command>-<profile-name>` living in a workspace bin directory. It is the only launch path for a profile and the identity the snapshot records.
- **Harness Translator**: The per-harness knowledge that turns a profile into wrapper content, per-profile config directory layout (including which entries are shared with the default directory and which are isolated), and hook installation. One translator per supported harness. Its behavioral contract MUST be written down (as a contracts document in the feature directory) before the second translator is implemented, per constitution principle II.
- **Snapshot Session Entry**: Extended with harness and profile name.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from an empty `profiles` section to a running profiled session in under five minutes using `cc-deck config profile add` and `cc-deck config profile sync`.
- **SC-002**: Two sessions with different profiles for the same harness run side by side in one workspace, each using its own credential and model, verified through the sessions' own reported model and the `CC_DECK_PROFILE` value.
- **SC-003**: The same wrapper commands work unchanged on local, SSH and OpenShell workspaces, with zero profile-specific configuration in the workspace definition.
- **SC-004**: A snapshot of a workspace with mixed profiles restores 100% of its sessions under the correct profile.
- **SC-005**: All `config.yaml` files and snapshots created before this feature load and behave exactly as before, with no migration step.
- **SC-006**: No generated wrapper file contains a credential value, verified by scanning generated wrappers for every configured secret value.
- **SC-007**: Adding support for a new harness requires adding one translator and no changes to sidebar, snapshot, or transport code.

## Documentation Requirements

- The configuration reference (`docs/modules/reference/pages/configuration.adoc`) MUST document the extended profile schema: `harness`, `auth` sources, `model`, `env`, `color`, `icon`, and the backward-compatible defaults.
- The CLI reference (`docs/modules/reference/pages/cli.adoc`) MUST document `cc-deck config profile sync`, `cc-deck config profile delete`, and the extended `add` and `show` output.
- A guide page MUST explain the two-account use case end to end (define profiles, sync, launch, sidebar legend, snapshot behavior) including the one-time login step for subscription profiles in remote workspaces.
- `README.md` MUST mention harness profiles as a user-facing capability.
- All documentation MUST use the prose plugin with the `cc-deck` voice profile.

## Assumptions

- Each supported harness exposes a way to select a separate configuration directory (Claude Code: `CLAUDE_CONFIG_DIR`; Codex: `CODEX_HOME`; OpenCode: `OPENCODE_CONFIG`) and a way to select the model through environment or config. The translator relies on these first-party mechanisms.
- Subscription (OAuth) login is bound to the harness config directory. For remote workspaces the user performs the harness login once per profile per workspace; cc-deck does not transport OAuth tokens (file-based on Linux, Keychain-based on macOS). The guide documents this one-time step.
- The per-profile config directory for a harness lives under a cc-deck-managed path inside the workspace home. Users do not need to know this path. Its shared entries point at the harness's default config directory, so user customizations made there apply to every profile.
- The cc-deck-managed bin directory for wrappers lives under the cc-deck data directory in the workspace home. cc-deck writes an idempotent, marker-delimited block into `~/.bashrc` and `~/.zshrc` that prepends it to `PATH` (this block is new; the existing tool PATH restoration only exists as an image build step). Images built by `cc-deck build` prepend the directory at build time as well.
- The existing credential transport (spec 079) can carry an arbitrary named environment variable and an arbitrary file; profile credential sources map onto those two primitives.
- For OpenShell workspaces, the endpoint union from FR-026 feeds the provider list from spec 085 (profile delegation). Each auth backend (Anthropic direct, Vertex, OpenAI) maps to a known set of endpoints already used by the credential detection of the agent adapters.
- `default_profile` in `config.yaml` keeps its current meaning for the Kubernetes deploy path and does not affect the plain harness command.
- The palette for auto-derived colors has at least eight distinguishable hues, each with a contrast ratio of at least 3:1 (WCAG AA for graphical objects) against both the plain sidebar background and the active-row highlight background.
- The Kubernetes deploy and compose backends are out of scope for wrapper generation in this feature; they continue to apply a single workspace-level profile.
- Out of scope: per-backend override sections inside a profile, profile inheritance, a `cc-deck run --profile` launch command, and switching a running session's profile.
