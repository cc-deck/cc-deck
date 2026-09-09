# Brainstorm: Harness Profiles

**Date:** 2026-09-09
**Status:** active
**Issue:** https://github.com/cc-deck/cc-deck/issues/36

## Problem Framing

One developer often has more than one way to reach the same model family. The concrete case: company access to Anthropic models through Vertex AI for work projects, and a personal subscription for private projects. The two paths differ in credentials, in login method (service account versus OAuth), and in which models are available (the company path cannot use the newest models). The same split is about to appear for Codex. On top of that, a developer may want to run a cheaper model for simple tasks next to a frontier model for hard ones.

Today cc-deck has no way to express this. The existing `Profile` in `config.yaml` is a Kubernetes deployment concern (Secret names, Vertex project and region) and applies to a whole deployed session. The agent adapters (Claude Code, OpenCode, Codex) detect credentials from the host environment, so every session in a workspace sees the same key and the same model. Users fall back to shell aliases and hand-maintained config directories, which do not travel into SSH or OpenShell workspaces and which the snapshot feature cannot recreate.

The requirement is a named profile that describes how a single agent session authenticates and which model it uses, so that several sessions with different profiles can run side by side inside one workspace, on any backend, and so that a saved workspace can be restored with each pane on the profile it was started with.

Key insight from the discussion: the axis of variation is the harness, not the backend. Claude Code already exposes the knobs (`CLAUDE_CONFIG_DIR`, `ANTHROPIC_MODEL`, `CLAUDE_CODE_USE_VERTEX` with project and region). Codex has `CODEX_HOME` and native `[profiles.<name>]` in `config.toml`. The backend only matters for delivering a small script and its credential material into the workspace, and cc-deck already has that path.

## Approaches Considered

### A: Unified profile plus generated wrappers (Chosen)

Extend `config.Profile` with `harness` (default `claude`, which keeps existing Kubernetes profiles valid), an `auth` block with source variants, `model`, a free-form `settings` map for harness-specific extras, and `color` plus optional `icon` for the sidebar. A small per-harness translator turns a profile into a wrapper script (`claude-work`, `claude-private`, `codex-team`) placed in a cc-deck-managed bin directory on `PATH` inside the workspace. The wrapper exports `CC_DECK_PROFILE=<name>`, sets the harness environment, points the harness at a per-profile config directory, and execs the real binary. Workspace provisioning delivers the script and any file or env credentials through the existing credential transport. The plain `claude` binary stays untouched and means "no profile".

- Pros: one concept; backend-agnostic by construction; the wrapper name is the profile, so snapshot restore needs nothing new; matches what users already do by hand with aliases; sidebar and hooks get the profile name for free from the environment.
- Cons: touches the existing `Profile` struct and the Kubernetes path; each new harness needs a translator; subscription login still requires a one-time login per profile directory per workspace.

### B: Profile as a synced harness config directory

A profile is a pointer to a local directory (for example `~/.claude-work`) holding auth, settings and model config. cc-deck syncs the whole directory into the workspace and generates a wrapper that only sets the config directory variable.

- Pros: minimal translator logic; harness config stays native.
- Cons: ships more files and secrets than needed; model and Vertex settings are buried in harness files instead of declared; Kubernetes profiles cannot be expressed this way, so two profile concepts remain.

### C: Harness-native profiles, no wrappers

Write profiles into each harness's own mechanism: Codex `[profiles.x]` with `--profile`, Claude Code through `--settings` files.

- Pros: nothing added to `PATH`; uses first-party features.
- Cons: Claude Code has no profile concept; separate auth needs a separate config directory anyway; snapshot must store the profile flag separately; the user has to remember a different invocation per harness.

### Sidebar visualization options

- **Background tint per profile: rejected.** The row background already carries the navigation signal (active row in cyan, cursor row in amber). A tint would fight with it and become unreadable under the highlight, and it cannot survive the status fade or the dim styling for paused sessions.
- **Profile color on the harness glyph: chosen.** Costs zero columns, reuses an indicator users already read. The glyph shape still identifies the harness, the color identifies the profile.
- **Line-2 badge: considered.** Would work through the existing badge machinery, but line 2 is shared with the branch name and the glyph approach is cheaper.

