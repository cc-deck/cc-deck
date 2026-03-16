# 22: Network Security and Domain Filtering

**Date**: 2026-03-16
**Status**: brainstorm
**Feature**: Network filtering for containerized sessions
**Inspired by**: paude project (Ben Browning)

## Problem

Containers running with `--dangerously-skip-permissions` (or equivalent YOLO modes) have unrestricted network access. An AI agent could exfiltrate code or secrets via HTTP/HTTPS requests, SSH git pushes, or other network protocols. cc-deck needs a defense-in-depth network filtering layer that restricts outbound connections to known-good domains only.

## paude's Approach

paude provides network isolation using a Squid proxy sidecar. Key architecture and design choices:

**Squid Proxy Architecture**: Sidecar container on internal Podman network. Session container joins only the internal network, proxy container joins both internal and external. All outbound HTTP/HTTPS traffic is forced through the proxy.

**Domain Alias System**: Predefined groups are **hardcoded in Python** (`src/paude/domains.py`). Nine groups ship with paude: `vertexai` (7 domains including regex for regional endpoints), `python` (3 domains), `golang` (5 domains), `nodejs` (3 wildcard domains), `rust` (3 domains), `github` (6 domains), `claude` (2 wildcard domains), `cursor` (4 wildcard domains), `gemini` (2 domains). Users **cannot define custom domain groups**; they can only reference predefined aliases or pass raw domain strings.

**Special Values**: `"default"` expands to `BASE_ALIASES` (`vertexai`, `python`, `github`) plus agent-specific extras. `"all"` disables the proxy entirely (unrestricted network).

**Agent-Specific Extras**: Each agent declares additional domain aliases. Claude adds `claude`, Cursor adds `cursor`, Gemini adds `gemini` + `nodejs`. These are merged with `"default"` during expansion.

**Wildcard Dedup Logic**: Squid treats `.example.com` as matching both exact and all subdomains. The `remove_wildcard_covered()` function deduplicates before injecting into Squid config to prevent fatal errors. The proxy entrypoint script (`entrypoint.sh`) deduplicates again at runtime.

**Regex Patterns**: Prefix `~` denotes regex domains (e.g., `~aiplatform\.googleapis\.com$`). These use a separate Squid ACL (`allowed_domains_regex`) and are excluded from wildcard dedup.

**Configuration Layers** (precedence order):
1. CLI `--allowed-domains` (highest, completely replaces other sources)
2. Project config `paude.json` + user defaults `~/.config/paude/defaults.json` (merged/union)
3. Built-in fallback: `["default"]`

**Post-Creation Modification**: `paude allowed-domains <session> --add/--remove/--replace` (mutually exclusive operations). Podman recreates the proxy container; OpenShift patches the Deployment (automatic pod rollout).

**Audit Trail**: `blocked-domains` command reads Squid access log.

**Threat Model**: HTTP/HTTPS exfiltration blocked by proxy, SSH/git push blocked by network isolation (only HTTP/HTTPS through proxy), all traffic forced through proxy.

**Limitation**: No user-defined domain groups. Users who need the same set of internal domains across multiple sessions must list them individually each time.

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Proxy software | Squid (Podman), NetworkPolicy (K8s/OpenShift) | Squid gives domain-level filtering for Podman; K8s native policies for cluster deployments |
| Domain groups | Built-in groups (embedded in Go binary) + user-extensible via `domains.yaml` | Improves on paude's hardcoded-only approach; users can define custom groups |
| Default policy | Deny-all with explicit allowlist | Security-first approach, matches paude's model |
| Domain config storage | Single `~/.config/cc-deck/domains.yaml` file | Simpler than a directory of files; cross-references between groups are easier in one file |
| Domain config in manifest | `cc-deck-build.yaml` `network.allowedDomains` references groups by name | Manifest declares intent, deploy enforces it |
| Group vs domain disambiguation | Convention: no dot = group name, contains dot = domain | Simple, covers 99% of cases; no prefix syntax needed |
| Logging | Access log for blocked attempts | Essential for debugging and security auditing |
| OpenShift integration | Use EgressFirewall CRD where available | Leverages existing `BuildEgressFirewall()` function in cc-deck |
| K8s NetworkPolicy | DNS-aware policies (Cilium/Calico) or proxy fallback | Standard NetworkPolicy uses CIDR-based rules, advanced CNIs support FQDN |

