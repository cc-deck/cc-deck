# OpenShell Integration Findings

**Date:** 2026-05-06
**OpenShell Version:** v0.0.36
**Context:** End-to-end integration testing of cc-deck's OpenShell workspace backend with a local Podman-based gateway on macOS (Apple Silicon).

## Summary

This document captures the friction points and workarounds discovered while integrating cc-deck with OpenShell v0.0.36. Each section describes the issue, the workaround used, and where applicable, links to existing GitHub issues or suggests improvements.

## 1. No Machine-Readable CLI Output

**Problem:** The `openshell` CLI outputs human-formatted text with ANSI color codes and decorative formatting. Tools wrapping the CLI must parse colored, multi-line output to extract data like sandbox names or phase status.

Examples:
- `sandbox create` outputs `Created sandbox: <name>` embedded in progress lines with ANSI codes
- `sandbox get` outputs a full formatted policy dump where the phase is buried in the middle
- `sandbox list` outputs a table with ANSI formatting

**Workaround:** We implemented `stripANSI()`, `parseSandboxName()`, and `parseSandboxPhase()` functions to parse the human-readable output. This is fragile since any formatting change will break the parsing.

**Suggestion:** Add a global `-o json` or `--output json` flag (like `kubectl -o json`) to all CLI commands that return structured data. At minimum for: `sandbox create`, `sandbox get`, `sandbox list`, `sandbox exec`.

