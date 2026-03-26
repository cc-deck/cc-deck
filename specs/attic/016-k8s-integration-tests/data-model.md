# Data Model: K8s Integration Tests

## Entities

### TestEnv

Shared test environment created once in `TestMain`, reused by all tests.

| Field | Type | Description |
|-------|------|-------------|
| clientset | kubernetes.Interface | K8s API client |
| restConfig | *rest.Config | REST config for SPDY/exec operations |
| namespace | string | Test namespace (e.g., "cc-deck-test") |
| image | string | Stub image name (e.g., "localhost/cc-deck-stub") |
| imageTag | string | Stub image tag (e.g., "latest") |

### TestSession

Per-test deploy configuration with a unique name for isolation.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Unique session name (e.g., "t-deploy-a1b2") |
| profile | config.Profile | Dummy Anthropic profile with test Secret reference |
| deployOpts | session.DeployOptions | Full deploy options built from TestEnv + test-specific overrides |

### Expected K8s Resources (per session)

| Resource | Name Pattern | Purpose |
|----------|-------------|---------|
| StatefulSet | `cc-deck-{name}` | Session Pod management |
| Service | `cc-deck-{name}` | Headless service for StatefulSet |
| ConfigMap | `cc-deck-{name}-zellij` | Zellij web server config |
| PVC | `data-cc-deck-{name}-0` | Persistent workspace storage |
| NetworkPolicy | `cc-deck-{name}` | Egress restriction |
| Pod | `cc-deck-{name}-0` | Running container (from StatefulSet) |

### Dummy Profile

| Field | Value |
|-------|-------|
| Backend | "anthropic" |
| APIKeySecret | "test-api-key" |

The test Secret `test-api-key` is created in `TestMain` with a dummy `api-key` value.

## Lifecycle

```
TestMain:
  1. Load kubeconfig (kind cluster)
  2. Create namespace (if not exists)
  3. Create dummy Secret
  4. Initialize TestEnv
  5. Run all tests (parallel)
  6. Delete namespace (cleanup)

Per test:
  1. Generate unique session name
  2. Build DeployOptions from TestEnv
  3. Call session.Deploy() / session.List() / session.Delete()
  4. Assert resource state
  5. t.Cleanup() deletes resources
```
