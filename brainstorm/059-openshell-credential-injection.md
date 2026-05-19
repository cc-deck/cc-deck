# Brainstorm: OpenShell Credential Injection

**Date:** 2026-05-19
**Status:** active

## Problem Framing

OpenShell sandboxes need credentials to function: an Anthropic API key for Claude Code, GitHub tokens for repo access, and potentially Google Vertex service account files for GCP-based inference. Today, cc-deck has no mechanism to bridge its credential system to OpenShell's provider architecture. The OpenShell workspace backend passes `--provider` to `sandbox create` but never creates the providers first, and has no way to inject file-based credentials like Vertex's `GOOGLE_APPLICATION_CREDENTIALS` JSON.

The challenge has three layers:
1. **Discovery**: Which credentials does a workspace need? (capture phase)
2. **Declaration**: How are credential requirements recorded in the manifest? (build.yaml)
3. **Injection**: How do credentials flow from the host into the sandbox at runtime? (ws new)

### OpenShell Provider System

OpenShell manages credentials as "providers," named credential bundles injected into sandboxes at creation. The sandbox process never sees real credential values; an HTTP proxy replaces opaque placeholder tokens with actual credentials before forwarding upstream. This is a strong security property.

Available provider profiles: `anthropic`, `claude`, `github`, `gitlab`, `openai`, `nvidia`, `codex`, `copilot`, `opencode`, plus `generic` for custom services.

**Vertex AI gap**: OpenShell has no Vertex provider today. [Issue #472](https://github.com/NVIDIA/OpenShell/issues/472) proposed a `CredentialRefresher` trait with Vertex as first implementation, closed as duplicate of [Issue #896](https://github.com/NVIDIA/OpenShell/issues/896) (Enhanced Provider Management, in progress 3/5). The proposed approach would use `GOOGLE_SERVICE_ACCOUNT_JSON` as a string credential with gateway-side OAuth2 refresh. Until this ships, Vertex needs a workaround.

## Approaches Considered

### A: Provider-First with Runtime File Upload (Chosen)

Use OpenShell's native provider system for API-key-based credentials (claude, github, etc.) and runtime file upload for file-based credentials (Vertex).

**Capture phase** adds a credential detection step:
- Auto-detect available credentials from host environment (scan for `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GOOGLE_APPLICATION_CREDENTIALS`, `AWS_*`, etc.)
- Present findings to user for confirmation and editing
- Write a `credentials` section to `build.yaml` with provider types and env var names (never values)

**Build phase** generates network policy entries for credential endpoints (e.g., Vertex needs `oauth2.googleapis.com`, `{region}-aiplatform.googleapis.com`).

**Workspace creation** (`ws new --type openshell`):
1. Read credential types from manifest
2. For each API-key provider: `openshell provider create --type <type> --from-existing` (reads from host env)
3. For file-based credentials: `openshell sandbox upload` the file after sandbox starts, set env var
4. Pass `--provider <name>` for each created provider to `sandbox create`

- Pros: Uses OpenShell's proxy-based credential security, auto-wires network policy, manifest-driven reproducibility, handles both API keys and files
- Cons: Vertex workaround until OpenShell adds native support, provider creation adds latency to `ws new`

### B: Auto-Providers Only

Pass `--auto-providers` to `sandbox create` and let OpenShell discover credentials from the host environment.

- Pros: Zero cc-deck code, works today for API key auth
- Cons: No Vertex support, no control over which credentials are injected, no manifest-driven reproducibility, breaks non-interactive mode

### C: Direct Env Injection (bypass providers)

Upload a `credentials.env` file to the sandbox and source it, similar to SSH workspaces.

- Pros: Simple, works for any credential type
- Cons: Loses OpenShell's proxy-based credential security (agent sees real keys), no auto-wired network policy

## Decision

**Approach A: Provider-First with Runtime File Upload.**

This integrates cleanly with OpenShell's security model (credentials never enter the sandbox, proxy handles injection) while providing a practical workaround for Vertex's file-based auth. When OpenShell adds native Vertex support via issue #896, cc-deck can switch from file upload to the vertex provider type without changing the manifest schema.

Scope: OpenShell workspace type only. A separate brainstorm should address unifying credential handling across all workspace types (container, SSH, K8s, compose).

## Key Requirements

1. **Manifest `credentials` section**: Declares provider types and env var names needed by the workspace. Never stores actual values. Example:
   ```yaml
   credentials:
     - type: claude
       env_vars: [ANTHROPIC_API_KEY]
     - type: github
       env_vars: [GITHUB_TOKEN]
     - type: vertex
       file: GOOGLE_APPLICATION_CREDENTIALS
       env_vars: [ANTHROPIC_VERTEX_PROJECT_ID, CLOUD_ML_REGION]
   ```

2. **Capture detection**: Auto-detect credentials from host environment, present to user for confirmation. Map detected env vars to known provider profiles.

3. **Provider creation at ws new**: Before sandbox creation, create OpenShell providers for each entry in `credentials`. Use `--from-existing` for API key types, `--credential KEY=VALUE` for explicit values.

4. **File upload for Vertex**: After sandbox starts, upload the service account JSON file via `openshell sandbox upload`. Set `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to the uploaded path inside the sandbox.

5. **Network policy integration**: Provider endpoints are auto-injected into network policy when `providers_v2_enabled` is set. For Vertex (no native provider), add GCP endpoints to the policy during build.

6. **Credential refresh**: Not in scope for initial implementation. When OpenShell ships credential refresh (issue #896), cc-deck should adopt it. For now, API key credentials are static and Vertex file credentials last as long as the service account key is valid.

## Open Questions

- How should `ws new` handle missing credentials? Error, warn, or prompt? (e.g., manifest says `claude` provider but `ANTHROPIC_API_KEY` is not set)
- Should credential provider creation be idempotent? (If provider `claude` already exists from a previous workspace, reuse or recreate?)
- When OpenShell ships the vertex provider type, how do we migrate existing workspaces? (Likely: just change `type: vertex` handling in the ws new code, manifest stays the same)
- Should the `credentials` section support custom provider types beyond the known profiles? (e.g., `type: generic` with explicit endpoint definitions)
- How does credential injection interact with `ws refresh-creds`? (Currently only SSH workspaces support credential refresh)
