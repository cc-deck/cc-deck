# Brainstorm: OpenShell Credential Masking for Harness Profiles

**Date:** 2026-09-10
**Status:** active
**Issue:** https://github.com/cc-deck/cc-deck/issues/38

## Problem Framing

Harness profiles (brainstorm 093, spec 087, [PR #37](https://github.com/cc-deck/cc-deck/pull/37)) render one wrapper command per profile that exports `CC_DECK_PROFILE`, points the harness at an isolated config directory, and exports the harness credential from a profile-specific variable (`export ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY_WORK"`). On local and SSH workspaces the variable holds the real key. OpenShell works differently, and the integration shipped in PR #37 does not account for it.

OpenShell (`main`, commit `16ed9093`) masks provider credentials unconditionally. The sandbox environment holds placeholders such as `openshell:resolve:env:v7_ANTHROPIC_API_KEY`; the egress proxy rewrites `x-api-key`, `Authorization: Bearer`, Basic auth, URL paths and query parameters, substituting the real secret keyed by the placeholder value and scoped by endpoint binding (host, port, path). There is no passthrough mode. Google Cloud credentials go through a GCE metadata emulator on loopback that hands the SDK a placeholder token; no service account JSON is ever placed in the sandbox. Two credentials of the same type coexist only through distinct placeholders and endpoint bindings; if two providers claim the same environment variable name, the last installed one wins.

Four defects follow from this for cc-deck on OpenShell:

1. A profile with `auth.api_key: {env: ANTHROPIC_API_KEY_WORK}` creates a provider whose credential key is `ANTHROPIC_API_KEY_WORK`. The built-in `anthropic` and `claude-code` provider profiles only know the slot that maps to `ANTHROPIC_API_KEY`, and nothing exports `ANTHROPIC_API_KEY_WORK` into the sandbox. `claude-work` exits with "credential ANTHROPIC_API_KEY_WORK is not available".
2. Two profiles of the same harness and backend produce two providers of type `claude` that both claim `ANTHROPIC_API_KEY`; the sandbox sees one of them. The headline use case (work and private side by side) does not hold on OpenShell.
3. A Vertex profile with `auth.credentials: {file: ...}` uploads the real ADC JSON into the sandbox and the wrapper exports `GOOGLE_APPLICATION_CREDENTIALS`, bypassing the metadata emulator and contradicting spec 073 ("the sandbox process never holds real GCP credentials").
4. Pre-existing since spec 058: `credential.InjectOpenShell` appends `export ANTHROPIC_API_KEY="<real key>"` to `/sandbox/.bashrc` and `.zshrc`. Every interactive shell overrides the placeholder with the real key and sends it to the proxy unmasked.

Subscription logins are a separate constraint. Released OpenShell has no masked path for Claude Pro/Max OAuth: the `claude-code` profile is API key only and its endpoint list excludes the OAuth hosts. The proxy forwards a real bearer token that carries no placeholder marker unchanged, so an in-sandbox `claude login` works if the policy allows the OAuth hosts, but the token is unmasked by design. Upstream is building gateway-owned subscription OAuth: [PR #2285](https://github.com/NVIDIA/OpenShell/pull/2285) adds an `anthropic-oauth` provider type created from the host login (Keychain or `.credentials.json`), with gateway-side refresh and an `ANTHROPIC_AUTH_TOKEN` placeholder in the sandbox; tracking issue [#1925](https://github.com/NVIDIA/OpenShell/issues/1925); Codex and Grok follow the same pattern ([#2740](https://github.com/NVIDIA/OpenShell/issues/2740), [#2742](https://github.com/NVIDIA/OpenShell/issues/2742)). The documented workarounds ([#620](https://github.com/NVIDIA/OpenShell/issues/620)) are an in-sandbox login or uploading `.credentials.json`.

## Approaches Considered

### A: One OpenShell provider profile per cc-deck profile (Chosen)

For every cc-deck profile that applies to an OpenShell workspace, cc-deck imports an ephemeral OpenShell provider profile derived from the built-in one for the backend (`anthropic`, `openai`), identical except that the credential slot's `env_vars` carries the cc-deck profile's own variable name (`ANTHROPIC_API_KEY_WORK`). It then creates the provider from that profile with the standard slot. The gateway exports `ANTHROPIC_API_KEY_WORK=<placeholder>` into the sandbox and binds the placeholder to the backend endpoint, so the wrapper works byte-identically and two profiles of one backend coexist because their variable names differ. Vertex profiles create one `google-cloud` provider per profile from project and region; no file is uploaded. Provisioning writes an optional per-profile environment file (`~/.config/cc-deck/profiles/<name>/env`) that the wrapper sources before its credential checks; on OpenShell it carries the Vertex project and region and a flag marking credentials as backend-managed, so the wrapper skips its file check. `InjectOpenShell` stops exporting any variable that a provider already covers. Spec 085 already imports ephemeral profiles, so the mechanism exists.

- Pros: uses first-class gateway features only; nothing to read back from the gateway; distinct placeholders per profile fall out of the provider model; the wrapper changes by one optional line; imported profiles with custom variable names are documented ("an imported custom profile discovers `CUSTOM_API_TOKEN`").
- Cons: one imported profile per cc-deck profile to name and clean up; two Vertex profiles with different service accounts in one sandbox are unproven (the metadata emulator serves a single default service account); relies on the gateway honoring `x-api-key` injection for imported profiles, which needs a live check.

### B: Read back placeholders and write them into a per-profile environment file

Create providers from the built-in profiles with the standard slot, read back each provider's stable placeholder after creation, and write `ANTHROPIC_API_KEY_WORK=<placeholder>` into the per-profile environment file in the sandbox.

- Pros: no profile imports.
- Cons: requires the SDK to expose credential handles or placeholders to clients, which the proto marks gateway-internal; cc-deck would hold a second copy of the placeholder mapping; still needs the environment file mechanism from A.

### C: Switch providers per session

One provider attached at a time; the wrapper asks cc-deck to attach its provider before executing the harness.

- Cons: sessions run concurrently, so the second launch steals the first session's credential; a gateway round trip on every launch. Rejected.

### D: Route through `inference.local`

- Cons: one provider and one model per gateway, so profiles cannot be distinguished; upstream lists multi-provider routing as not yet wired. Rejected for now; worth revisiting as a simplification once upstream wires it.

### Login profiles on OpenShell

- In-sandbox login now (add the OAuth hosts to the policy, user runs `claude login` once): works, token unmasked. Not chosen.
- Disallow entirely. Not chosen.
- Gateway-owned OAuth when the gateway offers it, otherwise skip with a message. Chosen.

## Decision

Approach A, including the fix for the pre-existing `InjectOpenShell` bypass. Design decisions:

1. **Scope includes the single-workspace path.** Masking only holds if no code path writes real secrets into the sandbox. `InjectOpenShell` exports only variables no provider covers (companion settings such as `CLAUDE_CODE_USE_VERTEX`, model overrides).
2. **One OpenShell provider profile and one provider per cc-deck profile**, derived from the built-in profile for the backend, with the credential variable renamed to the cc-deck profile's source variable. Deterministic names (`cc-deck-<ws>-<profile>` for both), cleaned up with the workspace.
3. **Vertex profiles use the `google-cloud` provider on OpenShell.** The ADC file is neither uploaded nor referenced; the same profile still uploads the file on SSH, where there is no emulator. `GOOGLE_APPLICATION_CREDENTIALS` is never exported in the sandbox.
4. **The wrapper sources an optional per-profile environment file** before its credential checks and honors a backend-managed flag that skips file checks. The wrapper stays byte-identical across backends; only the presence and content of the file differ.
5. **Login profiles are gated on gateway capability.** When the gateway offers the `anthropic-oauth` provider type, cc-deck maps `auth.login` to it (provider created from the host login, gateway holds and refreshes the token, sandbox receives the placeholder). Otherwise `sync` and `ws new` skip login profiles for OpenShell with a message pointing to upstream issue #1925. No unmasked OAuth token ever enters the sandbox through cc-deck.
6. **Provider lifecycle follows the workspace.** Profile providers and imported profiles are created at `ws new` and `sync --workspace`, and removed at `ws delete`.

## Key Requirements

- On OpenShell, every profile whose backend is `anthropic` or `openai` results in an imported provider profile and a provider whose placeholder is exported into the sandbox under the profile's own variable name; the wrapper works unchanged.
- Two profiles of the same harness and backend run side by side in one sandbox with their own credentials.
- A Vertex profile on OpenShell results in a `google-cloud` provider; no credential file enters the sandbox and `GOOGLE_APPLICATION_CREDENTIALS` is not set.
- `InjectOpenShell` never exports a variable that a provider covers.
- The wrapper template gains one optional line sourcing `~/.config/cc-deck/profiles/<name>/env` and skips file checks when the file marks credentials as backend-managed.
- Login profiles map to the `anthropic-oauth` provider when the gateway has it and are skipped with an actionable message otherwise.
- Imported profiles and providers carry deterministic names and are removed with the workspace.
- Scanning the sandbox after provisioning for any configured secret value finds nothing (extends SC-006 of spec 087 to OpenShell).
- Documentation: the harness profiles guide gets an OpenShell section (masking, what the sandbox sees, login gating); the troubleshooting notes cover "credential not available" on OpenShell.

## Open Questions

- Does the gateway accept an imported profile whose credential slot renames `env_vars` while keeping `auth_style: header` and `header_name: x-api-key`, and does the proxy inject for it exactly as for the built-in `anthropic` profile? Needs a live check before the spec is finalized.
- How does the SDK expose provider profile import and deletion (`ProfileInterface.Import`, `Delete`), and are imported profiles per gateway or per workspace?
- Two Vertex profiles with different service accounts in one sandbox: does the metadata emulator support more than the default service account, or is one Vertex profile per sandbox the limit?
- Capability detection for `anthropic-oauth`: query the gateway's profile list, or attempt creation and fall back?
- Should `sync --workspace` re-import a changed profile (model or env change does not affect the provider; a changed source variable does)?
- Where does upstream's Codex subscription work land relative to cc-deck's `codex` profiles, and should `codex.yaml`'s raw OAuth variables be treated as a login profile or as env-sourced credentials?
- Is the per-profile environment file also the right place for the `CC_DECK_PROFILE_COLOR` and `CC_DECK_PROFILE_ICON` idea from the inbox (`hook-profile-env-cache`)?