## Domain Group Architecture

### Three-Layer Resolution

| Layer | Source | Mutability |
|-------|--------|------------|
| **Built-in groups** | Embedded in Go binary (`internal/network/domains.go`) | Updated with cc-deck releases |
| **User groups** | `~/.config/cc-deck/domains.yaml` | User-managed, extends/overrides built-ins |
| **Manifest references** | `cc-deck-build.yaml` `network.allowedDomains` | Per-image, references groups by name + inline domains |

### Built-in Groups

Embedded in the Go binary, always available:

| Group | Domains |
|-------|---------|
| `python` | `pypi.org`, `files.pythonhosted.org`, `pypi.python.org` |
| `nodejs` | `.npmjs.org`, `.yarnpkg.com`, `.nodejs.org` |
| `rust` | `crates.io`, `static.crates.io`, `index.crates.io` |
| `golang` | `proxy.golang.org`, `sum.golang.org`, `go.dev`, `.golang.org` |
| `github` | `github.com`, `.github.com`, `raw.githubusercontent.com`, `api.github.com`, `codeload.github.com` |
| `gitlab` | `gitlab.com`, `.gitlab.com` |
| `docker` | `registry-1.docker.io`, `.docker.io`, `production.cloudflare.docker.com` |
| `quay` | `quay.io`, `.quay.io` |

Backend domains (automatic, not user-selectable):

| Backend | Domains |
|---------|---------|
| Anthropic | `api.anthropic.com`, `.claude.ai`, `statsigapi.net` |
| Vertex AI | `oauth2.googleapis.com`, `aiplatform.googleapis.com`, regional variants, `.googleapis.com` |

### User Domain Configuration: `~/.config/cc-deck/domains.yaml`

A single file for user-defined and extended domain groups:

```yaml
# Extend a built-in group (merges with built-in domains)
python:
  extends: builtin
  domains:
    - pypi.internal.corp

# New custom group
company:
  domains:
    - artifacts.internal.corp
    - git.internal.corp
    - registry.internal.corp

# Group that includes other groups (recursive expansion with cycle detection)
dev-stack:
  includes:
    - python
    - golang
    - company
  domains:
    - monitoring.internal.corp

# Override a built-in completely (no extends, replaces it)
nodejs:
  domains:
    - registry.internal.corp/npm
    - nodejs.org
```

**Semantics**:
- `extends: builtin`: Merges user domains with the built-in group of the same name
- `includes`: Pulls in all domains from referenced groups (recursive, with cycle detection)
- No `extends`: Replaces the built-in group entirely
- Groups defined here that do not match a built-in name are new custom groups

### Seeding the User Config

```bash
# Export all built-in groups to ~/.config/cc-deck/domains.yaml (commented out)
cc-deck domains init

# List all available groups (built-in + user-defined)
cc-deck domains list

# Show expanded domains for a group
cc-deck domains show python

# Show expanded domains for a group (with user overrides applied)
cc-deck domains show dev-stack
```

`cc-deck domains init` writes the built-in definitions as commented YAML, so users can see what exists and uncomment to extend.

### Manifest Integration

The `cc-deck-build.yaml` manifest references groups by name. Group names (no dot) are resolved at deploy time against built-in + user definitions. Domains (contain a dot) are passed through as-is.

```yaml
network:
  allowedDomains:
    - github             # group name -> expanded at deploy time
    - python             # group name -> expanded at deploy time
    - dev-stack          # user-defined group from domains.yaml
    - special.corp.com   # inline domain (contains dot, passed through)
```

### Domain Population During Image Build

`cc-deck build init` creates the manifest scaffold **before** any analysis. The `network.allowedDomains` section starts empty.

`/cc-deck.extract` analyzes project repositories and maps detected tools to domain groups:

