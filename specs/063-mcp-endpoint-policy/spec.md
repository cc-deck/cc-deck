# Feature Specification: MCP Endpoint Policy Integration

**Feature Branch**: `063-mcp-endpoint-policy`
**Created**: 2026-05-24
**Status**: Draft
**Input**: User description: "Add MCP server endpoints to the OpenShell network policy automatically during policy assembly"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Policy Assembly Includes MCP Endpoints (Priority: P1)

A user configures MCP servers in their build manifest with endpoint information. When they run the policy assembly, the generated network policy automatically includes entries for each MCP server that has an endpoint defined. Claude Code can then connect to these MCP servers at startup without being blocked by the OpenShell supervisor.

**Why this priority**: Without this, Claude Code hangs at startup because MCP server connections are denied by the supervisor. This is the core problem being solved.

**Independent Test**: Can be fully tested by assembling a policy from a manifest containing MCP entries with endpoints and verifying the generated policy contains the correct network entries with host, port, and binary paths.

**Acceptance Scenarios**:

1. **Given** a manifest with an MCP entry that has a non-empty endpoint (e.g., `mcp-google-work.int-tichny.org:8443`), **When** the policy is assembled, **Then** the output contains a network policy entry keyed as `mcp_<slugified-name>` with the correct host and port.
2. **Given** a manifest with an MCP entry that has no endpoint (e.g., a local stdio server like `playwright`), **When** the policy is assembled, **Then** no policy entry is generated for that MCP server.
3. **Given** a manifest with multiple MCP entries, some with endpoints and some without, **When** the policy is assembled, **Then** only entries with endpoints produce policy entries, and all generated entries include the correct binary paths from the Claude Code component.

---

### User Story 2 - Capture Command Extracts MCP Endpoints (Priority: P2)

A user runs the capture command to discover MCP servers from their Claude Code settings. The capture process extracts endpoint URLs from server configurations and presents them for confirmation before writing them into the manifest.

**Why this priority**: Capture is how users populate the manifest. Without endpoint extraction, users would need to manually determine and enter host:port values for each MCP server.

**Independent Test**: Can be tested by running capture against a Claude Code settings file containing HTTP and stdio MCP server configurations and verifying that endpoints are correctly extracted and presented for confirmation.

**Acceptance Scenarios**:

1. **Given** an HTTP/SSE MCP server with a `url` field (e.g., `https://mcp-google-work.int-tichny.org:8443/mcp`), **When** the capture command processes this server, **Then** the endpoint is extracted as `mcp-google-work.int-tichny.org:8443`.
2. **Given** a stdio MCP server using `mcp-remote` with a URL in its arguments (e.g., `npx @anthropic-ai/mcp-remote https://mcp.atlassian.com:443`), **When** the capture command processes this server, **Then** the endpoint is extracted as `mcp.atlassian.com:443`.
3. **Given** a local stdio MCP server with no URL in its arguments (e.g., `playwright`), **When** the capture command processes this server, **Then** no endpoint is extracted.
4. **Given** an extracted endpoint, **When** presented to the user during capture, **Then** the user can confirm, modify, or reject the endpoint before it is written to the manifest.

---

### User Story 3 - Node Binary Augmentation for NPM-based MCP Servers (Priority: P3)

When the policy includes the `pkg_node` component (for npm registry access), the Claude Code binaries are appended to its binary list. This allows Claude Code to spawn `npx` processes that download and run npm packages for stdio MCP servers (e.g., `@anthropic-ai/mcp-remote`).

**Why this priority**: This is a secondary concern that only applies when npm-based MCP servers are in use. Without it, Claude Code can reach MCP endpoints but cannot install the npm packages needed to run stdio MCP proxies.

**Independent Test**: Can be tested by assembling a policy where `pkg_node` is a matched component and verifying that the Claude Code binary paths are appended to the `pkg_node` component's binary list.

**Acceptance Scenarios**:

1. **Given** a policy assembly where `pkg_node` is a matched component, **When** the policy is assembled, **Then** the Claude Code component's binary paths are appended to the `pkg_node` component's binary list.
2. **Given** a policy assembly where `pkg_node` is NOT a matched component, **When** the policy is assembled, **Then** no binary augmentation occurs.

---

### Edge Cases

