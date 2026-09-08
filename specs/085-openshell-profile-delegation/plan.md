# Implementation Plan: OpenShell Profile Delegation

**Branch**: `085-openshell-profile-delegation` | **Date**: 2026-07-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/085-openshell-profile-delegation/spec.md`

## Summary

Replace cc-deck's homegrown policy generation for OpenShell targets with profile delegation. Instead of assembling a complete `SandboxPolicy` from embedded YAML components, domain groups, and binary probing, cc-deck maps detected tools and manifest declarations to OpenShell provider profile IDs. At build time, a profile manifest is embedded in the OCI image. At workspace creation time, cc-deck creates providers referencing those profiles and passes them to the gateway with `SandboxSpec.Policy` set to nil. The gateway resolves profiles into the complete sandbox policy.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), openshell-sdk-go v0.2.2 (SDK), gopkg.in/yaml.v3, serde/serde_json (plugin)
**Storage**: OCI image `/etc/openshell/profiles.yaml` (new artifact), gateway profile store (managed by OpenShell)
**Testing**: `make test` (Go tests via `go test ./...`), `make lint` (golangci-lint)
**Target Platform**: macOS/Linux CLI tool
**Project Type**: CLI + Zellij WASM plugin
**Performance Goals**: N/A (workspace creation is already gateway-bound)
**Constraints**: Backward compatibility with existing non-OpenShell targets; compose environments unaffected
**Scale/Scope**: ~12 files changed across `internal/openshell/`, `internal/ws/`, `internal/build/`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Plan includes unit tests for profile mapping, workspace creation, and build-time manifest generation. Doc updates for manifest reference and architecture guide. |
| II. Interface contracts | PASS | No new interface backends. Extends existing workspace creation flow with profile resolution. |
| III. Build and tool rules | PASS | Using `make test`/`make lint`, `internal/xdg`, `podman` |
| IV. Plugin debug logging | N/A | No plugin changes in this feature |
| V. Command files as code | N/A | No command file changes |

## Project Structure

### Documentation (this feature)

```text
specs/085-openshell-profile-delegation/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (from /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/openshell/
├── profiles.go          # NEW: ProfileMapping table + ResolveProfiles()
├── profiles_test.go     # NEW: Profile mapping tests
├── client.go            # MODIFY: Add profile verification helper
└── credentials.go       # MODIFY: Update provider creation to use profile types

cc-deck/internal/ws/
├── openshell.go         # MODIFY: Replace policy loading with profile-based providers
└── openshell_test.go    # MODIFY: Update workspace creation tests

cc-deck/internal/build/
├── policy.go            # MODIFY: Add profile manifest generation path for OpenShell
├── policy_test.go       # MODIFY: Add profile manifest tests
├── profiles.go          # NEW: ProfileManifest struct and serialization
└── profiles_test.go     # NEW: Profile manifest tests

cc-deck/internal/oci/
└── extract.go           # MODIFY: Add extraction for profiles.yaml (reuse existing)
```

**Structure Decision**: All changes occur within existing packages. Two new files (`internal/openshell/profiles.go` and `internal/build/profiles.go`) hold the profile mapping table and manifest format respectively. No new packages needed.

## Design Decisions

### D1: Profile Mapping Table

A static Go map in `internal/openshell/profiles.go` maps cc-deck identifiers to profile IDs. The function `ResolveProfiles(agents []string, tools []string, credentials []string) []string` takes manifest inputs and returns a deduplicated, sorted list of profile IDs.

The mapping is derived from the current domain groups in `internal/network/builtin.go` and agent `RequiredDomainGroups()` methods:
- Agent "claude" -> profiles: ["anthropic", "claude-agent"]
- Agent "opencode" -> profiles: ["openai", "opencode-agent"]
- Agent "codex" -> profiles: ["openai", "codex-agent"]
- Tool "python" -> profile: ["python"]
- Tool "node"/"npm" -> profile: ["nodejs"]
- Tool "go" -> profile: ["golang"]
- Tool "rust"/"cargo" -> profile: ["rust"]
- Credential "vertex" -> profile: ["vertexai"]
- Always: ["github", "gitlab"]

### D2: Build-Time Profile Manifest

During `cc-deck build run` for OpenShell targets, instead of calling `AssemblePolicy()`, the build generates a `profiles.yaml` and embeds it in the OCI image at `/etc/openshell/profiles.yaml`. The existing policy generation continues for compose targets.

The profile manifest is produced by `BuildProfileManifest(manifest *Manifest) (*ProfileManifest, error)` in `internal/build/profiles.go`. It:
1. Reads `manifest.EffectiveAgents()` and maps via the profile table
2. Reads matched tool components and maps via the profile table
3. Adds always-included profiles (github, gitlab)
4. Deduplicates and sorts

### D3: Runtime Profile Resolution

In `internal/ws/openshell.go`, the `Create()` method changes:

1. **Extract profiles** (replaces policy extraction): `oci.ExtractFileFromImage(image, "/etc/openshell/profiles.yaml")`
2. **Verify profiles**: For each profile ID, call `client.Providers().Profiles().Get()` to verify it exists on the gateway
3. **Create providers**: For each profile ID, call `client.Providers().Ensure()` with `Provider.Type` set to the profile ID
4. **Merge credential providers**: Credential-carrying providers (anthropic, vertexai) additionally set `ProviderSpec.Credentials`
5. **Import ephemeral profiles**: For MCP endpoints and user-defined domains, call `client.Providers().Profiles().Import()` then create providers
6. **Build SandboxSpec**: Set `Providers` to the full list, `Policy` to nil

### D4: Fix Hardcoded Agent Name

Replace `resolveAgentName()` (always returns "claude") with reading agents from the profile manifest. The credential resolution iterates over all agents instead of hardcoding a single one.

### D5: Ephemeral Profile Creation

For MCP endpoints:
- Profile ID: `cc-deck-<sanitized-workspace-name>-mcp`
- Category: "Other"
- Endpoints: From `manifest.MCP[].Endpoints` host/port entries
- Binaries: From agent component binaries (reuse `collectAgentBinaries()` logic or derive from matched profiles)

For user-defined domains:
- Profile ID: `cc-deck-<sanitized-workspace-name>-custom`
- Category: "Other"
- Endpoints: From `manifest.Network.AllowedDomains` + filtered `AllowedDomainsPerAgent`

### D6: Non-OpenShell Target Isolation

The existing `AssemblePolicy()` continues to work for compose targets. The build pipeline checks `manifest.Targets.OpenShell` to decide which path to take:
- OpenShell configured: Generate profile manifest, skip policy assembly for that target
- Compose configured: Generate policy file via `AssemblePolicy()` as before
- Both configured: Generate both artifacts

## Global Constraints

- **Go version**: 1.25 (from `go.mod`)
- **Build commands**: `make test`, `make lint`, `make verify`. Never `go build` directly.
- **XDG paths**: Use `internal/xdg` package
- **Container runtime**: `podman` exclusively
- **Backward compatibility**: Non-OpenShell targets must produce identical output

## Complexity Tracking

No constitution violations. No complexity justification needed.
