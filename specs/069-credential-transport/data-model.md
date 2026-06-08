# Data Model: Credential Transport Abstraction

## Entities

### CredentialSpec (existing, in `internal/agent`)

Represents one auth mode for an agent. Declared statically at compile time. **No changes from v1.**

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | Auth mode identifier (e.g., "api", "vertex", "bedrock", "openai") |
| EnvVars | `[]EnvVarSpec` | Environment variables required by this mode |
| FileCredential | `*FileCredentialSpec` | Optional file-based credential (e.g., GCP service account JSON) |
| Endpoints | `[]Endpoint` | Network endpoints needed (for firewall/network policy) |
| UnsetVars | `[]string` | Env vars to explicitly unset in the workspace (e.g., GEMINI_API_KEY for Vertex mode) |
| Priority | `int` | Prompt ordering priority (lower = higher priority, shown first) |

### EnvVarSpec (existing, in `internal/agent`)

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | Environment variable name (e.g., "ANTHROPIC_API_KEY") |
| FixedValue | `string` | If non-empty, always inject this value (e.g., "1" for CLAUDE_CODE_USE_VERTEX) |
| Required | `bool` | If true, this var must be present for the auth mode to be "available" |

### FileCredentialSpec (existing, in `internal/agent`)

| Field | Type | Description |
|-------|------|-------------|
| EnvVar | `string` | Env var that points to the file path |
| DefaultPath | `string` | Default path to check if env var is unset (supports `~` expansion) |
| Required | `bool` | If true, the file must exist for the auth mode to be "available" |

### Endpoint (existing, in `internal/agent`)

| Field | Type | Description |
|-------|------|-------------|
| Host | `string` | Hostname (e.g., "oauth2.googleapis.com") |
| Port | `int` | Port number (e.g., 443) |

### DetectedMode (new, in `internal/credential`)

Represents a detected auth mode from any agent, used in the detect-all flow.

| Field | Type | Description |
|-------|------|-------------|
| AgentName | `string` | Name of the agent that declares this mode (e.g., "claude") |
| Spec | `CredentialSpec` | The credential spec for this mode |
| Resolved | `ResolvedCredentials` | The resolved credential values from the host environment |

### WorkspaceSpec (modified, in `internal/ws`)

**Changed fields** (compared to v1):

| Field | Type | YAML Key | Description | Change |
|-------|------|----------|-------------|--------|
| Agent | `string` | `agent` | ~~Agent name~~ | **REMOVE** (workspaces are not tied to a single agent) |
| AuthMode | `string` | `auth-mode` | ~~Selected auth mode~~ | **REMOVE** (all modes auto-detected) |
| ExternalCredentials | `bool` | `external-credentials` | If true, skip host-side credential validation | **KEEP** |

### WorkspaceInstance (modified, in `internal/ws`)

| Field | Change |
|-------|--------|
| Agent | **REMOVE** (no single-agent binding) |

### wsListEntry (modified, in `internal/cmd`)

| Field | Type | JSON Key | Description | Change |
|-------|------|----------|-------------|--------|
| Agent | `string` | `agent` | ~~Agent name~~ | **REMOVE** |
| AuthMode | `string` | `auth_mode` | ~~Selected auth mode~~ | **REMOVE** |
| Auth | `string` | `auth` | Comma-separated list of active auth modes (e.g., "claude/vertex, opencode/openai") | **MODIFY** (derived at display time) |

## Relationships

```text
Agent 1──* CredentialSpec       (agent declares multiple auth modes)
CredentialSpec 1──* EnvVarSpec  (each mode needs multiple env vars)
CredentialSpec 0──1 FileCredentialSpec (some modes need a file)
CredentialSpec 0──* Endpoint    (some modes need network access)
WorkspaceDefinition ──── ExternalCredentials (flag)
```

## State Transitions

### Credential Detection (during `ws new`)

```text
[Start] → For each registered agent:
       → Query agent.CredentialSpecs()
       → For each spec, check host environment availability
       → Collect all DetectedModes
       → Merge all modes into one credential set
       → Inject into workspace
```

### Credential Injection (at workspace start)

```text
[Start] → Load workspace definition
       → Check ExternalCredentials flag
       ├─ true → Skip validation
       └─ false → Detect all available modes
                → Validate all modes
                ├─ All present → Resolve and inject via transport
                └─ Missing → ERROR: name missing credentials
```
