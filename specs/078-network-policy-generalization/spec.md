# Feature Specification: Network Policy Generalization

**Feature Branch**: `078-network-policy-generalization`
**Created**: 2026-07-05
**Status**: Review
**Input**: Brainstorm 068 - Network Policy Generalization for Multi-Agent Support

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent-Aware Policy Composition (Priority: P1)

A developer building a container image that includes multiple AI agents (for example, Claude Code and OpenCode) needs the generated network policy to automatically include the correct domain allowlists for each agent. Today, Claude Code domains are unconditionally included regardless of which agents are configured. The developer adds agents to the build manifest and expects the policy to reflect exactly those agents' network requirements, nothing more and nothing less.

**Why this priority**: Without agent-aware composition, multi-agent images either have incorrect network policies (missing required domains) or overly permissive policies (including domains for agents not present). This is the core capability that all other stories depend on.

**Independent Test**: Build an image with two agents declared in the manifest, inspect the generated policy, and confirm each agent's required domains appear while no extraneous agent domains are present.

**Acceptance Scenarios**:

1. **Given** a manifest listing agents "claude" and "opencode", **When** the policy is assembled, **Then** the output includes network entries for both agents' required domains and excludes domains belonging to agents not listed.
2. **Given** a manifest listing only agent "claude", **When** the policy is assembled, **Then** the output is identical to the current single-agent behavior (backward compatibility).
3. **Given** a manifest with no agents listed, **When** the policy is assembled, **Then** the system defaults to including Claude Code domains for backward compatibility.

---

### User Story 2 - Agent Domain Group Declaration (Priority: P1)

An agent adapter developer adding support for a new AI agent (such as Gemini CLI) needs to declare which domain groups the agent requires. The developer implements the agent interface and specifies domain group names. The build system queries these declarations and includes the corresponding domain endpoints in the generated policy.

**Why this priority**: This is the mechanism that enables agent-aware composition. Without it, adding a new agent requires hardcoding domain lists in the build system rather than letting the agent own its requirements.

**Independent Test**: Register a test agent that declares two domain groups, build a policy, and confirm the declared groups' endpoints appear in the output.

**Acceptance Scenarios**:

1. **Given** an agent adapter that declares domain groups "anthropic" and "vertexai", **When** the policy is assembled for a manifest including that agent, **Then** endpoints from both groups appear in the policy.
2. **Given** an agent adapter that declares a domain group not present in the builtin groups, **When** the policy is assembled, **Then** the system reports a clear warning identifying the missing group.

---

### User Story 3 - Removal of Hardcoded Claude Code Coupling (Priority: P2)

A maintainer working on the build system encounters the `match: always: true` directive in the Claude Code policy component and the `if comp.Key == "claude_code"` special case in the policy assembly code. These hardcoded assumptions prevent clean multi-agent support. The maintainer needs these coupling points replaced with the generalized agent-driven mechanism so that no single agent receives special treatment in the codebase.

**Why this priority**: While the system works for Claude-only builds today, these coupling points create maintenance burden and block clean multi-agent builds. Removing them is prerequisite work for a clean architecture.

**Independent Test**: Search the codebase after the change for any remaining `"claude_code"` string literals outside of the Claude agent adapter itself and the Claude-specific policy YAML file. None should exist in the build assembly logic.

**Acceptance Scenarios**:

1. **Given** the current `claude-code.yaml` with `match: always: true`, **When** the generalization is applied, **Then** the policy component uses agent-based matching instead of unconditional inclusion.
2. **Given** the current `policy.go` with `if comp.Key == "claude_code"`, **When** the generalization is applied, **Then** MCP endpoint processing uses the agent interface to locate binaries for any agent, not just Claude Code.

---

### User Story 4 - Per-Agent Domain Group Configuration (Priority: P3)

A power user who has deployed a custom MCP server on a private domain needs to associate additional domain groups with a specific agent in their workspace configuration. The user adds a domain group entry scoped to a particular agent, and the build system merges it with the agent's built-in domain requirements.

**Why this priority**: Most users will rely on the agent's built-in domain declarations. Custom per-agent domain configuration is a power-user feature that extends the base mechanism.

**Independent Test**: Add a custom domain group scoped to agent "claude" in the workspace config, build the policy, and confirm the custom domain appears alongside Claude's built-in domains.

**Acceptance Scenarios**:

1. **Given** a workspace config with a custom domain group scoped to agent "claude", **When** the policy is assembled, **Then** the custom domain group endpoints are included in the policy alongside the agent's built-in domains.
2. **Given** a workspace config with a custom domain group scoped to agent "opencode", **When** agent "opencode" is not in the manifest, **Then** the custom domain group is excluded from the policy.