| Detected artifact | Suggested group |
|-------------------|-----------------|
| `go.mod` | `golang` |
| `pyproject.toml`, `.python-version`, `requirements.txt` | `python` |
| `package.json`, `.nvmrc` | `nodejs` |
| `Cargo.toml` | `rust` |
| `.github/` | `github` |
| `.gitlab-ci.yml` | `gitlab` |

The extract command writes group names into `network.allowedDomains`. It does not need to know the actual domains in each group. Resolution happens at deploy time.

### Domain Expansion at Deploy Time

When `cc-deck deploy` runs, it expands all group references:

1. Load built-in groups
2. Merge with `~/.config/cc-deck/domains.yaml` (extends, overrides, new groups)
3. Read `network.allowedDomains` from manifest
4. Expand each group name recursively (resolve `includes`)
5. Add backend-specific domains automatically (based on configured backend)
6. Apply CLI overrides (`--allowed-domains +custom.com`, `--allowed-domains -rust`)
7. Deduplicate (wildcard dedup: `.example.com` covers `example.com` and subdomains)
8. Generate target-specific output (Squid ACLs for Podman, NetworkPolicy for K8s, EgressFirewall for OpenShift)

## Adaptation: Podman

cc-deck adopts the paude sidecar proxy approach for Podman deployments:

**Sidecar Proxy**: Squid container runs alongside the cc-deck session container. Standard Squid image with a generated config mount (no custom proxy image needed).

**Internal Network Isolation**: Session container joins only the internal Podman network (no external access). Squid container joins both internal and external networks.

**Forced Proxy**: All HTTP/HTTPS traffic from the session container goes through the Squid proxy via environment variables (`HTTP_PROXY`, `HTTPS_PROXY`). DNS traffic allowed for name resolution, but all other protocols denied by network isolation.

**CLI Integration**: `cc-deck deploy --compose` generates proxy sidecar in `compose.yaml` with appropriate network configuration and expanded domain ACLs.

**Runtime Domain Management**: `cc-deck domains` subcommand for managing domain allowlists on running sessions. Modifying domains recreates the proxy container with updated Squid ACLs.

## Adaptation: Kubernetes

cc-deck already has partial NetworkPolicy support in `cc-deck/internal/k8s/network.go`. This brainstorm extends it:

**Existing Foundation**: `BuildNetworkPolicy()` function creates default-deny egress policies with DNS exceptions and backend-specific CIDR rules (Anthropic, Vertex AI).

**Current Limitation**: Standard Kubernetes NetworkPolicy only supports IP/CIDR-based rules, not FQDN. The existing code resolves hostnames to IPs at policy creation time using `net.LookupIP()`. This breaks when IPs change (CDNs, cloud services with dynamic IPs).

**DNS-Aware NetworkPolicy**: Advanced CNI plugins (Cilium, Calico) support FQDN-based egress rules via CRDs. cc-deck should detect the CNI type and generate appropriate FQDN policies when available.

**Proxy Fallback**: For clusters without DNS-aware NetworkPolicy, fall back to Squid sidecar (same as Podman approach). This gives domain-level filtering even on standard K8s.

**Resource Generation**: `cc-deck deploy --k8s` generates NetworkPolicy resources with FQDN rules where supported, otherwise generates sidecar proxy configuration.

**Per-Session vs Per-Namespace**: NetworkPolicy applies at namespace level (affects all Pods with matching labels). Consider per-session policies for multi-tenant namespaces, or dedicated namespace per session.

## Adaptation: OpenShift

OpenShift provides native DNS-based egress filtering via OVN-Kubernetes:

**EgressFirewall CRD**: OpenShift-specific resource (k8s.ovn.org/v1) already supported in cc-deck. The existing `BuildEgressFirewall()` function creates FQDN-based allowlists with a default-deny rule.

**Current Backend Support**: Existing code includes FQDN rules for Anthropic (`api.anthropic.com`) and Vertex AI (including regional endpoints like `us-central1-aiplatform.googleapis.com`).

**Extension for Domain Groups**: Refactor `backendDNSNames()` to use the domain group expansion system. Replace hardcoded host lists with group references resolved through the same three-layer model.

