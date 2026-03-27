# Contract: Credential Management for K8s Deploy

**Feature**: 028-k8s-deploy | **Date**: 2026-03-27

## Overview

This contract defines how credentials are provided, stored, and mounted in k8s-deploy environments. The design ensures credentials are never exposed as environment variables, consistent with the spec requirement (FR-005).

## Credential Sources

### 1. Inline Credentials (`--credential KEY=VALUE`)

```
User provides:     --credential ANTHROPIC_API_KEY=sk-ant-...
System creates:    K8s Secret "cc-deck-<name>-creds" with data key "ANTHROPIC_API_KEY"
Pod sees:          File at /run/secrets/cc-deck/ANTHROPIC_API_KEY containing "sk-ant-..."
On delete:         Secret is deleted (cc-deck-managed)
```

### 2. Existing Secret (`--existing-secret <name>`)

```
User provides:     --existing-secret my-api-keys
System creates:    Nothing (Secret already exists)
Pod sees:          Files at /run/secrets/cc-deck/<key> for each key in the Secret
On delete:         Secret is NOT deleted (user-managed)
```

### 3. External Secrets Operator (`--secret-store`)

```
User provides:     --secret-store vault --secret-store-ref my-vault --secret-path secret/data/cc-deck
System creates:    ExternalSecret CR referencing the SecretStore
ESO creates:       K8s Secret (synced from vault)
Pod sees:          Files at /run/secrets/cc-deck/<key> for each synced key
On delete:         ExternalSecret CR is deleted; synced Secret is garbage-collected by ESO
```

## Behavioral Requirements

### Mount Path

1. All credential sources MUST mount at `/run/secrets/cc-deck/`.
2. Each key in the Secret becomes a file at that path.
3. The volume mount MUST be `readOnly: true`.

### Multiple Sources

1. When both inline and existing-secret are provided, BOTH are mounted.
2. Inline credentials mount to `/run/secrets/cc-deck/inline/`.
3. Existing secret mounts to `/run/secrets/cc-deck/external/`.
4. A ConfigMap entry documents the mount layout for the agent to discover.

### Cleanup Ownership

| Source | Created by | Deleted on env delete |
|--------|-----------|----------------------|
| Inline Secret | cc-deck | Yes |
| Existing Secret | User | No |
| ExternalSecret CR | cc-deck | Yes |
| ESO-synced Secret | ESO | Yes (GC by ESO) |

### ESO Pre-flight

1. Before generating an ExternalSecret, MUST check for ESO CRDs via API discovery.
2. If `external-secrets.io` API group is not found, MUST return a clear error: "External Secrets Operator is not installed on this cluster".
3. The ExternalSecret MUST use `apiVersion: external-secrets.io/v1`.

### ExternalSecret Spec

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: cc-deck-<name>-eso
  namespace: <ns>
  labels: {standard labels}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: <secret-store-ref>
    kind: SecretStore
  target:
    name: cc-deck-<name>-eso-secret
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: <secret-path>
```