- What happens when an MCP endpoint has no explicit port? The `endpoint` field requires `host:port` format. The capture command always extracts both values, so a missing port in the manifest is a validation error and the entry should be skipped with a warning.
- What happens when an MCP entry has a malformed endpoint string? The system should skip the entry and log a warning rather than failing the entire policy assembly.
- What happens when two MCP servers share the same endpoint host but different ports? Each should produce its own separate policy entry with a unique key.
- What happens when a manifest has no MCP entries at all? Policy assembly should proceed normally with no MCP-related entries, maintaining backward compatibility.

## Clarifications

### Session 2026-05-24

- Q: How should the MCP server name be slugified for the policy key `mcp_<slugified-name>`? → A: Replace hyphens, spaces, and non-alphanumeric characters with underscores, then lowercase. For example, `google-work` becomes `google_work`.
- Q: Is the port always required in the `endpoint` field, or can it be omitted to use protocol defaults? → A: Port is always required. The `host:port` format is mandatory. The capture command always extracts both host and port explicitly, so there is no ambiguity at policy assembly time.
- Q: What happens if the Claude Code component definition (`claude-code.yaml`) is not found during policy assembly? → A: Skip MCP policy entry generation entirely and log a warning. Do not fail the assembly. The remaining policy entries are still valid without MCP entries.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The manifest schema MUST support an optional `endpoint` field on MCP entries in `host:port` format.
- **FR-002**: Policy assembly MUST generate a network policy entry for each MCP entry that has a non-empty endpoint.
- **FR-003**: Each MCP policy entry MUST be keyed as `mcp_<slugified-name>` where the name is derived from the MCP server's name field.
- **FR-004**: Each MCP policy entry MUST include the endpoint host and port as a single endpoint entry.
- **FR-005**: Each MCP policy entry MUST include the Claude Code component's binary paths in its binary list.
- **FR-006**: MCP entries without an endpoint MUST NOT produce any policy entry.
- **FR-007**: The capture command MUST extract endpoint URLs from HTTP/SSE MCP servers by parsing the `url` field.
- **FR-008**: The capture command MUST extract endpoint URLs from stdio MCP servers that use `mcp-remote` by scanning arguments for HTTPS URLs.
- **FR-009**: The capture command MUST present extracted endpoints to the user for confirmation before writing them to the manifest.
- **FR-010**: Policy assembly MUST append Claude Code binary paths to the `pkg_node` component's binary list when `pkg_node` is a matched component.
- **FR-011**: Existing manifests without MCP endpoint fields MUST continue to work without modification (backward compatibility).
- **FR-012**: The `endpoint` field MUST only affect OpenShell targets. Container and SSH targets MUST remain unaffected.
- **FR-013**: Slugification of the MCP server name for the policy key MUST replace hyphens, spaces, and non-alphanumeric characters with underscores and lowercase the result (e.g., `google-work` becomes `mcp_google_work`).
- **FR-014**: If the Claude Code component definition is not available during policy assembly, MCP policy entry generation MUST be skipped with a warning. The assembly MUST NOT fail.
- **FR-015**: MCP entries with a malformed `endpoint` value (missing port or unparseable format) MUST be skipped with a warning rather than failing the entire policy assembly.

### Key Entities

- **MCP Entry**: Represents a configured MCP server with name, transport type, optional endpoint, and description. An MCP entry with an endpoint produces a network policy entry; one without an endpoint is skipped during policy assembly.
- **Network Policy Entry**: A generated policy rule allowing specific binaries to reach specific network endpoints. Keyed by a slug derived from the MCP server name.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Claude Code starts successfully in the sandbox when all configured MCP servers have their endpoints in the network policy, with no DENIED log entries for MCP connections.
- **SC-002**: Policy assembly completes in the same time frame as before (no measurable performance regression) when processing manifests with up to 10 MCP entries.
- **SC-003**: Users can capture MCP server configurations and have endpoints automatically extracted and confirmed in a single capture session, without needing to manually look up host:port values.
- **SC-004**: Existing manifests without MCP endpoint fields produce identical policy output as before the change (zero behavioral regression).

## Assumptions

- MCP servers configured in the manifest have stable, known endpoints that do not change dynamically at runtime.
- The Claude Code component definition (`claude-code.yaml`) is expected to be available during policy assembly. If missing, FR-014 defines the fallback behavior.
- The `host:port` format is sufficient for all MCP endpoint specifications (no path-based routing is needed at the network policy level).
- The `pkg_node` component, when present, already handles npm registry endpoint resolution. Only the binary list needs augmentation.
- This feature only affects OpenShell targets. Container and SSH targets do not use the supervisor-based network policy mechanism.
