# Implementation Plan: OpenShell Native Vertex Provider

**Branch**: `073-openshell-native-vertex` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/073-openshell-native-vertex/spec.md`

## Summary

Replace cc-deck's homegrown Vertex AI credential handling for OpenShell workspaces with OpenShell's native `google-cloud` provider type. The `claude-vertex` profile switches from `SkipProvider: true` (direct env var injection + file upload) to creating a `google-cloud` provider via `--from-gcloud-adc`. The standalone `vertex` profile is removed. File credential upload for Vertex is eliminated from the OpenShell path. Non-OpenShell workspace types are unchanged.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), internal/openshell (OpenShell client), internal/ws (workspace management), internal/network (domain filtering), internal/config (profile management), internal/agent (credential specs)
**Storage**: N/A (no data changes)
**Testing**: `make test` (Go tests), `make lint` (Go linters)
**Target Platform**: macOS, Linux (CLI tool)
**Project Type**: CLI tool with workspace management
**Performance Goals**: N/A (no performance-sensitive changes)
**Constraints**: Must preserve non-OpenShell Vertex support unchanged
**Scale/Scope**: 4-5 files modified, ~50 lines changed, ~30 lines removed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and docs | PASS | Unit tests for credential resolution will be updated. No new CLI commands/flags, so CLI reference unchanged. README update for changed Vertex auth behavior. |
| II. Interface contracts | PASS | No new interface implementations. Existing `KnownProviderProfile` map entries are modified/removed. |
| III. Build and tool rules | PASS | Using `make test`, `make lint`. No direct `go build`. |
| IV. Plugin debug logging | N/A | No plugin changes. |

## Project Structure

### Documentation (this feature)

```text
specs/073-openshell-native-vertex/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (files to modify)

```text
cc-deck/internal/
├── openshell/
│   ├── credentials.go       # PRIMARY: Update claude-vertex profile, remove vertex profile,
│   │                        #          remove SkipProvider logic for vertex
│   └── credentials_test.go  # Update tests to reflect removed vertex profile
├── ws/
│   └── openshell.go          # Update post-start injection: no file upload for google-cloud provider
├── network/
│   └── builtin.go            # Keep vertexai domain group (used by non-OpenShell workspaces)
├── config/
│   └── profile.go            # Skip credentials_secret prompt for OpenShell workspace type
└── agent/
    └── claude.go             # No changes (credential spec stays, used by non-OpenShell paths)
```

## Research Findings

No NEEDS CLARIFICATION items in Technical Context. Key research points:

1. **OpenShell `google-cloud` provider creation**: `openshell provider create --name <name> --type google-cloud --from-gcloud-adc --config project_id=<id> --config region=global`. The `--from-gcloud-adc` flag reads from `~/.config/gcloud/application_default_credentials.json`.

2. **EnsureProvider API**: The existing `client.EnsureProvider(ctx, name, type, fromExisting, credentials)` in `ws/openshell.go:300` supports `fromExisting=true`. Need to verify it also supports passing config options (project_id, region) for the `google-cloud` type. If not, the `Credentials` map field can carry these as config key-value pairs.

3. **`SkipProvider` removal scope**: Only the `claude-vertex` type sets `SkipProvider = true` (line 183). Once this type uses the `google-cloud` provider, the entire `SkipProvider` code path in `ws/openshell.go:297-333` can potentially be simplified, but only the Vertex-specific part should be removed. `InjectEnvVars` is still needed for injecting `CLAUDE_CODE_USE_VERTEX=1` and companion vars.

4. **Test impact**: `credentials_test.go` checks that `KnownProviderProfiles` contains expected types including `"vertex"`. This test must be updated to remove `"vertex"` from the expected list. `credential/validate_test.go` and `credential/resolve_test.go` reference `Name: "vertex"` but these test the agent-level credential spec (not the OpenShell provider profile), so they are unaffected.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
