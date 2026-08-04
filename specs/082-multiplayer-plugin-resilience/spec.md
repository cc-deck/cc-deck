# Feature Specification: Multiplayer Plugin Resilience

**Feature Branch**: `082-multiplayer-plugin-resilience`
**Created**: 2026-07-21
**Status**: Draft
**Input**: Brainstorm 087 - Multiplayer Plugin Resilience (revisited 2026-07-21 with empirical testing)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stable Sidebar During Multiplayer (Priority: P1)

A user shares their Zellij session with a colleague via `zellij attach`. Both users see the cc-deck sidebar in every tab. The sidebar displays session information correctly for the primary user without flickering, render storms, or duplicate entries. The second client's sidebar instances are silently excluded from render broadcasts.

**Why this priority**: This is the core problem. Without render broadcast filtering, every attached client multiplies the sidebar count (44+ observed in testing), causing visible flickering and 3-5x serialization overhead per render cycle.

**Independent Test**: Start a Zellij session with cc-deck. Attach a second terminal to the same session. Verify the primary client's sidebar remains stable (no flickering, correct session count, correct activity states). Detach the second client. Verify the sidebar continues to work without degradation.

**Acceptance Scenarios**:

1. **Given** a single-client Zellij session with cc-deck running normally, **When** a second client attaches to the session, **Then** the primary client's sidebar continues to render without flickering or duplicate entries.
2. **Given** two clients attached to the same session, **When** the controller broadcasts render payloads, **Then** only sidebars belonging to the controller's own `client_id` receive the payload (not zombie or other-client sidebars).
3. **Given** a second client that has attached and detached multiple times, **When** the sidebar registry is examined, **Then** zombie sidebar entries from disconnected clients do not accumulate in the broadcast target list.
4. **Given** 40+ zombie sidebar instances registered from prior client connections, **When** the controller renders, **Then** rendering completes without visible delay or flickering (graceful degradation).

---

### User Story 2 - Sidebar Registration Deduplication (Priority: P1)

When a second client attaches, Zellij creates new plugin instances that send `sidebar-hello` messages to the controller. The controller deduplicates registrations by (tab_index, client_id) pair so that each client has at most one sidebar per tab. Re-registrations from the same (tab, client_id) replace existing entries rather than accumulating duplicates.

**Why this priority**: Without dedup, sidebar count grows unboundedly with each client attach/detach cycle. This is the root cause of the render broadcast storms.

**Independent Test**: Attach a second client and verify the sidebar registry contains one entry per (tab, client_id). Detach and reattach. Verify the old client_id entries remain (zombie, but not broadcast to) and new entries use the new client_id. The registry never contains duplicate (tab, client_id) pairs.

**Acceptance Scenarios**:

1. **Given** a sidebar sends `sidebar-hello` with `client_id=1` on `tab=3`, **When** a second sidebar sends `sidebar-hello` with `client_id=1` on `tab=3`, **Then** the second registration replaces the first (no duplicate entry).
2. **Given** a sidebar sends `sidebar-hello` with `client_id=2` on `tab=3`, **When** `client_id=1` already has a sidebar on `tab=3`, **Then** both entries coexist in the registry (different clients).
3. **Given** a `sidebar-hello` message without a `client_id` field (old protocol), **When** the controller processes it, **Then** the registration succeeds with a default `client_id` of 0 (backward compatible).

---

### User Story 3 - Controller Election Stability (Priority: P2)

When a second client attaches, Zellij creates additional controller plugin instances. The leader election protocol uses `(client_id, plugin_id)` as the priority key (lowest wins) so that the primary terminal client's controller always wins over controllers from attached or web clients. Zombie controllers from disconnected clients cannot win elections.

**Why this priority**: Without client-aware election, a web client's controller could win the election (if it happens to get a lower plugin_id), then become a zombie when the web client disconnects, leaving no functioning leader until the heartbeat timeout triggers re-election.

**Independent Test**: Start a session. Note the leader's `(client_id, plugin_id)`. Attach a second client. Verify the original leader retains leadership (it has `client_id=1`, which is lower). Detach the second client. Verify the leader is unchanged.

**Acceptance Scenarios**:

1. **Given** a controller with `(client_id=1, plugin_id=0)` is the leader, **When** a new controller instance with `(client_id=2, plugin_id=0)` starts, **Then** the new instance goes dormant because `client_id=2 > client_id=1`.
2. **Given** a controller with `(client_id=2, plugin_id=0)` won the election (unusual case), **When** a controller ping arrives from `(client_id=1, plugin_id=0)`, **Then** `client_id=2` yields to `client_id=1` (lower client_id wins).
3. **Given** the leader controller is a zombie from a disconnected client, **When** the leader heartbeat times out, **Then** a new election starts and a controller from an active client wins.
4. **Given** a single-client session (no multiplayer), **When** the election runs, **Then** behavior is identical to the current implementation (backward compatible, `client_id` is simply the lowest available).

---

### User Story 4 - Zero Regression for Single-Client Operation (Priority: P1)

A user running cc-deck in a normal single-client Zellij session sees no behavioral changes. The `SidebarHello` protocol is backward compatible (the `client_id` field is optional with a default). The election protocol continues to work as before. Performance is unaffected.

**Why this priority**: Most users run single-client sessions. The multiplayer resilience changes must not introduce regressions or performance overhead for the common case.

**Independent Test**: Run all existing tests (`make test`). Start a normal single-client session. Verify sidebar rendering, session detection, voice relay, navigate mode, and all existing features work identically to before.

**Acceptance Scenarios**:

