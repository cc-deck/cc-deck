# Data Model: OpenShell Profile Delegation

## Entities

### ProfileMapping

A static table mapping cc-deck identifiers to OpenShell profile IDs.

| Field | Type | Description |
|-------|------|-------------|
| SourceType | enum(agent, tool, credential, always) | What kind of cc-deck identifier this is |
| SourceKey | string | The cc-deck identifier (e.g., "claude", "python", "vertex") |
| ProfileIDs | []string | OpenShell profile IDs this source maps to |

**Invariants:**
- Each (SourceType, SourceKey) pair maps to 1 or more ProfileIDs
- ProfileIDs are deduplicated when multiple sources map to the same profile
- "always" source type entries are included regardless of manifest content

### ProfileManifest

Embedded in OCI images at `/etc/openshell/profiles.yaml`. Lists the profiles required by the image.

| Field | Type | Description |
|-------|------|-------------|
| Profiles | []string | Ordered list of profile IDs required by this image |

**Invariants:**
- No duplicate entries
- Always includes "github" and "gitlab" (git hosting is mandatory)
- Order is deterministic (sorted alphabetically) for reproducible builds

### EphemeralProfile

A custom profile imported to the gateway for user-specific endpoints.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Deterministic name: `cc-deck-<workspace>-<type>` |
| DisplayName | string | Human-readable label |
| Category | ProfileCategory | "Other" for MCP/custom, matching category for domain overrides |
| Endpoints | []NetworkEndpoint | Host, Port, Protocol tuples |
| Binaries | []NetworkBinary | Binary paths (for MCP, from agent component binaries) |

**Naming patterns:**
- MCP endpoints: `cc-deck-<workspace-name>-mcp`
- Custom domain overrides: `cc-deck-<workspace-name>-custom`

**Workspace name sanitization:**
- Lowercase alphanumeric and hyphens only
- Truncated to 50 characters
- Invalid characters replaced with hyphens

## Relationships

```
Manifest.Agents ──→ ProfileMapping ──→ ProfileManifest.Profiles
Manifest.Tools  ──→ ProfileMapping ──→ ProfileManifest.Profiles
Auto-detection  ──→ ProfileMapping ──→ ProfileManifest.Profiles

ProfileManifest.Profiles ──→ Provider (Type = profile ID)
Manifest.MCP             ──→ EphemeralProfile ──→ Provider
Manifest.AllowedDomains  ──→ EphemeralProfile ──→ Provider

Provider ──→ SandboxSpec.Providers
```

## State Transitions

### Build Phase
```
Manifest + AutoDetection → ProfileMapping lookup → ProfileManifest → embedded in OCI image
```

### Workspace Creation Phase
```
OCI image → extract ProfileManifest → verify profiles on gateway → create providers → SandboxSpec
Manifest.MCP → import EphemeralProfile → create provider → SandboxSpec
Manifest.AllowedDomains → import EphemeralProfile → create provider → SandboxSpec
```
