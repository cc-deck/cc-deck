# Research: OpenShell Profile Delegation

## R1: Policy Assembly vs Workspace Creation Paths

**Decision**: The profile delegation affects BOTH the build-time and runtime paths.

**Rationale**: Currently there are two separate policy paths:
- **Build time** (`cc-deck build run`): `AssemblePolicy()` in `internal/build/policy.go` generates a complete `PolicyFile` from components + manifest, embeds it in the OCI image at `/etc/openshell/policy.yaml`.
- **Runtime** (`cc-deck ws new`): `loadSDKPolicy()` in `internal/ws/openshell.go` extracts the policy YAML from the OCI image and passes it as `SandboxSpec.Policy`.

With profile delegation, the build phase embeds a profile manifest (list of required profile names) instead of a full policy. The runtime phase reads the profile manifest and creates providers referencing those profiles.

**Alternatives considered**: Having `ws new` re-derive profiles from the image at runtime (too slow, requires re-probing). Having the gateway auto-detect profiles from the image (gateway has no visibility into image contents).

## R2: Profile Manifest Artifact

**Decision**: Embed a `profiles.yaml` file at `/etc/openshell/profiles.yaml` in the OCI image, replacing the embedded policy for OpenShell targets.

**Rationale**: The image needs to be self-describing. At `ws new` time, the original build manifest may not be available. The profile manifest is a simple list:

```yaml
profiles:
  - anthropic
  - claude-agent
  - python
  - github
```

**Format**: YAML list under a `profiles` key. No additional metadata needed since profile details are owned by the gateway.

**Alternatives considered**: Embedding profile info in the existing policy YAML (mixes two concerns). Requiring the manifest at `ws new` time (breaks the current workflow where only the image is needed).

## R3: Agent Name to Profile Mapping

**Decision**: Maintain a static mapping table in a new file `internal/openshell/profiles.go`.

**Rationale**: The mapping is simple and changes rarely (only when new agents or tools are added). The table maps cc-deck identifiers to OpenShell profile IDs:

| cc-deck identifier | Profile ID | Source |
|-------------------|------------|--------|
| `claude` (agent) | `anthropic`, `claude-agent` | `RequiredDomainGroups()` + agent profile |
| `opencode` (agent) | `openai`, `opencode-agent` | `RequiredDomainGroups()` + agent profile |
| `codex` (agent) | `openai`, `codex-agent` | `RequiredDomainGroups()` + agent profile |
| `python` (tool) | `python` | Tool detection |
| `node`/`npm` (tool) | `nodejs` | Tool detection |
| `go` (tool) | `golang` | Tool detection |
| `rust`/`cargo` (tool) | `rust` | Tool detection |
| `git` (always) | `github`, `gitlab` | Always included |
| `docker` (tool) | `docker` | Tool detection |
| `quay` (registry) | `quay` | Registry detection |
| `vertex` (credential) | `vertexai` | Credential spec |

**Alternatives considered**: Dynamic discovery from gateway profiles (adds latency, coupling). Storing mapping in YAML config (over-engineering for a static table).

## R4: Hardcoded Agent Name in Workspace Creation

**Decision**: Fix `resolveAgentName()` to use the manifest's `EffectiveAgents()` instead of hardcoding `"claude"`.

**Rationale**: Line 244 of `openshell.go` hardcodes `return "claude"`. Line 342 hardcodes `agentName := "claude"`. For multi-agent profile support, the workspace creation must iterate over all agents in the manifest (or in the profile manifest extracted from the image).

**Alternatives considered**: None. This is a bug fix prerequisite.

## R5: Provider Creation Strategy

**Decision**: For each profile in the profile manifest, cc-deck calls `client.Providers().Ensure()` with `Provider.Type` set to the profile ID. Credential-carrying providers (anthropic, openai, vertexai) additionally populate `ProviderSpec.Credentials`.

**Rationale**: `Ensure()` is idempotent (creates if absent, returns existing if present). Setting `Provider.Type` links the provider to its profile. The gateway uses the profile to determine network endpoints.

Credential providers (from `credential.Detect()` / `credential.Resolve()`) are a subset of profile-based providers. The same provider carries both the profile reference (via `Type`) and credentials (via `ProviderSpec.Credentials`).

**Alternatives considered**: Creating providers via `Create()` (not idempotent, fails on re-creation). Separating credential providers from profile providers (over-complicates, creates orphaned providers).

## R6: MCP Ephemeral Profile Strategy

**Decision**: For MCP endpoints, import a single profile per workspace via `ProfileInterface.Import()`, then create a provider referencing it.

**Rationale**: MCP endpoints are user-specific and change between workspaces. The profile name follows the pattern `cc-deck-<workspace-name>-mcp`. The profile is imported once (idempotent via `Import()`), then a provider of that type is created and attached.

The profile's `Endpoints` field is populated from the manifest's `MCP` entries. The `Binaries` field is populated from the agent-matched component binaries (same logic as current `collectAgentBinaries()`).

**Alternatives considered**: One profile per MCP endpoint (too granular, creates N profiles). Embedding MCP endpoints in the profile manifest YAML (mixes gateway-side and user-side concerns).

## R7: Compose/Container Environment Isolation

**Decision**: The existing `AssemblePolicy()` code path remains fully functional for non-OpenShell targets. Profile delegation is gated by the target type.

**Rationale**: `AssemblePolicy()` is called by the build system for all targets. For OpenShell targets, it will be replaced by profile manifest generation. For compose targets, it continues to produce the full `PolicyFile` as before.

The gate is in the build pipeline: if `manifest.Targets.OpenShell` is configured, generate a profile manifest. If compose is configured, generate a policy file. Both can coexist in the same build.

**Alternatives considered**: Removing `AssemblePolicy()` entirely (breaks compose environments). Feature-flagging (over-engineering for a target-type distinction).
