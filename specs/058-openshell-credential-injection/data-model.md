# Data Model: OpenShell Credential Injection

## Entities

### CredentialEntry (manifest)

A declaration in `build.yaml` describing a credential provider requirement.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | Provider profile type: `claude`, `github`, `gitlab`, `openai`, `nvidia`, `vertex`, `bedrock`, `generic` |
| `env_vars` | []string | no | Environment variable names to resolve from host. Auto-populated from known profiles if omitted. |
| `file` | string | no | Env var name whose value is a file path (e.g., `GOOGLE_APPLICATION_CREDENTIALS`). File is uploaded at runtime. |
| `endpoints` | []PolicyEndpoint | no | Custom network endpoints for `generic` type. Each has `host` and `port`. |

**Default env_vars by type** (used when `env_vars` is omitted):

| Type | Default env_vars |
|------|-----------------|
| `claude` | `ANTHROPIC_API_KEY` |
| `anthropic` | `ANTHROPIC_API_KEY` |
| `github` | `GITHUB_TOKEN`, `GH_TOKEN` |
| `gitlab` | `GITLAB_TOKEN`, `GLAB_TOKEN` |
| `openai` | `OPENAI_API_KEY` |
| `nvidia` | `NVIDIA_API_KEY` |
| `vertex` | `GOOGLE_APPLICATION_CREDENTIALS`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION` |
| `generic` | (must be specified explicitly) |

### KnownProviderProfile (code constant)

Maps a credential type to its OpenShell provider profile and detection env vars.

| Field | Type | Description |
|-------|------|-------------|
| `Type` | string | Provider type name (matches OpenShell profile ID) |
| `DetectVars` | []string | Env vars to scan for during capture detection |
| `RequiredVars` | []string | Env vars that must be set for the provider to be created |
| `FileVar` | string | Env var pointing to a file (empty for API key types) |
| `Endpoints` | []PolicyEndpoint | Network endpoints to add to policy (for types without native OpenShell provider) |

## Relationships

```
Manifest
  └── Credentials []CredentialEntry
        └── type → KnownProviderProfile (lookup)
              ├── → OpenShell Provider (created at ws new time)
              └── → PolicyFile.NetworkPolicies (added at build time)
```

## State Transitions

### Provider Lifecycle

```
[not exists] → CreateProvider → [active] → DeleteProvider → [not exists]
                                   ↑
                              UpdateProvider
```

- Providers are created during `ws new --type openshell`
- Providers are NOT deleted during `ws delete` (out of scope for v1)
- Providers are updated (idempotent) if they already exist

### Credential Resolution

```
build.yaml credentials → detect env vars on host → resolve values → create provider / upload file
```

Resolution happens at `ws new` time, not at build or capture time. The manifest stores only type and var names. Actual values come from the runtime environment.
