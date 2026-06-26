# Brainstorm: OpenShell Native Vertex Provider

**Date:** 2026-06-26
**Status:** active

## Problem Framing

cc-deck has a homegrown Vertex AI credential system spread across 6+ files (`openshell/credentials.go`, `ssh/credentials.go`, `agent/claude.go`, `network/builtin.go`, `compose/generate.go`, `config/profile.go`). For OpenShell workspaces, this includes a custom `vertex` provider profile, `GOOGLE_APPLICATION_CREDENTIALS` file upload as a workaround (since OpenShell had no native Vertex support), and manual network policy entries for GCP endpoints.

OpenShell has now merged native `google-cloud` provider support ([PR #1763](https://github.com/NVIDIA/OpenShell/pull/1763)). This adds a GCE metadata emulator running on loopback inside the sandbox. GCP SDKs discover it via `GCE_METADATA_HOST`, get credential placeholders, and include those in API calls. The sandbox proxy resolves placeholders to real tokens at egress. The sandbox process never holds a real credential.

The `inference.local` approach (Option B in [maxamillion's gist](https://gist.github.com/maxamillion/177a36f959911900e6f2a1f625cdcc3a)) is being deprecated for Vertex by the OpenShell maintainer in favor of this metadata emulator approach (confirmed in [Slack thread](https://redhat-internal.slack.com/archives/C0995TL0ZV3/p1780419265135739)).

Provider creation is straightforward:
```bash
openshell provider create --name redhat-gcp --type google-cloud \
  --from-gcloud-adc --config project_id="$(gcloud config get-value project)" \
  --config region=global
```

## Approaches Considered

### A: Swap provider type in existing credential system

Change the `claude-vertex` profile in `openshell/credentials.go` to create a `google-cloud` provider via `--from-gcloud-adc`. Remove file upload logic. Keep env var injection. Leave dead code in place.

- Pros: Smallest diff, lowest risk
- Cons: Leaves dead `vertex` profile, unused file credential logic, and Vertex network domains as dead code

### B: Swap + cleanup dead code (Chosen)

Same provider swap as A, plus remove all dead code paths: delete the separate `vertex` provider profile, remove `FileCredential` from OpenShell credential spec, remove Vertex-specific domains from `internal/network/builtin.go` for OpenShell policy, clean up profile configuration.

- Pros: Removes dead code immediately. Clear separation between "OpenShell handles GCP auth" vs. "other workspace types inject credentials directly"
- Cons: Slightly larger diff. Must preserve non-OpenShell Vertex support (domain allowlist and file credential handling still needed for container/SSH/K8s workspaces)

### C: Clean break aligned with brainstorm #069

Rewrite OpenShell credential handling to match the Agent interface `CredentialSpecs()` pattern from brainstorm #069.

- Pros: Forward-compatible with multi-agent support
- Cons: Pulls in unfinished design work from #069. Risk of premature abstraction

## Decision

**Approach B: Swap + cleanup dead code.**

Delivers the functional change and removes dead code without over-engineering. No backwards compatibility with older OpenShell versions needed.

## Key Requirements

1. **Replace `claude-vertex` provider profile** to use OpenShell's `google-cloud` provider type with `--from-gcloud-adc`
2. **Remove file credential handling** for Vertex in OpenShell workspaces (no more `GOOGLE_APPLICATION_CREDENTIALS` file upload)
3. **Delete the separate `vertex` provider profile** from `openshell/credentials.go` (dead code)
4. **Remove Vertex-specific domains** from `internal/network/builtin.go` for OpenShell policy generation (keep for other workspace types)
5. **Keep env var injection**: `CLAUDE_CODE_USE_VERTEX=1`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION` (Claude Code still needs these)
6. **Clean up profile config**: skip `credentials_secret` prompt for OpenShell workspaces
7. **Scope**: OpenShell workspaces only. Non-OpenShell workspace types (container, SSH, K8s, compose) keep their existing Vertex handling unchanged

## Open Questions

- Does OpenShell's `google-cloud` provider require the `providers_v2_enabled` flag, or is that only for `inference.local`? (The gist shows it for Option B only, suggesting it's inference.local-specific)
- Minimum OpenShell version that includes PR #1763? (For documentation purposes, not for version detection)
