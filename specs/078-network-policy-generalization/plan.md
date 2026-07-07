# Implementation Plan: Network Policy Generalization

**Branch**: `078-network-policy-generalization` | **Date**: 2026-07-05 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/078-network-policy-generalization/spec.md`

## Summary

Generalize the network policy system to support multiple AI agents. Each agent adapter declares which domain groups it requires via a new `RequiredDomainGroups()` method on the Agent interface. The build system queries agent adapters for manifests that include an `agents` list, replacing the hardcoded `match: always: true` on `claude-code.yaml` and the `if comp.Key == "claude_code"` special case in `policy.go`. Shared ecosystem components (github, go, python, node, rust) remain unchanged. Backward compatibility is maintained by defaulting to Claude Code when no agents are specified.

## Technical Context

**Language/Version**: Go 1.25 (CLI), Rust stable wasm32-wasip1 (plugin)
**Primary Dependencies**: cobra v1.10.2 (CLI), zellij-tile 0.43.1 (plugin SDK), gopkg.in/yaml.v3
**Storage**: N/A (policy is generated at build time)
**Testing**: `make test` (Go tests via `go test ./...`), `make lint` (clippy + golangci-lint)
**Target Platform**: macOS/Linux CLI tool
**Project Type**: CLI + Zellij WASM plugin
**Performance Goals**: N/A (build-time tool)
**Constraints**: Backward compatibility with existing single-agent manifests
**Scale/Scope**: ~10 files changed across `internal/agent/`, `internal/build/`, `internal/network/`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Plan includes unit tests for new interface method, policy assembly tests for multi-agent, and doc updates |
| II. Interface contracts | PASS | Adding `RequiredDomainGroups()` to Agent interface with behavioral contract |
| III. Build and tool rules | PASS | Using `make test`/`make lint`, `internal/xdg`, `podman` |
| IV. Plugin debug logging | N/A | No plugin changes in this feature |
| V. Command files as code | N/A | No command file changes |

## Project Structure

### Documentation (this feature)

```text
specs/078-network-policy-generalization/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── agent-domain-groups.md
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/agent/
├── agent.go             # Agent interface (add RequiredDomainGroups method)
├── claude.go            # ClaudeAgent (implement RequiredDomainGroups)
├── opencode.go          # OpenCodeAgent (implement RequiredDomainGroups)
├── claude_test.go       # Tests for domain group declarations
└── opencode_test.go     # Tests for domain group declarations

cc-deck/internal/build/
├── manifest.go          # Manifest struct (add Agents field)
├── component.go         # MatchCondition (add Agents field)
├── policy.go            # Policy assembly (generalize MCP binary lookup)
├── policies/
│   ├── claude-code.yaml # Change match from always:true to agents:[claude]
│   └── vertex-ai.yaml   # Remains credential-matched (per research.md R5)
└── policy_test.go       # Multi-agent assembly tests

cc-deck/internal/network/
└── builtin.go           # No structural changes (shared groups remain)
```

**Structure Decision**: All changes occur within the existing `internal/agent/`, `internal/build/`, and `internal/network/` packages. No new packages or directories needed.

## Design Decisions

### D1: Agent Interface Extension

Add `RequiredDomainGroups() []string` to the `Agent` interface in `agent.go`. Each adapter returns the builtin group names it needs:

- `ClaudeAgent`: returns `["anthropic"]`
- `OpenCodeAgent`: returns `["openai"]` (OpenAI API domains, to be added as a new builtin group)

This is the only Agent interface change required. The method returns group name references, not raw domain lists.

### D2: Manifest `agents` Field

Add `Agents []string` to the `Manifest` struct. This is a list of agent names (matching `Agent.Name()` values). When empty or absent, defaults to `["claude"]` for backward compatibility.

### D3: Policy Component `match.agents` Field

Add `Agents []string` to the `MatchCondition` struct. A component matches if any declared agent name is present in the manifest's `agents` list. This is OR'd with existing `tools`, `credentials`, and `features` fields.

### D4: Claude Code Policy Component

Change `claude-code.yaml` from `match: always: true` to `match: agents: [claude]`. The component is included only when Claude Code is in the manifest's agent list.

### D5: MCP Binary Lookup Generalization

Replace the `if comp.Key == "claude_code"` check in `policy.go` with a loop over all agents in the manifest. For each agent, query the agent registry for binary paths. Merge all agent binaries into MCP policy entries.

This requires a new method or using `CredentialSpecs()` to discover binary paths. Since `CredentialSpecs()` doesn't expose binary paths, we add a `BinaryPaths() []string` method to the Agent interface, or use the existing policy component binaries from matched agent components.

**Decision**: Use policy component binaries from matched agent components. No new interface method needed for binary paths. The MCP code already has access to matched components; it just needs to collect binaries from all agent-matching components rather than just `claude_code`.

### D6: OpenAI Domain Group

Add a new `"openai"` builtin group in `builtin.go` for OpenCode's API domains. This follows the same pattern as the existing `"anthropic"` group.

### D7: Backward Compatibility

When `manifest.Agents` is empty (no `agents:` key in the YAML), the system defaults to `["claude"]`. This means:
- Existing manifests without an `agents` field produce identical policies
- The `claude-code.yaml` component still matches via `agents: [claude]`
- MCP binaries still resolve from the Claude component

## Global Constraints

These values are copied verbatim from the spec and apply to every task implicitly.

- **Go version**: 1.25 (from `go.mod`)
- **Backward compatibility**: SC-001 requires byte-identical policy output for single-agent Claude Code builds before and after this change
- **Build commands**: `make test`, `make lint`, `make verify`. Never run `go build` or `cargo build` directly.
- **XDG paths**: Use `internal/xdg` package (not `adrg/xdg`)
- **Container runtime**: `podman` exclusively (never Docker)
- **Domain declarations**: Group name references only. No inline domain lists in agent adapter code (per clarification session 2026-07-05).

## Complexity Tracking

No constitution violations. No complexity justification needed.
