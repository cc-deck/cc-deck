# Review Guide: WASM Plugin Dead Code Removal and Code Health

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-06

---

## What This Spec Does

The cc-deck Zellij plugin went through a major architectural split (controller/sidebar), but the old monolithic code was never removed. About 4,000+ lines of dead code still sit alongside the live architecture, and a global `#![allow(dead_code)]` hides it all from the compiler. This spec removes that dead code, reorganizes what remains, and restores the compiler's ability to catch real issues.

**In scope:** Deleting legacy modules (state.rs, sidebar.rs, rename.rs, attend.rs, sync.rs, notification.rs), removing ~1,550 lines of dead `PluginState` impl blocks from main.rs, extracting debug/WASM code into focused modules, removing the global warning suppression.

**Out of scope:** Any functional changes to the controller or sidebar plugin behavior. This is purely a code health refactoring. The plan explicitly states FR-014: "existing behavior MUST remain unchanged." No new features, no behavior modifications.

## Bigger Picture

This cleanup is overdue technical debt from the controller/sidebar split (specs 012, 030). The dead code already caused a real bug: spec 042 documented a case where a permission fix was applied to the dead `PluginState::load()` instead of the live `SidebarRendererPlugin::load()`. That incident is the primary motivation here.

After this cleanup, the plugin codebase drops from ~14,400 lines to ~10,000 lines, and `main.rs` shrinks from 2,084 lines to under 500. More importantly, the compiler will catch future dead code instead of the global `#![allow(dead_code)]` silently suppressing it.

This is a prerequisite for further plugin development: any new feature (like the voice transcript recording in spec 048) will be easier to implement when developers can trust that every line of code they see is actually reachable.

---

## Spec Review Guide (30 minutes)

> Focus your time on the relocation decisions and the spec/plan divergence, since those are where judgment calls were made.

### Understanding the approach (8 min)

Read [Acceptance Scenarios for US1](spec.md#user-story-1---remove-legacy-dead-code-priority-p1) and the [Research Findings](plan.md#research-findings) section of the plan. As you read, consider:

- The research found that `sidebar.rs`, `sync.rs`, and `notification.rs` are classified differently than the spec implies. The spec lists them as "delete" targets, but the plan identifies active code in sidebar.rs (`HELP_LINES`) and sync.rs (several utility functions). Does the relocation strategy in [Phase 2](plan.md#phase-2-foundational-audit-and-relocate) handle this correctly?
- The plan's [module classification table](plan.md#dead-vs-active-module-classification) is the backbone of the entire cleanup. Is any module misclassified?

### Key decisions that need your eyes (12 min)

**Skipping keybindings.rs extraction** ([plan.md Keybinding Deduplication](plan.md#keybinding-deduplication))

The spec's FR-009 says keybinding registration "MUST be extracted into `src/keybindings.rs`." The plan recommends skipping this because the controller already has its own working copies in `controller/events.rs`, and the sidebar does not need keybinding registration. Creating a shared module would add abstraction for a single consumer.
- Question for reviewer: Should FR-009 be amended to match the plan's recommendation, or should the shared `keybindings.rs` module be created as the spec requires?

**sync.rs split across two destinations** ([research.md Relocation Strategy](research.md#decision-syncrs-relocation-strategy))

The plan splits sync.rs utilities into two locations: message-parsing functions go to `pipe_handler.rs`, file-cleanup functions go to `controller/state.rs`. The alternative was a single `sync_utils.rs` module.
- Question for reviewer: Is the two-destination split justified, or would a single utility module be simpler to maintain?

**Debug code extraction vs re-export** ([plan.md Phase 3](plan.md#phase-3-extract-and-reorganize-mainrs))

T027 suggests either updating all call sites from `crate::debug_log` to `crate::debug::debug_log`, or re-exporting from the crate root to minimize churn. The re-export approach touches fewer files but hides the actual module location.
- Question for reviewer: Which approach do you prefer? Re-export is less churn but arguably less transparent.

**PerfTimer/PerfTimerPipe conditional deletion** ([tasks.md T023](tasks.md))

T023 conditionally deletes `PerfTimer` and `PerfTimerPipe` from main.rs, but the plan notes they might be used by the controller. The task says "verify with grep first."
- Question for reviewer: Should this be resolved during planning (audited now) rather than left as a runtime decision during implementation?

### Areas where I'm less certain (5 min)

- [plan.md WASM Function Pair Audit](plan.md#wasm-function-pair-audit): Eight WASM shim functions (broadcast_action through auto_rename_tab) are marked "Verify" in the plan. The audit is deferred to implementation (T004). I traced the controller/sidebar imports but could not confirm whether these functions are called indirectly through other active code paths. If any are actually active, the deletion in T022 would break the build.

- [spec.md Edge Cases](spec.md#edge-cases): The spec says "shared helpers are relocated rather than deleted," but the plan's research found clean separation between active and legacy modules (no imports from legacy in controller/sidebar). If this finding is wrong and there are transitive dependencies I missed, the Phase 3 deletions could fail.

- [tasks.md T009](tasks.md): The test audit task asks to "port unique test scenarios," but determining what constitutes "unique" coverage is a judgment call. The legacy `state_machine_tests.rs` (1,080 lines) tests `SidebarMode` transitions exhaustively, while `sidebar_plugin/modes.rs` has 10 tests. The coverage gap could be significant, or the new architecture's mode system could be different enough that the old tests are irrelevant.

### Risks and open questions (5 min)

- The spec sets SC-002 at "at least 4,000 lines" reduction. The plan identifies ~4,490 lines of dead code (789 + 601 + 342 + 479 + 825 + 50 + 1,080 + 324) plus ~1,550 lines of PluginState impl in main.rs. But some of this is replaced by relocated code. Is the 4,000-line target realistic after accounting for code that moves rather than disappears?
- If the legacy fuzz tests (`fuzz_tests.rs`, 324 lines) use `proptest` and the active codebase does not, should property-based testing be ported to the new architecture, or is this a deliberate simplification?
- The plan does not mention updating `Cargo.toml` dependencies. If `proptest` is a dev-dependency only used by `fuzz_tests.rs`, should it be removed after deletion?

---

*Full context in linked [spec](spec.md) and [plan](plan.md).*
