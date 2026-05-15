# Spec Review: OpenShell Build Target

**Spec:** specs/056-openshell-build-target/spec.md
**Date:** 2026-05-15
**Reviewer:** Claude (spex:review-spec)

## Overall Assessment

**Status:** SOUND

**Summary:** The spec is well-structured, covers all required sections, and is directly implementable. Requirements are specific, testable, and derived from a thorough brainstorm session. Minor improvements suggested below.

## Completeness: 5/5

### Structure
- All required sections present (Purpose, Requirements, Success Criteria, Error Handling)
- Recommended sections included (Edge Cases, Out of Scope, Assumptions)
- No placeholder text or TBD markers

### Coverage
- 14 functional requirements, all specific and testable
- 3 error handling cases identified
- 4 edge cases with expected behavior defined
- 4 measurable success criteria

## Clarity: 4/5

### Language Quality
- Requirements use MUST consistently
- Acceptance scenarios use Given/When/Then format
- No vague terms ("should", "might", "appropriately")

### Minor Observations
- FR-006 says "all discovered binary paths" but should clarify: these are the binaries associated with the tool that uses that domain, not all binaries in the image
- The relationship between `tools[]` and `network.allowed_domains` is implicit. The spec doesn't define how the build system knows which tool uses which domain. This mapping is likely handled by the `/cc-deck.build` command (AI-driven), but the spec could note this dependency.

## Implementability: 5/5

### Plan Generation
- Clear target struct extension (mirrors existing ContainerTarget pattern)
- Build flow follows existing container target pattern
- CLI detection extension is straightforward
- Policy generation has well-defined merge semantics

## Testability: 5/5

### Verification
- Each user story has independent acceptance scenarios
- Success criteria are measurable (time, schema validity, correctness)
- Edge cases have expected outcomes
- Binary path discovery is testable per install method

## Constitution Alignment

- **Principle I (Tests + docs):** Spec doesn't mention tests/docs explicitly but doesn't preclude them. The tasks phase will add these per constitution.
- **Principle II (Interface contracts):** Not directly applicable (no existing interface being extended in the behavioral-contract sense). The TargetsConfig struct is extended but it's a data type, not a behavioral interface.
- **Principle III (Build/tool rules):** The spec correctly uses podman terminology. No `go build` or `cargo build` references.

## Recommendations

### Important (Should Fix)
- [ ] Clarify in FR-006 that "discovered binary paths" means the binaries associated with tools that use those specific domains, not all binaries in the image
- [ ] Add a note to Assumptions about the `/cc-deck.build` command being AI-driven (tool-to-domain mapping happens in the command prompt, not in Go code)

### Optional (Nice to Have)
- [ ] Consider adding FR for `cc-deck image verify --target openshell` to validate sandbox conventions (sandbox user exists, policy file at correct path, skills directory present)

## Spec Review Guide (30 minutes)

> This section guides a spec reviewer through the key decisions that need
> human judgment.

### Understanding the spec (8 min)

- Start with Purpose: single manifest, multiple targets, policy derivation from network config
- Then User Story 1 for the core flow, User Story 2 for policy override semantics
- Question: Is the "one manifest, multiple targets" approach the right level of integration, or should OpenShell builds be a separate workflow?

### Key decisions that need your eyes (12 min)

**Override vs. union for policy merge** (FR-007)

Explicit policy entries override auto-generated ones for the same endpoint host. This was chosen for predictability, but means a user who adds one per-binary rule for a domain loses the auto-generated rules for that domain's other binaries.
- Question: Should the override be at the endpoint level (whole domain replaced) or at the binary level (individual binary rules replaced)?

**Permissive default for unknown tools** (FR-006, Error Handling)

When a tool has no known binary path, all binaries are allowed for that tool's domains. This is the safest default for usability but the most permissive from a security perspective.
- Question: Is this the right trade-off for sandbox security, or should unknown tools be denied network access by default?

### Areas of uncertainty (5 min)

- The tool-to-domain mapping is implicit. The spec assumes the build system knows which tool needs which domain, but this mapping is not defined in the manifest schema. It relies on the AI-driven `/cc-deck.build` command inferring relationships.
- The spec lists specific filesystem paths in FR-013 (read_only, read_write). These may need adjustment for different OpenShell base image versions.

### Scope boundaries (5 min)

- Out of scope is well-defined: no capture phase changes, no skills integration, no runtime policy overrides, no push changes
- Question: Is deferring `cc-deck image verify --target openshell` the right call, or should basic validation (policy file exists, sandbox user exists) be in scope?

## Conclusion

Spec is sound and ready for implementation. The two "Important" recommendations are documentation clarifications, not blocking issues. The core design (manifest extension, Containerfile generation, policy derivation with overrides) is clear and follows established patterns.

**Ready for implementation:** Yes

**Next steps:**
- Plan with `/speckit-plan`
- Or implement directly with `/speckit-implement`
