# Research: Egress Recording Mode

**Date**: 2026-05-24

## R1: Podman Pod Lifecycle Management

**Decision**: Extend `internal/podman` package with pod create/remove operations using `podman pod create` and `podman pod rm` CLI commands.

**Rationale**: The existing `internal/podman` package has container-level operations (Run, Start, Stop, Remove, Exec, Cp, Volume) but no pod primitives. Pods are needed for shared network namespace between workspace and DNS sidecar containers. The codebase pattern is to shell out to the podman binary via `exec.Command`, so pod operations follow the same pattern.

**Alternatives considered**:
- Podman Go bindings library: rejected because the codebase consistently uses CLI invocation, not the Go API. Mixing approaches adds complexity.
- Docker Compose-style approach via `internal/compose`: rejected because compose generates YAML for external tools, while we need direct lifecycle control within a single `build record` command.

## R2: DNS Logger Sidecar Image and Configuration

**Decision**: Use the official `coredns/coredns` image with a minimal Corefile that enables the `forward` and `log` plugins. The Corefile is generated at runtime and mounted into the sidecar container.

**Rationale**: CoreDNS is lightweight (~50MB image), well-maintained, requires no special privileges, and produces structured log output. The `log` plugin outputs one line per query with the domain name, query type, and response code. The `forward` plugin passes all queries to upstream DNS for full internet access.

**Corefile template**:
```
.:53 {
    forward . /etc/resolv.conf
    log . {
        class all
    }
}
```

**Log format** (CoreDNS default): `[INFO] <client-ip>:<port> - <query-id> "A IN pypi.org. udp 28 false 512" NOERROR qr,rd,ra 68 0.023s`

The domain name can be extracted with a simple regex or field split from the quoted query string.

**Alternatives considered**:
- Custom minimal DNS logger: rejected because CoreDNS already does exactly what we need and is battle-tested.
- dnsmasq with logging: rejected because CoreDNS log format is more structured and easier to parse.
- tcpdump with TLS SNI: rejected because it requires `CAP_NET_RAW` and pcap parsing is complex.

## R3: Manifest Write-Back Strategy

**Decision**: Use `yaml.Marshal()` + `os.WriteFile()` to write the modified manifest back. No existing `SaveManifest()` function exists, so we add one.

**Rationale**: The `LoadManifest()` function uses `yaml.Unmarshal()`, so round-tripping through `yaml.Marshal()` is consistent. The `gopkg.in/yaml.v3` library handles formatting reasonably. User comments in build.yaml will be lost on write-back, but this is the same behavior as any other tool that modifies the manifest (the init template creates the file, subsequent modifications overwrite). The `/cc-deck.capture` command that writes to build.yaml establishes this pattern.

**Alternatives considered**:
- yaml.Node-based editing to preserve comments: rejected as over-engineering for this use case. The manifest is machine-generated initially and rarely hand-edited with comments.
- Appending raw YAML text: rejected because it risks format inconsistencies and duplicate keys.

## R4: Catalog Reverse Matching Implementation

**Decision**: Reuse the existing `LoadEmbeddedComponents()`, `LoadComponentTier()`, and `MatchComponent()` functions to load all catalog components, then iterate their endpoint hosts to build a domain-to-component lookup map. Compare observed domains against this map.

**Rationale**: The component loading and matching code already exists and handles all three tiers (embedded, catalog cache, user-local). Building a reverse index (domain -> component name) from the loaded components is straightforward and avoids duplicating the tier resolution logic.

**Alternatives considered**:
- Hardcoded domain-to-tool mapping: rejected because it would duplicate information already in the catalog components and drift over time.
- Fuzzy/substring matching: rejected because domain matching should be exact to avoid false positives.

## R5: Noise Filtering Implementation

**Decision**: Implement a `FilterNoise()` function with a hardcoded deny-list of suffix patterns and exact matches. Apply after deduplication.

**Deny-list**:
- Suffixes: `.local`, `.internal`, `.podman`, `.localhost`
- Exact: `localhost`
- Pattern: queries that are AAAA-only (no corresponding A record for the same domain)
- Reverse DNS: any domain matching `*.in-addr.arpa` or `*.ip6.arpa`

**Rationale**: These patterns cover all known Podman/container infrastructure noise without risk of filtering legitimate developer tool egress. The list is conservative by design (SC-006 requires zero false positives).
