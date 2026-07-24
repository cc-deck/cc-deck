# Deep Review Findings

**Date:** 2026-07-23
**Branch:** 083-multiplayer-focus-modes
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 5 | 5 | 0 |
| Minor | 10 | 0 | 10 |
| Notable | 1 | - | 1 |
| **Total** | **16** | **5** | **11** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** 0

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/sidebar_plugin/render.rs:485-491
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Presence indicators overwrote the right edge of line 1 with `\x1b[0m` reset, clearing the background color on highlighted (active/cursor) rows. This created a visual artifact where highlighted rows lost their background at the indicator position.

**How it was resolved:**
Changed the indicator overlay to restore the row's background/foreground colors (`bg` + `fg`) after the indicator instead of emitting a full reset. Non-highlighted rows still use `\x1b[0m`.

### FINDING-2
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:181
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Color index calculation `(cid as usize) % colors.len()` used client_id directly, but Zellij client_ids start at 1 and multiplayer colors are 0-indexed. Client_id 10 mapped to colors[0] (player_1) instead of colors[9] (player_10).

**How it was resolved:**
Changed to `(cid.saturating_sub(1) as usize) % colors.len()` for correct 1-to-0-based index conversion.

### FINDING-3
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/pipe_handler.rs:76-79
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`PipeAction::FocusReport` variant was dead code. The controller handled `cc-deck:focus-report` directly in mod.rs via string matching before `parse_pipe_message()` ran, so the PipeAction variant was never matched.

**How it was resolved:**
Removed the variant and its parsing arm. The early-return pattern in controller/mod.rs is the canonical handler.

### FINDING-4
- **Severity:** Important
- **Confidence:** 92
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:200-230
- **Category:** production-readiness
- **Source:** architecture-agent (also reported by: production-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Per-sidebar payload cloning: the code cloned the entire RenderPayload and re-serialized it for every sidebar when presence was active. With 4 clients and 2 sidebars each, this meant 8 clone+serialize cycles instead of 4.

**How it was resolved:**
Group sidebars by client_id first, build and serialize one payload per client group. Reduces serializations from N sidebars to K clients.

### FINDING-5
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/sidebar_plugin/input.rs:752-808
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`switch_focus_locally()` bypassed the controller's auto-unpause logic. When a secondary client clicked a paused session, the session remained paused in the controller's state because the old `handle_switch` auto-unpause code was no longer in the path.

**How it was resolved:**
Added auto-unpause logic to `handle_focus_report()`: if the target session is paused, it gets unpaused (clears paused flag, updates auto_sort_tail, saves sessions), mirroring handle_switch behavior.

## Notable Observations

### NOTABLE-1
- **File:** cc-zellij-plugin/src/sidebar_plugin/state.rs:167-184
- **Category:** architecture
- **Source:** architecture-agent
- **Description:** `apply_local_activation_order()` uses O(N * log(N) * M) sort due to linear `position()` lookups. Acceptable at current scale (<20 sessions) but would benefit from a HashMap for position lookups if sessions grow.
- **Rationale:** Over long sessions with many ephemeral sessions, stale pane IDs accumulate in the Vec. A pruning step after sort would keep it bounded.

## Post-Fix Spec Coverage

All spec requirements verified after fix loop.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001 | switch_focus_locally() in input.rs | ok |
| FR-002 | send_focus_report() + pipe message | ok |
| FR-003 | handle_focus_report() updates activity | ok |
| FR-004 | local_activation_order in SidebarState | ok |
| FR-005 | apply_local_activation_order() | ok |
| FR-007 | client_focus HashMap in ControllerState | ok |
| FR-008 | build_other_client_focus() per client group | ok |
| FR-009 | presence_indicator_string() + overlay | ok |
| FR-010 | saturating_sub(1) % 10 color mapping | ok |
| FR-014 | ModeUpdate subscription | ok |
| FR-016 | None guard on multiplayer_colors | ok |
| FR-017 | Empty other_client_focus for single client | ok |

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1 | cargo test | 0 | 0 | passed |

## Remaining Findings

10 Minor findings remain (cosmetic, edge-case-only, or optimization opportunities). None block the gate. See individual agent reports for details.
