# Deep Review Findings

**Date:** 2026-04-30
**Branch:** 047-landing-page-revival
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 1 | - | 1 |
| **Total** | **1** | **0** | **1** |

**Agents completed:** 5/5 (+ 0 external tools)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 75
- **File:** src/components/widgets/TabbedCode.astro:39,57
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (minor, no fix needed)

**What is wrong:**
The `aria-controls` attribute uses `tabpanel-${i}` and the panel `id` uses `tabpanel-${i}`, where `i` is a zero-based index local to each TabbedCode instance. If two TabbedCode components were rendered on the same page, both would generate elements with `id="tabpanel-0"` and `id="tabpanel-1"`, causing HTML ID collisions and breaking ARIA relationships.

**Why this matters:**
Duplicate IDs violate the HTML spec and cause accessibility tools to behave unpredictably. The `aria-controls` attribute would reference the wrong panel. However, the current page uses exactly one TabbedCode instance, so this is not a runtime bug today. It would become a bug if the component is reused.

**How it was resolved:**
Not fixed. This is a Minor finding with no current runtime impact. A future fix would use a unique prefix per instance (e.g., based on the `id` prop or a generated UUID). Since only one TabbedCode exists on the page, no action is required now.

## Remaining Findings

FINDING-1 remains as a Minor finding. It has no current runtime impact and does not affect spec compliance. If TabbedCode is reused on other pages with multiple instances, the IDs should be scoped per instance.
