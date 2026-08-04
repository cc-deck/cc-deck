# Brainstorm: Pluggable Tunnel Interface

**Date:** 2026-07-22
**Status:** active

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

---

## Revisit: 2026-07-24

### Updated Problem Framing

The next sharing story should expose one complete Zellij session—not an individual CC Deck session, tab, or pane—to multiple external collaborators. Every connected user sees the full Zellij experience, including all tabs, panes, sidebars, and interactive controls.

The host needs two simple invitation types for the same session:

- **Collaborator:** full interactive access
- **Observer:** read-only access for following along

Both invitation types must support a browser link and a terminal `zellij attach` command. This is ephemeral host-controlled sharing, not a persistent hosted or multi-tenant service.

### New Approaches Considered

#### A: Host-controlled ephemeral share

- Pros: Small, secure lifecycle; one action starts access and one action removes it completely.
- Cons: Invitations and endpoint change whenever sharing restarts.

#### B: Persistent sharing profile

- Pros: Faster reuse and stable configuration between sessions.
- Cons: Adds persistent state and a larger security surface before it is needed.

#### C: Collaboration dashboard

- Pros: Rich sharing, presence, invitation, and shutdown experience in the sidebar.
- Cons: Combines the essential remote-access story with follow-up presence UX.

For endpoint exposure, three scopes were considered:

1. Cloudflare Quick Tunnel only
2. Pluggable exposure providers from the first version
3. User-supplied public endpoint only

### Updated Decision

Choose **host-controlled ephemeral sharing** with **pluggable exposure providers from the first version**. Cloudflare Quick Tunnel is the default initial provider, while the sharing behavior remains provider-independent.

Each share operation creates exactly two shared temporary credentials: one interactive token and one read-only token. The host receives browser and terminal invitations for both roles. Multiple collaborators may concurrently reuse the appropriate role token.

Stopping sharing is a complete teardown: revoke both tokens, disconnect access, and close the external endpoint.

### Updated Scope

**In scope:**

- Share exactly one complete Zellij session
- Multiple concurrent interactive collaborators
- Multiple concurrent read-only observers
- One shared temporary token per role
- Browser URL and terminal attach command for each role
- Pluggable exposure providers with Cloudflare Quick Tunnel as the first default provider
- Host-controlled start, status, and complete stop lifecycle
- Ensure only the intended Zellij session is shared

**Out of scope:**

- User accounts or collaborator identities
- Per-person tokens, labels, or individual revocation
- Persistent invitations or always-on sharing
- Sharing an individual CC Deck session, pane, or tab
- Presence UI, pair-programming roles, or hand-off controls
- A hosted multi-tenant service
- Multi-backend workspace exposure beyond the local Zellij session story

### Key Requirements

1. Starting sharing exposes only the selected complete Zellij session.
2. The host receives an interactive browser invitation and interactive terminal attach command.
3. The host receives a read-only browser invitation and read-only terminal attach command.
4. The interactive invitation grants the full Zellij session experience.
5. The observer invitation allows following the full session without sending input.
6. Multiple users can concurrently use each shared role token.
7. Exposure-provider choice does not alter the invitation or lifecycle experience.
8. Stopping sharing revokes both tokens and closes the public endpoint.

### Open Threads

- Verify whether token revocation and endpoint shutdown immediately disconnect already-authenticated browser and terminal clients; define additional enforcement if they do not.
- Confirm how the product prevents a web-server login token from reaching any Zellij session other than the explicitly shared session.
- Define the minimum provider capability contract without pulling later multi-backend sharing into this story.
