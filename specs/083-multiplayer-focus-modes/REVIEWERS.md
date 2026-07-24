# Review Guide: Multiplayer Focus Modes

**Generated**: 2026-07-23 | **Spec**: [spec.md](spec.md)

## Why This Change

In a multiplayer Zellij session with cc-deck, clicking the sidebar on a second client switches focus on the first client's terminal. This happens because all Switch actions route through the controller (which runs on client 1's plugin instance), and `focus_terminal_pane()` affects the controller's client, not the one that initiated the click. The second user's sidebar is actively harmful: it disrupts the primary user every time they interact with it.

## What Changes

The sidebar handles focus switching locally instead of routing through the controller. Each client's sidebar calls `focus_terminal_pane()` and `switch_tab_to()` directly from its own plugin instance, which Zellij's API routes to the correct client automatically. The sidebar then sends a lightweight focus-report to the controller for activity tracking and presence indicator data. Each sidebar also maintains its own session ordering based on its navigation history, so switching sessions on one client no longer reorders the other client's sidebar.

A new visual feature adds presence indicators: right-aligned colored blocks on session lines showing which other clients are focused on each session. Colors come from Zellij's multiplayer palette (the same colors used in the tab bar), accessed via a new `ModeUpdate` event subscription.

## How It Works

The implementation touches only the Rust WASM plugin (`cc-zellij-plugin/`). The core change replaces the sidebar's `send_action(Switch)` call with direct `focus_terminal_pane_wasm()` / `switch_tab_to_wasm()` calls from the sidebar plugin instance. A new `cc-deck:focus-report` pipe message notifies the controller of each client's focus for two purposes: (1) maintaining the existing activity tracking (last-active timestamps), and (2) populating a new `other_client_focus` field in the render payload.

The controller subscribes to `ModeUpdate` events to extract Zellij's `multiplayer_user_colors` palette. When building render payloads for each sidebar, it filters `client_focus` to exclude the target sidebar's own client, then includes the remaining entries with their assigned colors. The sidebar renders these as inverted-space colored blocks, right-aligned on the session line, truncating the session name if needed.

Per-client activation order is maintained as a `local_activation_order: Vec<u32>` in sidebar state. On each focus switch, the selected pane moves to the front. When rendering, the sidebar applies its local order before the controller's global sort.

## When It Applies

**Applies when**:
- Two or more clients are attached to the same Zellij session running cc-deck
- Any client clicks a session in the sidebar or uses keyboard navigation (Enter) to select
- Presence indicators appear on all sidebars when 2+ clients are connected and `ModeUpdate` has delivered the color palette

**Does not apply when**:
- Single-client sessions (zero behavioral change, no indicators shown, no performance overhead)
- Synchronized/pair-programming mode (deferred to a future spec)
- Per-client keyboard shortcut routing (blocked on Zellij PR #4094; keyboard shortcuts work for the primary client only)
- Read-only client detection (Zellij doesn't expose this; all clients treated equally)

## Key Decisions

1. **Local focus instead of controller-mediated focus.** Zellij's plugin API is inherently per-client: `focus_terminal_pane()` targets the calling plugin's client. The bug existed because cc-deck routed focus through the controller. The fix is to call the API directly from the sidebar. Alternatives: adding a `client_id` parameter to the controller's switch handler (unnecessary complexity given the API design).

2. **Controller-mediated presence tracking (not sidebar-to-sidebar).** The controller collects focus-reports and includes presence data in the render payload. Alternatives: direct sidebar-to-sidebar pipe messages (sidebars don't know each other's plugin IDs) or Zellij's built-in `TabInfo.other_focused_clients` (only gives tab-level focus, not session-level).

3. **ModeUpdate subscription for theme-aware colors.** Presence indicators use Zellij's `multiplayer_user_colors` palette via `ModeUpdate` events, ensuring visual consistency with Zellij's tab bar. Alternative: hardcoded RGB values (breaks with custom themes).

4. **Accept keyboard shortcut limitation.** Pipe messages don't carry `client_id` (Zellij PR #4094 unmerged), so the controller can't determine which client pressed a keybinding. Keyboard shortcuts work for the primary client only; other clients use mouse clicks. Alternative: forward keybindings to all clients (confusing, all clients would enter navigate mode simultaneously).

5. **Per-client activation order is transient sidebar state.** The local sort order lives in `SidebarState` and is lost on sidebar recreation (client detach/reattach). This is acceptable because it resets to the controller's global order. Alternative: persist per-client order in the controller (adds complexity for marginal benefit).

## Areas Needing Attention

- **Empirical validation needed**: The assumption that `focus_terminal_pane()` from a non-primary client's sidebar correctly focuses that client's pane has not been empirically tested (only validated from API documentation). If it fails silently, a fallback to controller routing is needed.
- **Render payload size increase**: Each render now includes `other_client_focus` data. With 10 clients focused on 10 different sessions, this adds 10 entries per payload. Serialization overhead should be negligible, but worth monitoring.
- **`build_render_payload` signature change**: Currently builds a single payload for all sidebars. The new design requires a `client_id` parameter to filter presence data per target. This changes how `broadcast_render()` works (calls `build_render_payload` per sidebar instead of once).

## Open Questions

No open questions identified. All ambiguities were resolved during the brainstorming and clarification phases. The only deferred item (color wrapping for `client_id > 10`) is an implementation detail handled by modulo-10 indexing.

## Review Checklist

- [ ] Key decisions are justified
- [ ] Scope matches the stated boundaries (independent focus only, no sync mode)
- [ ] Success criteria are achievable and measurable
- [ ] No unstated assumptions
- [ ] Keyboard shortcut limitation is documented
- [ ] Presence indicators use theme colors (not hardcoded)
- [ ] Single-client regression path verified (zero behavioral change)
- [ ] Focus-report contract is backward compatible (new message type, not modified existing)

---

<!-- Code phase sections are appended below this line by the phase-manager command -->
