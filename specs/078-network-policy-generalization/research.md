# Research: Network Policy Generalization

## R1: Current Match Condition System

**Decision**: Extend `MatchCondition` with an `Agents` field rather than repurposing existing fields.

**Rationale**: The existing `MatchCondition` uses OR semantics across `Always`, `Tools`, `Credentials`, and `Features`. Adding `Agents []string` follows the same pattern. A component matches if any of its declared agent names appears in the manifest's agent list. This is orthogonal to tool/credential matching and allows agent-specific components (like `claude-code.yaml`) to coexist with tool-specific ones (like `go.yaml`).

**Alternatives considered**:
- Repurposing `Tools` field to match agent names: Rejected because tool matching searches within tool names (substring match), while agent matching needs exact name match. Different semantics.
- Adding a new `AgentMatch` struct: Rejected as over-engineering. A string slice on the existing struct is sufficient.

## R2: MCP Binary Resolution Without Hardcoded Agent Key

**Decision**: Collect binaries from all agent-matching policy components rather than looking up a specific component key.

**Rationale**: The current code does `if comp.Key == "claude_code" { claudeCodeBinaries = comp.Binaries }`. The generalized version iterates over all matched components, identifies which ones matched via the `agents` field, and collects their binaries. When multiple agents contribute binaries, they are merged (deduplicated by path).

**Alternatives considered**:
- Adding a `BinaryPaths()` method to the Agent interface: Rejected because binary paths are already declared in policy component YAML files. The agent adapter shouldn't duplicate this information.
- Using `CredentialSpecs()` endpoint data: Rejected because credential specs describe auth mechanisms, not sandbox binary paths.

## R3: OpenAI Domain Group for OpenCode

**Decision**: Add `"openai"` builtin group with OpenAI API domains.

**Rationale**: OpenCode uses the OpenAI API. The minimal domain set is `api.openai.com` and related CDN domains. This follows the same pattern as the existing `"anthropic"` group.

**Domains**: `api.openai.com`, `.openai.com`, `.oaiusercontent.com`

**Alternatives considered**:
- Letting OpenCode declare inline domains: Rejected per clarification (group name references only).
- Using a user-defined group: Rejected because OpenCode's API domains are stable and should ship as builtin.

## R4: Manifest `agents` Default Behavior

**Decision**: When `manifest.Agents` is nil or empty, default to `["claude"]`.

**Rationale**: All existing manifests lack an `agents` field. Defaulting to Claude ensures zero behavior change. The default is applied at policy assembly time, not during manifest parsing, so the serialized manifest is unchanged.

**Alternatives considered**:
- Requiring explicit `agents:` in manifests: Rejected because it would break all existing manifests.
- Defaulting to all registered agents: Rejected because it would include unwanted agent domains.

## R5: Vertex AI Component Matching

**Decision**: Keep `vertex-ai.yaml` matching on `credentials: [claude-vertex]` and do not change to agent-based matching.

**Rationale**: Vertex AI is a credential-based backend, not an agent. A user might use Claude via Vertex in one build and via direct API in another, both with the same agent "claude". The credential match is the correct semantic: include Vertex domains when Vertex credentials are configured, regardless of which agent uses them.

**Alternatives considered**:
- Moving to `agents: [claude]`: Rejected because Vertex is not Claude-specific. Future agents might also use Vertex AI.