**Proxy Sidecar Optional**: EgressFirewall provides namespace-level domain filtering natively. Proxy sidecar only needed for finer-grained control (e.g., per-session isolation within a shared namespace, or URL path filtering).

**Integration with Existing Code**: `BuildEgressFirewall()` already supports user-specified allowed egress hosts via `AllowedEgress []string` parameter. Feed expanded domain lists from the group resolution system.

## CLI Interface

```bash
# Domain group management
cc-deck domains init                      # Seed ~/.config/cc-deck/domains.yaml with built-in definitions
cc-deck domains list                      # List all groups (built-in + user-defined)
cc-deck domains show <group>              # Show expanded domains for a group

# Deploy with domain groups from manifest
cc-deck deploy --compose <build-dir>

# Deploy with explicit domain groups (overrides manifest)
cc-deck deploy --compose <build-dir> --allowed-domains vertexai,github,python

# Add/remove groups at deploy time (relative to manifest defaults)
cc-deck deploy --compose <build-dir> --allowed-domains +nodejs
cc-deck deploy --compose <build-dir> --allowed-domains -rust

# Runtime domain management on running sessions
cc-deck domains add <session-name> custom.example.com
cc-deck domains remove <session-name> pypi.org
cc-deck domains blocked <session-name>    # Show blocked requests from proxy log
```

## Implementation Notes

**Domain Expansion**: Create `internal/network/domains.go` with built-in group definitions and expansion logic. Load user overrides from `~/.config/cc-deck/domains.yaml` via `adrg/xdg`. Recursive `includes` resolution with cycle detection.

**Squid Configuration**: Generate Squid ACLs from expanded domain list. Wildcard domains use `acl allowed_domains dstdomain .example.com`. Regex domains (prefix `~`) use separate `acl allowed_domains_regex dstdom_regex`. Wildcard dedup runs before ACL generation.

**Squid Image**: Use the standard Squid container image with a generated `squid.conf` mounted as a volume. No custom proxy image to maintain.

**Podman Network**: Create internal network in `cc-deck deploy --compose` output. Session container uses `networks: [internal]`, proxy uses `networks: [internal, default]`.

**K8s CNI Detection**: Add CNI detection logic to determine if FQDN-based NetworkPolicy is supported. Check for Cilium CRDs (`cilium.io/v2/CiliumNetworkPolicy`) or Calico CRDs (`projectcalico.org/v3/GlobalNetworkPolicy`).

**Audit Log Integration**: For Podman deployments, mount Squid access log as a volume. `cc-deck domains blocked` parses the log for denied requests.

**Testing**: Integration tests should verify that allowed domains succeed and blocked domains fail. Test both Podman (proxy) and K8s (NetworkPolicy/EgressFirewall) paths.

## Open Questions

**Mandatory vs Opt-In**: Should domain filtering be mandatory or opt-in for Podman deployments? Default to enabled for security, but provide `--disable-network-filtering` escape hatch?

**Dynamic Domains**: How to handle CDNs and package mirrors with rotating subdomains? Support regex patterns (like paude's `~` prefix), or rely on wildcard matching (`.example.com`)?

**Sidebar Integration**: Should blocked-domain logs be visible in the sidebar plugin? Would require pipe communication from session to plugin. Could show notification badge when blocks occur.

**Image Metadata**: Should expanded domain groups be stored as OCI labels on the built image? This would let `cc-deck deploy` auto-detect expected domains without the user re-specifying the manifest. The manifest declares intent, the image carries the metadata, the deploy enforces it.

**SSH and Git Push Blocking**: Squid only filters HTTP/HTTPS. Should we add iptables rules to block SSH (port 22) entirely, or allow SSH to specific domains (e.g., `github.com` for git push over SSH)?

**Multi-Backend Sessions**: If a session uses multiple AI backends (e.g., Anthropic for fast queries, Vertex for long-running tasks), how should domain allowlists merge? Union of both backend defaults?

**Egress to Internal Services**: Should there be a separate allowlist for cluster-internal traffic (e.g., other services in the same namespace, monitoring endpoints)? K8s NetworkPolicy handles this separately from external egress.
