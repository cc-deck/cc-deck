# Research: OpenShell Credential Injection

## R1: OpenShell Provider CLI Commands

**Decision**: Wrap `openshell provider create/list/get/update/delete` via the existing `execCLI` pattern in `client.go`.

**Rationale**: The client already wraps all sandbox operations this way. Provider management follows the same pattern. The `--from-existing` flag auto-discovers credentials from the host environment, which aligns with FR-006.

**Key CLI patterns**:
- `openshell provider create --name <name> --type <type> --from-existing` (API key types)
- `openshell provider create --name <name> --type <type> --credential KEY=VALUE` (explicit)
- `openshell provider update <name> --type <type> --from-existing` (idempotent update, FR-011)
- `openshell provider list` (check for existing providers)
- `openshell provider delete <name>` (cleanup)

**Alternatives considered**: gRPC client for direct gateway communication. Rejected because the CLI already handles mTLS, gateway discovery, and error formatting. Adding gRPC would introduce build complexity for no benefit.

## R2: Provider Naming and Idempotency

**Decision**: Use `cc-deck-<workspace>-<type>` naming pattern. On create, first try `provider get` to check existence, then `provider update` if exists or `provider create` if not.

**Rationale**: The naming pattern avoids collisions between workspaces. The get-then-create-or-update pattern is simpler than parsing error messages from a failed create.

**Alternatives considered**: Shared providers across workspaces (e.g., just `cc-deck-claude`). Rejected because multiple workspaces might need different credentials (dev vs staging API keys).

## R3: Credential Detection for Capture

**Decision**: Scan the host environment for a fixed list of known env vars, map each to a provider profile. Present as a multi-select confirmation step.

**Known mappings**:
| Env Var | Provider Type |
|---------|--------------|
| `ANTHROPIC_API_KEY` | `claude` |
| `GITHUB_TOKEN`, `GH_TOKEN` | `github` |
| `GITLAB_TOKEN` | `gitlab` |
| `OPENAI_API_KEY` | `openai` |
| `NVIDIA_API_KEY` | `nvidia` |
| `GOOGLE_APPLICATION_CREDENTIALS` + `ANTHROPIC_VERTEX_PROJECT_ID` | `vertex` |
| `AWS_ACCESS_KEY_ID` + `CLAUDE_CODE_USE_BEDROCK` | `bedrock` (future) |

**Rationale**: Follows the pattern from `ssh/credentials.go:detectAuthMode()`. The capture step maps well-known env vars to provider profiles. The user confirms which to include.

**Alternatives considered**: Full environment scan for any `*_API_KEY` or `*_TOKEN` patterns. Rejected as too noisy and prone to false positives.

## R4: File-Based Credential Upload (Vertex)

**Decision**: After sandbox starts, use `openshell sandbox upload` to push the service account JSON to `/sandbox/.config/gcloud/credentials.json`. Then use `openshell sandbox exec` to set the env var in the sandbox's shell rc.

**Rationale**: The client already has an `Upload()` method. The sandbox exec can append `export GOOGLE_APPLICATION_CREDENTIALS=/sandbox/.config/gcloud/credentials.json` to `.bashrc` and `.zshrc`.

**Alternatives considered**: 
1. Pass JSON content as `--credential GOOGLE_SERVICE_ACCOUNT_JSON="$(cat sa.json)"` to a generic provider. OpenShell doesn't have a Vertex profile yet, so this wouldn't auto-wire network policy.
2. Bake into image at build time. Rejected because credentials in image layers are a security risk for shared images.

## R5: Policy Generation for Vertex

**Decision**: When the manifest's `credentials` section contains a `vertex` entry, `GeneratePolicy()` adds GCP endpoints to `network_policies`: `oauth2.googleapis.com:443` and `{region}-aiplatform.googleapis.com:443`.

**Rationale**: OpenShell's provider system auto-injects endpoints for known provider types (with `providers_v2_enabled`), but Vertex has no native profile yet. The build-time policy generation fills this gap.

**Region handling**: Read `CLOUD_ML_REGION` from the credential entry's `env_vars` at build time (resolved from host env). Default to `us-east1` if not set.

**Alternatives considered**: Hardcode all possible Vertex regions. Rejected as wasteful. The user specifies their region, and the policy includes only that endpoint.

## R6: Manifest Schema Extension

**Decision**: Add `credentials` as a top-level field in `Manifest`, not nested under `targets.openshell`. Credential requirements are workspace-level concerns that apply regardless of target type (future unification).

**Schema**:
```yaml
credentials:
  - type: claude
    env_vars: [ANTHROPIC_API_KEY]
  - type: github
    env_vars: [GITHUB_TOKEN]
  - type: vertex
    file: GOOGLE_APPLICATION_CREDENTIALS
    env_vars: [ANTHROPIC_VERTEX_PROJECT_ID, CLOUD_ML_REGION]
  - type: generic
    env_vars: [CUSTOM_API_KEY]
    endpoints:
      - host: api.custom.com
        port: 443
```

**Rationale**: Top-level placement mirrors how `tools`, `plugins`, and `network` are structured. The credential declarations are independent of the target type. The `env_vars` field contains names only (never values), consistent with how MCP auth is handled.

**Alternatives considered**: Nested under each target section. Rejected because credentials are typically shared across targets (same API key for container and openshell builds).
