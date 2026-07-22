# Brainstorm: Session Sharing Spike

**Date:** 2026-07-22
**Status:** active

## Problem Framing

CC Deck sessions run inside Zellij, which since v0.43.0 includes a built-in web server for remote session access with token authentication, browser and terminal clients, and read-only observer tokens. The raw capability exists, but nobody uses it because it requires manual setup of networking, TLS, tokens, and tunnel configuration.

The goal is to enable pair programming on CC Deck sessions by making session sharing a one-command experience. Before building the full feature, we need to validate the underlying technology stack through a hands-on spike.

## Context Discovered

### Zellij 0.44.3 Remote Session Capabilities

- Built-in HTTPS web server (`zellij web`) with WebSocket session streaming
- Token-based authentication: login tokens (full access) and read-only tokens (observer only)
- Browser client (no install needed for collaborator) and terminal attach (`zellij attach https://...`)
- `base_url` config for reverse proxy subpath support
- TLS required for non-localhost; supports `mkcert` certificates
- Tokens are hashed, displayed once, revocable but not retrievable
- Config options: `web_server_ip`, `web_server_port`, `web_server_cert`, `web_server_key`

### Home k3s Cluster Infrastructure

- Traefik ingress with wildcard TLS for `*.tichny.org` (cert resolver: `tichny`)
- Authelia for forward authentication
- MetalLB L2 load balancing (IP pool: `10.9.11.165-179`)
- HTTP-to-HTTPS redirect middleware
- Multiple services already exposed (MCP servers, Grafana, HedgeDoc, etc.)

### Tunnel Options Evaluated

| Tool | Pros | Cons |
|------|------|------|
| Cloudflare Tunnel | Free, auto-HTTPS, no port forwarding, works behind NAT | Requires `cloudflared`, random URLs on free tier |
| bore | Rust, simple, self-hostable relay | No built-in HTTPS, needs relay server |
| Tailscale Funnel | Stable URLs on `*.ts.net`, easy setup | Requires Tailscale, 3 funnels per account |
| ngrok | Feature-rich, request inspector | Free tier limitations, random URLs |
| Home Traefik | Persistent `*.tichny.org` URLs, existing TLS, full control | Only works from home network (or with additional tunneling) |

## Approaches Considered

### A: Ship the full `cc-deck share` feature directly

- Pros: Complete experience from day one
- Cons: Too many unknowns (WebSocket through tunnels, Zellij client API for presence, multi-backend networking models). Risk of building on assumptions.

### B: Spike first, then spec based on findings

- Pros: Validates all technical assumptions before committing to an architecture. Findings feed directly into a grounded spec.
- Cons: Delays the feature by one cycle. But the spike itself is useful knowledge.

### C: Spec now, defer unknowns to implementation

- Pros: Faster to start coding
- Cons: Architecture decisions made without data. Multi-backend support (SSH, OpenShell, K8s) has different networking models that could invalidate early design choices.

## Decision

**Chosen: B (Spike first).** The Zellij web server is the foundation, and we haven't tested it in our specific setup. A spike answers the critical unknowns in hours, not days.

## Key Requirements (Spike Scope)

1. **Start Zellij web server locally** with TLS (using `mkcert`)
2. **Create login and read-only tokens**, test both
3. **Test browser access** to a running CC Deck session
4. **Test terminal attach** from a second machine via `zellij attach https://...`
5. **Run Cloudflare Tunnel** (`cloudflared tunnel --url localhost:8082`), verify WebSocket passthrough and token auth work end-to-end
6. **Test Traefik IngressRoute** on home k3s cluster as alternative backend
7. **Explore Zellij's connected client information**: what does Zellij expose about active connections? (needed for future sidebar presence panel)
8. **Document findings**: what works, what's clunky, what's missing, what CC Deck needs to wrap

## Open Questions

- Does Zellij's WebSocket upgrade work cleanly through Cloudflare Tunnel?
- Can Traefik on the home cluster proxy to a Zellij web server running on the laptop (requires tunnel from laptop to cluster, or running Zellij on a cluster node)?
- What information does Zellij expose about connected clients? Is there a pipe message, API, or log we can parse for the presence panel?
- How does session sharing interact with CC Deck's multi-pane layout? Does the collaborator see the sidebar, or just the active pane?
- What happens to the shared session when the host's network drops temporarily?

## Future Features (Separate Brainstorms)

These topics are captured in dedicated parked brainstorm documents for later specification:

- **083**: Pluggable tunnel interface design (`cc-deck share start/stop`)
- **084**: Multi-backend sharing (SSH, OpenShell, K8s networking models)
- **085**: Sidebar presence panel (connected users, access levels)
- **086**: SpecKit pair programming hooks