**Existing issue:** [#1034 - Support --json in `openshell policy get`](https://github.com/NVIDIA/OpenShell/issues/1034) requests JSON output for the policy command. This could be extended to all CLI commands as a consistent `-o json` flag.

## 2. exec --tty Hangs

**Problem:** `openshell sandbox exec -n <name> --tty -- /bin/bash` hangs indefinitely. The command never returns output or produces an interactive session.

**Workaround:** Use `openshell sandbox connect <name>` instead for interactive sessions. This works but doesn't allow specifying a custom command (always runs the default shell).

**Impact on cc-deck:** The attach flow uses `connect` instead of `exec --tty`, which means cc-deck can't run `zellij attach --create` directly. The sandbox image must have Zellij as the default command/entrypoint.

**Existing issues:**
- [#1046 - sandbox exec hangs after receiving complete gRPC response](https://github.com/NVIDIA/OpenShell/issues/1046)
- [#828 - sandbox exec hangs after receiving complete gRPC response](https://github.com/NVIDIA/OpenShell/issues/828)

## 3. No File-Based Credential Injection via Providers

**Problem:** OpenShell's provider system injects credentials as environment variables with opaque placeholder tokens. The proxy intercepts HTTP requests and replaces placeholders with real values. This works for API key auth (bearer tokens) but **not for Google Cloud Vertex AI**, which requires:

1. A JSON credentials file (`application_default_credentials.json`) mounted at a specific path
2. An environment variable (`GOOGLE_APPLICATION_CREDENTIALS`) pointing to that file path
3. Additional config env vars (`CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`) that are **not secrets** but configuration values that must be set verbatim (not as proxy placeholders)

The provider system doesn't support:
- Injecting files into sandboxes (only env vars)
- Passing config values verbatim without placeholder substitution
- Google Cloud ADC as a credential type

**Workaround:** cc-deck handles Vertex credentials manually:
1. Upload the ADC JSON file via `openshell sandbox upload`
2. Append env var exports to `/sandbox/.bashrc` via `openshell sandbox exec`

This is fragile because it depends on the shell sourcing `.bashrc` and races with sandbox readiness.

**Suggestion:** Extend the provider system with:
- A file injection capability: `openshell provider create --type generic --file GOOGLE_APPLICATION_CREDENTIALS=~/.config/gcloud/application_default_credentials.json` that mounts the file into the sandbox at a known path
- A config (non-secret) injection mode: credentials that are injected as literal env vars, not proxy placeholders. Useful for `CLAUDE_CODE_USE_VERTEX=1` and `CLOUD_ML_REGION=us-central1`
- A `vertex` or `gcloud` provider type that handles Google Cloud ADC natively

**Existing issues:**
- [#896 - Enhanced Provider Management](https://github.com/NVIDIA/OpenShell/issues/896) proposes extensible provider profiles with OAuth2 support but doesn't address file-based credentials or Google Cloud ADC specifically
- No existing issue for file injection or Vertex AI provider type

## 4. Default Landlock Policy Blocks /dev/urandom

**Problem:** The default sandbox policy lists `/dev/urandom` under `read_only` paths, but Landlock enforcement prevents Zellij (and potentially other tools) from reading from it. Zellij's `uuid` crate panics with "could not retrieve random bytes: Permission denied".

**Workaround:** Create a custom policy that moves `/dev/urandom`, `/dev/random`, and `/dev/pts` to the `read_write` list.

**Suggestion:** The default policy should allow read access to `/dev/urandom` and `/dev/random` since these are essential for any application generating UUIDs, TLS sessions, or cryptographic operations. Most sandboxed workloads will need them.

**Existing issues:** None found. The closed issues [#902](https://github.com/NVIDIA/OpenShell/issues/902) and [#955](https://github.com/NVIDIA/OpenShell/issues/955) addressed related Landlock problems but not this specific case.

## 5. sandbox create Blocks and Timeout Is Not Configurable

**Problem:** `openshell sandbox create` blocks until the sandbox reaches Ready state, with a fixed 300s timeout. On first image pull (base image is ~1GB), the pull easily exceeds 300s, causing the create command to fail even though the pull continues in the background.

The sandbox eventually becomes Ready (the k3s pull continues), but the CLI has already returned an error. Retrying with `sandbox create` would attempt to create a duplicate.

**Workaround:** Accept the timeout error, then poll `sandbox get` / `sandbox list` until the phase changes to Ready. The image is cached after the first pull, so subsequent creates are fast.

**Suggestion:**
- Make the create timeout configurable: `openshell sandbox create --timeout 600 ...`
- Or: separate sandbox allocation from readiness waiting: `openshell sandbox create --no-wait` to return immediately with the sandbox name, then `openshell sandbox wait <name>` to poll for readiness

## 6. sandbox download Creates a Directory

**Problem:** `openshell sandbox download <name> /remote/file.txt /local/file.txt` treats the local path as a directory and creates `/local/file.txt/file.txt` instead of writing to `/local/file.txt` directly.

**Workaround:** Account for this behavior in the data channel by always treating the local path as a directory target.

**Suggestion:** Match `scp` semantics: if the local path doesn't exist and the remote is a file, write directly to the local path. If the local path exists and is a directory, place the file inside it.

## 7. No Vertex AI Network Policy Preset

**Problem:** When using Claude Code with Vertex AI, the sandbox needs network access to `aiplatform.googleapis.com` and `oauth2.googleapis.com`. The default policy doesn't include Google Cloud endpoints, and the `claude_code` network policy preset only allows `api.anthropic.com`.

**Workaround:** Create a custom policy adding the Google Cloud endpoints manually.

**Suggestion:** Add a `vertex_ai` or `gcloud` network policy preset (alongside the existing `claude_code` preset) that allows:
- `aiplatform.googleapis.com:443` (Vertex AI API)
- `oauth2.googleapis.com:443` (OAuth2 token refresh)
- `www.googleapis.com:443` (API discovery)
- Regional variants (`us-central1-aiplatform.googleapis.com`, etc.)

This could be tied to a future `vertex` provider type.

## References

| Issue | Title | Status | Relevant finding |
|-------|-------|--------|-----------------|
| [#1034](https://github.com/NVIDIA/OpenShell/issues/1034) | Support --json in `openshell policy get` | Open | Finding 1 (JSON output) |
| [#1046](https://github.com/NVIDIA/OpenShell/issues/1046) | sandbox exec hangs after complete gRPC response | Open | Finding 2 (exec --tty) |
| [#828](https://github.com/NVIDIA/OpenShell/issues/828) | sandbox exec hangs after complete gRPC response | Open | Finding 2 (exec --tty) |
| [#896](https://github.com/NVIDIA/OpenShell/issues/896) | Enhanced Provider Management | Open | Finding 3 (file credentials) |
| [#902](https://github.com/NVIDIA/OpenShell/issues/902) | Landlock unavailable tracing corrupts stdout | Closed | Finding 4 (Landlock) |
| [#955](https://github.com/NVIDIA/OpenShell/issues/955) | Node.js EINVAL inside sandbox | Closed | Finding 4 (device access) |

## Issues to Propose

Based on findings with no existing coverage:

1. **Global `-o json` flag for CLI commands** (extends #1034 scope beyond just `policy get`)
2. **File-based credential injection via providers** (new capability, not covered by #896)
3. **Vertex AI / Google Cloud provider type** with ADC file support
4. **Default policy should allow `/dev/urandom` and `/dev/random`** read access
5. **Configurable sandbox create timeout** or `--no-wait` mode
6. **Vertex AI network policy preset** alongside the existing `claude_code` preset
