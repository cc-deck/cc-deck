# Brainstorm: SpecKit Pair Programming Hooks

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

When two developers share a CC Deck session for pair programming, the SpecKit workflow (brainstorm, specify, plan, implement, review) should be visible and navigable by both parties. Today, the sidebar already shows spec workflow status via status icons and the verb tree. In a shared session, this information is naturally visible to the collaborator because they see the same Zellij session.

The question is whether SpecKit needs additional hooks or features specifically for the pair programming use case, or whether the existing sidebar integration is sufficient.

## Approaches Considered

### A: No changes needed (existing hooks suffice)

- Pros: Zero work, the shared session already shows everything. Both users see the sidebar, the spec status, and can interact with the terminal.
- Cons: No awareness of "who did what" in the spec workflow, no role differentiation (driver vs. navigator)

### B: Lightweight collaboration metadata

- Pros: Track which user triggered each spec workflow step, annotate the spec with contributor info
- Cons: Adds complexity to SpecKit's internal model for a niche use case

### C: Role-based pair programming mode

- Pros: Formal driver/navigator roles, turn-taking controls, role indicators in sidebar
- Cons: Over-engineering for what is essentially "two people looking at the same terminal"

## Decision

Parked: Start with approach A (no changes). The shared Zellij session already provides the pair programming experience because both users see the same sidebar, same spec status, same terminal. Revisit only if users report specific friction points.

**Preliminary direction:** The only SpecKit-specific enhancement worth considering is a `--pair` flag on spec workflow commands that adds a "Paired with: <username>" annotation to commit messages and spec metadata. Low effort, nice audit trail.

## Key Requirements

- Existing sidebar hooks (status icons, verb tree) must work correctly in shared sessions (verify during spike)
- Consider whether the sidebar plugin needs to handle multiple concurrent inputs gracefully
- Spec workflow commands should not break when two users invoke them simultaneously

## Open Questions

- Do Zellij plugins handle input from multiple connected clients correctly, or is there a race condition risk?
- Should SpecKit's gate reviews (spec review, plan review, code review) have a "co-reviewer" mode where both pair partners approve?
- Is there value in a "hand off" command that transfers the driver role and updates the sidebar indicator?
