# Brainstorm: Pluggable Tunnel Interface

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

CC Deck needs to expose local Zellij sessions to external collaborators. Different users have different networking setups: some are behind corporate firewalls, some have home servers with public IPs, some want zero-config solutions. A single tunnel backend won't serve everyone.

The tunnel interface should be pluggable from the start, with Cloudflare Tunnel as the default (free, works behind NAT), and additional backends for bore (self-hosted relay), Traefik IngressRoute (home/enterprise clusters), and potentially others.

## Spike Findings (from 082)

### Zellij Web Server CLI (validated)

The entire Zellij web server lifecycle is controllable via CLI. CC Deck wraps these commands directly:

| Operation | Command | Notes |
|-----------|---------|-------|
| Start server | `zellij web` / `zellij web --daemonize` | Binds 127.0.0.1:8082 by default |
| Stop server | `zellij web --stop` | Clean shutdown |
| Check status | `zellij web --status` | Returns online/offline + version |
| Create token | `zellij web --create-token` | Displayed once, stored hashed |
| Create read-only token | `zellij web --create-read-only-token` | Observer access only |
| List tokens | `zellij web --list-tokens` | Names + creation dates |
| Revoke token | `zellij web --revoke-token <name>` | Also revokes session tokens |
| Revoke all | `zellij web --revoke-all-tokens` | Nuclear option |

### Cloudflare Tunnel (validated)

- `cloudflared tunnel --url http://localhost:8082` works end-to-end with Zellij
- Token auth passes through the tunnel correctly
- WebSocket upgrade is automatic (no config needed)
- Quick tunnel URLs are random `*.trycloudflare.com` (changes per restart)
- **100-second idle timeout** on free plans is a real concern for terminal sessions
- No Cloudflare-level auth on quick tunnels (Zellij tokens are the only protection)
- Install: `brew install cloudflared`

### Traefik IngressRoute (design validated)

- Feasible via headless Service + Endpoints pointing to laptop IP
- Wildcard TLS cert for `*.tichny.org` available
- No Authelia middleware needed (Zellij has own auth)
- Challenge: laptop IP is dynamic, Endpoints need updating
- Only works on home network (laptop must be reachable from cluster)

## Approaches Considered

### A: Cloudflare-only, add backends later

- Pros: Ship fast, Cloudflare covers 80% of users
- Cons: Refactoring to add pluggability later is harder than designing it in

### B: Go interface with multiple backends

- Pros: Clean abstraction, each backend is independently testable, new backends are just a new implementation
- Cons: Slightly more upfront design work

### C: Shell-script backends (external executables)

- Pros: Users can add their own backends without modifying CC Deck
- Cons: Harder to manage lifecycle, error handling, and status reporting

## Decision

Parked: Ready for specification. The spike validated both Cloudflare and Traefik backends. Recommend approach B (Go interface) with Cloudflare as default.

**Preliminary direction:** Go interface with Cloudflare as default. The `cc-deck share` command would:
- `cc-deck share start [--backend cloudflare|bore|traefik]` - enable Zellij web server, start tunnel, create token, print shareable URL
- `cc-deck share stop` - tear down tunnel, optionally stop web server
- `cc-deck share status` - show tunnel URL, active tokens, backend info

## Key Requirements

- Tunnel backend interface: `Start() (url string, err error)`, `Stop() error`, `Status() TunnelStatus`
- Default backend auto-detection: if `cloudflared` is installed, use it; otherwise prompt
- Token management: wrap `zellij web --create-token` and `--create-read-only-token`
- Configuration in `~/.config/cc-deck/config.yaml` for default backend and backend-specific settings
- Zellij `web_sharing` config must be set to `"on"` (or CC Deck handles per-session opt-in)
- For Cloudflare quick tunnels: `--insecure` flag required on terminal attach (TLS cert mismatch)
- Output both browser URL and terminal attach command when sharing starts

## Answered Open Questions

| Question | Answer (from spike) |
|----------|---------------------|
| TLS termination | Delegate to the tunnel service (Cloudflare handles TLS; Traefik has cert resolver). Zellij's own TLS only needed when binding to non-localhost without a tunnel. |
| URL stability | Quick tunnels are ephemeral (acceptable for session sharing). Traefik backend gives stable `*.tichny.org` URLs. Named Cloudflare tunnels for persistent setup. |
| Auto-install cloudflared? | Yes, via `brew install cloudflared`. Check with `command -v cloudflared` first. |
| Idle timeout mitigation | 100-second Cloudflare timeout needs monitoring. Acceptable for active pair programming. May need heartbeat solution later. |
