# Brainstorm: Network Policy Generalization for Multi-Agent Support

**Date:** 2026-06-06
**Status:** active
**Depends on:** 066-agent-abstraction (core Agent interface must land first)
**Extracted from:** 022-multi-agent-support

## Problem Framing

The network policy system currently assumes Claude Code as the only agent. The `claude_code` component in `internal/build/policies/claude-code.yaml` has `match: always: true`, meaning it is unconditionally included in every image build. Hardcoded Anthropic domains live in `internal/network/builtin.go`. The `internal/build/policy.go` has an explicit `if comp.Key == "claude_code"` check.

When multiple agents are supported, each agent needs its own set of allowed domains (OpenAI endpoints for Codex, Google endpoints for Gemini, etc.), and the "always include" assumption must be replaced with agent-aware policy composition.

## Prior Analysis (from brainstorm 022)

### Current Coupling Points

- `internal/build/policies/claude-code.yaml`: Contains api.anthropic.com, claude.ai, platform.claude.com, statsig.anthropic.com with `match: always: true`
- `internal/network/builtin.go:7-20`: Hardcoded `"anthropic"` domain group with 8 domains
- `internal/build/policy.go:251`: Explicit `if comp.Key == "claude_code"` logic
- MCP endpoint processing assumes claude_code binaries exist (lines 248-254, 285)

### Per-Agent Domain Aliases (from paude analysis)

paude's approach: each agent declares `extra_domain_aliases` (e.g., Claude adds `["claude"]`, Gemini adds `["gemini", "nodejs"]`). When the user requests `"default"` domains, the expansion merges `BASE_ALIASES` with the agent's extras. The network filter adapts automatically to the agent type.

cc-deck's domain group system should adopt this: the Agent interface declares which domain groups it needs, and the workspace definition merges them with user-specified groups.

## Approaches Considered

### A: Agent Interface Declares Domain Groups

Each Agent adapter adds a `RequiredDomainGroups() []string` method. The build system queries this for each agent in the manifest and composes the policy from the union of all declared groups.

- Pros: Clean separation. Each agent owns its domain requirements. Policy composition is automatic.
- Cons: Adding a domain for an existing agent requires a code change (Go adapter update + rebuild).

### B: Per-Agent Policy YAML Files

Ship `policies/claude-code.yaml`, `policies/codex.yaml`, `policies/gemini-cli.yaml` etc. The build system includes policy files matching the manifest's `agents` list.

- Pros: Easy to add/modify domains without touching Go code. Users can override by placing custom policy files.
- Cons: Two sources of truth (policy YAML + Agent interface). Must keep them in sync.

### C: Hybrid (Recommended)

Agent interface declares base domain groups via code. Policy YAML files provide the detailed endpoint lists per group. The build system merges: agent-declared groups determine which YAML files to include.

- Pros: Agent ownership of requirements (code) + detailed endpoint management (YAML). Decoupled.
- Cons: Slightly more complex composition logic.

## Decision

Approach C (Hybrid) is recommended but not yet confirmed. To be finalized during specification.

## Key Requirements

- Remove `match: always: true` from the claude_code policy component
- Each agent declares its required domain groups via the Agent interface
- Policy YAML files exist per domain group, not per agent (agents reference groups)
- The builtin.go hardcoded domain map must become agent-driven
- The `if comp.Key == "claude_code"` special case in policy.go must be generalized
- MCP endpoint processing must work for any agent's binary paths, not just Claude's
- Backward compatibility: existing builds with only Claude Code must produce identical policies

## Open Questions

- Should users be able to add custom domain groups per agent in the workspace definition?
- How do per-agent domain aliases interact with the existing `domains.yaml` user config?
- Should the policy system support "deny" rules per agent (e.g., block OpenAI endpoints when running Claude only)?
