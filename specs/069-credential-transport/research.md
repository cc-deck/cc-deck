# Research: Credential Transport Abstraction

## R-001: Current credential handling locations

**Finding**: Credential handling is duplicated across three separate modules with Claude-specific hardcoding:

1. **`internal/ws/auth.go`** (container/compose workspaces): `DetectAuthMode()` checks `CLAUDE_CODE_USE_VERTEX`, `CLAUDE_CODE_USE_BEDROCK`, `ANTHROPIC_API_KEY`. `DetectAuthCredentials()` populates env var maps per mode (api/vertex/bedrock). Used by container.go, compose.go.

2. **`internal/ssh/credentials.go`** (SSH workspaces): Independent `detectAuthMode()` function (lowercase, package-private) with identical logic. `BuildCredentialSet()` resolves credentials, `WriteCredentialFile()` writes them to remote host, `CopyCredentialFile()` handles file-based credentials.

3. **`internal/openshell/credentials.go`** (OpenShell workspaces): `KnownProviderProfiles` map with hardcoded entries for claude, claude-vertex, anthropic, github, gitlab, openai, nvidia, vertex, generic. `ResolveCredentials()` and `DetectCredentials()` use this map.

**Decision**: All three converge into the new `internal/credential` package.
**Rationale**: Eliminates duplication and makes credential handling agent-aware.

## R-002: Agent interface extension pattern

**Finding**: The current `Agent` interface in `internal/agent/agent.go` has 9 methods covering identity, installation, hooks, and event translation. Adding `CredentialSpecs()` follows the existing pattern of compile-time agent registration via `init()`.

**Decision**: Add `CredentialSpecs() []CredentialSpec` to the Agent interface.
**Rationale**: Every agent implementation must be updated (ClaudeAgent, OpenCodeAgent), but since agents register at compile time via `init()`, the compiler enforces completeness.
**Alternatives considered**: Optional interface (type assertion) was rejected because credential specs are fundamental, not optional.

## R-003: WorkspaceSpec has no agent field

**Finding**: `WorkspaceSpec` in `definition.go` has `Auth string` and `Credentials []string` but no `Agent` field. The `Auth` field is currently a string matching `AuthMode` constants (auto/none/api/vertex/bedrock). The workspace command (`cmd/ws.go`) resolves auth mode at creation time and passes it to workspace backends.

**Decision**: Add `Agent string` field to `WorkspaceSpec` (YAML: `agent`). The `Auth` field is repurposed to store the selected CredentialSpec name (which may be "api", "vertex", "bedrock" for Claude, or "openai", "anthropic" for OpenCode).
**Rationale**: The agent field links the workspace to the correct set of credential specs. Without it, credential resolution cannot determine which agent's specs to use.

## R-004: Workspace listing output format

**Finding**: `wsListEntry` struct has fields: Name, Type, Infra, Session, Project, Storage, Image, LastAttached, Age. No auth mode or agent field. Table output is built in `writeWsStructured()`.

**Decision**: Add `Agent` and `AuthMode` fields to `wsListEntry`. Display as combined "agent/mode" column (e.g., "claude/vertex", "opencode/openai").
**Rationale**: Compact display that gives both agent and credential info at a glance.

## R-005: File permission handling across workspace types

**Finding**: 
- SSH: `WriteCredentialFile` already uses `chmod 600` on remote credential files.
- Container: Podman secrets are mounted read-only at `/run/secrets/`.
- K8s: Secrets have default 0644 in pod mounts; needs `defaultMode: 0o600` in volume spec.
- OpenShell: `UploadFileCredential` writes to sandbox but doesn't set permissions explicitly.

**Decision**: Standardize all credential file operations to 0600. Add permission setting to OpenShell upload. K8s Secret volume mounts specify `defaultMode: 0o600`.
**Rationale**: Consistent security posture per FR-017.

## R-006: OpenShell KnownProviderProfiles replacement

**Finding**: `KnownProviderProfiles` maps credential type names to `KnownProviderProfile` structs containing DetectVars, RequiredVars, ExtraEnvVars, FileVar, Endpoints, and Type. The `ResolveCredentials()` function uses these profiles to resolve env vars and determine provider types. The `DetectCredentials()` function scans the host for available credentials.

**Decision**: Replace `KnownProviderProfiles` with agent-declared `CredentialSpec` data. The credential package provides equivalent `Resolve()` and `Detect()` functions that work with any agent's specs. OpenShell-specific concerns (provider type mapping, sandbox rc injection) remain in the openshell package but delegate resolution to the credential package.
**Rationale**: Per FR-013, removes the hardcoded map while preserving OpenShell-specific provider creation logic.

## R-007: Priority and auto-selection behavior

**Finding**: Per clarification, priority is an integer that controls prompt ordering and default suggestion when multiple auth modes are available. It does NOT cause silent auto-selection. When only one mode is available, it is auto-selected without prompting.

**Decision**: CredentialSpec.Priority is `int` (lower = higher priority). Used to sort available modes in the prompt. The first (highest priority) mode is marked as default.
**Rationale**: Consistent with clarification Q1.

## R-008: Single-agent vs. multi-agent workspace model

**Finding**: The initial implementation tied each workspace to a single agent via `--agent` flag and `Agent`/`AuthMode` fields in the workspace definition. However, workspaces can host sessions from multiple agents simultaneously (e.g., Claude Code and OpenCode in the same container). A single-agent binding prevents multi-agent credential injection.

**Decision**: Detect-all model. At workspace creation, scan all registered agents, detect all available credentials, inject everything. No conflict resolution or exclusion mechanism.
**Rationale**: The common case (single credential set) requires zero user interaction. Multi-agent setups work automatically. Agents like OpenCode can hold multiple API keys simultaneously, so injecting everything maximizes flexibility.
**Alternatives considered**: (1) Single-agent binding with `--agent` flag (rejected: blocks multi-agent workspaces). (2) Detect-all with conflict resolution and `--exclude` (rejected as premature: adds complexity without proven need).
