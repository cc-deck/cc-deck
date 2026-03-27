# Data Model: Kubernetes Deploy Environment

**Feature**: 028-k8s-deploy | **Date**: 2026-03-27

## Entities

### K8sDeployEnvironment (Go struct)

The primary implementation struct for the `Environment` interface.

```go
type K8sDeployEnvironment struct {
    name  string
    store *FileStateStore
    defs  *DefinitionStore

    // K8s connection
    Namespace  string // Target namespace
    Kubeconfig string // Path to kubeconfig (empty = default)
    Context    string // Kubeconfig context (empty = current)

    // Storage
    StorageSize  string // PVC size (default: "10Gi")
    StorageClass string // StorageClass name (empty = cluster default)

    // Credentials
    Credentials    map[string]string // Inline key=value pairs
    ExistingSecret string            // Reference to pre-existing Secret
    SecretStore    string            // ESO SecretStore type
    SecretStoreRef string            // ESO SecretStore name
    SecretPath     string            // ESO secret path

    // Network
    NoNetworkPolicy bool     // Skip NetworkPolicy creation
    AllowDomains    []string // Additional allowed domains
    AllowGroups     []string // Additional allowed domain groups

    // MCP
    BuildDir string // Build directory containing cc-deck-image.yaml

    // Auth
    Auth AuthMode // auto, none, api, vertex, bedrock
}
```

### K8sFields (existing, in types.go)

Runtime state stored in EnvironmentInstance:

```go
type K8sFields struct {
    Namespace   string `yaml:"namespace,omitempty"`
    StatefulSet string `yaml:"stateful_set,omitempty"`
    Profile     string `yaml:"profile,omitempty"`     // kubeconfig context name
    Kubeconfig  string `yaml:"kubeconfig,omitempty"`
}
```

### Generated K8s Resources

For each k8s-deploy environment named `<name>` in namespace `<ns>`:

| Resource | Name | Purpose |
|----------|------|---------|
| StatefulSet | `cc-deck-<name>` | Manages the workspace Pod (replicas=0 or 1) |
| Headless Service | `cc-deck-<name>` | Stable DNS for StatefulSet Pods |
| ConfigMap | `cc-deck-<name>` | Environment configuration |
| PVC | `cc-deck-<name>-data-cc-deck-<name>-0` | Persistent workspace storage (via volumeClaimTemplates) |
| Secret | `cc-deck-<name>-creds` | Inline credentials (if provided) |
| NetworkPolicy | `cc-deck-<name>` | Egress filtering (unless `--no-network-policy`) |
| ExternalSecret | `cc-deck-<name>-eso` | ESO sync (only if `--secret-store` specified) |
| Route (OpenShift) | `cc-deck-<name>` | Web access (only on OpenShift with web port) |
| EgressFirewall (OpenShift) | `cc-deck-<name>` | OVN egress rules (only on OpenShift) |

### Resource Labels (applied to all)

```yaml
metadata:
  labels:
    app.kubernetes.io/name: cc-deck
    app.kubernetes.io/instance: <env-name>
    app.kubernetes.io/managed-by: cc-deck
    app.kubernetes.io/component: workspace
```

### StatefulSet Spec

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: cc-deck-<name>
  namespace: <ns>
  labels: {standard labels}
spec:
  replicas: 1
  serviceName: cc-deck-<name>
  selector:
    matchLabels:
      app.kubernetes.io/name: cc-deck
      app.kubernetes.io/instance: <name>
  template:
    metadata:
      labels: {standard labels}
    spec:
      containers:
        - name: workspace
          image: <resolved-image>
          command: ["sleep", "infinity"]
          volumeMounts:
            - name: data
              mountPath: /workspace
            - name: credentials
              mountPath: /run/secrets/cc-deck
              readOnly: true
          # MCP sidecar containers appended here
      volumes:
        - name: credentials
          secret:
            secretName: cc-deck-<name>-creds  # or existingSecret
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: <storage-size>  # default: 10Gi
        storageClassName: <storage-class>  # omitted if empty (cluster default)
```

### Headless Service Spec

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cc-deck-<name>
  namespace: <ns>
  labels: {standard labels}
spec:
  clusterIP: None
  selector:
    app.kubernetes.io/name: cc-deck
    app.kubernetes.io/instance: <name>
  ports:
    - name: placeholder
      port: 80
      targetPort: 80
```

### NetworkPolicy Spec

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cc-deck-<name>
  namespace: <ns>
  labels: {standard labels}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: cc-deck
      app.kubernetes.io/instance: <name>
  policyTypes:
    - Egress
  egress:
    # DNS resolution (always allowed)
    - to: []
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # Allowed domains (resolved to IPs)
    - to:
        - ipBlock:
            cidr: <resolved-ip>/32
      ports:
        - protocol: TCP
          port: 443
```

### Credential Sources

```
┌──────────────────────┐     ┌─────────────────────────┐
│  Inline Credentials  │     │    Existing Secret       │
│  --credential k=v    │     │    --existing-secret nm  │
│                      │     │                          │
│  Creates K8s Secret  │     │  No Secret created       │
│  cc-deck-<name>-creds│     │  References user Secret  │
│  Deleted on env del  │     │  Preserved on env del    │
└──────┬───────────────┘     └──────┬──────────────────┘
       │                            │
       ▼                            ▼
    Volume-mounted at /run/secrets/cc-deck/
```

### MCP Sidecar Generation

```
cc-deck-image.yaml                       Pod Spec
┌─────────────────────┐                  ┌──────────────────────┐
│ mcp:                │                  │ containers:          │
│   - name: github    │  ──────────────► │   - name: workspace  │
│     image: ghcr...  │                  │   - name: mcp-github │
│     transport: http  │                  │     image: ghcr...   │
│     port: 3000      │                  │     ports:           │
│     auth:           │                  │       - 3000         │
│       env_vars:     │                  │     env:             │
│         - GH_TOKEN  │                  │       - GH_TOKEN     │
└─────────────────────┘                  │         from: secret │
                                         └──────────────────────┘
```

MCP sidecars share the Pod's network namespace, so the main container reaches them via `localhost:<port>`.

## State Transitions

```
                    ┌─────────┐
          create    │         │  delete --force
     ┌─────────────►│ running ├──────────────────┐
     │              │         │                   │
     │              └────┬────┘                   ▼
     │                   │                   ┌────────┐
     │              stop │                   │deleted  │
     │                   │                   └────────┘
     │              ┌────▼────┐                   ▲
     │              │         │  delete           │
     │              │ stopped ├───────────────────┘
     │              │         │
     │              └────┬────┘
     │                   │
     │              start│
     │                   │
     │              ┌────▼────┐
     │              │         │
     └──────────────┤ running │
                    │         │
                    └─────────┘

Error state: entered when Pod fails to reach Running within timeout.
Reconciliation: Status checks against K8s API can correct stale stored state.
```

## Validation Rules

| Field | Constraint | Source |
|-------|-----------|--------|
| Name | `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 40 chars | `ValidateEnvName()` |
| Namespace | Must exist on cluster | Validated at create time |
| StorageSize | Valid K8s quantity (e.g., "10Gi", "50Gi") | K8s API validation |
| StorageClass | Must exist on cluster (if specified) | K8s API validation |
| Kubeconfig | Must be a readable file (if specified) | Validated before API calls |
| Credentials | KEY=VALUE format | `splitCredential()` existing helper |
