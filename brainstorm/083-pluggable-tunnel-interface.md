# Brainstorm: Pluggable Tunnel Interface

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

CC Deck needs to expose local Zellij sessions to external collaborators. Different users have different networking setups: some are behind corporate firewalls, some have home servers with public IPs, some want zero-config solutions. A single tunnel backend won't serve everyone.

The tunnel interface should be pluggable from the start, with Cloudflare Tunnel as the default (free, works behind NAT), and additional backends for bore (self-hosted relay), Traefik IngressRoute (home/enterprise clusters), and potentially others.

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

Parked: Depends on findings from the session sharing spike (082). The interface design should be informed by what we learn about each backend's capabilities and constraints.

**Preliminary direction:** Go interface with Cloudflare as default. The `cc-deck share` command would:
- `cc-deck share start [--backend cloudflare|bore|traefik]` - enable Zellij web server, start tunnel, create token, print shareable URL
- `cc-deck share stop` - tear down tunnel, optionally stop web server
- `cc-deck share status` - show tunnel URL, active tokens, backend info

## Key Requirements

- Tunnel backend interface: `Start() (url string, err error)`, `Stop() error`, `Status() TunnelStatus`
- Default backend auto-detection: if `cloudflared` is installed, use it; otherwise prompt
- Token management: wrap `zellij web --create-token` and `--create-read-only-token`
- Configuration in `~/.config/cc-deck/config.yaml` for default backend and backend-specific settings

## Open Questions

- Should the tunnel interface also handle TLS termination, or always delegate that to Zellij or the tunnel service?
- How to handle tunnel URL stability? Cloudflare free tier gives random URLs; bore and Traefik can provide stable ones.
- Should `cc-deck share` automatically install `cloudflared` if missing (like `cc-deck` does with other tools)?
