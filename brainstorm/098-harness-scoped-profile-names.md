# Brainstorm: Harness-Scoped Profile Names

**Date:** 2026-10-19
**Status:** active

## Problem Framing

Profiles currently live in a flat `profiles` map keyed by name. That makes `private` globally unique even though profiles belong to a harness and their launch commands already include it. A developer cannot configure both `claude-private` and `codex-private` as separate profiles in the same workspace. The intended identity is the pair `(harness, profile name)`: names may repeat across harnesses, but a pair may appear only once.

The name-only assumption also reaches profile commands, `default_profile`, credential file paths, hook enrichment, and profile sync. A change to validation alone would leave those paths ambiguous or allow credential files to collide.

This project currently has one user. A one-time migration of that user's configuration and profile data is acceptable; automatic support for the old flat schema is unnecessary.

## Approaches Considered

### A: Flat collection with qualified identities

Keep profiles in one collection, but add a distinct harness-qualified identity for each entry. Existing entries could be interpreted as legacy bare names.

- Pros: Less visible change to the config layout; some existing references could remain as written.
- Cons: The stored key and the short profile name become different concepts, and compatibility rules make duplicate-looking names harder to understand.

### B: Group profiles under each harness (Chosen)

Use harness groups in `config.yaml`, with profile names inside each group. For example, `profiles.claude.private` and `profiles.codex.private` are separate definitions. The harness comes from the group rather than a repeated field in each profile. Migrate the current user's flat config and profile data as a one-time implementation task.

- Pros: The config expresses the intended identity directly; duplicate short names are natural; validation can enforce a clear schema.
- Cons: Existing config must be migrated, and every name-only consumer must be checked for ambiguity or storage collisions.

### C: List of profile records

Store each profile as a record with explicit `name` and `harness` fields, then validate that no pair occurs twice.

- Pros: The uniqueness rule is explicit and could support future per-profile metadata without key conventions.
- Cons: Replaces the current map shape, makes lookup less direct, and duplicates the harness and name text in every record.

## Decision

Choose B. Profile identity is `(harness, name)`, represented by nested harness groups. The old flat schema is migrated once on this machine during implementation, without a permanent compatibility parser. Existing wrapper names remain `claude-private`, `codex-private`, and so on.

For commands that select one profile, a bare name remains accepted while it identifies exactly one profile. If the same name exists under multiple harnesses, the command requires a harness-qualified reference and reports the available choices. The same rule applies to `default_profile` references. The formal spec will define the exact qualified spelling and serialization.

## Key Requirements

- Accept `private` once under Claude and once under Codex; reject a second `private` within either harness.
- Define one strict nested `profiles` schema. Recognized harness groups determine the profile harness, so profile entries do not need a second `harness` field. Invalid groups, duplicate pairs, and old flat entries produce actionable validation errors.
- Preserve the existing wrapper commands and their behavior in local, SSH, and OpenShell workspaces, including snapshot restore.
- Make all profile lookups, CLI operations, default selection, sync results, hook enrichment, and sidebar identity unambiguous when short names repeat.
- Keep credentials and per-profile state isolated by both harness and name. A one-time migration on this machine must preserve existing credentials and login state.
- Update tests and user-facing configuration and CLI documentation in the implementation phase.

## Open Questions

- What qualified reference spelling should commands and `default_profile` use, and should `profile use` always write the qualified form even when a bare name is currently unambiguous?
- Which existing credential and state paths need migration, and how should migration avoid overwriting user-owned files?
- Should the profile list and sidebar legend show a qualified identity whenever short names repeat, or always show the harness for consistency?
