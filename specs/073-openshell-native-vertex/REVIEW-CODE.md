# Code Review: OpenShell Native Vertex Provider

**Branch**: `073-openshell-native-vertex`
**Date**: 2026-06-26
**Files changed**: 5 (88 insertions, 62 deletions)

## Spec Compliance Check

| Requirement | Status | Evidence |
|-------------|--------|----------|
| FR-001: Create google-cloud provider via --from-gcloud-adc | PASS | `claude-vertex` profile Type changed to `"google-cloud"`, `CreateProvider` uses `--from-gcloud-adc` for this type |
| FR-002: Pass project_id and region as --config options | PASS | `ResolveCredentials` populates `cfg.Credentials` with `project_id` and `region`, `CreateProvider` passes as `--config` for google-cloud type |
| FR-003: Keep injecting CLAUDE_CODE_USE_VERTEX env vars | PASS | `EnvVarsToInject` populated from detected env vars in `ResolveCredentials` |
| FR-004: No GOOGLE_APPLICATION_CREDENTIALS file upload | PASS | `FileVar` removed from profile, `ws/openshell.go` skips upload for `google-cloud` type |
| FR-005: Remove standalone vertex profile | PASS | `vertex` entry deleted from `KnownProviderProfiles` |
| FR-006: Remove Vertex domains from OpenShell policy path | PASS | `Endpoints` removed from `claude-vertex` profile |
| FR-007: Preserve vertexai domain group for non-OpenShell | PASS | `network/builtin.go` unchanged |
| FR-008: Update claude-vertex to use google-cloud provider | PASS | Type changed, `SkipProvider` logic replaced with provider creation |
| FR-009: Non-OpenShell workspace types unchanged | PASS | `agent/claude.go`, `ssh/credentials.go`, `config/profile.go` unchanged |
| FR-010: Skip credentials_secret for OpenShell | N/A | `credentials_secret` prompt in `profile.go` is for K8s workspaces, not triggered during OpenShell flows |

**Compliance Score: 9/9 applicable requirements satisfied (1 N/A)**

## Code Review Guide

### Files Modified

| File | Change Type | Risk |
|------|-------------|------|
| `cc-deck/internal/openshell/credentials.go` | Modified (profile entries, ResolveCredentials) | Medium - core credential path |
| `cc-deck/internal/openshell/credentials_test.go` | Modified (updated/added tests) | Low - test-only |
| `cc-deck/internal/openshell/client.go` | Modified (CreateProvider flag handling) | Medium - CLI construction |
| `cc-deck/internal/ws/openshell.go` | Modified (1 line - skip file upload) | Low - guard condition |
| `README.md` | Modified (documentation update) | Low - docs-only |

### Review Focus Areas

1. **Correctness of CLI flag construction** (`client.go:298-308`): When `providerType == "google-cloud" && fromExisting`, uses `--from-gcloud-adc` and `--config` instead of `--from-existing` and `--credential`. The condition correctly scopes this behavior to google-cloud only.

2. **Default region fallback** (`credentials.go:175`): When `CLOUD_ML_REGION` is unset, defaults to `"global"`. This matches the gist's recommended value.

3. **File upload guard** (`ws/openshell.go:322`): The condition `pc.Type != "google-cloud"` correctly prevents file upload for the new provider type. Belt-and-suspenders since `FilePath` will be empty anyway (no `FileVar` in the profile).

4. **Detection order** (`credentials.go:232`): `"vertex"` removed from the detection order. This means hosts with only `GOOGLE_APPLICATION_CREDENTIALS` set (no Vertex-specific vars) will no longer auto-detect vertex credentials. This is correct since the standalone vertex profile was removed.

## Deep Review Report

### Correctness Review

**Finding count**: 0 Critical, 0 Important, 0 Minor

The credential resolution logic correctly produces a `ProviderConfig` with `Type: "google-cloud"`, `FromExisting: true`, project_id/region in `Credentials`, and Claude Code env vars in `EnvVarsToInject`. The `SkipProvider` flag is no longer set, so `EnsureProvider` is called (creating the actual OpenShell provider). Test coverage confirms this flow with `TestResolveCredentials_ClaudeVertexUsesGoogleCloudProvider`.

### Architecture Review

**Finding count**: 0 Critical, 0 Important, 1 Minor

- **Minor**: The `providerType == "google-cloud"` check in `CreateProvider` couples provider-specific CLI flag knowledge into the generic provider creation method. Acceptable for now since it's the only provider needing special flags; would warrant abstraction if more provider types need custom flags.

### Security Review

**Finding count**: 0 Critical, 0 Important, 0 Minor

The change improves security posture: GCP credentials no longer enter the sandbox process (OpenShell's metadata emulator + proxy handle token resolution at egress). The injected env vars (`CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`) are non-secret configuration flags.

### Production Readiness Review

**Finding count**: 0 Critical, 0 Important, 0 Minor

Error handling preserved: `EnsureProvider` propagates creation failures. The `WARNING` log for failed env var injection remains. No new error paths introduced.

### Test Review

**Finding count**: 0 Critical, 0 Important, 0 Minor

Test coverage is thorough:
- `TestKnownProviderProfiles_AllTypesExist`: Updated to exclude `vertex`, assert it's absent
- `TestKnownProviderProfiles_ClaudeVertexUsesGoogleCloud`: Validates new profile fields
- `TestResolveCredentials_ClaudeVertexUsesGoogleCloudProvider`: Full flow test with assertions on Type, Credentials, EnvVarsToInject, empty FileVar/FilePath
- `TestResolveCredentials_ClaudeVertexDefaultsRegionToGlobal`: Region fallback coverage
- `TestDetectCredentials_ClaudeVertexDetection`: Detection with both detect vars set
- `TestDetectCredentials_NoStandaloneVertexProfile`: Ensures standalone GOOGLE_APPLICATION_CREDENTIALS no longer triggers detection

All 62 openshell package tests pass. Pre-existing pipe channel test failures (2) are unrelated.

### Summary

| Dimension | Critical | Important | Minor |
|-----------|----------|-----------|-------|
| Correctness | 0 | 0 | 0 |
| Architecture | 0 | 0 | 1 |
| Security | 0 | 0 | 0 |
| Production | 0 | 0 | 0 |
| Tests | 0 | 0 | 0 |
| **Total** | **0** | **0** | **1** |

**Gate: PASS** - No Critical or Important findings. 1 Minor architecture observation (acceptable coupling, deferred).
