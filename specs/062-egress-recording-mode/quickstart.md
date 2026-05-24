# Quickstart: Egress Recording Mode

## What This Feature Does

Adds `cc-deck build record` to discover which external domains your workspace tools contact at runtime. Runs a DNS-logging session in a Podman pod, captures all DNS queries, filters noise, matches against the catalog, and appends new domains to `build.yaml` `network.allowed_domains`.

## Minimal Usage

```bash
# Prerequisites: workspace image must already be built
cc-deck build run --target openshell

# Start a recording session
cc-deck build record

# Inside the session: install deps, build, use Claude Code normally
pip install requests
go mod download
# ... exit when done (Ctrl+D or exit)

# Recording output: summary printed, build.yaml updated
# Then refresh to regenerate policy
cc-deck build refresh
```

## What Happens Under the Hood

1. `build record` creates a Podman pod with two containers sharing a network namespace
2. The workspace container runs your built image with full internet access
3. A CoreDNS sidecar captures every DNS query to a log file
4. On exit, cc-deck extracts the log, deduplicates domains, filters noise
5. Observed domains are matched against catalog components (Python, Go, Rust, etc.)
6. New domains (not already in catalog or manifest) are appended to `build.yaml`
7. A summary report shows what was found and what was added

## Key Files

| File | Purpose |
|------|---------|
| `cc-deck/internal/record/` | Recording session orchestration |
| `cc-deck/internal/record/dns.go` | DNS log parsing and noise filtering |
| `cc-deck/internal/record/catalog.go` | Reverse catalog matching |
| `cc-deck/internal/podman/pod.go` | Podman pod lifecycle operations |
| `cc-deck/internal/cmd/build.go` | CLI integration (`newBuildRecordCmd`) |