---

### Edge Cases

- What happens when an agent declares a domain group that does not exist in the builtin groups or user-defined groups? The system should warn and skip the missing group without failing the build.
- What happens when two agents declare overlapping domain groups (both need "github")? The group should appear once in the policy with no duplicates.
- What happens when the manifest has no `agents` field at all? Backward compatibility: default to Claude Code behavior.
- What happens when a user-defined domain group has the same name as a builtin group? The user-defined group should take precedence (existing domain group resolution behavior).
- What happens when MCP endpoints are configured but the agent providing binaries is not in the manifest? The system should warn that MCP policies cannot be created without a matching agent.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each agent adapter MUST declare its required domain groups through the Agent interface
- **FR-002**: The policy assembly system MUST query agent adapters for required domain groups and include only those groups that match agents present in the manifest
- **FR-003**: The `match: always: true` directive MUST be removed from the Claude Code policy component and replaced with agent-based matching. Shared ecosystem policy components (git-hosting, go, python, node, rust) MUST retain their `always: true` matching because they serve all agents, not a specific one.
- **FR-004**: The `if comp.Key == "claude_code"` special case in policy assembly MUST be replaced with a generic mechanism that queries any agent for its binary paths
- **FR-005**: MCP endpoint processing MUST collect binaries from all agent-matched policy components (those with `match.agents` set) and merge them into each MCP policy entry. Binary paths come from the policy component YAML files, not from the Agent interface.
- **FR-006**: When multiple agents are in the manifest, their domain groups MUST be merged with deduplication (each unique domain appears once)
- **FR-007**: When no agents are specified in the manifest, the system MUST default to Claude Code behavior for backward compatibility
- **FR-008**: Users MUST be able to associate additional domain groups with a specific agent in the workspace configuration
- **FR-009**: Agent-specific domain groups (e.g., anthropic, openai) MUST be declared by the agent adapter that requires them. Multiple agents MAY declare the same domain group if they share that dependency. Shared ecosystem groups (e.g., github, python, nodejs) MUST remain as shared builtin groups available to all agents.
- **FR-010**: Agent domain group declarations MUST use group name references only. Domain endpoint details MUST remain in policy YAML files and the builtin groups map, not inline in agent adapter code.
- **FR-011**: The system MUST produce identical policy output for single-agent Claude Code builds before and after this change (backward compatibility)

### Key Entities

- **Agent Domain Declaration**: The set of domain group names an agent requires. Owned by the agent adapter. Each declaration is a list of group name references.
- **Domain Group**: A named collection of domain patterns (existing entity). Groups can come from builtin definitions, user configuration, or agent declarations.
- **Policy Component**: A YAML file defining endpoints, binaries, and match conditions for a specific software component (existing entity). The match condition changes from `always: true` to agent-based matching.
- **Manifest Agent List**: The list of agents declared in the build manifest. Determines which agents' domain groups are included in the assembled policy.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A build with only Claude Code configured produces a policy byte-identical to the current output (zero regression)
- **SC-002**: Adding a second agent to the manifest adds exactly that agent's domain groups to the policy, with no manual domain configuration required
- **SC-003**: No hardcoded agent-specific logic remains in the policy assembly code (zero `if comp.Key == "claude_code"` or equivalent checks)
- **SC-004**: Adding a new agent adapter requires implementing only the Agent interface methods; no changes to the build system or policy assembly code are needed
- **SC-005**: All existing tests continue to pass. Tests that assert `always: true` matching or hardcoded `claude_code` key lookups may be updated to reflect agent-based matching, but no test logic unrelated to the matching mechanism requires changes.

## Clarifications

### Session 2026-07-05

- Q: Should shared ecosystem domain groups (github, python, nodejs) remain as shared builtin groups, or move to agent declarations? → A: Shared ecosystem groups remain builtin. Only agent-API groups (anthropic, openai, vertexai) move to agent declarations. Agents reference shared groups by name.
- Q: Should agents declare inline domain lists in Go code, or only reference group names? → A: Group name references only. Domain details stay in YAML policy files and the builtin groups map. No inline domain lists in agent adapter code.

## Assumptions

- The Agent interface from spec 066 is fully implemented and stable. No changes to the core interface contract are needed beyond adding the domain group declaration method.
- The existing domain group system (`internal/network/`) and policy component system (`internal/build/policies/`) are stable and will be extended, not replaced.
- The build manifest schema can be extended to include an `agents` field without breaking existing manifests that lack this field.
- The OpenCode agent adapter (already implemented) will serve as the validation target alongside Claude Code for multi-agent policy composition testing.
- Deny rules (blocking specific domains per agent) are out of scope for this feature and will be addressed in a future specification if needed.
