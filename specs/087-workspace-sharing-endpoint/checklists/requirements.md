# Specification Quality Checklist: Workspace Sharing via External Endpoint

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-07
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`

### Validation iteration 1 (2026-09-07)

Two [NEEDS CLARIFICATION] markers were raised. They were the only two decisions in the source
design document with multiple reasonable readings and materially different user outcomes:

- **FR-014**: how a share created with verification skipped is reported afterwards. The design
  document specifies the override flag but never says what the resulting state is called.
- **FR-030**: whether a workspace session created by a combined start-and-share command is removed
  when verification fails. The design states that teardown never touches sessions, and separately
  that a failed share is rolled back. Those two statements conflict for this one case.

Everything else the source design left open was resolved with a documented default in the
Assumptions section rather than a marker, per the three marker limit.

### Validation iteration 2 (2026-09-07)

Both markers resolved by the author.

- **FR-014** resolved: no third state. A skipped verification reports the ordinary shared state
  with the verification age omitted. The absence of an age is the signal. This keeps the state
  model at two values and avoids a third rendering branch everywhere sharing status appears.
  Cascaded to US1 scenario 7, US4 scenario 3, the skipped-verification edge case, and a new
  assumption.
- **FR-030** resolved and split. FR-030 now covers rollback of sharing resources only, and a new
  FR-030a states that rollback never extends to the workspace session: sharing is additive, so a
  failed share leaves the workspace running and usable locally. Cascaded to US1 scenario 6, a new
  edge case, a new assumption, and SC-011.

All 16 checklist items now pass.

### Validation iteration 3 (2026-09-07) — review-spec gate

The `speckit.spex-gates.review-spec` gate found 6 Important and 5 Minor issues. All 11 were fixed.

Important:

1. `FR-030a` broke the `FR-NNN` numbering convention and would have been dropped from the task
   coverage matrix. Renumbered to FR-031, with the remaining requirements shifted. Requirements are
   now FR-001 through FR-042, sequential and unique.
2. FR-018 required a verification age unconditionally, contradicting FR-014, which omits the age for
   a share created with verification skipped. FR-018 now carries the exception.
3. FR-002 required the entry point to be "already serving", which FR-013 permits the user to leave
   unestablished. FR-002 now requires only that an address be determinable; serving is what
   verification establishes.
4. SC-002 referred to "the four failures recorded during the 2026-09-06 diagnostic session", which
   is neither self-contained nor accurate, since three of those four are not exposure failures an
   end to end test would exercise. SC-002 now enumerates the five failure modes the test must catch.
5. The edge case for a session ending during an active share implied prompt revocation, which
   FR-020 forbids by ruling out background checks. It now states that revocation happens at the next
   command that establishes state.
6. FR-033 through FR-035 described documentation content but named no artifact, so constitution
   Principle I would not have produced tasks. Added FR-038 through FR-042 naming the command
   reference, the configuration reference, the README, the guide page, and voice compliance, plus
   SC-012. This matters because the feature removes a configuration key and redefines the schema.

Minor:

7. User Story 4 had two acceptance scenarios numbered 3. Renumbered.
8. FR-016 said "healthy or degraded" where the rest of the specification says "shared or degraded".
   Normalized, and the two value state model is now stated in both FR-016 and the assumptions.
9. The edge case for two shares against one entry point stated no expected behaviour. It now states
   that this is permitted and that unsharing one must not affect the other.
10. SC-008 used "indistinguishable", which is not measurable. Now bounded at ten percent.
11. Neither FR-010 nor FR-011 said whether verification consumes a real invitation. FR-010 now
    forbids it from counting against, consuming, or invalidating any invitation issued to a person.

The specification is ready for `/speckit-plan`.

Implementation detail scan: the source design is written at implementation level, naming Go
interfaces, file names, HTTP paths, and the Zellij web client. The specification deliberately
restates all of it behaviourally. Terms such as "long lived connection", "credential exchange",
and "supporting service" replace WebSocket, `POST /command/login`, and `zellij web` respectively,
so the specification stays valid if the transport or multiplexer changes.

Constitution alignment: FR-036 encodes Principle VI, that acceptance criteria must be expressible
entirely in cc-deck commands. FR-033 through FR-035 encode Principle I, that documentation ships
with the feature rather than after it.
