# Data Model: OpenShell Native Vertex Provider

## Entity Changes

### KnownProviderProfile (modified)

**`claude-vertex` entry** - Updated from skip-provider-with-env-injection to real google-cloud provider:

| Field | Before | After |
|-------|--------|-------|
| Type | `"claude"` | `"google-cloud"` |
| SkipProvider (implicit) | `true` (hardcoded in ResolveCredentials) | `false` (creates provider via EnsureProvider) |
| FileVar | `"GOOGLE_APPLICATION_CREDENTIALS"` | removed (OpenShell handles credentials) |
| DetectVars | `["CLAUDE_CODE_USE_VERTEX", "ANTHROPIC_VERTEX_PROJECT_ID"]` | unchanged |
| RequiredVars | `["ANTHROPIC_VERTEX_PROJECT_ID"]` | unchanged |
| ExtraEnvVars | `["CLOUD_ML_REGION", "ANTHROPIC_MODEL"]` | unchanged |
| Endpoints | `[{oauth2.googleapis.com:443}]` | removed (OpenShell generates policy from provider) |

**`vertex` entry** - Removed entirely (dead code).

### ProviderConfig (modified)

New field to support OpenShell provider config options:

| Field | Type | Purpose |
|-------|------|---------|
| Config | `map[string]string` | Key-value pairs passed as `--config key=value` to `openshell provider create` |

The `FromExisting` field behavior changes for `google-cloud` type: when `true`, the provider uses `--from-gcloud-adc` instead of `--from-existing`.

### ResolveCredentials Flow (modified)

The special-case `if entry.Type == "claude-vertex"` block (lines 182-215) that sets `SkipProvider=true` and manually resolves file credentials is replaced with standard provider creation flow. The `claude-vertex` entry now produces a `ProviderConfig` with:
- `Type: "google-cloud"`
- `FromExisting: true` (triggers `--from-gcloud-adc`)
- `Config: {"project_id": <value>, "region": <value>}`
- `EnvVarsToInject: {"CLAUDE_CODE_USE_VERTEX": "1", "ANTHROPIC_VERTEX_PROJECT_ID": <value>, "CLOUD_ML_REGION": <value>}`

### DetectCredentials Order (modified)

The detection order changes from:
```
["claude-vertex", "claude", "github", "gitlab", "openai", "nvidia", "vertex"]
```
to:
```
["claude-vertex", "claude", "github", "gitlab", "openai", "nvidia"]
```

The standalone `"vertex"` entry is removed.
