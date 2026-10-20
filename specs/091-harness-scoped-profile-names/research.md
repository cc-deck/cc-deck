# Research: Harness-Scoped Profile Names

## Profile storage and selectors

**Decision:** Replace the flat profile map with a harness-to-name map. Represent a resolved identity as one harness and one short name. Resolve `<harness>/<name>` directly; resolve a bare name only after searching all harness groups and finding exactly one match. Persist defaults in qualified form.

**Rationale:** The current `Config.Profiles`, CRUD helpers, validation, and CLI all assume a globally unique map key. A single resolver keeps user-facing ambiguity rules consistent, while runtime consumers that already know the harness can use exact pair lookup.

**Alternatives considered:** Flat qualified keys retain the old map shape but make map key, short name, and wrapper name separate concepts. A record list adds duplicate checking and loses direct lookup.

## Strict YAML validation

**Decision:** Reject the legacy flat shape, unknown harness groups, redundant inner `harness`, and duplicate mapping keys before normal profile validation. Test the decoder's duplicate-key behavior explicitly; add a node-level guard where it does not provide the required error.

**Rationale:** Permissive struct decoding can ignore an inner `harness` or silently accept fields that make the strict schema misleading.

**Alternatives considered:** A permanent compatibility loader was rejected in brainstorm 098; silently ignoring redundant fields undermines the schema.

## Profile files and one-time migration

**Decision:** Keep the existing data/login directory `~/.local/share/cc-deck/profiles/<name>/<harness>` because it already identifies the pair. Change credential files to `~/.config/cc-deck/profiles/<harness>/<name>` and update all wrapper, resolver, sync, and remote provisioning paths together. Migrate the current user's config and credential files explicitly after implementation, with a timestamped recoverable config copy and no overwrites. Retain old credential files until validation succeeds.

**Rationale:** The credential directory currently uses only the short name and would collide. The data directory already has both identity components, so moving login state adds risk without benefit. The live installation has `private` and `work` Claude profiles, a `private` default, and credential files under the old layout; file contents need not be printed during migration.

**Alternatives considered:** Reversing every data path to harness/name would require unnecessary login-state moves. Automatic migration on load contradicts the strict-schema decision.

## Session reporting and restore

**Decision:** Keep `CC_DECK_PROFILE` and stored session `profile` as short names. Use the existing agent/harness context together with the short name at every lookup, including hooks, background attribution, sidebar legend keys, and snapshot restore. Always label legend entries `<harness>/<name>`.

**Rationale:** Existing session and snapshot data already carry an agent identifier. Combining it with the short name avoids a wire-format migration and still gives exact identity.

**Alternatives considered:** Replacing the profile field with `harness/name` would break valid-name checks and legacy session data throughout Go and Rust.

## Workspace integrations

**Decision:** Include both harness and name in OpenShell provider names and deduplication keys. Use exact pair lookup for Kubernetes defaults, git credentials, local sync, SSH and OpenShell provisioning, and restore.

**Rationale:** OpenShell currently builds `cc-deck-<workspace>-<name>` and suppresses a second provider with the same short name. Other consumers use name-only lookups that become ambiguous.

**Alternatives considered:** Resolving ambiguous runtime names through the global default would select the wrong account and violate the spec.

## Verification and project constraints

**Decision:** Add focused Go and Rust tests for pair identity, ambiguity, path isolation, provider naming, hooks, legend, and snapshot restore; use project `make` targets for full validation. Update README, CLI reference, configuration reference, and the harness profiles guide, then run the prose checks required by the constitution.

**Rationale:** This feature crosses the Go CLI and runtime plus Rust display code. The constitution requires tests and documentation in the same feature.

**Alternatives considered:** Testing only configuration would miss collisions in remote provisioning and session display.
