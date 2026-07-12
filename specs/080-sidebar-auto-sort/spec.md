# Feature Specification: Sidebar Auto-Sort with Pause Zone

**Feature Branch**: `080-sidebar-auto-sort`
**Created**: 2026-07-11
**Status**: Draft
**Input**: Brainstorm 080 - Sidebar Auto-Sort with Pause Zone

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Paused Sessions Automatically Sink (Priority: P1)

A user has 6 sessions running in the sidebar. They pause 2 sessions to focus on others. The paused sessions automatically move below a visual separator line, keeping the active zone compact and visible without scrolling.

**Why this priority**: This is the core value proposition. Users should not need to manually sort every time they pause a session.

**Independent Test**: Start 4 sessions. Pause session #2. Verify it moves below a separator line. The remaining 3 active sessions stay above, in their original relative order.

**Acceptance Scenarios**:

1. **Given** 4 active sessions in the sidebar, **When** the user pauses session #2, **Then** session #2 moves below a separator line and the other 3 sessions remain above in their original order.
2. **Given** 2 paused sessions below the separator, **When** the user unpauses one, **Then** it reappears at the bottom of the active zone (not at its original position).
3. **Given** no sessions are paused, **When** all sessions are active, **Then** no separator line is displayed.

---

### User Story 2 - Coexistence with Manual Sort (Priority: P2)

A user has auto-sort enabled (default). They press S in navigate mode to apply the finer three-tier sort within the active zone. Both sort mechanisms work together without conflict.

**Why this priority**: The manual sort provides fine-grained control within the active zone. Both must coexist.

**Independent Test**: With auto-sort on and 2 paused sessions below the separator, press S. Verify the active zone sessions re-sort by tier (Working/Waiting above Idle/Done) while paused sessions stay below the separator.

**Acceptance Scenarios**:

1. **Given** auto-sort is on with paused sessions below the separator, **When** the user presses S, **Then** the active zone sessions sort by three tiers while the paused zone remains unchanged below the separator.
2. **Given** the user presses S to sort, **When** a session transitions to paused, **Then** auto-sort moves it below the separator and the manual sort order within the active zone is preserved for the remaining sessions.

---

### User Story 3 - Configuration Toggle (Priority: P3)

A user who prefers manual control disables auto-sort in their configuration file. The sidebar shows all sessions in tab order with no separator line, identical to the behavior before this feature.

**Why this priority**: Users must be able to opt out of automatic reordering.

**Independent Test**: Set `sidebar.auto_sort: false` in the config file. Pause a session. Verify it stays in its tab position and no separator line appears.

**Acceptance Scenarios**:

1. **Given** `sidebar.auto_sort` is `false`, **When** a session is paused, **Then** it stays in its current sidebar position and no separator is rendered.
2. **Given** `sidebar.auto_sort` is not set in config, **When** the sidebar renders, **Then** auto-sort is enabled by default.
3. **Given** auto-sort is disabled, **When** the user presses S, **Then** the manual three-tier sort works as before (unchanged behavior).

---

### Edge Cases

- What happens when the last active session is paused? The separator disappears since all sessions are now in the paused zone. No empty active zone is shown.
- What happens when a new session is created while auto-sort is active? It appears at the end of the active zone (above the separator).
- What happens when a session is deleted while auto-sort is active? It is removed normally. If it was the only paused session, the separator disappears.
- What happens when a paused session receives an event that changes its status back to Working? It moves to the active zone (above the separator) at the bottom of the active zone, same as unpausing.
- What happens when the plugin starts and some sessions are already paused? Auto-sort immediately applies the two-zone split on the first render. Sessions already in the Paused state appear below the separator from the start.
- What happens if the auto-sort config value changes between plugin restarts? The new value takes effect on the next plugin load. No mid-session config reload is supported.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When auto-sort is enabled and a session transitions to the Paused state, the sidebar MUST move that session below a visual separator line.
- **FR-002**: When auto-sort is enabled and a paused session transitions to any non-paused state, the sidebar MUST move that session to the end of the active zone (above the separator).
- **FR-003**: The separator line MUST only be rendered when at least one session is paused and at least one session is active. No separator for all-active or all-paused.
- **FR-004**: The auto-sort MUST preserve relative tab order within each zone (stable sort). Sessions within the active zone maintain their relative positions; sessions within the paused zone maintain their relative positions.
- **FR-005**: Auto-sort MUST be virtual (display-only). No physical tab reordering in the terminal multiplexer.
- **FR-006**: Auto-sort MUST be configurable via the `auto_sort` KDL plugin parameter (matching the existing `sidebar_width` pattern), defaulting to `true`. The cc-deck CLI MAY read a `sidebar.auto_sort` value from `~/.config/cc-deck/config.yaml` and inject it into the KDL layout when launching the plugin.
- **FR-007**: When auto-sort is disabled, the sidebar MUST render sessions in tab order with no separator, identical to the behavior before this feature.
- **FR-008**: The manual three-tier sort (S keybinding) MUST continue to work when auto-sort is enabled. The manual sort operates within the active zone only; paused sessions stay below the separator.
- **FR-009**: When auto-sort is enabled and the manual sort is also active, the manual sort order MUST apply within the active zone while the auto-sort active/paused split MUST apply between zones.

### Key Entities

- **Active Zone**: The top section of the sidebar containing all non-paused sessions (Working, Waiting, Idle, Done, AgentDone, Init states).
- **Paused Zone**: The bottom section of the sidebar containing all paused sessions.
- **Separator**: A visual divider line rendered between the active and paused zones.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When a session transitions to Paused, the sidebar automatically reorders to place it in the paused zone without any user input.
- **SC-002**: The separator line appears within the same render cycle as the state transition (no visible delay after pausing).
- **SC-003**: Sessions that transition from paused to active always appear at the bottom of the active zone (predictable, minimal movement).
- **SC-004**: Disabling auto-sort in configuration produces a sidebar identical to the pre-feature behavior (zero visual or behavioral difference).

## Out of Scope

- Runtime keybinding to toggle auto-sort on/off (configuration file only for now).
- Auto-sorting by activity tier (Working/Waiting vs Idle/Done) within the active zone. That remains the manual S sort's job.
- Physical tab reordering. Auto-sort is virtual, consistent with spec 074.
- Separator line styling preferences (the implementation chooses an appropriate style).

## Assumptions

- The existing virtual sort infrastructure from spec 074 (`sort_order` field in controller state) provides the foundation for auto-sort ordering.
- The sidebar rendering path already supports custom session ordering via `sort_order`. Auto-sort extends this by always maintaining a two-zone order when enabled.
- The `auto_sort` plugin parameter follows the existing KDL config pattern used by `sidebar_width`: parsed in `PluginConfig::from_configuration()` from the KDL layout's key-value pairs. The cc-deck CLI may additionally support a `sidebar.auto_sort` key in `~/.config/cc-deck/config.yaml` that gets injected into the KDL layout at launch time.
- Auto-sort ordering is not persisted separately. It is recalculated from session states on each render and on plugin startup. The existing `sort_order` field in controller state handles manual sort persistence independently.
- The separator line is a simple visual element (e.g., a thin horizontal rule) rendered between the last active session and the first paused session. Exact styling is an implementation detail.
