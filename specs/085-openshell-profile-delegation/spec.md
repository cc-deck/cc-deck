# Feature Specification: OpenShell Profile Delegation

**Feature Branch**: `085-openshell-profile-delegation`
**Created**: 2026-07-31
**Status**: Draft
**Input**: Brainstorm 090 - OpenShell Profile Delegation

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Profile-Based Sandbox Creation (Priority: P1)

A developer creates an OpenShell workspace using `cc-deck ws new --type openshell`. Today, cc-deck generates a complete `SandboxPolicy` (network endpoints, filesystem rules, process config, Landlock) and passes it to the gateway. With profile delegation, cc-deck instead determines which OpenShell provider profiles the workspace needs (based on detected tools and manifest declarations), creates providers that reference those profiles, and passes them in the `SandboxSpec.Providers` list with `Policy` set to nil. The gateway resolves the attached providers' profiles into the sandbox policy.

**Why this priority**: This is the core behavioral change. Without it, cc-deck still generates policies itself and the entire feature has no effect.

**Independent Test**: Create a workspace with a manifest declaring `agents: [claude]` and `tools: [python]`. Verify that cc-deck does not generate a `SandboxPolicy` struct, instead creates providers referencing the "anthropic" and "python" profiles, and the gateway produces a working sandbox with the correct network access.

**Acceptance Scenarios**:

1. **Given** a manifest with `agents: [claude]` and no explicit tools, **When** a workspace is created, **Then** cc-deck creates providers referencing the "anthropic" and "claude-agent" profiles, passes them in `SandboxSpec.Providers` with `Policy` set to nil, and the sandbox has network access to Anthropic API domains.
2. **Given** a manifest with `agents: [claude, opencode]` and `tools: [python, go]`, **When** a workspace is created, **Then** cc-deck creates providers for "anthropic", "claude-agent", "openai", "opencode-agent", "python", and "golang" profiles, and the sandbox has network access to all corresponding domains.
3. **Given** a manifest with no `agents` field, **When** a workspace is created, **Then** cc-deck defaults to `["claude"]` for backward compatibility and creates providers for the "anthropic" and "claude-agent" profiles.

---

### User Story 2 - Auto-Detection Maps to Profiles (Priority: P1)

A developer builds a container image that includes Python and Node.js but does not explicitly list them in the manifest's `tools` field. During the build phase, cc-deck detects these tools in the image. Instead of generating policy YAML entries for them, cc-deck maps the detections to profile names ("python", "nodejs") and includes corresponding providers when creating the sandbox.

**Why this priority**: Auto-detection is the convenience layer that prevents users from manually listing every tool. Without it, users must exhaustively declare profiles in the manifest.

**Independent Test**: Build an image that has Python and Node.js installed. Create a workspace with a manifest that does not list any tools. Verify that cc-deck auto-detects Python and Node.js and creates providers referencing the "python" and "nodejs" profiles.

**Acceptance Scenarios**:

1. **Given** an image with Python installed and a manifest with no `tools` field, **When** a workspace is created, **Then** cc-deck detects Python and creates a provider referencing the "python" profile.
2. **Given** a manifest that explicitly lists `tools: [python]` and an image that also has Node.js installed, **When** a workspace is created, **Then** both "python" (from manifest) and "nodejs" (from detection) providers are created (union merge).
3. **Given** a manifest that explicitly lists `tools: [python]` and an image that also has Python installed, **When** a workspace is created, **Then** only one "python" provider is created (no duplicates).

---

### User Story 3 - MCP Endpoint Profiles (Priority: P2)

A developer configures custom MCP servers in their manifest (e.g., a private API on `internal.corp.example.com:8443`). Since no gateway profile exists for these custom endpoints, cc-deck imports an ephemeral profile via `ProfileInterface.Import()` containing the custom endpoints and binaries, creates a provider referencing that profile, and includes it in the sandbox's provider list.

**Why this priority**: MCP endpoints are user-specific and cannot be covered by standard gateway profiles. This is the escape hatch for custom network requirements without falling back to the old policy generation system.

**Independent Test**: Configure an MCP endpoint in the manifest pointing to a private domain. Create a workspace and verify that cc-deck imports a custom profile with that domain's endpoints and the sandbox has network access to it.

**Acceptance Scenarios**:

1. **Given** a manifest with an MCP endpoint at `internal.corp.example.com:8443`, **When** a workspace is created, **Then** cc-deck imports a profile with that endpoint via `ProfileInterface.Import()`, creates a provider referencing it, and the sandbox can reach that endpoint.
2. **Given** a manifest with multiple MCP endpoints, **When** a workspace is created, **Then** cc-deck imports a single profile containing all MCP endpoints (not one profile per endpoint).

