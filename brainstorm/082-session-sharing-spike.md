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
- `web_sharing` config: `"on"` (allow all), `"off"` (opt-in per session), `"disabled"` (never)

### Home k3s Cluster Infrastructure

- Traefik ingress with wildcard TLS for `*.tichny.org` (cert resolver: `tichny`, Let's Encrypt via Cloudflare DNS challenge)
- Authelia for forward authentication (can be omitted per-route for Zellij's own auth)
- MetalLB L2 load balancing (IP pool: `10.9.11.165-179`)
- HTTP-to-HTTPS redirect middleware
- Laptop at `10.9.11.19` on same subnet as cluster (direct routing possible)
- Pattern: IngressRoute without Authelia middleware exists (hedgedoc-public example)

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

## Spike Results

### 1. Zellij Web Server (VALIDATED)

**Status: Works perfectly.** Tested on Zellij 0.44.3.

- `zellij web` starts the server on `127.0.0.1:8082` (single command, sub-second)
- `zellij web --status` reports online/offline with version
- Token CLI is clean and automatable:
  - `zellij web --create-token` creates a login token (displayed once)
  - `zellij web --create-read-only-token` creates an observer token
  - `zellij web --list-tokens` lists token names and creation dates
  - `zellij web --revoke-token <name>` revokes by name
  - `zellij web --revoke-all-tokens` revokes everything
  - `--token-name <name>` flag for custom naming (conflicts with --create-token, must use separately)
- Session URL scheme: `http://127.0.0.1:8082/<session-name>` returns the web client HTML
- `web_sharing` defaults to `"off"` (sessions must opt-in). CC Deck would need to set this to `"on"` or handle per-session opt-in.
- `--daemonize` flag available for background operation
- `--stop` cleanly shuts down the server

**Key finding for CC Deck:** The entire Zellij web server lifecycle is controllable via CLI. CC Deck just wraps these commands. No custom web server needed.

### 2. Cloudflare Tunnel (VALIDATED)

**Status: Works end-to-end.** Token authentication through the tunnel succeeds.

- `cloudflared tunnel --url http://localhost:8082` creates a quick tunnel in ~5 seconds
- URL format: `https://<random-words>.trycloudflare.com`
- The Zellij web client HTML loads correctly through the tunnel (HTTP 200)
- **Token auth works through the tunnel**: `zellij attach https://<tunnel-url>/cc-deck-local --token <token> --insecure` successfully authenticated (the `--insecure` flag is needed for quick tunnels since the TLS cert is for `*.trycloudflare.com`, not the Zellij server's expected cert)
- WebSocket upgrade is handled automatically by Cloudflare (no config needed)

**Critical limitation: 100-second idle timeout** on free/Pro plans. If no data is transmitted for 100 seconds, Cloudflare closes the WebSocket (code 1006). This is a real problem for terminal sessions where the user pauses to read or think. Mitigations:
- Application-level heartbeat every 20-30 seconds (would need Zellij modification or a WebSocket proxy layer)
- Upgrading to Business/Enterprise plan (600-second timeout)
- For ephemeral pair programming, this is likely acceptable since both users are actively working

**Other Cloudflare Tunnel findings:**
- Quick tunnel URLs change on every restart (acceptable for ephemeral sharing)
- 200 concurrent request cap (irrelevant for 1-2 viewers)
- No Cloudflare-level auth on quick tunnels (Zellij's tokens are the only protection)
- Named tunnels with custom domains and Cloudflare Access are available for persistent setups
- SSE is NOT supported (but Zellij uses WebSocket, so irrelevant)

### 3. Traefik IngressRoute (DESIGN VALIDATED)

**Status: Architecture confirmed feasible, not yet tested end-to-end.**

The home cluster's Traefik setup supports this pattern:
- Laptop (`10.9.11.19`) is on the same L2 subnet as the cluster's MetalLB IPs (`10.9.11.165-179`)
- Traefik can proxy directly to the laptop's Zellij web server via a headless Service with manual Endpoints

**Proposed K8s resources:**

```yaml
# Service pointing to laptop IP
apiVersion: v1
kind: Service
metadata:
  name: zellij-proxy
  namespace: traefik
spec:
  ports:
  - port: 8082
    targetPort: 8082
---
apiVersion: v1
kind: Endpoints
metadata:
  name: zellij-proxy
  namespace: traefik
subsets:
- addresses:
  - ip: 10.9.11.19  # laptop IP (dynamic, needs updating)
  ports:
  - port: 8082
---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: zellij
  namespace: traefik
spec:
  entryPoints:
  - websecure
  routes:
  - kind: Rule
    match: Host(`zellij.tichny.org`)
    # NO Authelia middleware - Zellij has its own token auth
    services:
    - name: zellij-proxy
      port: 8082
  tls:
    certResolver: tichny
    domains:
    - main: '*.tichny.org'
      sans:
      - tichny.org
```

**Challenges:**
- Laptop IP is dynamic (DHCP). The Endpoints resource needs updating when the IP changes. Could use a CronJob or a small controller.
- Zellij web server must listen on `0.0.0.0` (not `127.0.0.1`) for the cluster to reach it, which requires TLS.
- This only works when the laptop is on the home network. For remote use, need Cloudflare Tunnel or similar.

### 4. Zellij Connected Client API (VALIDATED)

**Status: Sufficient API surface exists for a basic presence panel.**

The Zellij plugin API (`zellij-tile` 0.44.1, which the project uses) exposes:

**Connection tracking (aggregate):**
- `Event::SessionUpdate` fires when session state changes. Each `SessionInfo` includes:
  - `connected_clients: usize` (total connected clients, terminal + web)
  - `web_client_count: usize` (web clients specifically)
  - `web_clients_allowed: bool` (whether web sharing is enabled)
- `get_session_list()` (synchronous) returns the same `SessionInfo` data on demand

**Per-client details (partial):**
- `list_clients()` (async, results via `Event::ListClients`) returns `Vec<ClientInfo>`:
  - `client_id: u16`, `pane_id: PaneId`, `running_command: String`, `is_current_client: bool`
  - **Limitation:** No `is_web_client` field. Can't distinguish web vs. terminal per-client.

**Web server management from plugin:**
- `query_web_server_status()` returns `WebServerStatus::Online(base_url) | Offline | DifferentVersion`
- `start_web_server()` / `stop_web_server()` (requires `StartWebServer` permission)
- `generate_web_login_token()` / `list_web_login_tokens()` / `revoke_web_login_token()` / `revoke_all_web_tokens()`
- `share_current_session()` / `stop_sharing_current_session()`

**No connect/disconnect events.** Must poll via `SessionUpdate` subscription or timer + `list_clients()`.

**Known bug** ([Issue #4064](https://github.com/zellij-org/zellij/issues/4064)): Plugin instances continue running after client disconnects in multiplayer mode (orphaned `client_id: 0`).

**Impact on brainstorm 085 (Sidebar Presence Panel):** A meaningful presence panel IS feasible using `SessionInfo.web_client_count` from `SessionUpdate` events, plus `query_web_server_status()` for server state. Full per-user tracking (which web client is which) is not possible without upstream changes.

## Answered Open Questions

| Question | Answer |
|----------|--------|
| Does WebSocket work through Cloudflare Tunnel? | **Yes.** Automatic upgrade, no config needed. Token auth works through the tunnel. |
| Can Traefik proxy to laptop's Zellij? | **Yes, on same network.** Headless Service + Endpoints pointing to laptop IP. Dynamic IP is the main challenge. |
| What does Zellij expose about connected clients? | **Aggregate counts available.** `SessionInfo.web_client_count` and `connected_clients` via `SessionUpdate` events. No per-client web/terminal distinction. No connect/disconnect events (must poll). |
| How does sharing interact with CC Deck's multi-pane layout? | **Not yet tested.** Needs browser test. The collaborator should see the full Zellij layout including sidebar (it's the full terminal). |
| What happens on network drop? | **Not yet tested.** Cloudflare's 100-second idle timeout is a concern. Zellij's reconnection behavior needs testing. |

### 5. Plugin Instance Multiplication (CRITICAL BUG)

**Status: Blocker for session sharing. Needs fix before the feature is usable.**

When web clients connect to a shared session, Zellij loads the cc_deck.wasm plugin for each connected client. This causes:
- **44 sidebar instances** registered (from `sidebars=44` in debug log) instead of the expected 1 per tab
- Repeated `sidebar-hello` re-registrations (`plugin_id=0` on `tab=0` fires dozens of times)
- Constant render broadcast storms (`CTRL RENDER broadcast: sidebars=44`)
- Visible sidebar reshuffling and "Loading status-bar..." flickering
- **Orphaned instances persist after web clients disconnect** ([Zellij Issue #4064](https://github.com/zellij-org/zellij/issues/4064))
- Stopping the web server (`zellij web --stop`) does NOT clean up orphaned instances
- Only a session detach+reattach resets the plugin instance count

**Root cause:** Zellij's multiplayer model instantiates plugins per-client. The CC Deck controller/sidebar architecture assumes a small, stable number of plugin instances (1 controller + 1 sidebar per tab). Web clients violate this assumption.

**Required fixes before session sharing can ship:**
1. CC Deck controller must deduplicate sidebar registrations (ignore re-registrations from the same tab)
2. CC Deck controller must handle high instance counts gracefully (cap render broadcasts)
3. Upstream Zellij fix for orphaned plugin instances after client disconnect (#4064)
4. Consider whether the sidebar plugin should detect web client context and behave differently (lighter weight, no controller election)

**Impact on brainstorm 085 (Sidebar Presence Panel):** The `web_client_count` API is available but the plugin multiplication problem must be solved first. The presence panel would be rendered 44 times per update in the current state.

## Remaining Manual Tests

These need interactive testing (browser + second terminal):

1. **Browser access test**: Open `http://127.0.0.1:8082/cc-deck-local` in browser, authenticate with token, verify the full CC Deck layout (sidebar + panes) renders correctly
2. **Terminal attach from another machine**: Use a second device to `zellij attach https://<tunnel-url>/cc-deck-local --token <token>`
3. **Concurrent editing test**: Both host and collaborator type simultaneously, verify no input conflicts
4. **Read-only token test**: Verify observer cannot send input
5. **Network drop test**: Kill and restart the tunnel, verify reconnection behavior
6. **Idle timeout test**: Leave a Cloudflare Tunnel session idle for >100 seconds, observe what happens

### 6. Multiplayer Focus Synchronization (EXTRACTED)

**Status: Extracted to dedicated brainstorm 088.**

Clicking the sidebar on a second client switches focus on the first client's terminal (all Switch actions route through the controller on client_id=1). Design for two focus modes (synchronized + independent) is in [brainstorm 088](088-multiplayer-focus-modes.md).

## Future Features (Separate Brainstorms)

These topics are captured in dedicated parked brainstorm documents for later specification:

- **083**: Pluggable tunnel interface design (`cc-deck share start/stop`)
- **084**: Multi-backend sharing (SSH, OpenShell, K8s networking models)
- **085**: Sidebar presence panel (connected users, access levels)
- **086**: SpecKit pair programming hooks
