# Contract: Credential Transport

## Agent.CredentialSpecs() Contract

Every registered Agent MUST implement `CredentialSpecs() []CredentialSpec`.

### Behavioral Requirements

1. **Static return**: The method MUST return the same value on every call. Credential specs are compile-time constants.
2. **Unique names**: Each CredentialSpec in the returned slice MUST have a unique `Name` within that agent.
3. **At least one spec**: The slice MUST contain at least one entry. An agent without credentials is not supported by this system.
4. **Priority uniqueness**: Priority values SHOULD be unique within the slice. If two specs share a priority, their prompt ordering is deterministic (alphabetical by name).

### EnvVarSpec Requirements

1. A CredentialSpec with `Required: true` env vars is "available" only if ALL required env vars are set in the host environment.
2. Env vars with `FixedValue` are always injected regardless of host environment.
3. Non-required env vars are injected if present in the host environment but do not affect availability.

### FileCredentialSpec Requirements

1. If `Required: true`, the file MUST exist at the env var path or `DefaultPath` for the auth mode to be "available".
2. `DefaultPath` supports `~` expansion (resolved to `$HOME`).
3. File-based credentials are copied to the workspace and the env var is repointed to the remote path.

## Credential Package Contract

### resolve.Detect(specs []CredentialSpec) []AvailableMode

Returns the subset of specs whose required credentials are present on the host. Each AvailableMode includes the spec and a map of resolved credential values.

**Invariants**:
- Never reads credential values into log output.
- File existence is checked via `os.Stat`, not by reading content.
- `~` in DefaultPath is expanded before checking.

### validate.Check(spec CredentialSpec, externalCredentials bool) error

Returns nil if all required credentials for the spec are present. Returns a descriptive error listing each missing credential by env var name.

**Invariants**:
- If `externalCredentials` is true, returns nil immediately (skip validation).
- Error messages name the missing env var or file path, never credential values.
- Completes within 2 seconds (no network calls).

### transport.Inject(ctx, workspace, spec, resolvedCreds) error

Injects resolved credentials into a workspace according to its type.

**Invariants per workspace type**:

| Type | Env Vars | File Credentials | UnsetVars |
|------|----------|-----------------|-----------|
| local | Set in process environment | N/A (host path used directly) | Unset in process environment |
| container | Podman secrets + env flags | Secret mounted at `/run/secrets/` | `--env KEY=` (empty value) |
| compose | Environment section in compose YAML | Secret via compose secrets | Environment section with empty value |
| ssh | Written to `~/.config/cc-deck/credentials.env` (0600) | Copied via SSH upload (0600) | Added as `unset KEY` in env file |
| k8s-deploy | K8s Secret envFrom | K8s Secret with file data, volumeMount at `/run/secrets/` (defaultMode 0600) | Init container `unset` |
| openshell | Injected into sandbox rc files | Uploaded to sandbox | Added as `unset KEY` in rc files |

## CLI Contract

### `cc-deck ws new --agent <name> [--auth-mode <mode>]`

- If `--agent` is omitted, defaults to "claude".
- If `--auth-mode` is provided, validates that the mode exists in the agent's specs and that credentials are available. Errors if not.
- If `--auth-mode` is omitted and multiple modes are available, prompts the user. Modes are sorted by priority (lower first). The first mode is marked as default.
- If `--auth-mode` is omitted and exactly one mode is available, auto-selects it without prompting.
- If no modes are available, exits with error listing required credentials for all modes.

### `cc-deck ws ls`

- Displays an `AUTH` column showing `agent/mode` (e.g., "claude/vertex", "opencode/openai").
- JSON/YAML output includes `agent` and `auth_mode` fields.
