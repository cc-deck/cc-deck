# Brainstorm: Egress Recording Mode

**Date:** 2026-05-24
**Status:** active

## Problem Framing

Building a correct network policy for a workspace image requires knowing which external domains the tools actually contact at runtime. Today, cc-deck relies on a static catalog of known endpoints per tool ecosystem (e.g., `pypi.org` for Python, `crates.io` for Rust). This works for common tools but misses project-specific dependencies, MCP server backends, internal APIs, and any tool that contacts endpoints not in the catalog.

The catalog-driven approach is also one-directional: the operator declares what should be allowed, and hopes it covers everything. There is no feedback loop from actual runtime behavior back into the policy.

This feature introduces a "recording mode" that runs the workspace image with full network access, observes which domains are contacted during a normal work session, and feeds the results back into the build process as catalog-compatible component files and `allowed_domains` entries. The mechanism must be generic (not OpenShell-specific) so it works across all target types: plain Podman containers, OpenShell sandboxes, and (eventually) Kubernetes with NetworkPolicies.

## Approaches Considered

### A: Podman Pod with DNS Logger Sidecar (Chosen)

Run a CoreDNS sidecar in a Podman pod alongside the workspace container. CoreDNS logs every DNS query to a volume-mounted file. The workspace container resolves through `localhost:53` (via Podman's `--dns` flag). All network traffic flows freely. On session exit, cc-deck parses the DNS log, deduplicates domains, matches against known catalog components, and generates output files.

```
+--- Podman Pod ---------------------------------+
|                                                |
|  +-------------+      +------------------+     |
|  |  Workspace  |----->|  CoreDNS logger  |     |
|  |  container  |      |  port 53         |     |
|  |  (built     |      |  query log -->   |     |
|  |   image)    |      |  /data/dns.log   |     |
|  +-------------+      +------------------+     |
|        |                      |                 |
|        v                      v                 |
|   full internet          volume mount           |
+------------------------------------------------+
```

- Pros: Catches every domain (all tools do DNS). No special privileges. Lightweight sidecar. No proxy bypass issues. Simple structured log output. Works with any container image.
- Cons: No binary-to-endpoint mapping (acceptable, handled by catalog components and two-pass probing). No port accuracy (assume 443 for HTTPS, correct for 99% of dev tool egress). May capture DNS prefetch noise.

### B: Podman Pod with Forward Proxy Sidecar (Squid/tinyproxy)

Run a forward HTTP proxy in the pod. Set `HTTP_PROXY`/`HTTPS_PROXY` in the workspace container. The proxy logs all CONNECT requests with domain and port.

- Pros: Gets exact domain:port for every HTTPS CONNECT. Well-structured access log.
- Cons: Tools that ignore `HTTP_PROXY` are missed (git SSH, some CLI tools, hardcoded resolvers). More complex sidecar configuration.

### C: tcpdump with TLS SNI Extraction

Run tcpdump on the pod's network interface. Capture TLS ClientHello packets and DNS queries. Parse the pcap file on teardown.

- Pros: Catches everything at the network level. Gets domain + port from TLS SNI.
- Cons: Needs `CAP_NET_RAW`. Parsing pcap is complex. Captures noise.

## Decision

**Approach A: Podman Pod with DNS Logger Sidecar.** DNS capture is the most reliable and simplest mechanism. Every tool resolves DNS, regardless of whether it uses HTTP proxies, SSH, or direct connections. CoreDNS is lightweight, requires no special privileges, and produces structured logs.

Binary-to-endpoint mapping is not needed at this stage. The existing catalog components already declare which binaries access which endpoints. The recording mode discovers WHAT is accessed. The catalog and two-pass probing determine WHO accesses it.

## Key Requirements

1. **New command: `cc-deck build record [setup-dir]`** that:
   - Builds the workspace image from the manifest (reuses existing `build run` pipeline)
   - Creates a Podman pod with the workspace container + CoreDNS sidecar
   - Attaches the user interactively to the workspace container
   - On exit, extracts and parses the DNS query log

2. **CoreDNS sidecar configuration:**
   - Forward all queries to upstream DNS (full internet access)
   - Log every query to a file (domain, query type, timestamp)
   - Mount log file via a shared volume for extraction

3. **Post-session processing:**
   - Deduplicate observed domains
   - Filter infrastructure noise (localhost, container DNS, internal Podman domains, container registries used during build)
   - Match against known catalog components (e.g., `pypi.org` matches the `python` component, `api.anthropic.com` matches `claude-code`)
   - Group remaining domains for manual classification or as a new custom component

4. **Output: catalog-compatible component YAML files** in `.cc-deck/setup/openshell/policies/`:
   ```yaml
   name: recorded-session
   match:
     tools: []
   endpoints:
     - host: pypi.org
       port: 443
     - host: files.pythonhosted.org
       port: 443
   ```

5. **Output: `build.yaml` updates** to `network.allowed_domains`:
   ```yaml
   network:
     allowed_domains:
       - pypi.org
       - files.pythonhosted.org
   ```

6. **Integration with `build refresh`**: Recorded component files serve as user-local overrides, picked up by the existing policy assembly pipeline.

7. **Target-agnostic design**: The recording mechanism (Podman pod + DNS sidecar) is independent of the target type. The output feeds into whichever policy system the target uses:
   - OpenShell: component YAML files assembled into `/etc/openshell/policy.yaml`
   - Container (Podman): `allowed_domains` for iptables rules
   - Kubernetes: `allowed_domains` for NetworkPolicy generation

## Key Insight: Catalog Reverse Matching

After recording, cc-deck can match observed domains against the catalog in reverse. Instead of "which endpoints does this tool need?" (forward), it asks "which catalog component explains this observed domain?" (reverse). This auto-classifies most domains and highlights only the truly unknown ones for manual review.

For example, if the DNS log contains `pypi.org`, `files.pythonhosted.org`, and `api.anthropic.com`:
- `pypi.org` and `files.pythonhosted.org` match the `python` catalog component
- `api.anthropic.com` matches the `claude-code` catalog component
- Any unmatched domains are grouped into a `recorded-custom` component

This tells the user: "Your catalog already covers these domains. These additional domains were observed and need to be added."

## Open Questions

- Which CoreDNS image to use? A minimal custom image with only the `forward` and `log` plugins would be smallest. Alternatively, the official `coredns/coredns` image works out of the box.
- Should `cc-deck build record` also support a non-interactive mode (run a predefined script inside the container for CI-driven recording)?
- How to filter DNS noise: container registry queries during podman pull, internal Podman DNS names, mDNS, AAAA queries for IPv4-only services?
- Should recorded domains be merged into existing `allowed_domains` or presented as a diff for the user to review?
- Should the command support recording multiple sessions and merging the results (iterative discovery)?
- How does this relate to the OpenShell supervisor's OCSF logging? When the target is OpenShell, should `build record` additionally parse OCSF logs for binary-to-endpoint mapping as an enhancement?
