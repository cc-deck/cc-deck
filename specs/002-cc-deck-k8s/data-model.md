# Data Model: cc-deck (Kubernetes CLI)

## Entities

### Profile

A named credential configuration for accessing an AI backend.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Unique profile identifier (e.g., "anthropic-dev", "vertex-prod") |
| backend | enum | `anthropic` or `vertex` |
| model | string | Model identifier (e.g., "claude-sonnet-4-20250514") |
| permissions | string | Claude Code permissions mode (e.g., "default", "restricted") |
| allowed_egress | []string | Additional egress hosts to allowlist beyond the backend defaults |

#### Anthropic-specific fields
| Field | Type | Description |
|-------|------|-------------|
| api_key_secret | string | Name of K8s Secret containing the API key |

#### Vertex-specific fields
| Field | Type | Description |
|-------|------|-------------|
| project | string | GCP project ID |
| region | string | GCP region (e.g., "us-central1") |
| credentials_secret | string | Name of K8s Secret with SA key (empty for Workload Identity) |

### Session

A deployed Claude Code + Zellij environment on a cluster.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Session name (used in K8s resource names) |
| namespace | string | K8s namespace where session resources live |
| profile | string | Profile name used for this session |
| status | enum | `deploying`, `running`, `stopped`, `deleted`, `error` |
| pod_name | string | StatefulSet Pod name (`cc-deck-<name>-0`) |
| connection | ConnectionInfo | How to connect to this session |
| created_at | timestamp | When the session was deployed |
| sync_dir | string | Local directory synced to this session (if any) |

### ConnectionInfo

Connection details for a session.

| Field | Type | Description |
|-------|------|-------------|
| exec_target | string | Pod name for `kubectl exec` |
| web_url | string | Route/Ingress URL for web client (if available) |
| web_port | int | Local port for port-forward (default: 8082) |
| method | enum | `exec`, `web`, `port-forward` |

### Config

The local configuration file (`~/.config/cc-deck/config.yaml`).

| Field | Type | Description |
|-------|------|-------------|
| default_profile | string | Name of the default profile |
| profiles | map[string]Profile | Named credential profiles |
| sessions | []Session | Tracked active sessions |
| defaults | Defaults | Default settings for new deployments |

### Defaults

Default settings applied to new deployments.

| Field | Type | Description |
|-------|------|-------------|
| namespace | string | Default namespace (empty = current context namespace) |
| storage_size | string | Default PVC size (e.g., "10Gi") |
| image | string | Container image reference |
| image_tag | string | Container image tag (e.g., "latest", "0.1.0") |

## State Transitions

### Session Status

```
deploy command
     │
     v
 Deploying ──── Pod fails ────> Error
     │
     │ Pod Running
     v
  Running ─── delete command ──> Deleted
     │
     │ Pod crashes / stops
     v
  Stopped ─── delete command ──> Deleted
```

## Config File Format

```yaml
# ~/.config/cc-deck/config.yaml
default_profile: anthropic-dev

defaults:
  namespace: ""
  storage_size: "10Gi"
  image: "ghcr.io/rhuss/cc-deck"
  image_tag: "latest"

profiles:
  anthropic-dev:
    backend: anthropic
    api_key_secret: cc-deck-anthropic-key
    model: claude-sonnet-4-20250514
    permissions: default

  vertex-prod:
    backend: vertex
    project: my-gcp-project
    region: us-central1
    credentials_secret: cc-deck-vertex-sa
    model: claude-sonnet-4-20250514
    allowed_egress:
      - "*.github.com"

sessions:
  - name: api-server
    namespace: dev
    profile: vertex-prod
    status: running
    pod_name: cc-deck-api-server-0
    connection:
      exec_target: cc-deck-api-server-0
      web_url: https://cc-deck-api-server-dev.apps.cluster.example.com
      method: exec
    created_at: "2026-03-03T10:00:00Z"
    sync_dir: /home/user/projects/api-server
```
