# Research: Credential Transport Abstraction

## Codebase Analysis

### Legacy Credential Code (to be removed)

#### `internal/openshell/credentials.go`

- **`KnownProviderProfiles`** (map, line 58): Hardcoded map of 8 profiles (claude, claude-vertex, anthropic, github, gitlab, openai, nvidia, generic). Marked deprecated. Used only within the same file and by `ws/openshell.go`.
- **`ResolveDefaultEnvVars(credType)`** (func, line 103): Looks up env vars for a credential type from the hardcoded map. Called by `ws/openshell.go:319`.
- **`ResolveCredentials(entries, wsName)`** (func, line 117): Takes `[]CredentialInput`, iterates `KnownProviderProfiles`, resolves env vars from host, returns `[]ProviderConfig`. Called by `ws/openshell.go:309`.
- **`DetectCredentials()`** (func, line 233): Scans host env against hardcoded profile order. Not called by any production code outside this file (only tests). Already superseded by `credential.DetectAll()`.
- **`CredentialInput`** (type, line 42): Input from manifest layer. May need to be kept or replaced with `agent.CredentialSpec`.
- **`DetectedCredential`** (type, line 49): Output of detection. Replaced by `credential.DetectedMode`.
- **`ProviderConfig`** (type, line 30): Resolved config for provider creation. Used by `ws/openshell.go` extensively.

#### `internal/ssh/credentials.go`

- **`BuildCredentialSet(authMode, credentials, envVars)`** (func, line 14): Hardcoded switch on "api"/"vertex"/"bedrock". Called by `ws/ssh.go:203` and `cmd/ws.go:1821`. Marked deprecated.
- **`detectAuthMode()`** (func, line 88): Hardcoded priority: API_KEY > VERTEX > BEDROCK. Only called by `BuildCredentialSet`. Marked deprecated.

### Transport Functions (to be kept, signature changed)

#### `internal/openshell/credentials.go`

- **`InjectEnvVars(ctx, client, sandboxID, vars)`** (func, line 305): Writes env exports to shell rc files via OpenShell SDK. Used by `ws/openshell.go:381`. Depends on `v1.ClientInterface`.
- **`UploadFileCredential(ctx, client, sandboxID, localPath, remotePath, envVarName)`** (func, line 323): Uploads file via OpenShell SDK. Used by `ws/openshell.go:376`. Depends on `v1.ClientInterface`.

#### `internal/ssh/credentials.go`

- **`WriteCredentialFile(ctx, client, creds)`** (func, line 103): Writes credential env file to remote host via SSH. Used by `ws/ssh.go`. Depends on `*ssh.Client`.
- **`CopyCredentialFile(ctx, client, localPath, remoteName)`** (func, line 151): Copies file to remote host via SSH. Called by `WriteCredentialFile`. Depends on `*ssh.Client`.

### New Credential Package (already exists, no changes needed)

#### `internal/credential/`

- **`resolve.go`**: `Detect()`, `DetectAll()`, `Resolve()`, `MergeCredentials()` all work correctly.
- **`transport.go`**: `InjectContainer()` handles Podman secret creation. Only used by Podman/Compose path.
- **`validate.go`**: `Validate()`, `ValidateAll()` check required credentials.
- **`types.go`**: `AvailableMode`, `DetectedMode`, `ResolvedCredentials`, `ResolvedFile`.

### Workspace Definition Fields

The workspace definition (in `internal/ws/`) stores:
- `Auth` (string): Auth mode name (e.g., "api", "vertex", "bedrock")
- `Credentials` ([]string): Explicit credential env var names
- `Env` (map[string]string): Explicit env vars

These fields are currently passed to `BuildCredentialSet()`. After migration, `Auth` maps to a `CredentialSpec.Name`, and the explicit `Credentials`/`Env` are merged with the resolved spec output.

## Key Findings

1. The `openshell.ProviderConfig` type is deeply embedded in `ws/openshell.go`. The migration must produce equivalent data for the OpenShell provider creation calls downstream.

2. The `claude-vertex` special case in `openshell.ResolveCredentials()` (lines 171-198) uses OpenShell's native google-cloud provider type and passes `project_id`/`region` as provider config. This logic must be preserved in the migration.

3. `BuildCredentialSet` always injects common optional model variables (`ANTHROPIC_DEFAULT_SONNET_MODEL`, etc.) regardless of auth mode. The Claude agent's `CredentialSpecs()` already declares these as non-required env vars in each spec, so `credential.Resolve()` handles this correctly.

4. The Podman/Compose credential path already uses `credential.InjectContainer()` and does not reference the deprecated functions. No changes needed there.

5. The K8s credential path uses its own Secret/ConfigMap injection and does not reference the deprecated functions. No changes needed there.
