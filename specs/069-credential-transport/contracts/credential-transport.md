# Contract: Credential Transport

## Agent.CredentialSpecs() Contract

Every registered Agent MUST implement `CredentialSpecs() []CredentialSpec`.

### Behavioral Requirements (unchanged from v1)

1. **Static return**: The method MUST return the same value on every call.
2. **Unique names**: Each CredentialSpec MUST have a unique `Name` within that agent.
3. **At least one spec**: The slice MUST contain at least one entry.
4. **Priority uniqueness**: Priority values SHOULD be unique. Ties broken alphabetically by name.

### EnvVarSpec Requirements (unchanged)

1. Required env vars must be present for the mode to be "available".
2. FixedValue vars are always injected regardless of host environment.
3. Non-required vars are injected if present but do not affect availability.

### FileCredentialSpec Requirements (unchanged)

1. Required files must exist at env var path or DefaultPath for availability.
2. `~` expansion resolved to `$HOME`.
3. Files are copied to workspace and env var repointed to remote path.

## Credential Package Contract

### credential.DetectAll() []DetectedMode

Scans ALL registered agents, checks each spec against the host environment, returns all available modes grouped by agent.

**Invariants**:
- Never reads credential values into log output.
- Scans agents via `agent.All()` in stable alphabetical order.
- Returns `DetectedMode` with agent name, spec, and resolved values.
- An agent with zero available modes contributes nothing (no error).

### credential.MergeCredentials(modes []DetectedMode) ResolvedCredentials

Merges all detected modes into a single credential set for injection.

**Invariants**:
- Env vars with the same name and same value are deduplicated.
- Env vars with the same name and different values are an error.
- FileCredentials from different modes are collected into a list.
- UnsetVars from all modes are merged.

### credential.Validate(modes []DetectedMode, externalCredentials bool) error

Returns nil if all required credentials for all modes are present.

**Invariants**:
- If `externalCredentials` is true, returns nil immediately.
- Error messages name the missing env var or file path, never values.
- Completes within 2 seconds (no network calls).

### Transport Functions (unchanged from v1)

| Type | Env Vars | File Credentials | UnsetVars |
|------|----------|-----------------|-----------|
| local | Set in process environment | N/A | Unset in process |
| container | Podman secrets + env flags | Secret mounted at `/run/secrets/` | `--env KEY=` |
| compose | Environment section | Secret via compose secrets | Empty value |
| ssh | Written to `credentials.env` (0600) | Copied via SSH (0600) | `unset KEY` |
| k8s-deploy | K8s Secret envFrom | Secret with file data, volumeMount (0600) | Init container `unset` |
| openshell | Injected into sandbox rc files | Uploaded to sandbox | `unset KEY` in rc |

## CLI Contract

### `cc-deck ws new [name]`

- Scans all registered agents for available credentials.
- Injects all detected credentials without prompting.
- If no credentials are available from any agent, warns but creates the workspace.

### `cc-deck ws ls [-v]`

- Default output: no AUTH column.
- Verbose (`-v`): AUTH column shows derived `agent/mode` list for each workspace.
- JSON/YAML output always includes `auth` field.
