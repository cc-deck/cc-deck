# Deep Review Findings: 074-virtual-sort-fix

## Agent Results Summary

| Agent | Findings | Critical | Important | Minor |
|-------|----------|----------|-----------|-------|
| Correctness | 0 | 0 | 0 | 0 |
| Architecture & Idioms | 0 | 0 | 0 | 0 |
| Security | 0 | 0 | 0 | 0 |
| Production Readiness | 1 | 0 | 0 | 1 |
| Test Quality | 1 | 0 | 0 | 1 |

**Total: 2 findings (0 Critical, 0 Important, 2 Minor)**

---

## Detailed Findings

### FINDING DR-001 (Production Readiness)
- **Severity**: Minor
- **Confidence**: 55
- **File**: cc-zellij-plugin/src/sidebar_plugin/render.rs
- **Lines**: 883-902
- **Category**: production-readiness
- **Description**: Sort indicator tests (test_sort_indicator_present_when_sort_active and test_sort_indicator_absent_when_sort_inactive) test inline logic using hardcoded `if true` / `if false` rather than exercising the actual render_header() function with a RenderPayload that has sort_active set.
- **Rationale**: These tests verify that the ANSI string literal is correct but do not test the integration path where render_header() reads payload.sort_active. A regression in how render_header() reads the field would not be caught.
- **Fix**: Not required for this feature. The tests verify the indicator string values are correct, and the rendering path is implicitly tested by the full render pipeline. A future enhancement could add an integration-level render test.

### FINDING DR-002 (Test Quality)
- **Severity**: Minor
- **Confidence**: 45
- **File**: cc-zellij-plugin/src/sidebar_plugin/input.rs
- **Lines**: 947-952
- **Category**: test-quality
- **Description**: test_nav_s_sort_passes_cursor_pane_id asserts that navigate mode is preserved after pressing S but does not verify that sort_cursor_pane_id was actually set to the expected pane_id (20 in the test setup).
- **Rationale**: The test name implies it verifies cursor pane_id passing, but the assertion only checks mode preservation. The actual pane_id tracking is not verified by assertion.
- **Fix**: Not blocking. The cursor tracking works correctly in practice (verified by manual review of the implementation). A future enhancement could add `assert_eq!(state.sort_cursor_pane_id, Some(20))` to strengthen the test.

---

## Findings Not Raised (Considered and Dismissed)

1. **Two-pass stable sort performance**: The two-call pattern (`sort_by_key` for tab_index, then `sort_by_key` for tier) is O(n log n) twice, which is fine for <100 sessions. Not a concern.

2. **usize::MAX sentinel for missing tab_index**: Sessions without tab_index sort to the end. This is correct behavior per EH-003 (excluded from sort computation). No overflow risk since it is used as a sort key, not for arithmetic.

3. **serde(default) backward compatibility**: The `#[serde(default)]` on `sort_active: bool` in RenderPayload correctly handles old payloads that lack the field (defaults to false). No migration needed.

4. **sort_active not persisted across restarts**: Spec explicitly lists "Persistent sort preference" as out of scope. The default-false behavior on restart is correct.

5. **ANSI escape injection via sort indicator**: The indicator is a hardcoded string literal, not user-controlled input. No injection risk.

6. **Rapid sort toggling DoS**: Each toggle is O(1) (flip a bool + mark dirty). Render coalescing already batches multiple state changes into one broadcast. No DoS vector.

7. **Architecture: sort_tier visibility (pub(super))**: Correct. It needs to be visible to render_broadcast.rs within the controller module but should not be pub to external modules.
