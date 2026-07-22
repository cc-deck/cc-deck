# Brainstorm: Sidebar Presence Panel

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

When a CC Deck session is shared with collaborators, the host has no visibility into who's connected, what access level they have, or whether anyone is currently watching. A sidebar presence panel would show this information at a glance, similar to how collaborative editors show connected users.

## Spike Findings (from 082)

### Zellij Connection API: Sufficient for Basic Presence

The spike research revealed that Zellij 0.44.1+ (which the project uses) provides enough API surface for a meaningful presence panel:

**Available data:**

| Need | API | Mechanism |
|------|-----|-----------|
| Web client count | `SessionInfo.web_client_count` | `SessionUpdate` event or `get_session_list()` |
| Total client count | `SessionInfo.connected_clients` | Same as above |
| Web sharing enabled? | `SessionInfo.web_clients_allowed` | Same as above |
| Web server status | `query_web_server_status()` | Returns `Online(base_url)` / `Offline` |
| Individual client IDs | `list_clients()` | `Vec<ClientInfo>` with `client_id`, `pane_id`, `is_current_client` |
| Token management | Plugin API | `generate_web_login_token()`, `list_web_login_tokens()`, `revoke_*` |
| Server lifecycle | Plugin API | `start_web_server()`, `stop_web_server()`, `share_current_session()` |

**Limitations:**
- **No connect/disconnect events.** Must poll via `SessionUpdate` subscription (fires on session state changes) or timer + `list_clients()`.
- **No per-client web/terminal distinction.** `ClientInfo` has no `is_web_client` field. Only the aggregate `web_client_count` is available.
- **No access level per client.** Can't tell which client used a read-only vs. full token.
- **Known bug** ([#4064](https://github.com/zellij-org/zellij/issues/4064)): Orphaned plugin instances after client disconnect (client_id: 0).

### What This Means

A **V1 presence panel** showing "sharing active, N web clients connected, server URL" is fully achievable with the existing plugin API. A **V2** with per-user identity and access level tracking would require either upstream Zellij changes or a custom proxy layer.

## Approaches Considered

### A: Sharing indicator only (no user tracking)

- Pros: Simplest, just show "sharing active" + URL
- Cons: Doesn't leverage the available `web_client_count` data

### B: Aggregate presence panel (RECOMMENDED)

- Pros: Shows web client count, server status, and URL. Uses existing `SessionUpdate` events that the sidebar plugin already subscribes to. Token management via plugin API.
- Cons: Can't show individual user identities or access levels

### C: Full presence with proxy layer

- Pros: Per-user identity, access level tracking, connection history
- Cons: Requires a WebSocket proxy between Zellij and the tunnel. Adds latency and a failure point.

### D: Upstream Zellij contribution

- Pros: Add `is_web_client` to `ClientInfo` and connect/disconnect events. Benefits the community.
- Cons: Upstream acceptance timeline is unpredictable.

## Decision

Parked: Recommend **Approach B** (aggregate presence panel) for V1. The data is already available via `SessionInfo` in the `SessionUpdate` event, which the sidebar plugin already handles. The plugin API also covers token management and server lifecycle, so the entire sharing UX can live in the sidebar.

## Key Requirements

### V1 (Aggregate Presence Panel)

Sidebar section when sharing is active:

```
 SHARING
  ● Online: zellij.tichny.org
  👥 2 web clients
  🔑 3 tokens (1 read-only)
  [S]top  [T]oken  [C]opy URL
```

Implementation:
- Subscribe to `SessionUpdate` events, read `web_client_count` and `web_clients_allowed`
- Call `query_web_server_status()` on load and when sharing state changes
- Use `list_web_login_tokens()` for token count display
- Quick actions via keybindings: stop sharing, create token, copy URL to clipboard

### V2 (Per-Client Details, pending upstream)

- Individual client entries with web/terminal badge
- Access level indicator (full/read-only)
- Connection duration
- "Disconnect client" action

## Open Questions

- Should the sharing section replace or coexist with the session list in the sidebar?
- How frequently should `web_client_count` be polled if `SessionUpdate` doesn't fire often enough?
- Should the sidebar auto-collapse the sharing section when not actively sharing?
- File upstream issue for `is_web_client` on `ClientInfo` and connect/disconnect events?