## Decision

Approach A, unified profile plus generated wrappers, with the colored-glyph visualization.

Design decisions made during brainstorming:

1. **Profiles are agent-specific.** A profile names its harness. `claude-vertex-work` is inherently a Claude Code profile; `codex-team` is inherently a Codex profile.
2. **Profiles are declarative only.** They state what (harness, auth source, model, extra settings). The per-harness translator decides how. There are no per-backend override sections and no inheritance.
3. **Storage is the existing `profiles` map in `config.yaml`.** `harness` defaults to `claude` so existing Kubernetes profiles remain valid. The Kubernetes Secret fields become one of the credential source variants.
4. **Credential sources are variants per field.** `api_key` accepts `{env: NAME}`, `{file: path}` or `{secret: k8s-name}`. `auth: login` means the harness's own OAuth login, stored in a per-profile config directory that the user logs into once per workspace.
5. **Launch is by wrapper name only.** No `cc-deck run --profile` command. The wrapper is the entry point for the user, for the sidebar and for snapshot restore.
6. **The wrapper exports `CC_DECK_PROFILE`.** The SessionStart hook reports it, so the sidebar and the session registry know the profile without parsing the command line.
7. **Snapshot stores the wrapper command verbatim.** A restored pane runs `claude-work --resume <id>`. No profile parameters are added to the snapshot format.
8. **Sidebar shows profile as glyph color.** The show rule generalizes from "harnesses differ" to "(harness, profile) pairs differ". Sessions without a profile keep the brand color, so the current look is unchanged. A profile may declare `color` and optionally `icon`; otherwise a deterministic palette color is derived from the name. The help overlay lists a legend of glyph, color and profile name.

## Key Requirements

- A profile declares `harness`, an `auth` block with source variants, an optional `model`, an optional `settings` map, and optional `color` and `icon`.
- Existing `config.yaml` profiles without a `harness` field continue to work for the Kubernetes deploy path.
- Each supported harness (Claude Code, Codex, OpenCode) has a translator that renders a profile into a wrapper script; adding a harness means adding a translator, nothing else.
- Wrapper scripts are generated into a cc-deck-managed bin directory that is on `PATH` inside the workspace, for local, SSH and OpenShell backends alike.
- Wrapper scripts never embed secrets. Credential material is delivered separately through the existing credential transport and referenced by the wrapper.
- Per-profile harness config directories receive the cc-deck hooks, since a separate `CLAUDE_CONFIG_DIR` or `CODEX_HOME` means a separate settings file.
- The wrapper exports `CC_DECK_PROFILE`; the hook payload carries the profile name and its color to the plugin.
- The sidebar colors the harness glyph by profile and shows glyphs whenever more than one (harness, profile) pair is running; the help overlay shows a legend.
- Snapshot save records the pane command as launched (wrapper name included); restore replays it unchanged.
- `cc-deck config profile` subcommands (add, list, delete) cover the new fields; the CLI reference and configuration reference document them.

## Open Questions

- How does a subscription (OAuth) login travel to a remote workspace when the host is macOS and the token lives in Keychain rather than in a file? Is a one-time in-workspace login per profile acceptable for v1?
- Should the Codex translator write native `[profiles.<name>]` entries into `config.toml` under a shared `CODEX_HOME`, or use a separate `CODEX_HOME` per profile? Separate auth per profile likely forces the latter.
- Where does the wrapper bin directory live per backend, and how is it added to `PATH` (shell rc snippet, image layer, OpenShell policy)?
- Does the current snapshot format (spec 015) store the pane command verbatim, or does it reconstruct it from the harness name? The spec phase must verify this.
- Should the existing `default_profile` in `config.yaml` influence the plain `claude` binary, or does plain `claude` always mean "no profile"? Keeping them separate avoids surprises but leaves `default_profile` as a Kubernetes-only setting.
- For OpenShell, a Vertex profile needs Vertex network endpoints and an Anthropic profile needs Anthropic ones. How does the per-session profile feed the workspace-level provider list from spec 085 (profile delegation)? Likely the union of all profiles the workspace may run.
- Does a profile need a lightweight `credentials check` to tell the user before launch that a referenced env var or file is missing on the host?
