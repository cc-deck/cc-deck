# Feature Specification: Render Pipeline Stability and CPU Optimization

**Feature Branch**: `053-render-pipeline-stability`
**Created**: 2026-05-14
**Status**: Draft
**Input**: Brainstorm session 053 and user discussion

## Purpose

Eliminate render pipeline instability (session flickering, activity indicator blinking, session count oscillation) and reduce CPU overhead in the cc-deck Zellij plugin. The root cause is a phantom second controller instance that broadcasts conflicting state to sidebars. Secondary causes are excessive render triggers from TabUpdate events and fading color recomputation. The investigation phase must also explore WASI cache directory interactions as a potential contributor, optimize debug logging to reduce overhead, and add profiling hooks to establish performance baselines and detect regressions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stable Session Display (Priority: P1)

A developer opens Zellij with multiple tabs, each running a Claude Code session via cc-deck. The sidebar in every tab shows the same consistent session list without flickering, blinking, or oscillation. Sessions appear, update, and disappear only in response to actual state changes (hook events, pane open/close).

**Why this priority**: Flickering is the most visible and disruptive symptom. It makes the sidebar unusable for monitoring session state.

**Independent Test**: Open 14 tabs with Claude Code sessions. Observe the sidebar for 60 seconds in steady state. No visual changes should occur unless a session state actually changes.

**Acceptance Scenarios**:

1. **Given** 14 idle Claude Code sessions across 14 tabs, **When** no hook events fire for 60 seconds, **Then** all sidebars display the same stable session list with no visual changes
2. **Given** a sidebar showing 8 sessions, **When** a new session starts in a new tab, **Then** all sidebars update once to show 9 sessions and remain stable
3. **Given** a session transitions from Working to Done, **When** the activity indicator updates, **Then** it changes exactly once and stays in the new state until the next legitimate state transition

---

### User Story 2 - Low CPU Usage at Idle (Priority: P2)

A developer leaves Zellij running with multiple idle tabs. The plugin uses minimal CPU resources, allowing other processes to run without competition. CPU usage stays under 30% with 14 idle tabs.

**Why this priority**: High CPU usage (600%+) degrades the developer's entire system, not just the plugin. This is the second most impactful symptom after flickering.

**Independent Test**: Open 14 tabs with idle Claude Code sessions. Measure CPU usage over 30 seconds. It must stay under 30%.

**Acceptance Scenarios**:

1. **Given** 14 idle Claude Code sessions, **When** no state changes occur for 30 seconds, **Then** the combined plugin CPU usage (all instances) stays under 30%
2. **Given** 14 sessions with Done/Idle status, **When** fade animations complete, **Then** render broadcasts drop to zero per second (no continuous recomputation)
3. **Given** a single session transitions state, **When** the render broadcast fires, **Then** only one broadcast occurs (not one per second continuously)

---

### User Story 3 - Reliable First Render for New Sidebars (Priority: P2)

When a developer opens a new tab, the sidebar in that tab displays the current session list within 3 seconds. No sidebar ever shows a permanent "No Claude sessions" message when sessions exist.

**Why this priority**: A blank sidebar on new tabs breaks trust in the tool and requires manual workarounds.

**Independent Test**: With 8 active sessions, open a new tab. The new sidebar must show all 8 sessions within 3 seconds.

**Acceptance Scenarios**:

1. **Given** 8 active sessions and a new tab is opened, **When** the sidebar loads, **Then** it displays all 8 sessions within 3 seconds
2. **Given** the controller has not yet processed a PaneUpdate, **When** a sidebar loads, **Then** it shows "Connecting..." (not "No Claude sessions") until the first payload arrives
3. **Given** a sidebar sends a render request after 3 ticks with no payload, **When** the controller receives it, **Then** the controller responds with a targeted render payload

---

### User Story 4 - Performance Visibility (Priority: P3)

A developer or maintainer can observe render pipeline performance metrics to detect regressions. Profiling data is available when enabled, covering broadcast frequency, serialization cost, and delivery counts.

**Why this priority**: Without performance baselines, future changes may reintroduce CPU regressions that go undetected until they become severe.

**Independent Test**: Enable profiling, run with 14 tabs for 60 seconds, verify that performance data is recorded and includes render-specific metrics.

**Acceptance Scenarios**:

1. **Given** profiling is enabled, **When** the plugin runs for 30 seconds, **Then** performance data includes render broadcast count, serialization time, and pipe delivery count
2. **Given** debug logging is enabled, **When** 14 tabs are idle, **Then** debug I/O overhead does not measurably increase CPU usage compared to debug-disabled baseline

---

### Edge Cases

- What happens when Zellij is reattached after detach? Controller restores sessions from cache, sidebars re-register via discovery, and push-on-discovery ensures immediate payload delivery.
- What happens during rapid tab open/close? Conditional render marking must still trigger on tab count changes to prevent stale sidebar state.
- What happens when a sidebar loads before the first PaneUpdate arrives? The one-shot fallback render request covers this 0-3 second bootstrapping gap.
- What happens when all sessions are idle for an extended period? Fade throttling limits broadcasts to 1 per 5 seconds during active fading, then zero broadcasts once fading completes.

## Requirements *(mandatory)*

### Functional Requirements

