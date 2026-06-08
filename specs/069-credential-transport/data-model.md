# Data Model: Credential Transport Abstraction

## Entities

### CredentialSpec (new, in `internal/agent`)

Represents one auth mode for an agent. Declared statically at compile time.

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | Auth mode identifier (e.g., "api", "vertex", "bedrock", "openai") |
| EnvVars | `[]EnvVarSpec` | Environment variables required by this mode |
| FileCredential | `*FileCredentialSpec` | Optional file-based credential (e.g., GCP service account JSON) |
| Endpoints | `[]Endpoint` | Network endpoints needed (for firewall/network policy) |
| UnsetVars | `[]string` | Env vars to explicitly unset in the workspace (e.g., GEMINI_API_KEY for Vertex mode) |
| Priority | `int` | Prompt ordering priority (lower = higher priority, shown first) |

### EnvVarSpec (new, in `internal/agent`)

Describes a single environment variable within a credential spec.

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | Environment variable name (e.g., "ANTHROPIC_API_KEY") |
| FixedValue | `string` | If non-empty, always inject this value (e.g., "1" for CLAUDE_CODE_USE_VERTEX) |
| Required | `bool` | If true, this var must be present for the auth mode to be "available" |

### FileCredentialSpec (new, in `internal/agent`)

Describes a file-based credential within a credential spec.

| Field | Type | Description |
|-------|------|-------------|
| EnvVar | `string` | Env var that points to the file path (e.g., "GOOGLE_APPLICATION_CREDENTIALS") |
| DefaultPath | `string` | Default path to check if env var is unset (e.g., "~/.config/gcloud/application_default_credentials.json") |
| Required | `bool` | If true, the file must exist for the auth mode to be "available" |

### Endpoint (new, in `internal/agent`)

A network endpoint needed by an auth mode.

| Field | Type | Description |
|-------|------|-------------|
| Host | `string` | Hostname (e.g., "oauth2.googleapis.com") |
| Port | `int` | Port number (e.g., 443) |

### WorkspaceSpec (modified, in `internal/ws`)

Added fields to the existing struct:

| Field | Type | YAML Key | Description |
|-------|------|----------|-------------|
| Agent | `string` | `agent` | Agent name (e.g., "claude", "opencode"). Links to agent registry. |
| AuthMode | `string` | `auth-mode` | Selected CredentialSpec name. Replaces the meaning of existing `Auth` field. |
| ExternalCredentials | `bool` | `external-credentials` | If true, skip host-side credential validation for this workspace (K8s Secrets, OpenShell providers). |

The existing `Auth` field (YAML: `auth`) is deprecated in favor of `AuthMode`. During the transition, if `AuthMode` is empty and `Auth` is set, `Auth` is treated as the auth mode for backward compatibility.

### WorkspaceInstance (modified, in `internal/ws`)

Added field:

| Field | Type | YAML Key | Description |
|-------|------|----------|-------------|
| Agent | `string` | `agent` | Agent name, copied from definition at creation time |

### wsListEntry (modified, in `internal/cmd`)

Added fields for display:

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| Agent | `string` | `agent` | Agent display indicator |
| AuthMode | `string` | `auth_mode` | Selected auth mode name |

## Relationships

```text
Agent 1──* CredentialSpec       (agent declares multiple auth modes)
CredentialSpec 1──* EnvVarSpec  (each mode needs multiple env vars)
CredentialSpec 0──1 FileCredentialSpec (some modes need a file)
CredentialSpec 0──* Endpoint    (some modes need network access)
WorkspaceDefinition *──1 Agent  (each workspace uses one agent)
WorkspaceDefinition 1──1 CredentialSpec (selected auth mode)
```

## State Transitions

### Auth Mode Selection (during `ws new`)

```text
[Start] → Lookup agent by name
       → Query agent.CredentialSpecs()
       → For each spec, check host environment availability
       → Filter to available specs
       ├─ 0 available → ERROR: list required credentials
       ├─ 1 available → Auto-select (no prompt)
       └─ N available → Sort by priority, prompt user
       → Persist selected mode in workspace definition
```

### Auth Mode Switch (on existing workspace)

```text
[Start] → Lookup agent from workspace definition
       → Query agent.CredentialSpecs()
       → Validate new mode's credentials on host
       ├─ Valid → Update workspace definition
       └─ Invalid → ERROR: list missing credentials (no change)
```

### Credential Validation (at workspace start)

```text
[Start] → Load workspace definition
       → Check ExternalCredentials flag
       ├─ true → Skip validation
       └─ false → Lookup agent, get CredentialSpec for auth mode
                → Check all required env vars present
                → Check file credential exists (if any)
                ├─ All present → Proceed to transport
                └─ Missing → ERROR: name missing credentials
```
