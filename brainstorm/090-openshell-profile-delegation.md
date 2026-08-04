# Brainstorm: OpenShell Profile Delegation

**Date:** 2026-07-31
**Status:** active

## Problem Framing

cc-deck maintains a homegrown network policy system: 11 builtin domain groups, 8 embedded policy YAML components, a two-pass binary probing pipeline, and a policy assembly engine. This system generates a complete `SandboxPolicy` and passes it to the OpenShell gateway at sandbox creation time.

Meanwhile, the OpenShell SDK (v0.2.2) already has a `ProviderProfile` type with `NetworkEndpoint`, `NetworkBinary`, and `ProfileCategory` fields, plus a full `ProfileInterface` (List, Get, Import, Update, Lint, Delete). cc-deck doesn't use any of it.

The result: cc-deck duplicates knowledge that the gateway should own. Domain groups, binary paths, filesystem rules, process configuration, and Landlock settings are all coded into cc-deck, even though the gateway is the one enforcing them. Adding a new provider requires changes in both cc-deck and OpenShell. This coupling is unnecessary.

## Approaches Considered

### A: Full Profile Delegation (Chosen)

cc-deck stops generating `SandboxPolicy` entirely. The build pipeline becomes:

1. **Detect** tools in the image (or read from manifest declarations)
2. **Map** detections to OpenShell profile names (e.g., `python` detected -> `python` profile, `claude` agent -> `anthropic` profile)
3. **Import** ephemeral profiles for user-specific MCP endpoints
4. **Pass** the list of profile references when creating the sandbox
5. **Gateway resolves** profiles into the complete sandbox policy (network, filesystem, process, Landlock)

Removes: all embedded policy YAMLs, the builtin domain group registry, the two-pass binary probing (for policy purposes), the policy assembly engine, and `SandboxPolicy` struct generation.

What remains in cc-deck: tool detection (for mapping to profiles), the profile name mapping table, and MCP ephemeral profile creation.

- Pros: Maximum simplification. Single source of truth at the gateway. cc-deck's build code shrinks dramatically. New providers/tools just need a gateway profile, no cc-deck changes.
- Cons: Big bang change. Requires OpenShell gateway to have all current profiles. cc-deck's policy code (specs 059-078 worth of work) gets replaced. Compose/container environments that don't use OpenShell still need their own mechanism.

### B: Incremental Migration

Keep the current policy system as a fallback, add profile delegation as opt-in via `policy_source: profiles` in the manifest. Deprecate `builtin` once all gateway profiles exist.

- Pros: Zero-risk migration. Can validate profile-based policies against builtin-generated ones.
- Cons: Doubles maintenance burden during transition. Two code paths. Risk of never completing the migration.

### C: Profile-Sourced Policy Generation

cc-deck queries profiles from the gateway at build time and uses them to populate its own policy generation. Profiles replace embedded YAMLs as the data source, but assembly logic stays in cc-deck.

- Pros: Smaller change surface. Keeps cc-deck in control of policy assembly.
- Cons: Doesn't reduce coupling. cc-deck still owns policy assembly complexity. Two systems maintaining the same knowledge.

## Decision

**Approach A: Full Profile Delegation.** Aligns with the goal of leveraging the OpenShell ecosystem and reducing coupling. cc-deck becomes a profile selector, not a policy generator.

### Key design decisions made during brainstorming:

1. **Scope is full delegation**, not network-only. The gateway owns all policy aspects: network endpoints, filesystem rules, process configuration, and Landlock. cc-deck has no business telling the sandbox how to be a sandbox.

2. **Profile references replace SandboxPolicy.** cc-deck passes profile names when creating a sandbox. The gateway resolves them into the complete policy.

3. **Tool detection stays as a convenience layer.** Manifest declarations are the primary mechanism for specifying which profiles are needed. Auto-detection fills gaps for tools the user didn't explicitly list.

4. **MCP endpoints use ephemeral profiles.** User-specific MCP servers (private domains) are imported as temporary profiles via `client.Providers().Profiles().Import()`. The gateway resolves them uniformly alongside standard profiles.

5. **Gateway connectivity is required.** No offline fallback. OpenShell workspaces inherently need a gateway, so requiring it for profile resolution is not a new constraint.

## Key Requirements

- cc-deck MUST stop generating `SandboxPolicy` for OpenShell targets
- cc-deck MUST map detected tools and manifest declarations to OpenShell profile names
- cc-deck MUST create ephemeral profiles for user-specific MCP endpoints via the SDK `ProfileInterface`
- The OpenShell gateway MUST have profiles for all current policy components (anthropic, openai, python, node, go, rust, github, gitlab, docker, quay, vertexai) before this change ships
- Manifest declarations MUST take priority over auto-detection, with auto-detection filling gaps
- Compose/container environments (non-OpenShell targets) are NOT affected by this change

## Open Questions

- What does the profile name mapping table look like? Is it 1:1 with current domain groups, or a different granularity?
- How does auto-detection interact with manifest declarations? Union merge? Detection only for unlisted tools?
- What happens when a detected tool has no matching gateway profile? Warn and skip, or fail the build?
- Does the SDK `ProfileInterface.Import()` support ephemeral/temporary profiles, or do imported profiles persist?
- What changes are needed on the OpenShell gateway side to support profile-based policy resolution at sandbox creation time?
- How does this interact with the existing `manifest.Network.AllowedDomains` user-defined domain overrides?
