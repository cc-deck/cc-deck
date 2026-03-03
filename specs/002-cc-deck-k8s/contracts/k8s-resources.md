# Contract: Kubernetes Resource Templates

## Resources Created per Session

For each `cc-deck deploy <name>`, the following resources are created:

| Resource | Name Pattern | Always? |
|----------|-------------|---------|
| StatefulSet | `cc-deck-<name>` | Yes |
| Headless Service | `cc-deck-<name>` | Yes |
| PVC (via volumeClaimTemplate) | `data-cc-deck-<name>-0` | Yes |
| NetworkPolicy | `cc-deck-<name>-egress` | Yes (unless `--no-network-policy`) |
| Route (OpenShift only) | `cc-deck-<name>` | Only if `route.openshift.io/v1` detected |
| Ingress (K8s only) | `cc-deck-<name>` | Only if no Route API |
| EgressFirewall (OpenShift only) | `cc-deck-<name>` | Only if `k8s.ovn.org/v1` detected |

## Labels Applied to All Resources

```yaml
labels:
  app.kubernetes.io/name: cc-deck
  app.kubernetes.io/instance: <session-name>
  app.kubernetes.io/managed-by: cc-deck
  app.kubernetes.io/component: claude-session
```

## StatefulSet Spec

- `replicas: 1`
- `serviceName: cc-deck-<name>`
- `volumeClaimTemplates` with configurable storage size (default 10Gi)
- Pod template mounts:
  - PVC at `/workspace` (persistent work directory)
  - ConfigMap at `/home/user/.config/zellij/config.kdl` (Zellij config)
  - Secret at appropriate path for credentials (backend-dependent)
- Container ports: 8082 (Zellij web server)
- Command: `zellij` (starts Zellij with web server, Claude available inside)

## Credential Mounting by Backend

### Anthropic
- Secret key: `api-key`
- Mounted as env var: `ANTHROPIC_API_KEY`

### Vertex AI
- Secret key: `credentials.json` (service account key)
- Mounted at: `/var/run/secrets/gcp/credentials.json`
- Env vars: `GOOGLE_APPLICATION_CREDENTIALS=/var/run/secrets/gcp/credentials.json`, `CLOUD_ML_REGION=<region>`, `GOOGLE_CLOUD_PROJECT=<project>`
- If no Secret specified: assumes Workload Identity (no mount needed)

## NetworkPolicy Rules

Default-deny egress with allowlisted exceptions:
1. DNS: UDP/TCP 53 to kube-dns pods in kube-system namespace
2. Backend API: TCP 443 to backend-specific CIDRs or EgressFirewall FQDN rules
3. User allowlist: Additional hosts via `--allow-egress` flags
