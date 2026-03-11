# Quickstart: Kubernetes Remote Sessions

Deploy and manage Claude Code sessions on Kubernetes or OpenShift clusters.

## Prerequisites

- `cc-deck` CLI built and on your PATH
- `kubectl` configured with access to a Kubernetes 1.24+ or OpenShift 4.12+ cluster
- A namespace where you can create StatefulSets, Services, PVCs, and NetworkPolicies
- An API key for Anthropic or credentials for Google Vertex AI

## 1. Build cc-deck

```bash
cd cc-deck
go build -o cc-deck ./cmd/cc-deck
# Move to PATH or use ./cc-deck below
```

## 2. Create a Credential Secret

cc-deck reads API credentials from Kubernetes Secrets. Create one in your target namespace.

### Anthropic API Key

```bash
kubectl create secret generic anthropic-key \
  --from-literal=api-key="sk-ant-..." \
  -n my-namespace
```

### Google Vertex AI

```bash
kubectl create secret generic gcp-creds \
  --from-file=credentials.json=/path/to/service-account.json \
  -n my-namespace
```

## 3. Configure a Profile

Profiles store credential references and backend settings locally.

```bash
# Interactive setup
cc-deck profile add my-profile

# Or create config manually at ~/.config/cc-deck/config.yaml
```

**Example config.yaml** (Anthropic):

```yaml
default_profile: my-profile
defaults:
  namespace: my-namespace
  image: ghcr.io/anthropics/claude-code
  image_tag: latest
profiles:
  my-profile:
    backend: anthropic
    api_key_secret: anthropic-key
```

**Example config.yaml** (Vertex AI):

```yaml
default_profile: vertex-prod
defaults:
  namespace: my-namespace
profiles:
  vertex-prod:
    backend: vertex
    project: my-gcp-project
    region: us-central1
    credentials_secret: gcp-creds
```

Set the default profile:

```bash
cc-deck profile use my-profile
cc-deck profile list
```

## 4. Deploy a Session

```bash
# Deploy with default profile and namespace
cc-deck deploy myproject

# Deploy with explicit options
cc-deck deploy myproject \
  --profile my-profile \
  --namespace dev \
  --storage 20Gi \
  --image ghcr.io/myorg/claude-code:v1.0

# Deploy and sync a local directory
cc-deck deploy myproject --sync-dir /path/to/repo

# Deploy with additional egress allowed
cc-deck deploy myproject --allow-egress github.com --allow-egress pypi.org
```

This creates:
- **StatefulSet** `cc-deck-myproject` (1 replica)
- **Headless Service** `cc-deck-myproject`
- **PVC** `data-cc-deck-myproject-0` (10Gi default)
- **ConfigMap** `cc-deck-myproject-zellij` (Zellij web server config)
- **NetworkPolicy** `cc-deck-myproject` (default-deny egress with allowlist)

## 5. Connect to a Session

```bash
# Terminal exec (default, attaches to Zellij inside the Pod)
cc-deck connect myproject

# Web browser (port-forward + open browser)
cc-deck connect myproject --web

# Port-forward only (no browser)
cc-deck connect myproject --method port-forward

# Custom local port
cc-deck connect myproject --web --port 9090
```

Auto-detection: on OpenShift, if a Route exists, `cc-deck connect` opens the web URL directly.

## 6. Sync Files

```bash
# Push local directory to Pod
cc-deck sync myproject

# Pull changes from Pod back to local
cc-deck sync myproject --pull
```

Files sync to `/workspace` inside the Pod.

## 7. Manage Sessions

```bash
# List all sessions
cc-deck list

# View Pod logs
cc-deck logs myproject
cc-deck logs myproject --follow

# Delete session (removes all K8s resources)
cc-deck delete myproject
```

## 8. Git Credentials (Optional)

To let Claude push/pull from git repos inside the Pod:

### SSH Key

```bash
kubectl create secret generic git-ssh \
  --from-file=ssh-privatekey=$HOME/.ssh/id_ed25519 \
  -n my-namespace
```

Add to your profile in config.yaml:

```yaml
profiles:
  my-profile:
    backend: anthropic
    api_key_secret: anthropic-key
    git_credential_type: ssh
    git_credential_secret: git-ssh
```

### Token-based

```bash
kubectl create secret generic git-token \
  --from-literal=token="ghp_..." \
  -n my-namespace
```

```yaml
profiles:
  my-profile:
    backend: anthropic
    api_key_secret: anthropic-key
    git_credential_type: token
    git_credential_secret: git-token
```

## Global Flags

These flags work with any command:

| Flag | Description | Default |
|------|-------------|---------|
| `--kubeconfig` | Path to kubeconfig | `$KUBECONFIG` or `~/.kube/config` |
| `--namespace` | Kubernetes namespace | From config or kubeconfig context |
| `--profile` | Credential profile name | From config default |
| `--config` | Config file path | `~/.config/cc-deck/config.yaml` |
| `--verbose` | Verbose output | false |

## Resource Overlays

For environment-specific customization (node selectors, tolerations, resource limits), use kustomize overlays:

```bash
cc-deck deploy myproject --overlay ./my-overlay/
```

The overlay directory should contain a `kustomization.yaml` that patches the generated resources.

## Egress NetworkPolicy

By default, all outbound traffic is blocked except:
- DNS (UDP/TCP 53)
- Anthropic API (`api.anthropic.com:443`) or Vertex AI (`*.googleapis.com:443`)
- Any hosts from `--allow-egress` flags or profile `allowed_egress`

Skip the NetworkPolicy entirely with `--no-network-policy`.

## Troubleshooting

**Pod stays Pending**: Check events with `kubectl describe pod cc-deck-myproject-0 -n my-namespace`. Common causes: no StorageClass, insufficient resources, node selector mismatch.

**Secret not found**: Ensure the Secret exists in the same namespace as the session: `kubectl get secret anthropic-key -n my-namespace`.

**Connection refused**: For web/port-forward, ensure Zellij is running with web server enabled (the ConfigMap handles this automatically).

**Cluster unreachable**: Verify kubeconfig: `kubectl cluster-info`. Use `--kubeconfig` to point to a specific config.
