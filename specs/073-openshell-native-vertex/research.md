# Research: OpenShell Native Vertex Provider

## OpenShell google-cloud Provider

**Decision**: Use OpenShell's native `google-cloud` provider type with `--from-gcloud-adc` flag.

**Rationale**: OpenShell PR #1763 (merged) adds a GCE metadata emulator inside the sandbox. GCP SDKs discover it via `GCE_METADATA_HOST`, get credential placeholders, and the sandbox proxy resolves them to real tokens at egress. This eliminates the need for cc-deck to upload credential files or manage GCP tokens.

**Alternatives considered**:
- `inference.local` proxy (OpenShell Option B): Being deprecated for Vertex by the maintainer. Only suitable for simple request proxying, not full GCP SDK compatibility.
- Direct env var injection with file upload (current approach): Works but bypasses OpenShell's security model (real credentials enter the sandbox).

## Provider Config Options

**Decision**: Pass `project_id` and `region` as `--config` options to `openshell provider create`.

**Rationale**: The gist shows `--config project_id="$(gcloud config get-value project)" --config region=global`. These are needed by the metadata emulator to serve correct project/region values to GCP SDKs.

**Implementation**: The `EnsureProvider` function in `openshell/client.go` needs to support passing config key-value pairs. If the current `Credentials map[string]string` field is used, these flow as `--credential key=value` flags. If a separate config mechanism is needed, the `ProviderConfig` struct gets a `Config map[string]string` field.

## Env Var Injection After Provider Creation

**Decision**: Continue injecting `CLAUDE_CODE_USE_VERTEX=1`, `ANTHROPIC_VERTEX_PROJECT_ID`, and `CLOUD_ML_REGION` as env vars into the sandbox shell rc files.

**Rationale**: Claude Code requires these env vars to know it should use Vertex AI. The metadata emulator handles GCP authentication (tokens), but Claude Code still needs to be told which project and region to target. These are not secrets, so plain env var injection is appropriate.

**Implementation**: The `InjectEnvVars` function already handles this. The `claude-vertex` profile's `EnvVarsToInject` map continues to populate these vars, but the profile no longer sets `SkipProvider: true`.

## Dead Code Removal

**Decision**: Remove the standalone `vertex` profile from `KnownProviderProfiles` and the `SkipProvider` logic for `claude-vertex`.

**Rationale**: The `vertex` profile (lines 97-106) was a generic Vertex credential entry. With OpenShell's native provider, it serves no purpose. The `claude-vertex` entry no longer needs `SkipProvider` since it will create a real `google-cloud` provider.

**Scope**: Only the OpenShell credential path is affected. The agent-level credential spec (`internal/agent/claude.go` line 175, `Name: "vertex"`) is preserved because it is used by non-OpenShell workspace types for env var detection.
