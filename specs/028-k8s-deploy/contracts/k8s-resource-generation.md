# Contract: K8s Resource Generation

**Feature**: 028-k8s-deploy | **Date**: 2026-03-27

## Overview

This contract defines the interface for generating Kubernetes resource manifests from environment configuration. The resource generator is responsible for producing well-formed K8s objects that the K8sDeployEnvironment lifecycle methods apply to the cluster.

## Interface

```go
package env

import (
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    networkingv1 "k8s.io/api/networking/v1"
)

// K8sResourceSet holds all generated K8s resources for a single environment.
type K8sResourceSet struct {
    StatefulSet   *appsv1.StatefulSet
    Service       *corev1.Service
    ConfigMap     *corev1.ConfigMap
    Secret        *corev1.Secret          // nil if using existing-secret
    NetworkPolicy *networkingv1.NetworkPolicy // nil if --no-network-policy
    Sidecars      []corev1.Container      // MCP sidecar containers
}

// GenerateResources creates all K8s resource objects from environment config.
// Resources are generated in memory but not applied to the cluster.
func GenerateResources(opts K8sResourceOpts) (*K8sResourceSet, error)

// K8sResourceOpts captures all inputs needed for resource generation.
type K8sResourceOpts struct {
    Name           string
    Namespace      string
    Image          string
    StorageSize    string
    StorageClass   string            // empty = cluster default
    Credentials    map[string]string // inline credentials (key=value)
    ExistingSecret string            // name of pre-existing Secret
    Domains        []string          // resolved domain list for NetworkPolicy
    NoNetworkPolicy bool
    MCPSidecars    []MCPSidecarOpts  // from build manifest
    Labels         map[string]string // standard labels
}

// MCPSidecarOpts describes one MCP sidecar container.
type MCPSidecarOpts struct {
    Name    string
    Image   string
    Port    int
    EnvVars []string // env var names to inject from Secret
}
```

## Behavioral Requirements

### Resource Naming

1. All resources MUST use the name `cc-deck-<env-name>`.
2. The Pod template MUST use `cc-deck-<env-name>-0` as the predictable Pod name (StatefulSet ordinal).
3. The PVC name follows the StatefulSet convention: `<volumeClaimTemplate-name>-<statefulset-name>-<ordinal>`.

### Labels

1. All resources MUST include the standard label set:
   - `app.kubernetes.io/name: cc-deck`
   - `app.kubernetes.io/instance: <env-name>`
   - `app.kubernetes.io/managed-by: cc-deck`
2. The `component` label MUST be `workspace` for the main workload and `mcp-<name>` for sidecar-specific resources.

### StatefulSet

1. MUST use `replicas: 1` at creation time.
2. MUST reference a headless Service via `serviceName`.
3. MUST use `volumeClaimTemplates` for the workspace PVC (NOT a pre-created PVC).
4. The main container command MUST be `["sleep", "infinity"]`.
5. Credentials MUST be volume-mounted at `/run/secrets/cc-deck/`, never injected as environment variables.

### Headless Service

1. MUST set `clusterIP: None`.
2. MUST use the same selector labels as the StatefulSet Pod template.

### Secret

1. If inline credentials are provided, MUST create a Secret of type `Opaque` with the key-value pairs.
2. If `--existing-secret` is specified, MUST NOT create a Secret.
3. The Secret volume mount path MUST be `/run/secrets/cc-deck/`.

### NetworkPolicy

1. If `--no-network-policy` is set, MUST NOT generate a NetworkPolicy.
2. Default policy MUST be deny-all egress.
3. MUST always allow DNS egress (UDP/TCP port 53).
4. MUST add egress rules for each resolved domain IP on port 443.
5. Domain resolution uses `net.LookupHost()` at generation time.

### MCP Sidecars

1. Each MCP entry with a non-empty `Image` field MUST become a sidecar container.
2. Sidecar containers MUST share the Pod's network namespace (default K8s behavior).
3. Sidecar credential env vars MUST reference the same Secret used by the main container.
4. MCP entries without an `Image` field (stdio transport) MUST NOT generate sidecars.

## Error Handling

1. If the image is empty, MUST return an error.
2. If StorageSize is not a valid K8s quantity, MUST return an error.
3. If both inline credentials AND existing-secret are provided, MUST merge: inline creates a separate Secret; existing-secret is mounted additionally.
