# Data Model: Egress Recording Mode

## Entities

### RecordingSession

Ephemeral runtime state. Not persisted beyond the session.

| Field | Type | Description |
|-------|------|-------------|
| PodName | string | Podman pod name (auto-generated, e.g., `cc-deck-record-<timestamp>`) |
| WorkspaceImage | string | Image ref from manifest (`OpenShellImageRef()`) |
| SidecarImage | string | CoreDNS image ref (hardcoded constant) |
| LogVolume | string | Podman volume name for DNS log sharing |
| LogPath | string | Path inside sidecar where DNS log is written |
| SetupDir | string | Path to `.cc-deck/setup/` directory |
| ManifestPath | string | Path to `build.yaml` |

### DNSQueryLog

Parsed from CoreDNS log file after session exit. Intermediate in-memory representation.

| Field | Type | Description |
|-------|------|-------------|
| Entries | []DNSLogEntry | All parsed log entries |

### DNSLogEntry

| Field | Type | Description |
|-------|------|-------------|
| Domain | string | Queried domain name (without trailing dot) |
| QueryType | string | DNS record type: A, AAAA, CNAME, etc. |
| Timestamp | time.Time | When the query was made |

### RecordingResult

Output of post-session processing. Used to generate the summary report and manifest updates.

| Field | Type | Description |
|-------|------|-------------|
| ObservedDomains | []string | All unique domains after dedup and noise filtering |
| CoveredDomains | []CoveredDomain | Domains matched to existing catalog components or `allowed_domains` |
| NewDomains | []string | Domains not covered, to be appended to manifest |

### CoveredDomain

| Field | Type | Description |
|-------|------|-------------|
| Domain | string | The observed domain |
| CoveredBy | string | Component name or "allowed_domains" |

## Relationships

```
RecordingSession --produces--> DNSQueryLog (1:1)
DNSQueryLog --parsed-into--> RecordingResult (1:1)
RecordingResult.NewDomains --appended-to--> Manifest.Network.AllowedDomains
RecordingResult.CoveredDomains --matched-against--> PolicyComponent.Endpoints[].Host
```

## State Transitions

RecordingSession has no persisted state. The lifecycle is:

```
[not started] -> pod created -> containers running -> user exits -> log extracted -> pod removed -> [done]
```

If any step fails, cleanup removes the pod and volume. No partial state persists.

## Validation Rules

- Domain names: must be valid DNS names (no IP addresses, no ports)
- Deduplication: case-insensitive comparison, trailing dots stripped
- Noise filtering: applied after deduplication, before catalog matching
- Manifest write: `Network` field initialized to `&NetworkConfig{}` if nil before appending
