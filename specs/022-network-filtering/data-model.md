# Data Model: 022-network-filtering

## Entities

### DomainGroup

A named collection of domain patterns that can be referenced by name in manifests and CLI flags.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Group identifier (no dots). e.g., `python`, `company` |
| Domains | []string | Domain patterns. e.g., `pypi.org`, `.github.com` |
| Extends | string | Optional. `"builtin"` to merge with built-in group of same name |
| Includes | []string | Optional. Other group names to merge in (recursive) |
| Source | enum | `builtin` (embedded in binary) or `user` (from domains.yaml) |

**Identity**: Name (unique across built-in + user groups)

**Validation**:
- Name must not contain dots (dots indicate literal domains)
- Domains must be non-empty (at least one domain pattern)
- Includes must not create cycles (detect and error)
- Extends can only be `"builtin"` (reserved value)

### DomainPattern

A string representing an allowed network destination.

| Variant | Format | Example | Semantics |
|---------|--------|---------|-----------|
| Exact | `host.example.com` | `api.anthropic.com` | Matches exactly this domain |
| Wildcard | `.example.com` | `.github.com` | Matches `example.com` and all subdomains |

**Wildcard Dedup Rule**: If `.example.com` (wildcard) is present, remove any exact match `example.com` or subdomain match `sub.example.com` from the list.

### NetworkConfig (Manifest Section)

Added to the existing `cc-deck-build.yaml` manifest.

| Field | Type | Description |
|-------|------|-------------|
| AllowedDomains | []string | Mix of group names and literal domain patterns |

**Resolution**: At deploy time, group names (no dots) are expanded via domain group system. Literal domains (contain dots) are passed through.

### DomainsConfig (User Config File)

Located at `~/.config/cc-deck/domains.yaml`.

| Field | Type | Description |
|-------|------|-------------|
| Groups | map[string]DomainGroup | Keyed by group name |

**File lifecycle**:
- Created by `cc-deck domains init` (seeded with commented built-in definitions)
- Missing file is non-fatal (empty config, built-in groups still available)
- User edits preserved across `cc-deck domains init` re-runs

### ProxyConfig (Generated)

Generated configuration for the forward proxy sidecar (Podman deployments).

| Field | Type | Description |
|-------|------|-------------|
| ListenPort | int | Proxy listen port (default: 8888) |
| AllowedDomains | []string | Expanded and deduplicated domain list |
| LogPath | string | Path to proxy access log |

### ComposeOutput (Generated)

Generated compose.yaml for Podman deployments with network filtering.

| Component | Role |
|-----------|------|
| Session container | Main cc-deck container, attached only to internal network |
| Proxy container | Forward proxy, attached to internal + external networks |
| Internal network | Isolates session container from external access |

## State Transitions

### Domain Group Resolution

```
Manifest allowedDomains
    ↓ (load built-in groups)
    ↓ (load user domains.yaml, merge/override)
    ↓ (expand group names → domain lists)
    ↓ (expand includes recursively, cycle detection)
    ↓ (add backend-specific domains automatically)
    ↓ (apply CLI overrides: +add, -remove, replace)
    ↓ (wildcard dedup)
Expanded domain list
    ↓
    ├─→ Proxy config (Podman: tinyproxy whitelist)
    ├─→ NetworkPolicy (K8s: CIDR-based egress rules)
    └─→ EgressFirewall (OpenShift: FQDN-based rules)
```

## Relationships

```
Manifest.Network.AllowedDomains  ──references──►  DomainGroup (by name)
DomainGroup.Includes             ──references──►  DomainGroup (recursive)
DomainGroup.Extends              ──references──►  Built-in DomainGroup
Profile.AllowedEgress            ──merges with──► CLI --allowed-domains
Expanded domains                 ──generates──►   ProxyConfig | NetworkPolicy | EgressFirewall
```
