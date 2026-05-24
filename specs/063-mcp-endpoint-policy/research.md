# Research: MCP Endpoint Policy Integration

## Decision 1: Slugification function for MCP names

**Decision**: Create a separate `slugifyMCPName()` function distinct from the existing `slugify()`.

**Rationale**: The existing `slugify()` (policy.go:421) converts domain names by replacing only dots with underscores, preserving hyphens to avoid collisions between domains that differ only in dot/hyphen. MCP server names use hyphens (e.g., `google-work`) and do not contain dots, so a different function is needed: one that replaces hyphens, spaces, and non-alphanumeric characters with underscores and lowercases.

**Alternatives considered**:
- Modifying existing `slugify()`: Rejected because it would break domain-based slug collisions (test T599 verifies this behavior).
- Using the existing `slugify()` as-is: Would produce `google-work` instead of `google_work`, which is technically valid YAML but inconsistent with the `mcp_` prefix convention.

## Decision 2: Binary source for MCP policy entries

**Decision**: Load the `claude_code` component's explicit binaries from the embedded `claude-code.yaml` component file during MCP policy generation.

**Rationale**: The `claude-code.yaml` component (policies/claude-code.yaml) has `match.always: true` and explicit `binaries` entries. These are the exact paths Claude Code uses. Since MCP servers are reached by Claude Code processes, the same binary paths apply. The component is always present in `matched` (due to `always: true`), so we can find it by key lookup.

**Alternatives considered**:
- Hardcoding binary paths: Rejected because the paths already live in `claude-code.yaml` and should be the single source of truth.
- Adding a new component YAML per MCP server: Rejected because MCP servers are dynamic (user-configured), not static catalog entries.

## Decision 3: Where to insert MCP processing in assemblePolicyCore

**Decision**: Add MCP endpoint processing after the generic credential endpoint block (policy.go line ~242), before the final return statement.

**Rationale**: This follows the existing pattern where different manifest sections (components, domains, credentials) each add their entries to the `networkPolicies` map. MCP entries are another category, so they belong in the same sequence. Placing them after credentials keeps the flow logical: components -> domains -> credentials -> MCP.

**Alternatives considered**:
- Adding as a new PolicyComponent type: Rejected because MCP entries are dynamic per-manifest, not static catalog entries.
- Processing during component matching: Rejected because MCP entries are not components; they are manifest-level declarations.

## Decision 4: pkg_node binary augmentation approach

**Decision**: After building `networkPolicies`, check if `pkg_node` exists as a key. If so, find the `claude_code` component's binaries and append them to `pkg_node`'s binary list.

**Rationale**: The `pkg_node` component (node.yaml) covers npm registry access. When Claude Code spawns `npx` for MCP servers, the supervisor sees Claude Code binaries accessing npm endpoints. Without augmentation, the supervisor blocks this because `pkg_node` only allows node/npm/npx binaries (from probe), not Claude Code binaries.

**Alternatives considered**:
- Always augmenting `pkg_node` even without MCP entries: Rejected because augmentation is only needed when MCP servers use npm-based proxies. However, since we cannot tell from the manifest alone whether MCP servers use npm, and the cost of false positives is zero (extra allowed binaries for a registry that is already allowed), we augment whenever `pkg_node` is present and MCP entries exist.

## Decision 5: Capture command endpoint extraction

**Decision**: Extend Step 9 of the capture command to extract endpoints from MCP server configurations and present them for user confirmation.

**Rationale**: The capture command already discovers MCP servers from Claude Code settings. Adding endpoint extraction is a natural extension. HTTP/SSE servers have explicit URLs; stdio servers using `mcp-remote` have URLs in their arguments.

**Alternatives considered**:
- Separate capture step for endpoints: Rejected because endpoints are a property of MCP servers, not a separate concept.
- Automatic endpoint addition without confirmation: Rejected because the user should verify extracted endpoints (they may be wrong, especially for complex mcp-remote configurations).
