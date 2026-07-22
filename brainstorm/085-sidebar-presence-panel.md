# Brainstorm: Sidebar Presence Panel

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

When a CC Deck session is shared with collaborators, the host has no visibility into who's connected, what access level they have, or whether anyone is currently watching. A sidebar presence panel would show this information at a glance, similar to how collaborative editors show connected users.

This depends on what Zellij exposes about connected web clients, which is one of the spike's (082) key exploration areas.

## Approaches Considered

### A: Parse Zellij logs/state

- Pros: No Zellij changes needed
- Cons: Fragile, depends on log format stability, may not contain connection info

### B: Zellij pipe messages for connection events

- Pros: Clean integration using the existing plugin pipe mechanism
- Cons: May require Zellij upstream changes or a custom Zellij plugin

### C: Tunnel-level connection tracking

- Pros: The tunnel backend (Cloudflare, Traefik) knows about active connections
- Cons: Different info per backend, doesn't distinguish authenticated vs. unauthenticated connections

## Decision

Parked: Entirely dependent on spike findings about what Zellij exposes. If Zellij has no client connection API, this feature needs a different approach (or an upstream contribution).

## Key Requirements

- Show sharing status in sidebar: active/inactive, tunnel URL
- Show connected clients: count, access level (full/read-only)
- Show token info: how many active tokens, when they were created
- Quick actions: copy share URL, create new token, revoke all tokens, stop sharing

## Open Questions

- Does Zellij emit any events when a web client connects or disconnects?
- Can we query the Zellij web server's state from within a plugin (via pipe or HTTP)?
- Should the presence panel be part of the main sidebar or a separate plugin pane?
- How to handle the case where Zellij has no connection info? (degrade gracefully to just "sharing active" with no user count)
