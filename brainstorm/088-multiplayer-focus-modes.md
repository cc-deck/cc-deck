# Brainstorm: Multiplayer Focus Modes

**Date:** 2026-07-21
**Status:** active

## Problem Framing

In a multiplayer Zellij session with cc-deck, clicking the sidebar on a second client switches focus on the first client's terminal. This happens because all Switch actions route through the controller (which runs on client_id=1), and `focus_terminal_pane()` affects the controller's client, not the one that initiated the click.

Discovered during resilience testing for spec 082 (multiplayer plugin resilience). The client_id-aware architecture is in place, but focus control is still single-client.

Users sharing a session need two distinct collaboration styles: pair programming (stay in sync) and observation (browse independently). Neither works correctly today because the action routing always affects client 1.

## Approaches Considered

### A: Two explicit modes (synchronized + independent)

The sidebar supports two multiplayer focus modes, toggled via the sidebar header or a keybinding:

**Synchronized mode (pair programming):** Focus changes on either client are mirrored to all clients. When client 1 clicks a session, client 2's terminal also switches. Both users always see the same pane. The sidebar broadcasts focus changes to all client-matching sidebars.

**Independent mode (observation):** Each client controls its own focus. The sidebar handles Switch locally by calling `focus_terminal_pane` directly from the sidebar plugin (which runs on the clicking client's client_id) instead of routing through the controller.

- Pros: Covers both collaboration styles. Clean separation. Users can switch modes as needed.
- Cons: More complex. Requires a new message type for focus sync broadcast. Mode state needs to be shared or per-client.

### B: Independent-only (simplify)

Remove the controller as the focus intermediary entirely. The sidebar always handles Switch locally. Each client controls its own focus independently. No synchronization.

- Pros: Simplest change. Fix the bug without adding features. Each client works correctly in isolation.
- Cons: No pair programming support. Users who want to follow each other's navigation have no mechanism.

### C: Follow-the-leader (one-directional sync)

The primary client (client_id=1) is the leader. Its focus changes are broadcast to all other clients. Secondary clients can browse independently but see the leader's focus highlighted differently.

- Pros: Simple one-way sync. Leader is always in control. Observer can see what the leader is looking at.
- Cons: Asymmetric. Secondary clients can't drive the leader's focus. Less useful for true pair programming.

## Decision

**Chosen: A (two explicit modes)** with independent mode as the default and the immediate fix.

Phase 1: Implement independent mode. The sidebar calls `focus_terminal_pane` directly instead of routing through the controller. This fixes the bug where client 2's clicks affect client 1. No new protocol messages needed.

Phase 2: Add synchronized mode. The sidebar sends focus changes to the controller, which broadcasts a `cc-deck:focus-sync` message to all sidebars matching the controller's client_id. Each sidebar calls `focus_terminal_pane` locally on receipt. Toggle via sidebar header click or keybinding.

Read-only tokens should always be independent (observers should never drive focus).

## Key Requirements

1. **Independent mode (Phase 1):** Sidebar handles Switch locally via `focus_terminal_pane` from the sidebar plugin itself, not through the controller
2. **Synchronized mode (Phase 2):** Focus changes broadcast to all clients via a new `cc-deck:focus-sync` pipe message
3. **Mode toggle:** Accessible from the sidebar (header click or dedicated keybinding)
4. **Default:** Independent mode (safest default, fixes the immediate bug)
5. **Read-only clients:** Always independent, cannot sync focus to other clients
6. **Visual indicator:** Show current mode in the sidebar header (e.g., icon or text)

## Open Questions

- Should synchronized mode sync tab switches as well, or only pane focus within the current tab?
- How should the mode state be stored? Per-client (each client chooses its own mode) or global (one setting for the session)?
- Should the controller still receive Switch actions for session tracking (activity updates, last-active timestamp) even in independent mode?
- Does `focus_terminal_pane` called from a sidebar plugin on client_id=2 correctly focus the pane for client 2? (Needs empirical validation, similar to our client_id testing.)

## Related

- Brainstorm 082: Session sharing spike (section 6 documents the original finding)
- Spec 082: Multiplayer plugin resilience (prerequisite, client_id architecture)
- Brainstorm 085: Sidebar presence panel (would show which mode is active)