---

### User Story 4 - User-Defined Domain Overrides as Profiles (Priority: P2)

A developer adds custom domains to their manifest via `allowed_domains` or `allowed_domains_per_agent`. Instead of cc-deck generating policy entries for these, it imports a custom profile containing the user's domain endpoints and includes it alongside the standard profiles.

**Why this priority**: Users need a way to extend network access beyond what standard profiles provide. This replaces the current `AllowedDomains` mechanism with the profile system.

**Independent Test**: Add a custom domain to the manifest's `allowed_domains`. Create a workspace and verify that the custom domain appears in the sandbox's network policy via a profile.

**Acceptance Scenarios**:

1. **Given** a manifest with `allowed_domains: ["custom.example.com"]`, **When** a workspace is created, **Then** cc-deck imports a profile containing `custom.example.com` as an endpoint and includes it in the provider list.
2. **Given** a manifest with `allowed_domains_per_agent: {claude: ["extra.example.com"]}` and agent "claude" in the manifest, **When** a workspace is created, **Then** the extra domain is included in the imported profile.
3. **Given** a manifest with `allowed_domains_per_agent: {opencode: ["extra.example.com"]}` but agent "opencode" NOT in the manifest, **When** a workspace is created, **Then** the extra domain is excluded.

---

### User Story 5 - Policy Code Removal (Priority: P3)

A maintainer reviewing the codebase finds that the homegrown policy generation code has been removed for the OpenShell target. The embedded policy YAMLs, builtin domain group registry, two-pass binary probing (for policy purposes), and policy assembly engine are no longer used when building for OpenShell. The code may remain for non-OpenShell targets (compose environments) but is not invoked for OpenShell workspace creation.

**Why this priority**: This is cleanup work that follows from the delegation. The new profile-based path must be working and validated before removing the old code.

**Independent Test**: Search the OpenShell workspace creation path for any remaining `SandboxPolicy` struct construction. None should exist.

**Acceptance Scenarios**:

1. **Given** the codebase after this feature, **When** searching the OpenShell workspace creation code path, **Then** no `SandboxPolicy` struct is constructed or passed to `SandboxSpec.Policy`.
2. **Given** a compose environment workspace, **When** it is created, **Then** the existing policy generation code still works (non-OpenShell targets are unaffected).

---

### Edge Cases

- What happens when a detected tool has no matching gateway profile? cc-deck emits a warning identifying the tool and the missing profile, and skips it. The sandbox is created without that tool's profile. The build does not fail.
- What happens when the gateway is unreachable during workspace creation? The operation fails with a clear error message, since gateway connectivity is required for OpenShell workspaces regardless of this change.
- What happens when `ProfileInterface.Import()` fails for an MCP endpoint profile? cc-deck emits a warning and continues without the MCP profile. The sandbox is created but may not have access to the custom endpoint.
- What happens when the same profile is referenced by both manifest declaration and auto-detection? Only one provider is created (deduplication by profile name).
- What happens when the gateway has a profile but with different endpoints than what cc-deck's old system would have generated? The gateway's profile is authoritative. cc-deck does not validate profile contents.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: cc-deck MUST NOT generate a `SandboxPolicy` struct for OpenShell workspace creation. `SandboxSpec.Policy` MUST be nil.
- **FR-002**: cc-deck MUST maintain a mapping table from detected tools and agent names to OpenShell provider profile IDs. The mapping covers: anthropic, openai, python, nodejs, golang, rust, github, gitlab, docker, quay, vertexai, claude-agent, opencode-agent.
- **FR-003**: cc-deck MUST create providers referencing the appropriate profiles and pass their names in `SandboxSpec.Providers` when creating a sandbox.
- **FR-004**: Manifest declarations (`agents`, `tools`) MUST take priority over auto-detection. Auto-detection fills gaps for tools not explicitly listed. The result is a union of both sources, deduplicated by profile name.
- **FR-005**: cc-deck MUST use `ProfileInterface.Import()` to create profiles for user-specific MCP endpoints. Each MCP endpoint set is imported as a single profile with all endpoints.
- **FR-006**: cc-deck MUST use `ProfileInterface.Import()` to create profiles for user-defined domain overrides (`allowed_domains`, `allowed_domains_per_agent`).
- **FR-007**: When a detected tool has no matching gateway profile, cc-deck MUST emit a warning and skip the tool without failing the build.
- **FR-008**: When `ProfileInterface.Import()` fails, cc-deck MUST emit a warning and continue without the custom profile.
- **FR-009**: The existing policy generation code MUST continue to work for non-OpenShell targets (compose environments).
- **FR-010**: When the manifest has no `agents` field, cc-deck MUST default to `["claude"]` for backward compatibility.
- **FR-011**: The git-hosting profile ("github", "gitlab") MUST always be included regardless of manifest declarations, since git access is required for all workspaces.
- **FR-012**: cc-deck MUST verify each standard profile exists on the gateway via `ProfileInterface.Get()` before creating a provider for it. Missing profiles are handled per FR-007 (warn and skip).
- **FR-013**: Imported ephemeral profiles MUST use deterministic names based on the workspace name: `cc-deck-<workspace-name>-mcp` for MCP endpoints and `cc-deck-<workspace-name>-custom` for user domain overrides.
- **FR-014**: Credential providers MUST reference their profile via `Provider.Type` and carry credentials in `ProviderSpec.Credentials`. Profile declarations (network endpoints) and credential injection (API keys, ADC) are orthogonal and coexist on the same provider.

