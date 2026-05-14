# Review Guide: Render Pipeline Stability and CPU Optimization

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-14

---

## What This Spec Does

The cc-deck Zellij plugin sidebar flickers between showing sessions and showing nothing, activity indicators blink between wrong states, and Zellij consumes 600%+ CPU with idle tabs. This spec eliminates those symptoms by finding and fixing a phantom second controller instance, making render triggers conditional, and adding profiling to catch future regressions.

**In scope:** Dual controller investigation and elimination, sidebar bootstrapping for new tabs, render broadcast throttling, debug logging optimization, profiling instrumentation.

**Out of scope:** Changing the push-based render model to pull-based, modifying Zellij itself, sidebar-side rendering optimizations (color computation, layout), voice relay protocol changes. The push model exclusion is notable because it was seriously considered during brainstorming (Approach C) and rejected for latency reasons.

## Bigger Picture

This is a stability fix for the single-instance architecture introduced in [spec 030](../030-single-instance-arch/spec.md). That spec moved from N independent plugin instances to a controller-sidebar model, which eliminated sync protocol complexity but introduced a single point of failure: if the controller misbehaves, all sidebars are affected. The current symptoms are a direct consequence of that architectural choice interacting with Zellij's plugin loading behavior.

The recently completed [spec 052](../052-plugin-integration-e2e-testing/spec.md) added integration tests for the plugin. Those tests provide a regression safety net for the changes in this spec. If the render pipeline changes break existing behavior, the 052 tests should catch it.

After this spec, the render pipeline should be stable enough that future features (new sidebar modes, additional state displays) can be added without worrying about triggering CPU regressions or flickering.

---

## Spec Review Guide (30 minutes)

> Focus your time on the parts that need human judgment most.

### Understanding the approach (8 min)

Read the [Purpose](spec.md#purpose) and [brainstorm](../../brainstorm/053-render-pipeline-stability.md) for the full root-cause analysis. As you read, consider:

- Does the diagnosis of "phantom second controller" match what you have observed? The [brainstorm](../../brainstorm/053-render-pipeline-stability.md) documents specific debug log evidence, but the root cause has not been confirmed.
- Is the defensive guard approach (FR-007: skip processing when plugin_id == 0) sufficient even if the investigation does not find the root cause?

### Key decisions that need your eyes (12 min)

**Startup probe protocol** ([FR-006](spec.md#functional-requirements))

The plan introduces a ping/pong protocol where the controller with the higher plugin_id self-disables. This adds two new pipe message types and a brief race window during startup.
- Question: Is the plugin_id ordering assumption correct? Does Zellij always assign lower IDs to earlier-loaded plugins, or could the "legitimate" controller get a higher ID?

**Push-on-discovery for sidebar bootstrapping** ([FR-008, FR-009](spec.md#functional-requirements))

The plan sends a targeted render to newly-discovered sidebars in `discover_sidebars_from_manifest`, plus a one-shot fallback request after 3 ticks.
- Question: The sidebar subscribes to Timer events for the fallback. Does this introduce the same per-second overhead that the spec is trying to eliminate? Should the sidebar unsubscribe from Timer after receiving its first payload?

**Conditional handle_tab_update** ([FR-011](spec.md#functional-requirements))

The unconditional `mark_render_dirty()` at the end of handle_tab_update is removed. The plan relies on push-on-discovery to cover the bootstrapping gap this previously filled.
- Question: Are there other scenarios where the unconditional dirty was providing value beyond bootstrapping? For example, does it handle the case where a session's tab_name changes?

### Areas where I'm less certain (5 min)

- [FR-001](spec.md#functional-requirements): The investigation requirements are inherently open-ended. The plan assumes the second controller is caused by Zellij's `load_plugins` behavior with the same WASM URL, but this has not been confirmed. If the actual cause is different, the startup probe protocol may not be the right solution.

- [Plan Phase 5](plan.md#phase-5-debug-and-profiling-improvements-fr-003-fr-004): Buffered debug logging uses `static mut` for the log buffer, which requires `unsafe` in Rust. The plan does not discuss thread safety. WASI is single-threaded so this should be safe, but the approach may trigger clippy warnings or require explicit `unsafe` justification.

- [SC-002](spec.md#measurable-outcomes): The 30% CPU target is measured on the Zellij server process, not on plugin execution specifically. If Zellij's own overhead (layout rendering, event dispatch) accounts for significant CPU, the plugin changes may not be enough to reach the target even with perfect render throttling.

### Risks and open questions (5 min)

- If the startup probe sends a broadcast ping on every controller startup, does this interfere with sidebar instances that also receive the broadcast? Sidebars parse all pipe messages in their `pipe()` handler. Will they silently ignore `cc-deck:controller-ping`, or could it cause unexpected behavior?

- The plan adds Timer subscription to sidebars ([T024](tasks.md)). Currently sidebars only subscribe to Mouse, Key, and PermissionRequestResult. Adding Timer means Zellij will call `update()` on every sidebar every second. Does this negate some of the CPU savings from the render throttling?

- The one-shot render request ([FR-009](spec.md#functional-requirements)) is sent to the controller using `controller_plugin_id` learned from SidebarInit. But what if the sidebar sends the request before receiving SidebarInit? The controller_plugin_id would be None.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