**Investigation & Diagnostics**

- **FR-001**: The system MUST investigate and identify why a second controller instance appears (layout config, plugin URL reuse, plugin loading behavior)
- **FR-002**: The system MUST explore whether the shared WASI cache directory contributes to dual-instance behavior or state leakage between plugin instances
- **FR-003**: The system MUST optimize debug logging to reduce I/O overhead when debug mode is enabled (buffering, conditional formatting, reduced write frequency)
- **FR-004**: The system MUST add profiling instrumentation for render broadcast count, payload serialization time, and pipe delivery count per measurement interval

**Dual Controller Elimination**

- **FR-005**: The root cause of the phantom second controller MUST be identified and eliminated
- **FR-006**: If the root cause is external (Zellij behavior beyond the plugin's control), the system MUST ensure only one controller processes hook events. The CLI MUST use broadcast pipe delivery (no `--plugin` targeting) for hook events so that the active controller receives them regardless of which instance Zellij created first. A startup probe based on plugin_id comparison is NOT safe because Zellij's targeted pipe delivery routes hooks to a specific instance, and the probe could disable that exact instance.
- **FR-007**: Regardless of root-cause resolution, the controller MUST NOT process pipe messages when its plugin_id has not been set by a permission grant (defensive fallback)

**Sidebar Bootstrapping**

- **FR-008**: The controller MUST send a targeted render payload to each newly-discovered sidebar when it is first registered in the sidebar registry
- **FR-009**: A sidebar MUST send a one-shot render request to the controller if it has not received a render payload within 3 timer ticks after initialization
- **FR-010**: A sidebar MUST NOT send more than one render request during its lifetime

**Render Throttling**

- **FR-011**: The tab update handler MUST only mark the render as needing broadcast when focus, tab count, or session state has actually changed (no unconditional marking)
- **FR-012**: The existing 5-tick fade throttle MUST be preserved
- **FR-013**: The existing voice connection handler MUST continue to mark dirty only on initial connection, not on heartbeats

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With 14 idle sessions, no visual changes occur in any sidebar for 60 continuous seconds (zero flickering)
- **SC-002**: With 14 idle sessions, combined plugin CPU usage stays under 30% as measured over a 30-second window
- **SC-003**: Render broadcast frequency drops to zero per second when no state changes are occurring and fade animations have completed
- **SC-004**: New sidebars display the current session list within 3 seconds of loading
- **SC-005**: Only one controller instance processes hook events at any time (verified via log analysis showing a single plugin_id across all controller log entries)
- **SC-006**: When profiling is enabled, performance data includes render broadcast count, serialization time, and delivery count per interval
- **SC-007**: Debug logging overhead does not increase CPU usage by more than 5% compared to the debug-disabled baseline

## Error Handling

- If a duplicate controller instance is created by Zellij (due to the AddClient race), the orphaned instance is overwritten in Zellij's plugin_map. The defensive guard (FR-007) prevents it from processing events before permissions are granted. No crash, no user-visible error.
- If the one-shot render request from a sidebar receives no response (controller not yet ready), the sidebar displays "Connecting..." rather than "No Claude sessions" until a payload arrives.
- If profiling instrumentation encounters timing errors (clock unavailable), it silently degrades to no-op rather than blocking the render path.

## Dependencies

- Existing single-instance architecture (spec 030)
- Existing plugin integration test infrastructure (spec 052) for regression testing
- Zellij 0.43+ pipe message routing behavior

## Out of Scope

- Changing the push-based render model to a pull-based model
- Modifying Zellij itself to change plugin instantiation behavior
- Sidebar-side rendering optimizations (color computation, layout rendering)
- Voice relay protocol changes beyond existing fixes

## Open Questions

- Is the second controller a consequence of the plugin loading mechanism or an unintended side effect? The investigation phase (FR-001, FR-002) will determine this.
- Does the WASI shared cache directory cause state leakage between plugin instances? FR-002 will explore this.
- What is the minimal profiling overhead acceptable for always-on performance tracking? FR-004 will establish baselines to answer this.

## Clarifications

### Session 2026-05-14

- Q: How should simultaneous startup of two controllers be resolved in the startup probe? → A: ~~The controller with the higher plugin_id self-disables~~ Superseded: startup probe is unsafe because Zellij routes targeted pipe messages to a specific instance. The probe could disable the instance that receives hooks. Resolution: use broadcast pipe delivery for hooks so the active controller receives them regardless of Zellij's instance ordering.
- Q: Why does `load_plugins` create two controller instances? → A: Zellij bug. Race between background plugin loading (`load_plugins`) and `AddClient`: `add_client()` does not recognize the `initiating_client_id` as already connected, re-creates the plugin instance for the same `(plugin_id, client_id)` pair. The first instance is orphaned. See `brainstorm/zellij-load-plugins-duplicate-instance.md` for upstream issue draft.

## Assumptions

- The Zellij pipe message routing behavior (targeted vs. broadcast) works as documented and will not change in patch releases
- The existing PaneManifest-based sidebar discovery is reliable and covers all sidebar registration scenarios
- A 3-tick (3-second) timeout for the one-shot render request fallback provides sufficient margin for controller initialization
- CPU measurement via system monitoring tools provides accurate per-process readings for the Zellij server process
