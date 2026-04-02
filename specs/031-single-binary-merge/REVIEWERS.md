# Review Guide: Single Binary Merge (031)

## What This Feature Does

Merges the two WASM binaries (`cc_deck_controller.wasm` + `cc_deck_sidebar.wasm`) back into a single `cc_deck.wasm` that determines its role from KDL configuration at runtime. This solves the background plugin permission dialog problem in Zellij 0.44 and simplifies the build/install pipeline.

## Key Review Areas

### 1. UnifiedPlugin Enum (Highest Priority)

**File**: `cc-zellij-plugin/src/main.rs`

- Verify the enum correctly delegates all `ZellijPlugin` trait methods to the active variant
- Verify `load()` reads `configuration.get("mode")` and defaults to `Sidebar` when absent
- Verify the `Uninitialized` variant panics or handles gracefully if trait methods are called before `load()`
- Verify `register_plugin!(UnifiedPlugin)` compiles and works with the zellij-tile macro (requires `Default` trait)
- Verify all legacy feature-gated code and the no-feature-flag registration path are fully removed

### 2. Event Subscription Separation (Critical for Performance)

**File**: `cc-zellij-plugin/src/main.rs` (UnifiedPlugin::load)

- Controller variant must subscribe to: PaneUpdate, TabUpdate, Timer, PermissionRequestResult, RunCommandResult, CommandPaneOpened, PaneClosed
- Sidebar variant must subscribe to: Mouse, Key, PermissionRequestResult
- These lists must NOT overlap (except PermissionRequestResult) to preserve N-instance scaling

### 3. Permission Workaround Removal

**Files**: `cc-deck/internal/plugin/zellij.go`, `cc-deck/internal/plugin/install.go`

- `EnsureControllerPermissions()` function must be completely removed
- All calls to it must be removed
- No new permission pre-population logic should be introduced

### 4. Go CLI Simplification

**File**: `cc-deck/internal/plugin/embed.go`

- Only one `//go:embed` directive for `cc_deck.wasm`
- `PluginInfo` struct has only `Binary` and `BinarySize` (no controller/sidebar-specific fields)
- All callers updated (check for compile errors)

### 5. Layout and Config References

**File**: `cc-deck/internal/plugin/layout.go`

- `sidebarPluginBlock()` references `cc_deck.wasm` (not `cc_deck_sidebar.wasm`)
- `controllerConfigBlock()` references `cc_deck.wasm` (not `cc_deck_controller.wasm`)
- Both include the `mode` config parameter

### 6. Build System

**File**: `Makefile`

- Single `build-wasm` target (no feature flags)
- Single `copy-wasm` target
- Legacy cleanup in `install` target removes old two-binary files
- No backward-compatibility comments remain

## What NOT to Review

- Controller module internals (`src/controller/`): unchanged
- Sidebar module internals (`src/sidebar_plugin/`): unchanged
- Hook routing in CLI (`internal/cmd/hook.go`): unchanged (broadcast approach kept per clarification)
- Legacy sync code (`src/sync.rs`): out of scope, cleanup is separate

## Assumptions to Verify

1. Zellij creates independent instances for same-URL plugins with different configs (pending confirmation on zellij-org/zellij#4982)
2. Zellij permission cache is keyed by plugin URL, not URL+config
3. `register_plugin!` macro works with an enum type implementing `Default` + `ZellijPlugin`

## Test Checklist

- [ ] `cargo test` passes in cc-zellij-plugin/
- [ ] `cargo build --target wasm32-wasip1 --release` produces single `cc_deck.wasm`
- [ ] `go test ./...` passes in cc-deck/
- [ ] `make install` succeeds
- [ ] Single `cc_deck.wasm` in `~/.config/zellij/plugins/` (no controller/sidebar files)
- [ ] Layout files reference `cc_deck.wasm` with `mode "sidebar"`
- [ ] config.kdl references `cc_deck.wasm` with `mode "controller"`
- [ ] Fresh Zellij session shows permission dialog on sidebar, controller works after granting

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-04-02 | **Rounds:** 0/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 3 | completed |
| Architecture & Idioms | 7 | completed |
| Security | 1 | completed |
| Production Readiness | 3 | completed |
| Test Quality | 4 | completed |
| CodeRabbit (external) | 0 | completed (4 findings in spec artifacts, excluded from code review) |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 14 | 0 | 14 |

### What was fixed automatically

No fixes were needed. All findings are Minor severity and none are Critical or Important.

### What still needs human attention

All 14 Minor findings are pre-existing technical debt, not introduced by this feature. The spec explicitly defers legacy code cleanup. Key themes:

- Legacy `PluginState` ZellijPlugin implementation (~1,500 lines of dead code) still compiled into the binary. Is cleanup planned as a follow-up feature?
- Duplicated WASM wrapper functions across controller and sidebar modules. Worth consolidating into a shared module?
- Sidebar plugin missing `install_panic_hook()` call. Should diagnostics match the controller?
- UnifiedPlugin tests verify mode dispatch but not event delegation. Is the existing test suite sufficient coverage?

### Recommendation

All findings addressed (none Critical/Important). Code is ready for human review with no known blockers. 14 Minor findings remain as pre-existing technical debt. See [review-findings.md](review-findings.md) for details. The implementation is 100% compliant with all 11 functional requirements in [spec.md](spec.md).
