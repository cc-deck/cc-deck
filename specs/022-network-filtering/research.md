# Research: 022-network-filtering

**Date**: 2026-03-16

## Decision 1: Forward Proxy Software

**Decision**: tinyproxy

**Rationale**: Smallest footprint (2.7-3.5 MB vs 250MB+ for Squid), simple config for allowlist mode (`FilterDefaultDeny Yes` + `FilterType fnmatch`), supports HTTPS CONNECT tunneling, logs denied requests. Sufficient for cc-deck's use case (domain allowlist, deny everything else).

**Alternatives considered**:
- Squid: Battle-tested ACL system with native `dstdomain` wildcards, but 50-100x larger image. Overkill for simple allowlist filtering.
- 3proxy: Lightweight multi-protocol proxy, but less documentation on domain-based filtering and fewer upstream container images.
- NGINX + ngx_http_proxy_connect_module: Requires custom compilation, community module for CONNECT support. Wrong tool for forward proxy use case.

**Configuration template**:
```
Port 8888
Timeout 600
FilterDefaultDeny Yes
FilterType fnmatch
Filter /etc/tinyproxy/whitelist
FilterURLs On
ConnectPort 443
ConnectPort 563
LogLevel Info
LogFile /var/log/tinyproxy/tinyproxy.log
```

**Upstream image**: `vimagick/tinyproxy` or `tunnm/tinyproxy` (2.7 MB)

## Decision 2: Domain Group Storage

**Decision**: Built-in groups embedded in Go binary + user overrides in `~/.config/cc-deck/domains.yaml`

**Rationale**: Follows existing cc-deck config patterns (adrg/xdg, gopkg.in/yaml.v3, non-fatal loading). Single file is simpler than directory of files. Built-in groups updated with releases, user groups extend or override without binary changes.

**Alternatives considered**:
- Directory of YAML files (`~/.config/cc-deck/domains/`): More files to manage, cross-references between groups awkward across files. Rejected for simplicity.
- Hardcoded only (paude approach): No user extensibility. Rejected because enterprise users need custom groups for internal infrastructure.

## Decision 3: Existing Code Integration Points

**Decision**: Extend existing `network.go` functions, add new `domains` package

**Rationale**: The existing code has clear integration points:
- `backendDNSNames()` returns hardcoded FQDN lists. Refactor to use domain group expansion.
- `BuildNetworkPolicy()` and `BuildEgressFirewall()` accept `AllowedEgress []string`. Feed expanded domain lists through this parameter.
- `AllowedEgress` in Profile already merges CLI flags with config. New `--allowed-domains` follows same pattern.
- Compose generation is new code (no existing compose module).

**Key existing code**:
- `cc-deck/internal/k8s/network.go`: NetworkPolicy + EgressFirewall builders (12 unit tests, 1 integration test)
- `cc-deck/internal/config/config.go`: XDG config loading, `gopkg.in/yaml.v3`
- `cc-deck/internal/config/profile.go`: `BackendType`, `AllowedEgress` field, profile validation
- `cc-deck/internal/build/manifest.go`: Manifest struct, `LoadManifest()`
- `cc-deck/internal/cmd/deploy.go`: `--allow-egress` flag, `DeployFlags` struct
- `cc-deck/internal/session/deploy.go`: Merges CLI flags with profile, calls network builders

## Decision 4: CLI Command Structure

**Decision**: `cc-deck domains` as top-level command (sibling to deploy, profile, image)

**Rationale**: Domain management is cross-cutting (affects manifest, deploy, runtime). Not nested under `image` or `deploy` because it serves both. Follows existing command patterns (noun + verb: `domains list`, `domains show`).

**Subcommands**:
- `init`: Seed domains.yaml with commented built-in definitions
- `list`: Show all available groups with source
- `show <group>`: Display expanded domains for a group
- `blocked <session>`: Show blocked requests from proxy logs (Podman only)
- `add <session> <domain>`: Add domain to running session
- `remove <session> <domain>`: Remove domain from running session

## Decision 5: Manifest Extension

**Decision**: Add `network` section to `cc-deck-build.yaml` Manifest struct

**Rationale**: The manifest already captures image config, tools, MCP sidecars. Network config (allowed domain groups) is a natural addition. Filtering is opt-in: no `network` section means no proxy sidecar.

**New fields**:
```go
type NetworkConfig struct {
    AllowedDomains []string `yaml:"allowed_domains,omitempty"`
}
```

Added to `Manifest` struct as `Network *NetworkConfig yaml:"network,omitempty"`.

## Decision 6: Compose Generation Architecture

**Decision**: New `internal/compose/` package for compose.yaml generation

**Rationale**: Compose generation does not exist yet. Clean separation from K8s code in `internal/k8s/`. The compose package reads the expanded manifest (including network config) and generates a complete `compose.yaml` with proxy sidecar, internal network, and env vars.

**Key outputs**:
- `compose.yaml`: Session container + proxy sidecar + internal network
- `.env.example`: Template for required credentials
- Proxy config file (tinyproxy.conf + whitelist)

## Decision 7: Podman Native Networking

**Decision**: Podman does not support domain-based egress filtering natively. Forward proxy sidecar is required.

**Rationale**: Podman's networking backend (netavark) supports network isolation and port-level firewall rules, but not FQDN-based egress filtering. An internal Podman network isolates the session container, and the proxy container bridges internal and external networks.
