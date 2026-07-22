# Brainstorm: Sidebar Presence Panel

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

When a CC Deck session is shared with collaborators, the host has no visibility into who's connected, what access level they have, or whether anyone is currently watching. A sidebar presence panel would show this information at a glance, similar to how collaborative editors show connected users.

## Spike Findings (from 082)

### Zellij Connection API: Does Not Exist

The spike confirmed that Zellij 0.44.3 does not expose connected client information through any documented mechanism:

- **Plugin API (`zellij-tile`):** No events for web client connect/disconnect in the public event types
- **CLI (`zellij web --status`):** Reports only server online/offline, not connected client count
- **Share plugin (Ctrl+O, S):** Manages tokens only, does not show active connections
- **No pipe messages** for connection events

This is the single biggest blocker for a full presence panel.

### Workaround Options Identified

| Approach | Feasibility | Reliability |
|----------|------------|-------------|
| Parse Zellij server logs | Low effort | Fragile (log format not guaranteed) |
| Monitor port 8082 connections via `lsof`/`ss` | Low effort | Crude (counts TCP connections, not authenticated users) |
| Track at tunnel layer (Cloudflare metrics, Traefik access log) | Medium effort | Backend-specific, not portable |
| WebSocket proxy layer between Zellij and tunnel | High effort | Reliable but adds latency and complexity |
| Upstream Zellij contribution (pipe message for connection events) | High effort | Best long-term solution |

## Approaches Considered

### A: Sharing indicator only (no user tracking)

- Pros: Achievable today with no workarounds. Just show "sharing active" + tunnel URL in sidebar.
- Cons: No visibility into who's actually connected

### B: Connection count via `lsof`/`ss`

- Pros: Works without Zellij changes. Periodic polling from the plugin (via pipe to CLI helper).
- Cons: Can't distinguish authenticated vs. unauthenticated connections, or full vs. read-only users

### C: WebSocket proxy with connection tracking

- Pros: Full control over connection metadata. Can inject user identification.
- Cons: Adds a process between Zellij and the tunnel. Latency, complexity, failure mode.

### D: Upstream Zellij contribution

- Pros: Clean, reliable, benefits the entire Zellij community
- Cons: Time to merge is unpredictable. Feature may not be accepted upstream.

## Decision

Parked: Start with **Approach A** (sharing indicator only) for the initial release. It's achievable today and useful. Track upstream Zellij developments for connection events. Consider approach B (lsof polling) as a low-effort enhancement if users want to know "is anyone watching?"

## Key Requirements

### V1 (Sharing Indicator)
- Sidebar icon/badge when Zellij web server is running
- Show tunnel URL (copyable via pipe message)
- Show token count (from `zellij web --list-tokens` output)
- Quick actions via pipe: copy URL, create token, stop sharing

### V2 (Connection Tracking, pending API)
- Connected client count
- Access level per client (full/read-only)
- Client connect/disconnect notifications
- Token-to-client mapping

## Open Questions

- Should we file an upstream Zellij issue requesting connection events via the plugin API?
- Is the `lsof` approach reliable enough for a "good enough" connection count?
- Could the Share plugin's source code reveal internal APIs we could use?