1. **Given** a single-client session, **When** the sidebar sends `sidebar-hello`, **Then** the controller registers it normally and renders to it (no filtering applied since all sidebars share the controller's `client_id`).
2. **Given** a `SidebarHello` payload from an older plugin version (without `client_id` field), **When** the controller deserializes it, **Then** deserialization succeeds with `client_id` defaulting to 0.
3. **Given** the controller election protocol includes `client_id` in the ping payload, **When** running in single-client mode, **Then** election behavior is identical to the current implementation (single client_id value, plugin_id tiebreaker).

---

### Edge Cases

- What happens when two clients have the same `client_id`? This cannot happen per Zellij's monotonically-increasing assignment, but the registry should handle it gracefully (last registration wins).
- What happens when the controller itself is a zombie? The heartbeat timeout mechanism (already existing) handles this: dormant controllers detect the timeout and start a new election.
- What happens when all controllers are zombies? No live controller processes pipe messages, so no sidebar updates occur. This is the expected behavior when all clients disconnect. On reattach, new controller instances start fresh elections.
- What happens during rapid attach/detach cycles? Each attach creates new plugin instances with new `client_id` values. The registry grows but render broadcasts only target the controller's own `client_id`, so performance is bounded by the broadcast list size (one per tab), not the registry size.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `SidebarHello` protocol MUST include an optional `client_id: u16` field with `#[serde(default)]` for backward compatibility (matches `zellij_tile::prelude::PluginIds::client_id` type)
- **FR-002**: Each sidebar MUST read its own `client_id` from `get_plugin_ids()` and include it in the `sidebar-hello` payload
- **FR-003**: The sidebar registry MUST store `(tab_index, client_id)` per `plugin_id` entry (changing from `plugin_id -> tab_index` to `plugin_id -> (tab_index, client_id)`)
- **FR-004**: On `sidebar-hello`, if a sidebar with the same `(tab_index, client_id)` already exists in the registry, the new registration MUST replace the old entry
- **FR-005**: Render broadcast MUST filter the sidebar registry to only include entries whose `client_id` matches the controller's own `client_id`. The untargeted `broadcast_render_all()` fallback MUST be removed or guarded to prevent bypassing the client_id filter
- **FR-006**: The controller MUST store its own `client_id: u16` (from `get_plugin_ids()`) in `ControllerState` during permission grant
- **FR-007**: The controller election ping payload MUST include `client_id` alongside `plugin_id`
- **FR-008**: Election priority MUST use `(client_id, plugin_id)` tuple comparison (lowest wins), where `client_id` is the primary sort key
- **FR-009**: The `discover_sidebars_from_manifest` function MUST assign the controller's own `client_id` to auto-discovered sidebar entries
- **FR-010**: All existing single-client functionality MUST remain unchanged (zero regressions)
- **FR-011**: The `cleanup_dead_sidebars` function MUST continue to remove registry entries for plugin_ids not in the PaneManifest

### Key Entities

- **SidebarHello**: Protocol message from sidebar to controller, adding `client_id: u16` field
- **SidebarRegistry**: Maps `plugin_id -> (tab_index, client_id)`, supports dedup by (tab, client_id)
- **ControllerState**: Gains `client_id: u16` field for the controller's own client identity
- **Election Ping**: Payload format changes from `plugin_id` to `client_id:plugin_id`

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing tests pass without modification (zero regressions)
- **SC-002**: With 2 clients attached, the render broadcast targets only the controller's client sidebars (verified via debug log: sidebar count per broadcast equals number of tabs, not number of tabs times number of clients)
- **SC-003**: With 40+ zombie sidebar entries in the registry, render broadcast completes in under 10ms (no amplification from zombie count)
- **SC-004**: Attaching and detaching a second client 3 times does not cause visible sidebar flickering on the primary client
- **SC-005**: Single-client session performance is identical to before (no measurable overhead from client_id tracking)
- **SC-006**: A `SidebarHello` payload without `client_id` field deserializes successfully (backward compatibility)
- **SC-007**: Controller election resolves correctly when controllers from two different clients compete (lowest client_id wins)

## Documentation Requirements

- Update README.md with multiplayer session limitations and known Zellij #4064 behavior
- No CLI reference changes (no new commands or flags)
- No configuration reference changes (no new config options)
- No Antora guide needed (internal architectural change, not user-facing feature)

## Clarifications

### Session 2026-07-21

No critical ambiguities detected. All functional requirements are testable and unambiguous. The spec is grounded in empirical testing (brainstorm 087 revisit) which resolved the key open questions about `client_id` behavior.

**Deferred to planning**: Registry warning threshold (100 entries) should be one-time per render cycle (not per-entry). Low impact, implementation detail.

## Assumptions

- Zellij's `get_plugin_ids().client_id` returns a stable, monotonically increasing u16 per client connection (validated by empirical testing on 2026-07-21)
- Zombie plugin instances (from disconnected clients) remain alive in the Zellij process until session kill (confirmed by Zellij #4064)
- The primary terminal client typically gets `client_id=1` (observed in testing, but the protocol does not depend on this specific value, only on ordering)
- Single-client sessions always have a consistent `client_id` across all plugin instances for that client
- The existing heartbeat timeout mechanism (already implemented) is sufficient for detecting zombie leaders

## Error Handling

- If `get_plugin_ids()` returns `client_id=0` in a context where a non-zero value is expected, treat it as a valid client_id (do not special-case zero)
- If the election ping payload cannot be parsed as `client_id:plugin_id`, fall back to the old `plugin_id`-only comparison (backward compatibility during rolling upgrades)
- If the sidebar registry grows beyond 100 entries, log a warning but do not cap or prune (the render broadcast filter limits actual work)
