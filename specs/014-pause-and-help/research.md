# Research: 014-pause-and-help

No unknowns. This feature builds directly on the existing 013-keyboard-navigation infrastructure.

## R1: Pause Icon

**Decision**: Use ⏸ (U+23F8, double vertical bar) as the pause icon.
**Rationale**: Universally recognized pause symbol. Available in most terminal fonts.

## R2: Help Overlay Approach

**Decision**: Overlay rendering (replace session list temporarily with help text).
**Rationale**: Simpler than floating panes, fits within existing render function. Any key dismisses.
**Alternative**: Notification-based (too small), floating pane (overkill).

## R3: Pause State in Sync

**Decision**: Add `paused: bool` to `Session` struct (already serializable via serde).
**Rationale**: Session is already synced between instances via `broadcast_state`. The `paused` field will be included automatically.