### Key Entities

- **Profile Mapping Table**: A map from cc-deck tool/agent identifiers to OpenShell profile IDs. Maintained in cc-deck code. Updated only when new tools/agents are added.
- **Ephemeral Profile**: A custom profile imported to the gateway via `ProfileInterface.Import()` for user-specific endpoints (MCP servers, custom domains). Created per workspace, may persist on the gateway.
- **Provider**: An OpenShell provider instance that references a profile type. Created by cc-deck and attached to the sandbox. The gateway resolves the provider's profile into policy entries.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: OpenShell workspace creation does not construct or pass a `SandboxPolicy` struct. Zero policy assembly code is invoked for OpenShell targets.
- **SC-002**: A workspace created with `agents: [claude]` and `tools: [python]` results in providers referencing exactly the expected profiles, and the sandbox has working network access to Anthropic API and PyPI.
- **SC-003**: Adding a new tool/agent to cc-deck requires only adding an entry to the profile mapping table (one line of code), not modifying policy assembly or adding YAML components.
- **SC-004**: All existing tests for non-OpenShell targets continue to pass without modification.
- **SC-005**: The profile mapping table covers all 13 current mappings: anthropic, openai, python, nodejs, golang, rust, github, gitlab, docker, quay, vertexai, claude-agent, opencode-agent.

## Assumptions

- The OpenShell gateway (version TBD) supports profile-based policy resolution: when providers with profiles are attached to a sandbox and `Policy` is nil, the gateway generates the sandbox policy from the providers' profiles.
- The OpenShell gateway has standard profiles for all 13 tool/agent categories that cc-deck currently covers. These profiles are either pre-installed or can be imported.
- `ProfileInterface.Import()` creates profiles that persist on the gateway until explicitly deleted. cc-deck does not manage profile lifecycle (cleanup).
- The gateway handles profile-to-policy expansion internally, including L7 rules, TLS settings, enforcement modes, and all fields present in `PolicyNetworkEndpoint` but absent from profile-level `NetworkEndpoint`.
- Filesystem policy, process policy, and Landlock configuration are sandbox-level defaults managed by the gateway, not derived from profiles.
- Compose/container environments continue to use the existing policy generation system. This feature is OpenShell-only.

## Clarifications

### Session 2026-07-31

- Q: Should filesystem, process, and Landlock rules be delegated to the gateway or only network policy? A: Full delegation. The gateway owns all sandbox policy aspects. cc-deck has no business specifying filesystem mounts, process user/group, or Landlock settings.
- Q: Should there be an offline fallback for builds without gateway connectivity? A: No. OpenShell workspaces inherently require a gateway. No offline mode needed.
- Q: How should MCP endpoints be handled since they're user-specific? A: Import ephemeral profiles via `ProfileInterface.Import()`. The gateway resolves them uniformly alongside standard profiles.
- Q: Should auto-detection be removed in favor of manifest-only declarations? A: No. Both paths coexist. Manifest declarations take priority, auto-detection fills gaps. The result is a union.
- Q: Should cc-deck verify that referenced profiles exist on the gateway before creating providers? A: Yes. Call `ProfileInterface.Get()` to verify each profile exists. If missing, emit a warning and skip (consistent with FR-007). This prevents confusing gateway errors from non-existent profile references.
- Q: How are imported ephemeral profiles named to avoid collisions across workspaces? A: Use deterministic naming with `cc-deck-<workspace-name>-mcp` and `cc-deck-<workspace-name>-custom` patterns. Same workspace always references the same profile name, enabling re-creation without duplicates.
- Q: How do existing credential providers interact with profile-based providers? A: Orthogonal concerns. Credential providers use `ProviderSpec.Credentials` for secrets and `Provider.Type` to reference the profile. The profile declares network endpoints; the credential populates authentication. Same provider carries both.
